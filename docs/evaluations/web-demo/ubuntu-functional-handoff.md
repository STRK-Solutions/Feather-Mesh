# Ubuntu functional demo operator handoff

This is the current Ubuntu-only staging handoff, not a clean-host recreation
recipe. The functional gate is [U.01–U.07/U.G](../../ubuntu_web_dev_demo_workplan.md#ubuntu-functional-milestone).
The private operator files remain in the ignored
`.local/demo-deployment/functional-proposal/` directory; keep that directory,
the encrypted operator and budget ledgers, prior vault revisions, SSH inventory,
signed allocations, logs and archive receipts owner-only. Use the exact current
vault revision and readback hashes before each mutation. The detailed commands
and uncertainty rules are in [SITE_LIFECYCLE.md](../../../infra/demo/SITE_LIFECYCLE.md),
[W7](../../../infra/demo/W7.md), [W8](../../../infra/demo/W8.md), and the
private `u05-fake-to-live-handoff.md`. Do not replace that reviewed transition
with a fresh ledger or a replay of a provider request.

## Deployed identity and access

| Surface | Current pinned value or behavior |
| --- | --- |
| Portal | <https://feam.613202690.xyz> through Cloudflare Access email PIN |
| Administrator | <https://admin.613202690.xyz> through a separate Access application; admins receive no workspace |
| Workspace hosts | Open the exact owner link from the portal. Each host has its own Access audience and owner check; no wildcard origin listener exists. |
| Participant image | `sha256:1c11e6a030467522763a5f86b58283dc83d666440fc5d37e0d253621814fa60e` |
| Current Go source / immutable service release | `6f5daf80e77811210718252804adcbe930c03b9c45913bf0ef6f572d4a1837f8` |
| Gateway binary | `297b938d11a92695ff4f3f2bf962480c22688f012e4c2f275cdf9b331a0a41e6` |
| Current raw service manifest | `33feac6c5433aaa9da0a5acf4e50ec33c9d5a074da0281f24965af5f5f284c8c` |
| Approved climate release | `9894f3498b7d0828d3358b6dd415f6ee0ba298442bca7f615987af147a2aef41` |
| Connector | Pinned cloudflared 2026.9.3, QUIC transport, active while the demo is public and disabled at boot. The HTTP/2 Unix-origin WebSocket failure and repair are retained in [runtime evidence](u05-runtime-repairs.json). |

The real staging login, owner WebSockets, 25-case authenticated edge matrix,
admin disable/disconnect and old-token denials passed; see
[public access evidence](u05-browser-acceptance.json). The second regular test
alias remains **disabled** after that revocation proof. The other regular
participant and administrator remain active. Do not silently reenable the
disabled alias or import the larger private roster. The approved release is
granted to participant A only; B's ungranted read was denied. Browser SDK,
known table/raster and read-only mount evidence is in
[U.02](u02-browser-grant.json). A confirmed B reset kept its old slot while
the new slot had no old marker; A's reviewed TUI deny/confirm copy is in
[U.04](u04-browser-reset-stage.json).

## Browser use

Use each approved identity's own browser profile and real PIN. From the portal,
start the workspace and then reload the dashboard: the current Start POST
returns an **accepted JSON job response**, so that response is not the terminal
page. Once the completed job is visible, use **Open assisted FEAM** to reach the
exact owner host. An idle workspace stops after 30 minutes without activity;
start it again through the portal, retaining its files. A confirmed reset or
delete is a separate five-minute review and can discard workspace content.
Administrators use the separate dashboard to grant an exact digest, stop a
workspace, or review a reset/delete; grant changes stop the affected workspace
until reconfiguration completes. Reload after accepted jobs and inspect the
durable outcome before considering a fresh action. A 503 or unknown outcome
requires operator reconciliation, never a blind repeat.
For a reviewed test-account revocation, use the admin dashboard's **Disable**
action, keep the test terminal open to observe disconnect, and verify the old
session cannot reconnect. The second regular alias has already passed this
test and remains disabled; do not repeat it or reactivate that alias as a
routine demo step.

The two observed participant containers render the FEAM TUI through WSS.
Each is capped at 512 MiB, half a CPU, 128 processes, a 2 GiB writable
workspace and two terminal connections; network mode is `none`, the root
filesystem is read-only, and capabilities are dropped. These are enforced
per-container limits, not a measured concurrent-user capacity. Pool slots
01–02 remain reserved for the earlier W1 demo. Ten additional slots 03–12
were selected for this staging deployment; two owner workspaces used them and
the first B slot became retained after reset. Initially eight slots were
spare, but reset/retention changes the free count. Only **two simultaneous
participant workspaces** were observed; ten-user acceptance is deferred.

## Private operator status, logs and lifecycle

Run from the repository root on the durable operator Mac with `umask 077` and
the locked Python/Ansible environment. These commands use private file paths
without printing identities, credentials or event bodies:

```sh
umask 077
PY=/private/tmp/feam-web-w0-venv/bin/python
ANSIBLE=/private/tmp/feam-web-w0-venv/bin/ansible
PLAYBOOK=/private/tmp/feam-web-w0-venv/bin/ansible-playbook
BASE="$PWD/.local/demo-deployment/functional-proposal"
export ANSIBLE_LOCAL_TEMP="$BASE/ansible-tmp"

# Choose the current synthetic or later live vault; never mix their budgets.
RUN="$BASE/synthetic-lifecycle"
$PY infra/demo/scripts/operator_vault.py verify --key "$RUN/operator.key" --input "$RUN/operator.enc"
$PY infra/demo/scripts/site_lifecycle.py status --vault "$RUN/operator.enc" --key "$RUN/operator.key" --expected-revision REVISION_FROM_VERIFY
$ANSIBLE -i "$BASE/u04-inventory.yml" demo -b -m ansible.builtin.command -a 'systemctl --no-pager is-active feam-gateway.service feam-controller.service feam-broker.service feam-collector.service feam-pipeline.service feam-reconciler.service feam-edge.service'
$ANSIBLE -i "$BASE/u04-inventory.yml" demo -b -m ansible.builtin.command -a 'journalctl --no-pager -u feam-gateway.service -u feam-controller.service -u feam-broker.service -u feam-collector.service -u feam-pipeline.service -u feam-reconciler.service -n 50'
$ANSIBLE -i "$BASE/u04-inventory.yml" demo -b -m ansible.builtin.command -a 'journalctl --no-pager -u feam-edge.service -n 50'
```

Keep journal output in private operator storage; do not paste tokens, email
addresses, raw research text or trace bodies into a public issue. For a
read-only participant/account/controller snapshot, the scoped private
`u04_acceptance_snapshot.py snapshot --account-readback
$BASE/u04-account-readback.json` helper retains an owner-only JSON receipt.
Check pending jobs, collector archive state and the current lifecycle vault
before every stop or start. Do not interpret an `unknown` controller/provider
outcome as failed and retry it.

For an **ordinary whole-site stop**, first close browser routing by stopping
only the connector with the reviewed edge role; keep the four DNS/Access
resources and exact Terraform ingress intact. Then use the current run's
`site_lifecycle.py stop` with a fresh verified revision. This records stopped
intent, drains the broker, stops A/B and the six private writers, flushes and
verifies the local collector archive and keeps Ubuntu data. The edge role
requires the explicit stop flag while the connector is running:

```sh
$PLAYBOOK -i "$BASE/u04-inventory.yml" infra/demo/ansible/edge.yml \
  -e "@$BASE/edge-active-v2-proposed-vars.json" \
  -e '{"feam_edge_stop": true, "feam_edge_start": false}'
$PY infra/demo/scripts/site_lifecycle.py stop --vault "$RUN/operator.enc" \
  --key "$RUN/operator.key" --expected-revision FRESH_REVISION
```

If stop is interrupted, inspect the encrypted state and host. Resume only
the recorded operation ID with the documented `--resume-operation`; preserve
uncertain evidence and the old archive. After a verified stopped state, use
`site_lifecycle.py copy-archive` to a **new absent** owner-only Mac directory,
then `archive_copy.py verify` on that directory. This independent verified
copy is required before destructive teardown or a fake-to-live replacement:

```sh
$PY infra/demo/scripts/site_lifecycle.py copy-archive \
  --vault "$RUN/operator.enc" --key "$RUN/operator.key" \
  --expected-revision FRESH_STOPPED_REVISION --output-directory NEW_ABSENT_PRIVATE_MAC_DIR
$PY infra/demo/scripts/archive_copy.py verify --directory NEW_ABSENT_PRIVATE_MAC_DIR
```

For an **ordinary start**, use the current run's vault only after its finite
allocation, manifest/release/policy hashes, mounts and archive baseline pass
the documented preflight. Start the private site first; the lifecycle script
does not start participant workspaces or publish browser traffic. Start the
connector last with the same reviewed active variables (the installed template
uses QUIC), then let participants start their own workspace from the portal:

```sh
$PY infra/demo/scripts/site_lifecycle.py start --vault "$RUN/operator.enc" \
  --key "$RUN/operator.key" --expected-revision FRESH_STOPPED_REVISION
$PLAYBOOK -i "$BASE/u04-inventory.yml" infra/demo/ansible/edge.yml \
  -e "@$BASE/edge-active-v2-proposed-vars.json"
```

The current fake service variable record is
`.local/demo-deployment/functional-proposal/u05-gateway-service-vars.json`;
it binds the current gateway release and has no database initialization
selection. The connector's reviewed active variables are
`.local/demo-deployment/functional-proposal/edge-active-v2-proposed-vars.json`.
Do not apply old service variables with the previous runtime digest. The
separate [private fake-to-live recipe](../../../infra/demo/SITE_LIFECYCLE.md#separate-synthetic-to-live-handoff)
uses a **new** live run, primary budget ledger, rendered broker/config
variables and lifecycle vault; no existing synthetic allocation becomes live.
After a successful one-time live broker initialization, repeat converges use
the renderer's `service-vars-post-init.json`, not its initialization input.

## Evidence still to fill before U.G

| Gate | Current handoff entry |
| --- | --- |
| Fake capture and stop/archive | **Pending final receipt:** record correlated request/event counts, verified batches, ordinary stop result and retained Ubuntu archive inventory. |
| Independent Mac archive copy | **Pending final receipt:** record the fresh private copy directory's verified inventory/ledger hashes and vault revision. An Ubuntu archive alone is not host-loss protection. |
| Live five-task OpenRouter smoke | **Pending final receipt:** the owner-approved US$1 run, US$0.03/request ceiling, pinned route and five tasks are in [U.06](u06-functional-smoke-proposal.md). Issue the signed allocation only through the existing primary project ledger after fake stop/copy; record actual route, charges, unknown reservations and capture. Stop after task five even if money remains. No automatic retries, fallback, extra model tasks or fresh allocation are authorized by this handoff. |
| Ordinary live stop/start and unchanged converge | **Pending final receipt:** record fresh live vault revisions, service/socket health, no unexpected restart, retained grants/files and a zero-change repeat converge using post-initialization variables. |
| Final operator handoff | **Pending final values:** add the verified live runtime manifest, final archive copy and budget/capture totals when available. Do not copy private identities or credentials into this document. |

Cloud VMs and R2 remain shelved. Ten-user load, unattended reboot,
clean-host recreation, extended recovery and benchmark quality remain
deferred; they are not part of the functional U.G claim.
