# Feather Mesh Product Definition Document (PDD)

## 1. Purpose and Overview
High Performance Computing (HPC) environments generate high-value datasets, model artifacts, and analysis outputs across many teams. In most organizations, those outputs are hard to discover and reuse because they are distributed across team-owned storage locations, named inconsistently, and documented unevenly.

Feather Mesh (`feam`) is an HPC-native middleware layer that standardizes how teams publish, discover, and consume reusable data products without forcing teams to give up ownership of their data. It is intended to reduce duplicated work, improve cross-team interoperability, and make pipelines more reliable by replacing ad hoc path conventions with a governed product catalog and deterministic retrieval workflows.

## 2. Why Feather Mesh Instead of Shared Paths, Shared Filesystems, or Wiki Documentation
Shared filesystems are necessary infrastructure, but they only provide storage and transport. They do not provide standardized metadata, stable discovery interfaces, version selection rules, or consistent provenance visibility. As a result, users can access files but still cannot confidently determine which asset is fit for downstream use.

Wiki-style documentation helps with human context, but it is not a reliable system of record for automation. Documentation can drift from actual outputs, is rarely validated against current assets, and is difficult to query with strict filters inside production workflows.

Feather Mesh adds the missing coordination layer on top of existing storage systems: producers publish data products with required metadata, consumers discover products through queryable metadata, and pipelines resolve product references deterministically.

## 3. Scope, Boundaries, and Non-Goals
In this document, middleware means a coordination layer between data-producing workflows and data-consuming users or workflows. Feather Mesh registers reusable outputs as data products, stores and indexes metadata, and resolves product references to retrievable assets.

Feather Mesh catalogs products and metadata, provides discovery and retrieval resolution, and validates publication metadata against governance rules. Access enforcement remains delegated to the underlying filesystem. Feather Mesh does not replace enterprise storage systems, does not act as a heavy transformation engine, and does not guarantee scientific correctness of the underlying content.

Non-goals for V1 include building a universal data lake, replacing schedulers or workflow orchestrators, serving as a general documentation platform, and eliminating team autonomy over raw storage layout.

## 4. Core Definitions
### 4.1 Data product
A data product is a reusable, versioned asset (or asset set) plus keystone metadata that allows another user or pipeline to understand, discover, trust, and consume it without direct producer involvement.

### 4.2 Supported asset types
Feather Mesh supports registration of single files, structured directories, logical collections, model artifacts (for example checkpoints or packaged weights), report artifacts (for example PDF/HTML bundles), and manifest-backed virtual assets that reference many underlying objects.

### 4.3 Keystone metadata
Keystone metadata is the minimum metadata set required for reliable discovery and safe reuse. Required fields are `product_id`, `name`, `asset_type`, `owner_team`, `producer`, `created_at`, `version`, `source_path` (or source reference), `intended_use`, `input_dependencies` (empty list allowed), `license_or_usage_policy`, `access_classification`, and `data_quality`.

The `data_quality` field uses a fixed three-tier model: `production` (highest confidence), `qualified` (appropriate to use with caveats), and `unverified` (quality cannot be guaranteed).

Optional metadata can include domain tags, region or program context, model family, performance summary, retention notes, and contact channels. To increase adoption and consistency, Feather Mesh auto-extracts as much low-risk technical metadata as possible, such as file size, checksum, format, basic schema signatures, and timestamps. Manual input is reserved for high-context fields such as intended use, caveats, and policy interpretations.

### 4.4 Technical meaning of core actions
In Feather Mesh terminology, serving means registering and publishing a data product with metadata and policy controls. Consuming means resolving a product reference and retrieving or mounting the associated asset for downstream usage. Retrieval is the materialization step into a local path or workflow workspace, while discovery is search and filtering across catalog entries by metadata, policy, and relevance.

## 5. Users, Roles, and Access Controls
Feather Mesh is designed for data producers (research and engineering teams, including automated workflows), data consumers (analysts, modelers, and downstream pipelines), and platform governance operators.

The role model includes producer, consumer, maintainer, and admin. Serving and consuming are not exclusive; the same principal can hold both producer and consumer responsibilities. Producers can publish products in owned namespaces, consumers can discover and retrieve products allowed by policy, maintainers can manage schema and lifecycle rules, and admins hold platform-level operational authority.

Access control enforcement is delegated to the underlying filesystem permissions model. Feather Mesh does not override filesystem ACLs, POSIX ownership, or group-level access rules; it resolves and surfaces only what the caller is already permitted to access through the filesystem.

Cross-group sharing is implemented through filesystem-native linking patterns. For example, if Group A wants to share a product with Group B, Group A exposes a directory within its workspace to Group B using a symbolic link workflow and grants Group B read and execute permissions on the shared target directory. Feather Mesh then catalogs and serves references to that shared location, but effective access remains determined by filesystem permissions.

Governance still requires ownership and usage-policy metadata, audit logging for critical actions, and explicit `data_quality` assignment. Promotion to `data_quality=production` is self-certified by producer role in V1, while maintainers and admins can audit or correct metadata state when governance violations are identified.

## 6. Metadata and Discoverability
Feather Mesh uses a structured schema spanning identity, technical descriptors, governance fields, and reuse context. At serve time, keystone metadata is mandatory and validated before catalog publication.

The metadata source of truth is a central registry backed by a catalog database. A derived search index supports V1 keyword and faceted search. Semantic retrieval is explicitly out of scope for V1 and planned as a later capability. Metadata snapshots are tied to product versions so consumers can inspect what was known at publish time.

