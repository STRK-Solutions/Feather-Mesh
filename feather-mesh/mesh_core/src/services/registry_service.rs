use rusqlite::{Connection, Result};

use crate::models::{NewTeam, Team};
use crate::repositories::TeamRepository;

// RegistryService exposes API-style registry workflows for `mesh_cli`.

// note: `a -> lifetime of the connection reference, ensures registry service does not outlive the db connection it uses
pub struct RegistryService<'a> {
    conn: &'a Connection,
}

impl<'a> RegistryService<'a> {
    /// Creates a registry service backed by an existing database connection.
    pub fn new(conn: &'a Connection) -> Self {
        Self { conn }
    }

    /// Registers a team in the registry and returns the persisted team.
    pub fn register_team(&self, name: String) -> Result<Team> {
        TeamRepository::create(self.conn, NewTeam::new(name))
    }

    /// Returns all teams currently registered in the registry.
    pub fn list_teams(&self) -> Result<Vec<Team>> {
        TeamRepository::get_all(self.conn)
    }
}
