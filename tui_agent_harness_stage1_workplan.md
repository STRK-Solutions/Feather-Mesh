# Feather Mesh TUI and Agent Harness: Stage 1 Workplan

Status: **Stage 1 complete. Local implementation, required verification, clean-source handoff and live acceptance pass. The final frozen candidate scores 97/100 on fresh held-out requests, with zero detected unauthorized writes/disclosures; its full live TUI walkthrough passes. All prior attempts remain recorded. Target-HPC acceptance is separate.**

Date: **2026-09-23**.

Progress updated from implementation and executed checks on **2026-09-23**. See the [acceptance record](docs/tui_agent_stage1_acceptance.md) for commands, all live attempts, frozen artifacts, measured limits and separate follow-ups. Checked items cover the stated deliverables, not an implied whole-phase pass.

Design: [TUI and agent harness design, v0.4](tui_agent_harness_design.md).

Inspected baseline: `d8633e6` (`Fix peer resolver Clippy lint`), following the peer-data-access merge at `558e187`. Recheck HEAD and the working tree before implementing; the baseline is a reference, not an instruction to reset the repository.

## 1. Objective and instructions for the implementing agent

Deliver stage 1 completely: a usable manual `feam` TUI plus a hosted-model agent harness connected through an API-key-authenticated model router, with a reproducible development/demo environment and acceptance evidence. This covers design milestones M0–M3. Creating this workplan does not implement or validate those milestones.

Read these sources before editing:

