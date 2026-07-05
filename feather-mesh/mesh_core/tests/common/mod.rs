use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

static DB_COUNTER: AtomicU64 = AtomicU64::new(0);

pub fn unique_test_db_path(prefix: &str) -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time before UNIX EPOCH")
        .as_nanos();
    let mut path = std::env::temp_dir();
    path.push(format!(
        "{prefix}_{}_{}_{}.db",
        std::process::id(),
        timestamp,
        DB_COUNTER.fetch_add(1, Ordering::Relaxed)
    ));
    path
}
