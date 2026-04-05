use std::path::Path;

use rusqlite::{Connection, Result};

pub const DEFAULT_DB_FILENAME: &str = "registry.db";

/// Initialize the SQLite database at the default path `registry.db`.
///
/// This creates the file if it does not exist, enables WAL mode, turns on foreign keys,
/// and then creates the initial schema in an idempotent way.
pub fn init_default_db() -> Result<Connection> {
    init_db(Path::new(DEFAULT_DB_FILENAME))
}

/// Initialize the SQLite database at the provided filesystem path.
///
/// The schema creation is idempotent, so repeated calls on the same file are safe.
pub fn init_db<P: AsRef<Path>>(path: P) -> Result<Connection> {
    let mut conn = Connection::open(path)?;
    conn.pragma_update(None, "journal_mode", "WAL")?;
    conn.pragma_update(None, "foreign_keys", 1)?;

    let txn = conn.transaction()?;
    init_schema(&txn)?;
    txn.commit()?;

    Ok(conn)
}

fn init_schema(conn: &Connection) -> Result<()> {
    conn.execute_batch(
        r#"
        CREATE TABLE IF NOT EXISTS teams (
            team_id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            description TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (datetime('now'))
        );

        CREATE TABLE IF NOT EXISTS data_products (
            product_id INTEGER PRIMARY KEY AUTOINCREMENT,
            owner_team_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            description TEXT NOT NULL,
            data_format TEXT NOT NULL,
            access_uri TEXT NOT NULL,
            status TEXT NOT NULL,
            classification TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (datetime('now')),
            updated_at TEXT NOT NULL DEFAULT (datetime('now')),
            FOREIGN KEY(owner_team_id) REFERENCES teams(team_id) ON DELETE CASCADE
        );

        CREATE TABLE IF NOT EXISTS metadata (
            metadata_id INTEGER PRIMARY KEY AUTOINCREMENT,
            data_product_id INTEGER NOT NULL,
            namespace TEXT NOT NULL,
            meta_key TEXT NOT NULL,
            meta_value TEXT NOT NULL,
            value_type TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (datetime('now')),
            FOREIGN KEY(data_product_id) REFERENCES data_products(product_id) ON DELETE CASCADE
        );

        CREATE INDEX IF NOT EXISTS idx_data_products_owner_team_id ON data_products(owner_team_id);
        CREATE INDEX IF NOT EXISTS idx_metadata_data_product_id ON metadata(data_product_id);
        "#,
    )?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn unique_test_db_path() -> PathBuf {
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("System time before UNIX EPOCH")
            .as_nanos();
        let mut path = std::env::temp_dir();
        path.push(format!("mesh_core_registry_{}.db", timestamp));
        path
    }

    #[test]
    fn init_db_creates_registry_db_and_enables_wal() {
        let path = unique_test_db_path();
        if path.exists() {
            fs::remove_file(&path).unwrap();
        }

        let conn = init_db(&path).expect("Failed to initialize the test database");

        assert!(path.exists(), "Database file should be created");

        let journal_mode: String = conn
            .query_row("PRAGMA journal_mode;", [], |row| row.get(0))
            .expect("Failed to read journal_mode");
        assert_eq!(journal_mode.to_lowercase(), "wal");

        let table_count: i32 = conn
            .query_row(
                "SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name IN ('teams','data_products','metadata');",
                [],
                |row| row.get(0),
            )
            .expect("Failed to verify tables");
        assert_eq!(table_count, 3, "All expected tables should exist");

        fs::remove_file(&path).ok();
    }

    #[test]
    fn init_db_is_idempotent() {
        let path = unique_test_db_path();
        if path.exists() {
            fs::remove_file(&path).unwrap();
        }

        init_db(&path).expect("First init should succeed");
        init_db(&path).expect("Second init should still succeed");

        fs::remove_file(&path).ok();
    }
}
