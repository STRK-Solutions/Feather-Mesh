use std::fs;
use std::path::Path;
use std::sync::Arc;

use assert_cmd::Command;
use parquet::data_type::Int32Type;
use parquet::file::writer::SerializedFileWriter;
use parquet::schema::parser::parse_message_type;
use predicates::prelude::*;
use serde_json::Value;
use tempfile::tempdir;

fn feam() -> Command {
    Command::cargo_bin("mesh_cli").expect("binary exists")
}

fn registry_arg(path: &Path) -> String {
    path.to_string_lossy().into_owned()
}

fn write_parquet(path: &Path, values: &[i32]) {
    let schema = Arc::new(
        parse_message_type(
            "message observations { REQUIRED INT32 station_id; REQUIRED INT32 temperature; }",
        )
        .unwrap(),
    );
    let mut writer =
        SerializedFileWriter::new(fs::File::create(path).unwrap(), schema, Default::default())
            .unwrap();
    let mut group = writer.next_row_group().unwrap();
    let mut station = group.next_column().unwrap().unwrap();
    station
        .typed::<Int32Type>()
        .write_batch(values, None, None)
        .unwrap();
    station.close().unwrap();
    let mut temperature = group.next_column().unwrap().unwrap();
    temperature
        .typed::<Int32Type>()
        .write_batch(
            &values.iter().map(|value| value + 10).collect::<Vec<_>>(),
            None,
            None,
        )
        .unwrap();
    temperature.close().unwrap();
    group.close().unwrap();
    writer.close().unwrap();
}

#[test]
fn help_shows_canonical_commands() {
    feam()
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains("serve"))
        .stdout(predicate::str::contains("consume"))
        .stdout(predicate::str::contains("validate-metadata"));
}

#[test]
fn tui_requires_an_explicit_project_and_never_creates_a_legacy_registry() {
    let temp = tempdir().unwrap();
    feam()
        .current_dir(temp.path())
        .args(["tui"])
        .assert()
        .code(1)
        .stderr(predicate::str::contains("requires --project"));
    assert!(!temp.path().join("registry.db").exists());

    let assertion = feam()
        .args(["--project", &registry_arg(temp.path()), "tui"])
        .assert()
        .code(1);
    #[cfg(feature = "tui")]
    let assertion = assertion.stderr(predicate::str::contains("interactive terminal"));
    #[cfg(not(feature = "tui"))]
    let assertion = assertion.stderr(predicate::str::contains("not enabled"));
    let _ = assertion;
}

#[test]
fn primary_demo_workflow_works_with_json_outputs_and_receipt() {
    let temp = tempdir().unwrap();
    let registry = temp.path().join("registry.db");
    let source = temp.path().join("daily.csv");
    let out = temp.path().join("copy.csv");
    fs::write(&source, "date,value\n2026-07-05,42\n").unwrap();

    feam()
        .args(["--registry", &registry_arg(&registry), "init"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Registry initialized"));

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "serve",
            &registry_arg(&source),
            "--name",
            "Daily Observations",
            "--asset-type",
            "file",
            "--version",
            "v1.0.0",
            "--owner-team",
            "Climate",
            "--producer",
            "Climate Lab",
            "--usage-policy",
            "Internal research use",
            "--data-quality",
            "production",
            "--classification",
            "internal",
        ])
        .assert()
        .success()
        .stdout(predicate::str::contains("product_id: 1"));

    let search = feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "--format",
            "json",
            "search",
            "daily",
        ])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let search_json: Value = serde_json::from_slice(&search).unwrap();
    assert_eq!(search_json[0]["name"], "Daily Observations");

    let show = feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "--format",
            "json",
            "show",
            "1",
        ])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let show_json: Value = serde_json::from_slice(&show).unwrap();
    assert_eq!(show_json["selected_version"]["version"], "v1.0.0");

    let lineage = feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "--format",
            "json",
            "lineage",
            "1",
        ])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let lineage_json: Value = serde_json::from_slice(&lineage).unwrap();
    assert_eq!(lineage_json["status"], "no_lineage");

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "consume",
            "1",
            "--version",
            "v1.0.0",
            "--out",
            &registry_arg(&out),
        ])
        .assert()
        .success();

    assert_eq!(
        fs::read_to_string(&out).unwrap(),
        "date,value\n2026-07-05,42\n"
    );
    let receipt = out.with_file_name("copy.csv.feam-receipt.json");
    let receipt_json: Value = serde_json::from_str(&fs::read_to_string(receipt).unwrap()).unwrap();
    assert_eq!(receipt_json["product_id"], 1);
    assert!(receipt_json["checksum"].is_null());
}

