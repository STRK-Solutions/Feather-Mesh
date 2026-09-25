# W5 local synthetic evidence — 2026-09-25

Environment: MAC, source checkout on the web-demo feature branch; isolated Go
and Cargo caches under `/private/tmp`. No paid API, external model key, participant
conversation, cloud purchase or public route was used. Implementation and
commands: [broker runbook](../../../web_demo/BROKER.md).

Implemented locally: signed fixed-route allocations; encrypted off-host operator
ledger; conservative SQLite request reservation/reconciliation; hashed scoped
capabilities; gateway authority rechecks; bounded fair queue; private Unix broker;
HTTPS-loopback adapter; scoped Rust CA support; pinned provider-safe tool schemas;
stream limits/cancellation; typed sanitized errors; aggregate alerts/status.

Passed commands at this point:

```sh
# web_demo/, with pinned Go and writable temporary caches
go test -race ./internal/budget ./internal/capability ./internal/broker ./cmd/broker ./cmd/project-budget ./cmd/model-adapter
go vet ./internal/budget ./internal/capability ./internal/broker ./cmd/broker ./cmd/project-budget ./cmd/model-adapter

# feather-mesh/, isolated CARGO_TARGET_DIR
cargo test -p mesh_agent --features hosted
cargo build -p mesh_agent --features hosted --example broker_probe

# web_demo/, actual freshly built Rust probe
FEAM_BROKER_PROBE=/private/tmp/feam-web-broker-rust-target/debug/examples/broker_probe go test -race -v ./internal/broker
```

The explicit Rust bridge subtest passed: `RouterProvider` -> verified per-workspace
CA -> HTTPS `127.0.0.1` -> private Unix socket -> fake upstream, with complete
`help.lookup` alias, call ID/arguments and usage. The scoped CA negative test
rejects external hosts, HTTP, `localhost`, lookalike hosts, credential authorities,
port zero and relative paths. Twenty-two focused Rust tests passed.

Go checks exercise corrupted/torn accounting, lost request state, concurrent
reservations, idempotency conflict/no replay, exact-once run closure, unchanged
project exposure after unknown hosts, near-limit admission, overcharge pause,
wrong/expired allocation signatures, capability cross-socket/operation rejection,
revocation, dispatch-time rechecks, active cancellation, split UTF-8/SSE, complete
tool arguments, usage rounding, response bounds, provider identity rejection,
truncated/error streams and route/profile edits. Active revocation completed
in the five-second poll test; its reservation remained unknown.

Retained failed attempts: default Go cache/module paths were unwritable under
the sandbox; tests reran with the existing temporary caches. The initial Unix
socket and Rust loopback tests failed with `operation not permitted`; authorized
local socket runs subsequently passed. A formatting call from the wrong working
directory failed before editing and was rerun from `web_demo/`. No failed live
attempt exists: all upstream responses were synthetic.

W5.01 and W5.04 have MAC implementation and local automated proof. W5.02/W5.03
have MAC implementations and the actual local Rust bridge; Linux and deployed
Ubuntu configuration remain separate. W5.05/W5.06 implement durable budget logic
and local tests; operator lifecycle, dashboard and target-host evidence remain.
W5.07 and W5.G are not satisfied by these tests. Current route availability,
privacy compatibility, price/fee bounds, real identities/mounts, finite live
authorization, Ubuntu/cloud restarts and cross-host lifecycle remain pending.
Final shared Rust feature-matrix and schema-drift checks are recorded by the
integrating agent after concurrent source changes settle.

Image/config integration update: added the adapter binary and reviewed native
artifact-manifest input, fixed read-only launch profile, hosted startup checks,
adapter/terminal supervision, participant recording/budget status and explicit
W1 manual compatibility. Three local image-policy tests and shell syntax checks
passed. The adapter now reloads a read-only capability file per request, allowing
atomic renewal while an existing TUI retains its placeholder credential. The
TLS/Unix test exercises revoked-old/new-file renewal. Actual rebuilt image and
Ubuntu mounted identity proof remain integration gates.


Additional local shutdown integration (2026-09-25): broker activity start/end
uses the fixed controller Unix route at dispatch only, with bounded deadlines.
The controller-only drain persists pause, cancels queued/active handlers, retains
unknown costs, checkpoints and writes a private allocation-bound receipt.
SIGTERM checkpoint preserves the prior active/paused state. Tests cover no
queue activity, controller failure before dispatch, cancellation without replay,
unknown reservations across restart, explicit-pause preservation, receipt write
barrier/timeout, and gateway read-only aggregate status versus drain permission.
Focused race suite and the existing Rust HTTPS/Unix/tool/capability probe passed;
no external provider requests were made. The first new receipt test correctly
rejected its test directory's non-private mode; fixing the fixture to 0700 passed.
Ubuntu peer identities/systemd/reboot remain separate acceptance gates.
