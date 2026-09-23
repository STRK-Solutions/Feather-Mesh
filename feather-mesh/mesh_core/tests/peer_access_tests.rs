use std::collections::BTreeMap;
use std::fs::{self, File};
use std::path::{Path, PathBuf};
use std::sync::Arc;

use mesh_core::peer::{
    DataFormat, DataKind, DeclaredAsset, PeerError, PeerRouteConfig, Project, PublicationRequest,
    RasterPublication, TablePublication, publish, publish_with_precondition, refresh, resolve,
    stage, withdraw, withdraw_qualified,
};
use mesh_core::services::catalog_service::{CatalogQuery, query_catalog};
use parquet::data_type::Int32Type;
use parquet::file::writer::SerializedFileWriter;
use parquet::schema::parser::parse_message_type;
use tempfile::tempdir;
use tiff::encoder::{TiffEncoder, colortype};
use tiff::tags::Tag;

fn write_parquet(path: &Path, values: &[i32]) {
    let schema = Arc::new(
        parse_message_type(
            "message observations { REQUIRED INT32 station_id; REQUIRED INT32 temperature; }",
        )
        .unwrap(),
    );
    let file = File::create(path).unwrap();
    let mut writer = SerializedFileWriter::new(file, schema, Default::default()).unwrap();
    let mut row_group = writer.next_row_group().unwrap();
    let mut station = row_group.next_column().unwrap().unwrap();
    station
        .typed::<Int32Type>()
        .write_batch(values, None, None)
        .unwrap();
    station.close().unwrap();
    let mut temperature = row_group.next_column().unwrap().unwrap();
    temperature
        .typed::<Int32Type>()
        .write_batch(
            &values.iter().map(|value| value + 10).collect::<Vec<_>>(),
            None,
            None,
        )
        .unwrap();
    temperature.close().unwrap();
    row_group.close().unwrap();
    writer.close().unwrap();
}

fn write_geotiff(path: &Path) {
    let file = File::create(path).unwrap();
    let mut encoder = TiffEncoder::new(file).unwrap();
    let mut image = encoder.new_image::<colortype::Gray8>(2, 2).unwrap();
    image
        .encoder()
        .write_tag(
            Tag::GeoKeyDirectoryTag,
            &[1_u16, 1, 0, 1, 1024, 0, 1, 2][..],
        )
        .unwrap();
    image
        .encoder()
        .write_tag(Tag::ModelPixelScaleTag, &[1_f64, 1_f64, 0_f64][..])
        .unwrap();
    image.write_data(&[7_u8, 8, 9, 10]).unwrap();
}

fn table_request() -> PublicationRequest {
    PublicationRequest {
        schema_version: 1,
        namespace: "climate".into(),
        product_id: "observations".into(),
        name: "Daily observations".into(),
        version: "v1".into(),
        data_kind: DataKind::Table,
        data_format: DataFormat::Parquet,
        description: "Small registered observations fixture".into(),
        intended_use: "test direct reads".into(),
        limitations: "none".into(),
        owner_team: "climate".into(),
        producer: "Climate Lab".into(),
        contact: "climate@example.test".into(),
        usage_policy: "internal".into(),
        classification: "internal".into(),
        quality: "production".into(),
        assets: vec![
            DeclaredAsset {
                id: "part-000".into(),
                path: PathBuf::from("datasets/observations/v1/part-000.parquet"),
                role: "data".into(),
                media_type: "application/vnd.apache.parquet".into(),
                digest_opt_out: false,
            },
            DeclaredAsset {
                id: "part-001".into(),
                path: PathBuf::from("datasets/observations/v1/part-001.parquet"),
                role: "data".into(),
                media_type: "application/vnd.apache.parquet".into(),
                digest_opt_out: false,
            },
        ],
        lineage: vec![],
        table: Some(TablePublication {
            column_meanings: BTreeMap::from([
                ("station_id".into(), "station identifier".into()),
                ("temperature".into(), "daily temperature".into()),
            ]),
            column_units: BTreeMap::from([
                ("station_id".into(), "not_applicable".into()),
                ("temperature".into(), "celsius".into()),
            ]),
            partition_columns: vec![],
        }),
        raster: None,
    }
}

