use std::fs;
use std::path::Path;

use assert_cmd::Command;
use predicates::prelude::*;
use serde_json::Value;
use tempfile::tempdir;

fn feam() -> Command {
    Command::cargo_bin("mesh_cli").expect("binary exists")
}

fn registry_arg(path: &Path) -> String {
    path.to_string_lossy().into_owned()
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
