# Peer Data Access Implementation Workplan

Status: ready for P0 implementation after the completed [agent-context preparation check](data_access_agent_context_review.md#preparation-fix-completion). P0–P8 remain not started; preparation validation does not establish runtime or HPC acceptance.

This plan turns [data_access.md](data_access.md) into executable work for an implementation agent. It includes the CLI changes needed to deliver the requirements, as authorized by the user. The intended result is a working Rust implementation, supported Python SDK, STAC HTTP API, and verified notebook/HPC workflows—not another planning document.

## 1. Agent instructions and scope

- Read [AGENTS.md](AGENTS.md) and the applicable repository skills before implementation. Use [feam-peer-data-access](.codex/skills/feam-peer-data-access/SKILL.md) for all publication, resolution, cache, staging, SDK, and STAC work, including adapter-only changes. Also use the Rust workflow skill for Rust changes, CLI contract skill for command changes, and context maintainer skill when workspace layout, CI, or contributor instructions change.
- Treat `data_access.md` as the feature source of truth. Its confirmed publication model is mandatory; its example filenames, API signatures, metadata profile, and command syntax are proposals. This plan supplies implementation defaults for those proposals.
- Implement in `feather-mesh/`. Keep parsing, terminal output, and process exit behavior in `mesh_cli`; shared publication, visibility, resolution, and validation rules in `mesh_core::services`; SQL and row mapping in repositories.
- Inspect current code and working-tree changes before editing. Preserve unrelated work. Older CLI/PDD plans describe an earlier baseline and must not drive this implementation; the [older agent-plan notice](feather-mesh/.agents/feather_mesh_workplan.md) routes to the active sources. Explicit user instructions take precedence; once P0 creates `docs/data_access_contract.md`, it owns settled schema/API and migration decisions.
- Work through the dependency order below. Complete each phase's acceptance checks before calling it done. Record changed files, commands/results, decisions, and remaining blockers in the execution record at the end.
- Resolve routine technical choices using these defaults and repository evidence. Record deviations with their rationale; ask the user only when a material product or deployment decision cannot be resolved from the available context.

Required scope includes direct reads of registered GeoTIFF and Parquet products, safe explicit staging, a Python SDK, and an actual STAC search API. Static STAC JSON alone is insufficient. Native Python bindings, other table formats, mandatory COG conversion, remote asset streaming, and full lineage graph traversal are deferred.

## 2. Current implementation baseline

The following was checked against the source while preparing this plan; it is not a fresh runtime test report.

| Existing location | Current behavior | Required work |
| --- | --- | --- |
| `mesh_cli/src/main.rs` | Implements the nine documented commands, table/JSON output, and exit codes 0–5 | Extend commands for projects, manifests, qualified references, direct resolution, and STAC; define migration behavior |
| `mesh_core/src/services/registry_service.rs` | `serve` inserts a new SQLite product/version; validation primarily checks metadata strings | Stable provider identity, complete per-version metadata, format validation, and authoritative manifest publication |
| `mesh_core/src/domain.rs`, `services/requests.rs`, `models/` | Generic asset type and one source path per version | Separate data kind/format, explicit asset inventory, project context, typed descriptors and errors |
| `mesh_core/src/db.rs`, `repositories/` | SQLite WAL; unique product/version labels and globally unique source paths | Derived project cache and migration strategy; qualified identities must not depend on local integer IDs or globally unique paths |
| `RegistryService::consume` and copy helpers | Copies stored paths; overwrite removes destinations before copying; relative paths are not anchored to publication context | Prevent source/output collisions, preserve existing outputs on failure, and resolve project assets through the shared resolver |
| `mesh_core/tests`, `mesh_cli/tests/cli_workflow_tests.rs` | Existing registry and CLI coverage; CSV fixture | Peer manifests, GeoTIFF/Parquet fixtures, SDK/API integration, concurrency and HPC acceptance |
| `.github/workflows/rust.yml` | Rust build, formatting, Clippy, and tests | Required format-validation dependencies, SDK/API checks, and documented HPC checks |

Do not reimplement existing uniqueness checks blindly. The missing capability is a stable namespace/product identity across publications and immutable registered versions. Also move touched search SQL out of `RegistryService` into repositories rather than extending that existing layering exception.

## 3. Contract defaults to finalize in Phase 0

These are proposed implementation decisions, not claims that every detail was already specified in `data_access.md`.

| Topic | Initial contract |
| --- | --- |
| Project context | Explicit `--project <root>` and `Project.open(root)` anchor all new access. Store namespace, optional local serving root, and configured peer routes in `.feam/project.toml`. Resolve configuration paths relative to the project root, independent of process working directory. A project may both publish and consume. |
| Peer identity | Configure an expected provider namespace for each peer symlink. Validate it against the manifest namespace; the symlink basename is only a local alias. Reject conflicting providers for the same namespace. Symlink creation and grants remain administrative operations. |
| Authoritative record | Versioned `serving/manifest.json`, containing provider identity, manifest revision, registered products/versions, lifecycle state, and explicit assets. SQLite and STAC documents are rebuildable projections. |
| Product/version identity | `product://<namespace>/<dataset-id>` plus a required version for direct reads and staging. Publish another version under the same product identity. Reject duplicate version labels and edits to a published version's inventory or metadata. |
| Asset routes | Store serving-root-relative asset paths. Reject absolute paths, traversal, and unsupported entry types. The configured peer link may intentionally point outside the client project. Initially allow nested links only when their resolved targets remain inside the declared serving root; reject escaping nested links. |
| Lifecycle and freshness | Support active and withdrawn versions, with tombstones and withdrawal reasons. Refresh and new access recheck peer availability. Metadata-only discovery may return explicitly marked stale entries for an available peer; direct resolution verifies the current manifest revision and active version or fails. Never resolve through a cached canonical path after the configured route disappears. |
| Version integrity | Providers promise not to mutate published bytes. Record sizes and publication-time digests by default; allow an explicit recorded digest opt-out for expensive publications. Full integrity verification is opt-in at access time, not required for every window or lazy scan. Never claim membership pinning alone guarantees unchanged bytes. |
| Table profile | Parquet only; consistent physical column names/types across all declared shards. Partition columns, types, and layout are explicit. Record schema fingerprint, row counts where available, meanings, and units or explicit not-applicable/unknown values. |
| Raster profile | Validate GeoTIFF with a raster reader. Record CRS, transform, dimensions, bounds/footprint, bands, types, nodata, and semantics. Require meaningful observation time/interval for the initial STAC profile; missing context must produce an actionable validation error, never an inferred file-modification timestamp. |
| Python integration | Initially use an installed Rust CLI through argument-array subprocess calls and versioned JSON. Rust owns validation and resolution. Package the new SDK under `feather-mesh/python_sdk/`; native bindings can follow later. |
| STAC transport/access | A read-only service bound to loopback with a per-instance bearer token stored in an owner-readable runtime file. Return encoded local file URIs preserving the client peer route, plus product/version/asset identity for SDK resolution. The supported client shares the service's filesystem view. |
| Cache placement | Use a user-private process or host-local SQLite cache with an explicit local cache location. Shared project configuration may refer to the cache policy, but must not cause multiple nodes to share a WAL database. Detect configuration mismatches and document storage prerequisites. |
| CLI compatibility | Preserve existing command names and exit-code meanings where practical. New peer publication requires project context and the full metadata profile. Retain explicit legacy-registry inspection/staging during migration, but never import legacy entries automatically into peer discovery or allow legacy publication to bypass the new gate. |

Record supported manifest/config/JSON schema versions, pinned STAC specification and extension versions, unknown-field handling, cache freshness threshold, and minimum tested Rust/Python/native-library versions before implementing dependent adapters. Verify external specifications against the official sources linked in `data_access.md` when selecting versions.

## 4. Delivery sequence

| Phase | Deliverable | Depends on |
| --- | --- | --- |
| P0 | Written contracts and reproducible fixtures | None |
| P1 | Project context, typed records, safe filesystem primitives | P0 |
| P2 | Atomic typed publication and withdrawal | P1 |
| P3 | Scoped refresh/cache and shared resolver | P2 |
| P4 | Complete CLI workflows and safe managed staging | P3 |
| P5 | Python descriptors and lazy Polars access | P4 |
| P6 | STAC records and Rasterio workflow | P3, P5 |
| P7 | Authenticated STAC HTTP browsing/search | P4, P6 |
| P8 | End-to-end/HPC proof, CI, documentation, handoff | P5, P7 |

This sequence intentionally implements authoritative registration before completing the resolver: tests must not establish a directory-scanning path that later bypasses publication. P0/P1 may use schema-valid manifest fixtures to exercise path handling. Wire the minimum CLI needed for each phase incrementally; P4 completes its public contract.

### P0 — Establish contracts and acceptance fixtures

Tasks:

- [ ] Recheck the baseline and run existing Rust checks from `feather-mesh/`; record any pre-existing failures.
- [ ] Write `docs/data_access_contract.md` covering the decisions in section 3, complete metadata schemas, lifecycle, compatibility, cache placement, concurrency, and error mappings.
- [ ] Specify required reuse metadata: namespace, stable ID, name/version, description, intended use, limitations or explicit none, owner team, producer, contact, usage policy, classification, quality, asset roles, publication timestamp, and pinned upstream references when available. Permit explicit empty lineage. Freeze this context with each version.
- [ ] Choose and prove publication inspection libraries for Parquet footers and GeoTIFF metadata. Rust service calls must enforce both validators. If a capability is unavailable in a build/runtime, typed publication fails clearly instead of accepting unchecked data.
- [ ] Define the STAC Collection/Item/Asset mapping, multi-file granule grouping, temporal rules, conformance classes, and geometry transformation policy. Record exact supported specification/schema versions.
- [ ] Add tiny, reproducible GeoTIFF and Parquet fixtures under `mesh_core/tests/data/peer_access/`, with generation instructions and expected pixels/rows/schema/CRS. Include two table shards, multiple raster assets/granules, and a partitioned example.
- [ ] Define invalid fixtures/cases: renamed CSV, corrupt Parquet footer, incompatible shards, non-georeferenced TIFF, incomplete metadata, unregistered files, malformed manifest, traversal, broken/cyclic links, and namespace mismatch. Generate pathological filesystem cases in temporary directories.
- [ ] Document runnable fixture-generation and inspector setup/check commands, tested native-library versions, and supported feature configurations; add applicable checks to CI as introduced. Do not wait for P8.

Acceptance: contracts contain no unresolved implementation-blocking choices; the fixtures have independently known expected values; no production/HPC data is needed for ordinary CI tests.

### P1 — Add project models and filesystem correctness

Suggested locations: new `mesh_core/src/project.rs`, domain/model modules, `services/project_service.rs`, and a filesystem helper module; update `lib.rs` exports.

Tasks:

- [ ] Implement project config loading and initialization, namespace/product reference parsing, expected peer identity, local serving-root handling, and project-relative paths. Support a local provider and configured peers through the same identity model.
- [ ] Add typed manifest, publication, asset, raster/table, provenance, lifecycle, and resolved-descriptor models. Keep `asset_type`, `data_kind`, and `data_format` distinct. Version serialized contracts.
- [ ] Add field-specific validation and typed errors for unregistered/unknown/withdrawn versions, unsupported formats, bad metadata, unavailable peers, stale state, permission denial, integrity failure, and publication conflict. Keep exit codes outside core.
- [ ] Implement route validation using filesystem metadata and canonical identity checks while returning paths through the project route. Check every registered asset against the serving root and nested-link policy; reject traversal, cycles, dangling routes, unsupported file types, and namespace collisions.
- [ ] Add copy preflight helpers for source/output identity, symlink and hard-link aliases, ancestor/descendant overlap, dangling destination links, and receipt/source collisions. Do not remove any source or destination during validation.
- [ ] For new legacy registrations, anchor source paths at registration. Require an explicit migration base for old relative paths; never reinterpret an ambiguous old path against a consumer's current directory.

Acceptance: resolution/path helpers behave identically from two unrelated working directories; a peer root outside the project is allowed; an escaping nested link fails; rejected copy destinations leave source and existing output untouched.

### P2 — Implement authoritative publication

Suggested locations: `services/publication_service.rs`, format inspectors, manifest persistence module, and service tests. Keep manifest I/O separate from SQL repositories.

Tasks:

- [ ] Validate all required metadata in the service shared by `serve` and `validate-metadata`. Verify provider namespace, owning-team context, and write permissions against configured provider context; do not treat arbitrary supplied strings as authorization.
- [ ] Require an explicit complete asset inventory beneath the serving root. A provider helper may propose files, but publication must capture a fixed list. File presence and subsequent directory changes never publish data.
- [ ] Open every declared Parquet footer, check schemas across shards, validate partition declarations, and capture physical/semantic descriptors. Document that footer validation does not verify all data pages.
- [ ] Open every declared GeoTIFF, extract technical metadata, validate scientific metadata and granule/asset relationships, and reject extension-only or incomplete raster claims. Ordinary valid GeoTIFF remains supported.
- [ ] Compute configured digests and sizes and check for changes during validation/publication. Document the remaining producer immutability obligation.
- [ ] Implement new-product and new-version publication. Enforce qualified identity/version uniqueness without imposing the legacy globally unique source-path rule on the new manifest model.
- [ ] Coordinate concurrent writers with a lock protocol validated on the target filesystem. Under the lock, reload/check the expected revision; commit via a same-directory temporary file, flush, and atomic replacement. Readers see an entire old or new manifest. Define lock timeout and recovery; unsupported locking must fail clearly.
- [ ] Make the manifest commit the publication point. Derived cache/STAC failures after commit report successful publication with a rebuildable projection failure, not a false rollback. Handle retry after an uncertain commit without creating duplicate versions.
- [ ] Add withdrawal as a revisioned tombstone operation; retain provenance, never delete provider bytes, and reject future resolution of withdrawn versions.
- [ ] Add failure-injection and concurrent-writer tests covering validation failure, interrupted writes, duplicate publication, revision conflict, and projection failure. Verify failed registrations leave the manifest revision and visible entries unchanged.
- [ ] Run and document the supported inspector feature configurations, including enabled-format validation and explicit failure when a required inspector is unavailable; add them and native dependencies to CI before completing P2.

Acceptance: raw TIFF/Parquet files remain invisible; incomplete metadata and invalid formats cannot publish through any service/CLI entry point; two valid versions retain one product identity; publication never exposes partial inventory or loses another writer's update.

### P3 — Implement scoped cache and one shared resolver

Suggested locations: `services/catalog_service.rs`, `services/resolution_service.rs`, cache schema/repositories, and focused integration tests.

Tasks:

- [ ] Build/rebuild the derived cache only from validated registered entries in the local serving manifest and configured peers. Track provider identity, revision, cache schema version, refresh time, and per-peer refresh outcome.
- [ ] Keep cache storage private and node/process local under the deployment contract. Separate legacy authoritative registry tables from new derived cache tables; document migration and reject unsupported schema versions without overwriting user data.
- [ ] Refresh each provider snapshot transactionally. Surface malformed/unavailable peers without mixing partially imported revisions. A last-good snapshot may support explicitly stale metadata discovery only under the documented policy.
- [ ] Recheck current project configuration and peer route availability for discovery and new access. Removed peers must disappear from new results, including API pagination; withdrawn versions must stop resolving. Define behavior for a link retargeted to another provider/root.
- [ ] Implement `resolve(project, qualified_reference, version)` and typed asset/table helpers. Verify current registration/lifecycle, then return exactly the registered asset list through the client route. No directory globs or provider-absolute fallback paths.
- [ ] Return namespace, product/version, manifest revision, kind/format, asset IDs/routes/media types/roles/sizes/digests, format descriptors, usage/ownership context, lineage, cache timestamps, and freshness/integrity status.
- [ ] Distinguish an unregistered readable path from an inaccessible registered asset and a stale catalog. Explicit path lookup, if supported, only matches registered inventory and returns `dataset_not_registered` otherwise.
- [ ] Support optional integrity verification without forcing a full data read on every normal resolve. Return enough provenance for notebooks/jobs to persist an access record.

Acceptance: the same reference resolves from unrelated working directories; unconfigured, removed, retargeted, broken, and cyclic peer routes fail according to policy; refresh never includes unregistered files; cache loss is recoverable entirely from manifests.

### P4 — Deliver the CLI and safe explicit staging

Finalize syntax in P0 and implement it in `mesh_cli`. The following proposed surface is part of this plan, not a list of commands already available.

| Command | Required behavior |
| --- | --- |
| `feam init --project ROOT --namespace NAME [--serving-dir PATH]` | Create project configuration; allow provider/client/both roles; refuse destructive reinitialization |
| `feam serve PATH --project ROOT --metadata FILE` | Register the explicitly declared typed version using the common publication service; extend existing flags where useful without duplicating validation |
| `feam validate-metadata FILE --project ROOT` | Validate the publication profile and declared assets through the same rules without publishing |
| `feam refresh --project ROOT` | Refresh configured manifest projections and report per-peer freshness/errors |
| `feam cache status --project ROOT` | Report cache placement, revisions, and freshness without exposing secrets |
| `feam search / show / products / lineage --project ROOT ...` | Use scoped registered records; accept qualified references where relevant; expose version and provenance |
| `feam resolve REF --project ROOT --version VERSION [--asset ID]` | Return a pinned descriptor without copying; support a versioned JSON/error contract for the SDK |
| `feam consume REF --project ROOT --version VERSION --out PATH [--overwrite]` | Stage only registered assets using the same resolver, then write a provenance receipt |
| `feam withdraw REF --project ROOT --version VERSION --reason TEXT` | Publish a withdrawal tombstone under provider authorization |
| `feam stac serve --project ROOT --token-file PATH` | Run the distinct read-only STAC service added in P7; never overload publication `serve` |

Tasks:

- [ ] Define argument conflicts and precedence for `--project` and legacy `--registry`. Peer commands must not silently create/use `registry.db` in the working directory. Keep administrative team listing consistent with the selected context.
- [ ] Retain exit meanings: 0 success, 1 runtime, 2 CLI usage, 3 validation, 4 not found, 5 permission/policy. Map new domain errors explicitly; use machine-readable error kinds to distinguish failures sharing an exit code.
- [ ] For SDK-facing JSON mode, emit only result JSON on stdout and a documented structured error on stderr; keep diagnostics separate. Version the contract and preserve existing output fields where compatible. Add migration examples for intentional changes.
- [ ] Make safe staging use P1 preflight checks, temporary destinations, cleanup, and commit/recovery semantics. Preserve an existing destination until replacement data is ready. Specify recovery if receipt writing fails; never report complete success without its required receipt.
- [ ] For directories and multi-asset products, copy exactly the resolved inventory. Apply the same link policy and recheck routes before reads. Write a receipt with qualified identity, version, manifest revision, assets, timestamp, and available digests.
- [ ] Cover project initialization through publish/refresh/resolve/stage/withdraw in `mesh_cli/tests/cli_workflow_tests.rs`, including JSON/table output, bad arguments, permission errors, and safe-overwrite regressions.
- [ ] Document how the SDK finds the executable: user-facing name is `feam`, while the existing Cargo binary artifact is `mesh_cli`. Provide an explicit configurable binary path and a tested development/install command.

Acceptance: a fresh temporary provider/client pair can complete the workflow using only documented CLI operations plus administratively prepared symlinks; no direct database editing is required; direct resolution performs no copy; failed staging never changes provider files.

### P5 — Add the Python SDK and lazy table access

Suggested package: `feather-mesh/python_sdk/`, with `pyproject.toml`, `src/feam/`, and `tests/`. Keep the subprocess adapter small and replaceable by later native bindings.

Tasks:

- [ ] Implement `Project.open(root)`, `resolve`, `resolve_asset`, `resolve_table`, and `scan_table`, with typed descriptors and Python exceptions mapped from Rust error kinds. Require an explicit version for data access.
- [ ] Invoke the CLI with an argument array, explicit project root, JSON output, and configurable executable path; never use a shell. Check protocol compatibility and handle missing executable, failed commands, and malformed responses.
- [ ] Keep namespace, publication, path, and lifecycle rules in Rust. Python translates descriptors and calls data libraries; it must not reconstruct visibility by scanning directories.
- [ ] Return the registered explicit paths from `resolve_table`. Implement `scan_table` using `polars.scan_parquet(paths, glob=False)` and return the native `LazyFrame` without collecting or converting through CSV/row lists.
- [ ] Apply the declared partition contract explicitly. Test physical/partition column conflicts and schema compatibility; enable Hive-style pruning only once its behavior is covered.
- [ ] Provide optional table and raster/STAC dependency extras. Basic descriptor resolution must not require all scientific libraries.
- [ ] Document and test delayed-read behavior: paths, bytes, or permissions may change after query construction; errors can occur during `.collect()`. Do not promise a Rust recheck before every read of a native lazy query.
- [ ] Add known-value filter/projection/aggregation tests through a peer link, asserting lazy return type, fixed shard membership, pinned versions, paths with spaces, and unrelated-shard exclusion.
- [ ] Add concrete SDK build/install/test commands and optional-extra coverage to its documentation and CI. Test a fresh installation imported outside the source tree and real subprocess success/error behavior, including missing executable, incompatible protocol, malformed JSON, and relevant domain-error categories.

Acceptance: an installed SDK creates a native lazy Polars query over registered Parquet shards, returns expected results, and uses the same Rust errors/identity as the CLI. A newly added unregistered shard never changes the pinned result.

### P6 — Generate STAC records and prove raster reads

Suggested locations: shared STAC projection code over core descriptors, raster fixtures/tests, and SDK examples. Keep derived STAC metadata non-authoritative.

Tasks:

- [ ] Generate namespace-qualified Collections and version-distinguishing Items for registered raster records. Represent scientific granules and related data/mask/quality Assets explicitly; support one Item with multiple Assets and multiple Items per product where appropriate.
- [ ] Populate valid geometry/bbox, temporal properties, band/projection metadata, asset media types/roles, and namespaced Feather Mesh ownership/version/provenance. Transform footprints as required by the pinned STAC profile while retaining native CRS information.
- [ ] Validate outputs against the pinned schemas/extensions. Use locally available schema fixtures for repeatable CI rather than relying on live schema downloads.
- [ ] If exporting static documents, calculate asset paths relative to each Item document and test access through two different client symlink layouts. Regenerate projections from the manifest; do not allow independent STAC edits to become publication.
- [ ] Round-trip STAC product/version/asset identity into `Project.resolve_asset`. For HTTP responses, implement and test local file URI encoding/decoding for spaces, Unicode, and reserved characters; do not treat an HTTP-relative link as a filesystem path.
- [ ] Add a Rasterio example/test that resolves a selected Asset, opens its client path, reads one band and a small window, and verifies pixel values, CRS, and nodata. Avoid an implicit whole-raster read.
- [ ] Add the pinned-schema, fixture-generation, and Rasterio integration commands/dependencies to documentation and CI before completing P6.

Acceptance: schema-valid STAC records exist only for registered rasters; distinct versions/granules have distinct identities; a STAC-selected asset resolves to the expected window through the client peer route.

### P7 — Expose the STAC HTTP API

Suggested layout: a `mesh_stac` adapter crate added to the Rust workspace, with CLI startup in `mesh_cli`. Core services remain independent of the HTTP framework.

Tasks:

- [ ] Implement the pinned conformance classes, including landing/conformance documents, Collection browsing, Item retrieval, and Item Search with spatial/temporal filtering and pagination. Implement the methods and parameters required by the declared classes; reject unsupported queries clearly.
- [ ] Query the project-scoped cache through core services/repositories. Apply current visibility/lifecycle checks before returning Items and Assets, including subsequent pages. Do not expose unrelated cache namespaces.
- [ ] Define deterministic ordering and opaque pagination tokens tied to the query and catalog snapshot. On intervening publication/withdrawal, either preserve a permitted snapshot or require a documented restart; never leak withdrawn or newly unavailable peers through an old cursor.
- [ ] Require the per-instance bearer token on catalog/search endpoints, store it outside shared project files with restrictive permissions, and keep it out of logs and pagination links. Loopback is the initial binding default, not the access-control mechanism.
- [ ] Return local file URIs and resolvable identities under the shared-filesystem transport contract. Serve metadata only; raster byte proxying is deferred.
- [ ] Add a `pystac-client` integration test with authentication, collection selection, spatial/time filters, empty results, and multiple pages; pass a discovered identity through the SDK to Rasterio.
- [ ] Test unauthenticated/invalid-token access, invalid query/limit/cursor, refreshed publication, withdrawal, removed links, and peer failures. Report freshness without claiming unavailable data is readable.
- [ ] Document the real-client HTTP/conformance test commands and required dependencies and run them in CI before completing P7. Static STAC JSON cannot establish runtime search behavior.

Acceptance: a standard STAC client performs authenticated browsing and paginated spatial/temporal search, then reads the correct registered raster through local resolution. All declared conformance behavior is tested; static JSON files do not satisfy this phase.

### P8 — Prove complete workflows and prepare handoff

Tasks:

- [ ] Automate the complete scenario in section 6 using fresh temporary provider/client projects and the real built CLI/SDK/API.
- [ ] Verify the CI checks/dependencies added during P0–P7 together and extend them for the complete workflow. Keep default core/CLI tests meaningful; do not silently skip required adapters because dependencies are missing.
- [ ] Add notebook and batch-job examples with explicit project roots, versions, cache locations, token handling, provenance recording, bounded raster windows, and lazy table queries. Explain memory limits and optional streaming/sinks without claiming laziness alone bounds memory.
- [ ] Run a separate target-HPC checklist with distinct producer/consumer users or groups, permitted and denied access, publication during discovery, multi-node jobs with separate local caches, and measured window/table-query behavior.
- [ ] Verify the chosen manifest lock/rename protocol on the actual shared filesystem. Confirm readers never observe partial records and caches do not share a multi-node WAL database.
- [ ] Update the Rust README, new SDK README, format/profile and migration docs, fixture instructions, and affected agent context/skills. Explain direct read versus explicit staging and the limits of link removal for already-open handles or loaded data.
- [ ] Record real commands, environment/library versions, outcomes, and any unexecuted HPC checks. If the target cluster or separate identities are unavailable, finish local deliverables and explicitly leave HPC acceptance pending; do not substitute local mocks as proof of cluster behavior.

Acceptance: CI covers both access paths and the publication gate; onboarding works from a clean checkout; target-HPC evidence is attached before claiming all requirements complete.

## 5. Requirement-to-acceptance map

| Requirement from `data_access.md` | Delivery | Required evidence |
| --- | --- | --- |
| §§1–2: provider serving directory and explicit registration | P1–P3 | Readable but unregistered TIFF/Parquet absent from CLI, SDK, table discovery, and STAC; failed registration leaves no entry |
| §2: complete self-service metadata and format validation | P0, P2 | Field-specific missing-context failures; fake/unsupported formats rejected by service and CLI; complete descriptors survive round trip |
| §§2–3: namespace identity and configured peer visibility | P1, P3, P4 | Alias differs from namespace; unconfigured/mismatched/removed peers rejected; dual provider/client role works |
| §3: one pinned resolver and project-local access route | P3–P7 | CLI/SDK/STAC agree on identity, revision, inventory and paths; different working directories return the same product |
| §4: valid GeoTIFF and scientifically meaningful STAC | P2, P6 | Extracted CRS/bands/nodata; valid time/geometry; multiple granules/assets; known window/pixels verified |
| §4: actual scoped STAC API | P7 | Standard client uses authentication, browsing, spatial/time search, and pagination; URI conversion is tested |
| §5: Parquet-only lazy Polars access | P2, P5 | Footer/schema checks, declared partitions, native lazy query, expected rows/columns, unrelated shard excluded |
| §6: supported Python access over Rust rules | P4–P5 | Fresh SDK install, versioned JSON/error contract, optional extras, no Python namespace reimplementation |
| §§7–8: safe paths and managed copy | P1, P4 | Equality/alias/overlap/dangling-link/receipt collision failures preserve bytes; failed overwrite preserves prior destination |
| §8: version integrity, lazy-read limits, lifecycle | P2–P5 | Duplicate-version rejection, manifest revision/provenance, optional digest mismatch, withdrawal, and delayed execution failures |
| §8: concurrent publication and HPC cache placement | P2–P3, P8 | Fault/concurrency tests plus actual filesystem verification; isolated per-host/process caches |
| §9: full provider/client demonstration | P8 | Complete scenario below and separate-user/group HPC evidence |

## 6. Mandatory end-to-end scenario

1. Create a provider with a serving directory and a client with an administratively created peer link. Use a local alias different from the provider namespace; run client commands from outside its project directory.
2. Place a GeoTIFF and two Parquet shards in the serving directory. Confirm neither is discoverable or resolvable through Feather Mesh, including explicit path requests.
3. Attempt publication with missing required metadata, a renamed CSV, and incompatible Parquet schemas. Confirm actionable errors and an unchanged authoritative manifest.
4. Register complete raster/table versions; refresh the client. Assert stable namespace/product/version, manifest revision, metadata, and exact asset IDs/paths.
5. Start the authenticated STAC endpoint. Discover by spatial extent and time with a standard client, resolve the selected asset, and verify a known Rasterio band/window.
6. Resolve the table through the SDK, construct a lazy Polars filter/projection, and verify expected values on execution. Add an unregistered shard and confirm the pinned version remains unchanged.
7. Publish a second version under the same product and verify the original version still resolves its original inventory. Reject attempts to replace its registered metadata/assets. Exercise optional digest verification after a deliberate fixture mutation.
8. Safely stage one pinned version and validate its receipt. Attempt source/output aliases, overlaps, and a failing overwrite; verify provider files and previous outputs remain intact.
9. Withdraw a version, remove a peer link, deny access, and refresh while another publication runs. Verify documented lifecycle/freshness errors, no fallback to a physical provider path, no cross-peer leakage, and no partial publication.
10. Save the access provenance and test report. Repeat the applicable workflow on the target HPC filesystem with separate users/groups and node-local caches.

## 7. Validation and completion

During implementation, run the narrowest useful checks, then complete the repository-required checks from `feather-mesh/`:

```bash
cargo test -p mesh_core
cargo test -p mesh_cli
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
```

Add the concrete SDK, STAC schema/conformance, HTTP integration, and fixture-generation commands to CI and documentation as those components are introduced, before claiming the affected phase complete. Exercise documented supported feature configurations if optional Rust features are used, including failure without required inspectors. Report missing tools/dependencies and every unexecuted required check explicitly. Keep automated local evidence separate from actual target-HPC results. No runtime checks were run merely to author this workplan.

The implementation is complete only when every required phase passes, section 5 has test/evidence links, the mandatory scenario succeeds, required HPC checks have actual results, migration is documented, and no adapter bypasses core publication/visibility rules. Report locally verified delivery separately from pending deployment evidence when necessary.

## 8. Execution record for the implementing agent

Update this table after each phase; link tests, decisions, and reports rather than marking a phase complete based on code presence alone.

| Phase | Status | Changed files / decisions | Validation evidence / remaining blockers |
| --- | --- | --- | --- |
| Preparation | Complete | Source routing, shared invariant skill, affected-surface validation, CLI protocol/migration guidance, checker and discovery | [Preparation evidence](data_access_agent_context_review.md#preparation-fix-completion); runtime phases and target-HPC acceptance remain pending |
| P0 | Not started | — | — |
| P1 | Not started | — | — |
| P2 | Not started | — | — |
| P3 | Not started | — | — |
| P4 | Not started | — | — |
| P5 | Not started | — | — |
| P6 | Not started | — | — |
| P7 | Not started | — | — |
| P8 | Not started | — | — |
