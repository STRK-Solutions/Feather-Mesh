use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use mesh_core::domain::RegistryError;
use mesh_core::init_db;
use mesh_core::services::{
    CreateDataProductRequest, CreateDataProductVersionRequest, CreateLineageDependencyRequest,
    RegistryService,
};
use rusqlite::Connection;

static DB_COUNTER: AtomicU64 = AtomicU64::new(0);

// Builds an isolated temporary database path for each test.
fn unique_test_db_path() -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time before UNIX EPOCH")
        .as_nanos();
    let mut path = std::env::temp_dir();
    path.push(format!(
        "mesh_core_service_{}_{}_{}.db",
        std::process::id(),
        timestamp,
        DB_COUNTER.fetch_add(1, Ordering::Relaxed)
    ));
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
fn registry_service_compatibility_methods_register_product_version_and_lineage() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);
    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");

    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: Some("Daily climate station observations".to_string()),
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Internal research use".to_string(),
        })
        .expect("Failed to register product");

    let version = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: product.product_id,
            version_label: "v1.0.0".to_string(),
            asset_type: "model-artifact".to_string(),
            source_path: "/tmp/data with spaces/model.bin".to_string(),
            data_quality: "production".to_string(),
            classification: "internal".to_string(),
        })
        .expect("Failed to register version");

    assert_eq!(version.asset_type, "model_artifact");
    assert_eq!(version.source_path, "/tmp/data with spaces/model.bin");

    let dependency = service
        .register_lineage_dependency(CreateLineageDependencyRequest {
            downstream_version_id: version.version_id,
            upstream_product_uri: "/tmp/upstream with spaces.csv".to_string(),
            upstream_version: Some("v2".to_string()),
        })
        .expect("Failed to register lineage");
    assert_eq!(dependency.downstream_version_id, version.version_id);

    fs::remove_file(path).ok();
}

#[test]
fn registry_service_compatibility_methods_reject_missing_owner_and_legacy_quality() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let missing_owner = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: 404,
            intended_use: None,
            producer: "Climate Lab".to_string(),
            usage_policy: "Internal".to_string(),
        })
        .expect_err("missing owner should fail");
    assert!(matches!(missing_owner, RegistryError::NotFound(message) if message.contains("404")));

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: team.team_id,
            intended_use: None,
            producer: "Climate Lab".to_string(),
            usage_policy: "Internal".to_string(),
        })
        .expect("Failed to register product");
    let invalid_quality = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: product.product_id,
            version_label: "v1".to_string(),
            asset_type: "file".to_string(),
            source_path: "/tmp/daily.csv".to_string(),
            data_quality: "gold".to_string(),
            classification: "internal".to_string(),
        })
        .expect_err("legacy quality should fail");
    assert!(
        matches!(invalid_quality, RegistryError::Validation(error) if error.field == "DataQuality")
    );

    fs::remove_file(path).ok();
}