fn configure_client(client: &Project, provider_serving: &Path) {
    let peer_dir = client.root().join("peers");
    fs::create_dir_all(&peer_dir).unwrap();
    #[cfg(unix)]
    std::os::unix::fs::symlink(provider_serving, peer_dir.join("nearby-climate")).unwrap();
    let mut config = client.config().clone();
    config.peers.push(PeerRouteConfig {
        alias: "nearby-climate".into(),
        namespace: "climate".into(),
        path: PathBuf::from("peers/nearby-climate"),
    });
    fs::write(
        client.config_path(),
        toml::to_string_pretty(&config).unwrap(),
    )
    .unwrap();
}

#[test]
fn publishes_fixed_inventory_resolves_through_client_route_and_stages_safely() {
    let temp = tempdir().unwrap();
    let provider = Project::init(
        temp.path().join("provider"),
        "climate".into(),
        Some(PathBuf::from("serving")),
    )
    .unwrap();
    let serving = provider.serving_root().unwrap();
    let data = serving.join("datasets/observations/v1");
    fs::create_dir_all(&data).unwrap();
    write_parquet(&data.join("part-000.parquet"), &[1, 2]);
    write_parquet(&data.join("part-001.parquet"), &[3, 4]);
    write_parquet(&data.join("unregistered.parquet"), &[99]);

    let response = publish(&provider, &table_request()).unwrap();
    assert_eq!(response.manifest_revision, 1);
    assert!(matches!(
        publish(&provider, &table_request()),
        Err(PeerError::Conflict(_))
    ));

    let client = Project::init(temp.path().join("client"), "consumer".into(), None).unwrap();
    configure_client(&client, &serving);
    let cache = refresh(&Project::open(client.root()).unwrap()).unwrap();
    assert_eq!(cache.peers[0].revision, Some(1));

    let client = Project::open(client.root()).unwrap();
    let catalog = query_catalog(
        &client,
        CatalogQuery {
            limit: 10,
            ..Default::default()
        },
    )
    .unwrap();
    assert_eq!(catalog.entries.len(), 1);
    assert_eq!(catalog.coverage.len(), 1);
    assert!(catalog.coverage[0].available);
    assert_eq!(
        catalog.entries[0].reference,
        "product://climate/observations"
    );
    let resolved = resolve(&client, "product://climate/observations", "v1", None, true).unwrap();
    assert_eq!(resolved.assets.len(), 2);
    assert!(resolved.assets.iter().all(|asset| {
        asset
            .project_access_path
            .starts_with(client.root().join("peers/nearby-climate"))
    }));
    assert!(
        !resolved
            .assets
            .iter()
            .any(|asset| asset.project_access_path.ends_with("unregistered.parquet"))
    );
    // Cache loss cannot become an access bypass: direct resolution reopens the
    // configured route and manifest rather than trusting a saved physical path.
    fs::remove_file(client.cache_path()).unwrap();
    assert_eq!(
        resolve(&client, "product://climate/observations", "v1", None, false)
            .unwrap()
            .assets
            .len(),
        2
    );

    let output = temp.path().join("staged");
    let receipt = stage(&resolved, &output, false).unwrap();
    assert!(output.join("part-000").exists());
    assert_eq!(receipt.assets.len(), 2);
    assert!(output.with_file_name("staged.feam-receipt.json").exists());
    let one = resolve(
        &client,
        "product://climate/observations",
        "v1",
        Some("part-000"),
        false,
    )
    .unwrap();
    assert!(matches!(
        stage(&one, &one.assets[0].project_access_path, true),
        Err(PeerError::Policy(_))
    ));
    let protected_output = temp.path().join("protected-output");
    fs::write(&protected_output, "prior bytes").unwrap();
    fs::create_dir(protected_output.with_file_name("protected-output.feam-receipt.json")).unwrap();
    assert!(stage(&one, &protected_output, true).is_err());
    assert_eq!(
        fs::read_to_string(&protected_output).unwrap(),
        "prior bytes"
    );

    // A process may still have a readable physical path, but a removed client
    // route must deny new resolver access.
    #[cfg(unix)]
    fs::remove_file(client.root().join("peers/nearby-climate")).unwrap();
    #[cfg(unix)]
    assert!(matches!(
        resolve(&client, "product://climate/observations", "v1", None, false),
        Err(PeerError::PeerUnavailable(_))
    ));

    withdraw(&provider, "observations", "v1", "superseded").unwrap();
    assert!(matches!(
        resolve(
            &provider,
            "product://climate/observations",
            "v1",
            None,
            false
        ),
        Err(PeerError::Withdrawn(_))
    ));
    assert!(matches!(
        withdraw_qualified(
            &provider,
            "product://other/observations",
            "v1",
            "wrong namespace",
            None,
        ),
        Err(PeerError::Policy(_))
    ));
}

