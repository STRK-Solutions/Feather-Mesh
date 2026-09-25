<p align="center">
    <img src="../imgs/logo.png" width="400">
</p>

# Feather Mesh

Feather Mesh (feam) is an HPC-native middleware layer that standardizes how teams publish, discover, and consume reusable data products without forcing teams to give up ownership of their data. It is intended to reduce duplicated work, improve cross-team interoperability, and make pipelines more reliable by replacing ad hoc path conventions with a governed product catalog and deterministic retrieval workflows.

If you do not know the project yet, read the repository's
[`map.md`](../map.md) first. It defines the beginner mental model and terminology
while clearly separating personal orientation notes from binding contracts.

Start here when contributing to the current Rust implementation:

- CLI crate: `mesh_cli/`
- Core business logic crate: `mesh_core/`
- Optional terminal UI crate: `mesh_tui/`
- Optional hosted-assistant harness: `mesh_agent/`
- Supported Python adapter: `python_sdk/`
- Test command from this directory: `cargo test`

---

## Prerequisites

> **_NOTE:_** A Linux/Unix environment is recommended. WSL is a good option on Windows.

Make sure you have Rust installed.

### Install Rust

Using `rustup`:

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

After installation:

```bash
rustc --version
cargo --version
```

If both return versions, you're good to go!

---

## Project Setup

### 1. Clone the Repository

```bash
git clone https://github.com/STRK-Solutions/dcat_4_hpc.git
cd dcat_4_hpc/feather-mesh
```

### 2. Install Dependencies

Rust handles dependencies automatically via `Cargo.toml`.

To fetch dependencies:

```bash
cargo fetch
```

---

## Build the Project

### Debug Build (default)

```bash
cargo build
```

Output binary will be located in:

```
target/debug/mesh_cli
```

### Release Build (optimized)

```bash
cargo build --release
```

Output binary:

```
target/release/mesh_cli
```

---

## Run the CLI

The package is still named `mesh_cli`, but the command help and user-facing surface are `feam`.

```bash
cargo run -p mesh_cli -- --help
```

Use `--registry <path>` to select the legacy SQLite registry. If omitted, its
legacy commands use `registry.db` in the current directory. Project-scoped peer
commands always require `--project <root>` and never fall back to that registry.

### Core Workflow

```bash
cargo run -p mesh_cli -- --registry /tmp/feam.db init

cargo run -p mesh_cli -- --registry /tmp/feam.db serve ./data.csv \
  --name "Daily Observations" \
  --asset-type file \
  --version v1.0.0 \
  --owner-team Climate \
  --producer "Climate Lab" \
  --usage-policy "Internal research use" \
  --data-quality production \
  --classification internal

cargo run -p mesh_cli -- --registry /tmp/feam.db search daily
cargo run -p mesh_cli -- --registry /tmp/feam.db --format json show 1
cargo run -p mesh_cli -- --registry /tmp/feam.db lineage 1
cargo run -p mesh_cli -- --registry /tmp/feam.db consume 1 --version v1.0.0 --out ./copy.csv
```

### Project-scoped peer access

The authoritative peer record is a provider `serving/manifest.json`, not the
SQLite registry. First create project configuration, arrange peer links
administratively, and publish an explicitly declared Parquet or GeoTIFF
inventory with complete metadata:

```bash
cargo run -p mesh_cli -- --project /work/provider init --namespace climate --serving-dir serving --owner-team Climate
cargo run -p mesh_cli -- --project /work/provider serve /work/provider/serving \
  --metadata /work/provider/temperature-v1.json

# The client .feam/project.toml names a peer alias, expected namespace, and link.
cargo run -p mesh_cli -- --project /work/client refresh
cargo run -p mesh_cli -- --project /work/client --format json resolve \
  product://climate/temperature --version v1
cargo run -p mesh_cli -- --project /work/client consume \
  product://climate/temperature --version v1 --out /scratch/temperature-v1
```

Direct `resolve` does not copy data and returns only the registered pinned
inventory through the configured client route. `consume` is explicit staging;
it uses a temporary destination and writes a provenance receipt. A withdrawn
version or removed peer cannot be reopened through a cached provider path.

The full manifest, metadata, lifecycle, cache, protocol, STAC, integrity, and
staging contract is in [`docs/data_access_contract.md`](../docs/data_access_contract.md).
The supported subprocess SDK is [`python_sdk/`](python_sdk/README.md).
[Notebook and batch-job examples](../docs/data_access_examples.md) show bounded
Rasterio windows, lazy Polars scans, loopback STAC, provenance, and staging.

Run the optional adapter fixtures and integration suite with:

```bash
python3 -m venv /tmp/feam-peer-venv
/tmp/feam-peer-venv/bin/python -m pip install ./python_sdk[test]
/tmp/feam-peer-venv/bin/python mesh_core/tests/data/peer_access/generate_fixtures.py
cargo build --bin mesh_cli
FEAM_E2E=1 FEAM_EXECUTABLE="$(pwd)/target/debug/mesh_cli" \
  /tmp/feam-peer-venv/bin/python -m pytest python_sdk/tests
```

`feam stac serve --project ROOT [--addr 127.0.0.1:PORT]` starts the read-only,
unauthenticated STAC endpoint. Core rejects non-`127.0.0.1` bind addresses. It
returns metadata and local file URIs, never raster bytes; filesystem permissions
still govern opening those files.

### Optional project TUI and hosted assistant

The optional keyboard-driven TUI always requires a project root and never
falls back to a legacy `registry.db`:

```bash
cargo run -p mesh_cli --features tui -- tui --project /work/client --agent off
```

