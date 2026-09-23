# Historical Agent Workplan — Superseded for Peer Data Access

The older plan at this path is superseded for peer data access by [data_access.md](../../data_access.md) and the [implementation workplan](../../data_access_implementation_workplan.md). Once P0 creates `docs/data_access_contract.md`, use it for settled schema/API and migration decisions. Explicit user instructions take precedence.

The preparation review referenced an untracked older plan that is no longer present in this checkout or its available Git history. This notice preserves the routing location; it does not reconstruct the missing plan. Older central-registry and mandatory-copy assumptions must not drive the new feature.

For observed current behavior, inspect the Rust source and [workspace README](../README.md): the CLI implements nine commands and core services cover publication, discovery, inspection, lineage, and consumption. Claims that the CLI is a placeholder or services are team-only are stale. Peer manifests, the supported SDK, and STAC HTTP remain planned.