#[test]
fn rejects_fake_parquet_and_accepts_geotiff_with_required_context() {
    let temp = tempdir().unwrap();
    let project = Project::init(
        temp.path().join("provider"),
        "climate".into(),
        Some(PathBuf::from("serving")),
    )
    .unwrap();
    let serving = project.serving_root().unwrap();
    fs::create_dir_all(serving.join("datasets/raster/v1")).unwrap();
    let mut unauthorized = table_request();
    unauthorized.owner_team = "unconfigured-team".into();
    assert!(matches!(
        publish(&project, &unauthorized),
        Err(PeerError::Policy(_))
    ));
    fs::write(
        serving.join("datasets/raster/v1/not-parquet.parquet"),
        "date,value\n2026-01-01,42\n",
    )
    .unwrap();
    let mut invalid = table_request();
    invalid.assets.truncate(1);
    invalid.assets[0].path = PathBuf::from("datasets/raster/v1/not-parquet.parquet");
    assert!(matches!(
        publish(&project, &invalid),
        Err(PeerError::Parquet(_))
    ));
    assert!(!serving.join("manifest.json").exists());

    write_geotiff(&serving.join("datasets/raster/v1/temperature.tiff"));
    let request = PublicationRequest {
        schema_version: 1,
        namespace: "climate".into(),
        product_id: "temperature".into(),
        name: "Temperature".into(),
        version: "v1".into(),
        data_kind: DataKind::Raster,
        data_format: DataFormat::Geotiff,
        description: "raster".into(),
        intended_use: "window test".into(),
        limitations: "none".into(),
        owner_team: "climate".into(),
        producer: "Climate Lab".into(),
        contact: "climate@example.test".into(),
        usage_policy: "internal".into(),
        classification: "internal".into(),
        quality: "production".into(),
        assets: vec![DeclaredAsset {
            id: "data".into(),
            path: PathBuf::from("datasets/raster/v1/temperature.tiff"),
            role: "data".into(),
            media_type: "image/tiff; application=geotiff".into(),
            digest_opt_out: false,
        }],
        lineage: vec![],
        table: None,
        raster: Some(RasterPublication {
            datetime: "2026-01-01T00:00:00Z".into(),
            bbox: [-76.0, 45.0, -75.0, 46.0],
            semantics: BTreeMap::from([("band_1".into(), "temperature".into())]),
        }),
    };
    let response = publish(&project, &request).unwrap();
    assert_eq!(response.status, "published");
    let mut changed = request.clone();
    changed.version = "v2".into();
    assert!(matches!(
        publish_with_precondition(&project, &changed, Some(99)),
        Err(PeerError::Conflict(_))
    ));
    let manifest: serde_json::Value =
        serde_json::from_slice(&fs::read(serving.join("manifest.json")).unwrap()).unwrap();
    assert_eq!(manifest["revision"], 1);
}