#[test]
fn validation_not_found_and_policy_exit_codes_are_stable() {
    let temp = tempdir().unwrap();
    let registry = temp.path().join("registry.db");
    let source = temp.path().join("daily.csv");
    let out = temp.path().join("copy.csv");
    fs::write(&source, "x\n").unwrap();
    fs::write(&out, "existing\n").unwrap();

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "serve",
            &registry_arg(&source),
            "--name",
            "Bad/Name",
            "--asset-type",
            "file",
            "--version",
            "v1",
            "--owner-team",
            "Climate",
            "--producer",
            "Climate Lab",
            "--usage-policy",
            "Internal",
            "--data-quality",
            "production",
            "--classification",
            "internal",
        ])
        .assert()
        .code(3)
        .stderr(predicate::str::contains("name"));

    feam()
        .args(["--registry", &registry_arg(&registry), "show", "999"])
        .assert()
        .code(4);

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "serve",
            &registry_arg(&source),
            "--name",
            "Daily",
            "--asset-type",
            "file",
            "--version",
            "v1",
            "--owner-team",
            "Climate",
            "--producer",
            "Climate Lab",
            "--usage-policy",
            "Internal",
            "--data-quality",
            "production",
            "--classification",
            "internal",
        ])
        .assert()
        .success();

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "consume",
            "1",
            "--version",
            "v1",
            "--out",
            &registry_arg(&out),
        ])
        .assert()
        .code(5)
        .stderr(predicate::str::contains("destination exists"));
}

#[test]
fn table_output_handles_multibyte_text_when_truncating() {
    let temp = tempdir().unwrap();
    let registry = temp.path().join("registry.db");
    let source = temp.path().join("daily.csv");
    fs::write(&source, "x\n").unwrap();

    feam()
        .args([
            "--registry",
            &registry_arg(&registry),
            "serve",
            &registry_arg(&source),
            "--name",
            "Daily",
            "--asset-type",
            "file",
            "--version",
            "v1",
            "--owner-team",
            "Equipe Meteo",
            "--producer",
            "éééééééééééééééééééé",
            "--usage-policy",
            "Internal",
            "--data-quality",
            "production",
            "--classification",
            "internal",
        ])
        .assert()
        .success();

    feam()
        .args(["--registry", &registry_arg(&registry), "products"])
        .assert()
        .success()
        .stdout(predicate::str::contains("ééé"));
}

