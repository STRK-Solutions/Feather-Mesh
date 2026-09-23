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

## Change Workflow

1. Read [AGENTS.md](../../../AGENTS.md) first if it has not already been loaded.
2. For publication/manifests, project/peer resolution, cache refresh, or staging, apply [feam-peer-data-access](../feam-peer-data-access/SKILL.md). Read its feature sources; preserve the distinction between the existing SQLite registry and the new manifest authority.
3. Inspect the affected crate boundary before editing.
4. Keep edits scoped to the layer that owns the behavior:
   - CLI command surface or output: `mesh_cli`.
   - Business workflow, validation orchestration, or API-style behavior: `mesh_core::services`.
   - Domain enums, validation primitives, and shared errors: `mesh_core::domain`.
   - SQL and row mapping: `mesh_core::repositories`.
5. Add or update tests at the closest useful level, including the applicable negative cases from the peer-access skill. Positive publication/resolution tests alone are insufficient.
6. Run focused checks first, then broaden if the change crosses boundaries. Update guidance and CI in the phase that introduces each component or inspector configuration.

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

For optional format inspectors, test the documented supported feature combinations, including enabled validators and explicit publication failure when a required inspector is unavailable. Record exact feature flags and native dependencies in the contract/CI when introduced; a default build alone cannot prove an optional validator works.

Changes to shared DTOs, errors, publication, or resolution also need the affected CLI, installed SDK, and HTTP integration evidence described in the peer-access skill. Cargo success alone does not establish adapter behavior.

If a command cannot be run, report the missing tool/dependency, reason, and unverified behavior. Keep local automated results separate from target-HPC evidence; an unavailable cluster leaves HPC acceptance pending while independent work proceeds.
