# Explicit operator lifecycle

[site_lifecycle.py](scripts/site_lifecycle.py) runs on the durable operator Mac.
It operates only the six reviewed private services on one pinned SSH target.
It never creates compute, selects a public route, issues spending allocations,
raises a ceiling, or deletes containers, volumes or Terraform resources. A
teardown preparation always has `destruction_authorized: false`.

Every potentially mutating step is recorded through
[operator_vault.py](scripts/operator_vault.py) before execution. A single-writer
lock, expected revision, atomic encrypted replacement and retained prior revisions
preserve uncertainty. Ordinary lifecycle commands have individual deadlines, a
ten-minute overall bound and a 1 MiB private-output limit; archive transfer has
its own 30-minute total deadline and streamed byte verification. SSH uses a specific key and strict pinned
host checking. Raw configuration, archive inventories and service errors are not
printed. Local tests are synthetic; actual Ubuntu, R2 and cloud acceptance remain
separate gates.

## Private record

Add `site_lifecycle` to the existing encrypted operator configuration, preserving
its other namespaces. Its closed fields are:

| Field | Required value |
| --- | --- |
| `protocol` | `feam.site-lifecycle.v1` |
| `project_id`, `deployment_id`, `run_id` | Current UUIDs |
| `activation_generation`, `site_id`, `actor` | Positive generation, short site label, local operator audit label |
| `workspace_ids` | Exact provisioned workspace UUIDs |
| `expected` | `release_sha256`, `runtime_manifest_sha256`, `policy_sha256`, `seed_sha256`, `allocation_id`, `allocation_sha256` |
| `ssh` | `host`, `user: feam-deploy`, `port`, absolute `identity_file`, absolute `known_hosts` |
| `budget` | Absolute pinned `binary`, its `binary_sha256`, durable encrypted `ledger`, and separate `encryption_key` reference |
| `state` | Initially `{"desired":"stopped","phase":"ready"}` only for an explicitly prepared fresh activation |

The runtime digest binds exact `/etc/feam/services/runtime.json` bytes; the policy
digest binds `/etc/feam/services/collector/policy.json`. The runtime manifest binds
all service configurations and native artifacts. The allocation digest is Go
`SignedAllocation.Digest()`, not the hash of pretty-printed JSON. The project CLI's
fixed compact struct serialization supplies the same bytes. The seed digest
records the reviewed revision; controller/pipeline release validation remains
authoritative for actual assignments.

Use an owner-only copy of independently verified known-hosts entries in the
private operator directory. A normal mode-0644 `~/.ssh/known_hosts` is deliberately
not accepted. Keys, inventory, pinned artifacts, vault and project ledger remain
outside images/Git/logs. Use the locked Python environment with `cryptography`,
PyYAML and Ansible. Previous runs require `state.archive_ledger`; any prior unclosed
allocation on another deployment also requires its reviewed external-revocation
record under `state.prior_revocations[OLD_DEPLOYMENT_UUID]`.

## Start and bounded stop

For initial Ubuntu provisioning, pass `feam_services_controller_slots` in the
private services variables as an exact path-to-filesystem-UUID map for the
selected workspace slots and two spares. The services role checks each path,
owner UID 200999 and mounted UUID before granting only `feam-controller`
read/traverse access. On the first staged deployment, slots 03–12 were selected;
occupied W1 slots 01–02 were excluded. The reusable role accepts slots 01–12
once individually reviewed and unoccupied.

Initialize the new gateway before bootstrap, import only the approved staging
roster, and read back its generated account UUIDs before rendering the final
controller and collector policies. Initialize the controller only after binding
those exact owners. The first controller assignments do not queue gateway
snapshots automatically: publish each actual controller workspace through the
existing gateway `/v1/workspaces/snapshot` IPC as UID 2103, with `ready` equal
to whether its state is not `deleted`. This marks a stopped but valid assignment
ready; it does not claim a running terminal. The scoped reconciler can then
activate eligible staged users once the Access group is current. Derive all
snapshot fields from the controller's current record and verify gateway
readback; direct SQL or a forged browser identity is not part of this flow.

