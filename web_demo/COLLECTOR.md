# Structured research capture

`cmd/collector` implements the W6 private collection and archive boundary.
The [event record schema](schemas/event-record.schema.json) extends the W0
envelope with a closed, typed payload. Local automation establishes synthetic
Mac evidence; it does not establish deployed Ubuntu coverage, real R2 survival
after host deletion, participant eligibility or human review.

## Authority and capture

Each workspace gets its own Unix listener and short-lived `event` capability.
`POST /v1/events` verifies the workspace, generation, deployment, participant
mapping, current gateway authorization and capability before appending. It
accepts `client_reported` evidence only. There is no workspace query endpoint.
Policy maps internal account IDs to opaque participant IDs in a separate
owner-only file; it references existing consent and never requests repeat
consent. The policy controls eligibility, free-text permission and synthetic
status. Raw account/email mappings are absent from event storage.

The control socket uses kernel peer credentials. Controller UID 2103 can mint
and revoke event capabilities. Broker UID 2104 can send typed request, usage
and unknown-cost observations. The distinct configured verifier UID can submit
`independently_verified` evidence. It must independently check the authoritative
receipt/state before calling; possessing a receipt hash does not itself verify
an outcome. No automatically trusted participant outcome is implemented.
Research review/export/withdrawal requires explicit research operator UIDs,
separate from ordinary admin roles and these service UIDs. The reviewed owner
policy is 30 days, Saif as reviewer and Phase 2 owner; private support details
remain in the ignored deployment policy file.

The TUI reads optional `FEAM_EVENT_CONFIG`. With no config, ordinary local FEAM
continues with visible recording-off status. With capture configured, a bounded
background worker sends semantic events for requests, proposals, review
presentation/edit/approval/denial/cancellation, typed local outcomes and model
dispatch IDs. It does not capture keystrokes, terminal buffers, prompts,
arguments, private paths or hidden model reasoning. Outcomes remain
`client_reported`, including `committed_with_followup_error`. The hosted harness
emits a distinct UUID per model dispatch, and the broker receives that UUID in
`X-Request-ID` only through the scoped loopback TLS adapter.

The capture queue holds at most 32 events. Retries retain the exact event ID,
sequence and payload. Tokens are re-read for capability rotation. The TUI shows
connecting/unavailable/gap status and pauses new mutations/assistant requests
when capture fails; a review remains unapproved. Quit waits a bounded two
seconds for durable acknowledgments and reports an unacknowledged gap without
replaying an operation. Terminal restoration remains the TUI's responsibility.

Container config, supplied by the controller, is:

```json
{
  "socket": "/run/feam-events/events.sock",
  "capability_file": "/run/feam-config/event-capability",
  "deployment_id": "00000000-0000-4000-8000-000000000001",
  "workspace_id": "00000000-0000-4000-8000-000000000002",
  "generation": 1,
  "participant_id": "00000000-0000-4000-8000-000000000003",
  "software_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "profile_sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "dataset_sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
  "synthetic": true
}
```

These are synthetic placeholders. Bind only the selected workspace's socket
directory and config directory. Config/capability files must be regular files
with no other permissions or group write; `0640` permits the named ACL mask
needed by mapped UID 200999. The operator must verify that only the exact
mapped identity has read access. The TUI rejects symlinks. Set
`FEAM_EVENT_CONFIG=/run/feam-config/event.json`; never copy capabilities into
project contents or research records.

## Storage, redaction and archive

SQLite's immutable event rows are the authoritative structured append log;
derived ordering indexes can be rebuilt. An ACK follows the committed
transaction. Exact retries deduplicate; conflicting reuse and sequence
regression fail. Missing spans are marked and excluded from reviewed exports.
Limits are 64 KiB/event, 120 events and 1 MiB per participant/minute, a configured
local content budget up to 20 GiB and an archive content budget up to the
owner-approved 1 GiB. Filesystem/WAL overhead requires the host's separate disk
headroom and service limits. Disk-full/backpressure never yields a durable ACK.

Free text is omitted by default. When explicitly allowed, credentials, email
addresses, private paths and sensitive/multiline payloads are redacted before
persistence. Structured labels are bounded and reject sensitive patterns;
provider IDs are hashed. Operational billing remains in the broker ledger.

The Ubuntu demo selects `archive_mode: "local"` with the fixed directory
`/home/feam-service-data/traces/collector/archive`. Ansible creates it as
collector-owned `0700` only after checking the traces mount UUID. The runtime
validator and collector reject a changed directory, shared permissions, or a
different filesystem from the event database. The same 1 GiB archive content
quota applies. This archive survives ordinary service stop; a verified private
Mac copy is required before destructive teardown. R2 remains an explicit
`archive_mode: "r2"` option for later use, with separate private credentials.

Uploads run every five minutes, with immutable participant batches no larger
than 8 MiB. A pending database record precedes upload; SHA-256 readback verifies
each object before the persisted watermark advances. Interrupted trace and
export uploads resume the exact object. Shutdown flushes and checks all
watermarks. Withdrawal commits a local tombstone first, blocks further capture,
deletes trace and derived export copies, and verifies the remote deletion and
lineage tombstone. A failed deletion remains pending. Thirty-day expiry removes
only previously archive-verified copies. Both modes use the same local deletion
lineage and fresh archive readback checks.

`teardown-check` fails if any accepted event, pending export or deletion lacks
verification. The operator must run successful flush/check before deleting
disposable compute. Before destructive teardown, the operator must also verify
the independent Mac copy of retained objects and the private provenance and
deletion ledger. Same-host archive verification alone cannot prove survival of
host loss.

