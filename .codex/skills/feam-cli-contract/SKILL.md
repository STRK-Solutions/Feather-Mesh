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
- When changing output, check both table and JSON behavior if both are affected.
- When changing errors, verify the mapped exit code still matches `feather-mesh/README.md`.
- Prefer adding CLI workflow coverage in `feather-mesh/mesh_cli/tests/cli_workflow_tests.rs` for externally visible behavior.
- Add service-level tests when the CLI change exposes new `mesh_core` behavior.

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
