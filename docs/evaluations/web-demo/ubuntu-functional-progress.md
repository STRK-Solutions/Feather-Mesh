# Functional Ubuntu gate takeover — 2026-09-25

The active delivery scope is U.01–U.07/U.G in the
[workplan](../../ubuntu_web_dev_demo_workplan.md#ubuntu-functional-milestone).
Work continues on `feat/complete-ubuntu-web-demo`, preserving the earlier
uncommitted implementation and unrelated `.DS_Store`. The owner requested
GPT-6 sol agents for scoped work; three agents handle prerequisites/image,
collector local archival, and operator archive-copy lifecycle respectively.

## Current execution

| Task | Observed progress | Remaining proof |
| --- | --- | --- |
| U.01 | Complete: Ubuntu Python prerequisites, installed readers, image and synthetic hosted startup pass; current reused binary inputs match by content. [Receipt](u01-ubuntu-prerequisites.json). | Participant service/live acceptance belongs to U.04–U.06. |
| U.02 | Exact owner-approved release promoted once; independent hash, installed readers, loopback STAC and mapped-UID read/write denials pass. [Release](u02-ubuntu-release.json). | U.04 per-user grant checks. |
| U.03 | Explicit production local archive and protected Mac-copy lifecycle implemented. Four actual Ubuntu non-reviewer categories cannot read the existing collector archive or events DB or connect to its research-export socket. [Code evidence](u03-collector-local-archive.md), [deployed denials](u04-private-workspaces.json). | Deployed capture/flush/readback and independently verified Mac copy. |
| U.04 | Six private services, A/B terminal HTTP 200, exact bounds and owner denials passed. The first A failure was hash-bound aborted before a fresh start. [Build](u04-native-web-final.json), [services](u04-private-services.json), [private workspace proof](u04-private-workspaces.json). A bounded [gateway repair](u05-browser-form-repair.json) now permits native Chromium same-origin form POSTs while retaining CSRF checks. Real admin browser grant gave A only the promoted read-only dataset; B remains ungranted. W1 remains preserved. | Reviewed reset, fake/capture flow, ordinary stop/start and unchanged converge. |
| U.05 | **Complete for functional scope.** Owner-approved [public activation](u05-public-activation-proposal.md) applied four exact routes and protected Ubuntu ingress. Three real PIN logins reached dashboards; both owner terminals connected after the scoped [QUIC connector repair](u05-websocket-transport-repair.json). All 25 authenticated/negative HTTP and WebSocket probes passed. Admin browser disable closed the second regular alias’s current WebSocket in 0.506 seconds; old JWTs returned 403. The alias remains disabled, so current Access group includes one admin and one user. [Browser acceptance](u05-browser-acceptance.json), [activation checks](u05-public-activation.json). | Explicit expired-assertion test is deferred; U.G still depends on data, capture, live smoke and retained copy. |
| U.06 | Key file checked privately; owner approved [five tasks with a US$1 cap](u06-functional-smoke-proposal.md), US$0.03 per-request ceiling and expiry. Current route metadata matches price ceilings. No allocation issued or inference sent. | Private checks, signed allocation and deployed five-task smoke. |
| U.07 / U.G | Owner selected one admin and two regular staging identities, using separate browser profiles. | Real walkthrough, usable operator handoff and verified research copy. |

U.01, U.02 and U.05 are checked complete. Public activation, real PIN
dashboards, authenticated terminal WebSockets and measured revocation pass.
U.03/U.04/U.06/U.07/U.G remain open for fake capture/archive, ordinary
stop/start, live model, verified Mac copy and operator handoff. No invitation
or paid model request has been made. Cloud-host and R2 operations remain
shelved.

## EDGE read-only inventory and local tests

This section retains the pre-activation inventory and disabled-stage checkpoint;
the current public state is recorded in U.05 above. Authenticated
Cloudflare API reads on 2026-09-25 returned the selected zone active, with no DNS records, Access applications, Access groups or tunnels.
The existing organization was present; the sole IdP at this checkpoint was
later confirmed to be Cloudflare-account login, not email PIN. The explicit OTP
provider was added during public activation.
Universal SSL is enabled and its active certificate covers the zone apex and
first-level wildcard, expiring 2026-12-24. This is API configuration evidence;
no deployed hostname TLS handshake or email login is claimed. The Single
Redirect phase is absent; the three existing managed rulesets are untouched.

`infra/demo/terraform/edge/main.tf` defaults `provisioned_sites` to Ubuntu.
Selecting cloud without explicitly provisioning it fails validation. Existing
optional cloud support is retained; an already provisioned tunnel must remain
listed to preserve it while shelved. This account has no existing tunnel.

MAC: Terraform 1.14.5 formatting passed. The first sandboxed validation could
not start the provider's local socket. Running the mock suite with that socket
permission exposed an overbroad expected-error assertion; after correction,
all ten mock plans passed. The disabled live plan then applied eight creates.
The reconciler-owned groups were created separately with explicit denying
membership. Terraform state, saved plan, source and logs have an encrypted Mac
snapshot. No public route is active.

## Private operator inputs and human checks

The owner selected an existing admin and supplied a new regular staging
identity and a second staging alias in this conversation. The ignored private
roster now has four admins and seven regular users; the encrypted vault is
revision 10, retaining prior revisions. Two regular staging identities are
available through the owner's inbox. These
updates retain the full roster privately; only three staging identities enter
bootstrap. The owner separately approved their actual group reconciliation in
the [staging proposal](u05-staging-membership-proposal.md). No invitations ran.
No identity or credential is included in this public record.

The supplied Cloudflare token and model key passed owner-only file checks.
Tokens remain in private files, never chat or Git. The three approved
participants used their mailboxes for real PIN sign-in; private browser session
material is not included here.

The [OpenRouter endpoint listing](https://openrouter.ai/api/v1/models/deepseek/deepseek-v4.1-flash/endpoints)
on 2026-09-25 lists `deepinfra/fp8` with tools, FP8, and prices of US$0.14 input
and US$0.42 output per million tokens. This is metadata, not successful live
routing or billing evidence. The existing US$100 project ledger remains the
spending authority; no live allocation is inferred from that ceiling.

## Retained failed attempts

- Root's initial read-only SSH attempt used the old owner key from inventory
  with `feam-deploy` and failed authentication before executing a remote
  command. The documented dedicated deployment key then succeeded.
- Automatic approval review rejected the prerequisite agent's source transfer.
  The agent continued with the already present, hash-verified source/installer
  and completed the prerequisite repair without that transfer.
- Sandbox network access prevented a direct local public endpoint fetch;
  the web tool successfully retrieved the metadata. No inference was attempted.
- Exact source transfers were initially rejected by automatic approval review.
  The owner subsequently approved both source digests; each bounded native
  build completed and the earlier artifacts remain preserved.
- Host inspection found that UID 2108 belongs to the privileged deployment
  account. The edge role, preflight, tests and runtime configuration now use
  dedicated UID 2109; collision checks run again before account creation.
- The first full offline-check invocation omitted the locked environment from
  `PATH`; its Python checks passed before Ansible discovery failed. The complete
  command then passed with the locked environment on `PATH`. Agent context
  structural checks also passed.

The [disabled edge installation](u05-edge-disabled-install.json) passed on
Ubuntu, including unchanged repeat converge, exact binary hash, dedicated
identity and inactive/disabled/no-token checks. The six service binaries and
private configuration are installed; account bootstrap generated actual IDs for
exactly the approved three identities. [Service startup](u04-private-services.json)
and [runner limits](u04-runner-runtime.json) now pass. The real reconciler and
initial controller snapshots activated all three accounts. The owner is allowed
through private authorization; the other user and admin are denied its workspace.

The first actual A start was recorded as unknown with no container created.
Read-only inspection found that the existing rootless Docker namespace's
copied runtime directory hid the later bounded private mount. The
[scoped repair](u04-rootless-run-mount-repair.json) made that mount visible
without restarting Docker or changing the two W1 containers. After exact
job-hash review, an operator aborted the unknown start without replay and
verified the durable failed state and advanced generation. Fresh one-shot A
and B starts then completed. The [private workspace receipt](u04-private-workspaces.json)
records both real terminal HTTP responses, service UID/socket and cross-owner
checks, actual container limits/mounts, zero pending jobs and collector
non-reviewer denials. No dataset grant or capture event existed in this
snapshot. Public browser and live-provider results remain separate.

Next: complete fake capture/archive and ordinary stop/start on the deployed
Ubuntu release, then the approved bounded live smoke and verified Mac copy.
The [public activation record](u05-public-activation-proposal.md) retains the
initial failures and scoped repairs; [browser acceptance](u05-browser-acceptance.json)
records the 25-case matrix and disabled-account disconnect. Preserve private
session material and keep extended expired-token, capacity, reboot and cloud
acceptance separate.

## Owner follow-up: usable terminal, complete roster and Git delivery

On 2026-09-25 the owner requested a viewport-filling, larger terminal, complete
access setup for every identity in the private YAML roster, and commit/push/merge
to main including the guided tutorial and UI improvements already on main. This
authorizes the full configured cohort, including explicit restoration of the
second staging identity disabled in the U.05 test. It does not authorize email
invitations or expand the existing five-task paid smoke allocation.

Implementation is in progress: responsive terminal fitting, explicit account
restoration and additional workspace provisioning, and integration of main's
guided tutorial with hosted capture. The live smoke remains unstarted while
the updated release is prepared. Roster identities and account mappings remain
private; public evidence will report verified counts and role isolation.
