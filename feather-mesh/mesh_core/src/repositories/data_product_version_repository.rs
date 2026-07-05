use rusqlite::{Connection, Result, Row, params, types::Type};

use crate::models::{
    AccessClassification, AssetType, DataProductVersion, DataQuality, NewDataProductVersion,
    ValidationError,
};

use super::parse_naive_datetime;

pub struct DataProductVersionRepository;

impl DataProductVersionRepository {
    /// Inserts a new data product version and returns the persisted version row with database-managed fields.
    pub fn create(conn: &Connection, input: NewDataProductVersion) -> Result<DataProductVersion> {
        let classification = input.classification.map(|value| value.as_str());
        conn.execute(
            "INSERT INTO data_product_versions
                (data_product_id, version_label, asset_type, source_path, data_quality, classification)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            params![
                input.data_product_id,
                input.version_label,
                input.asset_type.as_str(),
                input.source_path,
                input.data_quality.as_str(),
                classification
            ],
        )?;

        let version_id = conn.last_insert_rowid();
        Self::get_by_id(conn, version_id)
    }

    /// Gets a persisted data product version by its primary key.
    pub fn get_by_id(conn: &Connection, version_id: i64) -> Result<DataProductVersion> {
        conn.query_row(
            "SELECT version_id, data_product_id, version_label, asset_type, source_path,
                    data_quality, classification, created_at
             FROM data_product_versions
             WHERE version_id = ?1",
            params![version_id],
            Self::from_row,
        )
    }

    /// Gets all persisted data product versions ordered by primary key.
    pub fn get_all(conn: &Connection) -> Result<Vec<DataProductVersion>> {
        let mut stmt = conn.prepare(
            "SELECT version_id, data_product_id, version_label, asset_type, source_path,
                    data_quality, classification, created_at
             FROM data_product_versions
             ORDER BY version_id",
        )?;
        let rows = stmt.query_map([], Self::from_row)?;

        rows.collect()
    }

    /// Maps database row -> DataProductVersion struct
    fn from_row(row: &Row<'_>) -> Result<DataProductVersion> {
        let created_at: String = row.get("created_at")?;
        let asset_type: String = row.get("asset_type")?;
        let data_quality: String = row.get("data_quality")?;
        let classification: Option<String> = row.get("classification")?;

        Ok(DataProductVersion {
            version_id: row.get("version_id")?,
            data_product_id: row.get("data_product_id")?,
            version_label: row.get("version_label")?,
            asset_type: parse_domain_value(3, &asset_type, AssetType::parse)?,
            source_path: row.get("source_path")?,
            data_quality: parse_domain_value(5, &data_quality, DataQuality::parse)?,
            classification: classification
                .map(|value| parse_domain_value(6, &value, AccessClassification::parse))
                .transpose()?,
            created_at: parse_naive_datetime(7, created_at)?,
        })
    }
}

/// Converts persisted metadata text into a typed domain value.
fn parse_domain_value<T>(
    column_index: usize,
    value: &str,
    parse: impl FnOnce(&str) -> Result<T, ValidationError>,
) -> Result<T> {
    parse(value).map_err(|err| {
        rusqlite::Error::FromSqlConversionFailure(column_index, Type::Text, Box::new(err))
    })
}
