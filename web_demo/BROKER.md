# Private inference broker and accounting

The broker fixes `deepseek/deepseek-v4.1-flash` to `deepinfra/fp8`, denies
provider fallbacks and automatic retries, and omits `parallel_tool_calls`.
The embedded [tools](internal/broker/tools.json) are exported from the current
Rust tool authority. Requests cannot replace those schemas or routing policy.
Server policy sets `data_collection: deny`; live availability and retention
compatibility still require the separate W8 check. See the upstream
[routing controls](https://openrouter.ai/docs/guides/routing/provider-selection)
and [streaming protocol](https://openrouter.ai/docs/api/reference/streaming).

`cmd/broker` owns the upstream credential, run ledger and capability hashes.
`cmd/model-adapter` runs inside a `--network none` workspace and listens only on
HTTPS `127.0.0.1`. Each mounted Unix socket is bound to exactly one workspace;
the capability must also match current account, grants, workspace generation,
deployment and activation. Queue dispatch rechecks current authority. Streaming
authorization repeats every five seconds with a four-second control deadline.
The fair bounded queue permits one request per account and three per host.

The parser bounds the whole SSE response to 1 MiB and request lifetime to 120
seconds. It preserves UTF-8 deltas, provider IDs, usage and complete tool calls.
It withholds `[DONE]` until the entire call batch and route identity validate.
Provider error bodies never pass through. Cancellation, missing usage, failed
transport, crash and interrupted streams retain reservations as unknown.
`GET /v1/requests/UUID` reconciles a recorded result; resubmitting the same
idempotency key never dispatches it again. `POST /v1/requests/UUID/cancel` cancels
only the capability owner's request. `GET /v1/usage` returns own charges,
aggregate run exposure, 75%/90% alerts and queue counts, without conversation data.

The broker filters credentials, email and private paths from outbound prose,
and allowlists structural/approved metadata keys in tool results. The existing
Rust disclosure filter remains active. Neither filter independently proves the
truth of client-supplied metadata or establishes a dataset's export eligibility.

## Offline validation

Use the pinned Go version in [toolchain.json](../infra/demo/toolchain.json).
From `feather-mesh/` build the synthetic probe and export current schemas:

```sh
cargo run --quiet -p mesh_agent --example tool_schemas > ../web_demo/internal/broker/tools.json
cargo build -p mesh_agent --features hosted --example broker_probe
```

From `web_demo/`:

```sh
FEAM_BROKER_PROBE="$PWD/../feather-mesh/target/debug/examples/broker_probe" go test -race ./...
go vet ./...
go build ./cmd/broker ./cmd/model-adapter ./cmd/project-budget
```

The probe uses a new test CA, HTTPS loopback, a real private Unix socket, SQLite,
the existing Rust `RouterProvider`, and fake upstream data. It asserts complete
`help.lookup` alias mapping, preserved call ID/arguments and usage. No provider
key or external request is used. Without `FEAM_BROKER_PROBE`, Go explicitly skips
the Rust bridge subtest; that run alone is not the full compatibility check.
Set the path to the actual artifact if using a dedicated `CARGO_TARGET_DIR`.
Local tests require permission to bind private Unix/TCP loopback sockets.

## Operator project ledger

`project-budget` runs outside disposable compute. Its encrypted snapshot and
encrypted append-only audit markers share a private directory. Every operation
locks, validates accounting, fsyncs the audit and atomically replaces/fsyncs the
snapshot. A torn audit/snapshot update fails closed and requires explicit
operator reconciliation from retained records; deleting the state is not recovery.
All amounts are integer USD microdollars. The initial allowance is `100000000`.
An active or lost run reserves its entire allocation until verified closure.
Reconciliation is idempotent under the allocation ID and receipt hash; fresh
launches, resets, site switches and key changes cannot replenish the allowance.

Provision an existing mode-0700 operator directory and run:

```sh
project-budget -action keygen -encryption-key /operator/feam/encryption.key -signing-key /operator/feam/signing.key -public-key /operator/feam/operator.pub
project-budget -action initialize -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key -project PROJECT_UUID -actor OPERATOR_ID -amount 100000000 -prior-charges VERIFIED_PRIOR_MICRODOLLARS
project-budget -action allocate -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key -signing-key /operator/feam/signing.key -actor OPERATOR_ID -allocation /operator/feam/reviewed-allocation.json
project-budget -action status -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key
project-budget -action unknown -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key -actor OPERATOR_ID -allocation-id ALLOCATION_UUID
project-budget -action reconcile -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key -actor OPERATOR_ID -allocation-id ALLOCATION_UUID -amount VERIFIED_RUN_COST -evidence-sha256 VERIFIED_RECEIPT_HASH
project-budget -action ceiling -ledger /operator/feam/project.enc -encryption-key /operator/feam/encryption.key -actor AUTHENTICATED_ADMIN_ID -amount APPROVED_NEW_CEILING
```

Capture `allocate` stdout into the private signed allocation document only after
the command succeeds. A failed output delivery leaves the run reserved; retrieve
the existing signed document from `status`, never issue the allocation again.
The encryption and signing keys remain with the operator. The broker receives
only the signed document and the public verification key. The `-actor` label is
an audit attribution supplied by the authenticated local operator; it is not a
browser authentication mechanism or a remotely callable permission check.

The unsigned document is the [Allocation Go DTO](internal/budget/allocation.go).
It binds project/run/deployment/activation, amount, finite validity (at most
seven days), fixed model/provider/profile, maximum output, integer price ceilings
and a conservative fee multiplier. `ledger_revision` is set by the locked writer.
Required `request_limit_usd_micros` and `user_daily_limit_usd_micros` bound
individual requests and each account's UTC-day exposure inside that run.
Resets cannot clear them; a new day only changes the daily guardrail and never
replenishes the run or project allowance. Their admission check shares the same
transaction as the global reservation.
Prices and fees must be independently verified before a real allocation.
Reservation uses request bytes as a conservative token bound, the complete
output allowance and upward integer rounding. Provider usage cost is parsed
without floating point. A charge exceeding the reserved bound pauses the run.

The run DB and capability DB have explicit initialization and hash-checked
migrations. Ordinary startup refuses missing/corrupt state and converts prior
outstanding requests to unknown; no request is replayed. Single-writer service
ownership and verified local storage remain deployment requirements.

## Workspace certificates and control integration

`model-adapter -initialize-cert /run/feam-config` explicitly provisions
an existing mode-0700 directory with `ca.pem`, `cert.pem` and `key.pem`. It refuses
existing files. The CA private key is discarded after signing a 24-hour leaf;
only this workspace's public CA and adapter leaf key are installed. Controller
start/reset must provision or rotate certificates deliberately and preserve the
planned ownership/ACLs. The leaf has only the `127.0.0.1` IP SAN.

```sh
model-adapter -listen 127.0.0.1:8443 -socket /run/feam-model/broker.sock -capability-file /run/feam-config/capability -cert /run/feam-config/cert.pem -key /run/feam-config/key.pem
```

The read-only `phase1-demo` Rust profile sets:

```toml
schema_version = 1
default_profile = "phase1-demo"

[profiles.phase1-demo]
backend = "router"
base_url = "https://127.0.0.1:8443/v1"
loopback_ca_file = "/run/feam-config/ca.pem"
model = "deepseek/deepseek-v4.1-flash"
api_key_env = "FEAM_BROKER_CAPABILITY"
context_policy = "metadata-only"
allow_user_text = true
max_context_chars = 32000
max_output_tokens = 1024
allowed_providers = ["deepinfra/fp8"]
allow_provider_fallbacks = false
reasoning_enabled = false
```

The example output limit must agree with the reviewed allocation. The profile
cannot extend this CA trust to an external host, HTTP, `localhost`, host aliases,
credentials or relative certificate paths. It disables proxy discovery for the
local bridge and keeps certificate verification enabled.

`broker -config PRIVATE_CONFIG -initialize` explicitly initializes the two local
DBs. `broker -config PRIVATE_CONFIG -fake-provider` starts synthetic-only serving.
Its config fields are `ledger`, `capabilities`, `allocation`,
`operator_public_key`, `deployment_id`, `activation_generation`,
`authority_socket`, `control_socket`, `controller_socket`, `controller_uid`,
`gateway_uid`, `receipt_file`, `upstream_key_file`, and
`sockets: [{workspace_id, path}]`. All socket directories and ACLs must be
provisioned before startup. `receipt_file` must be beside the ledger in its
private owner directory. The control mutation endpoints accept only the configured
controller's kernel peer UID:

* `POST /v1/capabilities/mint`: the closed [Claims DTO](internal/capability/capability.go),
  checked against gateway authority; returns `{ "capability": "..." }` once.
* `POST /v1/capabilities/revoke`: `{ "workspace_id": "UUID" }`; revoke on stop,
  reset, disable and activation change. Only capability hashes are stored.
* `POST /v1/run/drain`: `{}`; persistently pause admissions, cancel queued/active
  requests, join accounting within 30 seconds, checkpoint SQLite and atomically
  write/return the private `feam.broker-drain.v1` receipt. Failure returns no
  successful new receipt. Unknown costs retain their reservations.

`GET /v1/status` permits the configured gateway UID as well as the controller.
It returns aggregate usage, paused/75%/90% state, queue and recording status, with
no account/request/conversation content. The gateway cannot mint, revoke or drain.
The controller operation route receives a bounded start/end lease only at actual
provider dispatch; waiting in the queue never refreshes workspace activity.
Live mode requires the activity endpoint. Failure to start the lease denies
provider dispatch; a failed end update expires under the controller's short lease.

SIGTERM cancels all queued/active work, waits for accounting, checkpoints and
writes the same receipt before bounded HTTP shutdown. It preserves the prior
run state for ordinary service/OS restart. An explicitly drained or over-limit
run remains paused; an active run keeps only its existing remaining allowance.
No restart replays a request or replenishes an unknown reservation. The receipt
is `broker_observed` evidence bound to allocation/deployment/generation, not
independently verified provider billing; off-host project reconciliation still
requires verified cost. Teardown must also verify the separate archive receipt.

Capabilities expire in at most one hour and grant one workspace and operation.
They are shell-readable by design. They provide no upstream key, arbitrary URL,
cross-user conversation, filesystem or admin permission. Model and event
capabilities are separate. Current authority failure denies both admission and
continuing streams even if edge policy synchronization is unavailable.

The collector configuration adds `collector_socket`, `software_sha256`,
`profile_sha256`, and `dataset_sha256`; its private endpoint authenticates the
broker UID. Live mode refuses to start without the collector. Collector failure
before dispatch rejects admission; final capture failure withholds completion
without undoing durable charges. Fake mode without a collector is explicitly
unrecorded, and the participant launcher refuses to present it as recorded.

## Participant image and capability renewal

The controller sets `FEAM_DEMO_MODE=hosted` and mounts this workspace's directories
read-only at `/run/feam-model` and `/run/feam-config`. The latter contains
`ca.pem`, `cert.pem`, `key.pem`, `capability`, and `feam/agent.toml` copied from the
reviewed [profile example](../infra/demo/images/terminal/agent.toml.example).
The adapter replaces caller bearer headers with the current private capability
file on every request. Renew by atomically replacing `capability` within the
mounted directory; mounting the file itself would hide later replacements.
The shell can read its capability but cannot use it on another workspace socket.
The controller may own these files and give only the container's mapped UID a
named read ACL. The startup checks accept the resulting group-class read mask
(`0640`) while rejecting other permissions and group write; deployment must
independently verify `group::---`, the exact named reader and no unrelated ACLs.
Mode bits alone do not prove that identity boundary. A bounded nonempty startup
placeholder is permitted before generation activation, but cannot authorize a
request; the adapter and broker still validate the real token at dispatch.

The launcher sets a harmless `FEAM_BROKER_CAPABILITY=workspace-adapter` placeholder
for Rust configuration; the real scoped token is read by the adapter. Local
config, certificate/key, socket or recording failures visibly stop participant
launch. Startup supervises both the adapter and terminal and stops when either
exits. The launcher shows model, recording, shared budget/exposure and queue
status before the TUI. Default/explicit `FEAM_DEMO_MODE=manual` preserves the W1
operator workflow without requiring hosted files.

The context builder requires a reviewed JSON map with exactly `feam`, `ttyd`,
and `model-adapter` SHA-256 strings. It rejects wrong digests, symlinks and
non-Linux-amd64 ELF headers before creating the context. Use the native build
artifacts and source manifest, not local Mac binaries:

```sh
python3 infra/demo/scripts/build_terminal_context.py --feam /artifacts/feam --ttyd /artifacts/ttyd --model-adapter /artifacts/model-adapter --artifact-manifest /artifacts/reviewed-terminal-binaries.json --output /artifacts/terminal-context
```

The native web build helper now emits all introduced host-service binaries and
`model-adapter`, each with its digest. An artifact hash authenticates the reviewed
binary bytes; it is not an image runtime or actual-host acceptance result. Run
the image policy tests with:

```sh
python3 -m unittest discover -s infra/demo/tests/w1 -p test_hosted_image.py -v
bash -n infra/demo/images/terminal/entrypoint.sh infra/demo/images/terminal/launcher.sh infra/demo/scripts/native_web_build.sh
```

## Remaining deployment gates

This is local synthetic evidence. Production controller hooks, mounted socket
UID/ACLs, certificate rotation, the built image's actual Ubuntu isolation
and restarts, operator lifecycle integration and archive durability need their
named W3/W5/W6/W7 checks. Run and per-account usage are exposed by the broker;
dashboard rendering still needs integration.
The optional `EventRecorder` stops admission on capture failure and withholds
completion if final capture fails; the collector integration is separately
implemented in W6. W8 alone authorizes a finite live-provider test after current
route, price, privacy, project allocation and owner budget/key inputs are ready.
