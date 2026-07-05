//! Domain metadata values and validation helpers.
//!
//! This module holds Feather Mesh business rules that are not database row
//! shapes. The enums define the allowed values for keystone metadata fields,
//! while the validators normalize or reject user-provided service input before
//! it reaches the repository layer.

pub mod access_classification;
pub mod asset_type;
pub mod data_quality;
pub mod validation_error;
pub mod validators;

pub use access_classification::AccessClassification;
pub use asset_type::AssetType;
pub use data_quality::DataQuality;
pub use validation_error::ValidationError;
pub use validators::{
    optional_non_blank, required_string, validate_positive_id, validate_product_name,
    validate_source_reference, validate_version_label,
};

/// Normalizes free-form enum input before matching allowed values.
pub(crate) fn normalize_token(input: &str) -> String {
    input.trim().to_ascii_lowercase().replace('-', "_")
}
