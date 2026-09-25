# Ubuntu demo operator handoff — 2026-09-25

The current release is deployed at [the participant portal](https://feam.613202690.xyz).
[Administration](https://admin.613202690.xyz) uses a separate Access audience.
The exact private roster contains four active admins and seven active users;
all seven users have a workspace and the approved read-only climate release.
No invitations were sent. Saif remains the operator, sole research reviewer and
Phase 2 owner; the private roster/operator configuration retains the support contact.

The [release receipt](u07-updated-release.json), [live smoke](u06-live-functional-smoke.json)
and [maintenance/archive receipt](u07-live-maintenance.json) give measured evidence.
This supersedes the stopped [session handoff](ubuntu-session-handoff-20260925.md).

## What was verified

Two real participant sessions renewed their application tokens through their
existing Cloudflare SSO sessions, started workspaces and opened protected
terminal WebSockets. Both used the installed SDK for a 365-row Polars query and
a known 2×2 Rasterio window. Cross-user terminal access returned 403; actual
service authorization denied all non-owners, including all four admins.
Admins have no workspace. The [desktop terminal](u07-terminal-viewport.png)
fills a 1440×1000 viewport at 18px. The TUI needs at least 80×24 cells;
a narrow mobile viewport displays its minimum-size notice.

Five live tasks and two guide explanations used DeepSeek V4.1 Flash through
`deepinfra/fp8`. All 11 provider requests settled at **US$0.003119**, with no
reserved or unknown live cost at the measured checkpoint. Each has correlated,
non-synthetic broker request/usage events. No fallback or automatic retry ran.
The guide accepted ordinary tool history, explained opening Help and advanced
after the real local action. The complete 15-lesson curriculum was not tested.

One model stage proposal used an unknown handle and was rejected without a
write. A fresh manual local review copied the exact approved file once. A
separate prompt supplied its selected handle explicitly; the model opened the
review, local denial succeeded and the destination/receipt remained absent.
The raster explanation understated available metadata. These observations do
not establish five-of-five model success or scientific-answer quality.

**Remaining acceptance:** the saved admin session expired, and the owner could
not complete a fresh email-PIN sign-in. No admin login was fabricated. The
initial real admin grant/reset/revocation browser evidence remains in
[U.05](u05-browser-acceptance.json); new grants used the tested, audited privileged
operator command. U.07's fresh admin walkthrough and U.G final acceptance remain
unchecked. This is the only unavailable user-dependent acceptance check.

## Release and budget

| Item | Pin |
| --- | --- |
| Native Go source | `2812621523dab3d47ee44ac32a380282fe46c0b7d2e1927e1b6398fe04545642` |
| Native FEAM binary | `9e1465238a5bee23f2dfbce1e909042be6e03ab6dcd9d0e68cf8b5ff70acd330` |
| Participant image | `sha256:53644f8c841441a83440076218df0a2c683f477f6390f370d7fe6bd799c71878` |
| Climate release | `9894f3498b7d0828d3358b6dd415f6ee0ba298442bca7f615987af147a2aef41` |
| Runtime manifest | `67c185d5e5fa92c4e6a2b916cde656378440b1acd1f93e34ae3861c613cf37a4` |

The existing US$100 project issued one shared **US$50** allocation for all seven
users. It expires **2026-09-27 08:00 UTC**, caps each provider request at US$0.03
and 1,024 output tokens, and permits three simultaneous model requests globally,
one per user. Remaining allowance is available to the authorized cohort; the
smoke did not replenish it. Dashboard figures are authoritative for subsequent
activity. Project encryption/signing keys stay on the Mac. The upstream key is
restricted to the broker identity on Ubuntu.

All original images/binaries, W1 slots, participant files, retained reset data,
grants and fake-run archives were preserved. The fake run's intentional unknown
cancellation remains unknown; it is separate from the fully settled live smoke.
Research retention remains 30 days, including withdrawal/expiry maintenance of
all retained Mac copies. No teardown, reboot, cloud host or R2 operation ran.

## Operator commands and private records

Run from the repository root. The locked interpreter is
`/private/tmp/feam-web-w0-venv/bin/python`. Private inputs and receipts live under
`.local/demo-deployment/functional-proposal/`; none belongs in Git.

- Current live vault: `live-lifecycle-takeover/operator.enc`; its key is the
  existing `synthetic-lifecycle/operator.key`. The original fake vault remains
  at revision 47. Inspect the current live revision with `operator_vault.py verify`
  before each lifecycle action; never reuse a stale revision.
- Installed configuration: `/etc/feam/services/runtime.json` and each
  `/etc/feam/services/SERVICE/config.json`. Binary release directory is
  `/opt/feam/services/` followed by the Go source digest above.
- SSH uses the dedicated `feam-deploy` identity, strict host-key checking and
  the private `ssh-target.json`; privileged commands use `sudo -n`.
- Logs: `sudo -n journalctl -u feam-SERVICE.service --since '15 minutes ago'`,
  with SERVICE one of gateway, controller, broker, collector, pipeline or
  reconciler. Connector logs use `feam-edge.service`.
- Readiness: `u05_quic_ready_probe.py` checks the existing QUIC connector.
  `takeover_live_probe_remote.py` through `takeover_remote.py` records private
  accounting/capture; use a new receipt label for each read.
- Repeat converge uses **`takeover-live-configs/service-vars-post-init.json`**;
  its initialization list is empty. Never reuse the original initialization
  input or initialize any existing database/project ledger.
- Account disable: fresh admin sign-in → matching account → **Disable account**.
  This advances authorization and closes current streams. Restore/grant support
  is documented in [recovery](../../../web_demo/RECOVERY.md); inspect the current
  account/grant version and recorded outcome before another mutation.

Ordinary service maintenance was exercised by the private, hash-recorded
`takeover_maintenance.py`. `stop REVISION` stops admission/workspaces/services,
checks zero in-flight work, retains the **active existing allocation**, flushes
and verifies the archive, and independently copies it to a new private Mac
directory. It checkpoints `needs_review / stopped_for_converge`, not full
`stopped_verified`. After an unchanged post-init converge, `start REVISION`
checks the exact archive and project ledger, restarts core services and the
existing connector, and requires identical broker accounting. Inspect an
interrupted step; do not automatically replay it. The final private receipt
indexes the tested script hash and current vault revision.

For a full stop, use [the production lifecycle](../../../infra/demo/SITE_LIFECYCLE.md)
with the current live vault and exact revision. Its `stop` drains/pauses the run
and retains the allocation as unknown in the project ledger; restarting that
completed lifecycle requires its reviewed allocation transition. It is not the
ordinary same-allocation maintenance command. Research must remain retained,
with a verified Mac copy, before any separately authorized destructive teardown.

The connector is active but boot enablement remains off. Participant services
and the read-only health timer are enabled. Unattended reboot, ten-user capacity,
load/p95 measurements, clean-VM recreation, broad fault recovery, new held-out
benchmarking and cloud/R2 acceptance remain deferred or shelved.
