use serde::{Deserialize, Serialize};

use crate::models::{AccessClassification, AssetType, DataQuality};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NewDataProductVersion {
    pub data_product_id: i64,
    pub version_label: String,
    pub asset_type: AssetType,
    pub source_path: String,
    pub data_quality: DataQuality,
    pub classification: Option<AccessClassification>,
}

impl NewDataProductVersion {
    /// Creates NewDataProductVersion instance for database insertion, excluding fields handled by the database layer.
    pub fn new(
        data_product_id: i64,
        version_label: String,
        asset_type: AssetType,
        source_path: String,
        data_quality: DataQuality,
        classification: Option<AccessClassification>,
    ) -> Self {
        Self {
            data_product_id,
            version_label,
            asset_type,
            source_path,
            data_quality,
            classification,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::NewDataProductVersion;
    use crate::models::{AccessClassification, AssetType, DataQuality};

    #[test]
    // Verifies the constructor leaves optional fields unset and preserves input values.
    fn new_data_product_version_initializes_insert_model_with_optional_fields_to_none() {
        let version = NewDataProductVersion::new(
            12,
            "v1.0.0".to_string(),
            AssetType::File,
            "test/data".to_string(),
            DataQuality::Production,
            None,
        );

        assert_eq!(version.data_product_id, 12);
        assert_eq!(version.version_label, "v1.0.0");
        assert_eq!(version.asset_type, AssetType::File);
        assert_eq!(version.source_path, "test/data");
        assert_eq!(version.data_quality, DataQuality::Production);
        assert_eq!(version.classification, None);
    }

    #[test]
    // Verifies the constructor preserves optional user-provided values.
    fn new_data_product_version_preserves_optional_fields() {
        let version = NewDataProductVersion::new(
            12,
            "v1.0.0".to_string(),
            AssetType::File,
            "test/data".to_string(),
            DataQuality::Production,
            Some(AccessClassification::Internal),
        );

        assert_eq!(version.classification, Some(AccessClassification::Internal));
    }
}
