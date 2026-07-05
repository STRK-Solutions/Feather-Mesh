use rusqlite::{Connection, Result, Row, params};

use crate::models::{DataProduct, NewDataProduct};

use super::parse_naive_datetime;

pub struct DataProductRepository;

impl DataProductRepository {
    /// Inserts a new data product and returns the persisted data product row with database-managed fields.
    pub fn create(conn: &Connection, input: NewDataProduct) -> Result<DataProduct> {
        conn.execute(
            "INSERT INTO data_products (name, description, owner_team_id, intended_use)
             VALUES (?1, ?2, ?3, ?4)",
            params![
                input.name,
                input.description,
                input.owner_team_id,
                input.intended_use
            ],
        )?;

        let product_id = conn.last_insert_rowid();
        Self::get_by_id(conn, product_id)
    }

    /// Gets a persisted data product by product_id.
    pub fn get_by_id(conn: &Connection, product_id: i64) -> Result<DataProduct> {
        conn.query_row(
            "SELECT product_id, name, description, owner_team_id, intended_use, created_at
             FROM data_products
             WHERE product_id = ?1",
            params![product_id],
            Self::from_row,
        )
    }

    /// Gets all persisted data products ordered by primary key.
    pub fn get_all(conn: &Connection) -> Result<Vec<DataProduct>> {
        let mut query = conn.prepare(
            "SELECT product_id, name, description, owner_team_id, intended_use, created_at
             FROM data_products
             ORDER BY product_id",
        )?;
        let rows = query.query_map([], Self::from_row)?;

        rows.collect()
    }

    /// Gets all persisted data products with at least one version matching the asset type, case-insensitively.
    pub fn get_all_by_asset_type(conn: &Connection, asset_type: &str) -> Result<Vec<DataProduct>> {
        let mut query = conn.prepare(
            "SELECT DISTINCT p.product_id, p.name, p.description, p.owner_team_id, p.intended_use, p.created_at
             FROM data_products p
             INNER JOIN data_product_versions v ON v.data_product_id = p.product_id
             WHERE LOWER(v.asset_type) = LOWER(?1)
             ORDER BY p.product_id",
        )?;
        let rows = query.query_map(params![asset_type], Self::from_row)?;

        rows.collect()
    }

    /// Gets all persisted data products with at least one version matching the data quality, case-insensitively.
    pub fn get_all_by_data_quality(
        conn: &Connection,
        data_quality: &str,
    ) -> Result<Vec<DataProduct>> {
        let mut query = conn.prepare(
            "SELECT DISTINCT p.product_id, p.name, p.description, p.owner_team_id, p.intended_use, p.created_at
             FROM data_products p
             INNER JOIN data_product_versions v ON v.data_product_id = p.product_id
             WHERE LOWER(v.data_quality) = LOWER(?1)
             ORDER BY p.product_id",
        )?;
        let rows = query.query_map(params![data_quality], Self::from_row)?;

        rows.collect()
    }

    /// Gets all persisted data products with at least one version matching the classification, case-insensitively.
    pub fn get_all_by_classification(
        conn: &Connection,
        classification: &str,
    ) -> Result<Vec<DataProduct>> {
        let mut query = conn.prepare(
            "SELECT DISTINCT p.product_id, p.name, p.description, p.owner_team_id, p.intended_use, p.created_at
             FROM data_products p
             INNER JOIN data_product_versions v ON v.data_product_id = p.product_id
             WHERE v.classification IS NOT NULL
               AND LOWER(v.classification) = LOWER(?1)
             ORDER BY p.product_id",
        )?;
        let rows = query.query_map(params![classification], Self::from_row)?;

        rows.collect()
    }

    /// Maps a database row -> DataProduct struct
    fn from_row(row: &Row<'_>) -> Result<DataProduct> {
        let created_at: String = row.get("created_at")?;

        Ok(DataProduct {
            product_id: row.get("product_id")?,
            name: row.get("name")?,
            description: row.get("description")?,
            owner_team_id: row.get("owner_team_id")?,
            intended_use: row.get("intended_use")?,
            created_at: parse_naive_datetime(5, created_at)?,
        })
    }
}
