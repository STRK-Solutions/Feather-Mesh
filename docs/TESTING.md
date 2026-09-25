Local testing instructions

- Ensure Rust and Cargo are installed and on your PATH.
- From the repository root run tests for the core and tui crates:

```powershell
cd feather-mesh
cargo test -p mesh_core
cargo test -p mesh_tui
```

- To run a single test with output:

```powershell
cargo test -p mesh_core --test table_preview_csv -- --nocapture
```

- CI will run the full workspace tests on GitHub Actions via `.github/workflows/ci.yml`.
