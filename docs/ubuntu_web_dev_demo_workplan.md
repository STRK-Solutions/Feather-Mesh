# Web-accessible FEAM demo development workplan

Status: **Deliver a functional Ubuntu web demo first.** The owner has deferred lengthy acceptance exercises and shelved cloud VMs and R2. Cloudflare browser access and OpenRouter remain in scope. Follow the [Ubuntu functional milestone](#ubuntu-functional-milestone), which now defines delivery; completing every W2–W10 phase gate is no longer a prerequisite.

W0/W1 are complete; W2–W8 have substantial implementation and local evidence. Functional completion continues on `feat/complete-ubuntu-web-demo`. Follow the [current takeover record](evaluations/web-demo/ubuntu-functional-progress.md); the [review checkpoint](evaluations/web-demo/wrap-up-checkpoint.md) retains earlier attempts. U.01/U.02/U.05 pass: prerequisites and image smoke, the exact promoted climate release with real user A read/user B denial, and owner-approved [public activation](evaluations/web-demo/u05-public-activation-proposal.md). Four exact Cloudflare routes, the protected Ubuntu connector and six private services passed host checks. All three approved participants completed real email-PIN sign-in, dashboards and authenticated terminal/WebSocket checks; 25 of 25 edge probes passed. The administrator disabled the second regular staging alias through the browser; its current WebSocket closed in 0.506 seconds and old JWTs were denied. The alias remains disabled, leaving one active admin and one active regular staging user. [U.05 browser evidence](evaluations/web-demo/u05-browser-acceptance.json) retains the redirect, identity-provider, form and connector repairs. U.03/U.04/U.06/U.07/U.G remain open for fake capture/archive, lifecycle stop/start, bounded live inference, verified Mac copy and handoff. Existing work remains uncommitted. See the [implementation record](evaluations/web-demo/w2-w7-progress.md) and [acceptance index](ubuntu_web_dev_demo_acceptance.md).

Updated: 2026-09-25.

Source of truth: [Ubuntu web demo design](ubuntu_web_dev_demo_design.md), with the owner's Ubuntu-only functional scope recorded here and in the design. This milestone supersedes earlier requirements to finish cloud, clean-VM recreation, extended model evaluation, capacity or reboot acceptance before delivery. Keep architectural decisions in the design and delivery status here.

## 1. Instructions for the implementing agent

1. Read [AGENTS.md](../AGENTS.md), the design, this plan, and the latest [execution record](#8-execution-record). Inspect the checkout and preserve unrelated edits. Create or resume the task's feature branch before implementation; do not make untracked host edits the source of truth.
2. Keep the **Mac as the primary development workstation**. Reuse the implemented services, existing Ubuntu pool, native build path and W0/W1 evidence. Run bounded integration directly in the reviewed FEAM scope on Ubuntu. A new disposable-VM deployment is deferred; do not make it a prerequisite to the functional demo.
3. Apply the [context-maintenance skill](../.codex/skills/feam-agent-context-maintainer/SKILL.md) when components, commands, CI or routing change; the [peer-data-access skill](../.codex/skills/feam-peer-data-access/SKILL.md) for publication/resolution/import/SDK/STAC work; the [Rust skill](../.codex/skills/feam-rust-workflow/SKILL.md) for Rust changes; and the [CLI skill](../.codex/skills/feam-cli-contract/SKILL.md) for CLI/protocol changes. Read required sources before acting.
4. Use stable task IDs below. Change `[ ]` to `[x]` only when the task's artifact and stated verification exist. Do not renumber IDs; append new tasks when needed. Record a partial task as in progress rather than checking it off.
5. After each completed task or failed gate, update its evidence entry, the phase status, and the next-action field. Evidence names the commit/artifact digest, execution host, commands, result, limitations and relevant approval reference. Retain failures and superseded attempts.
6. The active delivery gate is U.G. Retain W0–W10 IDs and evidence as the fuller backlog; their unfinished gates do not block U.G unless the specific check is included in U.01–U.07. Deferred tests stay unchecked. Report a functional Ubuntu demo without claiming ten-user capacity, reboot resilience, benchmark quality or cloud readiness.
7. Fix failures on the active path and run short checks relevant to changed code. Reuse passing evidence for unchanged code; avoid repeated full matrices, soak tests and broad failure campaigns. If an external input blocks one task, continue independent active work. Authentication, ownership, spend limits and honest capture status still apply.
8. Before ending an implementation session, leave a concise handoff: completed IDs, changed files, executed/unexecuted checks, pending authorizations and the next ready task. Keep private identities, credentials and raw user traces out of public reports.

The owner has enabled the dedicated `feam-deploy` SSH account with passwordless sudo; reviewed, authorized Ubuntu provisioning needs no repeat owner-terminal handoff. This account has full host-root capability, so keep operations within the reviewed FEAM scope. Prepare exact dataset, public-route and finite model configurations before any still-needed approval; reuse existing authorizations. This planning revision does not itself authorize public activation, invitations or billable inference. Cloud purchases/provisioning and shared-host reboot tests are shelved; do not request those inputs for this milestone.

## 2. Where work happens

An environment label identifies **where the operation executes**, not where the agent's editor is open. A command launched from the Mac over SSH that modifies Ubuntu is an Ubuntu operation.

| Label | Environment | Work performed there | Does not establish |
| --- | --- | --- | --- |
| MAC | This macOS ARM64 development checkout | Source/docs/IaC authoring, application unit tests, browser tests against local fake services, synthetic import fixtures, review and operator orchestration. | Ubuntu security, disk/mount behavior, unattended boot, or deployed capacity. |
| LINUX | Native Ubuntu 24.04 x86-64 build environment | Current native artifacts and focused checks; the existing bounded build scope on Ubuntu is available. Full-VM recreation and destructive fault exercises are deferred. | Actual deployed application behavior or capacity merely because compilation passes. |
| UBUNTU | The actual Ubuntu demo machine | Reviewed FEAM provisioning, participant services, bounded browser/data/model checks and ordinary service stop/start. | Ten-user load, unattended reboot or HPC acceptance without those separate tests. |
| EDGE | Project-owned registrar/Cloudflare services, controlled from MAC or an approved runner | Approved domain/DNS/HTTPS/Tunnel/Access setup and actual identity/route tests. | Application correctness merely because the tunnel connects. |
| CLOUD | Shelved | No provider selection, account work, cloud images, VM/volume/IP creation, cloud tunnels, deployment or acceptance work. Preserve existing recipes and evidence. | No active delivery dependency. R2 archive work is also shelved. |

Use MAC for day-to-day coding and native Linux for release artifacts; record dependencies and digests. Reuse an existing binary only when its inputs still match. The prepared Go snapshot predates collector carryover and the planned local archive mode, so it needs rebuilding. Full-system VM, reboot and destructive mount tests remain available for later hardening.

### Mac-to-Ubuntu development loop

1. Edit and run narrow tests on MAC; keep fake authentication, inference and budget backends confined to explicit test configurations.
2. Build/test the exact revision on LINUX. Retain the FEAM binary, host-service artifacts, container image and dependency manifest by digest.
3. From MAC, invoke approved Ansible against a verified private SSH inventory using the [delegated deployment access](../infra/demo/README.md#unattended-ubuntu-deployment-access). Keep strict host checking and use noninteractive sudo for authorized privileged steps. Install artifacts on UBUNTU into the dedicated service/staging scope, not the personal home or existing Docker daemon.
4. Exercise the Ubuntu-hosted application from a browser on MAC through a private test connection initially, then through real EDGE authentication in U.05. Record the bounded U.04–U.07 results on Ubuntu.
5. Fix source/configuration on MAC, rebuild, and redeploy the new pinned release. Capture any emergency host repair in IaC before marking the task complete.

Before the machine serves participants, bounded native builds may use a separate Ubuntu build account/path if no Linux runner exists and the operator approves the resource use. Once demos run, move heavy builds, destructive tests and SLM training elsewhere. Keep staging data, service identities, sockets and ports separate from participant resources; do not expose Docker's API publicly.

## 3. Scope and settled decisions

- [x] Design exists, with Terraform for external resources, Ansible for host configuration, and systemd/application recovery rather than infrastructure applies at every reboot.
- [x] Four admins and five regular users are specified privately; enrollment stays invited. Admins have management access, not automatically allocated workspaces. Target capacity remains ten regular users.
- [x] Email PIN login, separate application audiences/role permissions, 30-minute idle stopping and 30-day inactive-workspace retention are accepted. Retention deletion needs advance warning; reset needs confirmation.
- [x] Participant consent already exists; no new participant-notice or repeat-consent gate is required. Recording status, withdrawal/deletion handling and restricted research exports remain required.
- [x] Initial data will be small Canadian government climate products: physically valid GeoTIFF `.tiff` rasters and Parquet-only published tables. AAFC/ECCC candidates and the proposed 250 MiB seed-bundle limit are in the design.
- [x] The selected model is `deepseek/deepseek-v4.1-flash` through OpenRouter, pinned to `deepinfra/fp8`, with provider fallback and automatic retries disabled. Reuse the evaluated profile, subject to current route/price validation.
- [x] The project allowance is US$100 total, including known spend and outstanding/unknown reservations. New requests pause until an admin raises it. It does not reset by user, month, key rotation or site change.
- [x] Ubuntu is the only active demo host. Cloud VMs, provider evaluation, cloud storage/R2 integration and cloud deployment acceptance are shelved. Keep Cloudflare Access/Tunnel for browser access and OpenRouter for hosted inference. Keep private operator inputs and project spending on the Mac.
- [x] MAC is the main development workstation; UBUNTU is an early and continuing integration/deployment target.
- [x] Owner switched the demo back to `613202690.xyz` on 2026-09-25 and confirmed Cloudflare Active status. Public NS queries through Cloudflare and Google return the assigned nameservers. Use `https://feam.613202690.xyz` with the sibling routes below; demo DNS routes, HTTPS, Tunnel and Access remain unverified.
- [x] Owner connected Cloudflare to Codex on 2026-09-25; Cloudflare tools are available in the current session. Use the existing connection first; verify account/zone permissions and automation authentication before EDGE operations. See the [connection record](#cloudflare-connected-to-codex-2026-09-25).
- [x] Research traces and reviewed exports survive ordinary shutdown in protected Ubuntu storage. Before destructive teardown, copy the retained corpus and deletion/provenance ledger to private Mac storage and verify hashes. Keep 30-day retention and restricted review. R2 is shelved; local archive support is an implementation task, not an existing deployed capability.

These checked items record decisions, **not implemented capabilities**. The [Stage-1 acceptance record](tui_agent_stage1_acceptance.md#final-fresh-acceptance) contains historical 97/100 held-out model evidence. U.G requires a small functional live smoke; the new held-out benchmark and deployed capacity/reboot measurements are deferred.

Reuse the current Rust workspace, [peer contract](data_access_contract.md), [SDK](../feather-mesh/python_sdk/README.md), [Stage-1 contract](tui_agent_stage1_contract.md), [demo runbook](tui_agent_stage1_demo.md), and [existing CI](../.github/workflows/rust.yml). The old proposed baseline in the [peer implementation plan](../data_access_implementation_workplan.md) is historical; its execution record and current source establish what is already implemented. Do not rebuild peer discovery, STAC or the harness as a new web-specific authority.

Out of scope for current delivery: all cloud-host and R2 work, ten-user soak/latency acceptance, shared-host reboot, clean-VM recreation, destructive fault campaigns, a new held-out model benchmark and a research-ready export corpus. Also excluded: public enrollment, Kubernetes, VM-per-user isolation, unrestricted sandbox internet, public STAC/download service, admin development workspaces, cross-host replication/recovery and SLM training. Preserve implemented capabilities; defer additional work on these paths.

### Demo hostname plan

Use the `613202690.xyz` Cloudflare zone following the owner's explicit switch back on 2026-09-25. The owner confirms Active status and public DNS returns the assigned nameservers. This supersedes the temporary `demo.saifshaikh.ca` selection; U.05 supplies the required live browser-access proof.

| Purpose | Planned hostname |
| --- | --- |
| User portal | `feam.613202690.xyz` |
| Admin pages/APIs | `admin.613202690.xyz` |
| Individual workspace | `u-<opaque-id>.613202690.xyz` |

Keep all three as first-level siblings in `613202690.xyz`, preserving separate origins and host-specific Access audiences. This fits Cloudflare Universal SSL's first-level coverage in the full DNS setup; nested names such as `admin.feam.613202690.xyz` require additional certificate coverage and are not part of this plan. Verify actual certificates in U.05. [Cloudflare certificate coverage](https://developers.cloudflare.com/ssl/edge-certificates/universal-ssl/limitations/)

Manage only the demo's explicit DNS records, Tunnel routes and Access applications in `613202690.xyz`. Preserve the registration, zone, nameservers and unrelated records/policies during setup and teardown. Leave the owner's `saifshaikh.ca` zone untouched. Domain activation is recorded separately from W8 application readiness.

## 4. Inputs, approvals and proposed artifacts

### Inputs that must be resolved without blocking unrelated development

| Input ID | Needed input or approval | Needed before | Work that can continue meanwhile |
| --- | --- | --- | --- |
| I1 | W0/W1 and the expanded storage pool have Ubuntu evidence. Dedicated `feam-deploy` SSH and noninteractive root sudo are verified; reuse this access and recheck the FEAM resource scope before mutation. | U.01–U.04; no reboot window is needed for current delivery. | Local changes and private Ubuntu integration. |
| I2 | Hostname selection is settled: `feam.613202690.xyz`, with the sibling routes above. Owner confirmed Cloudflare Active status, Zero Trust onboarding and connection to Codex. The exact four-route public activation was separately authorized and applied; three real email-PIN dashboard logins pass. Preserve the existing account/zone, unrelated resources and MFA. Capture renewal terms for handoff; no repeat domain purchase, nameserver change, onboarding or connection setup is required. | Public route authorization and configuration are supplied; U.05 authenticated WebSocket, role and revocation checks remain. | Continue the deployed functional browser checks and handoff. See [activation record](evaluations/web-demo/u05-public-activation-proposal.md) and [hostname plan](#demo-hostname-plan). |
| I3 | Encrypted operator vault and US$100 project budget exist on the Mac with confirmed zero prior charges. No run allocation exists. Keep the project keys on the Mac; issue only the bounded signed allocation and public verification key to Ubuntu. | U.04/U.06; the project ledger does not depend on R2. | Fake broker integration and local archive implementation. |
| I4 | Owner supplied the OpenRouter key privately and approved the exact five-task US$1 functional smoke, with a US$0.03 per-request ceiling and expiry. Route and current price checks are recorded in the [smoke proposal](evaluations/web-demo/u06-functional-smoke-proposal.md). No live allocation or request has yet been issued. | U.06 live smoke; the US$100 project ceiling alone is not the test allocation. | Complete fake/browser flow and private allocation checks before the bounded live requests. |
| I5 | **Shelved: all cloud-host work.** Preserve the [prior NYC3 proposal](evaluations/web-demo/cloud-archive-proposal.md), recipes and evidence. No resources were created. Do not request another provider, account, size or budget. | No current milestone dependency. | Ubuntu delivery only. |
| I6 | 30-day research retention, Saif as sole reviewer/Phase 2 owner and the support contact are settled. **R2 is shelved**, including credential transfer, live collector checks and storage Terraform. Preserve the existing bucket/credentials without new operations. U.03 adds protected Ubuntu archival and verified Mac copying before destructive teardown; ordinary stop retains local data. | U.03 capture and U.07 operator handoff. | Ubuntu collector/config/lifecycle changes; no cloud archive input is needed. |
| I7 | The [exact ECCC/AAFC objects](evaluations/web-demo/w4-source-proposal.md) were approved, acquired and verified. The exact immutable release was approved and [promoted on Ubuntu](evaluations/web-demo/u02-ubuntu-release.json); source/output hashes and real readers pass. Per-user browser grants and cross-user denial remain U.04 proof. | U.02 release is promoted; U.04 grant acceptance remains. | Continue the authorized browser/data integration. |

The private roster is at `.local/demo-deployment/participants.yaml` on the Mac and is already retained in the encrypted operator vault. Use current private policy when bootstrapping; a fresh clone has no private inputs. Keep identities, keys and raw traces out of Git, images and public evidence. No new cloud credential is required for this milestone.

### Proposed source layout

W0 provides `web_demo/{schemas,tests,validate.py,README.md}`, `infra/demo/{examples,scripts,tests,toolchain.json,toolchain.md,requirements-dev.lock,README.md}`, the read-only Ansible preflight/example inventory, and the acceptance/evidence index. W1 adds `web_demo/cmd/private-terminal`, `web_demo/internal/terminal`, scoped Ansible roles with `host.yml`/`deploy-w1.yml`, the terminal image and runtime probes. Participant services, in-package versioned migrations and Terraform modules are now implemented with local tests. Production lifecycle integration and named environment gates remain in progress. W0 selects Go host services/orchestration and Python climate conversion in the [implementation contract](ubuntu_web_dev_demo_contract.md).

```text
web_demo/                             new web/control services, separate from FEAM authority
  README.md                           MAC development and LINUX integration commands
  cmd/                                gateway, controller, broker, collector, pipeline entrypoints
  internal/                           auth, control DB, typed IPC, lifecycle, budgets, capture
  migrations/                         versioned control/ledger/index migrations
  templates/                          server-rendered portal/admin pages
  importers/                          bounded source conversion and provenance
  tests/                              browser, fake-provider, isolation and crash scenarios
infra/demo/                           layout follows the design
  README.md                           operator commands and approval boundaries
  terraform/{edge,research-storage,cloud-host}/
  ansible/{inventories,roles}/
  ansible/{preflight,host,deploy,lifecycle}.yml
  images/                             FEAM terminal and optional cloud image recipes
  tests/                              full Ubuntu VM provision/reboot/start/stop/teardown harness
docs/evaluations/web-demo/             sanitized task evidence, created as checks run
docs/ubuntu_web_dev_demo_acceptance.md  measured results and unresolved gates
```

W0 selects Go/server-rendered host services and Python climate conversion in the implementation contract. Separate processes, OS identities and typed private IPC preserve authority even when code shares a module. Keep FEAM peer business rules in `mesh_core`, terminal/review state in `mesh_tui`, and provider adaptation in `mesh_agent`. The gateway must never inherit Docker, host-root, Terraform, dataset-publication or upstream-model-key authority.

## 5. Delivery order

### Ubuntu functional milestone

**Done means:** an invited user opens the Ubuntu demo in a browser, logs in through Cloudflare Access, starts their workspace, reads the approved climate data and completes a hosted FEAM interaction. An admin can manage accounts/workspaces without receiving a terminal. Spending and redacted events are recorded; the operator can stop/start the demo and preserve research records. U.G closes this delivery independently of the extended W gates.

Implement only gaps that prevent this flow. Reuse the existing code and host resources, freeze one current release, and avoid rebuilding unchanged artifacts. U.01 dependency repair, U.02 candidate preparation, U.03 local archive support and preparation of U.05 edge inputs can proceed independently; complete U.03 before the final Go build/service configuration in U.04. Exercise fake inference before the brief live smoke.

- [x] U.01 [MAC → UBUNTU] **Repair prerequisites and finish the image.** Python 3.12 venv repair and pinned pipeline imports pass; the incomplete attempt is retained. Image `sha256:1c11e6a030467522763a5f86b58283dc83d666440fc5d37e0d253621814fa60e` passes installed-SDK/Polars/Rasterio and synthetic hosted-startup smoke. Current Rust/model-adapter build inputs match the reused native artifacts by content. W1 slots/containers are preserved. [Exact evidence and limitations](evaluations/web-demo/u01-ubuntu-prerequisites.json). No clean-VM recreation or live inference is claimed.
- [x] U.02 [UBUNTU; I7] **Publish the initial dataset pair.** The exact ECCC/AAFC candidate was reviewed and promoted once as immutable Parquet/GeoTIFF release `9894f3498b7d0828d3358b6dd415f6ee0ba298442bca7f615987af147a2aef41`; source/output hashes, provenance, native readers and loopback STAC passed. Real browser user A received only this read-only release and completed the installed SDK's 365-row table and known Rasterio window checks. User B had no mount and peer resolution was denied. The first browser raster call used a guessed asset ID and failed; the corrected ID came from the actual promoted manifest. [Release evidence](evaluations/web-demo/u02-ubuntu-release.json), [browser grant/readback](evaluations/web-demo/u02-browser-grant.json). U.04 owns subsequent workspace lifecycle and capture checks.
- [ ] U.03 [MAC → UBUNTU] **Remove the runtime R2 dependency.** Add an explicit Ubuntu local archive mode using the existing directory adapter, with a fixed private, bounded path on the verified traces filesystem. Update collector startup, settings validation, service configuration and lifecycle checks together; production currently requires R2, so a config-only change is insufficient. Preserve redaction, event correlation, quotas, 30-day retention, deletion lineage and reviewer restrictions. Show local capture/archive status accurately. Ordinary stop flushes and retains records. Add an operator command to copy a quiesced archive plus provenance/deletion ledger to owner-only Mac storage and verify its inventory/hashes; destructive teardown stays blocked until that independent copy is verified. Apply withdrawals/expiry to retained copies. Test one synthetic capture/flush/readback and denied reviewer access. Do not enable a fake production archive or require continuous Mac connectivity for startup; host-loss protection begins only after a verified copy.
- [ ] U.04 [MAC → UBUNTU] **Deploy one current private release.** Freeze source/configuration digests after U.03, rebuild current native Go services, and install the image/services under their intended identities using the existing pool and Ansible recipes. Initialize current roles/policy and one bounded fake run; check real service sockets with two workspaces and an admin. Prove start/stop/reconnect, one confirmed reset, a read-only shared mount, cross-user/role denial, fake tool/review/cancel flow and correlated capture. Perform one ordinary service stop/start and repeat converge, confirming files, grants and spend reservations survive. Repair encountered integration failures; defer broad crash/fault matrices and host reboot.
- [x] U.05 [MAC → EDGE + UBUNTU; I2] **Expose only the Ubuntu route.** The owner-approved exact four DNS/Tunnel/Access routes and connector were applied without cloud-host or direct-origin exposure. Three approved participants completed real email-PIN login; dashboards and both owner terminal WebSockets worked. All 25 authenticated/negative HTTP and WebSocket probes passed, including wrong-user/role/audience and Origin denial. Admin browser disable closed the second regular alias’s current WebSocket in 0.506 seconds (<30 seconds), and its old portal/workspace JWTs returned 403; that alias remains disabled. Initial redirect, wrong-IdP, native form-Origin and connector HTTP/2 Unix-WebSocket failures and their scoped repairs are retained in [activation](evaluations/web-demo/u05-public-activation-proposal.md) and [browser acceptance](evaluations/web-demo/u05-browser-acceptance.json). Explicit expired-assertion testing remains an extended check; invitations are separate.
- [ ] U.06 [UBUNTU → OpenRouter; I3/I4] **Run a small live functional smoke.** Supply the reviewed key/finite allocation after private checks pass. Use 3–5 representative browser-driven requests covering discovery, exact resolve/read and a reviewed action; include a denial or cancellation. Verify the selected provider/model, actual result, recorded usage/reservations and redacted trace correlation. Fix failures of these chosen flows. Do not create a new held-out corpus, measure a 90% benchmark or run prolonged live traffic for this milestone.
- [ ] U.07 [MAC browser + UBUNTU] **Finish a short usable-demo check and handoff.** Target a 10–15 minute walkthrough with two active user sessions and an admin; reuse U.04–U.06 evidence instead of repeating it. Confirm basic dashboard actions, terminal reconnect, dataset access, model/budget/recording status and ordinary stop/start. Verify one small archive copy to the Mac without destroying Ubuntu state. Record exact URL, release/image/config digests, start/stop/log/account-disable commands, owner/support and measured limits. Keep the supplied cohort configured with activation/invitations subject to existing authorization. Update acceptance and prepare a scoped Git handoff excluding `.DS_Store`, `.ansible/.lock` and private data; commit/push only under applicable user instructions.
- [ ] U.G [UBUNTU + EDGE] **Functional Ubuntu demo accepted.** U.01–U.07 pass for the deployed release and the operator has usable access/instructions. A participant completes the real browser/data/hosted flow, isolation negatives pass, spend/capture persist and retained research has a verified private Mac copy. Report the observed two-user smoke honestly; ten-user capacity, unattended reboot, full recovery, research-corpus quality and all cloud/R2 acceptance remain deferred. Their unfinished W gates do not prevent U.G completion.

### Deferred work and shelved scope

| Work | Status for this delivery | Retained task references |
| --- | --- | --- |
| Cloud provider/account/size research, VM/volume/IP provisioning, cloud images, cloud tunnels, host switching and cloud teardown/billing | **Shelved.** No further implementation, credentials, plans/applies or tests until explicitly resumed. | Cloud portions of W7.01/W7.04–W7.09/W8.02; W9.05–W9.10/W9.C; cloud portions of W10 |
| R2 credential transfer, collector S3 integration, cloud archive retention/deletion/survival and research-storage Terraform | **Shelved.** Preserve the existing bucket, private credentials and evidence. U.03 supplies the local path; no R2 gate blocks startup. | R2 portions of W6.05/W6.G/W7.05/W10.04 |
| Ten active users for 60 minutes and p95 performance/capacity targets | **Deferred.** Keep configured resource limits; claim only the concurrency actually observed in U.07. | W9.01/W9.02/W9.U |
| Clean-VM recreation, migration/rollback matrices, reboot, disk-full, corrupt-state and destructive fault campaigns | **Deferred.** One normal service stop/start and repeat converge remain in U.04. | Expanded W3.07/W4.05/W7.07/W7.08/W9.03/W9.04/W9.09 |
| Fresh held-out browser/model corpus and 90% success benchmark | **Deferred.** U.06 provides functional compatibility evidence only. | W8.06/W8.07 |
| Destructive archive-survival/recreation drills, research-ready exports and Phase 2 corpus preparation | **Deferred.** Keep capture/redaction and a verified private Mac copy; do not erase the only retained copy. | Extended W6.05/W6.07/W7.05/W10.04/W10.05 |
| All W2–W10 phase gates as one comprehensive release requirement | **Deferred.** U.G is sufficient for the requested functional Ubuntu delivery. Check a W task only when its full original evidence exists. | W2.G–W10.G, including W9.U/W9.C |

## 6. Checkable implementation tasks

The W checklist below retains the original task IDs and measured completion. It is the **extended backlog**, not a sequential prerequisite for U.G. Active work is the bounded subset in U.01–U.07 above; unlisted refinements and exhaustive verification wait until the functional demo is delivered. Completed checkboxes remain evidence of their original scope; shelving or deferring a task never checks it off.

### W0 — Baseline and execution setup

Output: reproducible development setup, interface decisions, safe target inventory and first-run test record.

- [x] W0.01 [MAC] Record branch/revision, existing dirty changes, relevant implemented commands and current test status. Create the acceptance/evidence index with environment labels; preserve existing Stage-1 evidence unchanged. See [acceptance index](ubuntu_web_dev_demo_acceptance.md) and [W0 baseline](evaluations/web-demo/w0-baseline.md).
- [x] W0.02 [MAC] Inspect/install only required development tools in isolated environments. Record Rust/Python and selected Go/IaC/browser-test versions, lockfiles and native dependency strategy; do not treat an existing Mac environment as a verified Linux toolchain. [Toolchain and explicit deferred tools](../infra/demo/toolchain.md); exact Python dependency lock and verified Go archive.
- [x] W0.03 [MAC] Define module/process ownership, local control/ledger/event schemas, typed Unix-socket protocols, request/generation IDs, and migration/bootstrap/restore boundaries. Specify that duplicate or uncertain mutations reconcile rather than retry blindly. [Implementation contract](ubuntu_web_dev_demo_contract.md), closed schemas/examples and negative tests; runtime enforcement belongs to later phases.
- [x] W0.04 [MAC] Define operator deployment records, persisted desired running/stopped state and project-to-run budget allocations. The sum of settled spend and outstanding run reservations must fit the project allowance; unknown prior runs retain their allocation. Specify one public deployment, safe switch/revocation and fresh-seed initialization without distributed recovery services. [Lifecycle/budget contract](ubuntu_web_dev_demo_contract.md#operator-deployment-and-budget-records); offline reservation/reconciliation validation passes.
- [x] W0.05 [LINUX; approved UBUNTU native-build path] Establish a native Ubuntu x86-64 build path and a disposable full-system test VM. Prove access to systemd/cgroup v2 and scoped loop mounts. Record what ordinary CI can and cannot test. Owner requested VM setup on Ubuntu; the bounded TCG guest passed the systemd/cgroup/64 MiB loop/remount probe and is now stopped. Native hosted-capable FEAM release built separately on physical Ubuntu; [artifact digests](evaluations/web-demo/w0-linux-artifacts.json) and [evidence](evaluations/web-demo/w0-baseline.md#owner-authorized-disposable-vm-and-native-build).
- [x] W0.06 [UBUNTU; I1] Run the design's read-only preflight: current OS/kernel, cgroups/AppArmor, existing Docker workloads/packages, SSD filesystem mapping, free bytes/inodes, identity/subordinate-ID collisions, time sync and private management reachability. Do not install or reconfigure during preflight. Unprivileged checks and the actual Ansible entrypoint pass (`changed=0`); exact filesystem identity/scope is recorded privately. [Reviewed owner-run privileged evidence](evaluations/web-demo/w0-privileged-preflight.json) reports no Docker container rows or errors, `/var/lib/docker` with `overlayfs`/cgroup v2, 21 Snap-only loop devices and loaded AppArmor. Earlier permission-denied attempts remain recorded separately; future runner enforcement belongs to W1.
- [x] W0.07 [MAC] Create sanitized inventory/config examples and a typed settings schema. Track I1–I7 without placing real identities/secrets in Git; retain the initial private roster and existing-consent confirmation separately. [Operator examples and input tracking](../infra/demo/README.md); original ignored private roster retained.
- [x] W0.08 [MAC/LINUX] Add executable development/test commands and CI jobs as components appear. Keep public PR checks offline and without production credentials; separate authorized live/deployment workflows. [Offline check command](../web_demo/README.md) passes on MAC and native Ubuntu; [native Ubuntu CI jobs](../.github/workflows/web-demo.yml) added, remote CI execution still pending. No production inventory or live workflow is invoked by PR checks.
- [x] W0.G [MAC + LINUX + UBUNTU] Gate: interfaces and toolchain are recorded, baseline failures are accounted for, a full-system test target exists, and actual-host preflight establishes a safe provisioning scope. Passed after [privileged preflight review](evaluations/web-demo/w0-baseline.md#owner-run-privileged-preflight-and-w0-closeout): future service resources stay under the verified `/home/feam-service-data` scope with dedicated identities/runtime; preserve the existing Docker daemon, Snap loops and host namespace protections. Recheck ownership/collisions immediately before W1 mutation. At W0 closeout, W1 host provisioning had not begun; completed W1 evidence is below.

### W1 — First isolated browser terminal on Ubuntu

Output: an approved, private one-user runtime proof, before developing the full portal. Manual/fake assistance is sufficient for this plumbing milestone, not for final demo acceptance.

- [x] W1.01 [MAC → LINUX] Build the FEAM terminal image with the hosted feature, `ttyd`, supported SDK/readers, pinned fixtures and final-path demo initialization. Install `mesh_cli` as `feam`; preserve project paths with spaces and provider symlinks. Retain native Linux image/binary digests.
- [x] W1.02 [MAC] Write idempotent Ansible accounts/runtime/storage roles. Use a dedicated rootless runner, non-overlapping subordinate IDs, private daemon socket/data root, supported host RootlessKit policy, a systemd user service with lingering and measured cgroup delegation. Preserve the existing host Docker daemon/workloads.
- [x] W1.03 [LINUX] Prove a non-root, read-only-image container with dropped capabilities, supported seccomp, no-new-privileges, `--network none`, bounded memory/CPU/PIDs/tmpfs and no extra swap allowance. Do not claim unsupported per-container AppArmor confinement in rootless mode.
- [x] W1.04 [LINUX] Create only disposable, ownership-recorded ext4 backing files and mount units. Test Unix-socket ACLs/UID mappings, bounded socket tmpfs, expected filesystem identities and system/user-manager ordering. Missing or wrong mounts must fail without writing into underlying directories.
- [x] W1.05 [MAC/LINUX] Add a minimal private browser-to-terminal proxy with fixed launcher arguments, WebSocket origin checks, header filtering and two connections per workspace. Any test identity mechanism must be unavailable in deployable production configuration; do not open an unauthenticated public terminal.
- [x] W1.06 [UBUNTU; I1] Apply the tested scoped roles and pinned artifact to a private test slot. From MAC, reach it over the approved private connection and execute the FEAM manual/fake walkthrough. Capture actual CPU/memory/PID enforcement, socket denial across identities, private-volume persistence and confined reset. [Actual-host acceptance](evaluations/web-demo/w1-ubuntu-acceptance.json), [Chromium manual/fake proof](evaluations/web-demo/w1-ubuntu-browser.json) and [enforcement](evaluations/web-demo/w1-ubuntu-enforcement.json) pass.
- [x] W1.07 [LINUX, then approved UBUNTU] Repeat converge and verify no reformat/reinitialization, no namespace-policy weakening and no impact on unrelated services. Test runner start without interactive login; defer actual shared-host reboot to an approved window. Final Ubuntu converge: `ok=34 changed=0 failed=0`; `Linger=yes`, no login sessions. Original Docker PID/start timestamps and namespace/AppArmor policy remain unchanged. [Attempts and recovery evidence](evaluations/web-demo/w1-progress.md) retained.
- [x] W1.G [UBUNTU] Gate: a real Ubuntu-hosted browser terminal works with correct isolation and persistent local files; exact provisioning/build/smoke commands and evidence are recorded. Passed with actual Chromium manual/fake exchanges, full constrained-container walkthroughs, preserved files and confined reset. [Operator commands](../infra/demo/W1.md). Public identity, hosted inference, capacity/cloud and shared-host reboot remain later gates.

### W2 — Identity, control data and authorization gateway

Output: tested roles/ownership and a portal that is ready for real Access integration in W8.

- [x] W2.01 [MAC] Implement explicit control-DB initialization and migrations for accounts, site-local workspaces, jobs, grants and audit records. Add unique identities/idempotency constraints and immutable local IDs; do not use emails as paths or FEAM's legacy SQLite as web control state.
- [x] W2.02 [MAC] Implement idempotent private bootstrap for four admins and five users. Subsequent converge must not resurrect deleted accounts or revert roles. New accounts stay pending until the edge membership and required workspace assignment are ready; admins receive no workspace.
- [ ] W2.03 [MAC/LINUX] Validate JWT signature, issuer, exact host-specific audience, expiry and active local identity. Use maintained validation libraries and bounded key refresh; reject forged headers, wrong audiences, stale/unknown identities and malformed tokens. Test key rotation/offline-key behavior explicitly.
- [ ] W2.04 [MAC/LINUX] Enforce separate portal/admin/workspace origins, exact hostname routing, host-only secure cookies, CSRF protection, WebSocket Origin checks and credential-header stripping before sandbox proxying. Bind every workspace hostname to an authorized server-side record.
- [x] W2.05 [MAC] Implement exact-email group reconciliation and pending/error reporting with a fake provider first. Terraform owns referencing applications/static policies, not mutable membership groups; test empty groups fail closed and a later apply cannot restore removed membership.
- [ ] W2.06 [MAC/LINUX] Add local account authorization versions and active-stream tracking. Disabling an account rejects new requests immediately, closes streams within 30 seconds, stops workspaces and revokes model/event capabilities even when edge sync fails.
- [ ] W2.07 [UBUNTU] Run the gateway and control store under their intended identities with synthetic test accounts on the private path. Verify cross-user/cross-role HTTP and WebSocket denial using two real containers, including sandbox-controlled response headers/cookies.
- [ ] W2.G [MAC/LINUX + UBUNTU] Gate: migrations, ownership, revocation and private integration pass with automated negative tests. Real email delivery/Access JWTs remain explicitly pending W8; test assertions are not production authentication.

### W3 — Workspace lifecycle and management dashboard

Output: users operate their own sandbox; admins manage the service through bounded actions without shell/editor access.

- [x] W3.01 [MAC] Implement controller-only access to the dedicated runtime. Accept workspace IDs and fixed actions/templates, never client-supplied host paths, mount specs, images or commands. Authenticate typed private IPC and separate gateway/controller permissions.
- [ ] W3.02 [MAC/LINUX] Implement transactional capacity reservation, per-workspace serialization, expected-generation checks, recorded container labels and recoverable lifecycle states. Test concurrent start/reset, lost responses, orphan reconciliation and partial initialization without duplicate allocations.
- [ ] W3.03 [MAC → UBUNTU] Expand the verified pool to ten 2 GiB workspaces plus two 2 GiB reset spares, and apply the design's aggregate budgets. Format only newly proven unassigned files; never create unbounded extra reset volumes or delete unowned runtime resources.
- [ ] W3.04 [MAC/LINUX] Implement 30-minute activity-based idle stop, warning, bounded in-flight-operation handling, two terminal connections, reconnect, and up to 30-day inactive-file retention within a retained deployment. Explicit confirmed reset/teardown can discard workspaces earlier; traces/exports remain durable. Heartbeats alone do not count as activity.
- [ ] W3.05 [MAC] Build server-rendered user controls and admin account/health/quota/job pages. Admin support access to private workspaces is separately authorized/audited; management roles cannot execute arbitrary commands, grant host sudo or allocate themselves editors.
- [ ] W3.06 [MAC/LINUX] Enforce image/config compatibility and explicit generation switching; failed resets preserve the old stopped generation but still obey current grants. Never replay TUI approvals or uncertain FEAM mutations after a restart.
- [ ] W3.07 [LINUX → UBUNTU] Test memory/CPU/PID/disk/inode containment, queueing, aggregate headroom, spare exhaustion and real read/write boundaries. Use small disposable volumes for disk-full tests; do not fill the shared host or apply a 20 GiB host-free-space threshold to a 2 GiB workspace.
- [ ] W3.G [UBUNTU] Gate: two-user isolation and all lifecycle actions work through the UI; full pool/limits are provisioned with preserved existing data. Ten-user performance remains W9, not inferred from configured quotas.

### W4 — Government climate datasets and shared access

Output: one approved `demo-climate` bundle with a small real raster and Parquet table, immutable releases and tested audience grants.

- [x] W4.01 [MAC] Define non-executable source/job schemas: allowed source, pinned object/version, exact asset inventory, limits, audience, model-disclosure/research-use policy, provenance and approval hash. Bound source count, redirects, elapsed time, bytes and archive expansion; reject LAN/metadata/host destinations and unsafe archive paths.
- [ ] W4.02 [MAC/LINUX; I7 for real fetch] Implement the ECCC bounded CSV/API-to-Parquet importer and AAFC GeoTIFF preparation. Keep raw source bytes private; publish no CSV/JSON tables. Pin actual station/date/variable and source/output hashes. Preserve nulls, quality flags, units and licensing/attribution.
- [ ] W4.03 [MAC/LINUX] Test raster georeferencing/nodata/scientific time and known pixels. If the actual source is not compatible with current WGS84 STAC geometry, create a tested EPSG:4326 derivative with explicit resampling/transform provenance; never relabel the CRS or invent observation time.
- [x] W4.04 [MAC] Implement private candidate construction, complete-byte reservation, per-bundle serialization and FEAM validation/publication through explicit project-root argument arrays. Preserve prior manifest records/tombstones. Registration is through Rust services/CLI, not file presence, SQLite edits or hand-written manifests.
- [ ] W4.05 [LINUX] Test corrupt/renamed formats, missing metadata/inspectors, incompatible Parquet shards, namespace conflicts, duplicate versions, partial downloads, concurrent writers and projection failures. Separately reconcile manifest commit versus release promotion; no blind retry after an unknown result.
- [ ] W4.06 [MAC/LINUX] Add exact-hash admin approval, immutable complete-release promotion on one filesystem, safe orphan recovery, grant-aware assignment and retention. No raw input credentials, drafts or scratch files may appear in a mounted serving tree.
- [ ] W4.07 [UBUNTU] Run the real approved small imports under the bounded pipeline identity. Validate the proposed 250 MiB seed bundle and host storage budgets; reserve complete candidates within the 200 GiB dataset filesystem, 50 GiB staging and 100 GiB retained-release policy.
- [ ] W4.08 [UBUNTU] Mount only authorized releases read-only; attach client links/configuration without replacing practice peers, refresh, and resolve exact inventory. Execute native Polars queries and STAC-selected Rasterio windows in the sandbox with known results; test standard-client tokenless pagination on exact IPv4 loopback (`127.0.0.1`) and rejected non-loopback binding where STAC is exercised. Keep the metadata service and client inside the same isolated workspace; browser/portal authentication remains separate and mandatory.
- [ ] W4.09 [UBUNTU] Grant only user A a test bundle and deny B even by shell path. Test mount writes/symlink writes, reset isolation, container recreation for upgrades/revocations, withdrawal tombstones and rollback restrictions. Already staged copies are not represented as remotely revocable.
- [ ] W4.G [LINUX + UBUNTU] Gate: real outputs/provenance, inspector failures, supported installed SDK/readers, exact manifests and grant enforcement pass. Source catalog links or static STAC JSON alone do not complete the dataset pipeline.

### W5 — Inference broker, socket adapter and project budget

Output: the existing hosted harness works through a constrained broker without upstream keys or general internet in user containers. Use fake provider responses until W7 durability is proven and W8 live traffic is authorized.

- [x] W5.01 [MAC] Implement server-selected model/provider/profile enforcement for the evaluated DeepSeek route, approved tools, bounded payloads and disclosure filters. Preserve internal/provider-safe tool-name mapping, tool-call IDs and typed errors; keep the tested omission of unsupported `parallel_tool_calls`.
- [ ] W5.02 [MAC → LINUX] Implement the in-container HTTPS-loopback-to-private-Unix-socket adapter, scoped trust configuration and revocable workspace/account capabilities. Store the real OpenRouter key only in the broker service. A shell-readable capability must grant no authority beyond its workspace and current budget.
- [ ] W5.03 [MAC/LINUX] Test full streaming exchanges, split UTF-8/SSE frames, complete tool arguments, usage frames, cancellation, disconnects, provider errors and no automatic retries. User-edited profiles must not change provider/model, proxy arbitrary URLs or bypass limits.
- [x] W5.04 [MAC] Implement one in-flight request per user, three across the active host, fair queueing and dispatch-time rechecks of account, workspace, grants, capability and activation generation.
- [ ] W5.05 [MAC/LINUX] Implement transactional broker reservations within an operator-issued bounded run allocation. Keep the US$100 project ledger outside disposable compute; reserve each allocation before launch and reconcile verified usage at shutdown. Test crashes, unknown requests, repeated starts/teardowns and lost hosts without replenishing spend. No off-host acknowledgment per request is required.
- [ ] W5.06 [MAC] Add proposed 75%/90% alerts, visible spend/reservations/queue state and an audited admin increase of the total ceiling. Test near-limit admission, concurrent reservations, unknown cost, no calendar/reset/site replenishment and no silent model substitution using simulated charges, not US$100 of real spending.
- [ ] W5.07 [UBUNTU] Test the actual socket/certificate/identity configuration with fake upstream streams, multiple isolated workspaces and broker restarts. Demonstrate that neither the gateway nor a sandbox can retrieve the upstream key or another conversation/capability.
- [ ] W5.G [MAC/LINUX + UBUNTU] Gate: fake end-to-end compatibility, negative authority checks and crash-safe run/project budget logic pass. Live route compatibility remains W8; lifecycle persistence is verified in W7.

### W6 — Structured usage capture and research controls

Output: correlated, bounded and provenance-aware events, with existing consent represented administratively.

- [x] W6.01 [MAC] Define versioned event schemas, pseudonymous IDs and correlation across request, tool proposal, local review/edit/denial, actual outcome, usage and optional feedback. Record software/profile/dataset revisions and distinguish broker-observed, client-reported and independently verified evidence.
- [ ] W6.02 [MAC/LINUX] Add targeted TUI/harness/service instrumentation and per-workspace event submission over private sockets. Preserve terminal lifecycle and mutation-review semantics. Do not replace structured events with raw terminal transcripts, keystroke capture, billing records or hidden model reasoning.
- [ ] W6.03 [MAC/LINUX] Implement authentication, schema/size/rate checks, deduplication, ordering, bounded append-only storage and index recovery. Test forged client outcomes, missing spans, disk-full, collector restart and explicit capture backpressure/unrecorded status.
- [x] W6.04 [MAC] Redact credentials, email addresses, private paths and raw dataset payloads before persistence/export. Keep participant/account mapping and operational billing separate from the research corpus; restrict trace review/export beyond ordinary admin dashboard access.
- [ ] W6.05 [MAC/LINUX] **R2 shelved; extended durability test deferred.** Retain the R2 implementation/evidence for possible later use, including bounded uploads, watermarks, deletion/withdrawal lineage and retrieval after host deletion. U.03 defines the active Ubuntu archive and Mac-copy path. A failed archive still blocks destruction of the only copy; no R2 transfer or live test is required now.
- [ ] W6.06 [UBUNTU] Trace a synthetic user workflow from request through verified result, including denial/cancellation and missing events. Prove reset does not erase eligible history or evade deletion restrictions; measure capture coverage with the actual deployed processes.
- [ ] W6.07 [MAC/LINUX] Produce a small synthetic reviewed export with verified labels, retained negative examples and participant/dataset/task-aware held-out separation. Mark synthetic material as synthetic; do not claim participant evidence or start SLM training.
- [ ] W6.G [UBUNTU] Gate: trace correlation, redaction, quotas, failure behavior and export permission checks pass. Real-cohort export eligibility/ownership remains a handoff gate, not inferred from test data.

### W7 — Complete IaC, releases and on-demand lifecycle

Output: reproducible start/stop/teardown on either host, scoped operator bootstrap, safe ordinary restarts and project spending that survives disposable environments.

- [x] W7.01 [MAC] Complete Terraform edge/research-storage/cloud-host modules with locked providers and separate lifecycles. Operator automation owns the selected public route; the reconciler owns exact-email membership. Cloud teardown cannot delete research archives, durable edge configuration or unrelated resources.
- [ ] W7.02 [MAC/LINUX] Complete reviewed sudo bootstrap and Ansible preflight/host/deploy/lifecycle entrypoints with ownership checks, secret-log suppression, versioned migrations and selective restarts. Use the verified deployment account and noninteractive sudo for authorized Ubuntu operations. Distinguish fresh initialization, repeat converge, stop and explicit destructive teardown.
- [ ] W7.03 [MAC/LINUX] Implement systemd units/timers and the cross-manager storage-to-rootless-runner ordering. Install bounded logs, socket mounts/ACL recreation and health checks. Disable autonomous Docker restarts so the controller checks current grants/storage/activation before starting workspaces.
- [ ] W7.04 [MAC/LINUX; I3] Implement restricted operator configuration/state and per-run spend allocation persistence outside demo compute. Protect real inventories/keys from Git/images/logs; verify fresh recreation uses current roster/policies and remaining budget.
- [ ] W7.05 [MAC/LINUX → UBUNTU] Implement bounded stop/drain, spend reconciliation and archive verification for all collected traces and reviewed exports. Treat uncertain spend conservatively. Runtime datasets/workspaces/control DB need no cross-host backup; failed archival blocks destructive teardown while preserving its source for retry.
- [ ] W7.06 [MAC/LINUX] Implement explicit desired running/stopped state and deployment ownership. Test that intentional stop, host reboot and health failures never create a cloud VM or restart a stopped demo. Reject ambiguous concurrent activation; verify prior public/model credentials are revoked before a host switch.
- [ ] W7.07 [UBUNTU] Test component restarts and missing/corrupt local state under intended identities. Fail closed for missing budget/policy/storage, preserve unknown costs, and use explicit fresh initialization after unrecoverable demo loss.
- [ ] W7.08 [LINUX] **Deferred.** Recreate from a clean VM using operator configuration and pinned artifacts: first/second converge, version migration/rollback, active and intentionally stopped reboot, missing/wrong mounts, and scoped teardown. No clean VM or host reboot is required for U.G.
- [ ] W7.09 [MAC/LINUX] Produce a reproducible artifact/seed manifest for fresh provisioning on either host. Verify current source hashes and explicit bounded downloads. Prebuilt cloud images are optional; no recovery-time or prior-database restore prerequisite.
- [ ] W7.G [LINUX + UBUNTU] Gate: repeat converge, start/stop/teardown, safe ordinary restart, current private policy inputs and persistent project spending pass. No recovery journal/lease service is required. Real cloud provisioning and billing verification remain W9.

### W8 — Real HTTPS, Access and hosted web integration

Output: a protected, staged end-to-end service on Ubuntu, with real provider compatibility and finite-test evidence.

- [ ] W8.01 [MAC → EDGE; I2] Use the existing Codex Cloudflare connection to verify the intended account and scoped access to the active `613202690.xyz` zone, then configure the explicit `feam`, `admin` and `u-<opaque-id>` records for the selected Tunnel. Verify required DNS/Tunnel/Access permissions and Terraform authentication; request additional credentials only for a demonstrated missing capability. Preserve unrelated resources and the separate `saifshaikh.ca` zone; reuse/import existing demo records before managing them. Domain purchase, activation and owner connection setup are complete; permission, route configuration and verification remain pending. Record MFA and scoped credential/recovery ownership.
- [ ] W8.02 [EDGE + UBUNTU] Configure the Ubuntu tunnel, deny unmatched routes, and enable valid automatically managed HTTPS certificates and HTTP-to-HTTPS redirects. Test browser trust, HTTPS WebSockets, renewal configuration, no direct-origin bypass and no need for public SSH/Docker ports. The separate cloud-host tunnel is shelved; remove it as a mandatory prerequisite of the Ubuntu recipe.
- [ ] W8.03 [MAC → EDGE/UBUNTU] Bootstrap approved private identities and reconcile Access groups with public admission disabled until ready. Activate only authorized staging testers initially; initial cohort invitations/activation belong to W10. Verify real PIN delivery, group sync, separate admin audience and one-hour/admin versus eight-hour/user session policy.
- [ ] W8.04 [MAC browser → EDGE → UBUNTU] Run cross-user/cross-role, forged/expired token, sibling-origin CSRF/WebSocket and live account-disable tests against real Access. Verify disconnection within 30 seconds despite edge API failure and an infrastructure apply cannot resurrect access.
- [ ] W8.05 [UBUNTU → OpenRouter; I4] Recheck the exact selected route, current price limits/privacy settings and evaluated profile. Run a bounded live tool/review/usage exchange through the actual HTTPS-loopback/socket/broker path; verify returned provider/model identity, upstream-key isolation, cancellation and durable accounting. Record all costs and unknown attempts.
- [ ] W8.06 [MAC/LINUX → UBUNTU] **Deferred benchmark.** Freeze the candidate profile/tool schema and a held-out web integration corpus. Reuse historical cases for regression only; keep fresh acceptance distinct. Test representative discovery, ambiguity, resolve, reviewed publication/staging/withdrawal and recovery with real fixtures and verified outcomes. U.06 needs only the small functional smoke.
- [ ] W8.07 [UBUNTU] **Deferred benchmark.** Measure the design's 90% end-to-end task-completion target and zero detected unauthorized writes/disclosures in the finite test set, with counts/failures/limits. The historical 97/100 result and U.06's short smoke do not complete this task or establish that quality claim.
- [ ] W8.G [EDGE + UBUNTU] Gate: valid HTTPS/Access, real hosted compatibility, durable usage/capture and negative authorization checks pass. The service remains staged; ten-user capacity and cloud lifecycle are not yet claimed.

### W9 — Deferred Ubuntu capacity and shelved cloud acceptance

Deferred output: measured Ubuntu capacity/reboot behavior. All cloud work is shelved. None of W9 is a prerequisite for U.G; U.04/U.07 retain the brief functional checks.

- [ ] W9.01 [UBUNTU] **Deferred:** ten active assisted sessions for 60 minutes, shared-data queries, staggered starts, bursts and one bounded ingestion job. No soak test is needed for current delivery.
- [ ] W9.02 [MAC/Ottawa browsers + UBUNTU] **Deferred:** warm-start p95 ≤5 seconds, first-initialization p95 ≤15 seconds and echo p95 ≤250 ms, plus queue/model latency, resource peaks, coverage and cost. U.07 records observations without claiming these targets.
- [ ] W9.03 [LINUX → UBUNTU] **Deferred:** extended component/provider failure, expiry, policy sync, interrupted import/reset, disk-full and missing-release campaigns. Keep the short normal service restart/converge in U.04.
- [ ] W9.04 [UBUNTU] **Deferred:** unattended host reboot, unavailable registry, mount ordering and five-minute readiness measurement. Do not request a maintenance window for U.G.
- [ ] W9.05 [MAC → CLOUD] **Shelved:** provider/region selection and compute/storage/IP quote. No substitute provider search or budget request.
- [ ] W9.06 [MAC → CLOUD] **Shelved:** fresh VM, shared Ansible, private service verification and cloud route selection.
- [ ] W9.07 [CLOUD] **Shelved:** repeated create/start/stop/delete cycles and billing measurements.
- [ ] W9.08 [CLOUD] **Shelved:** cloud ten-user workload and equivalence acceptance.
- [ ] W9.09 [LINUX + UBUNTU/CLOUD] **Deferred/shelved:** extended interruption, stale-credential, failed-allocation/teardown and host-switch tests. No automatic replacement host.
- [ ] W9.10 [CLOUD → MAC] **Shelved:** cloud spend reconciliation, archive survival after deletion and billable-resource teardown proof.
- [ ] W9.U [UBUNTU] **Deferred gate:** full capacity, fault recovery and unattended boot acceptance. It does not block functional Ubuntu acceptance U.G.
- [ ] W9.C [CLOUD] **Shelved gate:** cloud provisioning, equivalent capacity and complete teardown acceptance. It has no current delivery dependency.

### W10 — Invited cohort handoff and evidence readiness

Output: a usable invited service with operator/admin instructions, honest acceptance status and a reviewed path to later SLM work.

- [ ] W10.01 [MAC] Document exact tested bootstrap, start/stop/teardown, migration/rollback, account disable, spend reconciliation and cleanup commands. State who owns renewal, credentials, model/cloud budgets and support; include deployment-key/sudo access revocation and the retained owner access path.
- [ ] W10.02 [MAC + UBUNTU; I6] Confirm on-demand operation, support contact, research retention/export handling and reviewers. No fixed hours or new consent notice. Show model, budget, recording and disposable-workspace status in normal UI.
- [ ] W10.03 [MAC → EDGE/UBUNTU; invitation approval] Reconcile and activate the supplied four-admin/five-user cohort without broadening enrollment. Send invitations only when authorized; test the invited user/admin flows from Ottawa networks. No admin workspace is allocated by default.
- [ ] W10.04 [UBUNTU + MAC] Verify operator records, current policy exports, retained project spending and selected research-export handling through teardown/recreation. Document which resources remain billable after stop versus deletion. No automatic failover or backup service required.
- [ ] W10.05 [MAC/authorized research environment] Review a small eligible export with provenance, verified outcomes, negative examples, deletion lineage and a held-out split; distinguish synthetic development exports from later real-cohort evidence. Name the Phase 2 owner; do not start training as part of this task.
- [ ] W10.06 [MAC] Finish the acceptance report and execution log, link all required artifacts, list known limitations and unresolved gates, and prepare the requested version-control handoff. Commit/push/PR/merge only under the user's applicable instructions; retain unrelated changes untouched.
- [ ] W10.G [UBUNTU + EDGE + CLOUD] **Deferred comprehensive gate:** the original full scope requires all W tasks and W9.U/W9.C. The owner's narrowed request is complete at U.G; do not block that delivery on this historical full-scope gate or describe cloud/capacity work as passed.

## 7. Verification and evidence rules

The active verification budget is a focused local pass for changed components, one image/data smoke and the short U.04–U.07 deployed walkthrough. Reuse existing results for unchanged code. Do not rerun all historical suites, launch long load tests or complete the extended acceptance table merely to close U.G. A timeout or failure in an essential functional check must be fixed or reported as a blocker; it is not silently waived.

### Existing runnable checks

If Rust changes, follow the repository's normal checks from `feather-mesh/`, using focused crate tests while iterating:

```bash
cargo fmt -- --check
cargo clippy -- -D warnings
cargo test
```

Build `cargo build --release -p mesh_cli --features agent-hosted` natively on Linux only if a matching release is unavailable. The broader feature matrix remains in [existing CI](../.github/workflows/rust.yml); no manual rerun of every feature combination is required for this planning change or unchanged Rust. Give active PTY/evaluation runs a pinned artifact/private target directory.

For changed Go services, run `go test -race ./...`, `go vet ./...` and the native build from `web_demo/` with its pinned toolchain and writable caches. For changed deployment/configuration scripts, run `python infra/demo/scripts/check.py` in the locked Python environment. These short existing suites remain useful; do not repeatedly run them without new changes or a concrete unresolved failure. Existing offline suites may include dormant cloud mocks; no separate cloud validation work is required. Terraform format/validation/mock checks apply only to changed **edge** configuration. Cloud-host and research-storage modules are shelved.

Use the actual fixture-generation, fresh installed-SDK, native Polars/Rasterio, tokenless loopback `pystac-client`, PTY, manual/fake walkthrough and offline-evaluation commands in the [workspace README](../feather-mesh/README.md), [SDK README](../feather-mesh/python_sdk/README.md), [demo runbook](tui_agent_stage1_demo.md), and CI. STAC no longer accepts `--token-file`; verify that non-`127.0.0.1` binds are rejected. Quote package-extra specifications in zsh. Generate/run fixtures only in the intended fixture or disposable project scope; do not overwrite user data. Keep live inference opt-in and separate from ordinary tests.

For documentation/context changes, from the repo root, using a Python environment with the checker's requirements installed:

```bash
python .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.py
python .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py
git diff --check
```

These are structural checks, not proof of deployment or semantic correctness. Review changed requirements and verify local links in both this workplan and the design as well.

### Checks required for the functional milestone

The service and deployment commands now exist. Add focused coverage and runnable commands for U.03's new local archive mode, and update affected configuration/runbooks with that implementation. Apply this table to the active scope:

| Surface | Short required check | Deferred work |
| --- | --- | --- |
| Web/control | Focused local tests; real login, WebSocket, ownership/role negatives and disable/revocation with two Ubuntu workspaces | Exhaustive token/provider/failure permutations and capacity measurement |
| Edge | Validate only the changed Ubuntu route configuration; review exact plan and prove HTTPS/Access | Cloud-host tunnel, host switching and cloud Terraform |
| Ansible/systemd/storage | Existing syntax/lint; target mount/ownership checks; one service stop/start and unchanged repeat converge | Clean-VM recreation, shared-host reboot, destructive mount/disk-full tests |
| Image/data | Native digests; one installed SDK/hosted startup smoke; known Parquet/raster read; read-only/grant denial | Repeating unchanged inspector/adapter matrices and cloud images |
| Broker/budget | Fake exchange and stop/start reservation preservation before 3–5 live requests; record usage and unknown costs | Prolonged live traffic and held-out quality benchmark |
| Capture/archive | Redacted correlated events, local flush/readback, denied reviewer access, verified small Mac copy with metadata | R2/S3, destructive host-loss drills and research-corpus preparation |
| Deployed flow | Approximately 10–15 minutes, two users and admin; reuse earlier essential checks | Ten-user 60-minute soak, p95 claims and full recovery acceptance |

### Map back to the design's acceptance scenarios

This maps the **extended** design for future work; it is not an additional U.G checklist. Current required checks are U.01–U.07 and the table above.

| Design scenario | Workplan gates |
| --- | --- |
| 1. Authentication and roles | W2, W8, W10.03 |
| 2. Ownership and origins | W2.04/W2.07, W8.04 |
| 3. Isolation and quotas | W1, W3, W4.09, W5.07, W9.U |
| 4. FEAM and datasets | W4, W8.06 |
| 5. Hosted quality and budgets | W5, W7.04/W7.07, W8.05–W8.07 |
| 6. Usage evidence | W6, W7.05, W10.05 |
| 7. Concurrent load | W9.01/W9.02, W9.08 |
| 8. Failure and ordinary restart | W3, W7, W9.03/W9.04 |
| 9. IaC and boot | W1.07, W7.01–W7.08, W9.03/W9.04 |
| 10. On-demand cloud lifecycle | W7.06/W7.09, W9.05–W9.10, W9.C |

## 8. Execution record

Update this section with task checkboxes. Allowed statuses: `not started`, `in progress`, `implemented; verification pending`, `blocked: <specific input>`, `complete`, `deferred`, `shelved`. Deferred/shelved work remains unchecked and is not an active blocker. Completion requires linked evidence. Historical entries below retain the scope and evidence applicable when written; the latest owner scope and U.G govern current delivery.

| Phase | Implementation status | Required verification status | Evidence / next action |
| --- | --- | --- | --- |
| U | Active delivery plan; U.01–U.07/U.G not yet completed | Short functional checks on actual Ubuntu and Cloudflare; bounded OpenRouter smoke | Repair pipeline/image prerequisites and add local archive mode; prepare dataset/edge inputs alongside that work. |
| Planning | Complete | Documentation checks recorded below; no runtime evidence | Workplan, design and agent/README routing prepared. |
| W0 | Complete | MAC + native Ubuntu checks, LINUX full-VM proof and actual-host preflight including owner-run privileged inventory passed | [W0 evidence](evaluations/web-demo/w0-baseline.md). All eight tasks and exit gate complete. |
| W1 | Complete | W1.01–W1.07 and W1.G pass on MAC/LINUX/UBUNTU, including browser manual/fake flows, resource/socket enforcement, persistence/reset, lingering and zero-change repeat converge | [W1 execution record](evaluations/web-demo/w1-progress.md) and [actual-Ubuntu acceptance](evaluations/web-demo/w1-ubuntu-acceptance.json). Reuse this evidence; host reboot is deferred. |
| W2 | Implemented; local tests pass | Native/private UBUNTU integration pending | Real EDGE verification belongs to W8. |
| W3 | In progress; Ubuntu storage expanded | Short deployed lifecycle/isolation proof in U.04/U.07; full limits/load proof deferred | Reuse the 12-slot pool; no ten-user soak before U.G. |
| W4 | In progress; approved real pair prepared locally | Local importers, native readers, interval STAC and grant negatives pass; Ubuntu pipeline/mount proof pending | Exact Ubuntu candidate review remains required. |
| W5 | Implemented; local fake bridge/accounting tests pass | Actual Ubuntu socket/UID/restart proof pending | Live provider W8; finite allocation, key and price verification remain required. |
| W6 | Local collector/TUI/export tests pass; production currently requires R2 | U.03 local archive mode and U.04 deployed capture required; R2 proof shelved | Add mode/config/lifecycle support before the current native Go build. |
| W7 | Service/IaC/vault/budget recipes implemented; integration pending | Image, service stop/start and repeat converge required by U; full VM/reboot/teardown drills deferred | Reconcile missing ensurepip and partial installation; freeze the updated native release. |
| W8 | U.05 real Ubuntu Access/browser/WebSocket and revocation checks pass; full W8 phase remains extended backlog | U.06 bounded hosted smoke pending; benchmark deferred | Keep Cloudflare/OpenRouter and omit cloud-host tunnel dependencies. |
| W9 | Ubuntu extended tests deferred; cloud work shelved | W9.U/W9.C not required for U.G | No maintenance window, soak run, provider research or cloud allocation. |
| W10 | Minimal operator/demo handoff active through U.07 | Full export/recreation/cloud gate deferred | Current delivery can complete at U.G; invitations remain subject to authorization. |

### Owner input update and scope revision — 2026-09-24

SSH key authentication and a partial read-only preflight succeeded on the actual Ubuntu host. Observed Ubuntu 24.04.5 x86-64, 12 available CPUs, about 15 GiB reported RAM, 4 GiB swap, ext4 `/home` with about 1.5 TiB available, cgroup v2, enabled AppArmor and synchronized time. Ethernet and Wi-Fi are both present; the private inventory identifies the verified Ethernet target. Noninteractive sudo was unavailable; owner will run reviewed bootstrap commands. Docker is active, but unprivileged container inspection was denied: the piped count output of zero is not evidence that no containers exist. `/dev/kvm` was absent; a disposable full-system Linux test path remains unresolved. No remote settings changed in these checks. W0.06 remains partial: mount UUIDs, package origins, detailed delegation/namespace/egress and privileged runtime inspection still need evidence.

The owner confirms no workload requires preservation, already has Cloudflare, will supply the model key when needed, and authorizes candidate research before exact purchase approval. They require operator-started Ubuntu or cloud demos, disposable runtime contents and no fixed hours. Their later clarification removes cloud recovery workflows and mandatory Canadian regions, superseding the original 15-minute target and off-host recovery prerequisites. Existing task IDs remain stable; W0.04, W5.05, W7 and W9 now describe the simpler lifecycle/budget work rather than the superseded recovery architecture. None of that removed work is marked complete. The owner subsequently confirmed that collected research traces and reviewed exports survive shutdown, and raised North American latency: prioritize eastern North America and keep a separate durable archive. Retention duration/reviewers remain pending; survival across shutdown is settled.

Read-only commands included `whoami`, `hostname`, `uname -srm`, `cat /etc/os-release`, `nproc`, `free -h`, `df -hT`, `df -i`, `lsblk`, `stat -fc %T /sys/fs/cgroup`, AppArmor/time-sync checks, `ip -br -4 addr`, identity/subordinate-ID checks, package queries, `systemctl is-active docker.service docker.socket`, and `sudo -n true`. Access addresses, usernames and credential details stay out of this public record. The [options note](ubuntu_web_dev_demo_options.md) records provider/domain research. The agent made no purchases and ran no billable inference or deployment; the owner's subsequent domain purchase is recorded below.

### Session log

Append entries; do not overwrite earlier failures or label old results as a new run.

| Date | Task IDs / environment | Change and evidence | Remaining / next action |
| --- | --- | --- | --- |
| 2026-09-24 | Planning / MAC | Created this agent-ready checklist, environment/dependency map, approval boundaries and acceptance mapping; linked it from the design, README and AGENTS. Documentation verification is recorded in the planning verification entry below. | No web code, host changes, new live inference, domain registration or cloud provisioning. Next: W0.01. |
| 2026-09-24 | Owner scope update / MAC + partial UBUNTU inspection | Recorded verified SSH/partial preflight and owner bootstrap/account decisions. Revised design/tasks for on-demand disposable hosts, retained research with periodic uploads, eastern North American latency preference and no cloud recovery. Added the priced domain/provider/archive shortlist on branch `docs/on-demand-demo-options`. | Documentation only; 96 implementation tasks/gates remain unchecked. Finish W0 baseline/test-target/preflight; exact purchases, cloud tests and live inference remain unapproved. |
| 2026-09-25 | W0.01–W0.05, W0.07–W0.08 / MAC + native UBUNTU build + LINUX guest | Created feature branch; preserved initial edits/Stage-1 evidence; added process/IPC/state/budget contracts, closed schemas, examples, isolated tool/version locks, offline CI and reusable probes. 81 Rust tests, all-feature Clippy/format, 18 contract tests on Mac and Ubuntu, Ansible syntax/lint and 13 context regressions pass. Owner explicitly requested VM setup: signed-image QEMU guest passed systemd/cgroup/loop/remount checks; native hosted FEAM release digest retained. Actual Ansible preflight passed with no changes. Failed sandbox/cache/fixture/VM-start/ensurepip attempts are retained in [evidence](evaluations/web-demo/w0-baseline.md). | W0.06/W0.G pending owner-run privileged host inventory. No sudo host bootstrap, W1 demo service, public exposure, inference, purchase or invitation. VM stopped and retained. Next: review the four sudo-command summaries, then close W0; do not mark W1 runtime isolation complete from this baseline. |
| 2026-09-25 | W0.06 / W0.G closeout; owner on UBUNTU, review on MAC | Reviewed supplied output of the four read-only sudo commands: no Docker container rows/errors; existing `/var/lib/docker`, `overlayfs`, cgroup v2; 21 Snap-only loop devices; AppArmor loaded with 158 profiles (62 enforcing, 5 complain, 91 unconfined). Recorded [sanitized evidence and raw transcript digest](evaluations/web-demo/w0-privileged-preflight.json); raw output retained privately. Final scope preserves the existing daemon, Snap loops and host protections. Resumed the W0 branch at the same base commit, retaining the intervening `demo.saifshaikh.ca` planning edits. | W0 complete; W1–W10 remain unstarted. Next ready task: W1.01 hosted terminal image, followed by W1 runtime/storage proof and reviewed bootstrap. No new host operation was needed to review this evidence. |

Planning verification (2026-09-24, MAC): context structural checks passed; all 13 context-checker regression tests passed; local links/anchors passed in this workplan, the design, AGENTS and README; task-structure validation passed for 96 unique unchecked tasks/gates across all 11 phases, with execution labels and no private roster emails; `git diff --check` passed. The context checks used `/private/tmp/feam-context-venv/bin/python`; a fresh environment needs the checker's requirements. Semantic review covered design-to-task mapping, source-of-truth ownership, execution boundaries, approval gates and evidence separation. Rust/application/Ubuntu/cloud runtime checks are not required merely to author this plan and have not been executed for this planning task.

Scope-update verification (2026-09-24, MAC): reran the context structural checker and all 13 checker regression tests successfully using the existing `/private/tmp/feam-context-venv` environment. Local links/anchors across AGENTS, README, design, workplan and shortlist passed; all 96 stable implementation task/gate IDs remain unique and unchecked; `git diff --check` passed. Semantic review checked removed recovery/region/hour gates, durable traces versus disposable workspaces, current policy/project budget on recreation, paid-resource boundaries, and accurate pending Ubuntu/cloud evidence. No Rust or application code changed, so runtime/build tests were not run. Pre-existing `.DS_Store` changes were left untouched. Domain registry lookups and public pricing were read-only; no resources or credentials were provisioned.

Verification attempt retained: the first ad hoc link check interpreted README's repository-root links as filesystem-root paths and reported four missing targets. Correcting that check to GitHub's repository-root semantics passed all 76 local links/anchors; no link edit was needed.

Domain follow-up (2026-09-24, MAC; I2 remains pending): recorded the owner's Cloudflare $11.20/year quote for `613202690.xyz`, corrected the earlier numeric-XYZ cost assumption, and documented sibling `feam`/`admin`/workspace hostnames in the shortlist. Added `.win`, `.date`, `.bid` and `.uk` candidates with explicitly third-party, indicative Cloudflare prices and separate renewal amounts. No new candidate's checkout availability was verified. The public Cloudflare search fetch first failed sandbox DNS resolution, then returned HTTP 403 with network access; no authenticated checkout was attempted. No purchase, DNS mutation or implementation status change occurred. Replaced residual design wording about failover with fresh deployments/host changes.

Domain follow-up validation: context structural checks and local links/anchors passed; semantic review keeps reported quotes, comparison prices, configuration proposals and deployed evidence distinct. All 96 implementation task/gate IDs remain unique and unchecked. The first ad hoc counter matched only the 84 numeric task IDs; including the 12 lettered gate IDs corrected the check without changing task status. `git diff --check` passed. No runtime tests were needed for these documentation edits.

Spaceship follow-up (2026-09-24, MAC; I2 remains pending): added the owner's $0.95 numeric-XYZ option and verified that Spaceship permits Cloudflare nameservers. Official public pricing independently shows a US$0.75 introductory `.xyz` promotion plus US$0.20 ICANN fee, but neither that generic offer nor its generic renewal proves the requested numeric name's renewal. Recorded exact-name availability/renewal as unverified; no purchase or DNS changes occurred. The shortlist and design now include Spaceship registration with Cloudflare DNS.

Prerequisite update (2026-09-24, MAC; I2 partially satisfied): owner reports purchasing `613202690.xyz` at Spaceship. Their Cloudflare screenshots show the exact domain and assigned `mark.ns.cloudflare.com` / `norah.ns.cloudflare.com` nameservers, but not Active status. A read-only `dig @1.1.1.1 613202690.xyz NS +short +time=3 +tries=1` returned no records; the follow-up with `+noall +comments +answer +authority` returned NOERROR, zero answers and an `.xyz` SOA. This does not confirm Cloudflare delegation or activation; it does not contradict the reported purchase. The private roster file still exists and is ignored by Git; contents were not read. No additional owner input blocks MAC implementation. Finish DNS activation independently; request the reviewed sudo bootstrap, scoped edge/archive access, model key/test budget, priced cloud account and research retention/reviewers only when the corresponding work is ready. Linux test-target selection, full preflight and implementation remain agent work.

Hostname decision (2026-09-25, MAC; I2 selection settled, EDGE configuration pending): owner requested `demo.saifshaikh.ca` after stating a nine-hour demo deadline. Their screenshot shows `saifshaikh.ca` Active in Cloudflare. During this conversation, `dig @1.1.1.1 NS saifshaikh.ca +short` returned `mark.ns.cloudflare.com` and `norah.ns.cloudflare.com`; `dig @ns0.centralnic.net NS 613202690.xyz +norecurse +noall +comments +answer +authority` returned NOERROR with zero answers and an `.xyz` SOA. These read-only observations support reusing the existing zone, not a working demo endpoint. Updated the workplan, design and domain note to use the portal and first-level sibling routes above. The earlier requirement to activate `613202690.xyz` before W8 is superseded. No DNS, Tunnel, Access, application deployment or implementation checkbox was changed by this documentation update.

Hostname-update verification (2026-09-25, MAC): context structural checks passed using `PATH=/private/tmp/feam-context-venv/bin:$PATH bash .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh`; all 46 local links/anchors in the three affected documents passed an ad hoc Python check; all 96 unique implementation task IDs and checkbox states match the pre-edit snapshot; `git diff --check` passed. Semantic review confirmed the existing-zone scope, separate origins, first-level certificate coverage and pending W8 evidence. An initial validation orchestration attempt failed before shell execution because of quoting; the corrected invocation passed. This documentation-only change required no Rust/application tests and established no new EDGE, UBUNTU or CLOUD runtime acceptance.

### Domain activation and hostname switch (2026-09-25)

The owner reported Spaceship's servers online, then explicitly requested switching back to `613202690.xyz` and confirmed that Cloudflare shows it as **Active**. That dashboard status is owner-confirmed; the agent did not authenticate to Cloudflare. This supersedes the temporary `demo.saifshaikh.ca` decision above.

Read-only public DNS checks from MAC during this conversation:

```bash
dig @1.1.1.1 613202690.xyz NS +noall +answer +comments +time=3 +tries=1
dig @8.8.8.8 613202690.xyz NS +noall +answer +comments +time=3 +tries=1
dig @1.1.1.1 613202690.xyz A +noall +answer +comments +time=3 +tries=1
```

Both NS queries returned `NOERROR` with `mark.ns.cloudflare.com` and `norah.ns.cloudflare.com`. The apex A query returned `NOERROR` with zero answers. Initial sandboxed DNS queries were blocked by socket permissions; the successful read-only checks used approved network access. These results establish public visibility of the nameserver delegation through two resolvers, not a working application or HTTPS route.

Updated the design, hostname plan, domain note, operator inputs and acceptance index on MAC. Selected routes are `feam.613202690.xyz`, `admin.613202690.xyz` and `u-<opaque-id>.613202690.xyz`. Registration stays at Spaceship and the separate `saifshaikh.ca` zone remains outside the demo's scope. No DNS, Tunnel or Access mutation was performed. Scoped credentials, route/certificate/authentication verification and application deployment remain pending W8; this decision does not complete an implementation task or deployment gate.

Documentation verification: context structural checks passed with `PATH=/tmp/feam-context-venv/bin:$PATH bash .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh`; an ad hoc check passed all 84 local Markdown links/anchors across the five affected documents; `git diff --check` passed. Semantic review confirmed consistent first-level hostnames, separate origins, owner-attributed Active status and pending W8 verification. Rust and application tests were not run for this documentation-only change.

### Zero Trust onboarding confirmation (2026-09-25)

The owner confirms that Cloudflare Zero Trust onboarding is complete. Record this as a satisfied account prerequisite for I2; do not request onboarding again. The agent has not authenticated to the account. Account MFA remains unconfirmed, and scoped API credentials, DNS routes, HTTPS, Tunnel, Access applications/policies and real login verification remain W8 work. No account configuration or implementation task status was changed by this documentation update; current development can continue independently of scoped Cloudflare access.

### Ubuntu deployment access verified (2026-09-25)

I1 access update, MAC → UBUNTU: the owner completed the dedicated `feam-deploy` account/key and passwordless-sudo setup, then the agent verified it over private SSH. The read-only command below exited 0 without a password prompt and returned `feam-deploy` followed by `0`:

```bash
ssh -i "$HOME/.ssh/feam_ubuntu_deploy" \
  -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes \
  -o ConnectTimeout=10 feam-deploy@"${FEAM_UBUNTU_HOST:?set from private inventory}" \
  'id -un; sudo -n id -u'
```

The actual verified host address is redacted here; obtain it from private operator inventory. The dedicated private key remains on the operator Mac at `~/.ssh/feam_ubuntu_deploy`, outside Git. No password or private-key contents are required in chat. [Operator guidance](../infra/demo/README.md#unattended-ubuntu-deployment-access) records connection settings and revocation.

This supersedes the earlier owner-run-only bootstrap requirement and the historical noninteractive-sudo blocker for future authorized Ubuntu work. The deployment account has full root capability; it is separate from the seven runtime service identities and from participant/admin web roles. Keep the existing W1 browser operator UID/ACL unless an explicit configuration change requires otherwise. Access availability does not expand the task's approved scope or authorize reboots, public exposure, invitations, purchases or live inference.

Only SSH identity and noninteractive root execution were tested for this update; no provisioning, reset, reboot or application acceptance was rerun. W0/W1 evidence remains historical and W2–W10 stay pending. Next ready implementation is W2; scoped EDGE credentials, source pins, live-model budget/key, archive inputs and cloud/reboot approvals remain due at their existing gates.

Documentation verification on MAC: context structural checks and all 121 local links/anchors across the five affected documents passed; all 96 implementation task IDs and checkbox states were preserved; `git diff --check` passed. Semantic review confirmed delegated execution, full-root capability, unchanged resource/approval boundaries and separate historical acceptance. No Rust or application tests were needed for this documentation-only update.

### Cloudflare connected to Codex (2026-09-25)

I2 connection update: the owner reports that Cloudflare is connected to Codex. The agent confirmed that Cloudflare API search/execute and resource tools are present in the current session's tool catalog. This records completed connection setup, not an authenticated account/zone permission test; no Cloudflare API request or resource change was made for this documentation update.

Use the existing connection before requesting credentials or another onboarding step. Before EDGE implementation, verify the intended account, access to `613202690.xyz`, required DNS/Tunnel/Access permissions and whether the selected Terraform workflow has suitable authentication. Do not assume the Codex connection supplies credentials to Terraform or deployed services. Request only a specifically missing capability when its implementation is ready; retain secrets in private operator configuration.

This supersedes the generic request to supply Cloudflare access in earlier planning entries. MFA, renewal/recovery ownership, deployment configuration, HTTPS, Access login and public-route verification remain pending. The connection does not authorize public exposure, purchases or invitations. No W8 task or other implementation gate is completed by this prerequisite update; W2 remains the next ready implementation phase.

Documentation verification on MAC: context structural checks, all 129 local links/anchors across the six currently changed documents and `git diff --check` passed. All 96 implementation task IDs and checkbox states are preserved. Semantic review separates owner-reported connection setup/tool availability from untested permissions, automation authentication and deployment acceptance; no application or live EDGE tests were run for this documentation update.

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

- **Current execution:** use the [functional takeover record](evaluations/web-demo/ubuntu-functional-progress.md) for live task status, retained failures and pending human inputs. The older resume sequence below records the starting point; source and current receipts establish completed implementation.
- Use the [checkpoint](evaluations/web-demo/wrap-up-checkpoint.md) for retained implementation/evidence, then follow U.01–U.07. The owner has replaced the comprehensive release requirement with U.G: a functional Ubuntu web demo using Cloudflare access and OpenRouter.
- Continue on `feat/complete-ubuntu-web-demo`; existing implementation is uncommitted. Preserve unrelated `.DS_Store` and private inputs. Planning changes do not imply a deployment, commit or public activation.
- Next work: apply the prepared exact public-route activation after the pending owner response. Both staging terminals now pass private checks and the failed start is explicitly reconciled. Complete real PIN/browser, dataset grants, fake capture/review and the approved bounded hosted smoke, followed by ordinary stop/start and the verified Mac archive copy. U.01 is complete; the dataset is promoted and the final native service build is installed.
- The explicit local collector archive and root configuration/lifecycle support are implemented and pass local/native checks. Deployed capture/flush and independently verified Mac-copy evidence remain U.03/U.07 work. No R2 transfer, R2 Terraform apply or live S3 check is active work.
- Private operator inputs and the initialized US$100 ledger stay on the Mac. The owner approved the exact dataset release, final native build, three staging accounts and five-task US$1 smoke; supplied keys are stored privately. Public-route authorization remains pending the concrete proposal and private readiness. Preserve the existing R2 bucket and private credentials untouched while shelved.
- Complete short browser/live checks and operator handoff; record functional Ubuntu completion at U.G. Leave lengthy benchmark/capacity/reboot/VM/recovery checks deferred and all cloud-host/R2 work shelved. Do not revive those gates as a condition of delivery.

### Owner priority revision — functional Ubuntu demo (2026-09-25)

The owner requested the fastest route to a functional web demo, waived lengthy acceptance exercises for this delivery and shelved cloud-related work. They clarified that Cloudflare browser access and OpenRouter stay in scope, while R2 and cloud VMs are shelved. This supersedes the earlier resume sequence requiring full-VM proof, live R2 acceptance, ten-user load/reboot gates and cloud completion before final handoff.

Added U.01–U.07/U.G as the active milestone and retained all 96 W task IDs and their checkbox states. Local archival plus a verified private Mac copy before destructive teardown preserves the research-retention requirement without R2; this is planned implementation, because the present collector/config validator require R2. The milestone keeps a short real login/isolation/data/broker/capture demonstration and ordinary service stop/start, then allows delivery without the deferred extended gates. No runtime code, host, edge or cloud configuration changed during this plan revision.

Documentation verification (MAC): context structural checks and all 13 context-checker regressions passed; all 152 local links/anchors across the five changed documents resolve. Task comparison preserved the 96 W IDs/checks and confirmed all eight new U entries are unchecked. `git diff --check` passed. Semantic review confirmed the active U.G dependency chain, shelved cloud/R2 scope, deferred lengthy tests, and the distinction between proposed local archive support and current code. No application, native build, host or live-provider tests were run for this documentation change.
