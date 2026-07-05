use serde::{Deserialize, Serialize};

use super::{ValidationError, normalize_token};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AccessClassification {
    Public,
    Internal,
    Restricted,
}

impl AccessClassification {
    /// Parses and normalizes a supported access classification.
    pub fn parse(input: &str) -> Result<Self, ValidationError> {
        match normalize_token(input).as_str() {
            "public" => Ok(Self::Public),
            "internal" => Ok(Self::Internal),
            "restricted" => Ok(Self::Restricted),
            "" => Err(ValidationError::new(
                "classification",
                "must be one of public, internal, or restricted",
            )),
            _ => Err(ValidationError::new(
                "classification",
                format!(
                    "unsupported value '{}'; expected public, internal, or restricted",
                    input
                ),
            )),
        }
    }

    /// Returns the canonical database/API string for this classification.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Public => "public",
            Self::Internal => "internal",
            Self::Restricted => "restricted",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_supported_values() {
        assert_eq!(
            AccessClassification::parse("public").unwrap().as_str(),
            "public"
        );
        assert_eq!(
            AccessClassification::parse("Internal").unwrap().as_str(),
            "internal"
        );
        assert_eq!(
            AccessClassification::parse("restricted").unwrap().as_str(),
            "restricted"
        );
    }
}