For a fresh collector database with an empty verified local archive, run
`collector -mode initialize-archive-ledger` exactly once before starting its
service. A previous deployment instead requires the reviewed archive ledger
import; ordinary `collector -mode initialize` alone deliberately cannot serve.
Preserve any existing pipeline database and promoted release during service
converge.

If the rootless Docker daemon was already running before `run-feam.mount`,
RootlessKit's private copy of `/run` can hide the later FEAM tmpfs even though
the host and `feam-runner` can see it. The services role runs the fixed
[`rootless_run_mount.py`](scripts/rootless_run_mount.py) helper after starting
the mount. It verifies the 32 MiB root-owned tmpfs, the single UID 2101 daemon,
its exact mount namespace and an absent or identical target. When needed, it
clones only that host mount as a detached mount and attaches it at `/run/feam`
inside the existing daemon namespace. A repeated invocation reports
`already_visible`; a foreign mount fails closed. Future boot ordering requires
`run-feam.mount` before the user manager, so the live repair does not require a
Docker or W1 restart. Verify the actual W1 container identities and states after
any one-time repair.

An accepted workspace start that becomes `unknown` must never be retried with
the same request. Inspect the exact runtime container and job; if no container
exists, stop only `feam-controller.service` to take its exclusive process lock,
run `controller -action recovery-review` for the job, then use `-action
abort-job` with that review's exact `request_sha256`. Recheck that the job is
durably failed and the workspace generation advanced, restart the controller,
and submit a fresh workspace request only after the runtime mount is verified.
Keep the review, abort and readback in the private operator evidence.

Use existing private files and the current vault `REVISION`. Each successful
operation can advance the revision several times.

```sh
python infra/demo/scripts/site_lifecycle.py status --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION
python infra/demo/scripts/site_lifecycle.py start --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION
python infra/demo/scripts/site_lifecycle.py stop --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION
```

Start requires an existing finite active allocation, exact release/policy/mount
checks, settled stopped controller state, recorded broker readiness and any
required archive baseline. It enables the private services and controller intent;
it does not launch workspaces or enable public routing. An uncertain start cannot
be replayed. Inspect it privately, then explicitly stop it if needed.

Stop persists desired stopped on the Mac before contacting the host. It persists
host stop intent, drains inference, retains the entire project run as unknown,
waits for lifecycle completion and stopped containers, disables/stops writers and
the health timer, flushes the collector, freshly verifies archived objects, and
seals its private verification inventory and metadata ledger. The protected
Ubuntu local archive remains on its verified traces filesystem after ordinary
stop; this operation needs no continuous Mac connection. No raw event bodies
or account mapping are copied by stop. Intentional stopped state and disabled services
survive reboot. Unknown costs cannot replenish spending.

A failed archive preserves its source and blocks teardown preparation. An
unreachable host leaves stopped intent and uncertainty recorded; it does not
authorize a replacement. Resume an interrupted stop only by its exact operation ID:

```sh
python infra/demo/scripts/site_lifecycle.py stop --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION --resume-operation RECORDED_OPERATION_UUID
```

Completed steps are not repeated. An uncertain stop step can retry under that
explicit acknowledgment because it is a desired-state assignment, fixed service
stop, idempotent drain or content-addressed archive operation. Provider requests,
workspace start/reset and spending allocations are never replayed.

## Verified private Mac archive copy

After a verified stop, create a new copy under an existing owner-only Mac
directory. The operator command uses the pinned SSH identity to stream the
collector's fixed local archive as `feam-collector`, accepts only expected regular
objects, and checks every byte against the fresh collector inventory and
provenance/deletion ledger. It stores those metadata files beside the objects,
with owner-only permissions, and records the copy receipt in the encrypted vault.
Use a new output directory for each copy; keep the previous copy until the new
one and any withdrawal/expiry maintenance have been verified. A transfer has a
30-minute total deadline; an interrupted transfer leaves the Ubuntu source intact.