#[test]
fn project_cli_publishes_resolves_stages_and_withdraws_registered_inventory() {
    let temp = tempdir().unwrap();
    let provider = temp.path().join("provider");
    let client = temp.path().join("client with spaces");
    feam()
        .args([
            "--project",
            &registry_arg(&provider),
            "--format",
            "json",
            "init",
            "--namespace",
            "climate",
            "--serving-dir",
            "serving",
            "--owner-team",
            "Climate",
        ])
        .assert()
        .success();
    let serving = provider.join("serving");
    let data = serving.join("datasets/observations/v1");
    fs::create_dir_all(&data).unwrap();
    write_parquet(&data.join("part-000.parquet"), &[1, 2]);
    write_parquet(&data.join("part-001.parquet"), &[3, 4]);
    write_parquet(&data.join("unregistered.parquet"), &[99]);
    let metadata = provider.join("metadata.json");
    fs::write(
        &metadata,
        serde_json::json!({
            "schema_version": 1,
            "namespace": "climate",
            "product_id": "observations",
            "name": "Observations",
            "version": "v1",
            "data_kind": "table",
            "data_format": "parquet",
            "description": "known values",
            "intended_use": "CLI test",
            "limitations": "none",
            "owner_team": "Climate",
            "producer": "Climate Lab",
            "contact": "climate@example.test",
            "usage_policy": "internal",
            "classification": "internal",
            "quality": "production",
            "assets": [
                {"id":"part-000", "path":"datasets/observations/v1/part-000.parquet", "role":"data", "media_type":"application/vnd.apache.parquet"},
                {"id":"part-001", "path":"datasets/observations/v1/part-001.parquet", "role":"data", "media_type":"application/vnd.apache.parquet"}
            ],
            "lineage": [],
            "table": {
                "column_meanings": {"station_id":"identifier", "temperature":"daily temperature"},
                "column_units": {"station_id":"not_applicable", "temperature":"celsius"},
                "partition_columns": []
            }
        })
        .to_string(),
    )
    .unwrap();
    feam()
        .args([
            "--project",
            &registry_arg(&provider),
            "--format",
            "json",
            "serve",
            &registry_arg(&serving),
            "--metadata",
            &registry_arg(&metadata),
        ])
        .assert()
        .success()
        .stdout(predicate::str::contains("\"protocol\":\"feam.peer.v1\""));

    feam()
        .args([
            "--project",
            &registry_arg(&client),
            "init",
            "--namespace",
            "consumer",
        ])
        .assert()
        .success();
    let peers = client.join("peers");
    fs::create_dir_all(&peers).unwrap();
    #[cfg(unix)]
    std::os::unix::fs::symlink(&serving, peers.join("local-climate")).unwrap();
    fs::write(
        client.join(".feam/project.toml"),
        "schema_version = 1\nnamespace = 'consumer'\n\n[[peers]]\nalias = 'local-climate'\nnamespace = 'climate'\npath = 'peers/local-climate'\n",
    )
    .unwrap();
    feam()
        .args(["--project", &registry_arg(&client), "refresh"])
        .assert()
        .success();
    for (flag, value, expected_count) in [
        ("--asset-type", "table", 1),
        ("--classification", "public", 0),
        ("--data-quality", "qualified", 0),
        ("--owner-team", "nobody", 0),
    ] {
        let output = feam()
            .args([
                "--project",
                &registry_arg(&client),
                "--format",
                "json",
                "search",
                flag,
                value,
            ])
            .assert()
            .success()
            .get_output()
            .stdout
            .clone();
        assert_eq!(
            serde_json::from_slice::<Value>(&output)
                .unwrap()
                .as_array()
                .unwrap()
                .len(),
            expected_count
        );
    }
    feam()
        .args([
            "--project",
            &registry_arg(&client),
            "search",
            "--asset-type",
            "directory",
        ])
        .assert()
        .code(3);
    let output = temp.path().join("stage");
    let response = feam()
        .args([
            "--project",
            &registry_arg(&client),
            "--format",
            "json",
            "resolve",
            "product://climate/observations",
            "--version",
            "v1",
        ])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let value: Value = serde_json::from_slice(&response).unwrap();
    assert_eq!(value["assets"].as_array().unwrap().len(), 2);
    assert!(!String::from_utf8_lossy(&response).contains("unregistered.parquet"));
    feam()
        .args([
            "--project",
            &registry_arg(&client),
            "consume",
            "product://climate/observations",
            "--version",
            "v1",
            "--out",
            &registry_arg(&output),
        ])
        .assert()
        .success();
    assert!(output.join("part-000").exists());
    assert!(output.with_file_name("stage.feam-receipt.json").exists());
    feam()
        .args([
            "--project",
            &registry_arg(&provider),
            "withdraw",
            "product://climate/observations",
            "--version",
            "v1",
            "--reason",
            "superseded",
        ])
        .assert()
        .success();
    feam()
        .args([
            "--project",
            &registry_arg(&client),
            "--format",
            "json",
            "resolve",
            "product://climate/observations",
            "--version",
            "v1",
        ])
        .assert()
        .code(5)
        .stderr(predicate::str::contains("withdrawn_version"));
}

#[test]
fn tui_rejects_machine_format_before_terminal_setup() {
    let temp = tempdir().unwrap();
    feam()
        .args([
            "--project",
            &registry_arg(temp.path()),
            "--format",
            "json",
            "tui",
        ])
        .assert()
        .failure()
        .stdout("")
        .stderr(predicate::str::contains("does not support --format json"));
    assert!(!temp.path().join("registry.db").exists());
}
