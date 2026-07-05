---
name: feam-rust-workflow
description: Use when making Rust implementation changes in Feather Mesh, including mesh_core or mesh_cli edits, repository/service/domain changes, tests, refactors, build failures, clippy failures, or cargo workflow work.
---

# Feather Mesh Rust Workflow

Use this skill for Rust work in the `feather-mesh/` workspace.

## Orientation

- Work from `feather-mesh/` for Cargo commands.
- `mesh_core` owns domain types, SQLite setup, repositories, services, and reusable workflow behavior.
- `mesh_cli` owns CLI parsing, terminal output, JSON/table formatting, and process exit behavior.
- The root-level `python_mvp/` is historical prototype context unless the user explicitly asks for Python MVP work.

## Change Workflow

1. Read `AGENTS.md` first if it has not already been loaded.
2. Inspect the affected crate boundary before editing.
3. Keep edits scoped to the layer that owns the behavior:
   - CLI command surface or output: `mesh_cli`.
   - Business workflow, validation orchestration, or API-style behavior: `mesh_core::services`.
   - Domain enums, validation primitives, and shared errors: `mesh_core::domain`.
   - SQL and row mapping: `mesh_core::repositories`.
4. Add or update tests at the closest useful level.
5. Run focused checks first, then broaden if the change crosses boundaries.

## Validation

Use the narrowest check while iterating:

```bash
cargo test -p mesh_core
cargo test -p mesh_cli
```

Before finishing material Rust changes, run from `feather-mesh/`:

```bash
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
```

If a command cannot be run, report the reason and the residual risk.
