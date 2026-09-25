# Agent Context

Feather Mesh is an HPC-oriented data catalog and data mesh project. The current implementation lives in `feather-mesh/` as a Rust workspace.

## Primary Implementation

- [mesh_core](feather-mesh/mesh_core/Cargo.toml): shared Rust library for domain types, SQLite setup, repositories, and service workflows.
- [mesh_cli](feather-mesh/mesh_cli/Cargo.toml): `feam` command-line interface built on `mesh_core`.
- [mesh_tui](feather-mesh/mesh_tui/Cargo.toml): optional Ratatui/Crossterm interactive project UI; it calls shared services directly and owns terminal lifecycle/reviews.
- [mesh_agent](feather-mesh/mesh_agent/Cargo.toml): optional provider-neutral router/harness and typed tool schemas; it has no terminal or peer-mutation authority.
- Run Rust commands from `feather-mesh/`.

## Peer Data Access Routing

Explicit user instructions take precedence. For peer data access, read [requirements](data_access.md), the settled [contract](docs/data_access_contract.md), and the [implementation workplan](data_access_implementation_workplan.md). Confirmed requirements and the contract govern; source and tests establish observed behavior.

Peer publication uses authoritative provider `serving/manifest.json` records and first-class direct reads. Legacy SQLite remains separate and is not a peer authority. `mesh_core::services::peer_access` owns project config, manifests, inspection, refresh, resolution, and staging; `mesh_core::stac`/`stac_http` are derived adapters; `python_sdk/` is the supported subprocess adapter. Older PDDs and plans are historical for this feature; see the [older agent-plan notice](feather-mesh/.agents/feather_mesh_workplan.md).

Use [feam-peer-data-access](.codex/skills/feam-peer-data-access/SKILL.md) for publication/manifests, project/peer resolution, cache refresh, SDK, STAC, and peer staging changes, including adapter-only work. Also use [feam-rust-workflow](.codex/skills/feam-rust-workflow/SKILL.md) for Rust, [feam-cli-contract](.codex/skills/feam-cli-contract/SKILL.md) for CLI/protocol changes, and [feam-agent-context-maintainer](.codex/skills/feam-agent-context-maintainer/SKILL.md) when context or component commands change.

## TUI and Agent Harness Routing

For planned TUI and agent-harness work, read the [design](tui_agent_harness_design.md) and [stage-1 implementation workplan](tui_agent_harness_stage1_workplan.md). They distinguish planned components from existing behavior and define implementation scope and required evidence. Apply the Rust, CLI, peer-access, and context-maintenance skills above as appropriate.

The [demo runbook](docs/tui_agent_stage1_demo.md) gives manual/fake/live commands and PTY/evaluation checks; the [acceptance record](docs/tui_agent_stage1_acceptance.md) distinguishes measured results from remaining gates.

The [router model screening runbook](docs/tui_agent_model_screening.md) documents the synthetic development screen, schema export, and offline Python checks. Keep its development tasks separate from held-out acceptance; live runs require explicit opt-in.

## Web Demo Routing

For web-accessible demo work, read the [design](docs/ubuntu_web_dev_demo_design.md) and [development workplan](docs/ubuntu_web_dev_demo_workplan.md). The workplan owns task status, Mac/Linux/Ubuntu execution boundaries, dependencies, and deployment/evidence gates. Source and tests establish implemented Web/IaC components; local tests do not establish Ubuntu or cloud acceptance.

The active demo runs on Ubuntu with Cloudflare browser access and OpenRouter. Cloud hosts and R2 are shelved; the workplan defines the functional delivery gate and deferred extended tests. Research traces/exports must survive stop and have a verified private Mac copy before destructive teardown; project spending stays outside demo compute. The collector supports explicit local archival; [operator lifecycle](infra/demo/SITE_LIFECYCLE.md) documents verified copy and retained-copy maintenance. Source/tests and the acceptance index distinguish implementation from deployed proof.

The [web contract](docs/ubuntu_web_dev_demo_contract.md) defines process/IPC/state boundaries. [Web validation](web_demo/README.md) and [operator checks](infra/demo/README.md) document runnable schema checks and read-only preflight; use `python infra/demo/scripts/check.py` from the repository root after installing its locked environment. The [W1 private terminal](infra/demo/W1.md) and participant services live in `web_demo/cmd/` with separate gateway, controller, broker, collector, pipeline and reconciler owners. Run `go test -race ./...`, `go vet ./...` and `go build -trimpath ./cmd/...` from `web_demo/` with its pinned Go version. [Identity](web_demo/IDENTITY.md), [broker/accounting](web_demo/BROKER.md) and [Terraform modules](infra/demo/terraform/README.md) document their checks; consult the [acceptance index](docs/ubuntu_web_dev_demo_acceptance.md) for measured environment evidence. Implemented components and mock-provider tests do not establish public or cloud readiness.

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
| STAC | Pinned-schema validation plus real-client tokenless, loopback-only paginated HTTP integration, rejected non-loopback binding, and a known Rasterio window. |
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
- `refresh`
- `cache`
- `resolve`
- `withdraw`
- `stac`
- `tui`

`tui` is an optional feature, always requires `--project ROOT`, and never uses legacy `registry.db`.

Stable exit codes are documented in the [workspace README](feather-mesh/README.md#exit-codes) and tested in [CLI workflow tests](feather-mesh/mesh_cli/tests/cli_workflow_tests.rs).

## Engineering Rules

| Owner | Responsibility |
| --- | --- |
| mesh_cli | CLI parsing, terminal output, process exit behavior, and user-facing formatting. |
| mesh_tui | Interactive rendering, local input, review confirmation, and terminal restoration; never peer business rules. |
| mesh_agent | Router/config translation, bounded agent loop, and provider-facing tool validation; never confirmation or direct mutation. |
| mesh_core::services | Shared business workflows, publication, validation, and resolution; SDK/HTTP adapters call these rules. |
| mesh_core::repositories | SQL and row mapping. |
| mesh_core shared types | Reusable DTOs and domain errors; adapters translate them without duplicating visibility rules. |

- Add or update tests for CLI behavior, service behavior, validation, persistence, or exit-code changes.
