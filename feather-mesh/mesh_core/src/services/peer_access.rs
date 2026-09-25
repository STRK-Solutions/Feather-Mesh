//! Project-scoped peer publication, resolution, and staging.
//!
//! This module deliberately does not use the legacy SQLite registry as an
//! authority.  A provider's `manifest.json` is the record of publication;
//! cache files and all adapters are derived from it.

use std::collections::{BTreeMap, BTreeSet};
use std::fs::{self, File, OpenOptions};
use std::io::{BufReader, Read, Write};
use std::path::{Component, Path, PathBuf};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use chrono::{DateTime, Utc};
use parquet::file::reader::{FileReader, SerializedFileReader};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use thiserror::Error;
use tiff::decoder::Decoder;
use tiff::tags::Tag;

pub const PROJECT_SCHEMA_VERSION: u32 = 1;
pub const MANIFEST_SCHEMA_VERSION: u32 = 1;
pub const PEER_PROTOCOL_VERSION: &str = "feam.peer.v1";
/// Explicit limits for the initial manifest reader (before any allocation).
pub const MAX_MANIFEST_BYTES: u64 = 8 * 1024 * 1024;
pub const MAX_CATALOG_INPUT_BYTES: u64 = 32 * 1024 * 1024;
pub const MAX_PROJECT_BYTES: u64 = 256 * 1024;

pub(crate) fn read_bounded(path: &Path, limit: u64) -> PeerResult<Vec<u8>> {
    let mut bytes = Vec::new();
    File::open(path)?.take(limit + 1).read_to_end(&mut bytes)?;
    if bytes.len() as u64 > limit {
        return Err(validation(
            "input_size",
            format!("input exceeds {limit} bytes"),
        ));
    }
    Ok(bytes)
}

