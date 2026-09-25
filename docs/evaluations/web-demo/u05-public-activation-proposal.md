# Ubuntu staging browser activation

Status: the owner authorized the exact public activation on 2026-09-25. The
four routes are active, the Ubuntu connector is running, and three approved
participants completed real email-PIN sign-in and reached their dashboards
with HTTP 200. This is the executed activation record; U.05 browser/edge
acceptance passed, while U.G and separate data, capture and live gates remain
open. [Sanitized browser evidence](u05-browser-acceptance.json) records the
authenticated and revocation checks.

The initial saved Terraform plan SHA-256 was
`fc82a70a2ae1acf9c92bd8ef4b983aee04f3869bfb29632e297455f1b45be830`.
It planned five creates and one update with no deletions: four exact proxied
CNAMEs, one exact-host HTTPS redirect ruleset, and the existing Ubuntu tunnel
ingress update. The pre-activation readiness receipt was
`fd3d46d6a68f06410e5e2cf2c87abbfe5a0c480d8d0bca9415ccbd6d505fda02`.
An earlier saved plan beginning `68966a56` was superseded before apply.

The first apply created all four DNS records and updated the Ubuntu ingress,
then Cloudflare rejected the redirect expression's unsupported
`http.request.scheme` field (HTTP 400/code 20127). The saved redirect-only
repair plan SHA-256 was
`67d9834651a28cdea7d5c6752528d6ba90ec696ee575142e3db365ff20dbe220`.
It created one ruleset using Cloudflare's documented
[`ssl` boolean](https://developers.cloudflare.com/ruleset-engine/rules-language/fields/reference/ssl/)
to match unencrypted requests. The exact host set, 301 status, path and query
preservation stayed the same. Successful DNS and ingress changes were not
replayed; the failed attempt and partial-state evidence remain retained.

The original selected Access identity provider was discovered to be type
`cloudflare`, which sent fresh browsers to Cloudflare account login rather
than email PIN. A dedicated account provider of type `onetimepin` was created
and verified. The saved repair plan SHA-256 was
`75c5bdaba39e4b6e0ed05106b05ebf3f21efbcf9eca4a569939a0b013082d3c2`;
it changed only `allowed_idps` on the four existing applications. Their IDs,
audiences, policies and other selected settings stayed unchanged, as did the
original Cloudflare provider. Cloudflare's current
[OTP setup guidance](https://developers.cloudflare.com/cloudflare-one/integrations/identity-providers/one-time-pin/)
states that new Zero Trust organizations do not add OTP automatically.

The active routes are the regular dashboard at
`https://feam.613202690.xyz`, the separate administrator dashboard at
`https://admin.613202690.xyz`, and two exact private workspace hosts. Each
route uses the existing Ubuntu tunnel's private gateway Unix socket with its
own required Access audience; unmatched ingress ends in 404. The redirect
matches only these four hosts. No wildcard DNS, public origin listener, cloud
host, R2 operation, invitation, Access group expansion or unrelated ruleset
change was included. At activation membership was the approved one admin and
two users; the later revocation test intentionally disabled the second regular
alias, leaving one active admin and one active regular user.

During U.05 acceptance, the dedicated connector ran as UID 2109 with its
Ubuntu-tunnel credential. Its systemd unit was active but disabled at boot. All six participant services
are active; host readback found no direct 80/443 listener and no unauthenticated
direct-origin admission. Nine public unauthenticated and HTTP-to-HTTPS probes
passed. Fresh browser PIN forms and real sign-ins succeeded for all three
approved participants. Their first post-login browser navigation briefly
showed `ERR_TOO_MANY_REDIRECTS`; reloading the same authenticated sessions
reached the dashboards with HTTP 200. No edge configuration change was needed
for that transient browser state. The final Terraform plan reported no changes.

After the PIN flow, real Chromium native admin forms sent `Origin: null`
under the gateway’s prior `no-referrer` policy; two grant attempts received
403 before any mutation. A bounded gateway release changed only the response
policy to `same-origin`, retaining exact Origin and CSRF checks. The deployed
admin browser then granted the exact promoted release to A; B remained
ungranted. [Repair evidence](u05-browser-form-repair.json) records native
race/vet/build and the real Chromium regression.

Both owner terminal HTML pages reached HTTP 200, but their WebSockets first
received 502 and closed. The connector journal showed 14 `http: no Host in
request URL` errors under HTTP/2 with the Unix origin. A connector-only change
to QUIC retained the exact routes, Access audiences and origin socket. Both
real participant terminals then rendered FEAM and kept WebSockets open. The
[transport repair](u05-websocket-transport-repair.json) and
[runtime receipt](u05-runtime-repairs.json) retain the failure and bounded
change.

The final authenticated edge probe passed 25 of 25 HTTP, WebSocket, origin,
CSRF, role, audience and redirect cases. The owner WebSocket upgraded with
101; missing or sibling Origin received 403, and cross-user access with a
valid same-audience assertion received 403. The admin disabled the second
regular staging alias through the actual browser UI. Its current document
WebSocket closed 0.506 seconds after the queued action, within the 30-second
U.05 limit; old portal and workspace JWTs returned 403. The alias remains
disabled, while A and the administrator remain active. The explicit
expired-assertion probe is deferred to extended testing; wrong-signature and
unsigned-assertion denials passed.

U.05 is complete for this functional milestone. Dataset SDK reading, fake
capture/archive, reviewed reset, ordinary stop/start, live inference and a
verified Mac copy remain U.03/U.04/U.06/U.07/U.G work. Initial inference is
still the explicit synthetic provider. Private receipts and browser session
material stay outside this public record; the operator can stop the connector
and disable these exact routes without destroying application or research
state.