1. [AGENTS.md](AGENTS.md) and the [design](tui_agent_harness_design.md).
2. The [peer-data-access requirements](data_access.md), settled [contract](docs/data_access_contract.md), and prior [execution record](data_access_implementation_workplan.md#8-execution-record-for-the-implementing-agent).
3. The [Rust workflow](.codex/skills/feam-rust-workflow/SKILL.md), [CLI contract](.codex/skills/feam-cli-contract/SKILL.md), [peer-access](.codex/skills/feam-peer-data-access/SKILL.md), and [agent-context maintenance](.codex/skills/feam-agent-context-maintainer/SKILL.md) skills.

Explicit user instructions take precedence. The settled peer contract governs publication and access; source/tests establish implemented behavior. Some older planning text and skill introductions predate the peer implementation: do not interpret them as instructions to recreate existing SDK/STAC components or restore SQLite as peer authority.

Work through the phases below, updating the execution record with changed paths, exact commands, results, and remaining gaps. Complete routine reversible implementation and tests without stopping for phase-by-phase permission. Resolve routine choices using the defaults here and record substantive deviations in the stage-1 contract and design. Ask only for missing inputs that are actually needed, such as the live router model/key or spending limit, while continuing independent work.

Preserve unrelated user changes. Include this workplan and the design in the eventual handoff/change set; at authoring time the design was untracked. Never commit credentials, private conversation traces, generated user catalogs, or machine-specific paths. Do not mark a phase complete solely because code exists, screenshots look correct, or a fake provider passes.

## 2. Scope and completion boundary

### Included in stage 1

- A `feam tui --project ROOT` entry point, explicit initialization flow, and keyboard-driven catalog, product/version details, lineage, teams, peer health, refresh/cache status, and help.
- Manual publication draft editing/validation/publication, pinned resolution, explicit staging, and withdrawal, all using shared Rust services.
- Reviewable CLI and supported Python SDK examples; raster/STAC metadata and connection guidance.
- A provider-independent harness, one hosted router adapter, a deterministic fake provider, streaming assistant interaction, typed tools, bounded context, and resource/cost controls.
- Concrete mutation review, current-state checks, operation results, private recovery records, and honest cancellation behavior.
- Reproducible fixtures, a demo setup/reset workflow, deterministic tests, real API acceptance, and a reusable held-out evaluation suite.
- Build-feature isolation, CLI/SDK compatibility, documentation, and CI/context updates as components arrive.

### Follow-ups that must not block stage 1

| Follow-up | Preserve now | Defer |
| --- | --- | --- |
| Shared project SQLite catalog | Shared core catalog interface; qualified identity, bounded results, freshness, per-peer coverage | Database schema, snapshot/service decision, shared catalog publication and migration |
| Stage-2 small model | Provider interface, compact tool schemas, provider-neutral evaluation tasks | Local inference runtime, quantization, model selection, post-training |
| SLURM deployment | Bounded work and measured TUI/harness footprint; backend-independent session state | Allocation controller, CPU/GPU requests, queue optimization and target-HPC acceptance |
| Legacy registry | Existing CLI behavior and regression coverage | TUI registry mode, legacy data migration/removal, unrelated registry-default cleanup |
| General execution | Deterministic feam tools and generated examples | Arbitrary shell/Python/SQL execution, scheduler jobs, ACL or peer-link administration |
| STAC process management | Reuse existing descriptors and documented commands | Starting/stopping a long-lived STAC service from the TUI |

Stage 1 is complete only when manual workflows, fake-provider automation, live hosted tool use, and the acceptance report all pass. Missing live credentials can leave **local implementation complete; live acceptance pending**, but cannot justify a claim of full stage-1 completion. Actual cluster evidence belongs to the separate HPC checklist and stage 2.

## 3. Repository baseline and implementation boundaries

| Existing surface | Reuse / required adaptation |
| --- | --- |
| [CLI dispatch](feather-mesh/mesh_cli/src/main.rs) | Add explicit TUI dispatch before legacy/project command execution; preserve existing script output and binary naming |
| [Peer services](feather-mesh/mesh_core/src/services/peer_access.rs) | Reuse `Project`, `PublicationRequest`, validation, publication, resolution, staging, withdrawal, and typed errors |
| `list_registered`, `refresh`, `cache_status` | Add a catalog-facing service with coverage and bounded result DTOs; the current JSON snapshot stores peer status, not product contents |
| `Project::open` | Reopen at execution boundaries so a long-lived UI does not retain obsolete project configuration |
| `stage(ResolvedProduct, ...)` | Add an interactive workflow that resolves immediately before copying; never stage a descriptor retained across user review |
| Publication/withdrawal locks | Add relevant execution preconditions under the existing mutation lock; do not introduce an independent publication authority |
| [CLI workflow tests](feather-mesh/mesh_cli/tests/cli_workflow_tests.rs) and [peer tests](feather-mesh/mesh_core/tests/peer_access_tests.rs) | Extend for changed public behavior and relevant service invariants |
| [Python SDK](feather-mesh/python_sdk/README.md) and [peer fixtures](feather-mesh/mesh_core/tests/data/peer_access/README.md) | Reuse installed-package integration and known table/raster values |
| [Rust CI](.github/workflows/rust.yml) and [context CI](.github/workflows/agent-context.yml) | Add tests/features in the phase introducing them; ordinary CI never needs a paid API key |

Issues identified at the inspected baseline, with current disposition:

- Peer CLI search currently matches namespace, product ID, and name; other parsed filters are ignored in that branch. Move any newly advertised filtering into core and test CLI/TUI/tool agreement. Do not advertise unsupported filters.
- Legacy `list_registered` skips unavailable routes. The new catalog uses `list_registered_with_coverage`; retain per-peer coverage across interactive/tool views so an empty catalog is distinguishable from a failed search of a peer.
- The baseline CLI withdrawal branch discarded the namespace. The shared `withdraw_qualified` workflow and CLI namespace-preservation fix now exist with regression coverage; retain them while completing interactive review/recovery tests.
- Core calls are synchronous and have no cooperative progress/cancellation interface. Background execution is necessary for UI responsiveness, but does not cancel a running copy or undo a commit.
- `feam.peer.v1` has existing success/error shapes used by the SDK. Do not standardize or replace those envelopes as a side effect of creating agent tools.

The `mesh_tui` and `mesh_agent` library crates now exist inside `feather-mesh/`; they were absent at the inspected baseline. `mesh_cli` owns parsing/process behavior; `mesh_tui` owns terminal rendering and interaction; `mesh_agent` owns orchestration/provider translation; `mesh_core` owns shared DTOs, domain rules, and workflows. Repositories own any SQL; no SQLite addition is required for this stage.

## 4. Defaults to settle in S0

Use these as implementation defaults unless new evidence requires a recorded adjustment. They are sufficient to start work without asking the user to choose every library or file name.

| Decision | Stage-1 default |
| --- | --- |
| Terminal stack | Ratatui + compatible Crossterm; bounded event channels and blocking workers; select and pin compatible versions |
| Network stack | Async Rust HTTP transport with TLS, request deadlines, streaming support, and bounded payloads; keep dependencies out of core |
| Build modes | Existing CLI-only default; optional `tui` for manual interaction; optional `agent-hosted` includes TUI and router support; fake provider available to tests/demo without a paid account |
| Entry point | `feam tui --project ROOT`; proposed `--agent off` and `--agent-profile NAME`; missing/invalid credentials leave manual mode usable |
| Router | OpenRouter as the first concrete adapter, behind a provider-neutral interface; select the live model explicitly in a user profile |
| Credentials | Environment-variable reference in user config; no key in CLI arguments, project config, logs, fixtures, or model context |
| External context | Synthetic demo fixtures first; explicit session disclosure policy and field allowlist enforced at request serialization |
| Agent limits | One active run and one tool execution at a time; at most eight tool calls, one malformed-call repair, two minutes of model-request time; separately bound output/context and spending |
| Confirmation | Scoped reads proceed within the request; exact reviewed publication/staging/withdrawal require user action; approval is never a tool argument |
| Catalog | Manifest-backed implementation behind a shared interface; no shared SQLite feature dependency |
| Persistence | Minimal private operation journal for recovery, transcript saving opt-in, retention bounded; neither is a shared project catalog |
| Initial platform evidence | Linux CI and the available development terminal; record SSH/tmux smoke-test evidence when available |

Resolve flag conflicts, `--format json` handling for `tui`, non-terminal behavior, disabled-feature help, model-profile selection, and normal/error exit semantics explicitly. Do not let a TUI invocation fall into `run_legacy` or initialize `registry.db`. Preserve the public name `feam` and existing Cargo artifact `mesh_cli`.

## 5. Delivery phases

The default order is S0 → S1 → S2 → S3 → S4 → S5 → S6 → S7 → S8. A single implementing agent can complete this sequence. Each phase includes its tests and documentation; final handoff is not the first time CI learns about a component.

Each phase retains its full delivery requirements below its progress checklist. Implementation and local evidence are now recorded; all applicable Stage-1 gates now pass; retain the recorded limits and historical failures. The [acceptance record](docs/tui_agent_stage1_acceptance.md) contains current and historical results.

### S0 — Establish contracts, fixtures, and baseline evidence

**Deliver:** a concise stage-1 implementation contract, recorded baseline checks, fixture/task definitions, and settled initial build/provider choices.

**Progress — complete locally:**

- [x] Inspected the baseline and recorded focused core/CLI baseline tests; created the [Stage-1 contract](docs/tui_agent_stage1_contract.md).
- [x] Defined catalog DTOs, provider messages/events, tool protocol, profiles, operation outcomes, and ownership; selected dependencies and wired the CLI-only/manual/hosted feature matrix.
- [x] Defined disposable fixtures and the seven-category held-out task distribution. The distribution is established; distinct executable evaluation tasks and the runner remain S7 work.

1. Inspect HEAD, the working tree, current CLI/core boundaries, CI, and installed tools. Run the existing Rust baseline checks. Record pre-existing failures with evidence instead of attributing them to this feature.
2. Create `docs/tui_agent_stage1_contract.md` covering entry-point/feature behavior, ownership, catalog and operation DTOs, provider interface, `feam.agent.tools.v1`, profile schema, confirmation states, errors, and recovery outcomes. This new contract must refer to the existing peer contract instead of duplicating publication rules.
3. Define serializable agent messages/events and `answer`, `clarification`, and `tool proposal` outcomes. Preserve provider call IDs for protocol correlation, but use harness-owned operation IDs for execution.
4. Specify `CatalogQuery`/`CatalogPage` and coverage/freshness types that can later support SQLite. Define deterministic sorting, page/result limits, and restart behavior if relevant manifests/configuration change between pages.
5. Select dependency versions and feature wiring; document a three-mode build matrix. Record the router/model configuration required for live tests without selecting a paid model silently.
6. Define disposable provider/client fixtures and known expected outcomes for the demo and evaluations. Reuse existing small Parquet/GeoTIFF assets and include an unregistered asset, an unavailable peer, and two versions for ambiguity tests.
7. Outline the held-out task distribution before tuning prompts. Keep development examples separate from the release evaluation set.

**Acceptance:** existing behavior is understood and baseline results recorded; all defaults necessary for S1/S2 are concrete; no new command/crate is described as implemented before it exists. Missing API credentials do not block this phase.

### S1 — Shared catalog and interactive operation services

**Deliver:** UI-independent service interfaces reusable by manual and assistant paths.

**Progress — implemented and locally verified:**

- [x] Shared catalog filters, teams/lineage and CLI parity; cursors bind query, configuration, revisions and availability. Input is bounded to 8 MiB per manifest / 32 MiB per catalog read, with explicit oversized-input behavior.
- [x] Publication reviews bind exact validated metadata/inventory/source state; staging binds asset bytes, destination and receipt state. Mutation rechecks reject material changes.
- [x] Private recovery identities and read-only manifest/receipt reconciliation distinguish committed, failed, follow-up-error and unknown results without retry.
- [x] Core/CLI regressions cover changed drafts, sources, routes, filters, cursors, destination/receipt state, collision/hardlink safety and preservation; installed SDK/STAC tests cover affected adapters. See acceptance limits for crash/disk-failure injection not exercised.

1. Add catalog query/result services under `mesh_core::services`, using current manifests and existing validators. Return qualified reference, explicit versions, manifest revision, metadata summary, observation time, and per-peer coverage/errors. Handle absent provider configuration on consumer-only projects correctly.
2. Provide bounded query results with stable ordering. Bound manifest input and result sizes or report explicit limits; a paginated screen must not conceal unbounded loading. Measure the initial implementation and record limitations rather than implementing the deferred SQLite feature.
3. Centralize supported search filters and product/lineage/team views. Preserve externally consumed JSON shapes; intentional corrections to ignored filters need CLI compatibility tests and documentation.
4. Introduce prepared operation/preflight/result types for publication, staging, and withdrawal. Reuse existing format/path/collision validators. Include the exact draft/inventory, version, relevant revision/configuration fingerprint, destination/receipt state, and expensive-I/O estimate needed by a review screen.
5. Execute against a newly opened project. Recheck current routes and lifecycle; resolve pinned assets immediately before staging. Check publication/withdrawal preconditions under their mutation lock and reject cross-namespace withdrawal. Define what can be rechecked and the remaining filesystem race limits without promising immutable bytes.
6. Preserve existing APIs where possible; route CLI entry points through shared corrected workflows where necessary for parity. Keep confirmation authorization in the application controller, while core validates concrete execution preconditions.
7. Return honest outcome categories: committed, failed before commit, committed with follow-up error, or unknown pending reconciliation. Do not infer that every I/O error means nothing changed. Provide read-only reconciliation helpers where the journal will need them.

**Tests:** namespace/alias mismatch; partial peer coverage; page stability; unsupported filters; changed config/route/revision after preflight; unregistered files; withdrawal; source/destination/receipt collisions; namespace-safe withdrawal; duplicate/uncertain publication; unchanged provider bytes and prior outputs on tested failures. Exercise relevant CLI and adapter boundaries, not only the new wrapper.

**Acceptance:** services work without a terminal, provider, or SQLite index; all manual and tool workflows can call the same rules; focused core/CLI and affected adapter checks pass.

### S2 — TUI shell and manual discovery

**Deliver:** a useful model-free terminal application and reproducible feature builds.

**Progress — implemented and locally verified:**

- [x] Optional project-only TUI, explicit initialization, bounded core worker and normal-tick completion polling; manual navigation remains responsive during core/provider work.
- [x] Paging, scrolling, Catalog/Peers/Operations/Help/Teams/Cache/Lineage, state-clearing project changes, visible selection, NO_COLOR and 80×24 layouts.
- [x] Quoted local controls and reviewed text export, table/raster SDK examples and STAC connection guidance.
- [x] Real PTY keyboard/resize/INT/TERM/HUP/error/panic/restoration tests, missing configuration and unrelated-directory paths-with-spaces workflows; CLI JSON remains separate.

1. Add `mesh_tui`, workspace membership, optional CLI dependency/feature, and explicit `tui` command dispatch. Keep hosted HTTP libraries out of the manual-only dependency graph.
2. Implement terminal setup/teardown, event handling, focus, help, resize, input widgets, and bounded background workers. Test normal exit, supported signals, and error/panic paths; keep output away from existing machine-mode stdout/stderr.
3. Implement project selection/validation and explicit initialization for a missing config. Initialization must be deliberate and use existing core rules. Treat switching projects as a new session context, clearing pending operations and later model state.
4. Add catalog search/list, product/version selection, metadata/limitations/policy, asset inventory, lineage, teams, peer health, refresh/cache status, and operation/error panels.
5. Add pinned resolution and deterministic CLI/SDK example rendering. Include a reliable text export/copy fallback that works over SSH without depending on terminal clipboard features. Do not execute generated examples.
6. Add bounded layouts for 80×24 and larger terminals, keyboard-only navigation, visible focus, no-color mode, and escaped control sequences in external text. Unsupported terminal environments get guidance to use the CLI.
7. Document real build/run/test commands and update CI/context ownership as soon as the crate and feature exist.

**Tests:** rendering/state transitions plus a real pseudo-terminal launch/exit/resize flow. Snapshot tests can check layout but do not establish input routing or terminal restoration. Test paths with spaces, unrelated working directories, missing projects, missing TTY, unavailable peers, and CLI JSON regression behavior.

**Acceptance:** a user can initialize deliberately, discover/inspect a published version, understand partial coverage, and obtain correct direct-read examples with no key or model. Default and manual-only build modes pass.

### S3 — Complete manual producer/consumer workflows and recovery

**Deliver:** publication, staging, withdrawal, and operation review usable without the assistant.

**Progress — implemented and locally verified:**

- [x] Editable table/raster JSON drafts, explicit asset inventory, scoped save/export replacement, validation feedback and quoted destinations.
- [x] Separate inspection-I/O consent and complete validated publication/staging/withdrawal reviews; current-state rechecks precede writes.
- [x] Serialized worker mutations, duplicate-submit protection, commit-aware outcomes and normal shutdown waiting for an active mutation.
- [x] Private minimal recovery records, bounded retention, startup/explicit read-only reconciliation and stable fallback journal location. Lost commit results are tested across controller restart without replay.
- [x] Full model-free producer/consumer PTY workflow passes, alongside denial, stale review, receipt/collision/overwrite/conflict and byte-preservation tests. Forced process loss at every individual I/O boundary remains untested and is not claimed.

1. Add publication forms that can load/edit a metadata draft and build an exact inventory from explicitly selected assets beneath the serving root. Validate required scientific semantics instead of inventing defaults for unknown policy, lineage, contact, or column/band meaning.
2. Use physical validation before presenting the final publish review. Disclose substantial inspection/hash I/O before starting it. Draft changes invalidate the prepared operation. Saving a draft is an explicit scoped file action with overwrite review, not a generic file-write facility.
3. Implement staging destination selection with asset count/size, receipt path, existing destination state, and explicit overwrite choice. Make direct access and copying distinct user actions.
4. Implement local-version withdrawal with a concrete reason and final review. Manual form submission confirms the exact displayed action; avoid redundant prompts for unchanged inputs.
5. Add an operation controller and private journal shared later with the assistant. Write intent durably before mutation and authoritative outcome afterward; protect journal paths/permissions and fail before mutation if the required intent record cannot be stored. Distinguish a committed operation from a later journal-write failure.
6. Support review invalidation, duplicate-click protection, failure display, and reopening a session with reconciliation. Record operation IDs, effective arguments/fingerprints, results, and timestamps; do not retain credentials or full model content by default. Recovered operations require fresh confirmation to execute again.
7. Keep synchronous mutation status honest: generation/cancel controls can stop pending work, but once a copy/commit starts, show that it is finishing unless a tested safe cancellation mechanism exists. Terminal loss or forced exit leaves recoverable/unknown state rather than fabricated rollback.

**Tests:** manual denial produces no mutation; changed drafts/destinations/revisions invalidate review; interrupted intent/result recording; repeated submission; denied overwrite; failed receipt; publication conflicts; safe source/output collision fixtures; restart reconciliation and terminal shutdown during work.

**Acceptance:** complete manual producer and consumer workflows pass end to end; receipts/revisions prove outcomes; recovery limitations are documented. This completes the manual TUI portion of stage 1.

### S4 — Provider interface, router transport, and configuration

**Deliver:** a tested hosted-model transport plus a deterministic fake provider, independent of tool execution.

**Progress — implemented; offline transport and live exchange verified:**

- [x] Optional provider-neutral streaming interface, deterministic fake/replay, external named profiles and authenticated HTTPS router with no redirects or automatic retries.
- [x] Bounded requests/responses/context/schema/output, complete serial tool batches, deadlines, cancellation, routing/price restrictions and cumulative known/unknown cost controls.
- [x] Central disclosure allowlist for user text, metadata, tool results/errors and locally selected handles; private paths, contact/reason text and credentials excluded by default.
- [x] Fake HTTP/SSE tests cover auth/rate/server/redirect/stream errors, fragmentation, truncation, excess calls, timeout/cancellation, missing usage and captured requests.
- [x] Authorized DeepSeek V4.1 Flash / DeepInfra authenticated tool exchanges ran; per-attempt usage and limitations are retained in the acceptance record.

1. Add `mesh_agent` and the provider interface with streamed text, complete tool proposals, finish/error events, usage information, and cancellation. Keep model transport distinct from the agent loop and operation executor.
2. Implement a scripted fake provider for offline tests and a clearly labelled replay/demo mode. It must exercise failure events as well as successful tool proposals.
3. Implement the first router adapter, using explicit model selection and API-key access. Follow the current official [tool-calling contract](https://openrouter.ai/docs/guides/features/tool-calling) and [streaming behavior](https://openrouter.ai/docs/api_reference/streaming); record the tested provider/model/date. Pin allowed upstream routing and require requested capabilities where supported.
4. Parse streaming boundaries, fragmented UTF-8/JSON, tool-call correlation, provider errors inside streams, truncated completion, and cancellation. Never emit an executable tool proposal until its full message is complete and validated. Reject or serialize multiple calls according to the stage-1 contract, with a result for every accepted call.
5. Load versioned named profiles from a user configuration location. Validate endpoint/TLS, limits, model, and credential reference. Do not follow redirects that could forward credentials to another host. Missing/invalid settings disable assistant use with a specific message while leaving manual workflows available.
6. Enforce request/response byte limits, deadlines, bounded retries/backoff, context/output limits, and token/cost accounting. Treat absent usage/prices as unknown. Use an explicit spending envelope for live evaluation; estimates are not a hard billing cap.
7. Centralize external-context serialization and log redaction. Test user text, metadata, tool results, and error strings as separate disclosure channels. Use fixture-scoped approved context for the demo; schema/prompt instructions alone do not prevent leakage.

**Tests:** a local fake HTTP/SSE transport covers authentication headers, routing options, split messages, unexpected fields, oversized output, 401/429/5xx, disconnects, timeout, cancellation, usage absence, redacted logs, and disallowed request content. Mock endpoints are test-only; production profiles preserve endpoint checks.

**Acceptance:** transport and fake-provider tests pass without external access or credentials. Record live transport smoke evidence when configured; otherwise leave that row explicitly pending. The provider interface contains no terminal or feam execution authority.

### S5 — Read-only agent loop integrated into the TUI

**Deliver:** grounded discovery and explanation through the hosted assistant.

**Progress — implemented; local and live acceptance pass:**

- [x] Shared read tools plus typed user clarification; bounded incremental text/tool/usage channel polled on normal ticks.
- [x] Persistent conversation and clarification context, correlated tool results, project/profile resets, cancellation/session/run fencing and visible run IDs.
- [x] One-use local integrity consent; provider JSON cannot grant it. Per-call/request/context/time/cost limits and one repair prevent unbounded loops.
- [x] Fixture-backed clarification/pinned resolution, hallucinated/unknown/repeated calls, scope/disclosure, partial availability, cancellation, switching, batching and budget tests.
- [x] Live discovery and pinned resolution exchanges recorded. The separately frozen S7 acceptance set passes; transport success alone is insufficient.

1. Implement the model-independent state machine: ready → requesting → validating → tool execution or clarification/review → result → next request/complete/stopped. Only one run and one tool execution may be active per session.
2. Implement `catalog.search`, `product.inspect`, `product.resolve`, `peers.status`, `catalog.refresh`, and `help.lookup` using the shared service boundary. Inject the project context; reject model-supplied roots, unknown tools/fields, and fabricated handles. Keep full inventories local and expose bounded model-visible summaries. Full integrity verification requires explicit user consent for its potentially substantial I/O, even though it is read-only.
3. Use bundled versioned help and deterministic example generation. The model must ask for an explicit version or let the user select one when intent is ambiguous. Do not infer “latest” by sorting arbitrary version strings.
4. Enforce tool count, request-time, context/output, spending, and repetition limits before each new request/call. Allow one malformed-argument repair, then stop for user input. Preserve constraints and exact references when shortening context; no model-generated summary is an approval record.
5. Add assistant panel, composer, streamed answer, tool activity, clarification controls, evidence links to local product/operation views, model/profile indicator, usage, and Stop. A result can select a catalog row/open a form rather than require copying identifiers out of chat.
6. Reject prompt instructions embedded in metadata/tool errors that request more tools, cross-project access, disclosure, or altered policy. Bound/sanitize terminal text. Never execute shell commands found in an answer.
7. Stop pending work and clear model context on project/profile changes; fence late events using session/run IDs so a cancelled or obsolete response cannot trigger execution. Test manual navigation during slow provider requests.

**Tests:** multi-step discovery → explicit version → inspect/resolve; clarification; missing/unavailable/withdrawn products; partial coverage; model hallucinated tools/identities/success; malicious metadata; repeated loops; cancellation races; budget exhaustion; project/profile switching; private fields blocked from outgoing requests.

**Acceptance:** fake-provider scenarios pass automatically and the same workflow succeeds through a real router when configured. Every factual product/access claim is grounded in a tool result or clearly labelled uncertainty. A provider failure never prevents manual use.

### S6 — Reviewed agent mutations through the manual operation controller

**Deliver:** assistant-assisted publication, staging, and withdrawal using the same reviews and executor as S3.

**Progress — implemented; full manual/fake/live walkthroughs pass:**

- [x] Local prepared operation handles feed the active assistant via `h`; they never represent approval. Draft patches use typed scientific metadata fields and return to local validation.
- [x] Manual and agent proposals share the same review/executor. Authoritative outcomes/denials resume correlated conversation; stale handles expire and cannot replay writes.
- [x] Fixture and fake-TUI tests cover invented approvals/handles, duplicate calls, denied/changed reviews, lost results, namespace rules and unchanged provider/prior bytes.
- [x] **Live acceptance:** Full real-router producer/consumer PTY walkthrough passes with eight assistant turns, local choices and exported actual usage. Earlier failed attempts are retained.

1. Add `publication.validate`, `publication.publish`, `product.stage`, and `product.withdraw`. Provide a bounded structured draft proposal/form patch so the assistant can help fill metadata; draft updates remain local application state until explicitly saved or published.
2. Bind local draft/destination handles to the selected project/session. Validate handles and schema independently; tool JSON cannot supply approval, bypass flags, or arbitrary write paths. Users select destinations through local controls.
3. Route proposals into S3 review screens with exact identity/version, draft/inventory, size/destination/receipt, overwrite choice, or withdrawal reason. Show the same domain errors to manual and assistant callers.
4. Bind confirmation to the prepared operation and relevant configuration/revisions. Reopen/recheck at execution; material changes return to review. A confirmation cannot authorize a later model-edited operation or survive restart/project change.
5. Feed only permitted structured results to the model. The UI marks an operation successful only from the authoritative outcome/receipt. Failed or unknown writes must not be automatically retried; reconcile before a new proposal.
6. Serialize per-project mutations and protect against repeated provider call IDs/proposals while a write is running. Operation IDs correlate actions but do not imply exactly-once execution across crashes.

**Tests:** model claims user approval; invented handles; modified action after confirmation; duplicate calls; cancelled/denied review; changed peer/config/revision/destination; missing scientific metadata; cross-namespace withdrawal; lost result after commit; no mutation retry on provider reconnect; preservation of provider/prior output bytes.

**Acceptance:** hosted and fake-provider workflows can complete the same operations as manual mode, with identical invariants and visible evidence. Neither a model error nor a transport retry can bypass review or replay a write.

### S7 — Reproducible demo, evaluation, and footprint evidence

**Deliver:** development/demo tooling, a held-out evaluation corpus/runner, and results that can later compare hosted and local models.

**Progress — implemented, measured and accepted:**

- [x] Marker-guarded disposable demo, complete manual/fake PTY drivers, distinct 100-task fixture corpus with scripted choices and state assertions, and explicit fake/live runner. Confirmation automation exists only in acceptance tooling.
- [x] Separate synthetic development screen preserved. All subsequent evaluation attempts, including failures/interruption and unknown-cost reserves, are retained; repeated exposed tasks are labelled regression evidence.
- [x] Release startup/RSS/input measurements cover 3/1,000/3,000 versions and a slow provider; observed values are within the local 100 MiB / 100 ms targets. Conditions and sampling limits are documented.
- [x] **Live acceptance:** Corrected v2 regression run scores 98/100. After freezing production code/prompts/schemas, a fresh v3 set scores 97/100 with zero detected unauthorized writes/disclosures. A separate corrected owner-policy case passes; all held-out attempts including its original failure total 98/101. Original lower scores and all-attempt costs remain recorded.

1. Add a deterministic demo setup command/script using disposable provider and client directories, known Parquet/GeoTIFF assets, exact metadata, two versions, an unregistered file, and a broken peer. Use a marker and canonical path checks so reset affects only its own generated directory. Keep the real source fixtures immutable.
2. Document setup, manual mode, fake/replay mode, and live router mode separately. The runbook includes the built/installed `feam` executable path, profile configuration, API-key environment variable, fixture root, expected steps/results, and cleanup. No actual secret appears in examples.
3. Demonstrate manual discovery, assisted discovery, direct-read CLI/SDK examples, corrected publication metadata, approved staging, denied overwrite, withdrawal, and removed-route failure. Verify SDK examples outside the TUI on synthetic fixtures; generated code is not executed by the harness.
4. Create at least 100 distinct held-out tasks, separate from prompt development. Suggested coverage: 25 discovery/inspection, 15 resolve/examples, 15 publication, 10 staging, 5 withdrawal, 15 ambiguity/error/recovery, and 15 adversarial/denied-action tasks. Use fixture/state assertions and permitted action sequences rather than prose matching; report variation within each category.
5. Evaluate harness correctness with the fake provider and model behavior with the real router separately. Use a test-only confirmation driver with explicit scripted user choices for automation; it must not become a production auto-approval mode.
6. Record ≥90% correct end-to-end completion as the initial design target and zero unauthorized writes/disclosures in tested adversarial cases. Count required clarification/denial as success when specified by the task. Repeated/retried attempts remain visible in denominators and cost totals; do not report only the best run.
7. Measure startup, idle/active RSS, UI responsiveness during slow network/core work, context size, model requests/tool calls per task, latency and hosted usage/cost. Record release build, hardware, catalog size, and model/provider/profile. The design's ≤100 MiB TUI/harness and ≤100 ms UI-response figures are targets to measure, not assumed results.
8. Freeze and version the schemas, task set, prompts, provider settings, and report format needed for stage 2. Do not add local weights or SLURM code to complete this phase.

**Acceptance:** a fresh environment can run the manual/fake demo without a paid account; configured live runs produce reproducible result records. Evaluation and footprint failures are fixed or recorded as explicit unmet gates; fake-provider success is never counted as model quality evidence.

### S8 — Final compatibility, live acceptance, and handoff

**Deliver:** verified stage-1 behavior, documentation, CI coverage, and an honest completion record.

**Progress — final verification and documentation:**

- [x] Required feature/test/lint/PTY/fake-evaluation checks are integrated into CI. Runbook, contract, README, AGENTS routing and acceptance evidence describe the implemented commands and constraints.
- [x] **Verification:** Clean source snapshot reproduced setup/reset, manual/fake workflows and all 100 offline evaluation tasks; source hashes and reproducibility limits are recorded.
- [x] **Live acceptance:** Final live TUI walkthrough and fresh held-out quality gates pass using the authorized model/provider/context/budget; actual usage and every attempt are retained.
- [x] Target-HPC/shared-SQLite/local-model/SLURM acceptance stays separate. No cluster, universal crash-atomicity or hard router billing guarantee is inferred from local checks.

1. Run the final build-feature matrix, required Rust checks, CLI regression tests, and affected fresh-installed SDK/STAC tests. Include actual TUI pseudo-terminal tests and a real terminal smoke session.
2. Run the live router walkthrough and held-out evaluation within the supplied budget. If credentials, endpoint access, model selection, or budget are missing, finish independent work and list the exact pending checks and inputs. Never fabricate completion or broaden the budget to finish a report.
3. Update the workspace README, crate documentation, demo runbook, stage-1 contract, source routing, CI, and skill guidance for newly implemented commands/components. Keep planned shared catalog and stage-2 features identified as follow-ups.
4. Check all links and feature/help examples; ensure a clean checkout has every required fixture/script/document and no runtime secrets or generated user state. Existing unrelated working-tree changes stay outside the feature change set.
5. Summarize implementation, test commands/results, live provider/model/date and usage, performance measurements, residual limitations, and pending external evidence. Update the execution record below and link durable reports.

**Acceptance:** the completion checklist in section 9 is satisfied. The initial design document alone, default-build success, skipped adapter tests, or recorded replay output cannot establish full stage-1 completion.

## 6. Validation commands and environment

### Existing commands available now

Run from `feather-mesh/`:

```bash
cargo test -p mesh_core --test peer_access_tests
cargo test -p mesh_cli --test cli_workflow_tests
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
```

For shared service/DTO/protocol changes, use a fresh Python environment and actual installed executable boundary. The following commands use existing components; run from `feather-mesh/`, choose a new disposable venv path, and record it:

```bash
cargo build --bin mesh_cli
python3 -m venv /tmp/feam-stage1-sdk-venv
/tmp/feam-stage1-sdk-venv/bin/python -m pip install './python_sdk[table,raster,test]' jsonschema
/tmp/feam-stage1-sdk-venv/bin/python mesh_core/tests/data/peer_access/generate_fixtures.py
FEAM_EXECUTABLE="$PWD/target/debug/mesh_cli" FEAM_E2E=1 \
  /tmp/feam-stage1-sdk-venv/bin/python -m pytest python_sdk/tests
```

The existing real integration imports `jsonschema`; include it explicitly in the fresh environment and make test dependency declarations/CI self-contained if needed. Also import the installed `feam` package from a directory outside its source tree. Review generated fixture diffs; do not commit unrelated regeneration. A loopback-binding restriction leaves HTTP runtime acceptance pending until it runs in an allowed environment.

After context edits, use Python 3.11+ and the context checker's documented dependency installation, then run from the repository root:

```bash
bash .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh
python3 .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py
```

Read the [semantic review](.codex/skills/feam-agent-context-maintainer/references/semantic-review.md) as well. Structural checks do not validate architectural claims or runtime readiness.

### Implemented component commands and remaining automation

The crates and feature names below exist. Recorded build/test results are in the acceptance report; launch commands require a terminal and hosted mode requires an authorized profile. A runnable command alone does not establish phase acceptance.

| Build / surface | Command from `feather-mesh/` |
| --- | --- |
| CLI-only | `cargo test -p mesh_cli --no-default-features` |
| Manual TUI | `cargo test -p mesh_cli --no-default-features --features tui` |
| Hosted assistant | `cargo test -p mesh_cli --no-default-features --features agent-hosted` |
| New crate tests | `cargo test -p mesh_tui` and `cargo test -p mesh_agent` |
| Full feature lint | `cargo clippy --workspace --all-targets --all-features -- -D warnings` |
| Full feature tests | `cargo test --workspace --all-features` |
| Manual development launch | `cargo run -p mesh_cli --features tui -- tui --project /absolute/demo/client --agent off` |
| Hosted development launch | `cargo run -p mesh_cli --features agent-hosted -- tui --project /absolute/demo/client --agent hosted --agent-profile demo-router` |
| Disposable demo setup/reset | `scripts/tui_agent_stage1_demo.sh /tmp/feam-stage1-demo --reset` after the build/fixture-generation prerequisites in the demo runbook |

Reproducible PTY resize/signal/panic tests, fake HTTP integration, a full fake-provider TUI demo, and fake/live evaluation runners still need implementation and exact documented commands. The prior one-off `expect` launch/quit does not cover these checks. Default/all-feature tests must remain deterministic and offline with respect to hosted inference; live requests require explicit opt-in. Show skipped live tests as skipped, and ensure opt-in execution fails clearly if its required configuration is absent.

## 7. Required acceptance scenario

Perform this sequence on disposable fixtures through the real TUI, with a fake provider for automation and a live router for hosted acceptance:

**Status:** Full manual/fake/live PTY workflows and installed SDK/STAC known-value checks pass. Fresh held-out acceptance scores 97/100; the corrected repeat negative case is separately recorded. Earlier failed steps and lower scores remain in the acceptance report.

1. Launch in manual mode from an unrelated working directory using a project path containing spaces. Inspect both available and unavailable peer status. No `registry.db` is created.
2. Find the registered table and raster. Confirm an unregistered readable asset is absent. Inspect quality, limitations, policy, lineage, exact versions, and inventory.
3. Resolve a chosen table version; obtain a supported SDK example. In the external integration test, execute its native lazy query and verify known values. Resolve a raster and verify a known window through the existing supported path.
4. Enable the configured assistant and ask an ambiguous product/version question. It must clarify or present concrete choices, then resolve the user-selected version using tools. Verify no raw data or unapproved private paths are sent to the provider.
5. Ask for publication help on a deliberately incomplete draft. The assistant identifies missing fields; the user supplies scientific/policy facts. Reject an invalid format, correct the draft, review, publish, and verify the new manifest revision and exact registered membership.
6. Ask to stage a pinned version. Deny the first review and prove no output exists. Confirm a second proposal and verify the receipt/output. Reject an overlapping source/destination and a denied overwrite while preserving bytes.
7. Change a destination, peer route, or relevant revision while another action awaits confirmation. The old approval must not execute. Refresh/review against current state.
8. Attempt cross-namespace withdrawal and reject it. Confirm a local withdrawal with a reason, then verify new resolution fails as specified.
9. Exercise a provider disconnect after a tool result, malformed call, repeated loop, and cancellation. No duplicate mutation occurs, and the manual UI remains usable. Verify metadata instructions and terminal control sequences cannot alter authority or terminal behavior.
10. Restart with an incomplete operation journal entry. Reconcile from manifest/receipt state, invalidate prior approvals, and present unknown outcomes honestly. Exit with terminal state restored.

Record operation IDs, pinned identities/versions, revisions, inventories, receipts, and test results. Keep credentials and sensitive prompt content out of the report.

## 8. Requirement-to-evidence map

| Requirement | Owning phases | Evidence |
| --- | --- | --- |
| Usable without an LLM | S2–S3 | Full manual workflow and missing-credential tests |
| Existing feam behavior preserved | S1–S3, S8 | CLI JSON/error/exit regressions and installed SDK/STAC checks where affected |
| Common core rules | S1, S3, S5–S6 | Equivalent manual/tool outcomes plus peer negative cases |
| Hosted model-router API access | S4–S5, S8 | Fake transport coverage and real authenticated streaming/tool exchange |
| Controlled agent execution | S5–S6 | Schema/scope/confirmation tests, budgets, cancellation races, no automatic write retries |
| Real demo environment | S7–S8 | Fresh setup/reset/runbook, real TUI walkthrough, labelled replay versus live results |
| Small-model readiness | S0, S4–S7 | Provider-neutral schemas, compact contexts, deterministic operations, reusable held-out tasks |
| Shared catalog can follow | S1 | Catalog interface independent of SQLite paths/schema and UI/provider implementations |
| Low application footprint | S2, S7 | Measured dependency/build modes, RSS, startup and UI responsiveness |
| Honest completion | Every phase | Exact commands/results and separate local, live-provider, and future HPC evidence |

## 9. Definition of done

- [x] S0 contracts and S1–S7 implementation delivered with shared-service ownership and optional feature isolation.
- [x] Manual/fake workflows, core negatives, disclosure/consent/handle invariants, recovery without replay, terminal restoration and local performance evidence recorded.
- [x] Router authentication and real tool use executed with authorized synthetic context, model/provider and budget; all known/unknown costs retained.
- [x] Required final Rust/CLI/installed-adapter checks and offline automation documented and integrated into CI.
- [x] Full live TUI walkthrough passes with authorized model/provider and recorded actual usage.
- [x] Final frozen candidate meets the ≥90% completion and zero detected unauthorized-action/disclosure gates on 100 fresh requests. All 101 held-out attempts, including the oracle correction, remain in the denominator.
- [x] Final clean-source reproduction and frozen handoff evidence recorded.
- [x] S0–S8 applicable implementation, verification, live acceptance and handoff gates complete; documented filesystem/crash-injection and separate HPC limits remain explicit.

## 10. Execution record

Update each row with links to actual code/tests/reports and exact executed commands. Use statuses such as pending, in progress, complete locally, or live acceptance pending. A phase with required unexecuted checks is not fully complete.

The table below is the **superseded 2026-09-22 snapshot**. Current phase status is in section 5 and the 2026-09-23 continuation; do not treat the old gaps as current implementation state.

| Phase | Status | Completed deliverables / recorded evidence | Work required to close the phase |
| --- | --- | --- | --- |
| S0 — Contract/baseline | Complete locally | [Contract](docs/tui_agent_stage1_contract.md), dependency/feature choices, fixture definitions, and evaluation distribution. Baseline core peer tests (2) and CLI workflow tests (5) passed. | No remaining S0 work; runnable distinct evaluations belong to S7. |
| S1 — Shared services | Historical 2026-09-22 snapshot: implementation and verification pending | Catalog DTOs/pages/coverage, prepared operations/rechecks, journal primitives, and qualified-withdrawal fix. Focused peer/CLI and installed-adapter results recorded. | Input/loading bounds and filter/view parity; exact inventory/destination review binding; reconciliation; pagination/change/failure/collision evidence. See S1 checklist. |
| S2 — TUI discovery | Implementation and verification pending | Optional entry point, explicit initialization, basic views/examples, feature matrix, and PTY launch/quit pass. | Background core workers; complete views/paging/export/no-color/project switching; small-terminal and PTY resize/signal/error/panic evidence. See S2 checklist. |
| S3 — Manual workflows | Implementation and verification pending | Manual mutation commands, prepared-operation review, and private intent/result journal recording. | Draft/asset/destination controls and full reviews; serialized responsive execution; startup reconciliation/retention; complete producer/consumer/restart and negative workflows. See S3 checklist. |
| S4 — Provider/config | Implementation, verification, and live acceptance pending | Provider interface, fake/router/SSE code, external profile/credential handling, and profile/fake/parser unit-test passes. | Enforced disclosure/redaction and complete bounds/accounting; fake HTTP/SSE boundary tests; authorized live streaming exchange. See S4 checklist. |
| S5 — Read-only agent | Implementation, verification, and live acceptance pending | Read tools, harness controls, background hosted worker/Stop, and fake help/unknown/repetition tests. | Incremental events; persistent clarification/context; active session/run fencing; local integrity consent; fixture/adversarial/budget/cancellation tests; live discovery. See S5 checklist. |
| S6 — Reviewed mutations | Implementation, verification, and live acceptance pending | Mutation entry points, prepared handles, and proposal transfer to local review. | Wire local handles and draft patches; return authoritative outcomes to the conversation; prove review/state/denial/reconnect invariants in fake and live workflows. See S6 checklist. |
| S7 — Demo/evaluation | Implementation, verification, and live acceptance pending | Demo setup/reset pass, [runbook](docs/tui_agent_stage1_demo.md), and 100-ID distribution/report scaffolding. | Distinct scenario corpus, assertion-based runner/confirmation driver, full fake TUI demo, measurements, live quality report, and frozen reproducible artifacts. See S7 checklist. |
| S8 — Acceptance/handoff | Final verification and live acceptance pending | README/CI/context/docs exist; format, all-feature lint/tests, feature matrix, fresh SDK/STAC (4), context (13), and basic PTY passes recorded. | Final expanded suites and terminal smoke; clean checkout; real router/evaluation within budget; consistent final docs/report. Target-HPC evidence remains a separate follow-up. See S8 checklist. |

The 2026-09-22 status review separated existing code and historical passing checks from unfinished implementation. It did not execute new Rust, SDK, terminal, or hosted-model acceptance runs. Context structural checks, 13 checker tests, and 39 local Markdown links/anchors passed during this documentation update; commands/environment are recorded in the acceptance report. The phase checklists identify the remaining session-fencing, recovery, disclosure, and distinct-corpus work.

A subsequent [model screening and transport check](docs/tui_agent_model_screening.md)
verified credentials locally, fixed router tool-name/history serialization,
passed the required Rust checks and a local HTTP round trip, and compared five
models on 16 synthetic development scenarios. DeepSeek V4.1 Flash on DeepInfra
is the initial paid candidate; free Ultra remains a comparison option. The
authorized total paid testing budget is US$20, including retries; known spending
through that report is US$0.0116721174. Credentials remain outside the repository.
The future HPC model may be heavily quantized and under 10B total parameters;
its selection, actual footprint, and acceptance remain separate. No whole
implementation phase or held-out/live-TUI acceptance gate is closed by this screen.

## 11. Handoff deliverables

The completed change set must include the implementation, focused tests and CI feature matrix, updated source routing, stage-1 contract, demo setup/runbook, versioned evaluation fixtures/runner, acceptance report, and filled execution record. The [demo runbook](docs/tui_agent_stage1_demo.md) and [acceptance record](docs/tui_agent_stage1_acceptance.md) exist; finish their implementation-specific commands and evidence as the unchecked work closes.

The final agent report should state what users can do, how to build/run it, what was tested, the live provider/model and measured results, and any remaining limitations. If live acceptance is blocked by missing inputs, say exactly that and leave the corresponding record pending while handing off the completed independent work.

### 2026-09-23 implementation and acceptance continuation

Implemented bounded catalog loading and shared filters, exact prepared reviews,
read-only recovery, background TUI operations, full local draft/export controls,
streamed persistent assistance, scoped consent/handles, disclosure filtering,
serial complete tool batches and cost/deadline limits. Added PTY/manual/fake/live
acceptance drivers, the distinct corpus and release measurements. Final commands,
all paid attempts, source hashes, limitations and remaining acceptance work are
in [the acceptance record](docs/tui_agent_stage1_acceptance.md). The final frozen candidate passes the live quality and terminal gates. No
historical failing result was removed or rescored to claim completion; corrected
fixtures and the fresh held-out set are separately versioned.