## Operator commands

Build with the pinned Go version from `web_demo/`:

```bash
go build -trimpath -o /tmp/feam-collector ./cmd/collector
go test -race ./internal/collector ./cmd/collector ./cmd/capture-fixture
go vet ./internal/collector ./cmd/collector ./cmd/capture-fixture
```

The private owner-only collector JSON contains `db`, `capabilities_db`,
`control_socket`, `gateway_socket`, `workspace_sockets` (UUID to absolute socket
path), `broker_uid`, `controller_uid`, `verifier_uid`, `reviewer_uids`, `reviewer`,
`policy_file`, `archive_mode`, `archive_directory` (local mode),
`r2_credentials_file`, `r2_account_id`, `r2_bucket` (R2 mode),
`local_max_bytes` and `archive_max_bytes`. Service UIDs and reviewer UIDs must
be distinct. The R2 credential JSON has `access_key_id` and
`secret_access_key`; omit all R2 fields in local mode. The policy JSON is an array of
`{account_id,participant_id,consent_reference,eligible,allow_text,synthetic}`.
No real identities or credentials belong in checked-in examples.

```bash
/tmp/feam-collector -mode initialize -config /etc/feam/collector.json
/tmp/feam-collector -mode initialize-archive-ledger -config /etc/feam/collector.json
/tmp/feam-collector -mode serve -config /etc/feam/collector.json
/tmp/feam-collector -mode recover-index -config /etc/feam/collector.json
/tmp/feam-collector -mode flush -config /etc/feam/collector.json
/tmp/feam-collector -mode teardown-check -config /etc/feam/collector.json
```

Initialize once under the collector service identity. Startup opens existing
databases and fails closed on missing/schema-drift state; migrations use the
separate `migrate` mode. Stop the service before offline index recovery.
Reviewer-only `POST /v1/research/export` receives
`{participant_id,split,reviews:[{event_id,event_sha256}]}`. The reviewer inspects
the private stored event/hash pairs and chooses exact records; successful
labels require independent verified receipt evidence. Negative examples retain
their provenance. Each event needs task-template and dataset-family labels.
Participant, dataset family and task template cannot cross train/held-out
splits. Exports carry source hashes, reviewer, purpose and 30-day expiry.
`POST /v1/research/withdraw` takes `{participant_id}`; `POST /v1/archive/flush`
requires the same restricted reviewer peer identity. None is exposed by the
ordinary admin dashboard.

## Reproducible synthetic checks

From the Rust workspace, after building the hosted CLI and terminal probe:

```bash
cargo build -p mesh_cli --features agent-hosted
cargo build -p mesh_tui --features test-driver --example terminal_probe
python -m unittest discover -s scripts -p 'test_tui*.py'
python scripts/tui_walkthrough.py --mode manual --output /tmp/capture-manual.json
python scripts/tui_walkthrough.py --mode fake --output /tmp/capture-fake.json
```

The Python environment needs the existing walkthrough SDK/pyte dependencies.
Use new output filenames to retain earlier evidence. `test_tui_capture.py`
drives the actual PTY and a local Unix fixture through review edits, denial,
approval, typed outcomes, gap backpressure and terminal restoration. It checks
that entered private text and filenames never enter semantic events.

From `web_demo/`, create a new clearly marked development export:

```bash
go run ./cmd/capture-fixture -output /tmp/synthetic-reviewed-export.json
```

The checked-in [synthetic export](testdata/synthetic-reviewed-export.json)
contains one synthetic verified success and one negative example, reviewed by
`automated-synthetic-fixture`. It is not a claim of human review, participant
data or actual provider/Ubuntu execution. No training starts here.

Remaining Ubuntu evidence: actual processes/ACLs, a synthetic capture through
the live collector socket, local flush/readback, reviewer denial, and a verified
private Mac copy. Real R2 retrieval and research corpus preparation remain
deferred. Keep W6.06/W6.G pending until their separate evidence exists.

For the operator's final drain gate, `collector --config PRIVATE --mode
archive-verify --include-inventory` performs fresh GET/hash checks of every
recorded retained batch, export and deletion tombstone, and fresh absence
checks for deleted objects. It refuses pending capture/archive state and holds
a SQLite write reservation during verification, preventing a concurrent
collector from changing the ledger. Run after draining capture. The 60-second
deadline and 768 KiB inventory limit fail closed; retain the host on failure.
The JSON receipt uses `feam.archive-verification.v1`; its aggregate SHA256
binds each ordered inventory object's compact JSON plus one newline. Object
fields are `kind`, `object_key`, `sha256`, `state`, in that order; objects are
sorted by object key. No event bodies or identity mapping are included. A
bounded complete archive listing also
rejects unknown, omitted or changed objects; the collector preserves unknown
keys for operator investigation. The archive has one active writer, which must
be drained before this check. This is a point-in-time proof, not protection
against subsequent external writes. In R2 mode, `ListObjectsV2` pagination
uses its opaque continuation token and a fixed prefix.
[R2 S3 compatibility](https://developers.cloudflare.com/r2/api/s3/api/),
[ListObjectsV2](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html).

Before deleting compute, separately seal `export-archive-ledger` output in the
encrypted operator vault. The metadata baseline carries byte reservations,
original retention deadlines, deletion floors and held-out split assignments
without restoring event content. A fresh collector refuses capture until an
explicit verified baseline has been initialized or imported. Follow the
[archive carryover runbook](ARCHIVE_LEDGER.md); the smaller verification
receipt alone is insufficient for recreation.
