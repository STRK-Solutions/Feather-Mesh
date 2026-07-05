use std::fmt;
use std::fs;
use std::path::PathBuf;
use std::process::ExitCode;
use std::str::FromStr;

use clap::{Parser, Subcommand, ValueEnum};
use mesh_core::domain::{AssetType, Classification, DataQuality, RegistryError};
use mesh_core::services::{
    ConsumeRequest, InitResponse, LineageReference, RegistryService, SearchRequest, ServeRequest,
};
use mesh_core::{DEFAULT_DB_FILENAME, init_db};
use serde::Serialize;

#[derive(Debug, Parser)]
#[command(name = "feam", version, about = "Feather Mesh CLI")]
struct Cli {
    #[arg(long, global = true, value_name = "PATH", default_value = DEFAULT_DB_FILENAME)]
    registry: PathBuf,
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
    Init,
    Serve {
        path: PathBuf,
        #[arg(long)]
        name: String,
        #[arg(long, value_enum)]
        asset_type: CliAssetType,
        #[arg(long)]
        version: String,
        #[arg(long)]
        owner_team: String,
        #[arg(long)]
        producer: String,
        #[arg(long)]
        usage_policy: String,
        #[arg(long, value_enum)]
        data_quality: CliDataQuality,
        #[arg(long, value_enum)]
        classification: CliClassification,
        #[arg(long)]
        description: Option<String>,
        #[arg(long)]
        intended_use: Option<String>,
        #[arg(long = "lineage")]
        lineage: Vec<String>,
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

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("{err}");
            ExitCode::from(exit_code(&err))
        }
    }
}

fn run() -> Result<(), RegistryError> {
    let cli = Cli::parse();

    if matches!(cli.command, Command::Init) {
        init_db(&cli.registry)?;
        let response = InitResponse {
            registry_path: cli.registry.to_string_lossy().into_owned(),
            status: "initialized".to_string(),
        };
        return print_init(&response, cli.format);
    }

    let conn = init_db(&cli.registry)?;
    let service = RegistryService::new(&conn);
    match cli.command {
        Command::Init => unreachable!(),
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
        } => {
            let request = ServeRequest {
                source_path: path,
                name,
                asset_type: asset_type.into(),
                version,
                owner_team,
                producer,
                usage_policy,
                data_quality: data_quality.into(),
                classification: classification.into(),
                description,
                intended_use,
                lineage: parse_lineage(lineage)?,
            };
            let response = service.serve(request)?;
            print_serve(&response, cli.format)
        }
        Command::Search {
            query,
            asset_type,
            data_quality,
            classification,
            owner_team,
        } => {
            let products = service.search_products(SearchRequest {
                query,
                asset_type: asset_type.map(Into::into),
                data_quality: data_quality.map(Into::into),
                classification: classification.map(Into::into),
                owner_team,
            })?;
            print_products(&products, cli.format)
        }
        Command::Show {
            product_id_or_source_path,
            version,
        } => {
            let detail = service.show_product(&product_id_or_source_path, version.as_deref())?;
            print_show(&detail, cli.format)
        }
        Command::Consume {
            product_id_or_source_path,
            version,
            out,
            overwrite,
        } => {
            let receipt = service.consume(ConsumeRequest {
                product_ref: product_id_or_source_path,
                version,
                out,
                overwrite,
            })?;
            print_consume(&receipt, cli.format)
        }
        Command::Lineage {
            product_id_or_source_path,
            version,
        } => {
            let lineage = service.lineage(&product_id_or_source_path, version.as_deref())?;
            print_lineage(&lineage, cli.format)
        }
        Command::ValidateMetadata { metadata_file } => {
            let request = read_metadata(metadata_file)?;
            service.validate_serve_request(&request)?;
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
        Command::Products => {
            let products = service.list_products()?;
            print_products(&products, cli.format)
        }
    }
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
