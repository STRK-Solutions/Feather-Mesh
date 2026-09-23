# Agent Context

Feather Mesh is an HPC-oriented data catalog and data mesh project. The current implementation lives in `feather-mesh/` as a Rust workspace.

## Primary Implementation

- [mesh_core](feather-mesh/mesh_core/Cargo.toml): shared Rust library for domain types, SQLite setup, repositories, and service workflows.
- [mesh_cli](feather-mesh/mesh_cli/Cargo.toml): `feam` command-line interface built on `mesh_core`.
- Run Rust commands from `feather-mesh/`.

## Peer Data Access Routing

Explicit user instructions take precedence. For peer data access, read [requirements](data_access.md) and the [implementation workplan](data_access_implementation_workplan.md). Confirmed requirements govern; the workplan supplies proposed defaults. Once P0 creates `docs/data_access_contract.md`, use it for settled schema/API and migration decisions. Source and tests establish observed current behavior, not the intended feature contract.

The new workflow requires authoritative registered manifests and first-class direct reads; SQLite/STAC are derived views. The current implementation still uses a SQLite registry. Older PDDs and plans are historical for this feature; see the [older agent-plan notice](feather-mesh/.agents/feather_mesh_workplan.md). Proposed commands, `python_sdk/`, and `mesh_stac` are not implemented.

Use [feam-peer-data-access](.codex/skills/feam-peer-data-access/SKILL.md) for publication/manifests, project/peer resolution, cache refresh, SDK, STAC, and peer staging changes, including adapter-only work. Also use [feam-rust-workflow](.codex/skills/feam-rust-workflow/SKILL.md) for Rust, [feam-cli-contract](.codex/skills/feam-cli-contract/SKILL.md) for CLI/protocol changes, and [feam-agent-context-maintainer](.codex/skills/feam-agent-context-maintainer/SKILL.md) when context or component commands change.

## Validation

Before finishing Rust changes, run the narrowest useful checks, then broaden as risk increases:

- `cargo fmt -- --check`
- `cargo clippy -- -D warnings`
- `cargo test`

Use `cargo test -p mesh_core` or `cargo test -p mesh_cli` for focused iteration.

Validate every affected surface using the shared peer-access skill. Add concrete commands to guidance and CI when each component is introduced, before its phase is complete; do not wait for P8 or invent commands for absent components.

| Surface | Required evidence |
| --- | --- |
| SDK | Documented tests and fresh package installation; native Polars lazy query and subprocess success/error compatibility. |
| STAC | Pinned-schema validation plus real-client authenticated, paginated HTTP integration and a known Rasterio window. |
| Inspectors | Tested supported feature configurations, native dependencies, and explicit failure when a required validator is unavailable. |
| HPC | Actual target-filesystem, separate-identity and multi-node cache evidence, recorded separately from local automation. |

Report missing tools/dependencies and unexecuted checks explicitly. Skipped required tests and static STAC JSON cannot prove missing runtime behavior. If the cluster is unavailable, leave HPC acceptance pending and continue independent implementation.

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

Stable exit codes are documented in the [workspace README](feather-mesh/README.md#exit-codes) and tested in [CLI workflow tests](feather-mesh/mesh_cli/tests/cli_workflow_tests.rs).

## Engineering Rules

| Owner | Responsibility |
| --- | --- |
| mesh_cli | CLI parsing, terminal output, process exit behavior, and user-facing formatting. |
| mesh_core::services | Shared business workflows, publication, validation, and resolution; SDK/HTTP adapters call these rules. |
| mesh_core::repositories | SQL and row mapping. |
| mesh_core shared types | Reusable DTOs and domain errors; adapters translate them without duplicating visibility rules. |

- Add or update tests for CLI behavior, service behavior, validation, persistence, or exit-code changes.
