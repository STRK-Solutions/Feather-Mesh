use serde::{Deserialize, Serialize};

use super::{ValidationError, normalize_token};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DataQuality {
    Production,
    Qualified,
    Unverified,
}

impl DataQuality {
    /// Parses and normalizes a supported data quality tier.
    pub fn parse(input: &str) -> Result<Self, ValidationError> {
        match normalize_token(input).as_str() {
            "production" => Ok(Self::Production),
            "qualified" => Ok(Self::Qualified),
            "unverified" => Ok(Self::Unverified),
            "" => Err(ValidationError::new(
                "data_quality",
                "must be one of production, qualified, or unverified",
            )),
            _ => Err(ValidationError::new(
                "data_quality",
                format!(
                    "unsupported value '{}'; expected production, qualified, or unverified",
                    input
                ),
            )),
        }
    }

    /// Returns the canonical database/API string for this quality tier.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Production => "production",
            Self::Qualified => "qualified",
            Self::Unverified => "unverified",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_only_supported_values_and_normalizes_case() {
        assert_eq!(
            DataQuality::parse("production").unwrap().as_str(),
            "production"
        );
        assert_eq!(
            DataQuality::parse(" Qualified ").unwrap().as_str(),
            "qualified"
        );
        assert_eq!(
            DataQuality::parse("UNVERIFIED").unwrap().as_str(),
            "unverified"
        );
    }

    #[test]
    fn rejects_legacy_and_blank_values() {
        for value in ["gold", "silver", "bronze", "test", ""] {
            assert!(DataQuality::parse(value).is_err());
        }
    }
}
