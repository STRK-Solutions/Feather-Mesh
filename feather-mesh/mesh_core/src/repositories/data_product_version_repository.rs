use rusqlite::{Connection, Result, Row, params};

use crate::models::{DataProductVersion, NewDataProductVersion};

use super::parse_naive_datetime;

pub struct DataProductVersionRepository;

impl DataProductVersionRepository {
    /// Inserts a new data product version and returns the persisted version row with database-managed fields.
    pub fn create(conn: &Connection, input: NewDataProductVersion) -> Result<DataProductVersion> {
        conn.execute(
            "INSERT INTO data_product_versions
                (data_product_id, version_label, asset_type, source_path, data_quality, classification)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            params![
                input.data_product_id,
                input.version_label,
                input.asset_type,
                input.source_path,
                input.data_quality,
                input.classification
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

    /// Gets all persisted data product versions matching the asset type, case-insensitively.
    pub fn get_all_by_asset_type(
        conn: &Connection,
        asset_type: &str,
    ) -> Result<Vec<DataProductVersion>> {
        let mut stmt = conn.prepare(
            "SELECT version_id, data_product_id, version_label, asset_type, source_path,
                    data_quality, classification, created_at
             FROM data_product_versions
             WHERE LOWER(asset_type) = LOWER(?1)
             ORDER BY version_id",
        )?;
        let rows = stmt.query_map(params![asset_type], Self::from_row)?;

        rows.collect()
    }

    /// Gets all persisted data product versions matching the data quality, case-insensitively.
    pub fn get_all_by_data_quality(
        conn: &Connection,
        data_quality: &str,
    ) -> Result<Vec<DataProductVersion>> {
        let mut stmt = conn.prepare(
            "SELECT version_id, data_product_id, version_label, asset_type, source_path,
                    data_quality, classification, created_at
             FROM data_product_versions
             WHERE LOWER(data_quality) = LOWER(?1)
             ORDER BY version_id",
        )?;
        let rows = stmt.query_map(params![data_quality], Self::from_row)?;

        rows.collect()
    }

    /// Gets all persisted data product versions matching the classification, case-insensitively.
    pub fn get_all_by_classification(
        conn: &Connection,
        classification: &str,
    ) -> Result<Vec<DataProductVersion>> {
        let mut stmt = conn.prepare(
            "SELECT version_id, data_product_id, version_label, asset_type, source_path,
                    data_quality, classification, created_at
             FROM data_product_versions
             WHERE classification IS NOT NULL
               AND LOWER(classification) = LOWER(?1)
             ORDER BY version_id",
        )?;
        let rows = stmt.query_map(params![classification], Self::from_row)?;

        rows.collect()
    }

    /// Maps database row -> DataProductVersion struct
    fn from_row(row: &Row<'_>) -> Result<DataProductVersion> {
        let created_at: String = row.get("created_at")?;

        Ok(DataProductVersion {
            version_id: row.get("version_id")?,
            data_product_id: row.get("data_product_id")?,
            version_label: row.get("version_label")?,
            asset_type: row.get("asset_type")?,
            source_path: row.get("source_path")?,
            data_quality: row.get("data_quality")?,
            classification: row.get("classification")?,
            created_at: parse_naive_datetime(7, created_at)?,
        })
    }
}
