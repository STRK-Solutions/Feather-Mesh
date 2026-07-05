use rusqlite::{Connection, Result, Row, params};

use crate::models::{NewTeam, Team};

use super::parse_naive_datetime;

pub struct TeamRepository;

impl TeamRepository {
    /// Inserts a new team and returns the persisted team row with database-managed fields.
    pub fn create(conn: &Connection, input: NewTeam) -> Result<Team> {
        conn.execute("INSERT INTO teams (name) VALUES (?1)", params![input.name])?;

        let team_id = conn.last_insert_rowid();
        Self::get_by_id(conn, team_id)
    }

    /// Gets a persisted team by team_id.
    pub fn get_by_id(conn: &Connection, team_id: i64) -> Result<Team> {
        conn.query_row(
            "SELECT team_id, name, created_at FROM teams WHERE team_id = ?1",
            params![team_id],
            Self::from_row,
        )
    }

    pub fn get_by_name(conn: &Connection, name: &str) -> Result<Team> {
        conn.query_row(
            "SELECT team_id, name, created_at FROM teams WHERE lower(name) = lower(?1)",
            params![name],
            Self::from_row,
        )
    }

    /// Gets all persisted teams ordered by primary key.
    pub fn get_all(conn: &Connection) -> Result<Vec<Team>> {
        let mut query =
            conn.prepare("SELECT team_id, name, created_at FROM teams ORDER BY team_id")?;
        let rows = query.query_map([], Self::from_row)?;

        rows.collect()
    }

    /// Maps a database row -> Team struct
    fn from_row(row: &Row<'_>) -> Result<Team> {
        let created_at: String = row.get("created_at")?;

        Ok(Team {
            team_id: row.get("team_id")?,
            name: row.get("name")?,
            created_at: parse_naive_datetime(2, created_at)?,
        })
    }
}
