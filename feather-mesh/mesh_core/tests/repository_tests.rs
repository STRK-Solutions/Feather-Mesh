use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

use mesh_core::init_db;
use mesh_core::models::{
    AccessClassification, AssetType, DataQuality, NewDataProduct, NewDataProductVersion,
    NewLineageDependency, NewMetadata, NewTeam,
};
use mesh_core::repositories::{
    DataProductRepository, DataProductVersionRepository, LineageDependencyRepository,
    MetadataRepository, TeamRepository,
};
use rusqlite::Connection;

// Builds an isolated temporary database path for each test.
fn unique_test_db_path() -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time before UNIX EPOCH")
        .as_nanos();
    let mut path = std::env::temp_dir();
    path.push(format!("mesh_core_repository_{}.db", timestamp));
    path
}

// Opens a fresh test database and applies the registry schema.
fn test_connection() -> (Connection, PathBuf) {
    let path = unique_test_db_path();
    let conn = init_db(&path).expect("Failed to initialize repository test database");
    (conn, path)
}

#[test]
// Verifies the team repository can create, fetch by ID, and fetch all persisted teams.
fn team_repository_creates_gets_by_id_and_gets_all_persisted_teams() {
    let (conn, path) = test_connection();

    // Create returns the persisted row, including database-managed fields.
    let saved = TeamRepository::create(&conn, NewTeam::new("Platform".to_string()))
        .expect("Failed to create team");

    assert!(saved.team_id > 0);
    assert_eq!(saved.name, "Platform");

    // get_by_id returns the same row by primary key.
    let found = TeamRepository::get_by_id(&conn, saved.team_id).expect("Failed to get team");
    assert_eq!(found, saved);

    // get_all returns all rows in primary-key order.
    let teams = TeamRepository::get_all(&conn).expect("Failed to get teams");
    assert_eq!(teams, vec![saved]);

    // Remove the temporary database file.
    fs::remove_file(path).ok();
}

