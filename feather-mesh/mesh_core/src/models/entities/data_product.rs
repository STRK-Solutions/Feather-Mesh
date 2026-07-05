use chrono::NaiveDateTime;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DataProduct {
    pub product_id: i64,
    pub name: String,
    pub description: Option<String>,
    pub owner_team_id: i64,
    pub producer: String,
    pub usage_policy: String,
    pub intended_use: Option<String>,
    pub created_at: NaiveDateTime,
}

#[cfg(test)]
mod tests {
    use super::DataProduct;
    use chrono::NaiveDateTime;

    #[test]
    // Confirms serde preserves the model shape and timestamp values through a JSON round trip.
    fn serializes_and_deserializes_with_expected_shape() {
        let created_at =
            NaiveDateTime::parse_from_str("2026-03-11 08:00:00", "%Y-%m-%d %H:%M:%S").unwrap();
        let product = DataProduct {
            product_id: 11,
            name: "Orders".to_string(),
            description: Some("Order facts curated for finance reporting".to_string()),
            owner_team_id: 2,
            producer: "Finance".to_string(),
            usage_policy: "Internal analytics only".to_string(),
            intended_use: Some("Monthly reconciliations".to_string()),
            created_at,
        };

        let json = serde_json::to_string(&product).unwrap();
        let round_trip: DataProduct = serde_json::from_str(&json).unwrap();

        assert!(json.contains("\"product_id\":11"));
        assert!(json.contains("\"name\":\"Orders\""));
        assert!(json.contains("\"owner_team_id\":2"));
        assert!(json.contains("\"producer\":\"Finance\""));
        assert_eq!(round_trip, product);
    }
}
