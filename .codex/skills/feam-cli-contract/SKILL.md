---
name: feam-cli-contract
description: Use when changing or reviewing Feather Mesh CLI behavior, feam commands, command flags, output formatting, JSON/table responses, user-facing errors, exit codes, or CLI workflow tests.
---

# Feather Mesh CLI Contract

Use this skill for user-facing `feam` behavior.

## Contract

- The user-facing command is `feam`.
- The Rust package and binary crate remain `mesh_cli`.
- Keep command parsing, terminal UX, output formatting, and process exit behavior in `feather-mesh/mesh_cli`.
- Keep business rules and persistence workflows behind `mesh_core::services`.
- Keep shared result DTOs and domain errors in core; CLI rendering and error-to-exit-code mapping remain in `mesh_cli`.
- For peer publication, resolution, refresh, staging, or an SDK-facing protocol, also apply [feam-peer-data-access](../feam-peer-data-access/SKILL.md).

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

## Change Rules

- Preserve command names, flag names, JSON field names, and exit-code meanings unless the user explicitly requests a contract change.
- Execute an authorized contract change by documenting its version/migration and testing old/new behavior; the preservation rule does not require renewed approval for changes already within the requested scope. Record feature decisions in the existing `docs/data_access_contract.md`, rather than preserving conversation history in this skill.
- When changing output, check both table and JSON behavior if both are affected.
- When changing errors, verify the mapped exit code still matches `feather-mesh/README.md`.
- Prefer adding CLI workflow coverage in `feather-mesh/mesh_cli/tests/cli_workflow_tests.rs` for externally visible behavior.
- Add service-level tests when the CLI change exposes new `mesh_core` behavior.

## SDK Protocol and Migration

For the supported `python_sdk/` subprocess interface, read the [requirements](../../../data_access.md), [contract](../../../docs/data_access_contract.md) and [workplan](../../../data_access_implementation_workplan.md). Project mode uses the versioned peer protocol and structured machine-mode errors; legacy registry behavior remains separate.

- Define versioned result and error schemas, compatibility/unknown-field rules, and explicit mappings from each domain error to a machine error kind and exit code. Retain documented exit meanings unless an authorized migration changes them.
- Keep success stdout parseable as one result JSON value. In machine mode, use the documented structured error channel on failure (stderr for the current peer protocol); diagnostic text must not contaminate either JSON payload. Document how verbose diagnostics are separated.
- Specify conflicts/precedence for project and legacy registry options. Peer operations require explicit project anchoring and must not silently use a working-directory `registry.db`. Direct reads and staging require pinned product/version references.
- Keep ordinary `serve` as publication. The implemented `stac serve` starts authenticated HTTP. Keep the `feam` user-facing name and configurable `mesh_cli` executable path distinct.
- For intentional breaks, provide migration examples and compatibility/rejection tests for prior invocations, JSON consumers, relative paths, and registry selection. Legacy inspection/staging must not silently import unregistered entries into peer discovery or bypass the publication metadata gate.

Test success and failure through the actual installed SDK/subprocess boundary, with argument arrays (no shell), paths containing spaces, changed working directories, a missing executable, incompatible protocol versions, malformed JSON, and every relevant error category. Test stdout/stderr separation and project/registry conflicts. Skipped installed-adapter checks leave that evidence pending; CLI-only tests do not substitute for them.

## Validation

For focused CLI iteration, run from `feather-mesh/`:

```bash
cargo test -p mesh_cli
```

For contract-sensitive changes, also run:

```bash
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
```

The optional `tui` command always requires `--project`; JSON mode is rejected before terminal setup. CLI owns flags and errors, while `mesh_tui` owns local input, reviews and restoration. Follow the [TUI contract](../../../docs/tui_agent_stage1_contract.md) and [PTY/demo checks](../../../docs/tui_agent_stage1_demo.md) for this surface.