#[derive(Debug, Error)]
pub enum PeerError {
    #[error("validation failed: {field}: {message}")]
    Validation { field: String, message: String },
    #[error("not found: {0}")]
    NotFound(String),
    #[error("dataset not registered: {0}")]
    DatasetNotRegistered(String),
    #[error("peer unavailable: {0}")]
    PeerUnavailable(String),
    #[error("withdrawn version: {0}")]
    Withdrawn(String),
    #[error("permission or policy failure: {0}")]
    Policy(String),
    #[error("publication conflict: {0}")]
    Conflict(String),
    #[error("integrity verification failed: {0}")]
    Integrity(String),
    #[error("publication lock timed out: {0}")]
    LockTimeout(String),
    #[error("unsupported format: {0}")]
    UnsupportedFormat(String),
    #[error("filesystem error: {0}")]
    Io(#[from] std::io::Error),
    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),
    #[error("project configuration error: {0}")]
    TomlDeserialize(#[from] toml::de::Error),
    #[error("project configuration serialization error: {0}")]
    TomlSerialize(#[from] toml::ser::Error),
    #[error("Parquet inspection failed: {0}")]
    Parquet(String),
    #[error("GeoTIFF inspection failed: {0}")]
    GeoTiff(String),
}

pub type PeerResult<T> = Result<T, PeerError>;

impl PeerError {
    pub fn kind(&self) -> &'static str {
        match self {
            Self::Validation { .. } => "bad_metadata",
            Self::NotFound(_) => "not_found",
            Self::DatasetNotRegistered(_) => "dataset_not_registered",
            Self::PeerUnavailable(_) => "peer_unavailable",
            Self::Withdrawn(_) => "withdrawn_version",
            Self::Policy(_) => "policy_failure",
            Self::Conflict(_) => "publication_conflict",
            Self::Integrity(_) => "integrity_failed",
            Self::LockTimeout(_) => "lock_timeout",
            Self::UnsupportedFormat(_) => "unsupported_format",
            Self::Io(_) => "filesystem_error",
            Self::Json(_) => "malformed_manifest",
            Self::TomlDeserialize(_) | Self::TomlSerialize(_) => "project_config_error",
            Self::Parquet(_) | Self::GeoTiff(_) => "unsupported_format",
        }
    }

    pub fn exit_code(&self) -> u8 {
        match self {
            Self::Validation { .. }
            | Self::UnsupportedFormat(_)
            | Self::Parquet(_)
            | Self::GeoTiff(_) => 3,
            Self::NotFound(_) | Self::DatasetNotRegistered(_) => 4,
            Self::PeerUnavailable(_)
            | Self::Withdrawn(_)
            | Self::Policy(_)
            | Self::Integrity(_) => 5,
            Self::Conflict(_)
            | Self::LockTimeout(_)
            | Self::Io(_)
            | Self::Json(_)
            | Self::TomlDeserialize(_)
            | Self::TomlSerialize(_) => 1,
        }
    }
}

fn validation(field: impl Into<String>, message: impl Into<String>) -> PeerError {
    PeerError::Validation {
        field: field.into(),
        message: message.into(),
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DataKind {
    Table,
    Raster,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DataFormat {
    Parquet,
    Geotiff,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Lifecycle {
    Active,
    Withdrawn,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PeerRouteConfig {
    pub alias: String,
    pub namespace: String,
    pub path: PathBuf,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ProjectConfig {
    pub schema_version: u32,
    pub namespace: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub serving_dir: Option<PathBuf>,
    #[serde(default)]
    pub owner_teams: Vec<String>,
    #[serde(default)]
    pub peers: Vec<PeerRouteConfig>,
}

#[derive(Debug, Clone)]
pub struct Project {
    root: PathBuf,
    config: ProjectConfig,
}

impl Project {
    pub fn init(
        root: impl AsRef<Path>,
        namespace: String,
        serving_dir: Option<PathBuf>,
    ) -> PeerResult<Self> {
        Self::init_with_owner_teams(root, namespace, serving_dir, Vec::new())
    }

    pub fn init_with_owner_teams(
        root: impl AsRef<Path>,
        namespace: String,
        serving_dir: Option<PathBuf>,
        mut owner_teams: Vec<String>,
    ) -> PeerResult<Self> {
        validate_namespace(&namespace)?;
        let root = absolute_path(root.as_ref())?;
        fs::create_dir_all(root.join(".feam"))?;
        let config_path = root.join(".feam/project.toml");
        if config_path.exists() {
            return Err(PeerError::Conflict(format!(
                "project already initialized at '{}'",
                root.display()
            )));
        }
        if let Some(path) = serving_dir.as_deref() {
            validate_relative_path(path, "serving_dir")?;
            fs::create_dir_all(root.join(path))?;
        }
        if owner_teams.is_empty() {
            owner_teams.push(namespace.clone());
        }
        for team in &owner_teams {
            non_blank("owner_teams", team)?;
        }
        owner_teams.sort();
        owner_teams.dedup();
        let config = ProjectConfig {
            schema_version: PROJECT_SCHEMA_VERSION,
            namespace,
            serving_dir,
            owner_teams,
            peers: Vec::new(),
        };
        write_atomic_text(&config_path, toml::to_string_pretty(&config)?.as_bytes())?;
        Ok(Self { root, config })
    }

    pub fn open(root: impl AsRef<Path>) -> PeerResult<Self> {
        let root = absolute_path(root.as_ref())?;
        let bytes = read_bounded(&root.join(".feam/project.toml"), MAX_PROJECT_BYTES)?;
        let text =
            std::str::from_utf8(&bytes).map_err(|_| validation("project", "invalid UTF-8"))?;
        let mut config: ProjectConfig = toml::from_str(text)?;
        if config.peers.len() > 64 {
            return Err(validation("peers", "at most 64 peer routes are supported"));
        }
        if config.owner_teams.is_empty() {
            // Schema-v1 projects created before owner-team authorization use
            // their namespace as the single explicit provider authority.
            config.owner_teams.push(config.namespace.clone());
        }
        validate_project_config(&config)?;
        Ok(Self { root, config })
    }

    pub fn root(&self) -> &Path {
        &self.root
    }
    pub fn config(&self) -> &ProjectConfig {
        &self.config
    }
    pub fn namespace(&self) -> &str {
        &self.config.namespace
    }

    pub fn config_path(&self) -> PathBuf {
        self.root.join(".feam/project.toml")
    }

    pub fn serving_root(&self) -> PeerResult<PathBuf> {
        let Some(path) = self.config.serving_dir.as_deref() else {
            return Err(PeerError::Policy(
                "project is not configured as a provider (serving_dir is absent)".into(),
            ));
        };
        let root = self.root.join(path);
        if !root.is_dir() {
            return Err(PeerError::PeerUnavailable(format!(
                "serving root '{}'",
                root.display()
            )));
        }
        fs::canonicalize(root).map_err(PeerError::from)
    }

    pub fn cache_path(&self) -> PathBuf {
        let digest = hex_digest(self.root.to_string_lossy().as_bytes());
        let base = std::env::var_os("FEAM_CACHE_DIR")
            .map(PathBuf::from)
            .unwrap_or_else(|| {
                std::env::temp_dir().join(format!("feam-cache-{}", std::process::id()))
            });
        base.join(&digest[..16]).join("catalog.json")
    }

    fn route_for_namespace(&self, namespace: &str) -> PeerResult<Route> {
        if namespace == self.namespace() {
            let root = self.serving_root()?;
            return Ok(Route {
                displayed_root: root.clone(),
                canonical_root: root,
                namespace: namespace.to_string(),
            });
        }
        let peer = self
            .config
            .peers
            .iter()
            .find(|peer| peer.namespace == namespace)
            .ok_or_else(|| {
                PeerError::PeerUnavailable(format!("namespace '{namespace}' is not configured"))
            })?;
        let configured_path = if peer.path.is_absolute() {
            peer.path.clone()
        } else {
            self.root.join(&peer.path)
        };
        let canonical_root = fs::canonicalize(&configured_path).map_err(|_| {
            PeerError::PeerUnavailable(format!(
                "peer '{}' route '{}'",
                peer.alias,
                configured_path.display()
            ))
        })?;
        if !canonical_root.is_dir() {
            return Err(PeerError::PeerUnavailable(format!(
                "peer '{}' route is not a directory",
                peer.alias
            )));
        }
        Ok(Route {
            displayed_root: configured_path,
            canonical_root,
            namespace: namespace.to_string(),
        })
    }
}

#[derive(Debug, Clone)]
struct Route {
    displayed_root: PathBuf,
    canonical_root: PathBuf,
    namespace: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct LineageReference {
    pub product: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DeclaredAsset {
    pub id: String,
    pub path: PathBuf,
    pub role: String,
    pub media_type: String,
    #[serde(default)]
    pub digest_opt_out: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(deny_unknown_fields)]
pub struct TablePublication {
    #[serde(default)]
    pub column_meanings: BTreeMap<String, String>,
    #[serde(default)]
    pub column_units: BTreeMap<String, String>,
    #[serde(default)]
    pub partition_columns: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RasterPublication {
    #[serde(default)]
    pub datetime: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub start_datetime: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub end_datetime: Option<String>,
    pub bbox: [f64; 4],
    #[serde(default)]
    pub semantics: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PublicationRequest {
    #[serde(default = "one")]
    pub schema_version: u32,
    pub namespace: String,
    pub product_id: String,
    pub name: String,
    pub version: String,
    pub data_kind: DataKind,
    pub data_format: DataFormat,
    pub description: String,
    pub intended_use: String,
    pub limitations: String,
    pub owner_team: String,
    pub producer: String,
    pub contact: String,
    pub usage_policy: String,
    pub classification: String,
    pub quality: String,
    pub assets: Vec<DeclaredAsset>,
    #[serde(default)]
    pub lineage: Vec<LineageReference>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub table: Option<TablePublication>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub raster: Option<RasterPublication>,
}

fn one() -> u32 {
    1
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AssetRecord {
    pub id: String,
    pub path: PathBuf,
    pub role: String,
    pub media_type: String,
    pub size: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sha256: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TableDescriptor {
    pub columns: BTreeMap<String, String>,
    pub schema_fingerprint: String,
    pub row_count: i64,
    pub partition_columns: Vec<String>,
    pub column_meanings: BTreeMap<String, String>,
    pub column_units: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RasterDescriptor {
    pub width: u32,
    pub height: u32,
    pub color_type: String,
    pub geo_key_directory: Vec<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub crs: Option<String>,
    pub nodata: Option<String>,
    #[serde(default)]
    pub datetime: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub start_datetime: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub end_datetime: Option<String>,
    pub bbox: [f64; 4],
    pub semantics: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PublishedVersion {
    pub product_id: String,
    pub name: String,
    pub version: String,
    pub lifecycle: Lifecycle,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub withdrawal_reason: Option<String>,
    pub data_kind: DataKind,
    pub data_format: DataFormat,
    pub description: String,
    pub intended_use: String,
    pub limitations: String,
    pub owner_team: String,
    pub producer: String,
    pub contact: String,
    pub usage_policy: String,
    pub classification: String,
    pub quality: String,
    pub assets: Vec<AssetRecord>,
    pub lineage: Vec<LineageReference>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub table: Option<TableDescriptor>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub raster: Option<RasterDescriptor>,
    pub published_at: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub withdrawn_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ManifestProduct {
    pub id: String,
    pub name: String,
    pub versions: Vec<PublishedVersion>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ServingManifest {
    pub schema_version: u32,
    pub namespace: String,
    pub revision: u64,
    pub updated_at: String,
    #[serde(default)]
    pub products: Vec<ManifestProduct>,
}

impl ServingManifest {
    fn empty(namespace: String) -> Self {
        Self {
            schema_version: MANIFEST_SCHEMA_VERSION,
            namespace,
            revision: 0,
            updated_at: Utc::now().to_rfc3339(),
            products: Vec::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PublicationResponse {
    pub protocol: String,
    pub reference: String,
    pub version: String,
    pub manifest_revision: u64,
    pub status: String,
}

pub fn read_publication_request(path: impl AsRef<Path>) -> PeerResult<PublicationRequest> {
    Ok(serde_json::from_slice(&read_bounded(
        path.as_ref(),
        MAX_MANIFEST_BYTES,
    )?)?)
}

pub fn validate_publication(
    project: &Project,
    request: &PublicationRequest,
) -> PeerResult<PublishedVersion> {
    validate_publication_fields(project, request)?;
    let serving_root = project.serving_root()?;
    let mut ids = BTreeSet::new();
    let mut paths = BTreeSet::new();
    let mut assets = Vec::with_capacity(request.assets.len());
    let mut parquet_descriptors = Vec::new();
    let mut raster_descriptors = Vec::new();
    for asset in &request.assets {
        non_blank("assets.id", &asset.id)?;
        non_blank("assets.role", &asset.role)?;
        non_blank("assets.media_type", &asset.media_type)?;
        if !ids.insert(asset.id.clone()) {
            return Err(validation(
                "assets.id",
                format!("duplicate asset id '{}'", asset.id),
            ));
        }
        validate_relative_path(&asset.path, "assets.path")?;
        if !paths.insert(asset.path.clone()) {
            return Err(validation(
                "assets.path",
                format!("duplicate asset path '{}'", asset.path.display()),
            ));
        }
        let (access, canonical) = checked_asset_path(&serving_root, &serving_root, &asset.path)?;
        let metadata = fs::metadata(&canonical)?;
        if !metadata.is_file() {
            return Err(PeerError::Policy(format!(
                "asset '{}' is not a regular file",
                asset.path.display()
            )));
        }
        match (request.data_kind, request.data_format) {
            (DataKind::Table, DataFormat::Parquet) => {
                parquet_descriptors.push(inspect_parquet(&canonical)?)
            }
            (DataKind::Raster, DataFormat::Geotiff) => raster_descriptors.push(inspect_geotiff(
                &canonical,
                request.raster.as_ref().expect("validated raster profile"),
            )?),
            _ => {
                return Err(PeerError::UnsupportedFormat(
                    "data_kind/data_format must be table/parquet or raster/geotiff".into(),
                ));
            }
        }
        assets.push(AssetRecord {
            id: asset.id.clone(),
            path: asset.path.clone(),
            role: asset.role.clone(),
            media_type: asset.media_type.clone(),
            size: metadata.len(),
            sha256: (!asset.digest_opt_out)
                .then(|| sha256_file(&canonical))
                .transpose()?,
        });
        // Keep `access` in scope: construction above establishes the path through the serving route.
        let _ = access;
    }
    let (table, raster) = match request.data_kind {
        DataKind::Table => (
            Some(merge_table_descriptors(
                parquet_descriptors,
                request.table.as_ref().expect("validated table profile"),
            )?),
            None,
        ),
        DataKind::Raster => (None, Some(merge_raster_descriptors(raster_descriptors)?)),
    };
    Ok(PublishedVersion {
        product_id: request.product_id.clone(),
        name: request.name.clone(),
        version: request.version.clone(),
        lifecycle: Lifecycle::Active,
        withdrawal_reason: None,
        data_kind: request.data_kind,
        data_format: request.data_format,
        description: request.description.clone(),
        intended_use: request.intended_use.clone(),
        limitations: request.limitations.clone(),
        owner_team: request.owner_team.clone(),
        producer: request.producer.clone(),
        contact: request.contact.clone(),
        usage_policy: request.usage_policy.clone(),
        classification: request.classification.clone(),
        quality: request.quality.clone(),
        assets,
        lineage: request.lineage.clone(),
        table,
        raster,
        published_at: Utc::now().to_rfc3339(),
        withdrawn_at: None,
    })
}

pub fn publish(project: &Project, request: &PublicationRequest) -> PeerResult<PublicationResponse> {
    publish_with_precondition(project, request, None)
}

/// Publishes only when the provider manifest is still at the revision reviewed
/// by an interactive caller.  The revision comparison deliberately happens
/// while holding the existing manifest lock; a UI-only comparison would race a
/// concurrent publisher.
pub fn publish_with_precondition(
    project: &Project,
    request: &PublicationRequest,
    expected_manifest_revision: Option<u64>,
) -> PeerResult<PublicationResponse> {
    publish_reviewed(project, request, expected_manifest_revision, None)
}

pub(crate) fn publication_fingerprint(candidate: &PublishedVersion) -> PeerResult<String> {
    let mut candidate = candidate.clone();
    candidate.published_at.clear();
    Ok(format!(
        "{:x}",
        Sha256::digest(serde_json::to_vec(&candidate)?)
    ))
}

pub(crate) fn publish_reviewed(
    project: &Project,
    request: &PublicationRequest,
    expected_manifest_revision: Option<u64>,
    expected_candidate: Option<&str>,
) -> PeerResult<PublicationResponse> {
    let candidate = validate_publication(project, request)?;
    let serving_root = project.serving_root()?;
    let manifest_path = serving_root.join("manifest.json");
    let _lock = ManifestLock::acquire(&manifest_path, Duration::from_secs(30))?;
    if let Some(expected) = expected_candidate
        && publication_fingerprint(&candidate)? != expected
    {
        return Err(PeerError::Conflict(
            "validated publication inventory changed; review again".into(),
        ));
    }
    let mut manifest = read_or_empty_manifest(&manifest_path, project.namespace())?;
    if manifest.namespace != project.namespace() {
        return Err(PeerError::Policy(format!(
            "manifest namespace '{}' does not match project namespace '{}'",
            manifest.namespace,
            project.namespace()
        )));
    }
    if expected_manifest_revision.is_some_and(|expected| expected != manifest.revision) {
        return Err(PeerError::Conflict(format!(
            "manifest revision changed from {} to {} while publication awaited confirmation",
            expected_manifest_revision.expect("checked above"),
            manifest.revision
        )));
    }
    recheck_candidate(&serving_root, &candidate)?;
    if let Some(product) = manifest
        .products
        .iter_mut()
        .find(|product| product.id == candidate.product_id)
    {
        if product
            .versions
            .iter()
            .any(|version| version.version == candidate.version)
        {
            return Err(PeerError::Conflict(format!(
                "{} version {}",
                candidate.product_id, candidate.version
            )));
        }
        product.versions.push(candidate.clone());
    } else {
        manifest.products.push(ManifestProduct {
            id: candidate.product_id.clone(),
            name: candidate.name.clone(),
            versions: vec![candidate.clone()],
        });
    }
    manifest.revision += 1;
    manifest.updated_at = Utc::now().to_rfc3339();
    write_manifest_atomic(&manifest_path, &manifest)?;
    Ok(PublicationResponse {
        protocol: PEER_PROTOCOL_VERSION.into(),
        reference: reference(&manifest.namespace, &candidate.product_id),
        version: candidate.version,
        manifest_revision: manifest.revision,
        status: "published".into(),
    })
}

pub fn withdraw(
    project: &Project,
    product_id: &str,
    version: &str,
    reason: &str,
) -> PeerResult<PublicationResponse> {
    withdraw_with_precondition(project, product_id, version, reason, None)
}

/// Withdraws a local product only after checking an optional reviewed manifest
/// revision while holding the provider mutation lock.
pub fn withdraw_with_precondition(
    project: &Project,
    product_id: &str,
    version: &str,
    reason: &str,
    expected_manifest_revision: Option<u64>,
) -> PeerResult<PublicationResponse> {
    validate_product_id(product_id)?;
    validate_version(version)?;
    non_blank("reason", reason)?;
    let serving_root = project.serving_root()?;
    let manifest_path = serving_root.join("manifest.json");
    let _lock = ManifestLock::acquire(&manifest_path, Duration::from_secs(30))?;
    let mut manifest = read_manifest_at(&manifest_path, project.namespace())?;
    if expected_manifest_revision.is_some_and(|expected| expected != manifest.revision) {
        return Err(PeerError::Conflict(format!(
            "manifest revision changed from {} to {} while withdrawal awaited confirmation",
            expected_manifest_revision.expect("checked above"),
            manifest.revision
        )));
    }
    let record = manifest
        .products
        .iter_mut()
        .find(|product| product.id == product_id)
        .and_then(|product| {
            product
                .versions
                .iter_mut()
                .find(|record| record.version == version)
        })
        .ok_or_else(|| PeerError::NotFound(format!("{} version {}", product_id, version)))?;
    if record.lifecycle == Lifecycle::Withdrawn {
        return Err(PeerError::Conflict(format!(
            "{} version {} is already withdrawn",
            product_id, version
        )));
    }
    record.lifecycle = Lifecycle::Withdrawn;
    record.withdrawal_reason = Some(reason.to_string());
    record.withdrawn_at = Some(Utc::now().to_rfc3339());
    manifest.revision += 1;
    manifest.updated_at = Utc::now().to_rfc3339();
    write_manifest_atomic(&manifest_path, &manifest)?;
    Ok(PublicationResponse {
        protocol: PEER_PROTOCOL_VERSION.into(),
        reference: reference(&manifest.namespace, product_id),
        version: version.into(),
        manifest_revision: manifest.revision,
        status: "withdrawn".into(),
    })
}

/// Parses a qualified reference and rejects a withdrawal through a consumer
/// alias before any mutation lock or manifest write is attempted.
pub fn withdraw_qualified(
    project: &Project,
    qualified_reference: &str,
    version: &str,
    reason: &str,
    expected_manifest_revision: Option<u64>,
) -> PeerResult<PublicationResponse> {
    let (namespace, product_id) = parse_reference(qualified_reference)?;
    if namespace != project.namespace() {
        return Err(PeerError::Policy(format!(
            "only local namespace '{}' may withdraw; '{}' belongs to '{}',",
            project.namespace(),
            product_id,
            namespace
        )));
    }
    withdraw_with_precondition(
        project,
        product_id,
        version,
        reason,
        expected_manifest_revision,
    )
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResolvedAsset {
    pub id: String,
    pub project_access_path: PathBuf,
    pub media_type: String,
    pub role: String,
    pub size: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sha256: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResolvedProduct {
    pub protocol: String,
    pub namespace: String,
    pub product_id: String,
    pub version: String,
    pub manifest_revision: u64,
    pub data_kind: DataKind,
    pub data_format: DataFormat,
    pub assets: Vec<ResolvedAsset>,
    pub description: String,
    pub intended_use: String,
    pub limitations: String,
    pub owner_team: String,
    pub producer: String,
    pub contact: String,
    pub usage_policy: String,
    pub classification: String,
    pub quality: String,
    pub lineage: Vec<LineageReference>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub table: Option<TableDescriptor>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub raster: Option<RasterDescriptor>,
    pub cache_refreshed_at: Option<String>,
    pub freshness: String,
    pub integrity_verified: bool,
}

pub fn resolve(
    project: &Project,
    qualified_reference: &str,
    version: &str,
    asset_id: Option<&str>,
    verify_integrity: bool,
) -> PeerResult<ResolvedProduct> {
    let (namespace, product_id) = parse_reference(qualified_reference)?;
    validate_version(version)?;
    let route = project.route_for_namespace(&namespace)?;
    let manifest_path = route.displayed_root.join("manifest.json");
    let manifest = read_manifest_at(&manifest_path, &namespace).map_err(|err| match err {
        PeerError::Io(io) if io.kind() == std::io::ErrorKind::NotFound => {
            PeerError::PeerUnavailable(format!(
                "manifest for namespace '{namespace}' is unavailable"
            ))
        }
        PeerError::NotFound(_) => PeerError::PeerUnavailable(format!(
            "manifest for namespace '{namespace}' is unavailable"
        )),
        other => other,
    })?;
    if manifest.namespace != route.namespace {
        return Err(PeerError::Policy(format!(
            "configured namespace '{}' does not match manifest namespace '{}'",
            route.namespace, manifest.namespace
        )));
    }
    let record = manifest
        .products
        .iter()
        .find(|product| product.id == product_id)
        .and_then(|product| {
            product
                .versions
                .iter()
                .find(|candidate| candidate.version == version)
        })
        .ok_or_else(|| {
            PeerError::NotFound(format!("{} version {}", qualified_reference, version))
        })?;
    if record.lifecycle == Lifecycle::Withdrawn {
        return Err(PeerError::Withdrawn(format!(
            "{} version {}",
            qualified_reference, version
        )));
    }
    let selected: Vec<&AssetRecord> = if let Some(id) = asset_id {
        vec![
            record
                .assets
                .iter()
                .find(|asset| asset.id == id)
                .ok_or_else(|| {
                    PeerError::DatasetNotRegistered(format!(
                        "asset '{id}' in {qualified_reference} version {version}"
                    ))
                })?,
        ]
    } else {
        record.assets.iter().collect()
    };
    let mut assets = Vec::with_capacity(selected.len());
    for asset in selected {
        let (project_access_path, canonical) =
            checked_asset_path(&route.displayed_root, &route.canonical_root, &asset.path)?;
        let metadata = fs::metadata(&canonical).map_err(|_| {
            PeerError::PeerUnavailable(format!(
                "registered asset '{}' is unavailable",
                project_access_path.display()
            ))
        })?;
        if metadata.len() != asset.size {
            return Err(PeerError::Integrity(format!(
                "asset '{}' size changed",
                asset.id
            )));
        }
        if verify_integrity
            && let Some(expected) = &asset.sha256
            && sha256_file(&canonical)? != *expected
        {
            return Err(PeerError::Integrity(format!(
                "asset '{}' digest changed",
                asset.id
            )));
        }
        assets.push(ResolvedAsset {
            id: asset.id.clone(),
            project_access_path,
            media_type: asset.media_type.clone(),
            role: asset.role.clone(),
            size: asset.size,
            sha256: asset.sha256.clone(),
        });
    }
    let cache_refreshed_at = read_cache(project)
        .ok()
        .and_then(|cache| cache.refreshed_at_for(&namespace));
    Ok(ResolvedProduct {
        protocol: PEER_PROTOCOL_VERSION.into(),
        namespace,
        product_id: product_id.to_string(),
        version: version.to_string(),
        manifest_revision: manifest.revision,
        data_kind: record.data_kind,
        data_format: record.data_format,
        assets,
        description: record.description.clone(),
        intended_use: record.intended_use.clone(),
        limitations: record.limitations.clone(),
        owner_team: record.owner_team.clone(),
        producer: record.producer.clone(),
        contact: record.contact.clone(),
        usage_policy: record.usage_policy.clone(),
        classification: record.classification.clone(),
        quality: record.quality.clone(),
        lineage: record.lineage.clone(),
        table: record.table.clone(),
        raster: record.raster.clone(),
        cache_refreshed_at,
        freshness: "current".into(),
        integrity_verified: verify_integrity,
    })
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CachePeerStatus {
    pub namespace: String,
    pub revision: Option<u64>,
    pub refreshed_at: String,
    pub error: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheSnapshot {
    pub schema_version: u32,
    pub project_namespace: String,
    pub peers: Vec<CachePeerStatus>,
}

impl CacheSnapshot {
    fn refreshed_at_for(&self, namespace: &str) -> Option<String> {
        self.peers
            .iter()
            .find(|peer| peer.namespace == namespace && peer.error.is_none())
            .map(|peer| peer.refreshed_at.clone())
    }
}

pub fn refresh(project: &Project) -> PeerResult<CacheSnapshot> {
    let mut routes = Vec::new();
    if project.config.serving_dir.is_some() {
        routes.push(project.route_for_namespace(project.namespace())?);
    }
    for peer in &project.config.peers {
        match project.route_for_namespace(&peer.namespace) {
            Ok(route) => routes.push(route),
            Err(_) => routes.push(Route {
                displayed_root: PathBuf::new(),
                canonical_root: PathBuf::new(),
                namespace: peer.namespace.clone(),
            }),
        }
    }
    let peers = routes
        .into_iter()
        .map(|route| {
            let now = Utc::now().to_rfc3339();
            if route.displayed_root.as_os_str().is_empty() {
                return CachePeerStatus {
                    namespace: route.namespace,
                    revision: None,
                    refreshed_at: now,
                    error: Some("peer route unavailable".into()),
                };
            }
            match read_manifest_at(
                &route.displayed_root.join("manifest.json"),
                &route.namespace,
            ) {
                Ok(manifest) if manifest.namespace == route.namespace => CachePeerStatus {
                    namespace: route.namespace,
                    revision: Some(manifest.revision),
                    refreshed_at: now,
                    error: None,
                },
                Ok(manifest) => CachePeerStatus {
                    namespace: route.namespace,
                    revision: None,
                    refreshed_at: now,
                    error: Some(format!(
                        "manifest namespace '{}' does not match configured route",
                        manifest.namespace
                    )),
                },
                Err(err) => CachePeerStatus {
                    namespace: route.namespace,
                    revision: None,
                    refreshed_at: now,
                    error: Some(err.to_string()),
                },
            }
        })
        .collect();
    let snapshot = CacheSnapshot {
        schema_version: PROJECT_SCHEMA_VERSION,
        project_namespace: project.namespace().to_string(),
        peers,
    };
    let cache_path = project.cache_path();
    if let Some(parent) = cache_path.parent() {
        fs::create_dir_all(parent)?;
    }
    write_atomic_text(&cache_path, &serde_json::to_vec_pretty(&snapshot)?)?;
    Ok(snapshot)
}

pub fn cache_status(project: &Project) -> PeerResult<Option<CacheSnapshot>> {
    read_cache(project).map(Some).or_else(|err| {
        if matches!(err, PeerError::Io(ref io) if io.kind() == std::io::ErrorKind::NotFound) {
            Ok(None)
        } else {
            Err(err)
        }
    })
}
fn read_cache(project: &Project) -> PeerResult<CacheSnapshot> {
    Ok(serde_json::from_slice(&fs::read(project.cache_path())?)?)
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PeerCoverage {
    pub namespace: String,
    pub revision: Option<u64>,
    pub observed_at: String,
    pub available: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegisteredListing {
    pub entries: Vec<(String, ManifestProduct, u64)>,
    pub coverage: Vec<PeerCoverage>,
}

/// Reads each currently configured manifest once and reports failed routes in
/// band with successful records.  Callers must not mistake an empty result for
/// complete coverage.
pub fn list_registered_with_coverage(project: &Project) -> PeerResult<RegisteredListing> {
    let mut entries = Vec::new();
    let mut coverage = Vec::new();
    let mut namespaces = Vec::new();
    let mut input_bytes = 0u64;
    if project.config.serving_dir.is_some() {
        namespaces.push(project.namespace().to_string());
    }
    namespaces.extend(
        project
            .config
            .peers
            .iter()
            .map(|peer| peer.namespace.clone()),
    );
    for namespace in namespaces {
        let observed_at = Utc::now().to_rfc3339();
        let route = match project.route_for_namespace(&namespace) {
            Ok(route) => route,
            Err(error) => {
                coverage.push(PeerCoverage {
                    namespace,
                    revision: None,
                    observed_at,
                    available: false,
                    error: Some(error.to_string()),
                });
                continue;
            }
        };
        let manifest_path = route.displayed_root.join("manifest.json");
        let remaining = MAX_CATALOG_INPUT_BYTES.saturating_sub(input_bytes);
        if fs::metadata(&manifest_path).is_ok_and(|m| m.len() > remaining) {
            return Err(validation(
                "catalog_input",
                "configured manifests exceed 32 MiB; narrow the project routes",
            ));
        }
        let loaded =
            read_bounded(&manifest_path, remaining.min(MAX_MANIFEST_BYTES)).and_then(|bytes| {
                input_bytes += bytes.len() as u64;
                parse_manifest(&bytes)
            });
        let manifest = match loaded {
            Ok(manifest) if manifest.namespace == namespace => manifest,
            Ok(manifest) => {
                coverage.push(PeerCoverage {
                    namespace,
                    revision: Some(manifest.revision),
                    observed_at,
                    available: false,
                    error: Some(format!(
                        "configured namespace does not match manifest namespace '{}'",
                        manifest.namespace
                    )),
                });
                continue;
            }
            Err(error) => {
                coverage.push(PeerCoverage {
                    namespace,
                    revision: None,
                    observed_at,
                    available: false,
                    error: Some(error.to_string()),
                });
                continue;
            }
        };
        coverage.push(PeerCoverage {
            namespace: namespace.clone(),
            revision: Some(manifest.revision),
            observed_at,
            available: true,
            error: None,
        });
        for product in manifest.products {
            entries.push((namespace.clone(), product, manifest.revision));
        }
    }
    Ok(RegisteredListing { entries, coverage })
}

pub fn list_registered(project: &Project) -> PeerResult<Vec<(String, ManifestProduct, u64)>> {
    Ok(list_registered_with_coverage(project)?.entries)
}

/// Returns the revision that an interactive provider operation can bind to a
/// review.  A missing manifest has revision zero, matching publication's first
/// commit behavior.
pub fn local_manifest_revision(project: &Project) -> PeerResult<u64> {
    let manifest_path = project.serving_root()?.join("manifest.json");
    if !manifest_path.exists() {
        return Ok(0);
    }
    Ok(read_manifest_at(&manifest_path, project.namespace())?.revision)
}

/// A project configuration fingerprint is intentionally based on the actual
/// strict configuration file, not a model or UI-supplied root string.
pub fn configuration_fingerprint(project: &Project) -> PeerResult<String> {
    let mut hash = Sha256::new();
    hash.update(read_bounded(&project.config_path(), MAX_PROJECT_BYTES)?);
    for path in project
        .config
        .peers
        .iter()
        .map(|p| project.root.join(&p.path))
        .chain(
            project
                .config
                .serving_dir
                .iter()
                .map(|p| project.root.join(p)),
        )
    {
        hash.update(format!("{:?}", fs::canonicalize(path)).as_bytes());
    }
    Ok(format!("{:x}", hash.finalize()))
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StageReceipt {
    pub protocol: String,
    pub reference: String,
    pub version: String,
    pub manifest_revision: u64,
    pub assets: Vec<ResolvedAsset>,
    pub output_path: PathBuf,
    pub retrieved_at: String,
}

pub fn stage(
    resolved: &ResolvedProduct,
    out: impl AsRef<Path>,
    overwrite: bool,
) -> PeerResult<StageReceipt> {
    let out = absolute_path(out.as_ref())?;
    if resolved.assets.is_empty() {
        return Err(PeerError::DatasetNotRegistered(
            "resolved product has no assets".into(),
        ));
    }
    let parent = out
        .parent()
        .ok_or_else(|| PeerError::Policy("output has no parent directory".into()))?;
    if !parent.is_dir() {
        return Err(PeerError::Policy(format!(
            "output parent '{}' does not exist",
            parent.display()
        )));
    }
    preflight_stage(resolved, &out, overwrite)?;
    let nonce = unique_nonce();
    let temporary = parent.join(format!(".feam-stage-{nonce}"));
    let temporary_receipt = parent.join(format!(".feam-stage-{nonce}.receipt"));
    let receipt_path = receipt_path(&out);
    let result = (|| -> PeerResult<StageReceipt> {
        if resolved.assets.len() == 1 {
            fs::copy(&resolved.assets[0].project_access_path, &temporary)?;
        } else {
            fs::create_dir(&temporary)?;
            for asset in &resolved.assets {
                let destination = temporary.join(&asset.id);
                fs::copy(&asset.project_access_path, destination)?;
            }
        }
        let receipt = StageReceipt {
            protocol: PEER_PROTOCOL_VERSION.into(),
            reference: reference(&resolved.namespace, &resolved.product_id),
            version: resolved.version.clone(),
            manifest_revision: resolved.manifest_revision,
            assets: resolved.assets.clone(),
            output_path: out.clone(),
            retrieved_at: Utc::now().to_rfc3339(),
        };
        write_atomic_text(&temporary_receipt, &serde_json::to_vec_pretty(&receipt)?)?;
        let backup = parent.join(format!(".feam-stage-backup-{nonce}"));
        let had_output = out.exists();
        if had_output {
            fs::rename(&out, &backup)?;
        }
        if let Err(err) = fs::rename(&temporary, &out) {
            if had_output {
                let _ = fs::rename(&backup, &out);
            }
            return Err(PeerError::Io(err));
        }
        if let Err(err) = fs::rename(&temporary_receipt, &receipt_path) {
            let _ = remove_any(&out);
            if had_output {
                let _ = fs::rename(&backup, &out);
            }
            return Err(PeerError::Io(err));
        }
        if had_output {
            remove_any(&backup)?;
        }
        Ok(receipt)
    })();
    if result.is_err() {
        let _ = remove_any(&temporary);
        let _ = fs::remove_file(&temporary_receipt);
    }
    result
}

fn validate_publication_fields(project: &Project, request: &PublicationRequest) -> PeerResult<()> {
    if request.schema_version != 1 {
        return Err(validation("schema_version", "only version 1 is supported"));
    }
    if request.namespace != project.namespace() {
        return Err(PeerError::Policy(format!(
            "metadata namespace '{}' does not match provider namespace '{}'",
            request.namespace,
            project.namespace()
        )));
    }
    if !project
        .config
        .owner_teams
        .iter()
        .any(|team| team == &request.owner_team)
    {
        return Err(PeerError::Policy(format!(
            "owner team '{}' is not authorized by this provider project",
            request.owner_team
        )));
    }
    validate_namespace(&request.namespace)?;
    validate_product_id(&request.product_id)?;
    validate_version(&request.version)?;
    for (field, value) in [
        ("name", &request.name),
        ("description", &request.description),
        ("intended_use", &request.intended_use),
        ("limitations", &request.limitations),
        ("owner_team", &request.owner_team),
        ("producer", &request.producer),
        ("contact", &request.contact),
        ("usage_policy", &request.usage_policy),
        ("classification", &request.classification),
        ("quality", &request.quality),
    ] {
        non_blank(field, value)?;
    }
    if request.assets.is_empty() {
        return Err(validation("assets", "must declare at least one asset"));
    }
    match (
        request.data_kind,
        request.data_format,
        request.table.as_ref(),
        request.raster.as_ref(),
    ) {
        (DataKind::Table, DataFormat::Parquet, Some(_), None) => {}
        (DataKind::Raster, DataFormat::Geotiff, None, Some(raster)) => {
            raster_time_bounds(
                raster.datetime.as_deref(),
                raster.start_datetime.as_deref(),
                raster.end_datetime.as_deref(),
            )?;
            if raster.bbox[0] > raster.bbox[2] || raster.bbox[1] > raster.bbox[3] {
                return Err(validation(
                    "raster.bbox",
                    "must be [west, south, east, north]",
                ));
            }
        }
        _ => {
            return Err(validation(
                "data_kind/data_format",
                "table requires parquet/table metadata and raster requires geotiff/raster metadata",
            ));
        }
    }
    for lineage in &request.lineage {
        non_blank("lineage.product", &lineage.product)?;
        if let Some(version) = &lineage.version {
            validate_version(version)?;
        }
    }
    Ok(())
}

fn inspect_parquet(path: &Path) -> PeerResult<(BTreeMap<String, String>, i64)> {
    let reader = SerializedFileReader::new(File::open(path)?)
        .map_err(|err| PeerError::Parquet(format!("{}: {err}", path.display())))?;
    let metadata = reader.metadata().file_metadata();
    let mut columns = BTreeMap::new();
    for field in metadata.schema().get_fields() {
        let field = field.as_ref();
        columns.insert(field.name().to_string(), format!("{:?}", field));
    }
    if columns.is_empty() {
        return Err(PeerError::Parquet(format!(
            "{} has no physical columns",
            path.display()
        )));
    }
    Ok((columns, metadata.num_rows()))
}

fn inspect_geotiff(path: &Path, profile: &RasterPublication) -> PeerResult<RasterDescriptor> {
    let mut decoder = Decoder::new(BufReader::new(File::open(path)?))
        .map_err(|err| PeerError::GeoTiff(format!("{}: {err}", path.display())))?;
    let (width, height) = decoder
        .dimensions()
        .map_err(|err| PeerError::GeoTiff(format!("{}: {err}", path.display())))?;
    let color_type = format!(
        "{:?}",
        decoder
            .colortype()
            .map_err(|err| PeerError::GeoTiff(format!("{}: {err}", path.display())))?
    );
    let geo_keys = decoder
        .get_tag_u16_vec(Tag::GeoKeyDirectoryTag)
        .map_err(|_| PeerError::GeoTiff(format!("{} lacks GeoKeyDirectoryTag", path.display())))?;
    if geo_keys.len() < 4 {
        return Err(PeerError::GeoTiff(format!(
            "{} has malformed GeoKeyDirectoryTag",
            path.display()
        )));
    }
    let has_transform = decoder
        .find_tag(Tag::ModelPixelScaleTag)
        .ok()
        .flatten()
        .is_some()
        || decoder
            .find_tag(Tag::ModelTransformationTag)
            .ok()
            .flatten()
            .is_some();
    if !has_transform {
        return Err(PeerError::GeoTiff(format!(
            "{} lacks a GeoTIFF transform",
            path.display()
        )));
    }
    let nodata = decoder.get_tag_ascii_string(Tag::GdalNodata).ok();
    let crs = epsg_from_geo_keys(&geo_keys).map(|code| format!("EPSG:{code}"));
    Ok(RasterDescriptor {
        width,
        height,
        color_type,
        geo_key_directory: geo_keys,
        crs,
        nodata,
        datetime: profile.datetime.clone(),
        start_datetime: profile.start_datetime.clone(),
        end_datetime: profile.end_datetime.clone(),
        bbox: profile.bbox,
        semantics: profile.semantics.clone(),
    })
}

/// Exactly one scientific instant or a complete closed interval. Modification,
/// retrieval and publication timestamps are never substitutes for this metadata.
pub fn raster_time_bounds(
    datetime: Option<&str>,
    start: Option<&str>,
    end: Option<&str>,
) -> PeerResult<(DateTime<Utc>, DateTime<Utc>)> {
    let parse = |value: &str| {
        DateTime::parse_from_rfc3339(value)
            .map(|time| time.with_timezone(&Utc))
            .map_err(|_| validation("raster.datetime", "scientific time must be RFC 3339"))
    };
    match (datetime, start, end) {
        (Some(value), None, None) => {
            let instant = parse(value)?;
            Ok((instant, instant))
        }
        (None, Some(start), Some(end)) => {
            let (start, end) = (parse(start)?, parse(end)?);
            if start > end {
                return Err(validation(
                    "raster.datetime",
                    "interval start must not follow end",
                ));
            }
            Ok((start, end))
        }
        _ => Err(validation(
            "raster.datetime",
            "supply datetime alone or null datetime with both start_datetime and end_datetime",
        )),
    }
}

fn epsg_from_geo_keys(keys: &[u16]) -> Option<u16> {
    // GeoKeyDirectory entries are key id, TIFF tag location, count, value.
    let (entries, _) = keys.get(4..)?.as_chunks::<4>();
    entries.iter().find_map(|entry| {
        (matches!(entry[0], 2048 | 3072) && entry[1] == 0 && entry[2] == 1).then_some(entry[3])
    })
}

fn merge_table_descriptors(
    descriptors: Vec<(BTreeMap<String, String>, i64)>,
    declared: &TablePublication,
) -> PeerResult<TableDescriptor> {
    let Some((columns, first_rows)) = descriptors.first() else {
        return Err(validation(
            "assets",
            "must declare at least one table asset",
        ));
    };
    for (candidate, _) in descriptors.iter().skip(1) {
        if candidate != columns {
            return Err(PeerError::Parquet(
                "declared shards have incompatible physical schemas".into(),
            ));
        }
    }
    for name in columns.keys() {
        let meaning = declared.column_meanings.get(name).ok_or_else(|| {
            validation(
                "table.column_meanings",
                format!("missing meaning for column '{name}'"),
            )
        })?;
        let unit = declared.column_units.get(name).ok_or_else(|| {
            validation(
                "table.column_units",
                format!("missing unit or explicit not_applicable/unknown for column '{name}'"),
            )
        })?;
        non_blank("table.column_meanings", meaning)?;
        non_blank("table.column_units", unit)?;
    }
    for partition in &declared.partition_columns {
        if columns.contains_key(partition) {
            return Err(validation(
                "table.partition_columns",
                format!("partition column '{partition}' conflicts with a physical Parquet column"),
            ));
        }
    }
    let row_count = descriptors.iter().map(|(_, rows)| rows).sum::<i64>();
    let schema_fingerprint = hex_digest(serde_json::to_string(columns)?.as_bytes());
    let _ = first_rows;
    Ok(TableDescriptor {
        columns: columns.clone(),
        schema_fingerprint,
        row_count,
        partition_columns: declared.partition_columns.clone(),
        column_meanings: declared.column_meanings.clone(),
        column_units: declared.column_units.clone(),
    })
}

fn merge_raster_descriptors(
    mut descriptors: Vec<RasterDescriptor>,
) -> PeerResult<RasterDescriptor> {
    descriptors
        .drain(..)
        .next()
        .ok_or_else(|| validation("assets", "must declare at least one raster asset"))
}

fn recheck_candidate(serving_root: &Path, candidate: &PublishedVersion) -> PeerResult<()> {
    for asset in &candidate.assets {
        let (_, canonical) = checked_asset_path(serving_root, serving_root, &asset.path)?;
        let metadata = fs::metadata(&canonical)?;
        if metadata.len() != asset.size {
            return Err(PeerError::Integrity(format!(
                "asset '{}' changed during publication",
                asset.id
            )));
        }
        if let Some(expected) = &asset.sha256
            && sha256_file(&canonical)? != *expected
        {
            return Err(PeerError::Integrity(format!(
                "asset '{}' changed during publication",
                asset.id
            )));
        }
    }
    Ok(())
}

fn read_manifest_at(path: &Path, _expected_namespace: &str) -> PeerResult<ServingManifest> {
    if !path.exists() {
        return Err(PeerError::NotFound(format!(
            "manifest '{}'",
            path.display()
        )));
    }
    parse_manifest(&read_bounded(path, MAX_MANIFEST_BYTES)?)
}

fn parse_manifest(bytes: &[u8]) -> PeerResult<ServingManifest> {
    let manifest: ServingManifest = serde_json::from_slice(bytes)?;
    if manifest.schema_version != MANIFEST_SCHEMA_VERSION {
        return Err(validation(
            "manifest.schema_version",
            "unsupported manifest schema version",
        ));
    }
    validate_namespace(&manifest.namespace)?;
    Ok(manifest)
}

fn read_or_empty_manifest(path: &Path, namespace: &str) -> PeerResult<ServingManifest> {
    match read_manifest_at(path, namespace) {
        Ok(manifest) => Ok(manifest),
        Err(PeerError::NotFound(_)) => Ok(ServingManifest::empty(namespace.to_string())),
        Err(err) => Err(err),
    }
}

fn write_manifest_atomic(path: &Path, manifest: &ServingManifest) -> PeerResult<()> {
    let bytes = serde_json::to_vec_pretty(manifest)?;
    if bytes.len() as u64 > MAX_MANIFEST_BYTES {
        return Err(validation(
            "manifest",
            "publication would exceed the 8 MiB manifest limit",
        ));
    }
    write_atomic_text(path, &bytes)
}

struct ManifestLock {
    path: PathBuf,
}
impl ManifestLock {
    fn acquire(manifest: &Path, timeout: Duration) -> PeerResult<Self> {
        let path = manifest.with_file_name(".feam-manifest.lock");
        let started = Instant::now();
        loop {
            match OpenOptions::new().write(true).create_new(true).open(&path) {
                Ok(mut file) => {
                    writeln!(
                        file,
                        "pid={} created_at={}",
                        std::process::id(),
                        Utc::now().to_rfc3339()
                    )?;
                    file.sync_all()?;
                    return Ok(Self { path });
                }
                Err(err)
                    if err.kind() == std::io::ErrorKind::AlreadyExists
                        && started.elapsed() < timeout =>
                {
                    thread::sleep(Duration::from_millis(50))
                }
                Err(err) if err.kind() == std::io::ErrorKind::AlreadyExists => {
                    return Err(PeerError::LockTimeout(path.to_string_lossy().into_owned()));
                }
                Err(err) => return Err(PeerError::Io(err)),
            }
        }
    }
}
impl Drop for ManifestLock {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.path);
    }
}

fn checked_asset_path(
    displayed_root: &Path,
    canonical_root: &Path,
    relative: &Path,
) -> PeerResult<(PathBuf, PathBuf)> {
    validate_relative_path(relative, "asset.path")?;
    let displayed = displayed_root.join(relative);
    let canonical = fs::canonicalize(&displayed).map_err(|_| {
        PeerError::PeerUnavailable(format!("registered asset route '{}'", displayed.display()))
    })?;
    if !canonical.starts_with(canonical_root) {
        return Err(PeerError::Policy(format!(
            "registered asset '{}' escapes serving root",
            relative.display()
        )));
    }
    if !fs::metadata(&canonical)?.is_file() {
        return Err(PeerError::Policy(format!(
            "registered asset '{}' is not a regular file",
            relative.display()
        )));
    }
    Ok((displayed, canonical))
}

pub(crate) fn preflight_stage(
    resolved: &ResolvedProduct,
    out: &Path,
    overwrite: bool,
) -> PeerResult<()> {
    let parent = out
        .parent()
        .ok_or_else(|| PeerError::Policy("output has no parent".into()))?;
    let parent = fs::canonicalize(parent)?;
    let effective_out = parent.join(
        out.file_name()
            .ok_or_else(|| PeerError::Policy("output needs a filename".into()))?,
    );
    let receipt = receipt_path(&effective_out);
    // Never follow output/receipt links, including dangling links. Existing
    // receipts must be regular files; a directory cannot be replaced atomically.
    for path in [out, receipt.as_path()] {
        if let Ok(metadata) = fs::symlink_metadata(path)
            && (metadata.file_type().is_symlink() || (path == receipt && !metadata.is_file()))
        {
            return Err(PeerError::Policy(
                "output/receipt link or non-file receipt is not supported".into(),
            ));
        }
    }
    if fs::symlink_metadata(out)
        .map(|metadata| metadata.file_type().is_symlink())
        .unwrap_or(false)
        && fs::canonicalize(out).is_err()
    {
        return Err(PeerError::Policy(format!(
            "output '{}' is a dangling symlink",
            out.display()
        )));
    }
    if out.exists() && !overwrite {
        return Err(PeerError::Policy(format!(
            "destination exists: {}",
            out.display()
        )));
    }
    let out_canonical = fs::canonicalize(out).ok();
    for asset in &resolved.assets {
        let source = fs::canonicalize(&asset.project_access_path)
            .map_err(|_| PeerError::PeerUnavailable(format!("registered asset '{}'", asset.id)))?;
        if out_canonical.as_ref() == Some(&source) || receipt == source {
            return Err(PeerError::Policy(
                "source/output or receipt/source collision".into(),
            ));
        }
        if source.starts_with(&effective_out) || effective_out.starts_with(&source) {
            return Err(PeerError::Policy(
                "source/output ancestor or descendant overlap".into(),
            ));
        }
        if let Ok(receipt_metadata) = fs::metadata(&receipt) {
            let source_metadata = fs::metadata(&source)?;
            #[cfg(unix)]
            {
                use std::os::unix::fs::MetadataExt;
                if source_metadata.dev() == receipt_metadata.dev()
                    && source_metadata.ino() == receipt_metadata.ino()
                {
                    return Err(PeerError::Policy("receipt/source hard-link alias".into()));
                }
            }
        }
        if let Ok(output_metadata) = fs::metadata(out) {
            let source_metadata = fs::metadata(&source)?;
            #[cfg(unix)]
            {
                use std::os::unix::fs::MetadataExt;
                if source_metadata.dev() == output_metadata.dev()
                    && source_metadata.ino() == output_metadata.ino()
                {
                    return Err(PeerError::Policy("source/output hard-link alias".into()));
                }
            }
        }
    }
    Ok(())
}

pub(crate) fn receipt_path(out: &Path) -> PathBuf {
    out.with_file_name(format!(
        "{}.feam-receipt.json",
        out.file_name().unwrap_or_default().to_string_lossy()
    ))
}
fn remove_any(path: &Path) -> std::io::Result<()> {
    if path.is_dir() {
        fs::remove_dir_all(path)
    } else if path.exists() {
        fs::remove_file(path)
    } else {
        Ok(())
    }
}
pub(crate) fn sha256_file(path: &Path) -> PeerResult<String> {
    let mut file = File::open(path)?;
    let mut hash = Sha256::new();
    let mut buffer = [0_u8; 32 * 1024];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hash.update(&buffer[..read]);
    }
    Ok(format!("{:x}", hash.finalize()))
}
fn hex_digest(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}
fn write_atomic_text(path: &Path, bytes: &[u8]) -> PeerResult<()> {
    let parent = path
        .parent()
        .ok_or_else(|| PeerError::Policy(format!("path '{}' has no parent", path.display())))?;
    fs::create_dir_all(parent)?;
    let temporary = parent.join(format!(
        ".{}.{}.tmp",
        path.file_name().unwrap_or_default().to_string_lossy(),
        unique_nonce()
    ));
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&temporary)?;
    file.write_all(bytes)?;
    file.sync_all()?;
    drop(file);
    fs::rename(&temporary, path)?;
    Ok(())
}
fn unique_nonce() -> String {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos()
        .to_string()
}
pub(crate) fn absolute_path(path: &Path) -> PeerResult<PathBuf> {
    if path.is_absolute() {
        Ok(path.to_path_buf())
    } else {
        Ok(std::env::current_dir()?.join(path))
    }
}
fn validate_relative_path(path: &Path, field: &str) -> PeerResult<()> {
    if path.as_os_str().is_empty() || path.is_absolute() {
        return Err(validation(field, "must be a non-empty relative path"));
    }
    for component in path.components() {
        if !matches!(component, Component::Normal(_)) {
            return Err(validation(
                field,
                "must not contain traversal, root, or prefix components",
            ));
        }
    }
    Ok(())
}
fn validate_project_config(config: &ProjectConfig) -> PeerResult<()> {
    if config.schema_version != PROJECT_SCHEMA_VERSION {
        return Err(validation(
            "schema_version",
            "only project schema version 1 is supported",
        ));
    }
    validate_namespace(&config.namespace)?;
    if let Some(path) = &config.serving_dir {
        validate_relative_path(path, "serving_dir")?;
    }
    if config.owner_teams.is_empty() {
        return Err(validation(
            "owner_teams",
            "must contain at least one provider-authorized team",
        ));
    }
    for team in &config.owner_teams {
        non_blank("owner_teams", team)?;
    }
    let mut namespaces = BTreeSet::new();
    let mut aliases = BTreeSet::new();
    for peer in &config.peers {
        non_blank("peers.alias", &peer.alias)?;
        validate_namespace(&peer.namespace)?;
        if peer.namespace == config.namespace {
            return Err(validation(
                "peers.namespace",
                "must not duplicate the local namespace",
            ));
        }
        if !namespaces.insert(peer.namespace.clone()) {
            return Err(validation("peers.namespace", "must be unique"));
        }
        if !aliases.insert(peer.alias.clone()) {
            return Err(validation("peers.alias", "must be unique"));
        }
        if !peer.path.is_absolute() {
            validate_relative_path(&peer.path, "peers.path")?;
        }
    }
    Ok(())
}
fn non_blank(field: &str, value: &str) -> PeerResult<()> {
    if value.trim().is_empty() {
        Err(validation(field, "must not be blank"))
    } else {
        Ok(())
    }
}
fn validate_namespace(value: &str) -> PeerResult<()> {
    validate_token("namespace", value)
}
fn validate_product_id(value: &str) -> PeerResult<()> {
    validate_token("product_id", value)
}
fn validate_version(value: &str) -> PeerResult<()> {
    validate_token("version", value)
}
fn validate_token(field: &str, value: &str) -> PeerResult<()> {
    non_blank(field, value)?;
    if value
        .chars()
        .all(|char| char.is_ascii_alphanumeric() || matches!(char, '-' | '_' | '.'))
    {
        Ok(())
    } else {
        Err(validation(
            field,
            "may contain only ASCII letters, numbers, hyphens, underscores, and dots",
        ))
    }
}
pub fn reference(namespace: &str, product_id: &str) -> String {
    format!("product://{namespace}/{product_id}")
}
pub fn parse_reference(value: &str) -> PeerResult<(String, &str)> {
    let without_scheme = value
        .strip_prefix("product://")
        .ok_or_else(|| validation("reference", "must use product://<namespace>/<dataset-id>"))?;
    let (namespace, product_id) = without_scheme
        .split_once('/')
        .ok_or_else(|| validation("reference", "must contain namespace and dataset id"))?;
    if product_id.contains('/') {
        return Err(validation(
            "reference",
            "must contain exactly one dataset id",
        ));
    }
    validate_namespace(namespace)?;
    validate_product_id(product_id)?;
    Ok((namespace.to_string(), product_id))
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn parses_qualified_references_and_rejects_paths() {
        assert_eq!(
            parse_reference("product://climate/observations").unwrap(),
            ("climate".into(), "observations")
        );
        assert!(parse_reference("/readable/path.tiff").is_err());
        assert!(parse_reference("product://climate/../secret").is_err());
    }
    #[test]
    fn rejects_traversal_paths() {
        assert!(validate_relative_path(Path::new("datasets/x.parquet"), "asset.path").is_ok());
        assert!(validate_relative_path(Path::new("../x.parquet"), "asset.path").is_err());
        assert!(validate_relative_path(Path::new("/x.parquet"), "asset.path").is_err());
    }
}
