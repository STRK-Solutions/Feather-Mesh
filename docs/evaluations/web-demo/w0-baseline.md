# W0 baseline and execution evidence

Started 2026-09-24; completed 2026-09-25. Status: W0 complete, including W0.06/W0.G after owner-run privileged evidence review.

## Checkout and scope

- Feature branch: `feat/ubuntu-web-demo-w0`.
- Starting revision: `0b5b222a246242524e51c2443c39a4e7962ab9b2`.
- Pre-existing tracked edits: `.DS_Store`, `AGENTS.md`, `README.md`,
  `docs/ubuntu_web_dev_demo_design.md`, `docs/ubuntu_web_dev_demo_workplan.md`.
  Pre-existing untracked file: `docs/ubuntu_web_dev_demo_options.md`.
  Preserved; no cleanup, reset or rebase.
- Existing CLI source/README expose `init`, `serve`, `search`, `show`,
  `consume`, `lineage`, `validate-metadata`, `teams`, `products`, `refresh`,
  `cache`, `resolve`, `withdraw`, `stac`, `tui`. The optional hosted TUI and
  supported Python SDK already exist; no web service exists at this baseline.
- Initial work authorized: W0 by the owner's request; read-only SSH through the
  owner-supplied target. The owner subsequently explicitly requested VM setup
  on Ubuntu; that bounded scope and evidence are recorded below. No sudo host
  bootstrap, host reboot, billable inference, external configuration or paid
  allocation performed.
- The first branch creation was denied by the filesystem sandbox. The same
  scoped operation succeeded through approved escalation, before implementation.

## MAC baseline

macOS 26.6.2 / ARM64; Rust/Cargo 1.94.0; Python 3.13.7;
Node 22.19.0/npm 10.9.3. Go, Terraform, Ansible, Docker, Lima, Multipass and
QEMU were absent initially. This Mac cannot supply native x86-64 Linux or
full-system Ubuntu evidence by itself.

| Check | Result / limitation |
| --- | --- |
| `cargo test --workspace --all-features --offline` | Initial sandbox attempt failed five fake-provider HTTP tests because localhost socket binds were denied. Retained as a failed attempt; no live request occurred. |
| Same command with localhost permission | Passed: 81 tests, zero failures. No Rust source changed. |
| `cargo fmt -- --check` | Passed. |
| `cargo clippy --workspace --all-targets --all-features --offline -- -D warnings` | Passed. |
| Isolated development tools | Official Go 1.27.1 darwin/arm64 archive checksum verified; installed under `/private/tmp/feam-web-w0-tools`. Python tools installed in `/private/tmp/feam-web-w0-venv`. Version/lock strategy in [toolchain](../../../infra/demo/toolchain.md). |

Original local logs: `/private/tmp/feam-w0-rust-tests.log`,
`/private/tmp/feam-w0-rust-tests-unsandboxed.log`, `/private/tmp/feam-w0-clippy.log`.
These temporary logs are not durable acceptance artifacts; final summarized
counts and artifact hashes below are the handoff record.

## Actual Ubuntu preflight

The repository's [read-only probe](../../../infra/demo/scripts/preflight.py)
was streamed to Python over strict-host-key, batch SSH. The sandbox blocked
the initial private-network attempt; approved network execution succeeded.
Raw JSON and stderr are held privately in `.local/demo-deployment/` with mode
0600; no raw inventory is committed. Observation time: 2026-09-25 UTC.

