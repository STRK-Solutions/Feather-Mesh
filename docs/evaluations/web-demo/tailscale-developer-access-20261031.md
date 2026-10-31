# Ubuntu remote developer connectivity — 2026-10-31

Status: **Ubuntu enrolled and online; developer enrollment and external
SSH/sudo acceptance remain pending.**

The owner requested remote developer access equivalent to the Mac's deployment
login and a succinct runbook. The existing Ubuntu Tailscale service was running
and enabled at boot, but the VPN was stopped and had no saved login. The owner
completed browser authentication after the bounded reconnect command.

## Executed evidence

MAC orchestration used the existing dedicated SSH identity, strict host-key
checking, batch authentication and noninteractive sudo. UBUNTU commands included
`systemctl is-active/is-enabled tailscaled`, `tailscale status --json`, a private
filtered review of `tailscale debug prefs`, `sshd -T`, host-key fingerprint
inspection, and `sudo -n tailscale up --timeout=15s`.

The reconnect command initially timed out while awaiting browser authentication;
this was not treated as successful enrollment. Following the owner's approval,
the 2026-10-31 17:29 UTC readback established:

- Tailscale 1.104.4 reports `BackendState=Running`, `Online=true`, assigned
  private tailnet addresses and no health warnings.
- The existing `tailscaled` service is active and enabled at boot. Boot
  configuration was inspected; no reboot test was performed.
- Ordinary OpenSSH responds on TCP 22 at the assigned Tailscale IPv4 address,
  tested from Ubuntu itself. Tailscale's SSH feature remains disabled.
- No subnet or exit-node routes are advertised, and shields-up is disabled.
- All seven FEAM service states and PIDs match the preflight baseline.
- The existing deployment login still returns root UID `0` through `sudo -n`.
  This does not establish privileges for a new developer account.

Owner-only receipts, actual network/identity details, the host fingerprint and
a populated developer quickstart are retained under the ignored, mode-0700
`.local/demo-deployment/tailscale-access/` directory; private files are mode 0600.
The installed node key has an expiry recorded in that private handoff.

## Handoff and limits

The [developer runbook](../../../infra/demo/DEVELOPER_ACCESS.md) includes client
installation, individual SSH key generation, host-key verification, login and
sudo checks. The [owner procedure](../../../infra/demo/TAILSCALE_OWNER.md)
records sharing, account requirements and revocation.

This session has host SSH access but no authenticated Tailscale admin API or
browser connection. The owner confirmed sending the invitation links; recipient
acceptance and sharing-policy access have not been independently verified.
The developer has not yet generated an SSH key; their separate Ubuntu account
and sudo rule have not been provisioned. External developer connectivity,
sharing-policy acceptance, account authentication and sudo verification remain
pending. Browser-demo acceptance and existing U/W gate status are unchanged.

MAC documentation checks passed: context structural validation, all 13 existing
context-checker regressions, local runbook links, Bash snippet syntax and
`git diff --check`. The initial default-Python checker attempt lacked PyYAML;
validation passed in an isolated temporary environment with the repository's
pinned PyYAML 6.0.3. No application code changed, so application suites were not
rerun for this networking/documentation task.
