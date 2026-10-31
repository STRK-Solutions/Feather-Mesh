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

Obtain their preferred username and public SSH key. The
[developer enrollment playbook](ansible/developer-access.yml) creates a separate
login with a home directory and Bash shell. It refuses unmanaged existing
accounts and preserves any existing authorized keys on an enrolled account.
Keep the actual username/key in a private variables file:

```yaml
feam_developer_name: chosen_username
feam_developer_public_key: 'ssh-ed25519 REPLACE_WITH_PUBLIC_KEY optional-comment'
feam_developer_full_sudo: true
```

Use the existing deployment inventory with `ansible_user: feam-deploy`, its
private key, strict host checking and noninteractive sudo:

```bash
ansible-playbook -i /private/operator/inventory.yml infra/demo/ansible/developer-access.yml -e @/private/operator/developer-access.yml
```

The playbook validates the public key before creating the account, validates
sudoers before installing it, then verifies root sudo under the developer's
identity. A second run should report `changed=0`. The account configuration is:

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
