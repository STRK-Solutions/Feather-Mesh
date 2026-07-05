use std::path::PathBuf;

use crate::domain::{AssetType, Classification, DataQuality, ValidationError};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CreateDataProductRequest {
    pub name: String,
    pub description: Option<String>,
    pub owner_team_id: i64,
    pub intended_use: Option<String>,
    pub producer: String,
    pub usage_policy: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CreateDataProductVersionRequest {
    pub data_product_id: i64,
    pub version_label: String,
    pub asset_type: String,
    pub source_path: String,
    pub data_quality: String,
    pub classification: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CreateLineageDependencyRequest {
    pub downstream_version_id: i64,
    pub upstream_product_uri: String,
    pub upstream_version: Option<String>,
}

impl CreateDataProductVersionRequest {
    pub fn into_serve_parts(
        self,
    ) -> Result<(String, AssetType, PathBuf, DataQuality, Classification), ValidationError> {
        Ok((
            self.version_label,
            self.asset_type.parse()?,
            PathBuf::from(self.source_path),
            self.data_quality.parse()?,
            self.classification.parse()?,
        ))
    }
}
