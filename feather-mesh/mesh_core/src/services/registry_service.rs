use std::fs;
use std::path::{Path, PathBuf};

use chrono::Utc;
use rusqlite::{Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::domain::{
    AssetType, Classification, DataQuality, RegistryError, RegistryResult, SourceReference,
    validate_non_blank, validate_optional_text, validate_positive_id, validate_product_name,
    validate_version_label,
};
use crate::models::{
    DataProduct, DataProductVersion, LineageDependency, NewDataProduct, NewDataProductVersion,
    NewLineageDependency, NewTeam, Team,
};
use crate::repositories::{
    DataProductRepository, DataProductVersionRepository, LineageDependencyRepository,
    TeamRepository,
};
use crate::services::{
    CreateDataProductRequest, CreateDataProductVersionRequest, CreateLineageDependencyRequest,
};

pub struct RegistryService<'a> {
    conn: &'a Connection,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServeRequest {
    pub source_path: PathBuf,
    pub name: String,
    pub asset_type: AssetType,
    pub version: String,
    pub owner_team: String,
    pub producer: String,
    pub usage_policy: String,
    pub data_quality: DataQuality,
    pub classification: Classification,
    pub description: Option<String>,
    pub intended_use: Option<String>,
    #[serde(default)]
    pub lineage: Vec<LineageReference>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LineageReference {
    pub source: String,
    pub version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServeResponse {
    pub product_id: i64,
    pub version_id: i64,
    pub name: String,
    pub version: String,
    pub source_reference: String,
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchRequest {
    pub query: Option<String>,
    pub asset_type: Option<AssetType>,
    pub data_quality: Option<DataQuality>,
    pub classification: Option<Classification>,
    pub owner_team: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProductSummary {
    pub product_id: i64,
    pub name: String,
    pub owner_team: String,
    pub producer: String,
    pub version: Option<String>,
    pub asset_type: Option<String>,
    pub data_quality: Option<String>,
    pub classification: Option<String>,
    pub source_reference: Option<String>,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProductDetail {
    pub product_id: i64,
    pub name: String,
    pub description: Option<String>,
    pub owner_team: String,
    pub producer: String,
    pub usage_policy: String,
    pub intended_use: Option<String>,
    pub created_at: String,
    pub selected_version: VersionDetail,
    pub lineage: LineageResponse,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VersionDetail {
    pub version_id: i64,
    pub version: String,
    pub asset_type: String,
    pub source_reference: String,
    pub data_quality: String,
    pub classification: String,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LineageResponse {
    pub product_id: i64,
    pub version: String,
    pub status: String,
    pub dependencies: Vec<LineageDependencyDetail>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LineageDependencyDetail {
    pub upstream_source_reference: String,
    pub upstream_version: Option<String>,
    pub upstream_product_id: Option<i64>,
    pub upstream_name: Option<String>,
    pub upstream_producer: Option<String>,
    pub upstream_published_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConsumeRequest {
    pub product_ref: String,
    pub version: String,
    pub out: PathBuf,
    pub overwrite: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConsumeReceipt {
    pub product_id: i64,
    pub version: String,
    pub source_reference: String,
    pub output_path: String,
    pub retrieved_at: String,
    pub checksum: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InitResponse {
    pub registry_path: String,
    pub status: String,
}

impl<'a> RegistryService<'a> {
    pub fn new(conn: &'a Connection) -> Self {
        Self { conn }
    }

    pub fn register_team(&self, name: String) -> RegistryResult<Team> {
        validate_non_blank("team", &name)?;
        TeamRepository::create(self.conn, NewTeam::new(name)).map_err(RegistryError::from)
    }

    pub fn list_teams(&self) -> RegistryResult<Vec<Team>> {
        TeamRepository::get_all(self.conn).map_err(RegistryError::from)
    }

    pub fn list_products(&self) -> RegistryResult<Vec<ProductSummary>> {
        self.search_products(SearchRequest {
            query: None,
            asset_type: None,
            data_quality: None,
            classification: None,
            owner_team: None,
        })
    }

    pub fn register_data_product(
        &self,
        input: CreateDataProductRequest,
    ) -> RegistryResult<DataProduct> {
        validate_product_name(&input.name)?;
        validate_positive_id("owner_team_id", input.owner_team_id)?;
        validate_optional_text("description", input.description.as_deref())?;
        validate_optional_text("intended_use", input.intended_use.as_deref())?;
        validate_non_blank("producer", &input.producer)?;
        validate_non_blank("usage_policy", &input.usage_policy)?;
        TeamRepository::get_by_id(self.conn, input.owner_team_id).map_err(|err| {
            if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                RegistryError::NotFound(format!("owner team '{}'", input.owner_team_id))
            } else {
                RegistryError::Database(err)
            }
        })?;
        DataProductRepository::create(
            self.conn,
            NewDataProduct::new(
                input.name,
                input.description,
                input.owner_team_id,
                input.producer,
                input.usage_policy,
                input.intended_use,
            ),
        )
        .map_err(RegistryError::from)
    }

    pub fn register_data_product_version(
        &self,
        input: CreateDataProductVersionRequest,
    ) -> RegistryResult<DataProductVersion> {
        validate_positive_id("data_product_id", input.data_product_id)?;
        validate_version_label(&input.version_label)?;
        let asset_type: AssetType = input.asset_type.parse()?;
        SourceReference::unix_path(PathBuf::from(&input.source_path))?;
        let data_quality: DataQuality = input.data_quality.parse()?;
        let classification: Classification = input.classification.parse()?;
        DataProductRepository::get_by_id(self.conn, input.data_product_id).map_err(|err| {
            if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                RegistryError::NotFound(format!("product '{}'", input.data_product_id))
            } else {
                RegistryError::Database(err)
            }
        })?;
        DataProductVersionRepository::create(
            self.conn,
            NewDataProductVersion::new(
                input.data_product_id,
                input.version_label,
                asset_type.to_string(),
                input.source_path,
                data_quality.to_string(),
                classification.to_string(),
            ),
        )
        .map_err(RegistryError::from)
    }

    pub fn register_lineage_dependency(
        &self,
        input: CreateLineageDependencyRequest,
    ) -> RegistryResult<LineageDependency> {
        validate_positive_id("downstream_version_id", input.downstream_version_id)?;
        validate_non_blank("upstream_product_uri", &input.upstream_product_uri)?;
        validate_optional_text("upstream_version", input.upstream_version.as_deref())?;
        DataProductVersionRepository::get_by_id(self.conn, input.downstream_version_id).map_err(
            |err| {
                if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                    RegistryError::NotFound(format!(
                        "downstream version '{}'",
                        input.downstream_version_id
                    ))
                } else {
                    RegistryError::Database(err)
                }
            },
        )?;
        LineageDependencyRepository::create(
            self.conn,
            NewLineageDependency::new(
                input.downstream_version_id,
                input.upstream_product_uri,
                input.upstream_version,
            ),
        )
        .map_err(RegistryError::from)
    }

    pub fn validate_serve_request(&self, request: &ServeRequest) -> RegistryResult<()> {
        SourceReference::unix_path(request.source_path.clone())?;
        validate_product_name(&request.name)?;
        validate_version_label(&request.version)?;
        validate_non_blank("owner_team", &request.owner_team)?;
        validate_non_blank("producer", &request.producer)?;
        validate_non_blank("usage_policy", &request.usage_policy)?;
        validate_optional_text("description", request.description.as_deref())?;
        validate_optional_text("intended_use", request.intended_use.as_deref())?;
        for lineage in &request.lineage {
            validate_non_blank("lineage", &lineage.source)?;
            validate_optional_text("lineage.version", lineage.version.as_deref())?;
        }
        Ok(())
    }

    pub fn serve(&self, request: ServeRequest) -> RegistryResult<ServeResponse> {
        self.validate_serve_request(&request)?;
        let source = SourceReference::unix_path(request.source_path.clone())?.as_display_string();
        self.conn.execute_batch("BEGIN IMMEDIATE")?;
        let result = (|| -> RegistryResult<ServeResponse> {
            let team = self.get_or_create_team(&request.owner_team)?;
            let product = DataProductRepository::create(
                self.conn,
                NewDataProduct::new(
                    request.name,
                    request.description,
                    team.team_id,
                    request.producer,
                    request.usage_policy,
                    request.intended_use,
                ),
            )?;
            let version = DataProductVersionRepository::create(
                self.conn,
                NewDataProductVersion::new(
                    product.product_id,
                    request.version,
                    request.asset_type.to_string(),
                    source.clone(),
                    request.data_quality.to_string(),
                    request.classification.to_string(),
                ),
            )?;
            for lineage in request.lineage {
                LineageDependencyRepository::create(
                    self.conn,
                    NewLineageDependency::new(version.version_id, lineage.source, lineage.version),
                )?;
            }
            Ok(ServeResponse {
                product_id: product.product_id,
                version_id: version.version_id,
                name: product.name,
                version: version.version_label,
                source_reference: source,
                status: "published".to_string(),
            })
        })();

        match result {
            Ok(response) => {
                if let Err(err) = self.conn.execute_batch("COMMIT") {
                    let _ = self.conn.execute_batch("ROLLBACK");
                    return Err(RegistryError::Database(err));
                }
                Ok(response)
            }
            Err(err) => {
                let _ = self.conn.execute_batch("ROLLBACK");
                Err(err)
            }
        }
    }

    pub fn search_products(&self, request: SearchRequest) -> RegistryResult<Vec<ProductSummary>> {
        if let Some(owner_team) = request.owner_team.as_deref() {
            validate_non_blank("owner_team", owner_team)?;
        }

        let mut stmt = self.conn.prepare(
            "SELECT p.product_id, p.name, t.name AS owner_team, p.producer,
                    p.created_at, v.version_label, v.asset_type, v.data_quality,
                    v.classification, v.source_path
             FROM data_products p
             JOIN teams t ON t.team_id = p.owner_team_id
             LEFT JOIN data_product_versions v ON v.version_id = (
                SELECT version_id FROM data_product_versions
                WHERE data_product_id = p.product_id
                ORDER BY version_id DESC LIMIT 1
             )
             ORDER BY p.product_id",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(ProductSummary {
                product_id: row.get("product_id")?,
                name: row.get("name")?,
                owner_team: row.get("owner_team")?,
                producer: row.get("producer")?,
                created_at: row.get("created_at")?,
                version: row.get("version_label")?,
                asset_type: row.get("asset_type")?,
                data_quality: row.get("data_quality")?,
                classification: row.get("classification")?,
                source_reference: row.get("source_path")?,
            })
        })?;

        let query = request.query.map(|q| q.to_ascii_lowercase());
        let owner = request.owner_team.map(|o| o.to_ascii_lowercase());
        let asset_type = request.asset_type.map(|v| v.to_string());
        let data_quality = request.data_quality.map(|v| v.to_string());
        let classification = request.classification.map(|v| v.to_string());
        let products = rows
            .collect::<Result<Vec<_>, _>>()?
            .into_iter()
            .filter(|product| {
                query.as_ref().is_none_or(|query| {
                    product.name.to_ascii_lowercase().contains(query)
                        || product
                            .source_reference
                            .as_ref()
                            .is_some_and(|s| s.to_ascii_lowercase().contains(query))
                })
            })
            .filter(|product| {
                owner
                    .as_ref()
                    .is_none_or(|owner| product.owner_team.to_ascii_lowercase() == *owner)
            })
            .filter(|product| {
                asset_type
                    .as_ref()
                    .is_none_or(|value| product.asset_type.as_ref() == Some(value))
            })
            .filter(|product| {
                data_quality
                    .as_ref()
                    .is_none_or(|value| product.data_quality.as_ref() == Some(value))
            })
            .filter(|product| {
                classification
                    .as_ref()
                    .is_none_or(|value| product.classification.as_ref() == Some(value))
            })
            .collect();
        Ok(products)
    }

    pub fn show_product(
        &self,
        product_ref: &str,
        version_label: Option<&str>,
    ) -> RegistryResult<ProductDetail> {
        let (product, team_name) = self.resolve_product_with_team(product_ref)?;
        let version = self.resolve_version(product.product_id, version_label)?;
        let lineage = self.lineage_for_version(product.product_id, &version.version_label)?;
        Ok(ProductDetail {
            product_id: product.product_id,
            name: product.name,
            description: product.description,
            owner_team: team_name,
            producer: product.producer,
            usage_policy: product.usage_policy,
            intended_use: product.intended_use,
            created_at: product.created_at.to_string(),
            selected_version: VersionDetail {
                version_id: version.version_id,
                version: version.version_label,
                asset_type: version.asset_type,
                source_reference: version.source_path,
                data_quality: version.data_quality,
                classification: version.classification,
                created_at: version.created_at.to_string(),
            },
            lineage,
        })
    }

    pub fn lineage(
        &self,
        product_ref: &str,
        version_label: Option<&str>,
    ) -> RegistryResult<LineageResponse> {
        let (product, _) = self.resolve_product_with_team(product_ref)?;
        let version = self.resolve_version(product.product_id, version_label)?;
        self.lineage_for_version(product.product_id, &version.version_label)
    }

    pub fn consume(&self, request: ConsumeRequest) -> RegistryResult<ConsumeReceipt> {
        validate_version_label(&request.version)?;
        let detail = self.show_product(&request.product_ref, Some(&request.version))?;
        let source = PathBuf::from(&detail.selected_version.source_reference);
        if !source.exists() {
            return Err(RegistryError::SourceMissing(
                detail.selected_version.source_reference,
            ));
        }
        if request.out.exists() && !request.overwrite {
            return Err(RegistryError::DestinationExists(
                request.out.to_string_lossy().into_owned(),
            ));
        }
        self.copy_source(&source, &request.out, request.overwrite)?;
        let receipt = ConsumeReceipt {
            product_id: detail.product_id,
            version: detail.selected_version.version,
            source_reference: source.to_string_lossy().into_owned(),
            output_path: request.out.to_string_lossy().into_owned(),
            retrieved_at: Utc::now().to_rfc3339(),
            checksum: None,
        };
        let receipt_path = receipt_path(&request.out);
        let json = serde_json::to_vec_pretty(&receipt)?;
        fs::write(receipt_path, json).map_err(map_io_error)?;
        Ok(receipt)
    }

    fn get_or_create_team(&self, name: &str) -> RegistryResult<Team> {
        match TeamRepository::get_by_name(self.conn, name) {
            Ok(team) => Ok(team),
            Err(rusqlite::Error::QueryReturnedNoRows) => {
                TeamRepository::create(self.conn, NewTeam::new(name.to_string()))
                    .map_err(RegistryError::from)
            }
            Err(err) => Err(RegistryError::from(err)),
        }
    }

    fn resolve_product_with_team(
        &self,
        product_ref: &str,
    ) -> RegistryResult<(crate::models::DataProduct, String)> {
        let product_id = parse_product_ref(self.conn, product_ref)?;
        let product = DataProductRepository::get_by_id(self.conn, product_id).map_err(|err| {
            if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                RegistryError::NotFound(format!("product '{product_ref}'"))
            } else {
                RegistryError::Database(err)
            }
        })?;
        let team = TeamRepository::get_by_id(self.conn, product.owner_team_id)?;
        Ok((product, team.name))
    }

    fn resolve_version(
        &self,
        product_id: i64,
        version_label: Option<&str>,
    ) -> RegistryResult<crate::models::DataProductVersion> {
        let result = if let Some(label) = version_label {
            validate_version_label(label)?;
            DataProductVersionRepository::get_by_product_and_label(self.conn, product_id, label)
        } else {
            DataProductVersionRepository::get_latest_for_product(self.conn, product_id)
        };
        result.map_err(|err| {
            if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                RegistryError::NotFound(format!("version for product '{product_id}'"))
            } else {
                RegistryError::Database(err)
            }
        })
    }

    fn lineage_for_version(
        &self,
        product_id: i64,
        version_label: &str,
    ) -> RegistryResult<LineageResponse> {
        let version = DataProductVersionRepository::get_by_product_and_label(
            self.conn,
            product_id,
            version_label,
        )?;
        let dependencies =
            LineageDependencyRepository::get_by_downstream_version(self.conn, version.version_id)?;
        let mut details = Vec::with_capacity(dependencies.len());
        for dependency in dependencies {
            let upstream = self.upstream_context(&dependency.upstream_product_uri)?;
            details.push(LineageDependencyDetail {
                upstream_source_reference: dependency.upstream_product_uri,
                upstream_version: dependency.upstream_version,
                upstream_product_id: upstream.as_ref().map(|u| u.product_id),
                upstream_name: upstream.as_ref().map(|u| u.name.clone()),
                upstream_producer: upstream.as_ref().map(|u| u.producer.clone()),
                upstream_published_at: upstream.and_then(|u| u.published_at),
            });
        }
        let status = if details.is_empty() {
            "no_lineage".to_string()
        } else {
            "lineage_available".to_string()
        };
        Ok(LineageResponse {
            product_id,
            version: version.version_label,
            status,
            dependencies: details,
        })
    }

    fn upstream_context(&self, source_path: &str) -> RegistryResult<Option<UpstreamContext>> {
        let Some(version) =
            DataProductVersionRepository::get_by_source_path(self.conn, source_path).optional()?
        else {
            return Ok(None);
        };
        let product = DataProductRepository::get_by_id(self.conn, version.data_product_id)?;
        Ok(Some(UpstreamContext {
            product_id: product.product_id,
            name: product.name,
            producer: product.producer,
            published_at: Some(version.created_at.to_string()),
        }))
    }

    fn copy_source(&self, source: &Path, out: &Path, overwrite: bool) -> RegistryResult<()> {
        let metadata = fs::metadata(source).map_err(map_io_error)?;
        if metadata.is_file() {
            if overwrite && out.exists() {
                remove_existing(out)?;
            }
            fs::copy(source, out).map_err(map_io_error)?;
            return Ok(());
        }
        if metadata.is_dir() {
            if overwrite && out.exists() {
                remove_existing(out)?;
            }
            copy_dir_recursive(source, out)?;
            return Ok(());
        }
        Err(RegistryError::Permission(format!(
            "unsupported source type '{}'",
            source.display()
        )))
    }
}

#[derive(Debug)]
struct UpstreamContext {
    product_id: i64,
    name: String,
    producer: String,
    published_at: Option<String>,
}

fn parse_product_ref(conn: &Connection, product_ref: &str) -> RegistryResult<i64> {
    if let Ok(product_id) = product_ref.parse::<i64>() {
        validate_positive_id("product_id", product_id)?;
        return Ok(product_id);
    }
    let version =
        DataProductVersionRepository::get_by_source_path(conn, product_ref).map_err(|err| {
            if matches!(err, rusqlite::Error::QueryReturnedNoRows) {
                RegistryError::NotFound(format!("product or source path '{product_ref}'"))
            } else {
                RegistryError::Database(err)
            }
        })?;
    Ok(version.data_product_id)
}

fn copy_dir_recursive(source: &Path, out: &Path) -> RegistryResult<()> {
    fs::create_dir_all(out).map_err(map_io_error)?;
    for entry in fs::read_dir(source).map_err(map_io_error)? {
        let entry = entry.map_err(map_io_error)?;
        let target = out.join(entry.file_name());
        let metadata = entry.metadata().map_err(map_io_error)?;
        if metadata.is_dir() {
            copy_dir_recursive(&entry.path(), &target)?;
        } else if metadata.is_file() {
            fs::copy(entry.path(), target).map_err(map_io_error)?;
        } else {
            return Err(RegistryError::Filesystem(std::io::Error::new(
                std::io::ErrorKind::Unsupported,
                format!("unsupported entry type at '{}'", entry.path().display()),
            )));
        }
    }
    Ok(())
}

fn remove_existing(path: &Path) -> RegistryResult<()> {
    let metadata = fs::metadata(path).map_err(map_io_error)?;
    if metadata.is_dir() {
        fs::remove_dir_all(path).map_err(map_io_error)?;
    } else {
        fs::remove_file(path).map_err(map_io_error)?;
    }
    Ok(())
}

fn receipt_path(out: &Path) -> PathBuf {
    let file_name = out
        .file_name()
        .map(|name| format!("{}.feam-receipt.json", name.to_string_lossy()))
        .unwrap_or_else(|| "feam-receipt.json".to_string());
    out.with_file_name(file_name)
}

fn map_io_error(err: std::io::Error) -> RegistryError {
    if err.kind() == std::io::ErrorKind::PermissionDenied {
        RegistryError::Permission(err.to_string())
    } else {
        RegistryError::Filesystem(err)
    }
}
