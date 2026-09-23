use std::fmt;
use std::fs;
use std::path::PathBuf;
use std::process::ExitCode;
use std::str::FromStr;

use clap::{Parser, Subcommand, ValueEnum};
use mesh_core::domain::{AssetType, Classification, DataQuality, RegistryError};
use mesh_core::peer::{
    self, PeerError, PeerResult, Project, cache_status, publish, read_publication_request, refresh,
    resolve, stage,
};
use mesh_core::services::{
    ConsumeRequest, InitResponse, LineageReference, RegistryService, SearchRequest, ServeRequest,
};
use mesh_core::{DEFAULT_DB_FILENAME, init_db};
use serde::Serialize;
use thiserror::Error;

#[derive(Debug, Parser)]
#[command(name = "feam", version, about = "Feather Mesh CLI")]
struct Cli {
    #[arg(long, global = true, value_name = "PATH")]
    registry: Option<PathBuf>,
    #[arg(long, global = true, value_name = "ROOT", conflicts_with = "registry")]
    project: Option<PathBuf>,
    #[arg(long, global = true, value_enum, default_value_t = OutputFormat::Table)]
    format: OutputFormat,
    #[arg(long, global = true)]
    verbose: bool,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum OutputFormat {
    Table,
    Json,
}

#[derive(Debug, Subcommand)]
enum Command {
    Init {
        #[arg(long)]
        namespace: Option<String>,
        #[arg(long)]
        serving_dir: Option<PathBuf>,
        #[arg(long = "owner-team")]
        owner_teams: Vec<String>,
    },
    Serve {
        path: PathBuf,
        #[arg(long)]
        name: Option<String>,
        #[arg(long, value_enum)]
        asset_type: Option<CliAssetType>,
        #[arg(long)]
        version: Option<String>,
        #[arg(long)]
        owner_team: Option<String>,
        #[arg(long)]
        producer: Option<String>,
        #[arg(long)]
        usage_policy: Option<String>,
        #[arg(long, value_enum)]
        data_quality: Option<CliDataQuality>,
        #[arg(long, value_enum)]
        classification: Option<CliClassification>,
        #[arg(long)]
        description: Option<String>,
        #[arg(long)]
        intended_use: Option<String>,
        #[arg(long = "lineage")]
        lineage: Vec<String>,
        #[arg(long)]
        metadata: Option<PathBuf>,
    },
    Search {
        query: Option<String>,
        #[arg(long, value_enum)]
        asset_type: Option<CliAssetType>,
        #[arg(long, value_enum)]
        data_quality: Option<CliDataQuality>,
        #[arg(long, value_enum)]
        classification: Option<CliClassification>,
        #[arg(long)]
        owner_team: Option<String>,
    },
    Show {
        product_id_or_source_path: String,
        #[arg(long)]
        version: Option<String>,
    },
    Consume {
        product_id_or_source_path: String,
        #[arg(long)]
        version: String,
        #[arg(long)]
        out: PathBuf,
        #[arg(long)]
        overwrite: bool,
    },
    Lineage {
        product_id_or_source_path: String,
        #[arg(long)]
        version: Option<String>,
    },
    ValidateMetadata {
        metadata_file: PathBuf,
    },
    Teams,
    Products,
    Refresh,
    Cache {
        #[command(subcommand)]
        command: CacheCommand,
    },
    Resolve {
        reference: String,
        #[arg(long)]
        version: String,
        #[arg(long)]
        asset: Option<String>,
        #[arg(long)]
        verify_integrity: bool,
    },
    Withdraw {
        reference: String,
        #[arg(long)]
        version: String,
        #[arg(long)]
        reason: String,
    },
    Stac {
        #[command(subcommand)]
        command: StacCommand,
    },
    /// Launch the optional interactive project-scoped terminal UI.
    Tui {
        #[arg(long, value_enum, default_value_t = AgentMode::Off)]
        agent: AgentMode,
        #[arg(long, requires = "agent")]
        agent_profile: Option<String>,
    },
}

#[derive(Debug, Subcommand)]
enum CacheCommand {
    Status,
}

#[derive(Debug, Subcommand)]
enum StacCommand {
    Serve {
        #[arg(long)]
        token_file: PathBuf,
        #[arg(long, default_value = "127.0.0.1:8080")]
        addr: std::net::SocketAddr,
    },
}

#[derive(Debug, Clone, Copy, ValueEnum, PartialEq, Eq)]
enum AgentMode {
    Off,
    Hosted,
    Fake,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
#[value(rename_all = "snake_case")]
enum CliAssetType {
    File,
    Directory,
    Dataset,
    Table,
    ModelArtifact,
    ReportArtifact,
    ManifestCollection,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum CliDataQuality {
    Production,
    Qualified,
    Unverified,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum CliClassification {
    Public,
    Internal,
    Restricted,
}

#[derive(Debug, Error)]
enum AppError {
    #[error(transparent)]
    Registry(#[from] RegistryError),
    #[error(transparent)]
    Peer(#[from] PeerError),
    #[error("{0}")]
    Tui(String),
}

fn main() -> ExitCode {
    let cli = Cli::parse();
    let peer_machine = cli.project.is_some() && matches!(cli.format, OutputFormat::Json);
    match run(cli) {
        Ok(()) => ExitCode::SUCCESS,
        Err(AppError::Peer(err)) => {
            if peer_machine {
                let value = serde_json::json!({"protocol": peer::PEER_PROTOCOL_VERSION, "error": {"kind": err.kind(), "message": err.to_string()}});
                eprintln!(
                    "{}",
                    serde_json::to_string(&value).expect("error schema serializes")
                );
            } else {
                eprintln!("{err}");
            }
            ExitCode::from(err.exit_code())
        }
        Err(AppError::Registry(err)) => {
            eprintln!("{err}");
            ExitCode::from(exit_code(&err))
        }
        Err(AppError::Tui(err)) => {
            eprintln!("{err}");
            ExitCode::from(1)
        }
    }
}

fn run(cli: Cli) -> Result<(), AppError> {
    if let Command::Tui {
        agent,
        agent_profile,
    } = &cli.command
    {
        if matches!(cli.format, OutputFormat::Json) {
            return Err(AppError::Tui(
                "tui does not support --format json; use the noninteractive CLI".into(),
            ));
        }
        return run_tui(cli.project.clone(), *agent, agent_profile.clone());
    }
    if let Some(project_root) = cli.project.clone() {
        run_peer(cli, project_root).map_err(AppError::from)
    } else {
        run_legacy(cli).map_err(AppError::from)
    }
}

fn run_tui(
    project_root: Option<PathBuf>,
    agent: AgentMode,
    agent_profile: Option<String>,
) -> Result<(), AppError> {
    let project_root = project_root.ok_or_else(|| {
        AppError::Tui("feam tui requires --project ROOT and never falls back to registry.db".into())
    })?;
    let profile = match agent {
        AgentMode::Off => {
            if agent_profile.is_some() {
                return Err(AppError::Tui(
                    "--agent-profile requires --agent hosted".into(),
                ));
            }
            None
        }
        // An empty value means "use the configured default profile". `None`
        // remains distinct: manual mode never reads user agent configuration.
        AgentMode::Hosted => Some(agent_profile.unwrap_or_default()),
        AgentMode::Fake => Some("__fake__".into()),
    };
    #[cfg(feature = "tui")]
    {
        mesh_tui::run(mesh_tui::TuiOptions {
            project_root,
            agent_profile: profile,
        })
        .map_err(|error| AppError::Tui(error.to_string()))
    }
    #[cfg(not(feature = "tui"))]
    {
        let _ = (project_root, profile);
        Err(AppError::Tui(
            "feam tui is not enabled in this CLI-only build; rebuild with `--features tui`".into(),
        ))
    }
}

fn run_legacy(cli: Cli) -> Result<(), RegistryError> {
    let registry = cli
        .registry
        .unwrap_or_else(|| PathBuf::from(DEFAULT_DB_FILENAME));
    if let Command::Init {
        namespace,
        serving_dir,
        owner_teams,
    } = &cli.command
    {
        if namespace.is_some() || serving_dir.is_some() || !owner_teams.is_empty() {
            return Err(RegistryError::Validation(
                mesh_core::domain::ValidationError::new(
                    "init",
                    "--namespace and --serving-dir require --project",
                ),
            ));
        }
        init_db(&registry)?;
        return print_init(
            &InitResponse {
                registry_path: registry.to_string_lossy().into_owned(),
                status: "initialized".into(),
            },
            cli.format,
        );
    }
    let conn = init_db(&registry)?;
    let service = RegistryService::new(&conn);
    match cli.command {
        Command::Init { .. } => unreachable!(),
        Command::Serve {
            path,
            name,
            asset_type,
            version,
            owner_team,
            producer,
            usage_policy,
            data_quality,
            classification,
            description,
            intended_use,
            lineage,
            metadata,
        } => {
            if metadata.is_some() {
                return Err(RegistryError::Validation(
                    mesh_core::domain::ValidationError::new(
                        "metadata",
                        "peer publication metadata requires --project",
                    ),
                ));
            }
            let request = ServeRequest {
                source_path: path,
                name: required_legacy("name", name)?,
                asset_type: required_legacy("asset_type", asset_type)?.into(),
                version: required_legacy("version", version)?,
                owner_team: required_legacy("owner_team", owner_team)?,
                producer: required_legacy("producer", producer)?,
                usage_policy: required_legacy("usage_policy", usage_policy)?,
                data_quality: required_legacy("data_quality", data_quality)?.into(),
                classification: required_legacy("classification", classification)?.into(),
                description,
                intended_use,
                lineage: parse_lineage(lineage)?,
            };
            print_serve(&service.serve(request)?, cli.format)
        }
        Command::Search {
            query,
            asset_type,
            data_quality,
            classification,
            owner_team,
        } => print_products(
            &service.search_products(SearchRequest {
                query,
                asset_type: asset_type.map(Into::into),
                data_quality: data_quality.map(Into::into),
                classification: classification.map(Into::into),
                owner_team,
            })?,
            cli.format,
        ),
        Command::Show {
            product_id_or_source_path,
            version,
        } => print_show(
            &service.show_product(&product_id_or_source_path, version.as_deref())?,
            cli.format,
        ),
        Command::Consume {
            product_id_or_source_path,
            version,
            out,
            overwrite,
        } => print_consume(
            &service.consume(ConsumeRequest {
                product_ref: product_id_or_source_path,
                version,
                out,
                overwrite,
            })?,
            cli.format,
        ),
        Command::Lineage {
            product_id_or_source_path,
            version,
        } => print_lineage(
            &service.lineage(&product_id_or_source_path, version.as_deref())?,
            cli.format,
        ),
        Command::ValidateMetadata { metadata_file } => {
            service.validate_serve_request(&read_metadata(metadata_file)?)?;
            match cli.format {
                OutputFormat::Json => print_json(&serde_json::json!({"status": "valid"})),
                OutputFormat::Table => {
                    println!("Metadata valid");
                    Ok(())
                }
            }
        }
        Command::Teams => {
            let teams = service.list_teams()?;
            match cli.format {
                OutputFormat::Json => print_json(&teams),
                OutputFormat::Table => {
                    if teams.is_empty() {
                        println!("No teams registered");
                    } else {
                        println!("{:<8}  {:<24}  CREATED_AT", "TEAM_ID", "NAME");
                        for team in teams {
                            println!(
                                "{:<8}  {:<24}  {}",
                                team.team_id, team.name, team.created_at
                            );
                        }
                    }
                    Ok(())
                }
            }
        }
        Command::Products => print_products(&service.list_products()?, cli.format),
        Command::Refresh
        | Command::Cache { .. }
        | Command::Resolve { .. }
        | Command::Withdraw { .. }
        | Command::Stac { .. }
        | Command::Tui { .. } => Err(RegistryError::Validation(
            mesh_core::domain::ValidationError::new(
                "command",
                "peer data-access commands require --project",
            ),
        )),
    }
}

fn run_peer(cli: Cli, project_root: PathBuf) -> PeerResult<()> {
    let project = match &cli.command {
        Command::Init {
            namespace,
            serving_dir,
            owner_teams,
        } => {
            let namespace = namespace.clone().ok_or_else(|| PeerError::Validation {
                field: "namespace".into(),
                message: "--namespace is required with --project init".into(),
            })?;
            let initialized = Project::init_with_owner_teams(
                &project_root,
                namespace,
                serving_dir.clone(),
                owner_teams.clone(),
            )?;
            return print_peer(
                &serde_json::json!({"protocol": peer::PEER_PROTOCOL_VERSION, "project": initialized.root(), "status": "initialized"}),
                cli.format,
            );
        }
        _ => Project::open(&project_root)?,
    };
    match cli.command {
        Command::Init { .. } => unreachable!(),
        Command::Serve {
            path,
            metadata,
            name,
            asset_type,
            version,
            owner_team,
            producer,
            usage_policy,
            data_quality,
            classification,
            description,
            intended_use,
            lineage,
        } => {
            if name.is_some()
                || asset_type.is_some()
                || version.is_some()
                || owner_team.is_some()
                || producer.is_some()
                || usage_policy.is_some()
                || data_quality.is_some()
                || classification.is_some()
                || description.is_some()
                || intended_use.is_some()
                || !lineage.is_empty()
            {
                return Err(PeerError::Validation {
                    field: "serve".into(),
                    message: "legacy publication flags conflict with --project; use --metadata"
                        .into(),
                });
            }
            let metadata = metadata.ok_or_else(|| PeerError::Validation {
                field: "metadata".into(),
                message: "--metadata is required for project publication".into(),
            })?;
            if std::fs::canonicalize(path).map_err(|_| {
                PeerError::Policy("serve PATH must be the configured serving directory".into())
            })? != project.serving_root()?
            {
                return Err(PeerError::Policy(
                    "serve PATH must equal the project's configured serving directory".into(),
                ));
            }
            print_peer(
                &publish(&project, &read_publication_request(metadata)?)?,
                cli.format,
            )
        }
        Command::ValidateMetadata { metadata_file } => {
            let descriptor =
                peer::validate_publication(&project, &read_publication_request(metadata_file)?)?;
            print_peer(
                &serde_json::json!({"protocol": peer::PEER_PROTOCOL_VERSION, "status":"valid", "product_id":descriptor.product_id, "version":descriptor.version}),
                cli.format,
            )
        }
        Command::Refresh => print_peer(&refresh(&project)?, cli.format),
        Command::Cache {
            command: CacheCommand::Status,
        } => print_peer(
            &serde_json::json!({"protocol": peer::PEER_PROTOCOL_VERSION, "cache_path": project.cache_path(), "snapshot": cache_status(&project)?}),
            cli.format,
        ),
        Command::Resolve {
            reference,
            version,
            asset,
            verify_integrity,
        } => print_peer(
            &resolve(
                &project,
                &reference,
                &version,
                asset.as_deref(),
                verify_integrity,
            )?,
            cli.format,
        ),
        Command::Withdraw {
            reference,
            version,
            reason,
        } => print_peer(
            &peer::withdraw_qualified(&project, &reference, &version, &reason, None)?,
            cli.format,
        ),
        Command::Consume {
            product_id_or_source_path,
            version,
            out,
            overwrite,
        } => {
            let resolved = resolve(&project, &product_id_or_source_path, &version, None, false)?;
            print_peer(&stage(&resolved, out, overwrite)?, cli.format)
        }
        Command::Search {
            query,
            asset_type,
            data_quality,
            classification,
            owner_team,
        } => {
            use mesh_core::services::catalog_service::{CatalogQuery, query_products};
            let data_kind = match asset_type {
                None => None,
                Some(CliAssetType::Table) => Some(peer::DataKind::Table),
                Some(_) => return Err(PeerError::Validation { field: "asset_type".into(), message: "project search supports --asset-type table; other legacy asset types have no peer mapping".into() }),
            };
            let filter = CatalogQuery {
                text: query,
                data_kind,
                owner_team,
                quality: data_quality.map(|q| format!("{q:?}").to_ascii_lowercase()),
                classification: classification.map(|c| format!("{c:?}").to_ascii_lowercase()),
                limit: 100,
                ..Default::default()
            };
            print_peer(&query_products(&project, &filter)?, cli.format)
        }
        Command::Products => print_peer_products(&project, None, cli.format),
        Command::Show {
            product_id_or_source_path,
            version,
        }
        | Command::Lineage {
            product_id_or_source_path,
            version,
        } => {
            let version = version.ok_or_else(|| PeerError::Validation {
                field: "version".into(),
                message: "--version is required for project show/lineage".into(),
            })?;
            print_peer(
                &resolve(&project, &product_id_or_source_path, &version, None, false)?,
                cli.format,
            )
        }
        Command::Teams => {
            let teams = mesh_core::services::catalog_service::catalog_teams(&project)?;
            print_peer(&teams, cli.format)
        }
        Command::Stac {
            command: StacCommand::Serve { token_file, addr },
        } => {
            let token = mesh_core::stac_http::read_bearer_token(token_file)?;
            mesh_core::stac_http::serve(project, token, addr)
        }
        Command::Tui { .. } => unreachable!("TUI dispatch occurs before project routing"),
    }
}

fn required_legacy<T>(field: &str, value: Option<T>) -> Result<T, RegistryError> {
    value.ok_or_else(|| {
        RegistryError::Validation(mesh_core::domain::ValidationError::new(
            field,
            "is required without --project",
        ))
    })
}

fn print_peer<T: Serialize + ?Sized>(value: &T, format: OutputFormat) -> PeerResult<()> {
    match format {
        OutputFormat::Json => println!("{}", serde_json::to_string(value)?),
        OutputFormat::Table => println!("{}", serde_json::to_string_pretty(value)?),
    };
    Ok(())
}

fn print_peer_products(
    project: &Project,
    query: Option<&str>,
    format: OutputFormat,
) -> PeerResult<()> {
    let products = mesh_core::services::catalog_service::query_products(
        project,
        &mesh_core::services::catalog_service::CatalogQuery {
            text: query.map(str::to_owned),
            limit: 100,
            ..Default::default()
        },
    )?;
    print_peer(&products, format)
}

fn parse_lineage(values: Vec<String>) -> Result<Vec<LineageReference>, RegistryError> {
    values
        .into_iter()
        .map(|value| {
            let (source, version) = value
                .rsplit_once('@')
                .map(|(source, version)| (source.to_string(), Some(version.to_string())))
                .unwrap_or((value, None));
            Ok(LineageReference { source, version })
        })
        .collect()
}

fn read_metadata(path: PathBuf) -> Result<ServeRequest, RegistryError> {
    let contents = fs::read_to_string(path).map_err(map_io_error)?;
    serde_json::from_str(&contents).map_err(RegistryError::from)
}

fn print_init(response: &InitResponse, format: OutputFormat) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(response),
        OutputFormat::Table => {
            println!("Registry initialized: {}", response.registry_path);
            Ok(())
        }
    }
}

fn print_serve(
    response: &mesh_core::services::ServeResponse,
    format: OutputFormat,
) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(response),
        OutputFormat::Table => {
            println!("Published {}", response.name);
            println!("product_id: {}", response.product_id);
            println!("version_id: {}", response.version_id);
            println!("version: {}", response.version);
            println!("source_reference: {}", response.source_reference);
            println!("status: {}", response.status);
            Ok(())
        }
    }
}

fn print_products(
    products: &[mesh_core::services::ProductSummary],
    format: OutputFormat,
) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(products),
        OutputFormat::Table => {
            if products.is_empty() {
                println!("No products found");
            } else {
                println!(
                    "{:<10}  {:<24}  {:<18}  {:<18}  {:<10}  {:<12}  SOURCE",
                    "PRODUCT_ID", "NAME", "OWNER_TEAM", "PRODUCER", "VERSION", "QUALITY"
                );
                for product in products {
                    println!(
                        "{:<10}  {:<24}  {:<18}  {:<18}  {:<10}  {:<12}  {}",
                        product.product_id,
                        truncate(&product.name, 24),
                        truncate(&product.owner_team, 18),
                        truncate(&product.producer, 18),
                        product.version.as_deref().unwrap_or("-"),
                        product.data_quality.as_deref().unwrap_or("-"),
                        product.source_reference.as_deref().unwrap_or("-"),
                    );
                }
            }
            Ok(())
        }
    }
}

