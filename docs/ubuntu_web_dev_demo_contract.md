# Web demo W0 implementation contract

Version: `feam.web.v1`, selected 2026-09-25. This defines W1–W7 interfaces;
it does not claim that the services, databases or security controls exist.
The [design](ubuntu_web_dev_demo_design.md) governs architecture and the
[workplan](ubuntu_web_dev_demo_workplan.md) governs delivery/evidence.
Machine-readable records live in [schemas](../web_demo/schemas/).

## Implementation and process ownership

Use one Go module under `web_demo/` for gateway, controller, broker, collector,
account reconciler and pipeline orchestration. Use standard `net/http`,
`html/template`, `encoding/json` and `net` where adequate. Select and lock
maintained JWT, WebSocket and SQLite libraries when their consuming component
arrives; do not implement cryptography or a WebSocket parser here.
Use Python 3.12/3.13 for bounded climate conversion with the supported FEAM SDK,
Polars and Rasterio. Python runs under the pipeline's identity with fixed
argument arrays. FEAM Rust services/CLI remain the only publication/resolution
authority. No new Rust crate or parallel FEAM business implementation is needed.

Go entrypoints will live under `cmd/{gateway,controller,broker,collector,
reconciler,pipeline}` with shared records in `internal/`. W0 introduces schemas
and validation only; runnable servers arrive in their named workplan phases.

| Process / OS identity | Exclusive authority | Permitted inputs and boundaries |
| --- | --- | --- |
| gateway / `feam-gateway` | Identity, account/grant control DB, browser authentication, role/ownership checks and admin UI | Validated Access identities; typed controller/broker/reconciler/pipeline commands. No Docker, upstream key, sudo, Terraform or dataset write permission. |
| controller / `feam-controller` | Lifecycle/slot DB and access to the dedicated runner's daemon | Fixed workspace actions and server-derived templates. Runtime socket ACL permits this identity only; giving it that socket grants control over runner-owned resources and must be tested as such. |
| runtime / `feam-runner` | Dedicated rootless Docker data and user service | No personal daemon, host root, pipeline or research storage; no autonomous container restart policy. |
| broker / `feam-broker` | Upstream credential, run allocation and request ledger | Per-workspace scoped sockets/capabilities; fixed route/profile; requests bounded by policy, disclosure and budget. |
| collector / `feam-collector` | Redacted event log/index, restricted research archive | Per-workspace submission; no runtime socket or model key. Research export permission is separate from general admin. |
| pipeline / `feam-pipeline` | Candidate datasets and FEAM publication within approved provider trees | Exact-hash approved job definitions, fixed allowlisted importers, bounded subprocess arguments. No arbitrary browser command/path/URL. |
| reconciler / `feam-reconciler` | Exact-email membership of the two recorded Access groups | Current control policy plus scoped membership credential. No Terraform ownership of the same groups. |
| operator, outside demo compute | Deployment records, project ledger, Terraform state, private policy/roster and approved release catalog | Single writer; reviewed IaC. Never a website capability. |

Separate OS users and socket ACLs enforce the boundary even when binaries share
code. Each DB has one owning service. Other services query the owner's typed
endpoint; they do not open its SQLite file. Operation logs/state can refer to
control IDs without cross-database foreign keys. Authority checks fail closed
when their owner is unavailable. Authorization caches expire within 5 seconds;
active streams recheck at least every 5 seconds and must close within 30 seconds
of disable. Revocation push is an optimization, not the only invalidation path.

## Local persistence and migrations

All IDs are opaque lowercase UUIDs; timestamps are RFC3339 UTC. Email is never
a path or resource key. Money is integer USD microdollars (`100000000` = $100),
rounded upward when reserving; never floating point. Digests are SHA-256 of
canonical approved bytes. Generations and authorization versions are positive
monotonic integers and are checked inside the mutation transaction.

Each owner's DB uses SQLite on its verified local filesystem, foreign keys on,
WAL and full synchronous durability, bounded busy timeout and short write
transactions. No shared-network SQLite. Each migration has a version and hash
recorded in `schema_migrations(version PRIMARY KEY, sha256, applied_at)`.
Explicit operator `initialize`, `migrate` and `restore` are distinct future
entrypoints. Service startup checks an existing database/schema; it never
creates a missing DB, migrates automatically, or replenishes a budget.

