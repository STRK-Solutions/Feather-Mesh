# Ubuntu developer access

Connect through Tailscale, then use your own Ubuntu SSH account. This account has
full root sudo access, including permission to deploy the web demo and restart its
services. The website's browser admin role is separate.

## 1. Install and connect Tailscale

Install [Tailscale](https://tailscale.com/download) on your computer and sign in
with your own account. Open the Ubuntu machine-sharing invitation from Saif and
accept it using that account. Keep Tailscale connected while working.

## 2. Generate your SSH key

On macOS/Linux, run:

```bash
mkdir -p "$HOME/.ssh"
ssh-keygen -t ed25519 -f "$HOME/.ssh/feam_ubuntu" -C "your-name-feam"
cat "$HOME/.ssh/feam_ubuntu.pub"
```

On Windows, run in PowerShell:

```powershell
New-Item -ItemType Directory -Force "$HOME/.ssh" | Out-Null
ssh-keygen -t ed25519 -f "$HOME/.ssh/feam_ubuntu" -C "your-name-feam"
Get-Content "$HOME/.ssh/feam_ubuntu.pub"
```

Choose a passphrase. If that key file already exists, reuse its public key or
choose another filename instead of overwriting it. Send Saif the `.pub` line,
your preferred username, and the email you use for Tailscale. Keep the private
key and passphrase on your computer.

## 3. Connect after Saif confirms enrollment

Saif supplies your Ubuntu username, its Tailscale IPv4 address and its ED25519
SSH host-key fingerprint. Replace `USERNAME` and `TAILSCALE_IP` below:

```bash
ssh -i "$HOME/.ssh/feam_ubuntu" -o IdentitiesOnly=yes -o HostKeyAlgorithms=ssh-ed25519 USERNAME@TAILSCALE_IP
```

On the first connection, compare the displayed SHA256 fingerprint with the one
Saif supplied before accepting it. Enter your local key passphrase if prompted.
Use the Tailscale address when connecting from outside Saif's local network.

Verify your account and admin access on Ubuntu:

```bash
whoami
sudo -n id -u
sudo systemctl is-active feam-gateway.service
```

Expected: your username, `0`, and the gateway's current state. A stopped gateway
does not mean SSH access failed.

## 4. Work on the demo

Use your own repository checkout. Follow the [deployment runbook](W7.md) and
[operator baseline](README.md) to deploy reviewed changes. SSH and sudo permit
host administration; GitHub access, website enrollment, and Mac-held operator
vault/spending state require their own handoff. Coordinate deployments and
service restarts with Saif while the shared demo is in use.

## Troubleshooting

- **Connection timeout:** confirm Tailscale is connected, the Ubuntu share was
  accepted by the same account, and the owner allows TCP port 22 in both tailnets'
  policies. An offline Ubuntu node or expired node key also blocks access.
- **`Permission denied (publickey)`:** check the username and key filename, then
  ask Saif to confirm the public key was installed for that account.
- **Host-key warning:** ask Saif to verify the current host fingerprint before
  changing your known-hosts entry.
- **`sudo` asks for a password:** ask Saif to finish the account's sudo setup.

Owner setup and revocation: [Tailscale owner procedure](TAILSCALE_OWNER.md).