| Observation | Result / provisioning consequence |
| --- | --- |
| OS/kernel | Ubuntu 24.04.5, Linux 7.0.0-34-generic, x86-64; systemd 255.4-1ubuntu8.17. |
| CPU/memory | 12 available CPUs, 16,682,749,952 bytes RAM; 13,893,496,832 bytes available at sampling. 4,294,963,200 bytes swap, unused. These are observations, not capacity acceptance. |
| SSD mapping | `/home` is ext4 on `/dev/sdb2`, parent non-rotating `/dev/sdb`, 2,000,398,934,016 bytes. Separate 1 TB device is out of scope. |
| Free storage | `/home`: 1,633,136,242,688 bytes / 109,758,658 inodes available. `/`: 145,498,554,368 bytes / 11,724,670 inodes available. Root cannot hold the full service allocation. |
| Filesystem identity | UUID recorded privately; SHA-256 `28b516b9bb4cdb7ddfc30f0d8146e59717e6167938722fcf7d6d94a1e5be28c7`. Deployment must use the actual UUID from private configuration, not this digest. |
| Service scope | `/home/feam-service-data` absent; all seven proposed service accounts absent at preflight. Privileged review below confirms this dedicated future scope; recheck identities/mount ownership before W1 bootstrap. No whole-disk formatting or personal-home ownership changes. |
| Subordinate IDs | Existing UID/GID range is `[100000,165536)`. Proposed runner range `[200000,265536)` is non-overlapping at this observation; recheck immediately before creation. |
| cgroups | v2; root advertises cpu/memory/pids and others. Current login user manager has cpu/memory/pids and Delegate=yes. This does not prove delegation for the future runner. |
| Namespace policy | AppArmor enabled; restricted unprivileged user namespaces enabled, user namespace cloning enabled. Do not disable host restrictions. |
| Docker | Existing daemon/socket/containerd active and enabled. Docker CE/CLI/rootless extras 29.8.1; containerd.io 2.3.5. `uidmap` absent. Existing daemon is out of provisioning scope. |
| Workload visibility | Initial unprivileged Docker inspection and noninteractive sudo were denied; those attempts left inventory unknown. The separate owner-run transcript reviewed below reports no container rows/errors and closes the gap. |
| Linux test target at initial preflight | `/dev/kvm`, system QEMU and libvirt absent. `cc`/`make` present; Rust/Cargo/Go absent on PATH. The later isolated VM/native-build setup and successful proof are recorded below. |
| Egress | TLS HEAD requests reached Docker packages/registry, GitHub, PyPI, Go, OpenRouter and Cloudflare API (HTTP 200/301/404, depending on endpoint). This proves outbound TLS reachability only, not authentication, registry pulls, Tunnel connectivity or model compatibility. |

The first time-sync probe returned no properties despite exit 0. The corrected
second read-only run used repeated `--property` flags and confirmed
`NTPSynchronized=yes`, `Timezone=America/Toronto`. Both private JSON attempts
remain available (`w0-preflight.json` and `w0-preflight-v2.json`). The second
probe script SHA-256 is
`a65ff6ae29dc21370dd8573485a5aebebd83193408f9489c632af375c8716425`.
Package origins confirm Docker's Noble/stable amd64 repository and Ubuntu's
Noble updates/security repositories for namespace dependencies.
Package-query exit 1 reflects missing packages; service-account lookup exit 2
and service-path stat exit 1 reflect absence. Neither is silently treated as
successful provisioning. Loop-device presence alone is not a mounted-volume test.

## Remaining environment gates

W0.05 passed with the guest/native proof below. The owner's subsequent Docker,
loop and AppArmor output closes W0.06/W0.G. W1 runtime/isolation proof, reviewed
bootstrap and all later deployment gates remain separate. The earlier denied
commands remain failures; they are not relabelled as successful inspections.

## Owner-authorized disposable VM and native build

After the missing VM was reported, the owner requested: “setup the VM on the
ubuntu machine”. The repository's [VM manager](../../../infra/demo/scripts/disposable_vm.py)
creates only `~/feam-w0-vm`, mode 0700, with an ownership marker. Ubuntu QEMU
8.2.2 packages and required libraries are downloaded through the existing APT
metadata, checksum-verified and extracted locally; no host package is installed
and no maintainer script, sudo or host namespace-policy change is used.

The Ubuntu 24.04 cloud image is pinned to release `20260911`, SHA-256
`612b2c0cc1bc413a6cb8c38fd611794caf0f2b436c50013d8b3794db12ad7354`.
Its checksum list's GPG signature passed with Ubuntu's installed cloud-image
keyring. The VM uses 2 vCPUs, 2 GiB RAM, a 16 GiB maximum QCOW2 disk, QEMU TCG,
serial output and SSH bound to Ubuntu localhost port 22371 only. Disposable
guest/host keys are generated inside the private scope; guest SSH verifies the
provisioned host key, with no forwarded agent or host credential in the guest.

