# Feather Mesh agent context

This page is a compact orientation for coding assistants, including hosted
models such as DeepSeek. It is not an implementation contract. In a repository
checkout, read `AGENTS.md` and the task-specific requirements and contracts
before changing code. The repository's source and tests establish current
behavior; do not infer implementation from product proposals or this summary.

## Product model

Feather Mesh catalogs and resolves reusable, versioned data products while
provider teams retain ownership of their files. Filesystem permissions remain
the actual access-control boundary.

For the current project-scoped peer workflow:

- Provider `serving/manifest.json` files are authoritative for published
  product versions.
- Client `.feam/project.toml` configuration defines peer routes. A configured
  route is not itself a filesystem permission grant.
- Resolve returns a registered, pinned inventory for direct reads. It does not
  copy files.
- Staging is an explicit copy workflow with provenance; do not describe it as
  the default behavior of resolve.
- Legacy SQLite (`--registry`) is a separate workflow and is not peer
  publication authority.

## Implementation ownership

- `feather-mesh/mesh_core/src/services/`: shared business workflows and
  validation.
- `feather-mesh/mesh_core/src/repositories/`: SQL and row mapping.
- `feather-mesh/mesh_cli/`: command parsing, output, and process behavior.
- `feather-mesh/mesh_tui/`: optional interactive rendering, local input, and
  review confirmation.
- `feather-mesh/mesh_agent/`: optional provider transport, bounded orchestration,
  and tool validation. The model proposes actions; the harness does not own
  confirmation or direct mutation authority.
- `feather-mesh/python_sdk/` and `mesh_core` STAC adapters: adapters to shared
  behavior, not independent publication or resolution rules.

Put a change in the layer that owns it. Avoid duplicating visibility,
publication, or access rules in an adapter.

## Repository routing

Read the sources that govern the task:

- [`AGENTS.md`](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/AGENTS.md)
  routes work to the owning crate, task skills, and validation.
- [`map.md`](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/map.md)
  is a detailed repository orientation, not a contract.
- [Peer data-access requirements](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/data_access.md),
  [settled contract](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/data_access_contract.md),
  and [workplan](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/data_access_implementation_workplan.md)
  govern peer access. The contract and confirmed requirements take precedence
  over historical plans.
- [Workspace README](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/feather-mesh/README.md)
  documents CLI usage and Rust workspace commands.
- [TUI and agent Stage-1 contract](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/tui_agent_stage1_contract.md)
  describes the optional assistant/TUI behavior and its evidence boundaries.

The browser-based Ubuntu demo is a separate proposed service. Its
[design](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/ubuntu_web_dev_demo_design.md)
and [workplan](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/ubuntu_web_dev_demo_workplan.md)
do not make planned web or cloud components implemented.

## Working rules

1. Inspect the owning source and tests before making claims or edits.
2. Preserve the separation between project-scoped peer access and legacy
   SQLite.
3. Keep adapter behavior consistent with shared core rules; test each affected
   surface.
4. Do not claim hosted-model quality, target-HPC acceptance, or web/cloud
   deployment from local tests or documentation builds.
5. Follow the relevant repository skill and run its narrowest useful checks.
   For Rust changes, run checks from `feather-mesh/` and follow
   `AGENTS.md` validation guidance.
