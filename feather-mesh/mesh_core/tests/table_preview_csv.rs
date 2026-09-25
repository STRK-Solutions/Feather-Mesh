use std::io::Write;
use tempfile::tempdir;

#[test]
fn csv_preview_reads_rows() {
    let dir = tempdir().unwrap();
    let file_path = dir.path().join("data.csv");
    let mut f = std::fs::File::create(&file_path).unwrap();
    writeln!(f, "id,name,age").unwrap();
    writeln!(f, "1,Alice,30").unwrap();
    writeln!(f, "2,Bob,25").unwrap();

    let req = mesh_core::services::table_preview::TablePreviewRequest {
        assets: vec![file_path.clone()],
        columns: Vec::new(),
        limit: 10,
    };
    let res = mesh_core::services::table_preview::run_preview(req).expect("preview failed");
    assert_eq!(res.columns, vec!["id".to_string(), "name".to_string(), "age".to_string()]);
    assert_eq!(res.rows.len(), 2);
    assert_eq!(res.rows[0][0], "1");
}
