use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

use mesh_core::init_db;
use mesh_core::models::{AccessClassification, AssetType, DataQuality};
use mesh_core::repositories::{
    DataProductRepository, DataProductVersionRepository, LineageDependencyRepository,
};
use mesh_core::services::{
    CreateDataProductRequest, CreateDataProductVersionRequest, CreateLineageDependencyRequest,
    RegistryService, ServiceError,
};
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
// Verifies product and version metadata are validated and normalized before persistence.
fn registry_service_registers_valid_product_and_version_metadata() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: Some("Daily station observations".to_string()),
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect("Failed to register product");

    assert!(product.product_id > 0);
    assert_eq!(product.producer, "Climate Lab");
    assert_eq!(product.usage_policy, "Research and operational analytics");

    let version = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: product.product_id,
            version_label: "v1.0.0".to_string(),
            asset_type: "Table".to_string(),
            source_path: "/project/feather-mesh/climate/daily".to_string(),
            data_quality: "Production".to_string(),
            classification: "Internal".to_string(),
        })
        .expect("Failed to register version");

    assert!(version.version_id > 0);
    assert_eq!(version.asset_type, AssetType::Table);
    assert_eq!(version.data_quality, DataQuality::Production);
    assert_eq!(version.classification, Some(AccessClassification::Internal));

    fs::remove_file(path).ok();
}

#[test]
// Verifies optional product metadata remains optional, while blank optional strings are rejected.
fn registry_service_preserves_optional_product_metadata() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

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
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect("Failed to register product");

    assert_eq!(product.description, None);
    assert_eq!(product.intended_use, None);

    let err = service
        .register_data_product(CreateDataProductRequest {
            name: "Hourly Observations".to_string(),
            description: Some(" ".to_string()),
            owner_team_id: team.team_id,
            intended_use: None,
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect_err("Blank optional description should fail");

    assert!(matches!(err, ServiceError::Validation(_)));

    fs::remove_file(path).ok();
}

#[test]
// Verifies invalid product metadata is rejected before repository insertion.
fn registry_service_rejects_invalid_product_metadata_before_persistence() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let err = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily/Observations".to_string(),
            description: None,
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect_err("Invalid product name should fail");

    assert!(matches!(err, ServiceError::Validation(_)));
    let products = DataProductRepository::get_all(&conn).expect("Failed to list products");
    assert!(products.is_empty());

    fs::remove_file(path).ok();
}

#[test]
// Verifies invalid version metadata is rejected before repository insertion.
fn registry_service_rejects_invalid_version_metadata_before_persistence() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect("Failed to register product");

    let err = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: product.product_id,
            version_label: "v1.0.0".to_string(),
            asset_type: "table".to_string(),
            source_path: "/project/feather mesh/climate/daily".to_string(),
            data_quality: "production".to_string(),
            classification: "internal".to_string(),
        })
        .expect_err("Invalid source path should fail");

    assert!(matches!(err, ServiceError::Validation(_)));
    let versions = DataProductVersionRepository::get_all(&conn).expect("Failed to list versions");
    assert!(versions.is_empty());

    fs::remove_file(path).ok();
}

#[test]
// Verifies legacy quality tiers are rejected through the service workflow.
fn registry_service_rejects_legacy_data_quality_tiers() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect("Failed to register product");

    for quality in ["gold", "silver", "bronze"] {
        let err = service
            .register_data_product_version(CreateDataProductVersionRequest {
                data_product_id: product.product_id,
                version_label: format!("v1-{quality}"),
                asset_type: "table".to_string(),
                source_path: format!("/project/feather-mesh/climate/{quality}"),
                data_quality: quality.to_string(),
                classification: "internal".to_string(),
            })
            .expect_err("Legacy quality tier should fail");

        assert!(matches!(err, ServiceError::Validation(_)));
    }
    let versions = DataProductVersionRepository::get_all(&conn).expect("Failed to list versions");
    assert!(versions.is_empty());

    fs::remove_file(path).ok();
}

#[test]
// Verifies input dependency metadata is validated and persisted through the service workflow.
fn registry_service_registers_valid_input_dependency_metadata() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let team = service
        .register_team("Climate".to_string())
        .expect("Failed to register team");
    let product = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: team.team_id,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect("Failed to register product");
    let version = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: product.product_id,
            version_label: "v1.0.0".to_string(),
            asset_type: "table".to_string(),
            source_path: "/project/feather-mesh/climate/daily".to_string(),
            data_quality: "production".to_string(),
            classification: "internal".to_string(),
        })
        .expect("Failed to register version");

    let dependency = service
        .register_lineage_dependency(CreateLineageDependencyRequest {
            downstream_version_id: version.version_id,
            upstream_product_uri: "mesh://upstream-climate".to_string(),
            upstream_version: Some("v2.1".to_string()),
        })
        .expect("Failed to register input dependency");

    assert!(dependency.dependency_id > 0);
    assert_eq!(dependency.downstream_version_id, version.version_id);
    assert_eq!(dependency.upstream_product_uri, "mesh://upstream-climate");
    assert_eq!(dependency.upstream_version, Some("v2.1".to_string()));

    fs::remove_file(path).ok();
}

#[test]
// Verifies invalid identifiers and dependency fields are rejected before repository insertion.
fn registry_service_rejects_invalid_ids_and_dependencies_before_persistence() {
    let (conn, path) = test_connection();
    let service = RegistryService::new(&conn);

    let product_err = service
        .register_data_product(CreateDataProductRequest {
            name: "Daily Observations".to_string(),
            description: None,
            owner_team_id: 0,
            intended_use: Some("Operational climate analytics".to_string()),
            producer: "Climate Lab".to_string(),
            usage_policy: "Research and operational analytics".to_string(),
        })
        .expect_err("Invalid owner_team_id should fail");

    assert!(matches!(product_err, ServiceError::Validation(_)));
    assert!(
        DataProductRepository::get_all(&conn)
            .expect("Failed to list products")
            .is_empty()
    );

    let version_err = service
        .register_data_product_version(CreateDataProductVersionRequest {
            data_product_id: -1,
            version_label: "v1.0.0".to_string(),
            asset_type: "table".to_string(),
            source_path: "/project/feather-mesh/climate/daily".to_string(),
            data_quality: "production".to_string(),
            classification: "internal".to_string(),
        })
        .expect_err("Invalid data_product_id should fail");

    assert!(matches!(version_err, ServiceError::Validation(_)));
    assert!(
        DataProductVersionRepository::get_all(&conn)
            .expect("Failed to list versions")
            .is_empty()
    );

    let dependency_err = service
        .register_lineage_dependency(CreateLineageDependencyRequest {
            downstream_version_id: 0,
            upstream_product_uri: "mesh://upstream climate".to_string(),
            upstream_version: Some("v 2".to_string()),
        })
        .expect_err("Invalid dependency metadata should fail");

    assert!(matches!(dependency_err, ServiceError::Validation(_)));
    assert!(
        LineageDependencyRepository::get_all(&conn)
            .expect("Failed to list dependencies")
            .is_empty()
    );

    fs::remove_file(path).ok();
}
