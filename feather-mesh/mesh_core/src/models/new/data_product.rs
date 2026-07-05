use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NewDataProduct {
    pub name: String,
    pub description: Option<String>,
    pub owner_team_id: i64,
    pub producer: String,
    pub usage_policy: String,
    pub intended_use: Option<String>,
}

impl NewDataProduct {
    /// Creates NewDataProduct instance for database insertion, excluding fields handled by the database layer.
    pub fn new(
        name: String,
        description: Option<String>,
        owner_team_id: i64,
        producer: String,
        usage_policy: String,
        intended_use: Option<String>,
    ) -> Self {
        Self {
            name,
            description,
            owner_team_id,
            producer,
            usage_policy,
            intended_use,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::NewDataProduct;

    #[test]
    // Verifies the constructor leaves optional fields unset and preserves input values.
    fn new_data_product_initializes_insert_model_with_optional_fields_to_none() {
        let product = NewDataProduct::new(
            "Orders".to_string(),
            None,
            5,
            "Finance".to_string(),
            "Internal use".to_string(),
            None,
        );

        assert_eq!(product.name, "Orders");
        assert_eq!(product.description, None);
        assert_eq!(product.owner_team_id, 5);
        assert_eq!(product.producer, "Finance");
        assert_eq!(product.usage_policy, "Internal use");
        assert_eq!(product.intended_use, None);
    }

    #[test]
    // Verifies the constructor preserves optional user-provided values.
    fn new_data_product_preserves_optional_fields() {
        let product = NewDataProduct::new(
            "Orders".to_string(),
            Some("Order facts curated for finance reporting".to_string()),
            5,
            "Finance".to_string(),
            "Internal use".to_string(),
            Some("Monthly reconciliations".to_string()),
        );

        assert_eq!(
            product.description,
            Some("Order facts curated for finance reporting".to_string())
        );
        assert_eq!(
            product.intended_use,
            Some("Monthly reconciliations".to_string())
        );
    }
}
