# Graph Report - feather-mesh  (2026-11-22)

## Corpus Check
- 97 files · ~79,387 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: .parquet 6, .tiff 2, (none) 1)

## Summary
- 1684 nodes · 4313 edges · 77 communities (65 shown, 12 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 219 edges (avg confidence: 0.81)
- Token cost: unavailable. Structural extraction uses no model tokens; semantic extraction ran in a Codex subagent whose tool does not expose token usage. Zero counters in cost.json are placeholders, not measured zero usage.

## Community Hubs (Navigation)
- Domain Validation and Registry
- STAC Catalog and HTTP
- Terminal Lifecycle and Events
- Agent Tools and Proposals
- Agent Session Workflow Tests
- Interactive Input and Navigation
- Python SDK and Errors
- Manifest Types and Inspectors
- Research Capture and Transport
- Prepared Peer Mutations
- Synthetic Model Screening
- Operation Journal and Recovery
- Hosted Assistant and Tutorial
- Persisted and Insert Models
- Bounded Agent Harness
- Provider Streaming Abstractions
- Descriptive Source Catalogue
- Architecture and Adapter Contracts
- PTY Measurement and Fixtures
- CLI Output and Legacy
- STAC Item Schema
- SDK and STAC Integration
- Peer Cache and Project
- Catalog Queries and Pagination
- Interactive Mutation Recovery Tests
- Capture and Review Tests
- Router Transport and Cancellation
- CLI Commands and Options
- Asset Validation and Paths
- Peer Errors and Locking
- Interactive Command Vocabulary
- CLI Routing and Exit
- Peer Publication Workflow Tests
- Legacy Service Integration Tests
- Agent Runtime and Broker
- Agent Profile Configuration
- Tutorial Action Events
- Tutorial Progress State
- Tutorial Expected Events
- Review Approval and Execution
- Held Out Evaluation Runner
- Product Version Repository
- UI Sections and Viewports
- Terminal Rendering and Sanitization
- Classification and Quality Types
- Provider Errors and SSE
- Team Models and Repository
- Tutorial Observation Matching
- Router Tool Schema Translation
- SQLite Setup and Migration
- Lineage Models and Repository
- Draft Controls and Exports
- SDK Protocol Compatibility Tests
- Data Product Repository
- Metadata Repository and Mapping
- Live Launch and Walkthrough
- Source Catalogue View Updates
- Terminal Restoration Tests
- Evaluation Corpus and Metrics
- Tutorial Curriculum and Bounds
- Asset Type Translation
- Bounded Assistant Transcript
- Outbound Disclosure and Redaction
- Evaluation Task Categories
- Repository Integration Tests
- Review Operation Categories
- Tutorial Run States
- Peer Test Fixture Helpers
- Tutorial Applicability Rules
- Tutorial Observation Outcomes
- Rust Workspace Packages
- Stage One Demo Script
- feam Entry Point
- PTY Terminal Emulator

## God Nodes (most connected - your core abstractions)
1. `PeerResult` - 82 edges
2. `App` - 77 edges
3. `ProviderError` - 38 edges
4. `resolve()` - 34 edges
5. `GuideAction` - 31 edges
6. `PeerError` - 30 edges
7. `Project` - 28 edges
8. `GuideCommand` - 28 edges
9. `ToolExecutor` - 27 edges
10. `RegistryError` - 27 edges

## Surprising Connections (you probably didn't know these)
- `Rasterio Local Asset Access` --semantically_similar_to--> `Loopback STAC HTTP`  [INFERRED] [semantically similar]
  python_sdk/README.md → README.md
- `execute()` --calls--> `execute_publication()`  [INFERRED]
  mesh_agent/examples/stage1_eval.rs → mesh_core/src/services/interactive_operations.rs
- `execute()` --calls--> `execute_stage()`  [INFERRED]
  mesh_agent/examples/stage1_eval.rs → mesh_core/src/services/interactive_operations.rs
- `execute()` --calls--> `execute_withdrawal()`  [INFERRED]
  mesh_agent/examples/stage1_eval.rs → mesh_core/src/services/interactive_operations.rs
- `run_task()` --calls--> `reconcile()`  [INFERRED]
  mesh_agent/examples/stage1_eval.rs → mesh_core/src/services/interactive_operations.rs

## Import Cycles
- 1-file cycle: `mesh_core/src/models/entities/data_product.rs -> mesh_core/src/models/entities/data_product.rs`
- 1-file cycle: `mesh_core/src/models/entities/data_product_version.rs -> mesh_core/src/models/entities/data_product_version.rs`
- 1-file cycle: `mesh_core/src/models/entities/lineage_dependency.rs -> mesh_core/src/models/entities/lineage_dependency.rs`
- 1-file cycle: `mesh_core/src/models/entities/metadata.rs -> mesh_core/src/models/entities/metadata.rs`
- 1-file cycle: `mesh_core/src/models/entities/team.rs -> mesh_core/src/models/entities/team.rs`
- 1-file cycle: `mesh_core/src/models/new/data_product.rs -> mesh_core/src/models/new/data_product.rs`
- 1-file cycle: `mesh_core/src/models/new/data_product_version.rs -> mesh_core/src/models/new/data_product_version.rs`
- 1-file cycle: `mesh_core/src/models/new/lineage_dependency.rs -> mesh_core/src/models/new/lineage_dependency.rs`
- 1-file cycle: `mesh_core/src/models/new/metadata.rs -> mesh_core/src/models/new/metadata.rs`
- 1-file cycle: `mesh_core/src/models/new/team.rs -> mesh_core/src/models/new/team.rs`
- 2-file cycle: `mesh_core/src/repositories/data_product_repository.rs -> mesh_core/src/repositories/mod.rs -> mesh_core/src/repositories/data_product_repository.rs`
- 2-file cycle: `mesh_core/src/repositories/data_product_version_repository.rs -> mesh_core/src/repositories/mod.rs -> mesh_core/src/repositories/data_product_version_repository.rs`
- 2-file cycle: `mesh_core/src/repositories/metadata_repository.rs -> mesh_core/src/repositories/mod.rs -> mesh_core/src/repositories/metadata_repository.rs`
- 2-file cycle: `mesh_core/src/repositories/mod.rs -> mesh_core/src/repositories/team_repository.rs -> mesh_core/src/repositories/mod.rs`

## Hyperedges (group relationships)
- **Adapters Share Workflow Rules** — readme_mesh_cli, readme_mesh_tui, python_sdk_readme_python_sdk, readme_stac_http, mesh_core_src_services_readme_shared_workflows [EXTRACTED 1.00]

## Communities (77 total, 12 thin omitted)

### Community 0 - "Domain Validation and Registry"
Cohesion: 0.07
Nodes (36): domain_enum!, RegistryResult, SourceReference, UnixPath, validate_non_blank(), validate_optional_text(), validate_positive_id(), validate_product_name() (+28 more)

### Community 1 - "STAC Catalog and HTTP"
Cohesion: 0.09
Nodes (44): collection_id(), collections(), CONFORMANCE_CLASSES, file_uri(), hex(), collection_route(), Cursor, cursor_is_opaque_and_round_trips() (+36 more)

### Community 2 - "Terminal Lifecycle and Events"
Cohesion: 0.05
Nodes (21): ASSISTANT_EVICTION_NOTICE, ASSISTANT_TRANSCRIPT_LIMIT, body_layout(), BodyLayout, GuidedNeedsResize, GuidedThreePane, GuidedTwoPane, Inactive (+13 more)

### Community 3 - "Agent Tools and Proposals"
Cohesion: 0.08
Nodes (23): AGENT_TOOLS_PROTOCOL_VERSION, ConfirmableOperation, Publication, Stage, Withdrawal, HandleArgs, HelpArgs, parse() (+15 more)

### Community 4 - "Agent Session Workflow Tests"
Cohesion: 0.07
Nodes (26): a_mutation_proposal_stops_the_rest_of_a_complete_batch_for_local_review(), call(), clarification_retains_context_and_pinned_resolution_evidence(), complete_batches_are_serialized_and_every_call_is_correlated(), disclosure_channels_and_unknown_cost_stop_before_next_request(), fixture(), full_context_and_output_are_bounded(), late_cancel_and_project_switch_expire_handles() (+18 more)

### Community 5 - "Interactive Input and Navigation"
Cohesion: 0.12
Nodes (11): App, Review, Status, InputMode, Agent, Command, None, Search (+3 more)

### Community 6 - "Python SDK and Errors"
Cohesion: 0.10
Nodes (15): error_for(), ExecutableNotFoundError, FeamError, IntegrityError, NotFoundError, PeerUnavailableError, PolicyError, ProtocolError (+7 more)

### Community 7 - "Manifest Types and Inspectors"
Cohesion: 0.07
Nodes (30): AssetRecord, DataFormat, Geotiff, DeclaredAsset, epsg_from_geo_keys(), hex_digest(), inspect_geotiff(), Lifecycle (+22 more)

### Community 8 - "Research Capture and Transport"
Cohesion: 0.11
Nodes (15): Capture, Config, CONNECTING, digest_id(), disabled_capture_preserves_manual_workflows(), FAILED, identifiers_are_opaque_and_valid(), OFF (+7 more)

### Community 9 - "Prepared Peer Mutations"
Cohesion: 0.16
Nodes (28): execute_publication(), execute_stage(), execute_with_project(), execute_withdrawal(), fingerprint(), inventory_fingerprint(), NEXT_OPERATION, next_operation_id() (+20 more)

### Community 10 - "Synthetic Model Screening"
Cohesion: 0.10
Nodes (10): Budget, check_schema(), evaluate(), main(), NoRedirect, request(), score_step(), summarize() (+2 more)

### Community 11 - "Operation Journal and Recovery"
Cohesion: 0.14
Nodes (22): OperationState, Committed, CommittedWithFollowupError, FailedBeforeCommit, OutcomeUnknown, completed_retention_does_not_discard_unresolved_intents(), create_private_directory(), ensure_private_permissions() (+14 more)

### Community 12 - "Hosted Assistant and Tutorial"
Cohesion: 0.09
Nodes (16): AgentPending, AgentWorkerMessage, Complete, Event, AssistantRequestKind, GuidedLesson, GuidedObservation, GuidedQuestion (+8 more)

### Community 13 - "Persisted and Insert Models"
Cohesion: 0.10
Nodes (10): new_data_product_initializes_insert_model_with_optional_fields_to_none(), new_data_product_preserves_optional_fields(), NewDataProduct, new_data_product_version_initializes_insert_model(), NewDataProductVersion, new_metadata_initializes_insert_model_with_optional_fields_to_none(), new_metadata_preserves_optional_fields(), NewMetadata (+2 more)

### Community 14 - "Bounded Agent Harness"
Cohesion: 0.13
Nodes (16): AgentEvent, RequestStarted, Text, ToolResult, ToolStarted, Usage, AgentHarness<P>, AgentRun (+8 more)

### Community 15 - "Provider Streaming Abstractions"
Cohesion: 0.14
Nodes (17): Backend, Captured<P>, CancellationToken, FakeProvider, ModelProvider, ModelRequest, process_router_event(), Protocol (+9 more)

### Community 16 - "Descriptive Source Catalogue"
Cohesion: 0.10
Nodes (26): CatalogueManifest, Category, checked_regular_file(), load(), MAX_CATEGORIES_PER_SUITE, MAX_FILE_BYTES, MAX_PRODUCTS_PER_CATEGORY, MAX_SUITES (+18 more)

### Community 17 - "Architecture and Adapter Contracts"
Cohesion: 0.09
Nodes (27): Historical Workplan Notice, NewX Insert Models, Persisted Entity Models, SQL Repository Layer, Project Catalog Service, Prepared Interactive Mutations, Private Operation Journal, Peer Access Service (+19 more)

### Community 19 - "CLI Output and Legacy"
Cohesion: 0.11
Nodes (25): Cli, map_io_error(), OutputFormat, Json, Table, print_consume(), print_init(), print_json() (+17 more)

### Community 20 - "STAC Item Schema"
Cohesion: 0.07
Nodes (30): minProperties, type, maxItems, minItems, type, format, type, format (+22 more)

### Community 21 - "SDK and STAC Integration"
Cohesion: 0.16
Nodes (10): main(), write_table(), cli(), metadata_raster(), metadata_table(), peer_projects(), publish(), test_climatology_interval_cli_sdk_schema_http_and_pixels() (+2 more)

### Community 22 - "Peer Cache and Project"
Cohesion: 0.16
Nodes (22): cache_status(), CachePeerStatus, CacheSnapshot, local_manifest_revision(), parse_manifest(), Conflict, NotFound, PeerUnavailable (+14 more)

### Community 23 - "Catalog Queries and Pagination"
Cohesion: 0.13
Nodes (22): CATALOG_PROTOCOL_VERSION, catalog_teams(), CatalogCursor, CatalogEntry, CatalogPage, CatalogQuery, cursor_round_trips_without_exposing_json_to_callers(), decode_cursor() (+14 more)

### Community 24 - "Interactive Mutation Recovery Tests"
Cohesion: 0.10
Nodes (19): capture_failure_preserves_unapproved_review_and_denial(), Recovered, default_journal_root(), fallback_journal_root(), fixture_app(), full_review_renders_controls_at_eighty_by_twenty_four(), manual_publication_stage_denial_withdrawal_and_recovery(), missing_project_requires_explicit_initialization() (+11 more)

### Community 25 - "Capture and Review Tests"
Cohesion: 0.14
Nodes (5): main(), CaptureTests, Session, launch(), ScreenSession

### Community 26 - "Router Transport and Cancellation"
Cohesion: 0.14
Nodes (14): completion(), deadline_and_cancellation_interrupt_a_silent_socket(), FakeScript, missing_usage_stays_unknown_and_routing_is_captured(), once(), PartialToolCall, profile(), request() (+6 more)

### Community 27 - "CLI Commands and Options"
Cohesion: 0.08
Nodes (24): AgentMode, Fake, Hosted, Off, CacheCommand, Status, Command, Cache (+16 more)

### Community 28 - "Asset Validation and Paths"
Cohesion: 0.23
Nodes (22): absolute_path(), checked_asset_path(), Parquet, inspect_parquet(), merge_raster_descriptors(), merge_table_descriptors(), non_blank(), parse_reference() (+14 more)

### Community 29 - "Peer Errors and Locking"
Cohesion: 0.12
Nodes (20): ManifestLock, PeerError, DatasetNotRegistered, Integrity, Io, Json, LockTimeout, Parquet (+12 more)

### Community 30 - "Interactive Command Vocabulary"
Cohesion: 0.08
Nodes (23): GuideCommand, AgentDraft, AgentProfile, DraftLoad, DraftNewRaster, DraftNewTable, DraftSave, DraftSet (+15 more)

### Community 31 - "CLI Routing and Exit"
Cohesion: 0.15
Nodes (12): AppError, Peer, Registry, Tui, exit_code(), main(), parse_lineage(), print_peer() (+4 more)

### Community 32 - "Peer Publication Workflow Tests"
Cohesion: 0.19
Nodes (13): PublicationRequest, publish(), publish_with_precondition(), catalog_cursors_bind_filters_configuration_and_revisions(), changed_route_invalidates_prepared_access_even_at_same_revision(), configure_client(), oversized_manifest_reports_failed_coverage_and_stage_preflight_preserves_aliases(), publishes_fixed_inventory_resolves_through_client_route_and_stages_safely() (+5 more)

### Community 33 - "Legacy Service Integration Tests"
Cohesion: 0.13
Nodes (7): DB_COUNTER, unique_test_db_path(), registry_service_compatibility_methods_register_product_version_and_lineage(), registry_service_compatibility_methods_reject_missing_owner_and_legacy_quality(), registry_service_registers_and_lists_teams(), registry_service_serve_rolls_back_product_when_version_insert_fails(), test_connection()

### Community 34 - "Agent Runtime and Broker"
Cohesion: 0.10
Nodes (11): AgentConfig, AgentRunState, AwaitingClarification, AwaitingReview, Complete, Failed, Stopped, RouterProvider (+3 more)

### Community 35 - "Agent Profile Configuration"
Cohesion: 0.13
Nodes (10): AGENT_CONFIG_SCHEMA_VERSION, extra_ca_is_scoped_to_exact_https_loopback(), invalid(), ProfileError, Invalid, Io, Missing, Toml (+2 more)

### Community 36 - "Tutorial Action Events"
Cohesion: 0.10
Nodes (19): GuideAction, Command, ExamplesGenerated, HelpOpened, OperationCommitted, PageAttempt, ProductOpened, RecoveryCompleted (+11 more)

### Community 37 - "Tutorial Progress State"
Cohesion: 0.20
Nodes (3): CurriculumStep, GuideState, skipping_marks_the_lesson_and_discards_old_observations()

### Community 38 - "Tutorial Expected Events"
Cohesion: 0.11
Nodes (19): ExpectedEvent, AnySection, Committed, Examples, Help, Page, Preview, ProductOpen (+11 more)

### Community 39 - "Review Approval and Execution"
Cohesion: 0.20
Nodes (8): Operation, lost_journal_result_preserves_structured_committed_followup_error(), Review, Publication, Save, Stage, Withdrawal, section_results_and_scroll_positions_remain_isolated()

### Community 40 - "Held Out Evaluation Runner"
Cohesion: 0.19
Nodes (10): Fake, Live, Captured, default_profile(), execute(), fake(), main(), op_id() (+2 more)

### Community 42 - "UI Sections and Viewports"
Cohesion: 0.13
Nodes (14): key_hints(), Section, .ALL, Assistant, Cache, Catalog, Help, Lineage (+6 more)

### Community 43 - "Terminal Rendering and Sanitization"
Cohesion: 0.43
Nodes (14): render(), render_assistant(), render_body(), render_catalog(), render_catalog_panes(), render_current_view(), render_guide(), render_lineage() (+6 more)

### Community 44 - "Classification and Quality Types"
Cohesion: 0.14
Nodes (10): Classification, CliClassification, Internal, Public, Restricted, CliDataQuality, Production, Qualified (+2 more)

### Community 45 - "Provider Errors and SSE"
Cohesion: 0.23
Nodes (8): ProviderError, Authentication, Cancelled, Configuration, Malformed, RateLimited, TooLarge, SseParser

### Community 47 - "Tutorial Observation Matching"
Cohesion: 0.19
Nodes (6): Command, GuideObservation, matching_success_advances_but_failure_and_unrelated_do_not_disclose_arguments(), ObservationResult, Failure, Success

### Community 48 - "Router Tool Schema Translation"
Cohesion: 0.21
Nodes (9): AgentHarness, message(), ModelMessage, ProviderToolCall, router_rejects_ambiguous_names_and_unadvertised_calls(), router_translates_tool_schemas_and_correlated_history(), RouterToolNames, ToolSchema (+1 more)

### Community 49 - "SQLite Setup and Migration"
Cohesion: 0.29
Nodes (10): add_column_if_missing(), DB_COUNTER, DEFAULT_DB_FILENAME, init_db(), init_db_backfills_legacy_classification_and_adds_unique_indexes(), init_db_creates_registry_db_and_enables_wal(), init_db_is_idempotent(), init_default_db() (+2 more)

### Community 50 - "Lineage Models and Repository"
Cohesion: 0.27
Nodes (5): LineageDependency, new_lineage_dependency_initializes_insert_model_with_optional_fields_to_none(), new_lineage_dependency_preserves_optional_fields(), NewLineageDependency, LineageDependencyRepository

### Community 51 - "Draft Controls and Exports"
Cohesion: 0.19
Nodes (10): changed_export_is_never_overwritten_by_old_review(), draft_template(), example(), HELP, patch_draft(), paths_and_json_are_lexed_without_execution(), python_quote(), save_local_text() (+2 more)

### Community 52 - "SDK Protocol Compatibility Tests"
Cohesion: 0.27
Nodes (5): executable(), test_missing_executable_malformed_json_and_typed_error_are_distinct(), test_resolve_uses_an_argument_array_and_parses_the_pinned_descriptor(), main(), execute_example()

### Community 54 - "Metadata Repository and Mapping"
Cohesion: 0.31
Nodes (3): Metadata, MetadataRepository, parse_naive_datetime()

### Community 55 - "Live Launch and Walkthrough"
Cohesion: 0.21
Nodes (3): main(), event(), replay_turns()

### Community 56 - "Source Catalogue View Updates"
Cohesion: 0.17
Nodes (9): CoreUpdate, Catalog, Draft, Preview, Refreshed, SourceCatalogue, View, SourceCatalogue (+1 more)

### Community 58 - "Evaluation Corpus and Metrics"
Cohesion: 0.25
Nodes (8): CORPUS_VERSION, EVALUATION_SCHEMA_VERSION, EvaluationReport, EvaluationResult, frozen_corpus_has_required_distribution(), HELD_OUT_TASK_COUNT, held_out_tasks(), summarize()

### Community 59 - "Tutorial Curriculum and Bounds"
Cohesion: 0.20
Nodes (8): bound(), CURRICULUM, CURRICULUM_ID, LESSON_COUNT, step!, OBSERVATION_LIMIT, sanitize_error(), SUMMARY_LIMIT

### Community 60 - "Asset Type Translation"
Cohesion: 0.22
Nodes (9): AssetType, CliAssetType, Dataset, Directory, File, ManifestCollection, ModelArtifact, ReportArtifact (+1 more)

### Community 61 - "Bounded Assistant Transcript"
Cohesion: 0.36
Nodes (3): assistant_is_a_persistent_bounded_menu(), AssistantTranscriptEntry, AssistantViewState

### Community 62 - "Outbound Disclosure and Redaction"
Cohesion: 0.44
Nodes (5): AgentProfile, outbound(), redact(), scrub_value(), tool_summary()

### Community 63 - "Evaluation Task Categories"
Cohesion: 0.25
Nodes (8): TaskCategory, AdversarialDeniedAction, AmbiguityErrorRecovery, DiscoveryInspection, Publication, ResolveExamples, Staging, Withdrawal

### Community 64 - "Repository Integration Tests"
Cohesion: 0.50
Nodes (6): create_product_with_version(), discovery_queries_return_case_insensitive_matches_with_current_metadata_values(), repositories_create_get_by_id_and_get_all_full_registry_graph(), repositories_preserve_none_for_nullable_insert_fields(), team_repository_creates_gets_by_id_and_gets_all_persisted_teams(), test_connection()

### Community 65 - "Review Operation Categories"
Cohesion: 0.29
Nodes (6): ReviewKind, Export, Inspection, Publication, Staging, Withdrawal

### Community 66 - "Tutorial Run States"
Cohesion: 0.33
Nodes (5): GuideRunState, Complete, Explaining, Result, Waiting

### Community 68 - "Tutorial Applicability Rules"
Cohesion: 0.40
Nodes (5): Applicability, Always, CatalogData, DisposableDemoOnly, TableSelection

### Community 69 - "Tutorial Observation Outcomes"
Cohesion: 0.40
Nodes (5): ObservationDisposition, Advanced, Completed, Retry, Unrelated

### Community 70 - "Rust Workspace Packages"
Cohesion: 0.83
Nodes (4): mesh_agent, mesh_cli, mesh_core, mesh_tui

## Knowledge Gaps
- **205 isolated node(s):** `Io`, `Toml`, `Invalid`, `DiscoveryInspection`, `ResolveExamples` (+200 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 484 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `resolve()` connect `Peer Cache and Project` to `Peer Publication Workflow Tests`, `STAC Catalog and HTTP`, `Terminal Lifecycle and Events`, `Agent Tools and Proposals`, `Interactive Input and Navigation`, `Manifest Types and Inspectors`, `Prepared Peer Mutations`, `Interactive Mutation Recovery Tests`, `Asset Validation and Paths`, `Peer Errors and Locking`, `CLI Routing and Exit`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `ProviderError` (e.g. with `.stream()` and `.stream()`) actually correct?**
  _`ProviderError` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Io`, `Toml`, `Invalid` to the rest of the system?**
  _205 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Domain Validation and Registry` be split into smaller, more focused modules?**
  _Cohesion score 0.06971975393028025 - nodes in this community are weakly interconnected._
- **Why does `App` connect `Interactive Input and Navigation` to `Terminal Lifecycle and Events`, `Tutorial Progress State`, `Review Approval and Execution`, `Research Capture and Transport`, `UI Sections and Viewports`, `Operation Journal and Recovery`, `Hosted Assistant and Tutorial`, `Terminal Rendering and Sanitization`, `Catalog Queries and Pagination`, `Source Catalogue View Updates`, `Interactive Mutation Recovery Tests`, `Bounded Assistant Transcript`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Are the 10 inferred relationships involving `resolve()` (e.g. with `.product_inspect()` and `run_peer()`) actually correct?**
  _`resolve()` has 10 INFERRED edges - model-reasoned connections that need verification._
- **Should `STAC Catalog and HTTP` be split into smaller, more focused modules?**
  _Cohesion score 0.08531073446327683 - nodes in this community are weakly interconnected._
## Build Integrity and Coverage

- Scope: this feather-mesh/ directory only; sibling web_demo/, repository-level docs, and other parent content were not scanned.
- All 9 semantic input files were read completely. The structural extractor warned that 12 code/config files yielded no symbols. Ten unsupported/unclassified files also remain outside the graph.
- Graph health warning: 542 dangling-endpoint edge records, 1 self-loop, and 69 same-endpoint records collapsed by the default undirected graph. The directed diagnostic also counts 69 collapses; these are the same records, not an additional 69. See graph_health.json for details. These are extraction limitations, not confirmed application defects.
- Extraction produced 1,525 explicit nodes; graph building produced 1,684 nodes after endpoint handling. Inferred connections need source verification before relying on them for changes.
- The AST multiprocessing attempt failed on macOS when invoked from standard input; graphify completed through its sequential fallback.
- Retrieval benchmark: estimated 1,621 tokens/query versus a 112,266-token naive corpus, a 69.3x reduction. These are synthetic estimates, not measured semantic-extraction usage; the benchmark's independently counted corpus (84,200 words) differs from the initial detector (79,387 words).
- graphify-out/ was written into the working tree and is currently untracked. You can add graphify-out/ to .gitignore, or commit only outputs your team should share; see https://docs.graphify.com/guides/team-workflows.
