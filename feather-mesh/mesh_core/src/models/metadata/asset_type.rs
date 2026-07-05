use serde::{Deserialize, Serialize};

use super::{ValidationError, normalize_token};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AssetType {
    Dataset,
    Table,
    File,
    Directory,
    ModelArtifact,
}

impl AssetType {
    /// Parses and normalizes a supported product version asset type.
    pub fn parse(input: &str) -> Result<Self, ValidationError> {
        match normalize_token(input).as_str() {
            "dataset" => Ok(Self::Dataset),
            "table" => Ok(Self::Table),
            "file" => Ok(Self::File),
            "directory" => Ok(Self::Directory),
            "model_artifact" => Ok(Self::ModelArtifact),
            "" => Err(ValidationError::new(
                "asset_type",
                "must be one of dataset, table, file, directory, or model_artifact",
            )),
            _ => Err(ValidationError::new(
                "asset_type",
                format!(
                    "unsupported value '{}'; expected dataset, table, file, directory, or model_artifact",
                    input
                ),
            )),
        }
    }

    /// Returns the canonical database/API string for this asset type.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Dataset => "dataset",
            Self::Table => "table",
            Self::File => "file",
            Self::Directory => "directory",
            Self::ModelArtifact => "model_artifact",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_supported_values() {
        assert_eq!(AssetType::parse("dataset").unwrap().as_str(), "dataset");
        assert_eq!(
            AssetType::parse("model-artifact").unwrap().as_str(),
            "model_artifact"
        );
    }
}
