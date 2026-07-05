use std::fmt;

use rusqlite::Connection;

use crate::models::metadata::{
    optional_non_blank, required_string, validate_positive_id, validate_product_name,
    validate_source_reference, validate_version_label,
};
use crate::models::{
    AccessClassification, AssetType, DataProduct, DataProductVersion, DataQuality,
    LineageDependency, NewDataProduct, NewDataProductVersion, NewLineageDependency, NewTeam, Team,
    ValidationError,
};
use crate::repositories::{
    DataProductRepository, DataProductVersionRepository, LineageDependencyRepository,
    TeamRepository,
};
use crate::services::requests::{
    CreateDataProductRequest, CreateDataProductVersionRequest, CreateLineageDependencyRequest,
};

// RegistryService exposes API-style registry workflows for `mesh_cli`.
pub type ServiceResult<T> = Result<T, ServiceError>;

#[derive(Debug)]
pub enum ServiceError {
    Validation(ValidationError),
    Repository(rusqlite::Error),
}

impl fmt::Display for ServiceError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Validation(err) => write!(f, "validation failed: {err}"),
            Self::Repository(err) => write!(f, "repository operation failed: {err}"),
        }
    }
}

impl std::error::Error for ServiceError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Validation(err) => Some(err),
            Self::Repository(err) => Some(err),
        }
    }
}

impl From<ValidationError> for ServiceError {
    fn from(value: ValidationError) -> Self {
        Self::Validation(value)
    }
}

impl From<rusqlite::Error> for ServiceError {
    fn from(value: rusqlite::Error) -> Self {
        Self::Repository(value)
    }
}

// note: `a -> lifetime of the connection reference, ensures registry service does not outlive the db connection it uses
pub struct RegistryService<'a> {
    conn: &'a Connection,
}

impl<'a> RegistryService<'a> {
    /// Creates a registry service backed by an existing database connection.
    pub fn new(conn: &'a Connection) -> Self {
        Self { conn }
    }

    /// Registers a team in the registry and returns the persisted team.
    pub fn register_team(&self, name: String) -> ServiceResult<Team> {
        Ok(TeamRepository::create(self.conn, NewTeam::new(name))?)
    }

    /// Returns all teams currently registered in the registry.
    pub fn list_teams(&self) -> ServiceResult<Vec<Team>> {
        Ok(TeamRepository::get_all(self.conn)?)
    }

    /// Registers a data product after validating required keystone metadata.
    pub fn register_data_product(
        &self,
        input: CreateDataProductRequest,
    ) -> ServiceResult<DataProduct> {
        let product = NewDataProduct::new(
            validate_product_name(input.name)?,
            optional_non_blank("description", input.description)?,
            validate_positive_id("owner_team_id", input.owner_team_id)?,
            optional_non_blank("intended_use", input.intended_use)?,
            required_string("producer", input.producer)?,
            required_string("usage_policy", input.usage_policy)?,
        );

        Ok(DataProductRepository::create(self.conn, product)?)
    }

    /// Registers a data product version after validating version-level metadata.
    pub fn register_data_product_version(
        &self,
        input: CreateDataProductVersionRequest,
    ) -> ServiceResult<DataProductVersion> {
        let asset_type: AssetType = AssetType::parse(&input.asset_type)?;
        let data_quality: DataQuality = DataQuality::parse(&input.data_quality)?;
        let classification = AccessClassification::parse(&input.classification)?;
        let version: NewDataProductVersion = NewDataProductVersion::new(
            validate_positive_id("data_product_id", input.data_product_id)?,
            validate_version_label(input.version_label)?,
            asset_type,
            validate_source_reference(input.source_path)?,
            data_quality,
            Some(classification),
        );

        Ok(DataProductVersionRepository::create(self.conn, version)?)
    }

    /// Registers an input dependency after validating lineage metadata.
    pub fn register_lineage_dependency(
        &self,
        input: CreateLineageDependencyRequest,
    ) -> ServiceResult<LineageDependency> {
        let dependency = NewLineageDependency::new(
            validate_positive_id("downstream_version_id", input.downstream_version_id)?,
            validate_source_reference(input.upstream_product_uri)?,
            input
                .upstream_version
                .map(validate_version_label)
                .transpose()?,
        );

        Ok(LineageDependencyRepository::create(self.conn, dependency)?)
    }
}
