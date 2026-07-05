# Feather Mesh Project Exit Outcomes

Source: `feam_pdd.md`

These outcomes define what the project must demonstrate at exit to be considered a successful V1 delivery. They are written as observable product outcomes rather than implementation tasks.

## 1. Publish Reusable Data Products

Feather Mesh must allow a producer to publish a reusable asset as a versioned data product without moving ownership of the underlying files.

Exit outcomes:

- A producer can run `feam serve <path>` against a POSIX-like filesystem path.
- The published product records an identity, name, asset type, owner team, producer, created timestamp, version, and source path or source reference.
- Supported asset categories include at least files, directories, model artifacts, report artifacts, and manifest-backed collections at the metadata level.
- Publication fails before persistence when required keystone metadata is missing or invalid.
- Product publication keeps source data in its original storage location.

## 2. Enforce Keystone Metadata Quality

Feather Mesh must make metadata completeness a hard product requirement rather than optional documentation.

Exit outcomes:

- The required keystone metadata contract is represented in code and tests:
  - `product_id`
  - `name`
  - `asset_type`
  - `owner_team`
  - `producer`
  - `created_at`
  - `version`
  - `source_path` or source reference
  - `intended_use`
  - `input_dependencies`
  - `usage_policy`
  - version-level `classification`
  - `data_quality`
- `data_quality` accepts only `production`, `qualified`, and `unverified`.
- Invalid quality labels such as `gold`, `silver`, or `bronze` are rejected.
- A user can validate metadata independently through `feam validate-metadata <metadata_file>`.
- Metadata validation errors are specific enough for a producer to fix without reading source code.

## 3. Provide Deterministic Discovery

Feather Mesh must let consumers find reusable assets through structured catalog metadata instead of ad hoc filesystem path knowledge.

Exit outcomes:

- A consumer can run `feam search <query>` and receive matching products from the local registry.
- Search supports deterministic filters for fields such as asset type, owner team, data quality, access classification, and metadata key/value pairs.
- Search behavior is metadata-driven keyword and faceted search only; semantic retrieval is not required for V1.
- Empty, single-result, and multi-result searches are handled explicitly.
- Search results expose enough summary information for a consumer to decide whether to inspect a product.

## 4. Inspect Products, Versions, and Reuse Context

Feather Mesh must allow consumers to understand what a product is before retrieving it.

Exit outcomes:

- A consumer can run `feam show <product_id>` to inspect product metadata.
- A consumer can pin inspection to a version with `feam show <product_id> --version <v>`.
- Product inspection shows identity, owner, producer, asset type, version, source reference, intended use, policy metadata, access classification, and data quality.
- Metadata snapshots are tied to product versions so consumers can inspect what was known at publish time.
- Product summaries are assembled through service workflows rather than direct CLI repository access.

## 5. Support Explicit Versioning and Immutable Published Versions

Feather Mesh must support reproducible use of published assets through explicit version references.

Exit outcomes:

- A product can have one or more explicit versions.
- Publishing a changed asset, changed dependency, changed processing logic, or critical metadata update creates a new version.
- Published versions are immutable through the V1 workflow.
- Older versions remain referenceable unless removed by explicit policy.
- Automation can use pinned product/version references instead of relying on floating path conventions.

## 6. Surface Lineage and Dependency Context

Feather Mesh must expose provenance enough for consumers to evaluate trust and reuse risk.

Exit outcomes:

- A producer can attach upstream input dependencies to a product version.
- A consumer can run `feam lineage <product_id>` and inspect upstream dependencies and publication context.
- A consumer can request lineage for a specific version with `--version <v>`.
- Lineage includes producer identity, upstream product or source references where available, and publication timing.
- Missing lineage is represented explicitly rather than silently omitted.

## 7. Consume Products Through Managed Copy

Feather Mesh must implement the V1 default retrieval mode as managed copy into a downstream workspace.

Exit outcomes:

- A consumer can run `feam consume <product_id> --version <v> --out <path>`.
- Consumption resolves the product/version reference to a concrete source path.
- Consumption checks that the source exists and is readable through the caller's filesystem permissions.
- Files and directories can be copied to the requested output path.
- Existing output paths are not overwritten unless an explicit overwrite flag is provided.
- A receipt or equivalent metadata artifact is written with product id, version, source path, retrieval timestamp, and checksum where available.
- Reference-only retrieval is not the default V1 behavior.

