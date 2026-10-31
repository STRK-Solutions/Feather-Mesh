# Ubuntu remote developer connectivity — 2026-10-31

Status: **Ubuntu enrolled and online; the individual developer account is
provisioned with verified root sudo. External SSH/sudo acceptance remains pending.**

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
The owner subsequently supplied a public key and confirmed the developer's
username. Their individual account and root sudo rule are now provisioned as
described below. External developer connectivity, sharing-policy acceptance and
authentication with the developer-held private key remain pending. Browser-demo
acceptance and existing U/W gate status are unchanged.

MAC documentation checks passed: context structural validation, all 13 existing
context-checker regressions, local runbook links, Bash snippet syntax and
`git diff --check`. The initial default-Python checker attempt lacked PyYAML;
validation passed in an isolated temporary environment with the repository's
pinned PyYAML 6.0.3. Go and Rust suites were not rerun for this initial
networking/documentation snapshot. Provisioning validation below adds the full
infra/demo offline check entrypoint.

## Individual account provisioning follow-up

The checked-in [Ansible playbook](../../../infra/demo/ansible/developer-access.yml)
owns account enrollment. Inputs remain in owner-only inventory/variables files.
It validates the key before mutation, refuses unmanaged account/path collisions,
preserves existing authorized keys, validates sudoers before installation and
checks root sudo under the developer identity.

UBUNTU evidence from 2026-10-31 17:57 UTC:

- Initial account/home/group/sudo-rule collision checks found no existing target.
- Six check-mode rejection cases passed with zero changes: root and runtime
  identities, an unsafe username, missing root delegation, a multiline key and
  an invalid key blob.
- Initial converge passed with eight changed tasks. Repeat converge passed with
  nineteen successful tasks, zero changes, zero unreachable hosts and no failures.
- The installed public key matches the supplied ED25519 fingerprint. Its default
  comment is preserved and has no authentication role.
- The account's Bash home and SSH directory are developer-owned with mode 0700;
  authorized keys are mode 0600. The password is locked. The root-owned sudoers
  rule is mode 0440 and global `visudo -c` succeeds.
- Running `sudo -n id -u` through `runuser` as the developer returns `0`.
  This verifies host-side sudo; it is not an SSH private-key challenge.
- Newly allocated subordinate UID/GID ranges do not overlap existing identities.
  All seven FEAM service PIDs/states still match the pre-enrollment baseline,
  and Tailscale remains running with no health warnings.

MAC validation: `python infra/demo/scripts/check.py` passed in the locked tooling
environment, including the new playbook's syntax check, all offline suites,
schema checks and lint across eighteen Ansible files. The CI check entrypoint now
includes this playbook. No Go or Rust implementation changed. The external
developer must still complete the runbook's actual SSH/sudo checks.