fn review_fixture() -> (tempfile::TempDir, Project, PublicationRequest) {
    let temp = tempdir().unwrap();
    let project = Project::init(
        temp.path().join("provider"),
        "climate".into(),
        Some("serving".into()),
    )
    .unwrap();
    let request = table_request();
    let data = project
        .serving_root()
        .unwrap()
        .join("datasets/observations/v1");
    fs::create_dir_all(&data).unwrap();
    write_parquet(&data.join("part-000.parquet"), &[1, 2]);
    write_parquet(&data.join("part-001.parquet"), &[3, 4]);
    (temp, project, request)
}

#[test]
fn reviews_reject_changed_drafts_inventory_destinations_and_receipts() {
    use mesh_core::services::interactive_operations::*;
    let (temp, project, request) = review_fixture();
    let prepared = prepare_publication(project.root(), request.clone()).unwrap();
    let mut edited = prepared.clone();
    edited.request.description = "changed after review".into();
    assert_eq!(
        execute_publication(&edited).state,
        OperationState::FailedBeforeCommit
    );
    let source = project
        .serving_root()
        .unwrap()
        .join(&request.assets[0].path);
    write_parquet(&source, &[9, 8]);
    assert_eq!(
        execute_publication(&prepared).state,
        OperationState::FailedBeforeCommit
    );
    assert!(
        !project
            .serving_root()
            .unwrap()
            .join("manifest.json")
            .exists()
    );
    let fresh = prepare_publication(project.root(), request).unwrap();
    assert_eq!(execute_publication(&fresh).state, OperationState::Committed);
    assert_eq!(
        reconcile(&RecoveryIdentity::from(&fresh)).unwrap(),
        OperationState::Committed
    );
    assert_eq!(
        execute_publication(&fresh).state,
        OperationState::FailedBeforeCommit
    );
    let output = temp.path().join("output with spaces");
    let stage = prepare_stage(
        project.root(),
        "product://climate/observations",
        "v1",
        &output,
        true,
    )
    .unwrap();
    fs::write(&output, "new destination").unwrap();
    assert_eq!(
        execute_stage(&stage).state,
        OperationState::FailedBeforeCommit
    );
    assert_eq!(fs::read(&output).unwrap(), b"new destination");
    let stage = prepare_stage(
        project.root(),
        "product://climate/observations",
        "v1",
        &output,
        true,
    )
    .unwrap();
    fs::write(&stage.receipt_path, "new receipt").unwrap();
    assert_eq!(
        execute_stage(&stage).state,
        OperationState::FailedBeforeCommit
    );
    assert_eq!(fs::read(&stage.receipt_path).unwrap(), b"new receipt");
    let stage = prepare_stage(
        project.root(),
        "product://climate/observations",
        "v1",
        &output,
        true,
    )
    .unwrap();
    let original = fs::read(&source).unwrap();
    assert_eq!(execute_stage(&stage).state, OperationState::Committed);
    assert_eq!(fs::read(&source).unwrap(), original);
    assert_eq!(
        reconcile(&RecoveryIdentity::from(&stage)).unwrap(),
        OperationState::Committed
    );
    assert_eq!(
        execute_stage(&stage).state,
        OperationState::FailedBeforeCommit
    );
    let withdrawal = prepare_withdrawal(
        project.root(),
        "product://climate/observations",
        "v1",
        "superseded",
    )
    .unwrap();
    assert_eq!(
        execute_withdrawal(&withdrawal).state,
        OperationState::Committed
    );
    assert_eq!(
        reconcile(&RecoveryIdentity::from(&withdrawal)).unwrap(),
        OperationState::Committed
    );
}

