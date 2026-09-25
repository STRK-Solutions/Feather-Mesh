# Identity gateway and account reconciliation

`cmd/gateway` is the sole control-database owner. `cmd/reconciler` changes only
its two recorded Access groups, using a separate credential and Unix control
API. There is no test identity header or bypass switch in either executable.
Local tests use generated signing keys or test-only validators and do not prove
real Access login, email delivery or deployed Ubuntu isolation.

Run from `web_demo/` with Go 1.27.1:

```bash
go test -race ./internal/db ./internal/ipc ./internal/auth ./internal/control ./internal/gateway ./internal/reconcile
go vet ./internal/db ./internal/ipc ./internal/auth ./internal/control ./internal/gateway ./internal/reconcile ./cmd/gateway ./cmd/reconciler
go build -trimpath -o /tmp/feam-gateway ./cmd/gateway
go build -trimpath -o /tmp/feam-reconciler ./cmd/reconciler
```

On the current Mac the pinned tool is
`/private/tmp/feam-web-w0-tools/go/bin/go`; use
`GOPATH=/private/tmp/feam-web-w1-gopath` and
`GOCACHE=/private/tmp/feam-web-w1-go-cache` if the normal cache is restricted.
Local socket tests require permission to bind Unix sockets and loopback TCP.

Provision private service-owned directories and a verified local filesystem
before initialization. Run these commands as the gateway's intended identity,
with the operator's owner-only configuration and roster files:

```bash
/tmp/feam-gateway -mode initialize -config /private/operator/gateway.json
/tmp/feam-gateway -mode bootstrap -config /private/operator/gateway.json -input /private/operator/roster.json
/tmp/feam-gateway -mode assign -config /private/operator/gateway.json -input /private/operator/workspace.json
/tmp/feam-gateway -mode serve -config /private/operator/gateway.json
```

For an already bootstrapped deployment, `-mode invite` accepts a private JSON
object with `actor_id` (an active administrator) and `enrollment` (`email`,
`role`). It creates a pending account and prints its recorded identity; it
sends no email. Inspect the stored account after a lost reply rather than
replaying the command. Configure its exact workspace, Access audience, broker
and event endpoints before recording readiness. Actual group reconciliation
is required for activation. Re-running bootstrap never imports new accounts.

An explicitly authorized restoration uses `-mode restore` with private
`actor_id`, `account_id` and the current `auth_version`. Stop the site first
and verify the original revocation completed. This preserves the subject,
role and grants, advances authorization version, and returns the account to
pending until actual edge reconciliation. For regular users also remove the
completed controller barrier with the [offline restoration command](RECOVERY.md).
Ordinary converge never restores disabled identities.

`initialize` refuses an existing file. `migrate` applies explicit hash-recorded
schema changes; `serve` rejects a missing, modified, unknown or older schema.
Compatible rollback uses the existing schema. Incompatible rollback stops;
restore requires separately reviewed backups and current authorization floors.
Every SQLite connection uses foreign keys, WAL, full synchronous durability and
a bounded busy timeout. File names contain opaque IDs, never email addresses.

A gateway configuration has these fields (values below are synthetic):

```json
{
  "db": "/var/lib/feam/control/control.db",
  "browser_socket": "/run/feam/gateway-browser/http.sock",
  "control_socket": "/run/feam/gateway-control/http.sock",
  "issuer": "https://example.cloudflareaccess.com",
  "read_uids": [21002, 21003, 21004, 21005],
  "controller_uid": 21002,
  "reconciler_uid": 21006,
  "gateway": {
    "portal_host": "feam.example.invalid",
    "admin_host": "admin.example.invalid",
    "audiences": {
      "feam.example.invalid": "portal-app-audience",
      "admin.example.invalid": "admin-app-audience",
      "u-11111111-1111-4111-8111-111111111111.example.invalid": "workspace-app-audience"
    },
    "controller_socket": "/run/feam/controller/http.sock",
    "model": "Selected approved profile",
    "recording": "Structured research recording; 30-day retention",
    "budget": "Approved project allocation; current usage is available from the broker"
  }
}
```

The roster is an array of `{ "email": "exact@example.invalid", "role": "user" }`
with one to four admins and one to ten users, bounded by the workspace capacity.
For a staged activation, bootstrap only the selected admin/users; retain the
rest of the private cohort for separately authorized invitations. Membership
reconciliation includes every non-disabled imported account, so importing the
full roster would activate every admin after successful reconciliation.
`subject` may be pinned in advance;
otherwise the first cryptographically verified active login binds it once.
Bootstrap is a one-time import: subsequent converge does not overwrite roles,
disabled identities or removals. New users remain pending until exact edge
membership and a ready workspace assignment are recorded. Admins require edge
readiness and receive no workspace. No public signup exists.