```sh
python infra/demo/scripts/site_lifecycle.py copy-archive --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION --output-directory /operator/feam/research/deployment-RUN
python infra/demo/scripts/archive_copy.py verify --directory /operator/feam/research/deployment-RUN
```

The command uses the reviewed root sudo path and drops to `feam-collector`
through `/usr/sbin/runuser`; the deployment account has no direct `sudo -u`
grant on the verified Ubuntu host. It requires inactive services/containers and exact collector
`archive_mode: local` with `archive_directory:
/home/feam-service-data/traces/collector/archive`. A failed transfer leaves its
source intact and an inspectable partial Mac directory; use a fresh destination
for retry. On a later copy, the command applies freshly verified withdrawals and
expiry to every prior retained copy. Maintenance records deletion intent before
removing bytes; rerun the same operation to finish an interrupted update. After
Ubuntu teardown, run the offline
expiry operation against every retained Mac copy at least daily:

```sh
python infra/demo/scripts/archive_copy.py expire --directory /operator/feam/research/deployment-RUN
```

New withdrawals after host teardown require a reviewed provenance/deletion
update and maintenance of every retained copy before any research use. Do not
discard the sole verified copy while applying withdrawal or expiry. A same-host
archive alone is not host-loss protection; that begins after the Mac copy is
verified. `prepare-teardown` freshly rechecks the Ubuntu archive and the complete
Mac copy against the same inventory and ledger, then seals their proof hashes.

## Separate synthetic-to-live handoff

The staged synthetic run and the approved live smoke use **different project
ledgers and run IDs**. Do not use `prepare-next-run` for that switch: it is
limited to a new run within the *same* project. Finish the browser synthetic
capture first, stop it through the ordinary lifecycle, and verify the retained
Ubuntu archive plus an independent private Mac copy. Keep the stopped fake
broker run database and its WAL/SHM companions, signed allocation, verification
key, drain receipt and operator vault revision for audit. The renderer also
places owner-only copies of the fake allocation and verification key beside the
new proposal before the active broker files are replaced.

For the live proposal, use a fresh owner-only input file with
`broker_mode: live`, the actual primary project, distinct live run and
allocation IDs, signed real allocation, primary 32-byte verification key and
private OpenRouter key. The owner-only pending input is
`.local/demo-deployment/functional-proposal/u04-render-input-live-pending.json`;
its provider-key source is staged privately, while the real allocation source
remains absent until the already-approved finite allocation is issued from the
primary project ledger after the private/browser prerequisites. Render into a
new private directory after that issuance, review its hashes, then install with all six services
stopped and public routing still controlled separately. The live manifest uses
`broker_mode: live`, a separate broker run/capability/receipt path, real
provider key owned only by `feam-broker`, and `synthetic: false` for controller
endpoints and collector policy. The existing approved participant consent
reference remains the policy lineage; switching capture mode does not ask for
repeat consent. The renderer leaves start/enable false and selects only
the new broker databases for one-time initialization. The services role checks
both exact new database paths and their `-wal`/`-shm` companions are absent
before invoking that initialization;
if it fails partway, inspect the partial state rather than retrying. After a
successful one-time broker initialization, use the generated
`service-vars-post-init.json` for every repeat converge; it clears the
initialize selection while retaining the same configuration hashes.

Existing workspace `event.json` files were generated with the synthetic
marker. After the reviewed live config is installed, normally stop and freshly
start each workspace under the new controller endpoint, verify each regenerated
`event.json` has `synthetic: false`, and only then admit the first live request.
Carry the verified local archive ledger and every retained Mac copy into a
separate live lifecycle record bound to the primary project ledger. The owner
remains the sole writer of that ledger and the primary operator vault.

## A new run on the same stopped host

A deliberately drained run remains paused. First reconcile independently verified
prior charges if possible, or retain the entire old allocation. Then explicitly
issue a new finite signed allocation using [project-budget](../../web_demo/BROKER.md)
with new allocation/run UUIDs and the same project/deployment/activation. Only
remaining project funds can be allocated; the US$100 ledger is never reinitialized.

Create an empty canonical mode-0700 output directory, then prepare concrete inputs:

