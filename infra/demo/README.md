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
The actual host denies noninteractive sudo. The owner supplied these exact
read-only commands' output for W0; the [reviewed evidence](../../docs/evaluations/web-demo/w0-privileged-preflight.json)
closes that gate. Use the same commands when a fresh inventory is needed before
later provisioning; keep raw workload details private:

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
| I1 | W0 preflight and W1 owner-run scoped bootstrap passed. Actual runner enforcement, browser manual/fake flows, persistent storage, lingering and unchanged converge are verified. Private inventory/evidence retained locally. Shared-host reboot remains later; no sudo password is needed by the agent. |
| I2 | Owner switched back to `feam.613202690.xyz`, with `admin.613202690.xyz` and `u-<opaque-id>.613202690.xyz` siblings, and confirmed Cloudflare Active status and completed Zero Trust onboarding. Public DNS returns the assigned nameservers. Account MFA confirmation, scoped credentials, route/HTTPS/Tunnel/Access configuration and verification remain W8; retain renewal terms for handoff. |
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
