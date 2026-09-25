use mesh_core::peer::DataKind;
use mesh_core::services::catalog_service::CatalogEntry;
use mesh_core::services::interactive_operations::path_state;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::Path;

pub const HELP: &str = "Keyboard: Tab menus; / search; PgUp/PgDn scroll; q exit. Catalog: Up/Down select; Enter details; e examples; [ ] pages. Lineage: Up/Down selects the version shown in its version lineage. Hosted Assistant: a compose; s stop; output remains available in the Assistant menu.\nCommands (quote paths with spaces):\n:init NAMESPACE [SERVING_DIR]\n:project \"ROOT\"\n:resolve REF VERSION\n:filter '{\"owner_team\":\"Climate\",\"limit\":25}'\n:publish \"DRAFT.json\" (I/O consent then full review)\n:draft new table|raster\n:draft load \"DRAFT.json\"\n:draft set /description '\"Known scientific facts\"'\n:draft set /assets '[{\"id\":\"data\",\"path\":\"relative/file.parquet\",\"role\":\"data\",\"media_type\":\"application/vnd.apache.parquet\"}]'\n:draft validate\n:draft save NAME.json [--overwrite]\n:export NAME.txt [--overwrite]\n:stage REF VERSION \"DESTINATION\" [--overwrite]\n:withdraw REF VERSION REASON\n:recover\n:agent-profile NAME|off\n:agent-draft\n:integrity-consent REF VERSION\n:usage\n\nDrafts/exports stay in .feam/drafts or .feam/exports. Metadata editing never invents missing scientific/policy facts. Source assets are explicitly selected under the serving root. Full hashing needs local consent. Copies finish before normal shutdown. Forced loss requires reconciliation.\nHosted: a compose; s stop; local profile context policy controls disclosure. No shell/Python execution tools.";

/// Deliberately only lexical quoting; no expansion, interpolation or shell.
pub fn split_command(input: &str) -> Result<Vec<String>, String> {
    let mut words = Vec::new();
    let mut word = String::new();
    let mut quote = None;
    let mut escape = false;
    let mut started = false;
    for c in input.chars() {
        if escape {
            word.push(c);
            escape = false;
            started = true;
            continue;
        }
        if c == '\\' && quote != Some('\'') {
            escape = true;
            started = true;
            continue;
        }
        if let Some(q) = quote {
            if c == q {
                quote = None;
            } else {
                word.push(c);
            }
            started = true;
        } else if c == '\'' || c == '"' {
            quote = Some(c);
            started = true;
        } else if c.is_whitespace() {
            if started {
                words.push(std::mem::take(&mut word));
                started = false;
            }
        } else {
            word.push(c);
            started = true;
        }
    }
    if quote.is_some() || escape {
        return Err("Unterminated quote or escape".into());
    }
    if started {
        words.push(word);
    }
    Ok(words)
}
fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}
fn python_quote(value: &str) -> String {
    serde_json::to_string(value).unwrap()
}

pub fn example(root: &Path, entry: &CatalogEntry) -> String {
    let root = root.to_string_lossy();
    let mut text = format!(
        "CLI (direct access):\nfeam --project {} --format json resolve {} --version {}\n\nPython SDK:\nfrom feam import Project\nproject = Project.open({})\n",
        shell_quote(&root),
        shell_quote(&entry.reference),
        shell_quote(&entry.version),
        python_quote(&root)
    );
    let reference = python_quote(&entry.reference);
    let version = python_quote(&entry.version);
    match entry.data_kind {
        DataKind::Table => text.push_str(&format!("query = project.scan_table({reference}, version={version})\n# Native polars.LazyFrame over only registered shards\nprint(query.collect())\n")),
        DataKind::Raster => text.push_str(&format!("import rasterio\nfrom rasterio.windows import Window\nproduct = project.resolve({reference}, version={version})\nwith rasterio.open(product.assets[0].path) as raster:\n    print(raster.read(1, window=Window(0, 0, 2, 2)))\n\n# STAC uses the same registered identity/inventory.\n# Start separately: feam --project ROOT stac serve --token-file PRIVATE_TOKEN_FILE\n# Connect pystac_client.Client.open(URL, headers={{'Authorization': 'Bearer '+token}}).\n# Never put a token in a project draft or assistant message.\n")),
    }
    text
}