#[test]
// Verifies repositories can persist and read a complete registry graph through the minimal API.
fn repositories_create_get_by_id_and_get_all_full_registry_graph() {
    let (conn, path) = test_connection();

    // Start with the owning team required by data products.
    let team = TeamRepository::create(&conn, NewTeam::new("Climate".to_string()))
        .expect("Failed to create team");

    // Create a product with optional descriptive fields populated.
    let product = DataProductRepository::create(
        &conn,
        NewDataProduct::new(
            "Daily Observations".to_string(),
            Some("Daily climate station observations".to_string()),
            team.team_id,
            Some("Operational climate analytics".to_string()),
            "Climate Lab".to_string(),
            "Research and operational analytics".to_string(),
        ),
    )
    .expect("Failed to create data product");

    // Validate the persisted product fields, including database-managed ones.
    assert!(product.product_id > 0);
    assert_eq!(product.owner_team_id, team.team_id);
    assert_eq!(
        product.description,
        Some("Daily climate station observations".to_string())
    );
    assert_eq!(product.producer, "Climate Lab");
    assert_eq!(product.usage_policy, "Research and operational analytics");

    // get_by_id returns the same product by primary key.
    let found_product = DataProductRepository::get_by_id(&conn, product.product_id)
        .expect("Failed to get data product");
    assert_eq!(found_product, product);

    // get_all returns all products in primary-key order.
    let products = DataProductRepository::get_all(&conn).expect("Failed to get products");
    assert_eq!(products, vec![product.clone()]);

    // Create a version linked to the persisted product.
    let version = DataProductVersionRepository::create(
        &conn,
        NewDataProductVersion::new(
            product.product_id,
            "v1.0.0".to_string(),
            AssetType::Table,
            "/project/feather-mesh/climate/daily".to_string(),
            DataQuality::Production,
            Some(AccessClassification::Internal),
        ),
    )
    .expect("Failed to create data product version");

    // Validate the persisted version fields, including database-managed ones.
    assert!(version.version_id > 0);
    assert_eq!(version.data_product_id, product.product_id);
    assert_eq!(version.asset_type, AssetType::Table);
    assert_eq!(version.data_quality, DataQuality::Production);
    assert_eq!(version.classification, Some(AccessClassification::Internal));

    // get_by_id returns the same version by primary key.
    let found_version = DataProductVersionRepository::get_by_id(&conn, version.version_id)
        .expect("Failed to get data product version");
    assert_eq!(found_version, version);

    // get_all returns all versions in primary-key order.
    let versions = DataProductVersionRepository::get_all(&conn).expect("Failed to get versions");
    assert_eq!(versions, vec![version.clone()]);

    // Create metadata linked to the persisted product version.
    let metadata = MetadataRepository::create(
        &conn,
        NewMetadata::new(
            version.version_id,
            Some("quality".to_string()),
            "threshold".to_string(),
            Some("95".to_string()),
            Some("integer".to_string()),
        ),
    )
    .expect("Failed to create metadata");

    // Validate the persisted metadata fields, including database-managed ones.
    assert!(metadata.metadata_id > 0);
    assert_eq!(metadata.data_product_version_id, version.version_id);
    assert_eq!(metadata.namespace, Some("quality".to_string()));
    assert_eq!(metadata.meta_key, "threshold");

    // get_by_id returns the same metadata by primary key.
    let found_metadata =
        MetadataRepository::get_by_id(&conn, metadata.metadata_id).expect("Failed to get metadata");
    assert_eq!(found_metadata, metadata);

    // get_all returns all metadata in primary-key order.
    let metadata_rows = MetadataRepository::get_all(&conn).expect("Failed to get metadata");
    assert_eq!(metadata_rows, vec![metadata.clone()]);

    // Create lineage for the persisted product version.
    let dependency = LineageDependencyRepository::create(
        &conn,
        NewLineageDependency::new(
            version.version_id,
            "mesh://weather/raw-stations".to_string(),
            Some("v2".to_string()),
        ),
    )
    .expect("Failed to create lineage dependency");

    // Validate the persisted dependency fields, including database-managed ones.
    assert!(dependency.dependency_id > 0);
    assert_eq!(dependency.downstream_version_id, version.version_id);
    assert_eq!(dependency.upstream_version, Some("v2".to_string()));

    // get_by_id returns the same dependency by primary key.
    let found_dependency = LineageDependencyRepository::get_by_id(&conn, dependency.dependency_id)
        .expect("Failed to get lineage dependency");
    assert_eq!(found_dependency, dependency);

    // get_all returns all dependencies in primary-key order.
    let dependencies =
        LineageDependencyRepository::get_all(&conn).expect("Failed to get lineage dependencies");
    assert_eq!(dependencies, vec![dependency.clone()]);

    // Remove the temporary database file.
    fs::remove_file(path).ok();
}

#[test]
// Verifies nullable insert fields remain None after database persistence and row mapping.
fn repositories_preserve_none_for_nullable_insert_fields() {
    let (conn, path) = test_connection();

    // Create the minimum valid dependency chain.
    let team = TeamRepository::create(&conn, NewTeam::new("Minimal".to_string()))
        .expect("Failed to create team");
    let product = DataProductRepository::create(
        &conn,
        NewDataProduct::new(
            "Bare Product".to_string(),
            None,
            team.team_id,
            None,
            "Minimal Lab".to_string(),
            "Internal research use".to_string(),
        ),
    )
    .expect("Failed to create data product");
    let version = DataProductVersionRepository::create(
        &conn,
        NewDataProductVersion::new(
            product.product_id,
            "v1".to_string(),
            AssetType::File,
            "/tmp/data.csv".to_string(),
            DataQuality::Unverified,
            None,
        ),
    )
    .expect("Failed to create version");
    let metadata = MetadataRepository::create(
        &conn,
        NewMetadata::new(version.version_id, None, "owner".to_string(), None, None),
    )
    .expect("Failed to create metadata");
    let dependency = LineageDependencyRepository::create(
        &conn,
        NewLineageDependency::new(version.version_id, "mesh://upstream".to_string(), None),
    )
    .expect("Failed to create dependency");

    // Nullable insert fields should round trip as None when omitted.
    assert_eq!(product.description, None);
    assert_eq!(product.intended_use, None);
    assert_eq!(version.classification, None);
    assert_eq!(metadata.namespace, None);
    assert_eq!(metadata.meta_value, None);
    assert_eq!(metadata.value_type, None);
    assert_eq!(dependency.upstream_version, None);

    // Remove the temporary database file.
    fs::remove_file(path).ok();
}
