use chrono::NaiveDateTime;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DataProductVersion {
    pub version_id: i64,
    pub data_product_id: i64,
    pub version_label: String,
    pub asset_type: String,
    pub source_path: String,
    pub data_quality: String,
    pub classification: String,
    pub created_at: NaiveDateTime,
}

#[cfg(test)]
mod tests {
    use super::DataProductVersion;
    use chrono::NaiveDateTime;

    #[test]
    // Confirms serde preserves the model shape and timestamp values through a JSON round trip.
    fn serializes_and_deserializes_with_expected_shape() {
        let created_at =
            NaiveDateTime::parse_from_str("2026-03-11 14:45:00", "%Y-%m-%d %H:%M:%S").unwrap();
        let version = DataProductVersion {
            version_id: 8,
            data_product_id: 3,
            version_label: "2026.03".to_string(),
            asset_type: "test".to_string(),
            source_path: "test/data".to_string(),
            data_quality: "test".to_string(),
            classification: "internal".to_string(),
            created_at,
        };

        let json = serde_json::to_string(&version).unwrap();
        let round_trip: DataProductVersion = serde_json::from_str(&json).unwrap();

        assert!(json.contains("\"version_id\":8"));
        assert!(json.contains("\"data_product_id\":3"));
        assert!(json.contains("\"version_label\":\"2026.03\""));
        assert!(json.contains("\"classification\":\"internal\""));
        assert_eq!(round_trip, version);
    }
}