fn print_show(
    detail: &mesh_core::services::ProductDetail,
    format: OutputFormat,
) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(detail),
        OutputFormat::Table => {
            println!("product_id: {}", detail.product_id);
            println!("name: {}", detail.name);
            println!("owner_team: {}", detail.owner_team);
            println!("producer: {}", detail.producer);
            println!("usage_policy: {}", detail.usage_policy);
            if let Some(description) = &detail.description {
                println!("description: {description}");
            }
            if let Some(intended_use) = &detail.intended_use {
                println!("intended_use: {intended_use}");
            }
            println!("created_at: {}", detail.created_at);
            println!("version: {}", detail.selected_version.version);
            println!("version_id: {}", detail.selected_version.version_id);
            println!("asset_type: {}", detail.selected_version.asset_type);
            println!("data_quality: {}", detail.selected_version.data_quality);
            println!("classification: {}", detail.selected_version.classification);
            println!(
                "source_reference: {}",
                detail.selected_version.source_reference
            );
            println!("lineage_status: {}", detail.lineage.status);
            println!(
                "lineage_dependencies: {}",
                detail.lineage.dependencies.len()
            );
            Ok(())
        }
    }
}

fn print_lineage(
    lineage: &mesh_core::services::LineageResponse,
    format: OutputFormat,
) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(lineage),
        OutputFormat::Table => {
            println!("product_id: {}", lineage.product_id);
            println!("version: {}", lineage.version);
            println!("status: {}", lineage.status);
            if lineage.dependencies.is_empty() {
                println!("No upstream dependencies recorded");
            } else {
                println!(
                    "{:<32}  {:<12}  {:<10}  PRODUCER",
                    "UPSTREAM_SOURCE", "VERSION", "PRODUCT_ID"
                );
                for dep in &lineage.dependencies {
                    println!(
                        "{:<32}  {:<12}  {:<10}  {}",
                        truncate(&dep.upstream_source_reference, 32),
                        dep.upstream_version.as_deref().unwrap_or("-"),
                        dep.upstream_product_id
                            .map(|id| id.to_string())
                            .unwrap_or_else(|| "-".to_string()),
                        dep.upstream_producer.as_deref().unwrap_or("-"),
                    );
                }
            }
            Ok(())
        }
    }
}

