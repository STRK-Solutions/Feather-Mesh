use std::path::PathBuf;
use std::process::Command;

fn probe_path() -> Option<PathBuf> {
    if let Ok(p) = std::env::var("FEAM_TERMINAL_PROBE") {
        let p = PathBuf::from(p);
        if p.exists() { return Some(p); }
    }
    // common CI/build location
    let candidate = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../target/debug/examples/terminal_probe");
    if candidate.exists() { return Some(candidate); }
    None
}

#[test]
fn tutorial_probe_runs_if_available() {
    let probe = match probe_path() {
        Some(p) => p,
        None => {
            eprintln!("SKIP tutorial_probe_runs_if_available: terminal_probe not built");
            return;
        }
    };

    // running with `panic` should result in nonzero exit (probe panics)
    let status = Command::new(&probe).arg("panic").status().expect("failed to spawn probe");
    assert!(!status.success(), "probe(panic) unexpectedly succeeded");

    // running without args returns error (intentional Io error); still nonzero
    let status2 = Command::new(&probe).status().expect("failed to spawn probe");
    assert!(!status2.success(), "probe() unexpectedly succeeded");
}