Retained startup attempts: initial extracted QEMU could not locate its TCG
module; adding its private module directory resolved that. The next attempt
requested an unavailable VGA ROM; disabling VGA for this serial-only guest
resolved that. The third start succeeded. These were failures before guest
execution, not failed systemd/mount acceptance. No global package/loader change
was used. Guest proof passed with exit 0 on Linux 6.8.0-139-generic x86-64,
`systemd-detect-virt=qemu`, cloud-init `done`. The 64 MiB loop filesystem UUID
and remount contents matched, and the transient systemd cgroup exposed the
expected memory/PID ceilings. These prove W0 access and plumbing, not W1
rootless enforcement or W9 performance. The guest was gracefully powered off;
final manager status is `stopped`. Repeat `prepare` refused the existing disk.

A separate [native build](../../../infra/demo/scripts/native_build.sh) runs
under the existing user's transient systemd unit with CPUQuota=200%, MemoryMax=4G,
TasksMax=256 and Nice=10. It installs Rust only inside the owned W0 scope and
builds the archived starting revision, with no personal Cargo/profile changes.
This distinguishes native x86-64 compilation from the software-emulated VM;
neither constitutes production performance acceptance. Build passed in 2 min
38.466 seconds including isolated tool installation, with a successful CLI
`--help` smoke. Binary is an x86-64 Linux ELF; SHA-256
`6d21243cdac07ddf60fc92aae1b79e6e39e80c16444afca444ae2a9824db50c3`.
Source archive SHA-256 is
`69f9c9cbe01057a569886e8b9dbb79e3236ffb08cbdc4816fba71b3096945be7`.
[Artifact provenance](w0-linux-artifacts.json) records tool/image/package hashes.
The final package lock was compared against all 16 downloaded archives; all
matched. Initial and delivered VM-manager hashes are separate in that record.

## Local validation and retained failures

- 18 schema/semantic tests pass on MAC Python 3.13 and native UBUNTU Python 3.12,
  including unknown reservations, duplicate
  allocations, verified closure, stopped/public conflict, unarchived teardown,
  forbidden lifecycle arguments and format enforcement without optional extras.
- Ansible syntax and offline lint pass with zero lint failures/warnings.
  Initial checks attempted the default `~/.ansible` cache and failed under the
  sandbox; the check runner now supplies isolated temporary Ansible directories.
  Actual `ansible-playbook` against the private Ubuntu inventory passed with
  `ok=2 changed=0 unreachable=0 failed=0`; denied sudo probes remain visible
  exit codes. A successful playbook does not turn those probes into successes.
- Context structural validation and all 13 regression tests pass. The first
  run after adding web/infra routing exposed missing directories in isolated
  test fixtures; extending the fixture copy fixed it without weakening checks.
- Shell syntax passes for the VM probe; native-build syntax is included in the
  final check command. Real VM execution is recorded separately.
- The first native Python venv attempt failed because Ubuntu `ensurepip` was
  absent. A `--without-pip` venv plus the SHA-256-pinned official pip 26.2.1 wheel
  resolved this entirely inside the W0 scope; locked native checks then passed.
- No browser, rootless sandbox, live-model, EDGE, cloud capacity or HPC test is
  claimed by these local checks. Browser binaries/Terraform provider modules
  remain deferred until their consuming implementation exists.
- Existing installed-SDK/STAC-reader, PTY, manual/fake walkthrough, held-out
  evaluation and no-default-feature matrices were not rerun for W0; their source
  is unchanged. The 81-test Rust result is the all-feature baseline only.

Final documentation/source checks: all local links/anchors across the ten
affected documentation files pass, `git diff --check` passes, and all 96 stable
workplan task IDs remain unique (all eight W0 tasks and W0.G complete after the
owner-run closeout below; all W1–W10 tasks remain unchecked).
[Source artifact hashes](w0-source-artifacts.json) pin the 29 delivered
contract/schema/config/tool/CI files independently of the uncommitted worktree.
W0 adds no Rust source change and leaves the prior dirty files/Stage-1 records
intact. The native release remains under the private Ubuntu W0 build directory;
the guest and its source/image/package inventory remain stopped and reusable.

