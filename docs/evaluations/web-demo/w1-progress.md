# W1 implementation and execution record

Status: W1 complete. Work began on `feat/ubuntu-web-demo-w1`, based on W0 merge
`643992a4fd4ad1d72b47e9ce23760e6949c98e0d` (PR #60). W0 was merged at the
owner's request without waiting for remaining CI. Only `.DS_Store` was dirty
after updating the W1 branch; that unrelated edit remains excluded.

Final [actual-Ubuntu acceptance](w1-ubuntu-acceptance.json) passes, including
[Chromium manual/fake exchanges](w1-ubuntu-browser.json), full
[manual](w1-ubuntu-manual.json)/[fake](w1-ubuntu-fake.json) walkthroughs,
[enforcement](w1-ubuntu-enforcement.json), socket separation, persistence,
confined reset, unchanged converge and live lingering without login. W1.01–W1.07
and W1.G are complete. W2–W10, shared-host reboot and target-HPC acceptance are
separate. The chronological entries below retain what was pending or failed at
each attempt; they do not override this final status.

## MAC implementation and measured checks

- Added a Go private-terminal binary with Unix-only listener/backend, a private
  operator credential, exact loopback browser Host/Origin checks, request and
  response header allowlists, fixed routes, bounded HTTP/stream lifetimes and
  two active terminal streams. There is no test identity or production gateway
  mode; W2 owns participant authentication. No Docker authority in this binary.
- Go race tests pass: private configuration, authentication/route negatives,
  header filtering/response size, actual WebSocket relay/connection limits and
  slot release, sanitized unavailable-backend errors. The first sandboxed run
  was denied socket binds; the authorized local-socket run passed.
- Eleven storage/state admission tests pass, including unknown files, symlinks,
  incomplete initialization, wrong filesystem and preserved terminal state.
  The first Mac test needed to isolate macOS's
  symlink `/home` from the intended wrong-UUID assertion; no host was mutated.
- All 18 W0 contract tests, five examples, Bash syntax, Ansible syntax and
  offline lint pass with the expanded check command. Native Linux tests below
  are separate; final actual-Ubuntu service evidence is linked above.
- Image recipe pins Ubuntu amd64 digest, W0 native hosted-capable FEAM binary,
  ttyd 1.7.7 SHA-256 and exact hash-verified Python wheel URLs. Fixture generation
  occurs in the image and practice initialization at the final container path.
- Accounts, storage and runtime roles use seven dedicated identities, a checked
  runner subordinate range, recorded preallocated ext4 files, system mount units,
  a mount-bound runner user manager, bounded socket tmpfs/ACLs, lingering and a
  private rootless daemon. Existing physical-host Docker is outside this scope.

## LINUX attempts (retained, not relabelled as successes)

The retained W0 QEMU Ubuntu 24.04 x86-64 guest was explicitly started for W1.
It remains a two-vCPU, two-GiB TCG functional fixture, not a performance benchmark.
Guest dependencies were installed through the checked guest-only script; no
physical-host package was installed by that operation. Package inventory:
Docker CE/CLI/rootless extras 29.8.1, containerd 2.3.5, uidmap
`1:4.13+dfsg1-4ubuntu3.2`, AppArmor `4.0.1really4.0.1-0ubuntu0.24.04.8`.

- Native Go build in the existing bounded physical-Ubuntu engineering scope:
  first invocation failed because systemd did not inherit the shell working
  directory. The absolute-path rerun passed Go race tests, `go vet` and build.
  This is build evidence, not acceptance of a deployed Ubuntu terminal.
- Guest converge 1 stopped before allocation: play-variable precedence selected
  the 20-GiB physical runtime size instead of the 4-GiB guest size. Headroom
  admission rejected it. Corrected precedence and retained the error.
- Guest converge 2 allocated the recorded files but refused malformed systemd
  mount names caused by escaped hyphens. Replaced expression-based escaping with
  explicit reviewed unit names. Converge 3 preserves recorded files/UUIDs and is
  being exercised; no reformat is used to recover an existing volume.
- Guest converge 3 mounted the files and started the runner user manager, then
  exposed a daemon readiness race: its socket ACL step ran before the socket
  existed. Matched the installed Docker rootless template's `Type=notify` and
  `NotifyAccess=all`; converge 4 is testing that ordering. Two malformed units
  from converge 2 were removed only after matching their W1 ownership/content.
- Filesystem review found mkfs's default discard undid backing-file preallocation.
  New formats use `-E nodiscard`; existing owned sparse files are reallocated
  without formatting or changing their bytes/UUIDs. Admission now checks
  allocated blocks and underlying filesystem headroom as well as logical size.
- Image build 1 rejected an incomplete hash lock: cross-platform pip resolution
  on Python 3.13 omitted `typing_extensions`, required under Python 3.12. Added
  that conditional dependency and pinned exact wheel URLs. Build 2 passed. Image config digest: `sha256:0c4be2a296e46e09447327ff931953b0347be1f9bf54d4c69f63809787f68792`; builder image/index digest: `sha256:7c96201e639210264d14fb526d597209c6c2ca5e7ea5876fd107c50aab71341e`.

Private raw logs live in the ignored operator directory under `w1-*`. Publish
only reviewed observations, artifact hashes and synthetic results here. No
participant roster, upstream model key, live inference, public terminal,
physical-host reboot or cloud operation is part of these attempts.

## Remaining verification and handoff

W1.01 and W1.05 are complete on MAC/LINUX; other tasks remain pending their named evidence. Finish image reader/manual/fake tests,
rootless container enforcement, cross-identity socket denial, missing/wrong
mount faults, persistence/reset, repeat converge and login-independent startup
in LINUX. Prepare the exact reviewed physical-host bootstrap only after those
pass; the owner-run privileged step recorded in I1 precedes UBUNTU proof. Then
run the actual browser terminal from MAC, record UBUNTU enforcement/persistence,
and update W1.01–W1.07/W1.G individually from measured results.

### Intermediate LINUX results

Converge 4 passed all runtime assertions with 22 successful tasks: rootless
Docker reports systemd cgroup v2, built-in seccomp and the dedicated data root.
The runner user manager started through lingering without an interactive login.
This does not yet prove the per-container limit tests or mount-fault cases.

The image's installed SDK suite passed all four tests, including native Polars,
known Rasterio pixels, tokenless paginated loopback STAC and subprocess error
compatibility. The first invocation failed because its test environment omitted
a writable `/workspace` cache and put synthetic fake executables on noexec
`/tmp`. The successful rerun used bounded writable cache plus a separately
labelled executable test tmpfs. Deployed terminal mounts remain noexec; this
installed-package check is not rootless isolation evidence. Four Rasterio
pending-deprecation warnings were retained.

### W1.01 / W1.05 completed on MAC and LINUX

The exact image imported into the dedicated rootless daemon as
`sha256:0c4be2a296e46e09447327ff931953b0347be1f9bf54d4c69f63809787f68792`.
The running container's image smoke passed the installed native LazyFrame query,
known Rasterio window/CRS/nodata and final-path provider symlink with spaces.
[Artifact pins](w1-artifacts.json) retain image, FEAM, ttyd, wheel-lock and native
Go proxy digests.

[Chromium browser proof](w1-guest-browser.json) ran on MAC against the actual
QEMU-hosted rootless terminal through two private SSH forwards. It received the
live TUI over WebSocket, entered a manual pinned resolve command and observed
the registered inventory. Anonymous HTTP and URL-command query attempts were
denied. Go tests separately prove cross-origin rejection, filtering, the two-
stream cap and released-slot reuse. This is MAC-to-LINUX browser evidence;
physical UBUNTU acceptance remains pending.

### W1.03 completed on LINUX

The [rootless enforcement probe](w1-guest-enforcement.json) passed inside the
same running image: UID 1000, read-only root, zero effective capabilities,
seccomp mode 2, no-new-privileges, loopback-only network namespace and denied
external connect. Measured 512-MiB memory enforcement killed the oversized child
once; the 128-PID ceiling denied forks; the 0.5-CPU quota produced 104 throttled
periods during bounded load. `memory.swap.max` is zero. The terminal socket
filesystem rejected both excess bytes and excess inodes. No per-container
AppArmor or native-performance claim is made from this TCG guest.

### Walkthrough, socket and persistence/reset proof on LINUX

[Manual](w1-guest-manual.json) passed eight checks in 78.27 seconds;
[fake](w1-guest-fake.json) passed nine in 96.70 seconds. Both used the existing
walkthrough with the documented installed-binary symlink adaptation on noexec
storage. They exercise local review denial, exact staging receipts, changed-
destination rejection, draft validation/publication, namespace denial and
withdrawal. These are synthetic functional results, not live-model evaluation.

Gateway terminal socket access succeeds; broker/collector access fails. Controller
runtime socket access succeeds; gateway runtime access fails. Another container
UID (1001, with dropped capabilities/read-only root/no network) cannot connect
even when presented the other identity's socket directory read-only.

Stop/start preserved the probe file. Reset from generation 1 to 2 used only the
empty fixed spare and retained the prior slot's bytes and UUID. A second reset
was rejected with `spare occupied` while generation 2 stayed running; this
expected error is retained in the raw test log. The
[post-reset browser](w1-guest-browser-after-reset.json) passed the same real
WebSocket/manual-resolution and admission checks against the new workspace.

### W1.02 and LINUX portion of W1.07

The final full repeat converge passed `ok=33 changed=0 failed=0`; no file was
formatted or initialized again, and service/account/storage settings converged
without changes. The native Docker/rootless-extra version and systemd/cgroup
observations above establish the tested guest runtime. W1.02 is complete.
The UBUNTU repeat-converge portion of W1.07 and physical-host startup proof are
still pending. The disposable mount-fault test is the final LINUX storage gate.

### W1.04 completed on LINUX

The [disposable mount-fault probe](w1-guest-mount-faults.json) passed: stopping a
required mount stopped the runner; a missing backing file prevented manager
startup; a wrong tmpfs at the expected path failed verification; the runner
could not write into the underlying empty mountpoint. Cleanup restored the
recorded UUIDs/filesystem inventory and started the runner without login.
The restricted user-namespace policy remained unchanged. No physical-host
mount fault or reboot was performed.

W1.01–W1.05 now pass. The remaining tasks are the owner-run physical-host
bootstrap and UBUNTU proof (W1.06), UBUNTU repeat converge/startup (W1.07), and
W1.G. The physical prerequisite simulation adds only `uidmap` and `libsubid4`
version `1:4.13+dfsg1-4ubuntu3.2`: zero upgrades/removals. The existing pinned
Ansible 2.21.4 environment is available. The owner bootstrap disables automatic
package-triggered service restarts and does not edit the system Docker daemon.

## Owner-run UBUNTU handoff (pending execution)

The tested bundle is staged privately in the existing native-build handoff
scope on Ubuntu. Bundle SHA-256:
`e534ca04763493cfe34039dd219f2b496a29f510694ca888a0e131d6fb35f3d9`
(248,401,920 bytes). Script SHA-256:
`55cdf398df5fbd41f299447793b08f651acc806ee726530031a21911c64d604f`.
Local and remote bundle hashes match. The
[reviewable bootstrap](../../../infra/demo/scripts/owner_bootstrap_w1.sh)
verifies/extracts a root-owned copy, installs only the two simulated missing
prerequisites, applies the tested roles, imports the pinned image, initializes
only an empty slice, installs/starts the private terminal and repeats converge.
It returns the browser secret through an owner-only file, never stdout.

The owner was asked to run:

```bash
sudo bash ~/feam-w0-vm/native-build/w1-handoff/owner_bootstrap_w1.sh --reviewed-bundle "$HOME/feam-w0-vm/native-build/w1-handoff/feam-w1-owner-bundle.tar" e534ca04763493cfe34039dd219f2b496a29f510694ca888a0e131d6fb35f3d9
```

Reason: workplan I1 assigns privileged bootstrap to the owner, and the host
rejects noninteractive sudo. No sudo password is requested or stored. This is
pending owner execution, not completed UBUNTU acceptance. After completion,
verify actual-host browser/manual/fake, resource/socket limits, persistence/reset,
repeat converge and preservation of existing Docker/namespace policy. Actual
shared-host reboot remains deferred to a later approved window.

### First owner-run UBUNTU bootstrap attempt

The owner ran the initial staged bundle and reported failure at
`runtime : Start the mount-bound user manager without a login`:
`user@2101.service` exited during startup. Prior account/storage changes remain
owned and must be preserved; this attempt does not pass W1.06. Inspect the
read-only service status/journal and correct the cause before another bounded
converge. A revised handoff also includes the already guest-tested host evidence
collector so one owner invocation can perform the required privileged checks.

The read-only journal identified `backing file reservation is sparse` in the
root storage pre-start check. All three existing files and mounts remained
present. The default lazy ext4 inode-table initialization can zero blocks
through the loop device by punching holes even when mkfs used `nodiscard`.
A fresh, disposable 256-MiB regression reproduced allocation falling to
251,666,432 bytes after mounting. Reallocation preserved the file inode,
filesystem UUID and a written sentinel. Eager inode/journal initialization
kept the new file fully reserved (268,439,552 allocated bytes).

The corrected helper requires completed inode initialization, uses eager
initialization for new files and restores only holes in identity-verified
recorded files. It retains the sparse-file admission check. No existing file
is reformatted. The [regression result](w1-guest-storage-reservation.json) is
disposable LINUX evidence; physical-host repair still requires the owner-run
corrected bootstrap. Offline checks pass (18 contract and 10 storage/state
tests plus Ansible syntax/lint); an initial check invocation lacked the venv
on PATH, and the corrected invocation passed.

### Corrected owner bootstrap (pending execution)

Versioned v2 files retain the first attempt. Bundle SHA-256:
`d39d1c73c86d935af6efab75d5473cee9db6622798aba34e833a32d1610c222e`
(248,412,160 bytes); entrypoint SHA-256:
`221067ba2efd2fefe6f58246b5752db667b8bd8f317cca9d6dd95e7ab30f4a78`.
Both remote hashes match the local artifacts. The owner was asked to run:

```bash
sudo bash ~/feam-w0-vm/native-build/w1-handoff/owner_bootstrap_w1_v2.sh --reviewed-bundle "$HOME/feam-w0-vm/native-build/w1-handoff/feam-w1-owner-bundle-v2.tar" d39d1c73c86d935af6efab75d5473cee9db6622798aba34e833a32d1610c222e
```

This bundle also installs the bounded acceptance probes, records each successful
phase durably, retains failed logs, and stops on failure. It proves readers,
resource enforcement, manual/fake walkthroughs, socket separation, persistence
and one confined reset, then repeats converge and compares the unrelated Docker
daemon and namespace/AppArmor policy. A private owner export makes the results
and browser credential available without granting additional sudo access.
An interrupted reset requires reconciliation; it is never blindly repeated.

The corrected roles also converged on the retained disposable guest:
`ok=34 changed=1 failed=0`. The one changed task installed the explicit
`nodiscard` mount definitions; the existing owned storage required no further
allocation or formatting, and the runner remained active. The storage helper
had already been installed for the reservation regression. This is separate
from the still-pending corrected owner invocation on the physical host.

### Second owner-run UBUNTU bootstrap attempt

The owner ran v2. The allocation repair succeeded: all three physical-host
volumes are fully reserved, the dedicated user manager/rootless daemon is
active, and the pinned image was loaded. Deployment stopped before terminal
state initialization because the added `dumpe2fs` scan returned status 156
while reading the populated, mounted runtime filesystem. This is another
failed acceptance attempt, retained separately from the successful repair.

The next revision records completed inode initialization once during provision,
after flushing legacy mounted filesystems, and requires that root-owned marker
at admission. It no longer reads live allocation bitmaps on every start. New
files still complete initialization before mounting; sparse/UUID/identity
checks remain. A failed initial scan cannot create the marker. The populated
guest migrated successfully, passed verification and then repeated provision
with `changed=false`, preserving its existing file identities and UUIDs.
Offline checks pass with 18 contract and 11 storage/state tests.

### Third owner bootstrap (pending execution)

The populated guest deployment passed `ok=14 changed=1 failed=0`; the only
changed task installed the bounded acceptance scripts. Existing terminal state,
image and running generation were preserved. V3 bundle SHA-256:
`aed0244a13aa78e2e6994e1c8adb017b8e08fc6626ff57f5445b6fa0788e0a78`
(248,412,160 bytes). The entrypoint remains
`221067ba2efd2fefe6f58246b5752db667b8bd8f317cca9d6dd95e7ab30f4a78`.
Both hashes match on Ubuntu. The owner was asked to run:

```bash
sudo bash ~/feam-w0-vm/native-build/w1-handoff/owner_bootstrap_w1_v3.sh --reviewed-bundle "$HOME/feam-w0-vm/native-build/w1-handoff/feam-w1-owner-bundle-v3.tar" aed0244a13aa78e2e6994e1c8adb017b8e08fc6626ff57f5445b6fa0788e0a78
```

The additional browser fake-agent probe initially timed out matching raw
WebSocket bytes. Its screenshot showed the correct assistant clarification;
the test must reconstruct terminal cursor updates before matching screen text.
Those failed local probe attempts remain retained. The original manual browser
and full in-container manual/fake walkthrough evidence remains separate.

The corrected [browser manual/fake probe](w1-guest-browser-fake.json) passes:
manual resolution, deterministic assistant clarification and pinned resolution,
query-argument rejection and anonymous denial all ran through Chromium and the
guest's private WebSocket terminal. Terminal screen parsing uses pinned
`pyte==0.8.2` and `wcwidth==0.9.1`; the initial missing-parser invocation is
retained separately from the passing run.

An independent read-only comparison after the v3 host proxy became active found
the unrelated Docker PID, active state and start timestamps, restricted
user-namespace setting and packaged RootlessKit AppArmor policy hash unchanged
from the original pre-bootstrap snapshot, spanning all three owner attempts.

### Third owner-run UBUNTU result and lingering reconciliation

The owner reached the final `loginctl show-user` audit after all acceptance
phases and the final `ok=33 changed=0 failed=0` converge. That audit failed with
"User ID 2101 is not logged in or lingering." Read-only inspection confirmed
the runner and proxy were active and the linger file existed, while logind's
user list lacked UID 2101. The original failed startup had left an on-disk
linger file without current logind registration. The role's `creates` guard
incorrectly treated that file alone as convergence.

The corrected role checks logind's live `Linger` property, invokes
`loginctl enable-linger` when registration is missing and verifies `Linger=yes`
after manager start. The final bootstrap also requires an empty `Sessions`
property. Completed acceptance markers remain root-owned and will be reused;
the successful confined reset must not be performed again. The private export
is pending this final reconciliation, so the actual-host browser gate remains
open.

The guest already exhibited the same missing registration after its mount-fault
test. The initial recovery fixture stopped before mutation when it tried to
terminate that absent registration; the corrected fixture accepts this initial
state. [Recovery passed](w1-guest-linger-recovery.json): the retained linger
file was insufficient, `enable-linger` restored live registration and readiness
without login, and terminal generation and storage inventory were unchanged.

V4 is staged with matching local/remote hashes. Bundle SHA-256:
`7579ef01f2ae5a1134e5404ea3c2e39a9546f4589bb23c78dee9e6c17fa57fcb`
(248,422,400 bytes); entrypoint SHA-256:
`ef482e10baa89eb3a9c2565ecc8f419c16174fa480cd0e53a1f15bc197c1b274`.
The owner was asked to resume with:

```bash
sudo bash ~/feam-w0-vm/native-build/w1-handoff/owner_bootstrap_w1_v4.sh --reviewed-bundle "$HOME/feam-w0-vm/native-build/w1-handoff/feam-w1-owner-bundle-v4.tar" 7579ef01f2ae5a1134e5404ea3c2e39a9546f4589bb23c78dee9e6c17fa57fcb
```

The script reuses completed phase markers, including reset, and checks both
`Linger=yes` and `Sessions=` before exporting the private reports/credential.
The current offline checks, Ansible syntax and lint pass.

## W1 closeout on actual Ubuntu

The owner confirmed the v4 bootstrap completed. Retrieved private exports match
the pinned image and all six completed phase markers. The constrained container
passed eight manual checks (5.95 seconds) and nine fake checks (8.92 seconds).
Actual enforcement recorded one bounded OOM kill, two PID denials and 81 CPU
throttled periods; non-root UID 1000, read-only root, no capabilities, seccomp,
no-new-privileges, no network and zero additional swap were verified. Socket
tmpfs byte/inode limits and all planned cross-identity denials passed.

Stop/start preserved the sentinel. Reset selected fixed slot-02/generation 2,
retained the old slot's bytes and UUID, and refused a second reset before
stopping the current container. Readers and final-path symlinks passed before
and after reset. Final physical-host converge: `ok=34 changed=0 failed=0`.
The repaired guest runtime role also repeated with `ok=13 changed=0 failed=0`.
Live Ubuntu logind reports `Linger=yes` and `Sessions=`. No host reboot occurred.

Chromium on MAC reached the **actual Ubuntu host** via the private SSH/Unix
forward, resolved the pinned inventory manually, completed deterministic fake
clarification and resolution, and verified anonymous/query-argument rejection.
The existing physical-host Docker PID/start times, namespace restriction and
packaged RootlessKit policy are unchanged across the complete provisioning run.

The private terminal remains available on Ubuntu. Owner-only credential and
raw evidence exports are under the private `w1-handoff` directory; a local copy
is ignored under `.local/demo-deployment/`. No provider credential, live model
call, public listener, DNS/Access/Tunnel deployment or cloud resource was used.
Exact recipes and operator commands are in [the W1 runbook](../../../infra/demo/W1.md).

After acceptance, the disposable VM was gracefully powered off and verified
stopped; its disk and evidence remain retained. Its local SSH forwards were
closed. Final actual-host checks still report both W1 services active,
`Linger=yes`, no login sessions and the original unrelated Docker PID/start
timestamp. The actual-host browser forward remains loopback-only on port 18771.
W1 changes remain uncommitted. The final reflog shows the shared checkout was
switched from the W1 branch to `docs/zero-trust-onboarding-complete` at 01:30 EDT
and to `chore/add-demo-users` at 02:14 EDT during this work. The current branch
and concurrent edits were preserved; no W1 commit/push/merge was performed.
The pre-existing `.DS_Store` edit remains untouched.