#[test]
fn catalog_cursors_bind_filters_configuration_and_revisions() {
    let (_temp, project, request) = review_fixture();
    publish(&project, &request).unwrap();
    let mut second = request.clone();
    second.version = "v2".into();
    publish(&project, &second).unwrap();
    let first = query_catalog(
        &project,
        CatalogQuery {
            limit: 1,
            ..Default::default()
        },
    )
    .unwrap();
    let cursor = first.next_cursor.unwrap();
    let next = query_catalog(
        &project,
        CatalogQuery {
            limit: 1,
            cursor: Some(cursor.clone()),
            ..Default::default()
        },
    )
    .unwrap();
    assert_eq!(next.entries[0].version, "v2");
    assert!(
        query_catalog(
            &project,
            CatalogQuery {
                limit: 1,
                text: Some("different".into()),
                cursor: Some(cursor.clone()),
                ..Default::default()
            }
        )
        .is_err()
    );
    let mut config = project.config().clone();
    config.owner_teams.push("new-team".into());
    fs::write(project.config_path(), toml::to_string(&config).unwrap()).unwrap();
    assert!(
        query_catalog(
            &Project::open(project.root()).unwrap(),
            CatalogQuery {
                limit: 1,
                cursor: Some(cursor),
                ..Default::default()
            }
        )
        .is_err()
    );
    assert!(
        query_catalog(
            &project,
            CatalogQuery {
                limit: 1,
                cursor: Some("v1:éé".into()),
                ..Default::default()
            }
        )
        .is_err()
    );
    let page = query_catalog(
        &project,
        CatalogQuery {
            limit: 1,
            ..Default::default()
        },
    )
    .unwrap();
    second.version = "v3".into();
    publish(&Project::open(project.root()).unwrap(), &second).unwrap();
    assert!(
        query_catalog(
            &project,
            CatalogQuery {
                limit: 1,
                cursor: page.next_cursor,
                ..Default::default()
            }
        )
        .is_err()
    );
}

#[test]
fn oversized_manifest_reports_failed_coverage_and_stage_preflight_preserves_aliases() {
    use mesh_core::services::interactive_operations::prepare_stage;
    let (temp, project, request) = review_fixture();
    publish(&project, &request).unwrap();
    let source = project
        .serving_root()
        .unwrap()
        .join(&request.assets[0].path);
    let source_bytes = fs::read(&source).unwrap();
    let output = temp.path().join("output");
    fs::hard_link(&source, &output).unwrap();
    assert!(
        prepare_stage(
            project.root(),
            "product://climate/observations",
            "v1",
            &output,
            true
        )
        .is_err()
    );
    fs::remove_file(&output).unwrap();
    fs::hard_link(&source, output.with_file_name("output.feam-receipt.json")).unwrap();
    assert!(
        prepare_stage(
            project.root(),
            "product://climate/observations",
            "v1",
            &output,
            true
        )
        .is_err()
    );
    assert_eq!(fs::read(&source).unwrap(), source_bytes);
    let manifest = project.serving_root().unwrap().join("manifest.json");
    File::create(manifest)
        .unwrap()
        .set_len(mesh_core::peer::MAX_MANIFEST_BYTES + 1)
        .unwrap();
    let page = query_catalog(
        &project,
        CatalogQuery {
            limit: 1,
            ..Default::default()
        },
    )
    .unwrap();
    assert!(page.entries.is_empty());
    assert!(!page.coverage[0].available);
    assert!(page.coverage[0].error.as_ref().unwrap().contains("exceeds"));
}

#[test]
fn changed_route_invalidates_prepared_access_even_at_same_revision() {
    use mesh_core::services::interactive_operations::*;
    let (temp, project, request) = review_fixture();
    publish(&project, &request).unwrap();
    let client = Project::init(temp.path().join("client"), "consumer".into(), None).unwrap();
    configure_client(&client, &project.serving_root().unwrap());
    let output = temp.path().join("output");
    let prepared = prepare_stage(
        client.root(),
        "product://climate/observations",
        "v1",
        &output,
        false,
    )
    .unwrap();
    let route = client.root().join("peers/nearby-climate");
    fs::remove_file(route).unwrap();
    assert_ne!(execute_stage(&prepared).state, OperationState::Committed);
    assert!(!output.exists());
    assert!(
        project
            .serving_root()
            .unwrap()
            .join(&request.assets[0].path)
            .is_file()
    );
}