| Owner / tables | Minimum records and constraints |
| --- | --- |
| gateway: `accounts` | `id PK`, unique exact canonical verified email and provider subject; role `user/admin`, status `pending/active/disabled`, `auth_version`, created/updated time. Enrollment is invited; no implicit admin workspace. |
| gateway: `grants`, `release_assignments` | Grant ID, account/audience, bundle, immutable release digest, `grant_version`, status/effective time, actor; uniqueness of live account/bundle assignment. Current revocation overrides restored grants. |
| gateway: `policy_jobs`, `audit` | Job/action/hash/idempotency, requested policy revision, edge sync state/error; audit actor/target/correlation/time/outcome without secrets or raw prompts. Account creation is pending until required edge/workspace setup succeeds. |
| controller: `workspaces` | ID PK, unique owner and hostname, deployment, desired state, observed state, generation, approved image/profile, assignment version, slot/socket IDs, activity time. States: stopped/starting/running/stopping/resetting/failed/pending-delete. |
| controller: `slots`, `lifecycle_jobs` | Slot ID PK, fixed backing-file identity and filesystem UUID, generation/assignment, size; unique active slot assignment. Job ID PK, authenticated caller, request hash, unique scoped idempotency key, expected generation, action, state and outcome. Unique live mutation per workspace. |
| pipeline: `dataset_jobs`, `releases` | Job ID/hash, source pins, byte reservation, namespace, parent revision, candidate hash, approval hash, manifest commit outcome, promotion outcome, current withdrawal floor. Unique bundle/version; serialize publication per bundle. |
| broker: `run_allocations`, `requests` | Allocation ID PK and signed/approved document hash; deployment/generation/profile/price ceiling, amount, state. Request ID PK, unique account/idempotency key, request hash, auth/workspace generation, worst-case reservation, provider ID, known cost or unknown amount, dispatch/reconciliation timestamps. |
| collector: `events`, `batches`, `exports`, `deletions` | Event ID PK, stream/sequence unique, pseudonym, run/workspace/conversation/request/proposal IDs, provenance/trust; payload hash/schema; batch hash/upload watermark; export reviewer/lineage/held-out policy; withdrawal/deletion tombstones and archive reconciliation. Mapping to real identity is separate and restricted. |

DB restore requires a consistent backup and explicit operator action with current
authorization/withdrawal state and conservative spend reconciliation. Old policy
or budget data cannot overwrite newer restrictions or charges. Compatible
binary rollback may reuse the current schema; incompatible downgrade must stop.
Fresh deployment uses current operator policy and approved seeds; no live process
or prior workspace restore is promised.

## Private IPC v1

Control traffic uses HTTP/1.1 with UTF-8 JSON over filesystem Unix sockets;
not public TCP. Maximum control body 64 KiB, read-header timeout 5 seconds,
request timeout 10 seconds. Long operations return a durable job ID and are
polled; they do not hold a mutation request open. Streaming inference uses the
separate broker socket: bounded JSON request up to 1 MiB, SSE responses, explicit
cancellation, 120-second maximum lifetime, no automatic replay. Event submission
is at most 64 KiB/event and 1 MiB/batch with per-workspace rate/byte admission.
Final limits can only change through reviewed settings/profile revisions.

System sockets live under `/run/feam/` in service-specific directories. Workspace
terminal/model/event directories are separate bounded mounts. Authenticate
system peers with Linux `SO_PEERCRED` plus the installed UID/ACL map. A body field
claiming an actor is not authentication. Sandbox UIDs cannot identify an account
by themselves: bind endpoint capability, deployment/workspace generation and
active account together. Store only capability hashes in records; rotate/revoke
on disable, reset and activation change. No credential is in the schemas' public
examples or persisted research payloads.

Control request envelope: protocol, request ID, idempotency key, deployment ID,
activation generation, workspace ID/expected generation when applicable,
server-authenticated actor/account and authorization version, fixed method and
typed parameters. Unknown fields, versions and methods fail validation.
The [lifecycle request schema](../web_demo/schemas/lifecycle-request.schema.json)
freezes the initial controller method set. It intentionally cannot accept host
paths, commands, mount specifications or image choices.

| Endpoint owner | Initial methods / authorization |
| --- | --- |
| controller | workspace status, start, stop, reset and delete; self-service ownership or specifically audited admin policy, expected generation for all mutations. Reset/delete need fresh confirmation bound to exact request hash. |
| gateway control API | Read active account/version and grant/release snapshots for authenticated services. A stale snapshot cannot authorize dispatch/start. |
| broker | Submit/cancel request, query durable result and redacted usage. Capability grants one active workspace only; admin budget increase travels through operator project-ledger workflow. |
| collector | Append bounded structured events; ACK durable event/sequence only. Separate restricted export/deletion endpoint; sandbox cannot list/read history. |
| pipeline | Prepare approved job, inspect candidate, approve exact hash, promote, reconcile; no generic process executor. |
| reconciler | Reconcile exact recorded policy revision, inspect status; no caller-defined Access group ID or broad domain allowlist. |