## 8. Preserve Filesystem-Native Access Boundaries

Feather Mesh must coordinate discovery and retrieval without replacing the HPC filesystem security model.

Exit outcomes:

- Feather Mesh does not bypass POSIX permissions, filesystem ACLs, ownership, or group access rules.
- Products resolve only to paths the caller can access through the underlying filesystem.
- Permission failures produce actionable errors.
- Cross-team sharing remains compatible with filesystem-native linking and permission workflows.
- Access classification is recorded as metadata and does not imply filesystem access by itself.

## 9. Provide a CLI Surface Suitable for Humans and Automation

Feather Mesh must deliver a CLI-first V1 workflow that works in scripted HPC jobs.

Exit outcomes:

- `feam --help` shows the V1 command surface:
  - `serve`
  - `search`
  - `show`
  - `consume`
  - `lineage`
  - `validate-metadata`
- Each core command supports non-interactive execution with explicit parameters.
- Machine-readable output such as `--json` is available where practical for automation.
- CLI parsing and terminal UX remain in `mesh_cli`.
- Business workflows remain in `mesh_core::services`.
- CLI-facing workflows do not call repositories directly.

## 10. Maintain a Local Registry Source of Truth

Feather Mesh must keep catalog metadata in a queryable registry that is usable within the V1 single-cluster scope.

Exit outcomes:

- The V1 registry is backed by SQLite.
- The registry stores teams, products, product versions, metadata, and lineage dependencies.
- SQL query behavior lives in repository modules or database modules.
- Service workflows map repository failures into domain-level errors.
- Registry initialization is tested.
- The system remains scoped to one cluster and one POSIX-like filesystem for V1.

## 11. Provide Auditability for Critical Actions

Feather Mesh must record enough operational history to support governance and troubleshooting.

Exit outcomes:

- Publish, read/show, metadata mutation, and consume actions produce audit records or equivalent structured logs.
- Audit records include actor, action, target product/version where applicable, timestamp, and outcome.
- Metadata policy fields are visible during publication and inspection.
- Governance operators can identify who published or consumed a product version.
- Audit behavior is covered by tests or a documented manual verification flow.

## 12. Deliver Stable Operational Behavior

Feather Mesh must be reliable enough for repeatable HPC workflows.

Exit outcomes:

- Catalog operations are idempotent where reasonable.
- Managed-copy failures distinguish missing product, missing version, missing source, permission failure, and existing destination.
- CLI failures are stable and actionable enough for scripts.
- Retrieval either completes successfully or leaves clear state for retry.
- `cargo test` passes from `feather-mesh/` at project exit, or any failures are documented with accepted rationale.

## 13. Demonstrate End-to-End V1 Workflow

Feather Mesh must demonstrate the full producer-to-consumer loop.

Exit outcomes:

- A producer can publish a valid product with required metadata.
- A consumer can discover that product with search.
- A consumer can inspect the product and its version metadata.
- A consumer can inspect lineage for the selected version.
- A consumer can consume a pinned version into a local output path.
- The workflow is reproducible from documented commands in the repository.

## 14. Document Scope, Non-Goals, and Contributor Guidance

Feather Mesh must leave the project in a state that future contributors can continue without rediscovering product decisions.

Exit outcomes:

- The README identifies the CLI crate, core crate, product definition document, and test command.
- Architecture documentation explains the split between `mesh_cli`, `mesh_core::services`, and repositories.
- The V1 boundaries are documented:
  - single cluster,
  - single filesystem,
  - SQLite-backed local registry,
  - managed-copy default consumption,
  - metadata-driven search only,
  - filesystem-native access enforcement.
- Non-goals are documented, including no universal data lake, no scheduler replacement, no semantic retrieval in V1, and no replacement for filesystem access control.

## 15. Define Success Signals for Project Review

Feather Mesh must provide measurable signals that the V1 satisfies the PDD's intended value.

Exit outcomes:

- At least one fixture or demo workflow shows product publication volume greater than zero.
- At least one cross-team-style discovery and consumption scenario is demonstrated with separate producer and consumer roles or fixtures.
- Keystone metadata completeness is measurable across registered products.
- Pinned version consumption is demonstrated.
- Retrieval failures can be categorized by cause class.
- The project can explain how V1 reduces reliance on ad hoc paths, stale wiki documentation, and producer-to-consumer manual handoff.
