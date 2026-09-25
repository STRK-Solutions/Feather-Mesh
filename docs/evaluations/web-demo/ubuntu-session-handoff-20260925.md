# Ubuntu demo session handoff — owner-requested stop

Historical checkpoint. Superseded by the [resumed operator handoff](ubuntu-operator-handoff-20260925.md).

Stopped gracefully on 2026-09-25 at approximately 12:38 UTC so another Codex
session can take over. This file supersedes older resume instructions; the
[workplan](../../ubuntu_web_dev_demo_workplan.md) still owns completion status.
Continue on `feat/complete-ubuntu-web-demo` in the same working tree.

## Verified state at stop

- U.01, U.02, U.03 and U.05 have measured evidence. U.04, U.06, U.07 and U.G
  remain open. Earlier three-person PIN, data/isolation, reset/review and
  revocation evidence is retained; it is not an eleven-person login claim.
- All six participant services and the Cloudflare connector are **inactive**.
  Controller desired state is stopped, both assigned workspaces are stopped,
  and there are zero pending jobs or snapshots. The public demo is therefore
  **not serving a usable workspace at this checkpoint**.
- The full private YAML roster has eleven accounts: four admins and seven
  regular users. One admin and one user are active; three admins and six users
  are pending. Eight missing accounts were created through audited application
  commands. The disabled staging alias was explicitly restored with a fresh
  authorization version; its controller barrier was removed after verifying
  completed revocation and stopped state. Five new workspaces still need
  assignment, configuration and actual group reconciliation.
- The exact owner-approved edge expansion is applied: nine distinct Access
  apps/audiences and DNS records, nine exact tunnel hosts plus the final 404,
  and matching HTTPS redirect. Direct API readback and a zero-change Terraform
  plan pass. Existing four audiences remain unchanged. See the
  [edge receipt](u07-full-cohort-edge-proposal.md).
- The **US$50 shared allocation** was issued exactly once against the existing
  US$100 project ledger. Project revision is 2 with one active run. No paid
  request has been sent; the live broker database and installed upstream key
  are absent. The cap is shared across seven users, US$0.03 per provider
  request, 1,024 output tokens, expiry **2026-09-27 08:00 UTC**, pinned DeepSeek
  V4.1 Flash through `deepinfra/fp8`, no fallback/retry or credit purchases.
- Fake-run lifecycle vault revision 47 is `stopped_verified`. Eight archive
  objects and provenance have a separately verified owner-only Mac copy. Two
  fake requests settled at zero; an intentional cancellation remains unknown
  with its recorded gap. Do not zero or replay it. The W1 reconverge regression
  was repaired; the exact original container was restored without data reset.
- Headed test browsers were saved to private storage-state files and closed.
  All subagents stopped; no native build, test or deployment process remains.

## Git and validation

Two local commits exist; neither has been pushed:

- `29d0e39` — Ubuntu demo services and bounded operator lifecycle.
- `f1bc22e` — merge of main's guided tutorial and TUI improvements into the
  feature branch. Main from origin was `d6f1339` (PR 63). The feature has **not**
  been merged back to main.

The committed state passed the full Go race suite, vet/build, offline infra
checks, context checks, Rust workspace all-feature tests/Clippy, and CLI feature
matrix. The terminal fix also passed isolated Chromium viewport checks.

Two subsequent source edits are **uncommitted and untested**:

- `feather-mesh/mesh_agent/src/provider.rs`: guided requests advertise no tools
  and use `tool_choice=none`; historical tool-call translation uses the closed
  approved schema while new response tool calls remain forbidden.
- `web_demo/internal/broker/protocol.go`: accept that explicit no-tool mode and
  validate approved historical tool names separately from new response tools.

These fix a discovered integration gap: the previously built broker rejected
empty tool lists, and the router could not translate existing tool history when
switching into the guide. Regression tests and formatting are still required.
Do not treat the earlier test receipts as coverage of these edits.

Documentation status updates and two U.07 proposals are also uncommitted.
Preserve unrelated `.DS_Store` and `.ansible/` changes and exclude them from Git.
Private roster, credentials, raw browser records and traces stay ignored under
`.local/demo-deployment/`; the candidate secret scan found no roster identities
or live keys in the public files.

## Remaining work, in order

1. Finish the guided transport fix and regression tests for empty-tool mode,
   ordinary-to-guided history, and rejection of a returned tool call. Run the
   focused Go/Rust checks, then the required affected-surface checks. Commit
   the completed fix on the feature branch.
2. Rebuild exact updated native Go/Rust source with two jobs and a new immutable
   participant image. Preserve the existing binaries/images. The first merged
   candidate image exists but predates the last fix and has **no image smoke
   result**: its local smoke script failed parsing before contacting Ubuntu.
3. Finish the seven-workspace private inputs: use the actual eleven-account
   readback, nine actual audiences, current binaries/image and the **already
   signed $50 allocation**. Correct the candidate allocation path. Review and
   render into a fresh private directory; do not reuse stale two-user inputs.
4. Install while stopped, initializing **only the absent live broker DBs**.
   Never reinitialize gateway/controller/pipeline/collector or the project
   budget. Assign the five new controller and gateway workspaces with the new
   explicit commands; upgrade the two retained workspaces to the new image
   with generation/image checks. Preserve their files and grants, W1 slots,
   and retained reset data. Complete actual reconciler acknowledgment for all
   four admins/seven users and grant the exact approved climate release.
5. Create a separate live lifecycle vault after actual runtime readback.
   Adapt its preparation helper to the explicitly reviewed seven-workspace
   scope/new release while retaining the fake archive/copy lineage. Start
   core services, then explicitly start the existing QUIC connector. Do not
   use `prepare-next-run` across the separate fake and real budget projects.
6. Reopen browsers from private storage state. Admin Access has expired and
   may require a fresh human email PIN; keep PINs out of chat. Verify the real
   protected terminal fills the viewport at a readable size, tutorial behavior,
   all-account readiness, role/isolation and dataset access. Execute the five
   prepared live tasks and a bounded guided-mode check, recording real route,
   usage, review/denial and capture. Remaining shared allowance is authorized
   for the seven users after the smoke.
7. Complete ordinary service restart/convergence and private archive-copy
   evidence without discarding research or replenishing spend. Full lifecycle
   stop drains/marks its allocation unknown, so review the lifecycle/accounting
   transition before using it as a casual restart. Keep actual known costs and
   reservations across any successor allocation within the $50 shared total.
8. Update final acceptance, workplan and operator handoff from measured results.
   Commit scoped changes, push the feature, merge to current main including
   the guided tutorial/UI changes, and verify remote main. The owner explicitly
   authorized commit/push/merge; do not request the same authorization again.

## Resume inputs

Private path and hash inventory, exact state revisions, source candidates,
browser state, approved budget and known script issues are recorded in
`.local/demo-deployment/functional-proposal/session-handoff-20260925.json`.
Read that before execution. Current private status proof is
`session-stop-readback-20260925.json` in the same directory.

No further approval is pending: the owner approved the exact edge plan after
automatic review initially rejected it, and the approved retry succeeded.
The later explicit US$50 shared-cap instruction supersedes the older US$1
proposal. Invitations/messages remain unauthorized. No reboot, destructive
teardown, cloud-host/R2 work or extended capacity/benchmark gate is required.