An assignment JSON has `id`, `owner_id`, `hostname`, `socket`, `ready`,
`generation`, `grant_version`, `deployment_id` and `activation_generation`.
IDs are lowercase UUIDs; versions start at one. The operator supplies a fixed
terminal socket location. Browser requests cannot choose paths or commands.

Both listeners use filesystem Unix sockets. Socket parents must be real,
non-group/world-writable directories. Provision directory traversal and default
socket ACLs for each specific service UID before startup; sockets have mode
0660 so inherited ACL masks remain usable. Do not give these permissions to
sandbox users. Linux authenticates service peers with `SO_PEERCRED`; Darwin
uses `LOCAL_PEERCRED` for local tests. An inactive socket can be reclaimed only
when it is owned by the current UID, connection is refused, and its inode is
unchanged. Active, foreign, symlink and regular-file paths are preserved.

The private control listener supports:

| Route | Caller | Contract |
| --- | --- | --- |
| `POST /v1/authorize` | Recorded read-service UIDs | `{account_id,auth_version,workspace_id?,workspace_generation?,grant_version?,deployment_id?,activation_generation?}`; current active identity/ownership and every supplied version must match. |
| `GET /v1/accounts/{id}` | Recorded read-service UIDs | Active account record; disabled/pending identities are denied. |
| `POST /v1/workspaces/snapshot` | Controller UID | Monotonic recorded assignment snapshot; cannot change owner, deployment, host or socket. |
| `GET /v1/policy` | Reconciler UID | Current exact user/admin email arrays and policy revision. |
| `POST /v1/policy/result` | Reconciler UID | `{revision,success}`; stale acknowledgments cannot activate accounts. |

Gateway pages use exact configured host audiences, maintained RS256 validation,
a fixed issuer key URL and bounded certificate refresh. Unknown keys trigger
at most one refresh a minute; cached keys expire after 15 minutes and unavailable
refresh then denies authentication. Expiry, issued-at, issuer, signature,
application token type, exact single audience, email and subject are checked.
Plain identity headers are ignored.

CSRF cookies are host-only `__Host-`, Secure, HttpOnly and SameSite Strict. POSTs
require the same exact Origin, signed account/version-bound CSRF token and an
Access login issued within the last hour. Reset/delete review lasts five minutes,
binds the complete action and is registered with the controller before display.
The controller consumes confirmation and durably deduplicates action admission.
Currently workspace actions are owner-only; admin support access is not exposed.

The terminal gateway limits each workspace to two WebSockets. Only ttyd input
frames count as activity; resize and ping traffic do not extend idle lifetime.
The proxy strips all cookies, Access assertions and non-allowlisted identity
headers in both directions. Sandbox responses cannot set cookies or redirect to
other origins. HTTP responses are bounded; WebSocket frames have explicit limits
and writes time out. Streams recheck local ownership, versions and token expiry
at least every five seconds, with concurrent two-second authorization deadlines.
Disable commits an authorization-version increment and durable revocation job
before edge work. It immediately denies new requests, closes local streams and
retries controller stop until acknowledged; model/event services reject the
changed authorization version independently of edge success.

Reconciler configuration contains `control_socket`, `account_id`, `user_group`,
`admin_group`, `group_names` (recorded ID to name map) and `token_file`:

```bash
/tmp/feam-reconciler -config /private/operator/reconciler.json -once
/tmp/feam-reconciler -config /private/operator/reconciler.json
```

The continuous mode reconciles every 15 seconds, recording pending/error status
in the control owner. Unknown group IDs cannot be selected. Nonempty groups use
only exact-email rules; an empty group includes and excludes Everyone in the
same replacement request, producing impossible membership. Terraform must only
reference these separately managed IDs. Real API acceptance, repeat Terraform
apply and real removed-user login denial remain EDGE verification gates.
See the [Access JWT validation documentation](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/)
and [group update API](https://developers.cloudflare.com/api/resources/zero_trust/subresources/access/subresources/groups/methods/update/).

Current local evidence covers migration drift/missing-file refusal, identity
binding, pending activation, bootstrap replay, role/ownership/CSRF negatives,
key rotation and offline expiry, real Unix kernel-peer checks, two-client
WebSocket limits and revocation, and malicious response-cookie filtering.
Actual containers, Ubuntu service identities, real Cloudflare JWTs, PIN delivery,
Ottawa browsers and Terraform-after-removal remain separately measured gates.