```sh
python infra/demo/scripts/site_lifecycle.py prepare-next-run --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION --evidence /operator/feam/next-signed-allocation.json --service-vars /operator/feam/current-service-vars.yml --output-directory /operator/feam/next-run
```

This requires the new reservation already in the original ledger and writes
private `broker.json`, `allocation.json`, `runtime-manifest.json`,
`service-vars.json`, and `operator-next.json`. It preserves unrelated operator
namespaces and the archive baseline. New broker ledger/capability/receipt filenames
contain the new run UUID; old files remain untouched. It neither applies host
changes nor changes the vault, and refuses to overwrite prior proposal files.

Review the paths and hashes, apply the generated full private variables, then
verify the resulting root manifest matches the proposed hash:

```sh
ansible-playbook -i .local/demo-deployment/inventory.yml infra/demo/ansible/services.yml -e @/operator/feam/next-run/service-vars.json
python infra/demo/scripts/operator_vault.py seal --key /operator/feam/operator.key --input /operator/feam/next-run/operator-next.json --output /operator/feam/operator.enc --expected-revision REVISION
```

Only the new broker databases are initialized; application start/enable remain
false. Manifest bytes are tested against Ansible's serialization. Seal only after
successful reviewed application at the unchanged operator revision. Start with
the new vault revision; actual hashes and budget are checked again. A missing or
partially initialized DB fails startup, never authorizes overwriting old state.
Remove the initialization list from later ordinary-converge input.

## Fresh-host archive carryover

Stop/preparation retain `state.archive_ledger` from collector
`export-archive-ledger`. It includes object hashes/sizes/pseudonymous lineage,
withdrawal tombstones, expiration metadata and held-out split assignments, without
event bodies. Preserve it encrypted; do not create a fresh accounting universe
when replacing compute.

For an explicitly fresh initialized collector, extract the baseline into a
mode-0600 private file without printing it, install it as an explicit collector
private file, then run as `feam-collector` before serving:

```sh
/opt/feam/services/RELEASE_SHA/collector -config /etc/feam/services/collector/config.json -mode import-archive-ledger -ledger /etc/feam/services/collector/archive-ledger.json -confirm-ledger-sha256 EXACT_PRIVATE_FILE_SHA256
```

Import binds the reviewed file hash, requires empty initialized state and freshly
checks the archive before atomically carrying metadata forward. Old objects count
against archive quota and preserve deletion/retention behavior. Lifecycle start
exports the current ledger and rejects missing previous objects, tombstones or
split assignments. Unknown archive state remains a reconciliation gate.

## Closure and preparation receipts

Budget closure needs a private `feam.provider-cost-review.v1` record with
`allocation_id`, `allocation_sha256`, `broker_receipt_sha256`, integer
`verified_cost_usd_micros`, and `provider_evidence_sha256`. Bind it to the retained
broker receipt after independent provider-cost review. A broker-observed charge
alone is not independent evidence; conflicting closure is rejected.

```sh
python infra/demo/scripts/site_lifecycle.py reconcile --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION --evidence /operator/feam/provider-cost-review.json
```

Teardown preparation additionally requires a `feam.external-revocation.v1` record
with `deployment_id`, `activation_generation`, `site_id`, `route_revoked: true`,
`membership_writer_revoked: true`, `model_credential_revoked: true`, and
`evidence_sha256`. These are owner-reviewed external checks. The script validates
the evidence structure/binding; it does not call Cloudflare/OpenRouter revocation
APIs or independently verify invoices. Never attest uncertain checks as true.

```sh
python infra/demo/scripts/site_lifecycle.py prepare-teardown --vault /operator/feam/operator.enc --key /operator/feam/operator.key --expected-revision REVISION --evidence /operator/feam/revocations.json
```

This repeats stopped/disabled service and container checks, fresh archive readback,
ledger checks and full Mac copy verification, then seals proof hashes. Failed
re-verification invalidates an
older preparation. Unknown allocations remain held after any later approved
disposal. Actual deletion requires separate exact-scope approval and tooling;
research archives, durable edge resources and operator state are never targets.
