use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

use mesh_core::init_db;
use mesh_core::services::{RegistryService, RegistryServiceError};
use rusqlite::Connection;

// Builds an isolated temporary database path for each test.
fn unique_test_db_path() -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time before UNIX EPOCH")
        .as_nanos();
    let mut path = std::env::temp_dir();
    path.push(format!("mesh_core_service_{}.db", timestamp));
    path
}

// Opens a fresh test database and applies the registry schema.
fn test_connection() -> (Connection, PathBuf) {
    let path = unique_test_db_path();
    let conn = init_db(&path).expect("Failed to initialize service test database");
    (conn, path)
}

#[test]
// Verifies the service exposes a CLI-friendly workflow for registering teams.
fn registry_service_registers_and_lists_teams() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn); // init service instance with test db connection

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");

    assert!(team.team_id > 0);
    assert_eq!(team.name, "Climate");

    let teams = service.list_teams().expect("Failed to list teams");
    assert_eq!(teams, vec![team]);

    fs::remove_file(path).ok();
}

#[test]
// Verifies the service can register a data product without CLI callers touching repositories.
fn registry_service_registers_data_product_for_existing_owner_team() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);
    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");

    let product = service
        .register_data_product(
            "Daily Observations".to_string(),
            Some("Daily climate station observations".to_string()),
            team.team_id,
            Some("Operational climate analytics".to_string()),
        )
        .expect("Failed to register data product");

    assert!(product.product_id > 0);
    assert_eq!(product.name, "Daily Observations");
    assert_eq!(product.owner_team_id, team.team_id);
    assert_eq!(
        product.description,
        Some("Daily climate station observations".to_string())
    );
    assert_eq!(
        product.intended_use,
        Some("Operational climate analytics".to_string())
    );

    fs::remove_file(path).ok();
}

#[test]
// Verifies missing owners produce a stable service-level error before insert.
fn registry_service_rejects_data_product_with_missing_owner_team() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let error = service
        .register_data_product("Daily Observations".to_string(), None, 404, None)
        .expect_err("Missing owner team should fail");

    assert!(matches!(error, RegistryServiceError::MissingOwnerTeam(404)));

    fs::remove_file(path).ok();
}

#[test]
// Verifies clearly invalid input is rejected by the service workflow.
fn registry_service_rejects_data_product_with_invalid_input() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);
    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");

    let empty_name_error = service
        .register_data_product("   ".to_string(), None, team.team_id, None)
        .expect_err("Empty name should fail");
    assert!(matches!(
        empty_name_error,
        RegistryServiceError::InvalidInput(message) if message.contains("name")
    ));

    let invalid_owner_error = service
        .register_data_product("Daily Observations".to_string(), None, 0, None)
        .expect_err("Non-positive owner_team_id should fail");
    assert!(matches!(
        invalid_owner_error,
        RegistryServiceError::InvalidInput(message) if message.contains("owner_team_id")
    ));

    fs::remove_file(path).ok();
}
