# Agent Context

Feather Mesh is an HPC-oriented data catalog and data mesh project. The current implementation lives in `feather-mesh/` as a Rust workspace. `python_mvp/` is an earlier prototype and should not be treated as the primary implementation unless the task explicitly targets it.

## Primary Implementation

- `feather-mesh/mesh_core`: shared Rust library for domain types, SQLite setup, repositories, and service workflows.
- `feather-mesh/mesh_cli`: `feam` command-line interface built on `mesh_core`.
- Run Rust commands from `feather-mesh/`.

## Validation

Before finishing Rust changes, run the narrowest useful checks, then broaden as risk increases:

- `cargo fmt -- --check`
- `cargo clippy -- -D warnings`
- `cargo test`

Use `cargo test -p mesh_core` or `cargo test -p mesh_cli` for focused iteration.

## CLI Contract

The user-facing command is `feam`. The package and binary crate are still named `mesh_cli`.

Implemented commands:

- `init`
- `serve`
- `search`
- `show`
- `consume`
- `lineage`
- `validate-metadata`
- `teams`
- `products`

Stable exit codes are documented in `feather-mesh/README.md` and tested in `feather-mesh/mesh_cli/tests/cli_workflow_tests.rs`.

## Engineering Rules

- Keep CLI parsing, terminal output, process exit behavior, and user-facing formatting in `mesh_cli`.
- Keep business workflows in `mesh_core::services`.
- Keep SQL and row mapping in `mesh_core::repositories`.
- Add or update tests for CLI behavior, service behavior, validation, persistence, or exit-code changes.
- Do not use the Python MVP as source of truth for Rust behavior.
