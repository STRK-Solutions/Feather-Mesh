use std::fmt;
use std::path::PathBuf;
use std::str::FromStr;

use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum RegistryError {
    #[error("validation failed: {0}")]
    Validation(#[from] ValidationError),
    #[error("not found: {0}")]
    NotFound(String),
    #[error("permission or policy failure: {0}")]
    Permission(String),
    #[error("source missing: {0}")]
    SourceMissing(String),
    #[error("destination exists: {0}")]
    DestinationExists(String),
    #[error("database error: {0}")]
    Database(#[from] rusqlite::Error),
    #[error("filesystem error: {0}")]
    Filesystem(#[from] std::io::Error),
    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),
}

pub type RegistryResult<T> = Result<T, RegistryError>;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Error)]
#[error("{field}: {message}")]
pub struct ValidationError {
    pub field: String,
    pub message: String,
}

impl ValidationError {
    pub fn new(field: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            field: field.into(),
            message: message.into(),
        }
    }
}

macro_rules! domain_enum {
    ($name:ident { $($variant:ident => $value:literal),+ $(,)? }) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
        #[serde(rename_all = "snake_case")]
        pub enum $name {
            $($variant),+
        }

        impl $name {
            pub fn as_str(self) -> &'static str {
                match self {
                    $(Self::$variant => $value),+
                }
            }

            pub fn parse(value: &str) -> Result<Self, ValidationError> {
                value.parse()
            }
        }

        impl fmt::Display for $name {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                f.write_str(self.as_str())
            }
        }

        impl FromStr for $name {
            type Err = ValidationError;

            fn from_str(value: &str) -> Result<Self, Self::Err> {
                match normalize_token(value).as_str() {
                    $($value => Ok(Self::$variant),)+
                    _ => Err(ValidationError::new(
                        stringify!($name),
                        format!("unsupported value '{value}'"),
                    )),
                }
            }
        }
    };
}

pub(crate) fn normalize_token(input: &str) -> String {
    input.trim().to_ascii_lowercase().replace('-', "_")
}

domain_enum!(AssetType {
    File => "file",
    Directory => "directory",
    Dataset => "dataset",
    Table => "table",
    ModelArtifact => "model_artifact",
    ReportArtifact => "report_artifact",
    ManifestCollection => "manifest_collection",
});

domain_enum!(DataQuality {
    Production => "production",
    Qualified => "qualified",
    Unverified => "unverified",
});

domain_enum!(Classification {
    Public => "public",
    Internal => "internal",
    Restricted => "restricted",
});

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum SourceReference {
    UnixPath(PathBuf),
}

impl SourceReference {
    pub fn unix_path(path: PathBuf) -> Result<Self, ValidationError> {
        if path.as_os_str().is_empty() {
            return Err(ValidationError::new("source_path", "must not be empty"));
        }
        Ok(Self::UnixPath(path))
    }

    pub fn as_display_string(&self) -> String {
        match self {
            Self::UnixPath(path) => path.to_string_lossy().into_owned(),
        }
    }
}

pub fn validate_positive_id(field: &str, value: i64) -> Result<(), ValidationError> {
    if value > 0 {
        Ok(())
    } else {
        Err(ValidationError::new(field, "must be a positive integer"))
    }
}

pub fn validate_product_name(value: &str) -> Result<(), ValidationError> {
    validate_non_blank("name", value)?;
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || ch == ' ' || ch == '_' || ch == '-')
    {
        Ok(())
    } else {
        Err(ValidationError::new(
            "name",
            "may contain only ASCII letters, numbers, spaces, underscores, and hyphens",
        ))
    }
}

pub fn validate_version_label(value: &str) -> Result<(), ValidationError> {
    validate_non_blank("version", value)?;
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || ch == '.' || ch == '_' || ch == '-')
    {
        Ok(())
    } else {
        Err(ValidationError::new(
            "version",
            "may contain only ASCII letters, numbers, dots, underscores, and hyphens",
        ))
    }
}

pub fn validate_non_blank(field: &str, value: &str) -> Result<(), ValidationError> {
    if value.trim().is_empty() {
        Err(ValidationError::new(field, "must not be blank"))
    } else {
        Ok(())
    }
}

pub fn validate_optional_text(field: &str, value: Option<&str>) -> Result<(), ValidationError> {
    if let Some(value) = value {
        validate_non_blank(field, value)?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_domain_enums() {
        assert_eq!("file".parse::<AssetType>().unwrap(), AssetType::File);
        assert_eq!(
            "model-artifact".parse::<AssetType>().unwrap(),
            AssetType::ModelArtifact
        );
        assert_eq!(
            "PRODUCTION".parse::<DataQuality>().unwrap(),
            DataQuality::Production
        );
        assert_eq!(
            "restricted".parse::<Classification>().unwrap(),
            Classification::Restricted
        );
        assert!("gold".parse::<DataQuality>().is_err());
        assert!("silver".parse::<DataQuality>().is_err());
        assert!("bronze".parse::<DataQuality>().is_err());
    }

    #[test]
    fn validates_product_names_and_versions() {
        assert!(validate_product_name("Daily Observations_2026").is_ok());
        assert!(validate_product_name(" ").is_err());
        assert!(validate_product_name("bad/name").is_err());
        assert!(validate_version_label("v1.0_2026-07").is_ok());
        assert!(validate_version_label("v 1").is_err());
        assert!(validate_version_label("v1/slash").is_err());
    }

    #[test]
    fn validates_ids_text_and_sources() {
        assert!(validate_positive_id("product_id", 1).is_ok());
        assert!(validate_positive_id("product_id", 0).is_err());
        assert!(validate_optional_text("description", Some("  ")).is_err());
        assert!(SourceReference::unix_path(PathBuf::from("/tmp/data with spaces.csv")).is_ok());
    }
}
