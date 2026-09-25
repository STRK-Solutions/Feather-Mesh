# Demo operator baseline

W0 introduces read-only preflight, sanitized examples, validation and a bounded
disposable-VM probe. W1 adds scoped account/storage/rootless-runtime roles, a
terminal image and the private operator proxy; see the [W1 runbook](W1.md).
Production lifecycle services and Terraform modules remain planned. See [contract](../../docs/ubuntu_web_dev_demo_contract.md),
[toolchain](toolchain.md) and [acceptance](../../docs/ubuntu_web_dev_demo_acceptance.md).

## Local checks

Run from the repository root after the [development install](../../web_demo/README.md):

```bash
python infra/demo/scripts/check.py
```

This runs offline schema/semantic tests, Bash syntax, Ansible syntax and offline
lint. It uses temporary Ansible/cache directories instead of the user's home.
No inventory connection or VM mutation is performed by this check command.

## Private read-only preflight

Copy [example.yml](ansible/inventories/example.yml) into an operator-private
inventory and replace the reserved placeholder host/user. Verify the SSH host
key independently; strict host checking stays enabled. Do not use a real
inventory in public CI. Preserve the private roster's four admins/five users
and existing consent; it is not a broadly matching public allowlist.

```bash
export ANSIBLE_HOME=/tmp/feam-web-w0-ansible
export ANSIBLE_LOCAL_TEMP=/tmp/feam-web-w0-ansible/tmp
ansible-playbook -i .local/demo-deployment/inventory.yml infra/demo/ansible/preflight.yml
```

The playbook suppresses raw output, reporting only probe exit codes. For a
private evidence file, stream the script directly over verified SSH:

```bash
umask 077
ssh -o BatchMode=yes -o StrictHostKeyChecking=yes operator@ubuntu.example.invalid \
  'python3 -' < infra/demo/scripts/preflight.py > .local/demo-deployment/preflight.json
```

No packages/accounts/mounts/services are changed. Expected missing resources
and permission-denied probes must be reviewed, not converted to empty results.
W0 preceded delegated sudo access, so the owner supplied these exact read-only
commands' output; the [reviewed evidence](../../docs/evaluations/web-demo/w0-privileged-preflight.json)
closes that gate. The deployment account now supports noninteractive sudo.
Use the same probes with `sudo -n` through that account when a fresh inventory
is needed before later provisioning; keep raw workload details private:

```bash
sudo docker ps -a --format '{{.ID}} {{.State}}'
sudo docker info --format 'root={{.DockerRootDir}} driver={{.Driver}} cgroup={{.CgroupVersion}}'
sudo losetup --list --output NAME,BACK-FILE
sudo aa-status
```

These inspect existing resources; they are not the W1 bootstrap. A failed
Docker command is unknown inventory, not zero containers. Reviewed future mutation
scope: only `/home/feam-service-data` on its verified private UUID,
the seven named FEAM service accounts, dedicated systemd/socket configuration
and a non-overlapping runner subordinate-ID range. Recheck collisions before
mutation; preserve the current Docker daemon and other disk. No host changes
are authorized by example placeholders alone.

## Unattended Ubuntu deployment access

