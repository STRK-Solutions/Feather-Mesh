use super::ValidationError;

/// Requires a non-empty string and returns its trimmed value.
pub fn required_string(field: &'static str, value: String) -> Result<String, ValidationError> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        Err(ValidationError::new(field, "is required"))
    } else {
        Ok(trimmed.to_string())
    }
}

/// Validates an optional string when it is present.
pub fn optional_non_blank(
    field: &'static str,
    value: Option<String>,
) -> Result<Option<String>, ValidationError> {
    value.map(|inner| required_string(field, inner)).transpose()
}

/// Requires a database identifier to be positive.
pub fn validate_positive_id(field: &'static str, value: i64) -> Result<i64, ValidationError> {
    if value > 0 {
        Ok(value)
    } else {
        Err(ValidationError::new(field, "must be greater than zero"))
    }
}

/// Validates a user-facing data product name.
pub fn validate_product_name(value: String) -> Result<String, ValidationError> {
    let value = required_string("name", value)?;
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '_' | '-' | ' '))
    {
        Ok(value)
    } else {
        Err(ValidationError::new(
            "name",
            "must contain only ASCII letters, numbers, spaces, underscores, or hyphens",
        ))
    }
}

/// Validates a product version label.
pub fn validate_version_label(value: String) -> Result<String, ValidationError> {
    let value = required_string("version_label", value)?;
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '.' | '_' | '-'))
    {
        Ok(value)
    } else {
        Err(ValidationError::new(
            "version_label",
            "must contain only ASCII letters, numbers, dots, underscores, or hyphens",
        ))
    }
}

/// Validates a file path or URI-like source reference.
pub fn validate_source_reference(value: String) -> Result<String, ValidationError> {
    let value = required_string("source_path", value)?;
    if value.contains(char::is_whitespace) {
        Err(ValidationError::new(
            "source_path",
            "must not contain whitespace",
        ))
    } else {
        Ok(value)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_required_product_fields() {
        assert_eq!(
            validate_product_name("Daily Observations".to_string()).unwrap(),
            "Daily Observations"
        );
        assert_eq!(validate_positive_id("owner_team_id", 5).unwrap(), 5);
        assert!(validate_product_name(" ".to_string()).is_err());
        assert!(validate_product_name("bad/name".to_string()).is_err());
        assert!(validate_positive_id("owner_team_id", 0).is_err());
        assert!(required_string("producer", "Climate Lab".to_string()).is_ok());
        assert!(required_string("usage_policy", "".to_string()).is_err());
    }

    #[test]
    fn validates_required_version_fields() {
        assert_eq!(
            validate_version_label("v1.0.0".to_string()).unwrap(),
            "v1.0.0"
        );
        assert!(validate_version_label("v 1".to_string()).is_err());
        assert_eq!(
            validate_source_reference("/data/climate.csv".to_string()).unwrap(),
            "/data/climate.csv"
        );
        assert!(validate_source_reference("/data/climate daily.csv".to_string()).is_err());
    }
}