Responses carry protocol/request ID, `completed`, `accepted`, `rejected` or
`unknown` status, durable job/result ID and typed error when applicable. Error
codes include unauthenticated, forbidden, stale_generation, stale_authorization,
conflict, invalid_request, capacity, budget_exhausted, unavailable and
reconciliation_required. Bound/sanitize details; never pass through provider
bodies, private paths, credentials or terminal contents. HTTP 400/401/403/409/
429/503 describe admission failures; a lost transport does not describe the
mutation's outcome. Exact response DTOs are added with each endpoint before use.

Record idempotency key + canonical request hash before external effects. Reusing
the same key/hash returns the recorded job/result; a changed hash conflicts.
After timeout/crash, query that ID and reconcile recorded labels, generation,
provider IDs or manifest outcome. Never repeat an uncertain billable dispatch,
publication, reset or promotion. FEAM `committed_with_followup_error` remains a
committed publication with reconciliation work, not a failed safe-to-retry call.
No local TUI review approval crosses a process restart or new conversation.

## Operator deployment and budget records

Operator files are private, encrypted at rest, single-writer locked, atomically
replaced and fsynced with an append-only audit. They live outside disposable
compute. Missing/corrupt files stop activation; initializing an empty ledger is
never recovery. JSON examples/schemas describe records, not a production ledger
implementation. Authentication/signing and atomic writer are W5/W7 work.

Deployment record fields: schema/protocol, deployment and run IDs, monotonic
activation generation, site, desired running/stopped, lifecycle status, exact
release/config/seed hashes, private inventory reference, actual mount UUID,
policy revision, allocation ID, selected-route ownership, credential references,
archive watermark and last operator action. Secrets are file/key-store references
only; Terraform state is separately restricted. `running` is intent, not proof
of readiness. Public routing remains disabled until all readiness gates pass.

Project ledger contains the current ceiling, attributable settled charges,
revision and bounded run reservations. The invariant is:

`settled_usd_micros + sum(outstanding_usd_micros) <= allowance_usd_micros`.

Allocate under one lock before issuing the run document; never launch before
the reservation is durable. Within a run:

`run_settled + request_reserved + request_unknown + next_worst_case <= run_amount`.

Do not count local run charges a second time as project settled charges until
closing/reconciling that allocation transaction. An active or lost/unknown run
retains its full issued amount at project level. Verified closure atomically
adds actual cost to project settled and releases the allocation, exactly once
under its unique ID. Missing provider usage leaves request reservations charged;
inaccessible/uncertain runs retain the full outstanding allocation indefinitely
until operator reconciliation. No monthly reset, key rotation, workspace reset,
teardown or site switch replenishes the allowance. A run cannot increase its own
allocation. An authenticated admin ceiling increase is a durable operator-ledger
action before another allocation is issued; the website cannot rewrite a local
ceiling and bypass this invariant. Lowering below committed exposure is rejected.

Run allocation documents bind project/run/deployment/activation IDs, amount,
ledger revision, approved model/profile and bounded validity. The broker checks
the operator-authenticated document plus its local persisted state; copying a
document into a new deployment/generation is invalid. Requests recheck current
account/grants/capability at dispatch, reserve before sending bytes and never
retry automatically. Provider key limits are secondary safeguards.

One operator-selected public deployment is allowed. Start records desired
running, initializes a new identified deployment only on explicit fresh start,
verifies mounts/current policy/allocation, then admits private health tests and
selects the public route. Ordinary boot uses that desired state and existing
local records. Stop persists desired stopped first, denies new admission,
drains bounded operations, reconciles spend and flushes the archive. A failed
archive can stop compute but blocks destruction of its only trace/export copy.
Teardown requires verified archival and exact deployment ownership, revokes
credentials and deletes only that disposable scope. Unknown charges remain
reserved even after resources are deleted.

Switch: stop/drain old host if reachable, revoke its public tunnel, membership
writer and upstream model credential, verify revocation, reserve a new allocation
from remaining funds, seed a fresh host with current policy, then select its
route. An unreachable old host does not justify simultaneous activation: confirm
external revocation first and keep its allocation reserved. Route/account/key
checks failing leaves the new host stopped. No distributed lease, recovery
runner, automatic replacement or workspace replication is introduced.

## Settings, examples and approval inputs

[settings.schema.json](../web_demo/schemas/settings.schema.json) is a closed,
versioned configuration contract. Defaults are explicit in the sanitized
[example](../infra/demo/examples/settings.json); validation never fills missing
policy, secret, UUID or accounting state. Test mode cannot enable public
admission or billable inference. Production requires exact issuer/audiences,
approved immutable artifacts, configured retention/reviewers and current inputs;
W0's placeholder examples are structurally valid, not deployment-ready.

Private inventory uses SSH strict host checking, no production credentials in
CI, and Vault-encrypted secrets/roster transferred deliberately from the ignored
local source. Existing consent remains recorded; do not invent another consent
gate. [Input tracking](../infra/demo/README.md#inputs) names I1–I7 and later gates.
No W0 example authorizes live traffic, public exposure or a paid purchase.
