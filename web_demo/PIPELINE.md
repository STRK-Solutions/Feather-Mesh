# Dataset pipeline

The pipeline prepares exact approved source jobs privately, calls FEAM for every
validation/publication, and promotes a complete immutable provider snapshot only
after exact-hash approval. It has its own explicit SQLite initialization, byte
reservations and per-bundle writer exclusion. Runtime/browser inputs cannot
supply commands, executable paths, arbitrary hosts or mount paths.

`cmd/pipeline` accepts explicit operator actions `initialize`, `migrate`, `hash`,
`prepare`, `status`, `approve`, `promote`, `reconcile` and `serve`. Service startup
never initializes or migrates state. `prepare` is a bounded synchronous operator
worker; `serve` also processes the durable dashboard queue. Only queued jobs are
started automatically. Interrupted preparing/publishing jobs require reconciliation.
Migration 2 adds the terminal pruned state to the per-bundle writer index; existing
version-1 stores require an explicit `pipeline migrate --root ABSOLUTE_ROOT`.

From `web_demo/`, using the pinned Go toolchain:

```bash
go build -trimpath -o /tmp/feam-pipeline ./cmd/pipeline
go test -race ./internal/pipeline ./cmd/pipeline
go vet ./internal/pipeline ./cmd/pipeline
```

Install Python dependencies using the existing image's exact wheel/hash lock
on Linux amd64 CPython 3.12; other platforms need matching pinned native wheels:

```bash
python3.12 -m venv /tmp/feam-pipeline-venv
/tmp/feam-pipeline-venv/bin/python -m pip install --require-hashes \
  -r web_demo/importers/requirements-linux.lock
/tmp/feam-pipeline-venv/bin/python -m pip install --no-deps ./feather-mesh/python_sdk
/tmp/feam-pipeline-venv/bin/python -m unittest discover -s web_demo/importers/tests -v
```

The meaningful publication integration uses the actual FEAM binary, Python
converter and native readers, with no downloads or source credentials. Run from
`web_demo/`; all three variables are required when the tag is selected:

```bash
FEAM_PIPELINE_FEAM="$(pwd)/../feather-mesh/target/debug/mesh_cli" \
FEAM_PIPELINE_PYTHON=/tmp/feam-pipeline-venv/bin/python \
FEAM_PIPELINE_CONVERTER="$(pwd)/importers/convert.py" \
  go test -race -tags=pipeline_integration ./internal/pipeline
```

Tests create disposable private projects and do not establish host storage,
separate-identity or sandbox grant acceptance. Production admission retains
50 GiB staging, 100 GiB retained-release budgets and the dataset filesystem's
15% or 20 GiB free-space threshold, whichever is greater. It reserves the
complete candidate, every source's explicit byte ceiling and metadata headroom.
Tests use small temporary directories and explicitly disable the host percentage
threshold; the production command has no flag to do that.

The [closed job schema](schemas/dataset-job.schema.json) describes all required
fields. Go additionally validates cross-field relationships, duplicate keys,
station/date bounds, public source destinations and exact required field presence.
Archives are unsupported: file count and expansion limits are zero. Fetches
disable environment proxies and decompression, validate every redirect, reject
all private/reserved DNS answers, and dial a validated literal IP while preserving
HTTPS hostname verification. The converter has no network or executable job field.

`pipeline hash --job PRIVATE_JOB` prints SHA-256 of Go's canonical typed JSON.
Prepare only with that exact approved job hash:

```bash
pipeline initialize --root /datasets/pipeline --feam /usr/local/bin/feam \
  --python /opt/feam/pipeline/bin/python --converter /opt/feam/importers/convert.py
pipeline prepare --root /datasets/pipeline --job /private/job.json \
  --hash APPROVED_JOB_SHA256 --inbox /private/pinned-sources \
  --feam /usr/local/bin/feam --python /opt/feam/pipeline/bin/python \
  --converter /opt/feam/importers/convert.py
pipeline status --root /datasets/pipeline --id JOB_UUID
pipeline approve --root /datasets/pipeline --id JOB_UUID \
  --hash REVIEWED_CANDIDATE_SHA256 --actor APPROVING_ADMIN_UUID
pipeline promote --root /datasets/pipeline --id JOB_UUID
```

An omitted `--inbox` invokes the approved, checksum-pinned bounded fetch. Initial
acquisition of objects without known hashes is a separately reviewed operator
action; [the W4 source proposal](../docs/evaluations/web-demo/w4-source-proposal.md)
records the first approved scope. Raw bytes and reports remain outside `serving/`.

Every FEAM invocation is an argument array anchored to the explicit provider
project. Copies preserve previous manifest records/tombstones and use independent
file bytes. The pipeline never edits manifests, scans to infer a published
inventory or interprets file presence as registration. It rechecks registered
asset hashes and refuses any unregistered serving-tree file before approval.

`unknown`, `publishing` or `promoting` outcomes require `reconcile`; repeating
`prepare` never repeats `serve`. Manifest commit and release rename are separate
outcomes. Reconciliation compares exact manifests, source/output reports and
candidate digests; it can register a verified orphan after an already completed
rename without repeating that rename. Incomplete/ambiguous artifacts remain
private and reserved for operator investigation. No automated deletion of such
artifacts or grant-sensitive retained releases is implemented. Retention budgets
fail closed rather than deleting potentially assigned or uncertain bytes.

