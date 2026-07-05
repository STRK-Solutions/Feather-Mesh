use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NewDataProductVersion {
    pub data_product_id: i64,
    pub version_label: String,
    pub asset_type: String,
    pub source_path: String,
    pub data_quality: String,
    pub classification: String,
}

impl NewDataProductVersion {
    /// Creates NewDataProductVersion instance for database insertion, excluding fields handled by the database layer.
    pub fn new(
        data_product_id: i64,
        version_label: String,
        asset_type: String,
        source_path: String,
        data_quality: String,
        classification: String,
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

    #[test]
    // Verifies the constructor leaves optional fields unset and preserves input values.
    fn new_data_product_version_initializes_insert_model() {
        let version = NewDataProductVersion::new(
            12,
            "v1.0.0".to_string(),
            "test".to_string(),
            "test/data".to_string(),
            "test".to_string(),
            "internal".to_string(),
        );

        assert_eq!(version.data_product_id, 12);
        assert_eq!(version.version_label, "v1.0.0");
        assert_eq!(version.asset_type, "test");
        assert_eq!(version.source_path, "test/data");
        assert_eq!(version.data_quality, "test");
        assert_eq!(version.classification, "internal");
    }
}
