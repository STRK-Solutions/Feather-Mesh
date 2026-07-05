use rusqlite::{Connection, Result, Row, params};

use crate::models::{LineageDependency, NewLineageDependency};

pub struct LineageDependencyRepository;

impl LineageDependencyRepository {
    /// Inserts a new lineage dependency and returns the persisted dependency row with database-managed fields populated.
    pub fn create(conn: &Connection, input: NewLineageDependency) -> Result<LineageDependency> {
        conn.execute(
            "INSERT INTO lineage_dependencies
                (downstream_version_id, upstream_product_uri, upstream_version)
             VALUES (?1, ?2, ?3)",
            params![
                input.downstream_version_id,
                input.upstream_product_uri,
                input.upstream_version
            ],
        )?;

        let dependency_id = conn.last_insert_rowid();
        Self::get_by_id(conn, dependency_id)
    }

    /// Gets a persisted lineage dependency by its primary key.
    pub fn get_by_id(conn: &Connection, dependency_id: i64) -> Result<LineageDependency> {
        conn.query_row(
            "SELECT dependency_id, downstream_version_id, upstream_product_uri, upstream_version
             FROM lineage_dependencies
             WHERE dependency_id = ?1",
            params![dependency_id],
            Self::from_row,
        )
    }

    /// Gets all persisted lineage dependencies ordered by primary key.
    pub fn get_all(conn: &Connection) -> Result<Vec<LineageDependency>> {
        let mut stmt = conn.prepare(
            "SELECT dependency_id, downstream_version_id, upstream_product_uri, upstream_version
             FROM lineage_dependencies
             ORDER BY dependency_id",
        )?;
        let rows = stmt.query_map([], Self::from_row)?;

        rows.collect()
    }

    pub fn get_by_downstream_version(
        conn: &Connection,
        downstream_version_id: i64,
    ) -> Result<Vec<LineageDependency>> {
        let mut stmt = conn.prepare(
            "SELECT dependency_id, downstream_version_id, upstream_product_uri, upstream_version
             FROM lineage_dependencies
             WHERE downstream_version_id = ?1
             ORDER BY dependency_id",
        )?;
        let rows = stmt.query_map(params![downstream_version_id], Self::from_row)?;
        rows.collect()
    }

    /// Maps a database row -> LineageDependency struct
    fn from_row(row: &Row<'_>) -> Result<LineageDependency> {
        Ok(LineageDependency {
            dependency_id: row.get("dependency_id")?,
            downstream_version_id: row.get("downstream_version_id")?,
            upstream_product_uri: row.get("upstream_product_uri")?,
            upstream_version: row.get("upstream_version")?,
        })
    }
}