## Owner-run privileged preflight and W0 closeout

On 2026-09-25 the owner supplied the requested terminal output from the actual
Ubuntu host. This is owner-run evidence reviewed on MAC, not a newly executed
agent sudo session. The full transcript is retained only at
`.local/demo-deployment/w0-owner-privileged-preflight.txt` with mode 0600;
SHA-256 `d1eb5facafddbe11b63ab103ebddada5b125630298498fe7dcf3f47760dfe029`.
The [sanitized receipt](w0-privileged-preflight.json) records exact commands,
observations and scope without the personal process/application inventory.
The transcript has no captured exit codes or embedded command timestamp; no
command error is shown, and successful Docker info follows the empty container
listing. These limits do not require repeating the owner's successful inspection.

| Owner-run command | Observation / decision |
| --- | --- |
| `sudo docker ps -a --format '{{.ID}} {{.State}}'` | No container rows or reported errors. Treat as an empty owner-observed Docker inventory at this inspection, independently of the earlier denied attempt. |
| `sudo docker info --format 'root={{.DockerRootDir}} driver={{.Driver}} cgroup={{.CgroupVersion}}'` | `/var/lib/docker`, `overlayfs`, cgroup v2. Preserve this daemon/data root; future FEAM runtime uses its dedicated runner and data path. |
| `sudo losetup --list --output NAME,BACK-FILE` | 21 loop devices, all backed by Snap files under `/var/lib/snapd/snaps/`; no FEAM backing files reported. Existing loops are out of scope. |
| `sudo aa-status` | Module loaded; 158 profiles: 62 enforcing, 5 complain, 91 unconfined; 19 profiled processes, 18 enforcing and one unconfined. `docker-default` is enforcing; `rootlesskit` is listed in unconfined mode. Preserve host policy. These observations do not prove future runner namespace creation or per-container AppArmor confinement. |

Together with the measured filesystem UUID, free space, dedicated-path/account
absence, non-overlapping proposed subordinate IDs, time sync and egress, this
establishes W0's safe future provisioning scope. W0.06 and W0.G are complete.
W1 must recheck current ownership/collisions and prove the runner's actual
namespace/resource/socket/storage behavior before any demo service acceptance.
No host mutation, public route, inference or paid resource was needed for this
closeout. W1.01 is the next task, outside the owner's W0-only request.

The checkout was on `docs/demo-saifshaikh-domain` when the evidence arrived;
that branch and `feat/ubuntu-web-demo-w0` had the same base revision. Resumed the
W0 feature branch while preserving all intervening domain-plan and other dirty
edits, including the selected `demo.saifshaikh.ca` route. Closeout checks passed:
context structure, all 13 context regressions, local links/anchors across ten
documents, all 96 unique task IDs with exactly the nine W0 entries checked,
all 29 source-artifact hashes, private transcript digest/mode and
`git diff --check`. The previous operator-guide digest is retained in the
source manifest history. No application code changed in this closeout; prior
runtime results remain the earlier measured evidence, not new test runs.

During final verification the shared checkout moved again to
`feat/demo-domain-613202690` at the same base revision. Left that concurrent
branch selection untouched; the W0 closeout remains in the shared uncommitted
worktree. No commit, reset or cleanup was performed.

## Version-control handoff verification — 2026-09-25

The owner requested pushing and merging W0 before W1 implementation. Resumed
`feat/ubuntu-web-demo-w0` from the shared W0 worktree; excluded the unrelated
`.DS_Store` edit and all ignored private deployment inputs. The current selected
domain is `613202690.xyz`; the historical branch/domain observations above are
retained as history. Refreshed the operator-guide hash after that later domain
update, retaining its prior hash in the source manifest.

Fresh MAC verification passed: 18 contract tests, all five sanitized records,
Bash syntax, Ansible syntax/offline lint, context structural checks, 13 context
regressions, and `git diff --check`. All 29 source hashes match. Runtime source
is unchanged; previous Rust, full-VM and Ubuntu results remain dated evidence.
GitHub PR checks and the merge are recorded on the pull request; this local
verification does not itself establish remote CI success. No W1 implementation
or host provisioning is included in this handoff.
