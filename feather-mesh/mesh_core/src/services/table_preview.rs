use serde::{Serialize, Deserialize};
use std::path::PathBuf;
use std::fs::File;
use std::io::BufReader;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TablePreviewRequest {
    pub assets: Vec<PathBuf>,
    pub columns: Vec<String>,
    pub limit: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TablePreviewResult {
    pub rows: Vec<Vec<String>>,
    pub columns: Vec<String>,
}

/// Minimal safe preview implementation:
/// - supports CSV files (comma-separated) and Parquet via the `parquet` crate when available
/// - returns up to `limit` rows as strings
pub fn run_preview(req: TablePreviewRequest) -> Result<TablePreviewResult, String> {
    let mut all_columns: Vec<String> = Vec::new();
    let mut rows: Vec<Vec<String>> = Vec::new();
    let mut remaining = req.limit;

    for asset in req.assets.iter() {
        if remaining == 0 { break; }
        let path = asset;
        let ext = path.extension().and_then(|s| s.to_str()).unwrap_or("").to_lowercase();
        match ext.as_str() {
            "csv" => {
                let f = File::open(path).map_err(|e| format!("open {:?}: {}", path, e))?;
                let mut rdr = csv::Reader::from_reader(BufReader::new(f));
                // collect header
                if all_columns.is_empty() {
                    all_columns = rdr.headers()
                        .map_err(|e| format!("csv header {:?}: {}", path, e))?
                        .iter().map(|s| s.to_string()).collect();
                }
                for result in rdr.records() {
                    if remaining == 0 { break; }
                    let record = result.map_err(|e| format!("csv record {:?}: {}", path, e))?;
                    let row = record.iter().map(|s| s.to_string()).collect();
                    rows.push(row);
                    remaining -= 1;
                }
            }
            "parquet" => {
                // Try to read using the arrow/parquet reader if available; otherwise return an informative error.
                match read_parquet_preview(path, remaining) {
                    Ok((cols, mut r)) => {
                        if all_columns.is_empty() {
                            all_columns = cols.clone();
                        }
                        // append rows
                        for row in r.drain(..) {
                            if remaining == 0 { break; }
                            rows.push(row);
                            remaining -= 1;
                        }
                    }
                    Err(e) => return Err(format!("parquet {:?}: {}", path, e)),
                }
            }
            _ => return Err(format!("unsupported asset extension: {:?}", path)),
        }
    }

    Ok(TablePreviewResult { rows, columns: all_columns })
}

fn read_parquet_preview(path: &PathBuf, limit: usize) -> Result<(Vec<String>, Vec<Vec<String>>), String> {
    use parquet::file::reader::{SerializedFileReader, FileReader};

    let file = File::open(path).map_err(|e| format!("open {:?}: {}", path, e))?;
    let reader = SerializedFileReader::new(file).map_err(|e| format!("parquet read: {}", e))?;
    let iter = reader.get_row_iter(None).map_err(|e| format!("get_row_iter: {}", e))?;
    let mut cols: Vec<String> = Vec::new();
    let mut rows: Vec<Vec<String>> = Vec::new();
    for (i, row_res) in iter.enumerate() {
        let row = row_res.map_err(|e| format!("parquet row iter: {}", e))?;
        if i == 0 {
            cols = row.get_column_iter().map(|c| c.0.to_string()).collect();
        }
        if rows.len() >= limit { break; }
        let vals = row.get_column_iter().map(|(_, v)| v.to_string()).collect();
        rows.push(vals);
    }
    Ok((cols, rows))
}