fn print_consume(
    receipt: &mesh_core::services::ConsumeReceipt,
    format: OutputFormat,
) -> Result<(), RegistryError> {
    match format {
        OutputFormat::Json => print_json(receipt),
        OutputFormat::Table => {
            println!("Consumed product {}", receipt.product_id);
            println!("version: {}", receipt.version);
            println!("source_reference: {}", receipt.source_reference);
            println!("output_path: {}", receipt.output_path);
            println!("retrieved_at: {}", receipt.retrieved_at);
            Ok(())
        }
    }
}

fn print_json<T: Serialize + ?Sized>(value: &T) -> Result<(), RegistryError> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}

fn truncate(value: &str, width: usize) -> String {
    if value.chars().count() <= width {
        value.to_string()
    } else {
        let truncated: String = value.chars().take(width.saturating_sub(3)).collect();
        format!("{truncated}...")
    }
}

fn map_io_error(err: std::io::Error) -> RegistryError {
    if err.kind() == std::io::ErrorKind::PermissionDenied {
        RegistryError::Permission(err.to_string())
    } else {
        RegistryError::Filesystem(err)
    }
}

fn exit_code(err: &RegistryError) -> u8 {
    match err {
        RegistryError::Validation(_) => 3,
        RegistryError::NotFound(_) | RegistryError::SourceMissing(_) => 4,
        RegistryError::Permission(_) | RegistryError::DestinationExists(_) => 5,
        RegistryError::Database(_)
        | RegistryError::Filesystem(_)
        | RegistryError::Serialization(_) => 1,
    }
}

impl From<CliAssetType> for AssetType {
    fn from(value: CliAssetType) -> Self {
        AssetType::from_str(&value.to_string()).expect("clap enum values match core asset types")
    }
}

impl From<CliDataQuality> for DataQuality {
    fn from(value: CliDataQuality) -> Self {
        DataQuality::from_str(&value.to_string()).expect("clap enum values match core qualities")
    }
}

impl From<CliClassification> for Classification {
    fn from(value: CliClassification) -> Self {
        Classification::from_str(&value.to_string())
            .expect("clap enum values match core classifications")
    }
}

impl fmt::Display for CliAssetType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::File => "file",
            Self::Directory => "directory",
            Self::Dataset => "dataset",
            Self::Table => "table",
            Self::ModelArtifact => "model_artifact",
            Self::ReportArtifact => "report_artifact",
            Self::ManifestCollection => "manifest_collection",
        })
    }
}

impl fmt::Display for CliDataQuality {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Production => "production",
            Self::Qualified => "qualified",
            Self::Unverified => "unverified",
        })
    }
}

impl fmt::Display for CliClassification {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Public => "public",
            Self::Internal => "internal",
            Self::Restricted => "restricted",
        })
    }
}