pub fn draft_template(kind: &str) -> serde_json::Value {
    let mut value = serde_json::json!({"schema_version":1,"namespace":"","product_id":"","name":"","version":"","data_kind":kind,"data_format":if kind == "table" {"parquet"} else {"geotiff"},"description":"","intended_use":"","limitations":"","owner_team":"","producer":"","contact":"","usage_policy":"","classification":"","quality":"","assets":[],"lineage":[]});
    if kind == "table" {
        value["table"] =
            serde_json::json!({"column_meanings":{},"column_units":{},"partition_columns":[]});
    } else {
        value["raster"] = serde_json::json!({"datetime":"","bbox":[],"semantics":{}});
    }
    value
}
pub fn patch_draft(
    draft: &mut serde_json::Value,
    pointer: &str,
    value: serde_json::Value,
) -> Result<(), String> {
    if value.to_string().len() > 64 * 1024 {
        return Err("Draft patch exceeds 64 KiB".into());
    }
    let field = draft.pointer_mut(pointer).ok_or(
        "Use an existing JSON pointer; replace a complete object/asset array to add fields",
    )?;
    *field = value;
    Ok(())
}

pub fn save_local_text(
    root: &Path,
    path: &Path,
    text: &str,
    state: &str,
    overwrite: bool,
) -> Result<(), String> {
    let feam = root.join(".feam");
    let parent = path.parent().ok_or("Missing save parent")?;
    if ![feam.join("drafts"), feam.join("exports")].contains(&parent.to_path_buf()) {
        return Err("Save is outside local draft/export scope".into());
    }
    for dir in [&feam, parent] {
        if fs::symlink_metadata(dir).is_ok_and(|m| m.file_type().is_symlink()) {
            return Err("Draft/export directories cannot be symlinks".into());
        }
    }
    if path_state(path).map_err(|e| e.to_string())? != state {
        return Err("Save destination changed; review again".into());
    }
    if fs::symlink_metadata(path).is_ok_and(|m| !m.is_file() || m.file_type().is_symlink()) {
        return Err("Save destination must be a regular file".into());
    }
    fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    if !overwrite {
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(path)
            .map_err(|e| e.to_string())?;
        file.write_all(text.as_bytes())
            .and_then(|_| file.sync_all())
            .map_err(|e| e.to_string())?;
    } else {
        let temporary = parent.join(format!(".feam-export-{}", std::process::id()));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&temporary)
            .map_err(|e| e.to_string())?;
        let result = file
            .write_all(text.as_bytes())
            .and_then(|_| file.sync_all())
            .and_then(|_| fs::rename(&temporary, path));
        if result.is_err() {
            let _ = fs::remove_file(&temporary);
        }
        result.map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn paths_and_json_are_lexed_without_execution() {
        assert_eq!(
            split_command("stage ref v1 '/tmp/with spaces' --overwrite").unwrap(),
            ["stage", "ref", "v1", "/tmp/with spaces", "--overwrite"]
        );
        assert_eq!(
            split_command("draft set /description '\"a $HOME `command`\"'").unwrap()[3],
            "\"a $HOME `command`\""
        );
        assert!(split_command("'unfinished").is_err());
        assert_eq!(shell_quote("a'b"), "'a'\\''b'");
    }
    #[test]
    fn changed_export_is_never_overwritten_by_old_review() {
        let temp = tempfile::tempdir().unwrap();
        let folder = temp.path().join(".feam/exports");
        fs::create_dir_all(&folder).unwrap();
        let path = folder.join("example.txt");
        let state = path_state(&path).unwrap();
        fs::write(&path, "prior bytes").unwrap();
        assert!(save_local_text(temp.path(), &path, "replacement", &state, true).is_err());
        assert_eq!(fs::read_to_string(path).unwrap(), "prior bytes");
    }
}