It supports manifest-backed catalog browsing with per-peer coverage, pinned
details/lineage/inventory, direct-resolution CLI/SDK examples, peer refresh,
and reviewed publication/staging/withdrawal. Catalog shows product versions
beside metadata and inventory; Lineage uses the same selectable version list
beside the product version lineage. Peers, Operations,
Help, Teams, and Cache remain separate full-width views. A missing project
is initialized only through its explicit TUI action. Normal noninteractive CLI
JSON is not used or contaminated by TUI rendering.

The hosted router is separately opt-in and uses a profile outside the project:

```bash
cargo run -p mesh_cli --features agent-hosted -- tui \
  --project /work/client --agent hosted --agent-profile demo-router
```

Hosted builds add an Assistant menu. Submitting with `a` opens it; streaming
and completed replies remain available while navigating other menus, with an
unread marker and independent `PgUp`/`PgDn` scrolling. Transcripts are bounded
to 1 MiB of rendered text, reset on project/profile changes, and are never
persisted automatically.

Profiles, disclosure policy, API-key environment references, bounded tools,
review semantics, and recovery outcomes are defined in
[the Stage-1 contract](../docs/tui_agent_stage1_contract.md). See the
[demo runbook](../docs/tui_agent_stage1_demo.md) for a synthetic provider/client
fixture and the [acceptance record](../docs/tui_agent_stage1_acceptance.md) for
separate local/live/HPC evidence. Drafts are editable with `:draft` commands;
`:recover` reconciles pending journal records without replay. The runbook includes
PTY restoration tests, complete manual/fake walkthroughs and the fixture-backed
100-task evaluation runner.

For hosted model comparison, see the [synthetic screening runbook and results](../docs/tui_agent_model_screening.md).
The screen exports the Rust tool schemas and simulates results; it does not
execute peer operations or establish Stage-1 acceptance. Its offline checks are
`python3 -m unittest discover -s scripts -p test_router_model_screen.py`.

### Commands

The implemented command surface is:

- `init`
- `serve`
- `search`
- `show`
- `consume`
- `lineage`
- `validate-metadata`
- `teams`
- `products`
- `refresh`, `cache status`, `resolve`, and `withdraw` (project-scoped)
- `stac serve` (project-scoped, unauthenticated and restricted to `127.0.0.1`)
- `tui` (optional feature, project-scoped)

Global options:

- `--registry <path>`
- `--project <root>` (conflicts with `--registry`)
- `--format <table|json>`
- `--verbose`
- `--help`
- `--version`

### Exit Codes

| Code | Meaning |
| ---- | ------- |
| `0` | success |
| `1` | general runtime error |
| `2` | invalid CLI usage |
| `3` | validation failure |
| `4` | not found |
| `5` | permission or policy failure |

---

## Run Tests

Run tests from this workspace directory:

```bash
cargo test
```

---

## Formatting & Linting

Format code:

```bash
cargo fmt
```

Run Clippy:

```bash
cargo clippy
```

---

## Project Structure

```
feather-mesh/
├── Cargo.toml
├── mesh_cli/
│   ├── Cargo.toml
│   └── src/
│       └── main.rs        # CLI parsing, terminal UX, and process behavior
├── mesh_tui/              # Optional interactive terminal application
├── mesh_agent/            # Optional hosted router and typed agent harness
├── python_sdk/             # Supported Python subprocess adapter
├── scripts/                # TUI demos, evaluation, and offline checks
└── mesh_core/
    ├── Cargo.toml
    ├── src/
    │   ├── lib.rs         # Library exports
    │   ├── db.rs          # SQLite connection and schema setup
    │   ├── peer.rs        # Public project/peer workflow facade
    │   ├── stac.rs        # Manifest-derived STAC projection
    │   ├── stac_http.rs   # Unauthenticated, IPv4-loopback-only STAC API
    │   ├── models/        # Domain data structures
    │   │   ├── entities/  # Persisted database row models
    │   │   └── new/       # Insertable NewX models
    │   ├── repositories/  # SQL queries and object mapping
    │   └── services/      # Shared registry, peer, catalog, and operation workflows
    └── tests/
        └── data/          # Static test fixtures
```

`mesh_cli` is responsible for command-line parsing, terminal output, process exit behavior, and other terminal UX concerns. It should translate user input into calls against the core library, then format results for the terminal.

`mesh_core::services` defines shared Feather Mesh workflows such as publishing,
discovering, resolving, staging, and legacy registry access. The peer workflow
uses provider manifests; the legacy workflow coordinates models and SQLite
repositories. Repository modules own SQL queries and database row mapping.

`mesh_tui` calls the shared services directly and owns terminal rendering,
confirmation, and restoration. `mesh_agent` owns only the bounded model loop and
typed proposals; it has no terminal or direct peer-mutation authority. The
Python SDK invokes the structured CLI protocol and does not reimplement peer
visibility or publication rules.

For general product background, see `Feather_Mesh_PDD_Revised.pdf` at the repository root. For peer data access, [data_access.md](../data_access.md) supplies confirmed requirements and [`docs/data_access_contract.md`](../docs/data_access_contract.md) supplies the settled implementation contract. The legacy SQLite workflow above remains available for migration but does not bypass peer publication or discovery rules.

---

## Useful Cargo Commands

| Command       | Description                 |
| ------------- | --------------------------- |
| `cargo check` | Type-check without building |
| `cargo build` | Build project               |
| `cargo run`   | Build and run               |
| `cargo test`  | Run tests                   |
| `cargo clean` | Remove build artifacts      |
