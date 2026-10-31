# Tailscale developer handoff: owner procedure

Developer-facing steps are in [DEVELOPER_ACCESS.md](DEVELOPER_ACCESS.md).
Store the real address, username, invitation and fingerprints in private handoff
material under `.local/demo-deployment/tailscale-access/`; that directory is
ignored by Git. Never copy an owner's private SSH key to another developer.

## Connect the Ubuntu node

From the existing verified LAN SSH session, inspect and connect:

```bash
systemctl is-active tailscaled
systemctl is-enabled tailscaled
sudo -n tailscale up --timeout=30s
tailscale status
tailscale ip -4
sudo -n tailscale debug prefs
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Complete any authentication URL using the owner's existing Tailscale account.
The node must report `Running` and an assigned Tailscale address before handoff.
Review `debug prefs` privately: use ordinary OpenSSH over Tailscale with
`RunSSH=false`, incoming connections allowed, and no advertised subnet/exit-node
routes. Preserve unrelated network settings. Do not publish the raw preferences.
If the service is disabled/stopped, `sudo systemctl enable --now tailscaled`
restores its boot configuration without rebooting the host.

In the [Machines console](https://login.tailscale.com/admin/machines), identify
Ubuntu by its assigned Tailscale IP, choose **Share**, and create a single-use
invite for the intended developer. Share it privately. Verify the accepting
identity in the Share dialog and ensure policy permits that identity to reach
this node on TCP 22; a share remains subject to both tailnets' access policies.
Review the node's key-expiry date and arrange renewal before it expires.

## Enroll the developer's Ubuntu account

Obtain their preferred username and public SSH key. Use the host's reviewed
Ansible configuration to create a separate login with a home directory and Bash
shell. Keep it separate from `feam-deploy`, the owner and runtime service users.
For that account, configure:

| Item | Required value |
| --- | --- |
| Home `.ssh/` | Owned by the developer; mode `0700` |
| `.ssh/authorized_keys` | Their public key; developer-owned; mode `0600` |
| Password login | No password issued; authenticate with their SSH key |
| `/etc/sudoers.d/90-feam-dev-USERNAME` | `USERNAME ALL=(root) NOPASSWD: ALL`; root-owned; mode `0440` |

Validate the sudoers file with `visudo -cf FILE` before installation, then run
`sudo visudo -c`. Check for an existing account before provisioning; do not take
over another user's home or replace existing authorized keys without review.
Root sudo grants access to all host files and service credentials. This is the
same host privilege level as the owner's deployment account.

Send the developer their username, Tailscale IPv4 address, ED25519 fingerprint,
private single-use invite and the developer runbook. The public-key step can be
completed after they follow the runbook to generate their key.

## Verify and revoke

Have the developer connect from outside the LAN and run the runbook's SSH/sudo
checks. Record that result separately from an owner-side listener check. Confirm
the FEAM service states before and after changes; do not reboot or restart the
demo merely to enroll an SSH user.

To revoke, remove their machine share and sudoers entry, remove their authorized
key, validate sudoers, and arrange termination of their active sessions/processes.
Review access through any other tailnet membership or network path as well.
Removing a key or share alone does not terminate established root processes.

References: [Tailscale sharing](https://tailscale.com/docs/features/sharing),
[Tailscale CLI](https://tailscale.com/docs/reference/tailscale-cli),
[Ubuntu SSH](https://ubuntu.com/server/docs/how-to/security/openssh-server/).
