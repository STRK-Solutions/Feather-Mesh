# Functional Ubuntu execution — 2026-09-25

The updated release is deployed at [the participant portal](https://feam.613202690.xyz).
The [operator handoff](ubuntu-operator-handoff-20260925.md) supersedes the stopped
[session handoff](ubuntu-session-handoff-20260925.md). No subagents were used in
this resumed execution. Unrelated `.DS_Store`, `.ansible/`, raw evidence and
private credentials/roster remain excluded from Git.

- [x] Guided empty-tool transport regression tests and implementation committed.
- [x] Exact native Go/Rust builds and immutable image smoke pass.
- [x] Stopped installation preserves prior files, grants, W1 slots and reset data.
- [x] Four admins/seven users are active after exact real Access reconciliation;
  all seven workspaces have the approved climate release.
- [x] Two real participant sessions renew through normal SSO, start workspaces,
  read with native Polars/Rasterio, and exercise live model/review/guide behavior.
- [x] Eleven live requests settle at US$0.003119; non-synthetic capture correlates
  every request and usage receipt. One model handle failure is preserved.
- [x] Ordinary maintenance preserves the allocation/spend and research archive;
  private Mac copies independently verify retained objects and provenance.
- [x] Operator instructions and sanitized acceptance evidence prepared.
- [ ] Fresh admin browser walkthrough: expired saved session; owner cannot
  complete email-PIN login. Earlier admin browser evidence remains separate.
- [ ] U.G final acceptance: depends on that unavailable check.

[Updated release and validation](u07-updated-release.json),
[live results and limits](u06-live-functional-smoke.json), and
[maintenance/copy](u07-live-maintenance.json) are the current receipts.
The workplan owns U.01–U.07/U.G status. Git publication is authorized and the
final response records remote-main verification. No invitation, reboot,
destructive teardown, cloud-host or R2 operation was performed.

## Historical execution record

The following sections retain earlier attempts and prerequisites. Statements
about pending roster deployment, the initial US$1 proposal, absent allocations
or pre-activation state describe those checkpoints, not the deployed state above.

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
snapshot. This paragraph records the earlier disabled-stage checkpoint; U.05
above records the later public activation.

## Historical private operator inputs and human checks

The owner selected an existing admin and supplied a new regular staging
identity and a second staging alias in this conversation. The ignored private
roster now has four admins and seven regular users; the encrypted vault's
earlier revision 10 retained prior revisions. Two regular staging identities
were available through the owner's inbox. The initial bootstrap admitted only
three staging identities; the owner later authorized the full private roster
and restoration of the disabled alias, with deployment still pending. The owner
separately approved the initial group reconciliation in
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

Next: integrate the updated native release and full private roster, then the
approved bounded live smoke and ordinary live stop/start. Fake capture/stop and
the first independently verified Mac copy are complete under U.03.
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
