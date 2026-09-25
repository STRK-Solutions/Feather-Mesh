---
name: feam-peer-data-access
description: Implement or review Feather Mesh peer publication/manifests, project and peer resolution, cache refresh, supported Python SDK, STAC, or peer staging changes, including adapter-only work.
---

# Feather Mesh Peer Data Access

## Sources and Scope

Read [AGENTS.md](../../../AGENTS.md), [requirements](../../../data_access.md), and the [workplan](../../../data_access_implementation_workplan.md). Explicit user instructions take precedence. Confirmed requirements and the [settled contract](../../../docs/data_access_contract.md) govern. Keep detailed schemas, link policies and migration decisions in that contract; source and tests establish observed behavior.

Peer publication and reads use authoritative `serving/manifest.json` records through `mesh_core::services::peer_access`. The supported `python_sdk/` subprocess adapter and STAC HTTP adapter are implemented. Legacy SQLite remains separate and is not a peer authority. Older central-registry/copy-first plans do not govern this feature. Direct peer reads are first-class; staging is explicit.

## Shared Invariants

- Publish one authoritative registered record with an exact versioned asset inventory. Validate before atomic publication; readers see a complete old or new record. STAC is derived; a future shared catalog must also remain a projection rather than an independently editable authority. Existing legacy SQLite is separate. Failure before commit leaves no visible partial version; failure after commit must distinguish committed publication from projection failure and support safe retry/recovery.
- Enforce required reuse metadata and physical Parquet/GeoTIFF inspection in Rust core services for every publication entry point. Extensions alone do not validate format. If a required inspector is unavailable, fail clearly; never accept unchecked publication. Keep provider bytes intact.
- Use one Rust resolver for CLI, SDK, and HTTP. Return exactly the registered inventory for a pinned product/version. No filesystem globbing for discovery/resolution, recursive directory scans labeled as pinned versions, or explicit-path access to unregistered readable assets. Provider inventory preparation may suggest files but cannot itself publish them.
- Anchor every peer operation to an explicit project root, independent of process working directory. Check configured peers and expected namespaces; aliases need not equal provider namespaces. Validate registered serving-root-relative paths against traversal and the settled link policy. Canonical identity may support validation, but returned access must preserve the configured peer route. Never reopen an unavailable/removed peer via a cached physical path.
- Recheck registration, lifecycle, and current peer availability for new access, refresh, and HTTP pages. Keep explicitly stale metadata distinct from readable data. A pinned inventory does not prove unchanged bytes, and removing a link cannot revoke already-open handles or loaded data. Native lazy queries may fail at later execution.
- Before staging mutates anything, reject source/output equality, hard-link and symlink aliases, ancestor/descendant overlap, dangling output links, and receipt collisions with sources or outputs. Use disposable fixtures for collision tests. Copy only registered assets, preserve provider bytes and prior destinations on failure, and define temporary-copy/receipt commit and recovery behavior. Do not reuse remove-before-copy overwrite helpers without fixing their semantics.

## Required Negative Evidence

Select applicable cases for the behavior changed and exercise the affected public adapters as well as core services:

| Change | Evidence required |
| --- | --- |
| Publication/format validation | Incomplete metadata, renamed/corrupt formats, incompatible shards, missing inspectors, duplicate versions, interrupted/partial publication, concurrent writers, and projection failure leave the documented authoritative state. |
| Discovery/resolution | An unregistered readable TIFF is rejected even by explicit path. A new unregistered Parquet shard never changes a pinned result. Reject proposals using `**/*.parquet` for these operations; test exact registered membership. |
| Project/peer/cache | Resolve from unrelated working directories; test alias/namespace mismatch, unavailable/unconfigured/removed/retargeted peers, withdrawal, and stale state. With a saved physical path still readable, removal of the configured route must make new access fail. |
| Staging | Disposable source/output alias, overlap, receipt collision, interrupted copy, failed overwrite, and receipt-write failure cases preserve provider bytes and recover prior outputs as specified. |
| SDK/HTTP | The same identity, manifest revision, inventory, visibility, and typed failure semantics reach each adapter; pagination or Python cannot bypass core rules. |

## Affected-Surface Completion

Run [Rust checks](../feam-rust-workflow/SKILL.md) for Rust changes and [CLI/protocol checks](../feam-cli-contract/SKILL.md) for CLI changes. SDK-only and HTTP-only work still requires this skill's invariants and its own runtime evidence.

| Surface | Completion evidence |
| --- | --- |
| Python SDK / Polars | Run the SDK's documented tests and build/install commands in a fresh environment; import the installed package outside its source directory. Test the real CLI boundary and optional extras. Assert a native `polars.LazyFrame`, then execute known-value filter/projection queries over only registered Parquet shards; test delayed-read failures. |
| Raster / STAC records | Validate against pinned local schemas/extensions. Resolve a STAC-selected product/version/asset through the client peer route and use Rasterio to read a small band/window, verifying known pixels, CRS, and nodata. Static schema validity does not prove HTTP behavior. |
| STAC HTTP | Use a real standard client such as `pystac-client` against the tokenless `127.0.0.1` server for collection browsing, spatial/time search, empty results, and multiple pages. Test rejected non-loopback binding, invalid queries/cursors, withdrawal/removed peers between pages, and identity/URI round trips into SDK/Rasterio. Exercise every declared conformance class. |
| Native inspectors | Run documented supported feature configurations and native-library versions, including enabled-format validation and required-inspector-unavailable failures. Record exact flags/dependencies as introduced. |
| Target HPC | Record actual separate-user/group allowed/denied access, shared-filesystem publication/lock/rename behavior, and multi-node jobs with separate host/process-local caches. Verify no multi-node shared SQLite WAL cache. Local mocks do not establish this evidence. |

Add real install/test/fixture commands, dependencies, and supported configurations to guidance and CI in the phase introducing each component, before marking it complete. Do not defer this to P8 or invent runnable SDK/STAC commands before components exist. The workplan execution record owns phase evidence.

Report missing tools/dependencies and every unexecuted required check with its unverified behavior. An unavailable cluster leaves HPC acceptance pending while independent implementation continues. Skipped tests, default-only builds, static STAC output, and local permission mocks cannot establish missing runtime or deployment behavior.