`serve --socket /run/feam/services/pipeline/control.sock --controller-uid 2103 --gateway-uid 2102` provides
controller/gateway read-only `POST /v1/releases/resolve` with `{ "bundle": "demo-climate", "digest":
"SHA256" }`, over the existing owner-only database. Kernel peer UID is required;
JSON actors and headers cannot authenticate. The response carries the exact
serving path, namespace/revision and approved disclosure/research policy. The
gateway/controller still owns live account grants and read-only mounting.

Raster publication now supports truthful intervals: `datetime: null` with
`start_datetime` and `end_datetime`. Instant records retain their previous
serialization. The January normal records both the 1991–2020 interval and January
aggregation semantics; no retrieval/modification date becomes an observation time.

The [MAC source-reader receipt](../docs/evaluations/web-demo/w4-mac-real-source-readers.json)
and [candidate review](../docs/evaluations/web-demo/w4-mac-candidate-review.json)
describe real private local outputs. They do not establish Ubuntu publication,
release approval or sandbox access. To rerun native readers on a private candidate:

```bash
python web_demo/importers/verify_bundle.py --provider PRIVATE_PROVIDER \
  --raw PRIVATE_SOURCE_DIRECTORY --job PRIVATE_JOB --output SANITIZED_RECEIPT \
  --executable ABSOLUTE_FEAM_BINARY
```

For the installed Linux web demo, pass `--web-demo-acl` to operator pipeline
actions and the service. The fixed deployment map is runner UID 2101, controller
UID 2103 and sandbox mapped UID 200999. Promotion seals payload files 0400 and
directories 0500, clears release extended ACLs, grants those identities traversal
on release ancestors, and grants mapped UID 200999 read/traverse only within the
verified `provider/serving` subtree. Raw inputs, draft metadata, `.feam` and
provenance stay private. The dataset mount ancestor must already allow traversal.
Missing `setfacl`, wrong ownership or ACL errors leave promotion unknown and
unassignable until reconciliation completes; no chmod-to-world-readable fallback
exists. Linux tagged integration additionally requires `acl` (`setfacl`/`getfacl`)
and checks serving versus private ACLs. Separate-identity sandbox reads/writes
remain a deployed acceptance gate.

The gateway adds audited administrator assignment/revocation at `/datasets/grant`.
Assignment validates the exact release against pipeline IPC. Grant CAS increments
auth and grant versions, pauses workspace admission, closes streams and persists
a reconfiguration outbox. Only a successful controller acknowledgment clears it;
stale acknowledgments cannot clear newer changes. A stopped controller snapshot
restores lifecycle admission after old mounts are removed. The participant image
reads the controller-owned `releases.json` at startup; `attach_releases.py` changes
only its marked peer section and refreshes via FEAM, preserving practice peers.

The same listener grants only the gateway UID access to `GET /v1/jobs`,
`GET /v1/jobs/{id}/review`, `POST /v1/jobs/plan`, `POST /v1/jobs/enqueue`,
`POST /v1/jobs/{id}/approve`, `/promote`, `/reconcile`,
`POST /v1/withdrawals/plan` and `POST /v1/releases/prune`. None accepts executable,
interpreter, staging, inbox or mount paths. The admin host checks current role,
account auth version, fresh Access identity and same-origin CSRF. Five-minute
signed review tokens bind actor, auth version, action, job ID and exact hash.
Source job review precedes enqueue; a separate page presents the complete candidate
inventory, bytes and hashes before approval and promotion. Failed/unknown replies
lead back to recorded status, never an automatic publication retry. Fresh fetches
also retain an actual download-time/hash/byte receipt independently of upstream
or prior acquisition timestamps.

An optional closed `withdrawal` job field contains exact `{id,version}` targets
and a reason. Normal jobs omit it and retain their prior canonical hashes. The
pipeline builds a withdrawal plan from the exact current parent, preserving its
source and disclosure policy. It clones the complete release and invokes FEAM
`withdraw` for each target. Approval and promotion bind the resulting complete
candidate. Promotion advances a monotonic revision floor and revokes old bundle
grants through the control authority before the successor becomes current.
Subsequent new versions preserve tombstones; publishing the withdrawn version
again is rejected by FEAM. Retained files are not edited in place, and prior
staged copies are not claimed to be remotely revocable.

Installed web pipeline invocations must include
`--control-socket /run/feam/services/gateway/control.sock` alongside `--web-demo-acl`.
The gateway's `pipeline_uid` is 2106, included in its read UID/socket ACL map.
The pipeline authenticates to `GET /v1/releases/pins` and
`POST /v1/grants/revoke-bundle`; the latter is allowed only to pipeline UID 2106
and rechecks an active admin actor. Revocation is idempotent, closes access via
auth-version rechecks and persists workspace reconfiguration. Pins include older
grants until pending mount changes and account revocations are acknowledged.

The candidate review offers explicit deletion only for a retained release.
Pruning requires its exact hash and a successful authoritative pin query, and
rejects current/revoked releases, withdrawal-protected bundles, active candidate
parents, missing authority and unknown/corrupt trees. It first moves the reviewed
release to private quarantine, then deletes and releases accounting only after
completion. Partial or uncertain deletion remains reserved for operator inspection.
There is no autonomous deletion, no world-readable fallback and no assumption
that unavailable grant authority means an empty pin set.