Keystone metadata aligns to a Feather Mesh DCAT profile, meaning DCAT-aligned core fields are extended with HPC-specific metadata required for practical reuse and governance. Metadata is considered sufficient when a new consumer can determine what the asset is, whether it is the right version and quality tier, whether they are authorized to use it, and how to retrieve it safely.

## 7. Versioning and Lineage
Feather Mesh supports explicit versioning. A new version is required when underlying content changes, dependency inputs change, processing logic materially changes outcomes, or critical interpretation/policy metadata changes.

Published versions are immutable. Superseding a version requires publishing a new one. Older versions may be archived but remain referenceable for reproducibility unless policy mandates removal.

Lineage tracks upstream product dependencies, producer identity, workflow context, and publication timing. Consumers can inspect provenance before retrieval. Reproducibility is a core objective for production-grade use, so pinned version references are recommended in automation while floating references such as `latest` are limited to exploratory workflows.

## 8. User Flows (Serving and Consuming)
In the serving flow, a producer selects an asset path or reference, provides required metadata, passes policy and schema validation, and publishes a version that becomes discoverable to users who already have filesystem permissions to access it.

In the consuming flow, a consumer searches by query and metadata filters, inspects version and lineage context, resolves a product identifier, and retrieves a concrete version into a downstream workspace.

## 9. Data Architecture and Persistence
Feather Mesh primarily registers metadata and asset references while keeping source data in existing storage systems. V1 default consumption behavior is managed-copy, which stages data into approved locations for downstream processing. This default is required in environments where cross-lab directory permissions are limited to read/execute and direct dependency on remote paths is operationally fragile.

Reference-only retrieval can exist as a policy-controlled mode, but it is not the default. Metadata remains the source of truth in the central catalog, while physical asset truth remains in underlying storage systems.

Client-side responsibilities are handled by the `feam` CLI, and service-side responsibilities are handled by the metadata registry API, search/index services, and policy/audit components.

## 10. HPC Environment and Deployment Assumptions
For this PDD, “same HPC environment” means users and workflows operate under a shared trust and governance domain with compatible identity, scheduler, and storage assumptions.

V1 deployment scope is intentionally narrow: single cluster and single filesystem only. Multi-cluster and cross-filesystem retrieval are out of scope for V1.

Feather Mesh is scheduler-agnostic at interface level and can be invoked from Slurm, PBS, LSF, or scheduler-independent runners. Storage assumptions for V1 are POSIX-like semantics within the chosen filesystem implementation.

## 11. Security, Compliance, and Auditability
Security is based on least-privilege filesystem permissions, with Feather Mesh inheriting the effective access of the caller at retrieval time. Compliance posture requires policy metadata on publication and audit trails for publish, read, and metadata mutation actions. Retention and archival requirements are represented through metadata and lifecycle controls.

## 12. Reliability and Operations
Operationally, catalog operations should be idempotent where possible, and retrieval should support retry and checkpoint-aware execution to fit HPC job behavior. The platform should degrade gracefully so metadata discovery remains available even when individual storage endpoints are temporarily unavailable.

## 13. Success Metrics
Success is measured by adoption and outcome signals such as product publication volume, cross-team consumption rate, reduction in duplicated outputs, and time-to-discovery for reusable artifacts. Quality signals include completeness of keystone metadata, proportion of production pipelines using pinned versions, and retrieval failure rates by cause class.

## 14. Risks and Mitigations
Primary risks include metadata drift, policy complexity, user friction during serving, and trust gaps during discovery. Mitigation relies on strict required fields, schema validation, template-driven defaults, high auto-extraction coverage, clear lineage visibility, and consistent use of `data_quality` semantics.

## 15. CLI and User Experience
The CLI is designed as one command surface for both human interaction and automation. Interactive usage supports guided metadata entry and discovery refinement. Non-interactive usage supports deterministic execution with explicit parameters and machine-readable output.

Core command surface (target design):
- `feam serve <path> --name <name> --asset-type <type> [flags]`
- `feam search <query> [--filter key=value]`
- `feam show <product_id> [--version <v>]`
- `feam consume <product_id> --version <v> --out <path>`
- `feam lineage <product_id> [--version <v>]`
- `feam validate-metadata <metadata_file>`

Example usage:
```bash
# Serve a directory artifact
feam serve /data/team_a/yield_model/v3 \
  --name yield_model_training_output \
  --asset-type model_artifact \
  --owner-team team_a \
  --intended-use "Regional yield forecasting"

# Discover products
feam search "yield forecast" --filter data_quality=production

# Inspect lineage before use
feam lineage product://team_a/yield_model --version 3.1.0

# Consume a pinned version into pipeline workspace
feam consume product://team_a/yield_model --version 3.1.0 --out ./inputs/yield_model
```

Manual input should be minimal for repeated and pipeline-based operations through defaults and templates, while still requiring enough human context at first publication to preserve reuse quality.

## 16. Finalized V1 Decisions
`production` promotion is self-certified by producer role via the `data_quality` keystone attribute. The `data_quality` tiers are `production`, `qualified`, and `unverified`. Default consume mode is managed-copy. V1 search is metadata-driven keyword and filter search only. V1 deployment is single cluster and single filesystem. Keystone metadata aligns to a Feather Mesh DCAT profile with strong auto-extraction to increase adoption and baseline metadata quality.
