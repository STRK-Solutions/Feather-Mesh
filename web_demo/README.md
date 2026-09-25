# Web demo development baseline

W0 supplies versioned schemas, examples and offline validation. W1 adds the
operator-only `cmd/private-terminal` Unix-socket proxy. Participant gateway, controller, broker, collector, pipeline and reconciler
now have separate entrypoints and private IPC owners. Follow the
[interface contract](../docs/ubuntu_web_dev_demo_contract.md) and
[workplan](../docs/ubuntu_web_dev_demo_workplan.md); Rust remains FEAM's authority.

From the repository root, with Python 3.13:

```bash
python3 -m venv /tmp/feam-web-w0-venv
/tmp/feam-web-w0-venv/bin/python -m pip install -r infra/demo/requirements-dev.lock
export PATH="/tmp/feam-web-w0-venv/bin:$PATH"
python infra/demo/scripts/check.py
python web_demo/validate.py settings infra/demo/examples/settings.json
python -m unittest discover -s web_demo/tests -v
```

These commands require no SSH inventory, provider key, model request or cloud
account. The Python schema tests exercise schema/semantic rejection. The separate Go
suites exercise authenticated IPC, transactional stores, fake broker streams
and negative authorization; actual-host isolation remains a separate gate. The examples contain fake IDs
and cannot provision a host. Event payload schemas/redaction and versioned SQLite migrations are
implemented in the consuming control, lifecycle, broker and collector packages. A schema-valid caller
claim is never authorization; service peers/capabilities must be verified.

Use [operator checks](../infra/demo/README.md) for the separate actual-host and
full-system VM gates. The implemented Go proxy checks are below; the W1 runbook
includes the Chromium manual/fake probe and browser admission negatives.

## W1 private terminal development

With the pinned Go toolchain, run from `web_demo/`:

```bash
go test -race ./...
go vet ./...
go build -trimpath ./cmd/private-terminal
```

Tests use actual local Unix sockets and WebSockets; socket-denying sandboxes
need scoped local socket access. The binary accepts only a Unix listener and
backend plus an exact loopback browser origin and an owner-only random operator
credential file. Reach it through verified SSH; it has no TCP listener, test
identity headers, runtime authority or production identity mode. W2 owns the
Cloudflare/participant gateway. W1's Basic credential is restricted to the
operator smoke path, consumed by the proxy and stripped before the sandbox.

The image's launcher invokes manual FEAM with a fixed project path. Hosted
capability is compiled in; no provider credential or live dispatch is configured.
Use the [W1 operator runbook](../infra/demo/W1.md) for build, VM and actual-host
commands. [Execution evidence](../docs/evaluations/web-demo/w1-progress.md)
distinguishes local tests, native builds, VM proof and actual Ubuntu acceptance.

## Participant services

With the pinned Go toolchain, run `go test -race ./...`, `go vet ./...` and
`go build -trimpath ./cmd/...` from this directory. These tests require local
Unix and loopback sockets; they make no paid provider requests. Follow
[identity](IDENTITY.md), [broker/accounting](BROKER.md) and the
[lifecycle contract](../docs/ubuntu_web_dev_demo_contract.md). The broker runbook
adds the real Rust-to-loopback-to-Unix fake-provider compatibility probe.

Each service opens its own explicitly initialized, hash-migrated SQLite store.
The gateway never receives the runtime socket, model provider key or research
export privilege. The controller accepts fixed lifecycle methods and recorded
workspace IDs; immutable release resolution is delegated to the pipeline.
Sources, Terraform state, provider credentials, private inventories and actual
participant data remain outside the public checkout.

The participant image uses explicit hosted mode and requires scoped model/event
sockets and read-only configuration. W1 manual mode is still an operator test
path. A missing recording/archive/budget dependency must be shown as unavailable.
See the [implementation record](../docs/evaluations/web-demo/w2-w7-progress.md)
for tests completed and the environment gates that remain open.
