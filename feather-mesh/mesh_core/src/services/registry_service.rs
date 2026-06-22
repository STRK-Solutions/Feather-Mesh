use std::fmt;

use rusqlite::Connection;

use crate::models::{DataProduct, NewDataProduct, NewTeam, Team};
use crate::repositories::{DataProductRepository, TeamRepository};

// RegistryService exposes API-style registry workflows for `mesh_cli`.

#[derive(Debug)]
pub enum RegistryServiceError {
    InvalidInput(String),
    MissingOwnerTeam(i64),
    Database(rusqlite::Error),
}

impl fmt::Display for RegistryServiceError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidInput(message) => write!(formatter, "invalid input: {message}"),
            Self::MissingOwnerTeam(team_id) => {
                write!(formatter, "owner team {team_id} does not exist")
            }
            Self::Database(error) => write!(formatter, "{error}"),
        }
    }
}

impl std::error::Error for RegistryServiceError {}

impl From<rusqlite::Error> for RegistryServiceError {
    fn from(error: rusqlite::Error) -> Self {
        Self::Database(error)
    }
}

pub type RegistryServiceResult<T> = std::result::Result<T, RegistryServiceError>;

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
    pub fn register_team(&self, name: String) -> rusqlite::Result<Team> {
        TeamRepository::create(self.conn, NewTeam::new(name))
    }

    /// Registers a lab-owned data product after validating the owning team exists.
    pub fn register_data_product(
        &self,
        name: String,
        description: Option<String>,
        owner_team_id: i64,
        intended_use: Option<String>,
    ) -> RegistryServiceResult<DataProduct> {
        let name = name.trim().to_string();
        if name.is_empty() {
            return Err(RegistryServiceError::InvalidInput(
                "data product name must not be empty".to_string(),
            ));
        }

        if owner_team_id <= 0 {
            return Err(RegistryServiceError::InvalidInput(
                "owner_team_id must be a positive integer".to_string(),
            ));
        }

        match TeamRepository::get_by_id(self.conn, owner_team_id) {
            Ok(_) => {}
            Err(rusqlite::Error::QueryReturnedNoRows) => {
                return Err(RegistryServiceError::MissingOwnerTeam(owner_team_id));
            }
            Err(error) => return Err(error.into()),
        }

        DataProductRepository::create(
            self.conn,
            NewDataProduct::new(name, description, owner_team_id, intended_use),
        )
        .map_err(RegistryServiceError::from)
    }

    /// Returns all teams currently registered in the registry.
    pub fn list_teams(&self) -> rusqlite::Result<Vec<Team>> {
        TeamRepository::get_all(self.conn)
    }
}
