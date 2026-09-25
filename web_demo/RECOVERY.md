# Operator lifecycle recovery and fixed-slot reclamation

Unknown lifecycle jobs never cause an automatic start/reset replay. Ordinary
inspection can reconcile an already observed result. When the intended runtime
is missing or the operator chooses to abandon an uncertain action, use the
explicit hash-bound abort below. This resolves runtime control only; FEAM's
local operation journal remains authoritative about peer mutations.

`serve`, migration and runtime-mutating operator commands hold one persistent
`<database>.controller.lock` using an exclusive nonblocking `flock`. Stop the
controller service before those operator commands. Never remove this lock
file. Status/review reads and atomic `activate`/`stop-intent` updates remain
available while the controller runs. The latter update desired state; they
do not themselves issue runtime calls.

An existing deployment can receive a newly configured workspace with
`controller -action assign -config CONFIG -workspace WORKSPACE_UUID`.
The assignment must exist in the private controller configuration with an
endpoint and matching deployment/image. The command requires a stopped,
quiescent deployment, allocates one free workspace slot, and queues its gateway
snapshot. Existing accounts, slots and files remain intact; initialization
must never be repeated to add users.

After an explicitly authorized account restoration, use
`controller -action restore-account -config CONFIG -workspace WORKSPACE_UUID
-expected-generation GENERATION`. It verifies the recorded container is
stopped and removes only that owner's controller revocation barrier. Gateway
restoration and successful Cloudflare group reconciliation are still required.

To deploy an updated immutable participant image, first stop and archive the
site, install the reviewed image and updated configuration, then run
`controller -action upgrade-image -config CONFIG -workspace WORKSPACE_UUID
-expected-generation GENERATION -expected-image OLD_IMAGE_DIGEST` for each
retained workspace. The new image comes from the installed configuration.
It verifies the old runtime is stopped, preserves the writable slot and grants,
advances the generation once and queues a gateway snapshot. Prior containers
remain stopped. A stale image/generation, pending job/reclaim or running site
is rejected. Inspect durable state after an uncertain reply; do not replay.

Run controller commands under its configured service identity using the
installed reviewed binary. Substitute an exact job ID and the SHA-256 returned
by review; the review contains no terminal contents or participant email:

```bash
controller -action recovery-review -config /etc/feam/services/controller/config.json -job JOB_UUID
controller -action abort-job -config /etc/feam/services/controller/config.json -job JOB_UUID -confirm-sha256 REVIEWED_REQUEST_SHA256
```

The controller persists an abort-pending marker before stopping anything. The
daemon preserves that marker across a crash. Abort revokes capabilities, stops
and verifies the recorded current and reset-target generations, retains all
private slot bytes, marks the job failed without replay, advances generation
past the uncertain target and queues the current gateway snapshot. Failure
leaves the job unresolved; repeat only this same explicit abort after restoring
the failed dependency. The exact completed abort is idempotent. An unrelated
job/hash and an already completed normal operation cannot be aborted this way.

## Inactive-file warning

The daemon records a warning after 27 days of inactivity for stopped
workspaces. The portal receives `retention_notified` and
`retention_delete_after` with workspace status. Deletion eligibility starts
after 30 days and at least three days after the recorded warning. A long
offline period therefore never converts a first warning into immediate
deletion. New activity invalidates the old warning. The operator can inspect
`retention-status`; `retention-warn` is the equivalent explicit offline update.
Research events and reviewed exports follow the independent collector policy.

Retention cleanup is an explicit operator action under the accepted policy.
The operator must establish that the advance warning was presented before
executing the reviewed scope. Configured policy alone is not evidence of
notification or completed deletion. A confirmed reset/delete can make its
retained generation eligible earlier; other retained generations wait 30 days.

## Reclaim one exact fixed slot

This path does not format, resize or allocate a volume. It removes private
contents from one recorded stopped slot. Set deployment stop intent, let the
daemon drain/stop workspaces, then stop the controller unit. A running or
uncertain workspace blocks reclamation. Under the controller identity:

```bash
controller -action prepare-reclaim -config /etc/feam/services/controller/config.json -slot slot-01
controller -action reclaim-review -config /etc/feam/services/controller/config.json -reclaim RECLAIM_UUID
```

Preparation revokes capabilities and removes only exact stopped,
template-verified containers with `force=false` and `v=false`. It preserves
private bytes, persists the slot/generation/filesystem UUID and hash, and blocks
new lifecycle admission for that workspace. Old grant versions come from their
recorded immutable runtime templates. Current grants cannot silently replace
the authority for an old container's mount inventory.

Review the returned plan, then run the fixed helper as the root operator:

```bash
python3 infra/demo/scripts/slot_reclaim.py check --id RECLAIM_UUID --confirm-sha256 PLAN_SHA256
python3 infra/demo/scripts/slot_reclaim.py apply --id RECLAIM_UUID --confirm-sha256 PLAN_SHA256
```

The helper requires the root-owned service manifest and storage inventory. It
checks the reviewed controller config hash, same controller process lock,
pending intent, stopped deployment, exact slot assignment, mount UUID, backing
file device/inode/size, initialized non-sparse reservation and loop backing
path. It rejects nested mounts and any container, including stopped containers,
whose mount overlaps the slot. Descriptor-relative traversal refuses symlink
roots and never follows child symlinks. A bounded preflight walk precedes
deletion; any incomplete cleanup retains the pending intent. The verified
filesystem `lost+found` must be empty and is preserved. No app service receives
sudo or access to arbitrary root commands.

Only successful empty-directory and runtime-absence verification produces
`/run/feam/reclamation/RECLAIM_UUID.json`, a root-owned receipt bound to the
intent hash and filesystem UUID. Then, under the controller identity:

```bash
controller -action complete-reclaim -config /etc/feam/services/controller/config.json -reclaim RECLAIM_UUID
```

Completion rejects missing, writable, unowned or mismatched receipts. A
retained slot becomes a free finite reset spare. An inactive current workspace
keeps its assignment with a fresh generation and stopped state. It starts only
after a later explicit authorized action. No stale private container is reused.
The receipt and DB result are idempotent; if the root helper fails after some
removal, inspect and repeat only the same pending scope. Files are never freed
merely because the controller prepared a plan.

## Local validation and remaining evidence

```bash
# From web_demo:
go test -race ./internal/lifecycle ./cmd/controller
go vet ./internal/lifecycle ./cmd/controller
# From repository root:
python -m unittest discover -s infra/demo/tests/w3 -p 'test_*.py' -v
```

Local tests cover hash mismatch, lost abort responses, revocation failure,
process-lock collision/symlinks, offline advance warning, early cleanup denial,
admission during cleanup, generation advancement and exact receipt binding.
Unprivileged filesystem fixtures prove child symlinks do not erase outside
files and retained runtime references block cleanup. Actual root helper
execution against the Ubuntu fixed pool, UI notification delivery, restart and
native service identities remain separate W3/W7 acceptance gates.
