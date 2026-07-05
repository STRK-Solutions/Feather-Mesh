use rusqlite::{Connection, Result, Row, params};

use crate::models::{DataProduct, NewDataProduct};

use super::parse_naive_datetime;

pub struct DataProductRepository;

impl DataProductRepository {
    /// Inserts a new data product and returns the persisted data product row with database-managed fields.
    pub fn create(conn: &Connection, input: NewDataProduct) -> Result<DataProduct> {
        conn.execute(
            "INSERT INTO data_products
                (name, description, owner_team_id, intended_use, producer, usage_policy)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            params![
                input.name,
                input.description,
                input.owner_team_id,
                input.intended_use,
                input.producer,
                input.usage_policy
            ],
        )?;

        let product_id = conn.last_insert_rowid();
        Self::get_by_id(conn, product_id)
    }

    /// Gets a persisted data product by product_id.
    pub fn get_by_id(conn: &Connection, product_id: i64) -> Result<DataProduct> {
        conn.query_row(
            "SELECT product_id, name, description, owner_team_id, intended_use,
                    producer, usage_policy, created_at
             FROM data_products
             WHERE product_id = ?1",
            params![product_id],
            Self::from_row,
        )
    }

    /// Gets all persisted data products ordered by primary key.
    pub fn get_all(conn: &Connection) -> Result<Vec<DataProduct>> {
        let mut query = conn.prepare(
            "SELECT product_id, name, description, owner_team_id, intended_use,
                    producer, usage_policy, created_at
             FROM data_products
             ORDER BY product_id",
        )?;
        let rows = query.query_map([], Self::from_row)?;

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
            producer: row.get("producer")?,
            usage_policy: row.get("usage_policy")?,
            created_at: parse_naive_datetime(7, created_at)?,
        })
    }
}
