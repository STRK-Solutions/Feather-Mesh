# Web-accessible FEAM demo development workplan

Status: ready for implementation; W0–W10 have not started. Creating this plan does not implement, deploy, or validate the web service.

Updated: 2026-09-24.

Source of truth: [Ubuntu web demo design](ubuntu_web_dev_demo_design.md). Explicit owner decisions take precedence; this workplan supplies task order, execution environments, evidence requirements, and an updateable execution record. Keep architectural decisions in the design and delivery status here.

## 1. Instructions for the implementing agent

1. Read [AGENTS.md](../AGENTS.md), the design, this plan, and the latest [execution record](#8-execution-record). Inspect the checkout and preserve unrelated edits. Create or resume the task's feature branch before implementation; do not make untracked host edits the source of truth.
2. Keep the **Mac as the primary development workstation**. Write and review source, tests, image recipes, and IaC there. Execute Linux-specific integration on disposable Ubuntu first, then the actual Ubuntu machine. Do not postpone the first real-host proof until the dashboard is finished.
3. Apply the [context-maintenance skill](../.codex/skills/feam-agent-context-maintainer/SKILL.md) when components, commands, CI or routing change; the [peer-data-access skill](../.codex/skills/feam-peer-data-access/SKILL.md) for publication/resolution/import/SDK/STAC work; the [Rust skill](../.codex/skills/feam-rust-workflow/SKILL.md) for Rust changes; and the [CLI skill](../.codex/skills/feam-cli-contract/SKILL.md) for CLI/protocol changes. Read required sources before acting.
4. Use stable task IDs below. Change `[ ]` to `[x]` only when the task's artifact and stated verification exist. Do not renumber IDs; append new tasks when needed. Record a partial task as in progress rather than checking it off.
5. After each completed task or failed gate, update its evidence entry, the phase status, and the next-action field. Evidence names the commit/artifact digest, execution host, commands, result, limitations and relevant approval reference. Retain failures and superseded attempts.
6. A phase is complete only after its exit gate passes in the named environment. `Implemented on Mac`, `passed on a disposable VM`, `verified on Ubuntu`, and `verified on cloud` are different statuses. Historical Stage-1 results do not complete new web/broker tests.
7. If host access, a dependency or an external approval is unavailable, record the exact blocked task and continue independent Mac/offline work whose prerequisites are satisfied. Do not broaden permissions, bypass authentication, silently skip a required validator, or substitute a mock for target-host evidence.
8. Before ending an implementation session, leave a concise handoff: completed IDs, changed files, executed/unexecuted checks, pending authorizations and the next ready task. Keep private identities, credentials and raw user traces out of public reports.

This document is a development plan, not authorization to purchase a domain, provision paid resources, send invitations, expose a terminal publicly, reboot the shared Ubuntu host, or run billable inference. Obtain the applicable approval once for a clearly bounded operation; do not request repetitive permission for ordinary authorized implementation steps. Preauthorized failover must have its own recorded scope and cost ceiling before it is enabled.

## 2. Where work happens

An environment label identifies **where the operation executes**, not where the agent's editor is open. A command launched from the Mac over SSH that modifies Ubuntu is an Ubuntu operation.

| Label | Environment | Work performed there | Does not establish |
| --- | --- | --- | --- |
| MAC | This macOS ARM64 development checkout | Source/docs/IaC authoring, application unit tests, browser tests against local fake services, synthetic import fixtures, review and operator orchestration. | Ubuntu security, disk/mount behavior, unattended boot, or deployed capacity. |
| LINUX | Disposable Ubuntu 24.04 x86-64 VM or suitable Linux CI/build environment | Native `linux/amd64` builds, service integration, real systemd/rootless/cgroup/mount tests in a full VM, destructive fault fixtures, repeat provisioning. | Behavior on the owner's physical machine or paid cloud provider. Container-only CI is not a full Ubuntu VM. |
| UBUNTU | The actual Ubuntu demo machine | Read-only preflight, approved Ansible provisioning, private staging deployment, socket/identity/storage verification, reboot and ten-user acceptance. | Cloud allocation, real provider failover time, or multi-node HPC acceptance. |
| EDGE | Project-owned registrar/Cloudflare services, controlled from MAC or an approved runner | Approved domain/DNS/HTTPS/Tunnel/Access setup and actual identity/route tests. | Application correctness merely because the tunnel connects. |
| CLOUD | Selected cloud region and independent recovery services/runner | Approved state/journal/backup deployment, prebuilt image, cold VM creation, fenced promotion, same-capacity acceptance and timed handback. | A 15-minute recovery capability until actual cold-start drills pass. |

Use MAC for day-to-day coding and LINUX for release artifacts; do not copy a native macOS binary into a Linux image. Record OS/architecture, native dependencies and digests. Emulated Linux on the Mac can assist functional iteration but cannot supply native performance or actual-host acceptance evidence. Do not assume ordinary hosted CI permits systemd, reboot or loop-mount tests; provision a suitable disposable test VM when those are required.

### Mac-to-Ubuntu development loop

1. Edit and run narrow tests on MAC; keep fake authentication, inference and recovery backends confined to explicit test configurations.
2. Build/test the exact revision on LINUX. Retain the FEAM binary, host-service artifacts, container image and dependency manifest by digest.
3. From MAC, invoke approved Ansible against a verified private SSH inventory. Install artifacts on UBUNTU into the dedicated service/staging scope, not the personal home or existing Docker daemon.
4. Exercise the Ubuntu-hosted application from a browser on MAC through a private test connection initially, and through real EDGE authentication in W8. Collect host-side measurements on UBUNTU.
5. Fix source/configuration on MAC, rebuild, and redeploy the new pinned release. Capture any emergency host repair in IaC before marking the task complete.

Before the machine serves participants, bounded native builds may use a separate Ubuntu build account/path if no Linux runner exists and the operator approves the resource use. Once demos run, move heavy builds, destructive tests and SLM training elsewhere. Keep staging data, service identities, sockets and ports separate from participant resources; do not expose Docker's API publicly.

## 3. Scope and settled decisions

- [x] Design exists, with Terraform for external resources, Ansible for host configuration, and systemd/application recovery rather than infrastructure applies at every reboot.
- [x] Four admins and three regular users are specified privately; enrollment stays invited. Admins have management access, not automatically allocated workspaces. Target capacity remains ten regular users.
- [x] Email PIN login, separate application audiences/role permissions, 30-minute idle stopping and 30-day inactive-workspace retention are accepted. Retention deletion needs advance warning; reset needs confirmation.
- [x] Participant consent already exists; no new participant-notice or repeat-consent gate is required. Recording status, withdrawal/deletion handling and restricted research exports remain required.
- [x] Initial data will be small Canadian government climate products: physically valid GeoTIFF `.tiff` rasters and Parquet-only published tables. AAFC/ECCC candidates and the proposed 250 MiB seed-bundle limit are in the design.
- [x] The selected model is `deepseek/deepseek-v4.1-flash` through OpenRouter, pinned to `deepinfra/fp8`, with provider fallback and automatic retries disabled. Reuse the evaluated profile, subject to current route/price validation.
- [x] The project allowance is US$100 total, including known spend and outstanding/unknown reservations. New requests pause until an admin raises it. It does not reset by user, month, key rotation or site change.
- [x] Cloud fallback must offer the same service and recover within **15 minutes after failure detection**. New private workspaces are acceptable; cross-host workspace transfer is not required. Accounts, grants, datasets, restrictions, research records and spending must remain coherent.
- [x] MAC is the main development workstation; UBUNTU is an early and continuing integration/deployment target.

These checked items record decisions, **not implemented capabilities**. The [Stage-1 acceptance record](tui_agent_stage1_acceptance.md#final-fresh-acceptance) contains historical 97/100 held-out model evidence. Keep it distinct from W8's new broker/browser validation and W9's deployed capacity/recovery proof.

Reuse the current Rust workspace, [peer contract](data_access_contract.md), [SDK](../feather-mesh/python_sdk/README.md), [Stage-1 contract](tui_agent_stage1_contract.md), [demo runbook](tui_agent_stage1_demo.md), and [existing CI](../.github/workflows/rust.yml). The old proposed baseline in the [peer implementation plan](../data_access_implementation_workplan.md) is historical; its execution record and current source establish what is already implemented. Do not rebuild peer discovery, STAC or the harness as a new web-specific authority.

Out of scope: public enrollment, Kubernetes, VM-per-user isolation, unrestricted internet for sandboxes, an internet-facing STAC/download service, admin development workspaces, migrating live terminal processes, workspace replication between sites, and training/deploying an SLM. This plan delivers the reviewed evidence/export foundation for later SLM work, not Phase 2 training itself. HPC acceptance remains separate.

## 4. Inputs, approvals and proposed artifacts

### Inputs that must be resolved without blocking unrelated development

| Input ID | Needed input or approval | Needed before | Work that can continue meanwhile |
| --- | --- | --- | --- |
| I1 | Verified Ubuntu SSH/operator access, sudo/provisioning scope, service paths and maintenance window | W0 actual-host preflight; W1 host mutation; W9 reboot/fault checks | MAC work and disposable LINUX tests. Never guess a hostname or use broad filesystem targets. |
| I2 | Exact domain, current registration/renewal price, owner-controlled registrar/Cloudflare account and scoped credentials | W8 EDGE setup | Fake-JWT gateway tests, local hostnames, Terraform validation. A numeric `.xyz` is only a candidate; nothing is registered. |
| I3 | Independently recoverable encrypted state/backups/keys, strongly consistent journal/lease backend and runner; storage/API budget | W7 real recovery-backend tests; any W8 billable dispatch | Backend interfaces, local fault doubles, disposable restore tests. |
| I4 | OpenRouter credential, current pinned-route compatibility/prices, finite live-test authorization and per-request/user guardrails | W8 live model testing | W5–W6 fake inference, stream and spending tests. US$100 is the project envelope, not a mandate to spend it in tests. |
| I5 | Canadian-region cloud provider, supported full-service instance/storage, standing and active cost ceilings, image/drill/failover approval | W9 CLOUD resources and drills | Terraform/image recipes, independent-runner logic and local two-site simulations. |
| I6 | Demo hours/contact, research retention/recovery age, export reviewers and Phase 2 ownership | Real participant capture/export and W10 handoff | Synthetic telemetry tests; do not ask for participant consent again. |
| I7 | Exact government source objects, licenses/attribution, checksums, station/date/raster subset and bounded fetch scope | W4 real seed imports and approval | Synthetic CSV-to-Parquet/GeoTIFF tests; no live fetch is needed for ordinary CI. |

The private roster is at `.local/demo-deployment/participants.yaml` on the current Mac, ignored by Git; it will not exist in a fresh clone. Transfer it deliberately into encrypted operator configuration before bootstrap. Do not print it, commit it, embed it in images/Terraform state, or silently replace missing private input with a broad allowlist. Keep decryption credentials separately recoverable.

### Proposed source layout

These are **future deliverables**, not runnable paths today. W0 records any justified layout adjustment and updates routing/commands immediately.

```text
web_demo/                             new web/control services, separate from FEAM authority
  README.md                           MAC development and LINUX integration commands
  cmd/                                gateway, controller, broker, collector, pipeline entrypoints
  internal/                           auth, control DB, typed IPC, lifecycle, budgets, recovery
  migrations/                         versioned control/ledger/index migrations
  templates/                          server-rendered portal/admin pages
  importers/                          bounded source conversion and provenance
  tests/                              browser, fake-provider, isolation and crash scenarios
infra/demo/                           layout follows the design
  README.md                           operator commands and approval boundaries
  terraform/{edge,recovery-storage,cloud-host}/
  ansible/{inventories,roles}/
  ansible/{preflight,host,deploy,restore}.yml
  images/                             FEAM terminal and prebuilt cloud image recipes
  tests/                              full Ubuntu VM provision/reboot/restore/failover harness
docs/evaluations/web-demo/             sanitized task evidence, created as checks run
docs/ubuntu_web_dev_demo_acceptance.md  measured results and unresolved gates
```

Use the design's small Go/server-rendered gateway as the starting proposal; select and record the remaining host-service/importer implementation details in W0. Separate processes, OS identities and typed private IPC must preserve authority even if code shares a module. Keep FEAM peer business rules in `mesh_core`, terminal/review state in `mesh_tui`, and provider adaptation in `mesh_agent`. The gateway must never inherit Docker, host-root, Terraform, dataset-publication or upstream-model-key authority.

## 5. Delivery order

| Phase | Deliverable | Prerequisite | Implementation / required verification |
| --- | --- | --- | --- |
| W0 | Baseline, interfaces, safe execution setup | None | MAC; LINUX toolchain; UBUNTU read-only preflight |
| W1 | First isolated browser terminal on actual Ubuntu | W0 relevant baseline/access | MAC authoring → LINUX build/VM proof → UBUNTU private proof |
| W2 | Control DB, identity and authorization gateway | W0 contracts, W1 runtime interface | MAC/LINUX tests → UBUNTU private integration; real EDGE in W8 |
| W3 | Workspace controller and management UI | W1, W2 | MAC/LINUX → UBUNTU lifecycle/resource proof |
| W4 | Climate imports, immutable releases and grants | W2, W3; I7 for real sources | MAC conversion tests → LINUX real readers → UBUNTU pipeline |
| W5 | Hosted broker, adapter and shared budget | W2, W3; W0 journal contract | MAC fake-provider tests → LINUX/UBUNTU sockets; live only after W7 |
| W6 | Structured capture and research controls | W2, W5 | MAC/LINUX synthetic traces → UBUNTU durable collection |
| W7 | Complete IaC, release/reboot recovery and off-host durability | W1–W6; I3 for real backend | MAC recipes → LINUX failure tests → UBUNTU + independent backend |
| W8 | HTTPS/Access and real hosted web integration | W2–W7; I2–I4, I6 for real capture | MAC orchestrates → EDGE + UBUNTU; browser on MAC |
| W9 | Ten-user Ubuntu and 15-minute cloud acceptance | W8; I1 maintenance + I5 cloud approval | UBUNTU and CLOUD; browsers from Ottawa |
| W10 | Invited cohort handoff and evidence/export readiness | W9 gates, I6 | MAC docs → UBUNTU/EDGE cohort activation; CLOUD readiness recorded |

W4 and W5 can be developed independently after their prerequisites; W7 recovery interfaces/tests should start during W0–W5 rather than be retrofitted after live traffic. An unavailable real host/backend may leave a phase's target gate pending while independent MAC/LINUX tasks progress. It never turns a pending gate into a pass.

## 6. Checkable implementation tasks

### W0 — Baseline and execution setup

Output: reproducible development setup, interface decisions, safe target inventory and first-run test record.

- [ ] W0.01 [MAC] Record branch/revision, existing dirty changes, relevant implemented commands and current test status. Create the acceptance/evidence index with environment labels; preserve existing Stage-1 evidence unchanged.
- [ ] W0.02 [MAC] Inspect/install only required development tools in isolated environments. Record Rust/Python and selected Go/IaC/browser-test versions, lockfiles and native dependency strategy; do not treat an existing Mac environment as a verified Linux toolchain.
- [ ] W0.03 [MAC] Define module/process ownership, local control/ledger/event schemas, typed Unix-socket protocols, request/generation IDs, and migration/bootstrap/restore boundaries. Specify that duplicate or uncertain mutations reconcile rather than retry blindly.
- [ ] W0.04 [MAC] Define recovery-journal durability and activation-lease interfaces now: one active generation, off-host reservation before model dispatch, idempotent replay, credential scoping and stale-holder denial. Provide fake backends for local tests, with no production fallback to them.
- [ ] W0.05 [LINUX] Establish a native Ubuntu x86-64 build path and a disposable full-system test VM. Prove access to systemd/cgroup v2 and scoped loop mounts. Record what ordinary CI can and cannot test.
- [ ] W0.06 [UBUNTU; I1] Run the design's read-only preflight: current OS/kernel, cgroups/AppArmor, existing Docker workloads/packages, SSD filesystem mapping, free bytes/inodes, identity/subordinate-ID collisions, time sync and private management reachability. Do not install or reconfigure during preflight.
- [ ] W0.07 [MAC] Create sanitized inventory/config examples and a typed settings schema. Track I1–I7 without placing real identities/secrets in Git; retain the initial private roster and existing-consent confirmation separately.
- [ ] W0.08 [MAC/LINUX] Add executable development/test commands and CI jobs as components appear. Keep public PR checks offline and without production credentials; separate authorized live/deployment workflows.
- [ ] W0.G [MAC + LINUX + UBUNTU] Gate: interfaces and toolchain are recorded, baseline failures are accounted for, a full-system test target exists, and actual-host preflight establishes a safe provisioning scope. If UBUNTU is unavailable, record that gate pending and continue only independent work.

### W1 — First isolated browser terminal on Ubuntu

Output: an approved, private one-user runtime proof, before developing the full portal. Manual/fake assistance is sufficient for this plumbing milestone, not for final demo acceptance.

- [ ] W1.01 [MAC → LINUX] Build the FEAM terminal image with the hosted feature, `ttyd`, supported SDK/readers, pinned fixtures and final-path demo initialization. Install `mesh_cli` as `feam`; preserve project paths with spaces and provider symlinks. Retain native Linux image/binary digests.
- [ ] W1.02 [MAC] Write idempotent Ansible accounts/runtime/storage roles. Use a dedicated rootless runner, non-overlapping subordinate IDs, private daemon socket/data root, supported host RootlessKit policy, a systemd user service with lingering and measured cgroup delegation. Preserve the existing host Docker daemon/workloads.
- [ ] W1.03 [LINUX] Prove a non-root, read-only-image container with dropped capabilities, supported seccomp, no-new-privileges, `--network none`, bounded memory/CPU/PIDs/tmpfs and no extra swap allowance. Do not claim unsupported per-container AppArmor confinement in rootless mode.
- [ ] W1.04 [LINUX] Create only disposable, ownership-recorded ext4 backing files and mount units. Test Unix-socket ACLs/UID mappings, bounded socket tmpfs, expected filesystem identities and system/user-manager ordering. Missing or wrong mounts must fail without writing into underlying directories.
- [ ] W1.05 [MAC/LINUX] Add a minimal private browser-to-terminal proxy with fixed launcher arguments, WebSocket origin checks, header filtering and two connections per workspace. Any test identity mechanism must be unavailable in deployable production configuration; do not open an unauthenticated public terminal.
- [ ] W1.06 [UBUNTU; I1] Apply the tested scoped roles and pinned artifact to a private test slot. From MAC, reach it over the approved private connection and execute the FEAM manual/fake walkthrough. Capture actual CPU/memory/PID enforcement, socket denial across identities, private-volume persistence and confined reset.
- [ ] W1.07 [LINUX, then approved UBUNTU] Repeat converge and verify no reformat/reinitialization, no namespace-policy weakening and no impact on unrelated services. Test runner start without interactive login; defer actual shared-host reboot to an approved window.
- [ ] W1.G [UBUNTU] Gate: a real Ubuntu-hosted browser terminal works with correct isolation and persistent local files; exact provisioning/build/smoke commands and evidence are recorded. A Mac-only container or idle HTML page does not pass.

### W2 — Identity, control data and authorization gateway

Output: tested roles/ownership and a portal that is ready for real Access integration in W8.

- [ ] W2.01 [MAC] Implement explicit control-DB initialization and migrations for accounts, site-local workspaces, jobs, grants and audit records. Add unique identities/idempotency constraints and immutable local IDs; do not use emails as paths or FEAM's legacy SQLite as web control state.
- [ ] W2.02 [MAC] Implement idempotent private bootstrap for four admins and three users. Subsequent converge must not resurrect deleted accounts or revert roles. New accounts stay pending until the edge membership and required workspace assignment are ready; admins receive no workspace.
- [ ] W2.03 [MAC/LINUX] Validate JWT signature, issuer, exact host-specific audience, expiry and active local identity. Use maintained validation libraries and bounded key refresh; reject forged headers, wrong audiences, stale/unknown identities and malformed tokens. Test key rotation/offline-key behavior explicitly.
- [ ] W2.04 [MAC/LINUX] Enforce separate portal/admin/workspace origins, exact hostname routing, host-only secure cookies, CSRF protection, WebSocket Origin checks and credential-header stripping before sandbox proxying. Bind every workspace hostname to an authorized server-side record.
- [ ] W2.05 [MAC] Implement exact-email group reconciliation and pending/error reporting with a fake provider first. Terraform owns referencing applications/static policies, not mutable membership groups; test empty groups fail closed and a later apply cannot restore removed membership.
- [ ] W2.06 [MAC/LINUX] Add local account authorization versions and active-stream tracking. Disabling an account rejects new requests immediately, closes streams within 30 seconds, stops workspaces and revokes model/event capabilities even when edge sync fails.
- [ ] W2.07 [UBUNTU] Run the gateway and control store under their intended identities with synthetic test accounts on the private path. Verify cross-user/cross-role HTTP and WebSocket denial using two real containers, including sandbox-controlled response headers/cookies.
- [ ] W2.G [MAC/LINUX + UBUNTU] Gate: migrations, ownership, revocation and private integration pass with automated negative tests. Real email delivery/Access JWTs remain explicitly pending W8; test assertions are not production authentication.

### W3 — Workspace lifecycle and management dashboard

Output: users operate their own sandbox; admins manage the service through bounded actions without shell/editor access.

- [ ] W3.01 [MAC] Implement controller-only access to the dedicated runtime. Accept workspace IDs and fixed actions/templates, never client-supplied host paths, mount specs, images or commands. Authenticate typed private IPC and separate gateway/controller permissions.
- [ ] W3.02 [MAC/LINUX] Implement transactional capacity reservation, per-workspace serialization, expected-generation checks, recorded container labels and recoverable lifecycle states. Test concurrent start/reset, lost responses, orphan reconciliation and partial initialization without duplicate allocations.
- [ ] W3.03 [MAC → UBUNTU] Expand the verified pool to ten 2 GiB workspaces plus two 2 GiB reset spares, and apply the design's aggregate budgets. Format only newly proven unassigned files; never create unbounded extra reset volumes or delete unowned runtime resources.
- [ ] W3.04 [MAC/LINUX] Implement 30-minute activity-based idle stop, warning, bounded in-flight-operation handling, two terminal connections, reconnect behavior, 30-day inactive-file retention with advance warning, and confirmed reset. Heartbeats alone do not count as activity.
- [ ] W3.05 [MAC] Build server-rendered user controls and admin account/health/quota/job pages. Admin support access to private workspaces is separately authorized/audited; management roles cannot execute arbitrary commands, grant host sudo or allocate themselves editors.
- [ ] W3.06 [MAC/LINUX] Enforce image/config compatibility and explicit generation switching; failed resets preserve the old stopped generation but still obey current grants. Never replay TUI approvals or uncertain FEAM mutations after a restart.
- [ ] W3.07 [LINUX → UBUNTU] Test memory/CPU/PID/disk/inode containment, queueing, aggregate headroom, spare exhaustion and real read/write boundaries. Use small disposable volumes for disk-full tests; do not fill the shared host or apply a 20 GiB host-free-space threshold to a 2 GiB workspace.
- [ ] W3.G [UBUNTU] Gate: two-user isolation and all lifecycle actions work through the UI; full pool/limits are provisioned with preserved existing data. Ten-user performance remains W9, not inferred from configured quotas.

### W4 — Government climate datasets and shared access

Output: one approved `demo-climate` bundle with a small real raster and Parquet table, immutable releases and tested audience grants.

- [ ] W4.01 [MAC] Define non-executable source/job schemas: allowed source, pinned object/version, exact asset inventory, limits, audience, model-disclosure/research-use policy, provenance and approval hash. Bound source count, redirects, elapsed time, bytes and archive expansion; reject LAN/metadata/host destinations and unsafe archive paths.
- [ ] W4.02 [MAC/LINUX; I7 for real fetch] Implement the ECCC bounded CSV/API-to-Parquet importer and AAFC GeoTIFF preparation. Keep raw source bytes private; publish no CSV/JSON tables. Pin actual station/date/variable and source/output hashes. Preserve nulls, quality flags, units and licensing/attribution.
- [ ] W4.03 [MAC/LINUX] Test raster georeferencing/nodata/scientific time and known pixels. If the actual source is not compatible with current WGS84 STAC geometry, create a tested EPSG:4326 derivative with explicit resampling/transform provenance; never relabel the CRS or invent observation time.
- [ ] W4.04 [MAC] Implement private candidate construction, complete-byte reservation, per-bundle serialization and FEAM validation/publication through explicit project-root argument arrays. Preserve prior manifest records/tombstones. Registration is through Rust services/CLI, not file presence, SQLite edits or hand-written manifests.
- [ ] W4.05 [LINUX] Test corrupt/renamed formats, missing metadata/inspectors, incompatible Parquet shards, namespace conflicts, duplicate versions, partial downloads, concurrent writers and projection failures. Separately reconcile manifest commit versus release promotion; no blind retry after an unknown result.
- [ ] W4.06 [MAC/LINUX] Add exact-hash admin approval, immutable complete-release promotion on one filesystem, safe orphan recovery, grant-aware assignment and retention. No raw input credentials, drafts or scratch files may appear in a mounted serving tree.
- [ ] W4.07 [UBUNTU] Run the real approved small imports under the bounded pipeline identity. Validate the proposed 250 MiB seed bundle and host storage budgets; reserve complete candidates within the 200 GiB dataset filesystem, 50 GiB staging and 100 GiB retained-release policy.
- [ ] W4.08 [UBUNTU] Mount only authorized releases read-only; attach client links/configuration without replacing practice peers, refresh, and resolve exact inventory. Execute native Polars queries and STAC-selected Rasterio windows in the sandbox with known results; test standard-client tokenless pagination on exact IPv4 loopback (`127.0.0.1`) and rejected non-loopback binding where STAC is exercised. Keep the metadata service and client inside the same isolated workspace; browser/portal authentication remains separate and mandatory.
- [ ] W4.09 [UBUNTU] Grant only user A a test bundle and deny B even by shell path. Test mount writes/symlink writes, reset isolation, container recreation for upgrades/revocations, withdrawal tombstones and rollback restrictions. Already staged copies are not represented as remotely revocable.
- [ ] W4.G [LINUX + UBUNTU] Gate: real outputs/provenance, inspector failures, supported installed SDK/readers, exact manifests and grant enforcement pass. Source catalog links or static STAC JSON alone do not complete the dataset pipeline.

### W5 — Inference broker, socket adapter and project budget

Output: the existing hosted harness works through a constrained broker without upstream keys or general internet in user containers. Use fake provider responses until W7 durability is proven and W8 live traffic is authorized.

- [ ] W5.01 [MAC] Implement server-selected model/provider/profile enforcement for the evaluated DeepSeek route, approved tools, bounded payloads and disclosure filters. Preserve internal/provider-safe tool-name mapping, tool-call IDs and typed errors; keep the tested omission of unsupported `parallel_tool_calls`.
- [ ] W5.02 [MAC → LINUX] Implement the in-container HTTPS-loopback-to-private-Unix-socket adapter, scoped trust configuration and revocable workspace/account capabilities. Store the real OpenRouter key only in the broker service. A shell-readable capability must grant no authority beyond its workspace and current budget.
- [ ] W5.03 [MAC/LINUX] Test full streaming exchanges, split UTF-8/SSE frames, complete tool arguments, usage frames, cancellation, disconnects, provider errors and no automatic retries. User-edited profiles must not change provider/model, proxy arbitrary URLs or bypass limits.
- [ ] W5.04 [MAC] Implement one in-flight request per user, three across the active host, fair queueing and dispatch-time rechecks of account, workspace, grants, capability and activation generation.
- [ ] W5.05 [MAC/LINUX] Implement a transactional US$100 project ledger with worst-case reservation before dispatch, final/unknown accounting, price/fee handling and idempotent reconciliation. Require W0's off-host journal acknowledgment before any real dispatch; test crashes between local commit, remote acknowledgment, dispatch and finalization.
- [ ] W5.06 [MAC] Add proposed 75%/90% alerts, visible spend/reservations/queue state and an audited admin increase of the total ceiling. Test near-limit admission, concurrent reservations, unknown cost, no calendar/reset/site replenishment and no silent model substitution using simulated charges, not US$100 of real spending.
- [ ] W5.07 [UBUNTU] Test the actual socket/certificate/identity configuration with fake upstream streams, multiple isolated workspaces and broker restarts. Demonstrate that neither the gateway nor a sandbox can retrieve the upstream key or another conversation/capability.
- [ ] W5.G [MAC/LINUX + UBUNTU] Gate: fake end-to-end compatibility, negative authority checks and crash-safe ledger logic pass. Live route compatibility remains W8; actual off-host durability remains W7 until independently verified.

### W6 — Structured usage capture and research controls

Output: correlated, bounded and provenance-aware events, with existing consent represented administratively.

- [ ] W6.01 [MAC] Define versioned event schemas, pseudonymous IDs and correlation across request, tool proposal, local review/edit/denial, actual outcome, usage and optional feedback. Record software/profile/dataset revisions and distinguish broker-observed, client-reported and independently verified evidence.
- [ ] W6.02 [MAC/LINUX] Add targeted TUI/harness/service instrumentation and per-workspace event submission over private sockets. Preserve terminal lifecycle and mutation-review semantics. Do not replace structured events with raw terminal transcripts, keystroke capture, the recovery journal or hidden model reasoning.
- [ ] W6.03 [MAC/LINUX] Implement authentication, schema/size/rate checks, deduplication, ordering, bounded append-only storage and index recovery. Test forged client outcomes, missing spans, disk-full, collector restart and explicit capture backpressure/unrecorded status.
- [ ] W6.04 [MAC] Redact credentials, email addresses, private paths and raw dataset payloads before persistence/export. Keep participant/account mapping and operational billing separate from the research corpus; restrict trace review/export beyond ordinary admin dashboard access.
- [ ] W6.05 [MAC; I6 before real capture] Implement configured research retention, deletion/withdrawal propagation and export lineage through backups. Represent the owner's existing-consent confirmation without inventing signatures/versions or adding a participant-notice/consent popup.
- [ ] W6.06 [UBUNTU] Trace a synthetic user workflow from request through verified result, including denial/cancellation and missing events. Prove reset does not erase eligible history or evade deletion restrictions; measure capture coverage with the actual deployed processes.
- [ ] W6.07 [MAC/LINUX] Produce a small synthetic reviewed export with verified labels, retained negative examples and participant/dataset/task-aware held-out separation. Mark synthetic material as synthetic; do not claim participant evidence or start SLM training.
- [ ] W6.G [UBUNTU] Gate: trace correlation, redaction, quotas, failure behavior and export permission checks pass. Real-cohort export eligibility/ownership remains a handoff gate, not inferred from test data.

### W7 — Complete IaC, releases and durable recovery

Output: independently recoverable state, supervised services and safe repeatable deployment; prerequisites for real billed web traffic.

- [ ] W7.01 [MAC] Complete Terraform edge/recovery-storage/cloud-host modules with locked providers and separate state lifecycles. Encode one writer per resource: static applications/policies in Terraform, exact-email membership in the reconciler, active tunnel targets in the failover controller. Test promotion/removal followed by apply without rollback of runtime decisions.
- [ ] W7.02 [MAC/LINUX] Complete Ansible preflight/host/deploy/restore entrypoints with idempotent ownership checks, secret-log suppression, compatible migrations, selective restarts and approved-artifact rollback. Bootstrap, schema migration and restore must never run as ordinary boot side effects.
- [ ] W7.03 [MAC/LINUX] Implement systemd units/timers and the cross-manager storage-to-rootless-runner ordering. Install bounded logs, socket mounts/ACL recreation and health checks. Disable autonomous Docker restarts so the controller checks current grants/storage/activation before starting workspaces.
- [ ] W7.04 [MAC → independent backend; I3] Deploy and verify encrypted versioned state/backups plus independently recoverable keys, strongly consistent activation/control/billing journal and scoped identities. Prove CAS/lease and acknowledgment semantics on the selected real service; local mocks are insufficient. Operator/recovery credentials never enter sandboxes or ordinary CI artifacts.
- [ ] W7.05 [MAC/LINUX → UBUNTU] Implement bounded coordinated SQLite checkpoints, journal replay, seven daily recovery points and configured incremental state capture. Preserve current revocations, consent restrictions, budget increases and unknown reservations. Keep historical research archives recoverable without bulk-hydrating them on startup.
- [ ] W7.06 [MAC/LINUX] Implement independent monitoring/recovery-runner contracts, activation generation/lease checks on every relevant service, and fencing of stale public access and external mutation authority. Test partitions, expired leases, paused/resumed processes, stale credentials and runner restart. Health-probe failure alone never authorizes a second writer.
- [ ] W7.07 [UBUNTU + independent backend] Test ordinary component restarts and control-state restore under the intended identities. Simulate journal/backup outage and uncertain billing; verify new billed dispatch stops and restrictive account/grant changes fail closed during restore.
- [ ] W7.08 [LINUX] Recreate the service from a clean VM with no developer checkout or personal credentials: first converge, second converge, release migration/rollback, registry-unavailable reboot and missing/wrong mounts. Keep actual shared-host reboot approval for W9.
- [ ] W7.09 [MAC/LINUX] Implement a reproducible prebuilt cloud image and bounded startup-state/release manifest for the accepted cold-start design. No FEAM compilation, government-source processing or full workspace/trace restoration may be required during failover. Verify an assigned dataset has an independent recovery copy before declaring it recovery-ready.
- [ ] W7.G [LINUX + UBUNTU + independent backend] Gate: repeat deployment/recovery is proven; real off-host reservations and lease authority work; artifacts/secrets/state are independently recoverable. Fake backends remain test-only. Paid VM allocation and 15-minute timing are still W9.

### W8 — Real HTTPS, Access and hosted web integration

Output: a protected, staged end-to-end service on Ubuntu, with real provider compatibility and finite-test evidence.

- [ ] W8.01 [MAC → EDGE; I2] Verify and obtain approval for the exact domain/renewal price, then configure ownership/DNS and the fixed single-level portal/admin/workspace hostnames. No bundled paid certificate/email/hosting purchase is assumed. Record MFA and scoped credential/recovery ownership.
- [ ] W8.02 [EDGE + UBUNTU] Configure the separate local/cloud tunnel resources, deny unmatched routes, and enable valid automatically managed HTTPS certificates and HTTP-to-HTTPS redirects. Test browser trust, HTTPS WebSockets, renewal configuration, no direct-origin bypass and no need for public SSH/Docker ports.
- [ ] W8.03 [MAC → EDGE/UBUNTU] Bootstrap approved private identities and reconcile Access groups with public admission disabled until ready. Activate only authorized staging testers initially; initial cohort invitations/activation belong to W10. Verify real PIN delivery, group sync, separate admin audience and one-hour/admin versus eight-hour/user session policy.
- [ ] W8.04 [MAC browser → EDGE → UBUNTU] Run cross-user/cross-role, forged/expired token, sibling-origin CSRF/WebSocket and live account-disable tests against real Access. Verify disconnection within 30 seconds despite edge API failure and an infrastructure apply cannot resurrect access.
- [ ] W8.05 [UBUNTU → OpenRouter; I4] Recheck the exact selected route, current price limits/privacy settings and evaluated profile. Run a bounded live tool/review/usage exchange through the actual HTTPS-loopback/socket/broker path; verify returned provider/model identity, upstream-key isolation, cancellation and durable accounting. Record all costs and unknown attempts.
- [ ] W8.06 [MAC/LINUX → UBUNTU] Freeze the candidate profile/tool schema and an appropriate held-out web integration corpus before live evaluation. Reuse historical cases for regression only; keep fresh acceptance distinct. Test representative discovery, ambiguity, resolve, reviewed publication/staging/withdrawal and recovery with real fixtures and verified outcomes.
- [ ] W8.07 [UBUNTU] Measure at least the design's 90% end-to-end task-completion gate and zero detected unauthorized writes/disclosures in the finite test set, with counts/failures/limits. Correlate traces and accounting across the entire real browser/broker path; fake-provider success and the historical 97/100 result do not satisfy this new gate.
- [ ] W8.G [EDGE + UBUNTU] Gate: valid HTTPS/Access, real hosted compatibility, durable usage/capture and negative authorization checks pass. The service remains staged; ten-user capacity and failover are not yet claimed.

### W9 — Ubuntu capacity and cloud recovery acceptance

Output: measured service behavior on the actual Ubuntu machine and selected cloud provider.

- [ ] W9.01 [UBUNTU; approved test window] Run ten genuinely active assisted sessions for 60 minutes with shared-data queries, staggered starts, tool/model bursts and one bounded ingestion job. Use test identities and a finite inference budget; ten idle tabs are not a load test.
- [ ] W9.02 [MAC/Ottawa browsers + UBUNTU] Measure warm-start p95 ≤5 seconds, first-initialization p95 ≤15 seconds and terminal echo p95 ≤250 ms as the design's initial targets. Record queue wait, first-token/full-task latency, memory/CPU/I/O peaks, event coverage, cost and OOM/swap behavior separately. Do not claim a model-latency SLA from terminal latency.
- [ ] W9.03 [LINUX fault fixtures → approved UBUNTU] Exercise controller/gateway/broker/collector/pipeline restart, token expiry, denied policy sync, interrupted import/reset, bounded disk-full and missing release conditions. Repeat converge without modifying unrelated host workloads, account removals, private files or budget balances.
- [ ] W9.04 [UBUNTU; I1 maintenance window] Reboot without an interactive login, with installed artifacts available but registry access unavailable. Verify mount ordering, active-site checks, current permissions and persistent same-host files; new TUI processes have no replayed approvals. Measure the separate five-minute readiness target after OS/storage/network availability.
- [ ] W9.05 [MAC → CLOUD; I5] Price/approve and provision a Canadian-region fallback candidate with equivalent service quotas. Start from the design's 16 GiB/8 vCPU and roughly 320 GiB service-storage candidate plus OS/scratch, then validate actual limits/performance. Do not infer equivalence from vCPU count or rent 2 TB merely because Ubuntu has a 2 TB SSD.
- [ ] W9.06 [CLOUD] From the independent runner, exercise a real cold VM launch using the prebuilt image, recover current startup state and assigned datasets, create fresh user workspaces, fence the previous authority, validate hosted readiness and switch the fixed tunnel targets. The Mac and Ubuntu must not be required to initiate or complete this workflow.
- [ ] W9.07 [CLOUD + UBUNTU controlled failure; approved drill scope] Perform at least three recorded cold-start drills with known data/image/state versions and approved total spend. Each successful drill must reach authenticated assisted-service readiness within **15 minutes after detection**. Record detection delay separately, every stage's duration, slow/failed attempts and actual charges; three samples are not a reliability guarantee or a meaningful p95 claim.
- [ ] W9.08 [CLOUD; Ottawa browsers] Repeat the full ten-user workload, same model/queue and bounded ingestion test. Verify identical roles, grants, datasets and project allowance; check fresh-workspace disclosure. A reduced cohort, missing datasets or permanently disabled ingestion does not pass equivalent service.
- [ ] W9.09 [LINUX simulations + CLOUD/UBUNTU controlled drill] Attempt stale-host reactivation after partition/reboot, paused lease-holder resumption, failed journal sync, stale credentials, a route switch followed by Terraform apply and provider allocation failure. Deny a second writer and report missed timing rather than bypassing security/accounting.
- [ ] W9.10 [CLOUD → UBUNTU] Demonstrate controlled handback with current control/research/dataset changes and spending. Recheck grants before reusing old Ubuntu workspaces; never overwrite them implicitly with cloud-local private files. Disclose cloud workspace loss, preserve independent recovery material, and remove only approved temporary billable resources after verified handback.
- [ ] W9.U [UBUNTU] Gate: actual-host capacity, browser/identity isolation, fault recovery, repeat converge and unattended boot pass with evidence. Record local completion independently if cloud is waiting for approval.
- [ ] W9.C [CLOUD] Gate: real provider cold-start timing, same-capacity service, stale-host fencing and handback pass. If they do not, keep cloud acceptance pending, diagnose and propose a budgeted change; do not silently weaken the 15-minute requirement or create a permanent paid standby.

### W10 — Invited cohort handoff and evidence readiness

Output: a usable invited service with operator/admin instructions, honest acceptance status and a reviewed path to later SLM work.

- [ ] W10.01 [MAC] Complete operator/admin runbooks with the exact tested deployment, migration, rollback, account disable, journal reconciliation, disk/OOM recovery, cloud promotion/handback and emergency-access commands. Include who owns domain renewal, patches, budgets, backup keys and incident response.
- [ ] W10.02 [MAC + UBUNTU; I6] Confirm advertised hours, support contact, applicable retention/research settings and export reviewers. Keep existing-consent records private and omit a new consent/notice gate; show model, budget, recording, reset and fresh-fallback-workspace status in normal UI.
- [ ] W10.03 [MAC → EDGE/UBUNTU; invitation approval] Reconcile and activate the supplied four-admin/three-user cohort without broadening enrollment. Send invitations only when authorized; test the invited user/admin flows from Ottawa networks. No admin workspace is allocated by default.
- [ ] W10.04 [UBUNTU + independent storage] Verify scheduled retention/backup monitoring and alert delivery, a recoverable non-workspace checkpoint, independent credentials and the enabled bounded failover authorization. A backup-success log without a restore test is insufficient.
- [ ] W10.05 [MAC/authorized research environment] Review a small eligible export with provenance, verified outcomes, negative examples, deletion lineage and a held-out split; distinguish synthetic development exports from later real-cohort evidence. Name the Phase 2 owner; do not start training as part of this task.
- [ ] W10.06 [MAC] Finish the acceptance report and execution log, link all required artifacts, list known limitations and unresolved gates, and prepare the requested version-control handoff. Commit/push/PR/merge only under the user's applicable instructions; retain unrelated changes untouched.
- [ ] W10.G [UBUNTU + EDGE + CLOUD] Gate: all required tasks and W9.U/W9.C pass, operator/admin handoff is usable, and cohort activation is authorized. If only Ubuntu is ready, label that partial delivery explicitly; do not describe the whole requested web/cloud service as complete.

## 7. Verification and evidence rules

### Existing runnable checks

For changes to the Rust integration, run from `feather-mesh/` on MAC for iteration and on LINUX for release verification:

```bash
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
cargo test --workspace --all-features
cargo clippy --workspace --all-targets --all-features -- -D warnings
cargo build --release -p mesh_cli --features agent-hosted
```

Run narrower crate tests during development and retain the CLI-only/manual-TUI/hosted feature matrix in [existing CI](../.github/workflows/rust.yml). The release command produces a binary for the build host; Linux deployment requires the Linux-built artifact. Do not let concurrent feature builds replace the binary under a running PTY or evaluation: give each run a pinned artifact/private target directory.

Use the actual fixture-generation, fresh installed-SDK, native Polars/Rasterio, tokenless loopback `pystac-client`, PTY, manual/fake walkthrough and offline-evaluation commands in the [workspace README](../feather-mesh/README.md), [SDK README](../feather-mesh/python_sdk/README.md), [demo runbook](tui_agent_stage1_demo.md), and CI. STAC no longer accepts `--token-file`; verify that non-`127.0.0.1` binds are rejected. Quote package-extra specifications in zsh. Generate/run fixtures only in the intended fixture or disposable project scope; do not overwrite user data. Keep live inference opt-in and separate from ordinary tests.

For documentation/context changes, from the repo root, using a Python environment with the checker's requirements installed:

```bash
python .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.py
python .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py
git diff --check
```

These are structural checks, not proof of deployment or semantic correctness. Review changed requirements and verify local links in both this workplan and the design as well.

### Checks to add as new components arrive

Do not pretend the proposed `web_demo/` or `infra/demo/` commands exist today. Every introducing task must add its real install/build/test commands, dependencies and CI entry before it can be checked complete:

| Surface | Required checks | Execution/evidence |
| --- | --- | --- |
| Web/control services | Unit/schema/migration tests, race/concurrency tests, fake JWT/provider/journal tests, browser role/CSRF/WebSocket tests, redaction tests | MAC + LINUX; deployed repeats on UBUNTU |
| Terraform | Format, validation, provider locks, mocked/static ownership checks, reviewed real plan with safe state/credential handling | MAC/LINUX; real EDGE/CLOUD plans separately approved |
| Ansible/systemd/storage | Syntax/lint, secret-log checks, full-VM first/second converge, unit validation, wrong-mount/identity safety, reboot/restore | LINUX full VM + approved UBUNTU checks |
| Images/artifacts | Native architecture, installed FEAM feature/SDK compatibility, digest/dependency provenance, scoped vulnerability/secret checks | LINUX; run exact release on UBUNTU/CLOUD |
| Pipeline and FEAM readers | Physical format/metadata failures, pinned membership, conversion provenance, fresh SDK install, native queries/windows and real tokenless loopback STAC HTTP with rejected non-loopback binding if exercised | MAC/LINUX + actual UBUNTU identities/mounts |
| Budget/recovery/telemetry | Crash boundaries, unknown costs, journal outage, duplicate replay, lease expiry, deletions, capture gaps and bounded restore state | MAC/LINUX fault tests + independent real backend |
| Deployed service | Actual Access/HTTPS, Ubuntu capacity/reboot, native cloud capacity, cold launch, fencing and handback | EDGE + UBUNTU + CLOUD; observations from Ottawa |

### Map back to the design's acceptance scenarios

| Design scenario | Workplan gates |
| --- | --- |
| 1. Authentication and roles | W2, W8, W10.03 |
| 2. Ownership and origins | W2.04/W2.07, W8.04 |
| 3. Isolation and quotas | W1, W3, W4.09, W5.07, W9.U |
| 4. FEAM and datasets | W4, W8.06 |
| 5. Hosted quality and budgets | W5, W7.04/W7.07, W8.05–W8.07 |
| 6. Usage evidence | W6, W7.05, W10.05 |
| 7. Concurrent load | W9.01/W9.02, W9.08 |
| 8. Failure and recovery | W3, W7, W9.03/W9.04 |
| 9. IaC and boot | W1.07, W7.01–W7.08, W9.03/W9.04 |
| 10. Cloud promotion and handback | W7.06/W7.09, W9.05–W9.10, W9.C |

## 8. Execution record

Update this section in the same change as task checkboxes. Allowed statuses: `not started`, `in progress`, `implemented; verification pending`, `blocked: <specific input>`, `complete`. A phase's verification column must name its pending environment even when its source work is finished. Completion requires linked evidence, not only a commit.

| Phase | Implementation status | Required verification status | Evidence / next action |
| --- | --- | --- | --- |
| Planning | Complete | Documentation checks recorded below; no runtime evidence | Workplan, design and agent/README routing prepared. Begin W0.01. |
| W0 | Not started | MAC/LINUX baseline and UBUNTU preflight pending | Record current tools, interfaces and I1 scope. |
| W1 | Not started | First actual UBUNTU runtime proof pending | Build the one-terminal slice before the full dashboard. |
| W2 | Not started | MAC/LINUX negatives and UBUNTU integration pending | Real EDGE verification belongs to W8. |
| W3 | Not started | Actual UBUNTU lifecycle/limits pending | Ten-user performance belongs to W9. |
| W4 | Not started | Real imports/readers/mount grants pending | I7 source pins required for real seed release. |
| W5 | Not started | Socket/fake-provider/budget proof pending | Real backend W7; live provider W8. |
| W6 | Not started | Actual collector/capture/retention proof pending | Keep synthetic and participant evidence separate. |
| W7 | Not started | Full-VM recovery and real off-host durability pending | Resolve I3 before real dispatch. |
| W8 | Not started | EDGE and live hosted Ubuntu proof pending | Resolve I2/I4 and applicable capture inputs. |
| W9 | Not started | W9.U and W9.C both pending | Resolve maintenance/cloud approval; no 15-minute claim yet. |
| W10 | Not started | Cohort/operator/cloud handoff pending | No invitations or full-service completion claim yet. |

### Session log

Append entries; do not overwrite earlier failures or label old results as a new run.

| Date | Task IDs / environment | Change and evidence | Remaining / next action |
| --- | --- | --- | --- |
| 2026-09-24 | Planning / MAC | Created this agent-ready checklist, environment/dependency map, approval boundaries and acceptance mapping; linked it from the design, README and AGENTS. Documentation verification is recorded in the planning verification entry below. | No web code, host changes, new live inference, domain registration or cloud provisioning. Next: W0.01. |

Planning verification (2026-09-24, MAC): context structural checks passed; all 13 context-checker regression tests passed; local links/anchors passed in this workplan, the design, AGENTS and README; task-structure validation passed for 96 unique unchecked tasks/gates across all 11 phases, with execution labels and no private roster emails; `git diff --check` passed. The context checks used `/private/tmp/feam-context-venv/bin/python`; a fresh environment needs the checker's requirements. Semantic review covered design-to-task mapping, source-of-truth ownership, execution boundaries, approval gates and evidence separation. Rust/application/Ubuntu/cloud runtime checks are not required merely to author this plan and have not been executed for this planning task.

For each subsequent task/attempt, append an evidence record with:

```text
Task ID(s):
Status and date:
Execution environment: MAC / LINUX / UBUNTU / EDGE / CLOUD
Host OS/architecture and test-scope identity (sanitized):
Source revision and artifact/profile/dataset digests:
Files changed:
Prerequisites/approval reference (if applicable):
Commands actually executed and exit results:
Assertions/measurements, sample counts, known/unknown costs:
Sanitized evidence links or restricted-artifact reference:
Failures, unexecuted checks and limitations:
Next ready task / blocking input and owner:
```

### Resume handoff

- Last completed work: planning only; no W0–W10 implementation tasks are checked.
- Next ready task: W0.01 on MAC, followed by interface/toolchain work and authorized read-only UBUNTU preflight.
- Private input reminder: the ignored roster and future inventories/keys do not travel through Git; obtain them through the operator's approved private channel.
- Final completion condition: all required task and environment gates pass, including actual Ubuntu service evidence and real 15-minute cloud cold-start acceptance. Otherwise report the precise partial delivery and continue safe independent work.