Verified on 2026-09-25 after owner setup: the dedicated `feam-deploy` account
authenticates with the operator Mac's `~/.ssh/feam_ubuntu_deploy` key, and
`sudo -n id -u` returns `0` without prompting. See the
[access record](../../docs/ubuntu_web_dev_demo_workplan.md#ubuntu-deployment-access-verified-2026-09-25).
The account has full host-root capability, not a command-restricted helper.
Use it only for authorized work within the reviewed FEAM scope. Reboot timing,
public exposure, invitations, purchases and live inference retain their own gates.

Read the verified host address from private inventory, then check access from MAC:

```bash
ssh -i "$HOME/.ssh/feam_ubuntu_deploy" \
  -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes \
  -o ConnectTimeout=10 feam-deploy@"${FEAM_UBUNTU_HOST:?set from private inventory}" \
  'id -un; sudo -n id -u'
```

For delegated Ansible operations, configure the private inventory with
`ansible_user: feam-deploy`,
`ansible_ssh_private_key_file: ~/.ssh/feam_ubuntu_deploy`, and SSH options
`-o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o BatchMode=yes`.
The host/deploy playbooks already use `become: true`; no interactive sudo
password handoff is needed. These are inventory instructions, not a claim that
the new account has already run a full Ansible converge.

Keep this deployment login separate from the seven runtime service accounts.
Preserve W1's recorded `feam_operator_uid`, browser socket ACL and private output
directory ownership when changing the Ansible login. The key and real host
address stay out of Git, images, public CI and logs. No password or key contents
need to be supplied in chat. Keep the original owner login for recovery.

When delegation ends, the owner can remove its sudo rule and future key access
from the original Ubuntu account:

```bash
sudo rm /etc/sudoers.d/90-feam-deploy
sudo rm /home/feam-deploy/.ssh/authorized_keys
sudo visudo -c
```

This does not terminate existing sessions or already-running root commands;
finish or stop those separately before considering delegation fully revoked.

## Disposable Linux target

The new CI workflow selects native `ubuntu-24.04` x86-64 for locked Rust builds
and W0 checks. A workflow definition is not a measured successful run. Hosted
CI covers compilation and offline application checks. It is not treated as
proof of full-system first/second converge, rootless boot, mount loss or reboot.

The owner authorized a local disposable VM on Ubuntu during W0. The tested
[VM manager](scripts/disposable_vm.py) extracts checksum-verified Ubuntu QEMU
packages from [the package lock](scripts/ubuntu-vm-packages.lock.json) into
`~/feam-w0-vm`, verifies the signed/pinned cloud image and creates
one guest with 2 vCPUs, 2 GiB RAM and a 16 GiB maximum virtual disk. It needs no
host sudo/package installation. Because KVM is absent, it uses TCG; that is a
full-system functional target, not a performance benchmark. It refuses an
existing directory without its ownership marker and never reinitializes a disk.

On Ubuntu, with these repository scripts available, initial setup is:

```bash
python3 infra/demo/scripts/disposable_vm.py prepare --probe infra/demo/tests/full_vm_probe.sh
python3 infra/demo/scripts/disposable_vm.py start
python3 infra/demo/scripts/disposable_vm.py status
```

The current prepared copy also resides at `~/feam-w0-vm/source/disposable_vm.py`;
do not run `prepare` again on that existing VM. SSH is bound only to Ubuntu's
loopback port 22371, with separately generated disposable keys and a pinned
guest host key. On Ubuntu, reach the guest with:

```bash
ssh -p 22371 -i ~/feam-w0-vm/guest_access \
  -o StrictHostKeyChecking=yes -o UserKnownHostsFile=~/feam-w0-vm/known_hosts \
  feamtest@127.0.0.1
```

The guest automatically runs the W0 probe once through cloud-init, with exit
status and output under `/var/log/feam-w0-probe.{exit,log}`. For an explicit
repeat, run **inside the disposable guest only**:

```bash
sudo bash /opt/feam-w0/full_vm_probe.sh --confirm-disposable
```

The script refuses non-VM/non-Ubuntu/non-x86-64/non-systemd targets, creates one
64 MiB temporary ext4 loop filesystem, verifies UUID/remount persistence and a
transient constrained systemd unit, then unmounts/removes its own scope. It does
not install dependencies or reconfigure namespaces. W0 execution passed on the
actual QEMU guest; [artifact provenance](../../docs/evaluations/web-demo/w0-linux-artifacts.json)
records the image/package/probe hashes. Reboot and rootless enforcement tests
are W1/W7, not implied by this probe.

For a graceful stop, on the Ubuntu host:

```bash
python3 ~/feam-w0-vm/source/disposable_vm.py stop
python3 ~/feam-w0-vm/source/disposable_vm.py status
```

Wait for `stopped` before any explicit cleanup; the command does not delete
the VM or its keys/evidence. No public port, autostart or replacement host is
configured. Start it explicitly for later integration work.

Native FEAM compilation uses a separate `~/feam-w0-vm/native-build` scope on
the physical x86-64 Ubuntu host. Transfer a reviewed `git archive` of
`feather-mesh/` as `source.tar` with `source.tar.sha256`, then run
[native_build.sh](scripts/native_build.sh) in a bounded user systemd unit:

```bash
systemd-run --user --wait --pipe --collect --unit=feam-w0-native-build \
  --property=MemoryMax=4G --property=CPUQuota=200% --property=TasksMax=256 \
  --property=Nice=10 bash infra/demo/scripts/native_build.sh
```

The script verifies the official Rust 1.94.0 archive checksum and installs only
the needed components into that scope, builds the locked hosted-capable release
with two jobs, and records its digest. A repeated run refuses a changed source
archive against an existing extracted tree. This passed in W0. The repository
also supplies [linux_checks.py](scripts/linux_checks.py) to bootstrap locked
Python checks inside that scope when host `ensurepip` is absent; it verifies a
pinned pip wheel and does not alter system Python.

## Inputs

| Input | Current state / next gate |
| --- | --- |
| I1 | W0 preflight and W1 owner-run scoped bootstrap passed. Actual runner enforcement, browser manual/fake flows, persistent storage, lingering and unchanged converge are verified. Dedicated deployment SSH and noninteractive sudo passed on 2026-09-25; authorized Ubuntu work no longer needs manual sudo handoffs. Private inventory/evidence retained locally. Shared-host reboot still requires an approved window. |
| I2 | Owner selected `feam.613202690.xyz` with `admin` and `u-<opaque-id>` siblings, confirmed Cloudflare Active status/Zero Trust onboarding, and connected Cloudflare to Codex on 2026-09-25. Cloudflare tools are available; use the existing connection first. Account/zone permissions, Terraform/runtime authentication, MFA and route/HTTPS/Tunnel/Access configuration remain unverified. Request only missing scoped capabilities and retain renewal terms for handoff; see the [connection record](../../docs/ubuntu_web_dev_demo_workplan.md#cloudflare-connected-to-codex-2026-09-25). |
| I3 | Operator-held state and project/run allocation contract defined; actual encrypted storage/ledger/archive setup pending W5–W7. |
| I4 | Selected route fixed; key, current prices and finite live budget pending W8. No live inference now. |
| I5 | Cloud account/provider and bounded exact purchase approval pending W9. W0 does not create paid resources. |
| I6 | Existing consent and trace survival confirmed; retention, reviewers, support and Phase 2 owner pending before real capture/export. |
| I7 | Exact source objects, licenses, subsets and checksums pending W4 real imports. |

`settings.json` deliberately marks all deployment inputs pending because a
partial decision is not a completed readiness gate. Keep current private
records outside disposable compute. Never commit real inventories, secrets,
Terraform state/plans, raw traces, roster or private preflight JSON. The existing
`.local/demo-deployment/` ignore rule holds temporary private session evidence;
it is not a substitute for encrypted recoverable operator storage.
