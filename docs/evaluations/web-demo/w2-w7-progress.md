# Participant-service implementation record

Started 2026-09-25 on `feat/complete-ubuntu-web-demo`. Work is stopped at the
owner-requested review checkpoint. See the [wrap-up handoff](wrap-up-checkpoint.md)
for final checks and resume order. This record does not establish any W2–W10 exit gate.

## Scope and measured checks

- MAC: implementation is split across identity/gateway, dataset pipeline,
  broker/accounting and lifecycle work. Separate process identities and
  SQLite owners follow the implementation contract.
- MAC: initial combined `go test -race ./...` passed for the source present
  during that run. Some new packages were still awaiting their own tests;
  this preliminary build is not the final validation record.
- MAC: the existing offline checker and 13 W1 storage/state tests pass after
  adding finite 12-slot expansion. The complete pool playbook subsequently passed syntax/lint and actual
  Ubuntu first/repeat converge; clean-VM recreation remains pending.
- MAC → UBUNTU: dedicated deployment-key SSH and `sudo -n id -u` were freshly
  verified (expected account and UID 0, exit 0). This access check alone proves no participant-service acceptance;
  subsequent storage provisioning is recorded below.
- EDGE read-only: existing Codex connection returns HTTP 200 for the selected
  active zone, Access apps/groups, Tunnel list and Zero Trust organization.
  The zone belongs to the connected account. Apps, groups and tunnels are
  empty. The organization exposes its auth domain. No edge resources changed;
  this initial read did not verify write permissions, Terraform credentials, MFA
  or actual login. Later R2 mutations and the successful MFA recheck are below;
  public Access/Tunnel/DNS configuration remains pending.

## Owner inputs captured in this session

Research-event retention is 30 days. Saif is the sole export reviewer and
Phase 2 owner. The supplied support address and policy are stored in the
ignored owner-only `.local/demo-deployment/research-policy.json`. Existing
consent is retained; no new consent gate was added. Independent archival and
deletion/retrieval evidence are still required before real participant capture.

## Current gaps

- W2 gateway negative tests pass locally; private Ubuntu integration remains pending.
- W3 has durable finite lifecycle jobs, confirmation expiry, conservative
  unknown outcomes and a fixed Docker template. Grant-aware mounts, capability preparation/revocation, readiness checks and
  host headroom admission are implemented. Hash-bound operator recovery,
  advance retention warnings and guarded slot reclamation pass local tests;
  actual process/socket/UI/isolation and root cleanup proof remain pending.
- W4 bounded source acquisition was approved and completed. Real small
  Parquet/raster candidates and interval-aware STAC/SDK checks pass locally.
  Exact Ubuntu candidate approval/promotion and mounted access remain pending.
- W5 uses fake upstreams only; live inference requires independent durable
  accounting/archive proof and an approved finite budget.
- W6–W7 integration and W8–W10 environment gates remain pending. No paid
  compute resource, public terminal, invitation or billable inference was created.

Next action after owner review: resolve the recorded pipeline dependency failure,
freeze the current source into a new native artifact, and complete private Ubuntu
integration before public-edge setup. No further deployment runs at this checkpoint.

## Actual Ubuntu storage expansion

The reviewed `pool.yml` expands the existing ownership inventory to twelve
2 GiB slots, a 20 GiB runtime, 200 GiB datasets, 5 GiB operations and 20 GiB
traces: 269 GiB in sixteen filesystems. First converge passed with five changed
tasks; repeat converge passed with zero changes. A separate read-only check
verified every mounted UUID, backing inode/device and complete physical byte
reservation. See [sanitized storage evidence](w3-ubuntu-storage.json).
Existing recorded W1 filesystems were neither reformatted nor cleared. The
private command logs are retained under `.local/demo-deployment/`; there was
no reboot, public terminal, paid cloud allocation or live model request.
This establishes storage provisioning, not aggregate resource or ten-user
capacity acceptance. The full clean-VM recreation gate is still open.

## Native build authorization

Automatic approval review initially blocked transfer of the 4.35 MiB source
snapshot because payload/destination authorization was not explicit enough.
The owner then explicitly approved snapshot
`a01aec8aaf89b2be9cbf46ebbd5f22676553be0264d93315db26893acf4b9769`
to the existing owned Ubuntu native-build directory with two Cargo jobs.
The approved retry passed: native Linux amd64 hosted FEAM release build
with Cargo locked dependencies and two jobs. The source hash above identifies
its inputs; the binary digest is retained in [native build evidence](w7-native-feam.json).
Private inventories, credentials, participant records and research traces are
excluded from that archive.

## Operator state

The roster, research settings and existing approvals were sealed in an
authenticated AES-256-GCM operator vault on the Mac. Its new owner-only key
is outside the repository under the owner's private configuration directory.
The vault now verifies revision 3, SHA-256
`4e9000fe24a900dc9848455a4703abf48df0478a466393e0f2a6deb3e5269887`;
earlier encrypted revisions are preserved.
A temporary plaintext conversion was removed after verification; the original
owner-supplied ignored inputs are retained. Three automated vault tests cover
wrong keys/tampering, stale revision rejection, retained encrypted revisions,
unsafe permissions/symlinks and refusal to overwrite an existing export.
The owner explicitly confirmed zero prior project charges. A real encrypted
US$100 project ledger was initialized once on the Mac with zero prior charges,
no run allocations and no live spending. Its receipt and keys are owner-only;
the vault retains its receipt, the confirmed prior charge amount, MFA verification
and the owner's DigitalOcean deferral. No budget keys were copied to Ubuntu.

The owner enabled R2. The connected API then created the approved private
`feam-research-web-demo` Standard bucket in ENAM, disabled public r2.dev access,
and verified no custom domains and a 30-day research lifecycle policy. A
167-byte synthetic connectivity object was uploaded and listed, then deleted;
the bucket was verified empty. The connector cannot return raw object bodies
and cannot create service credentials, so no checksum retrieval or collector
durability acceptance is claimed. The owner subsequently supplied the R2 S3
credential file. Its ownership, 0600 mode, 0700 parent and nonempty required fields
were verified locally without printing secrets. It has not been copied to Ubuntu,
and token scope/live S3 access remain untested.
The account API initially reported MFA disabled; after the owner was asked,
a subsequent API read confirms it enabled. No real participant data was uploaded.

The owner-only DigitalOcean token authenticates successfully to an active,
email-verified account, but its size list omits the approved 8-vCPU/16-GiB size.
The owner shelved DigitalOcean. No billable compute resource has been created,
and cloud allocation remains deferred until the owner resumes it.

## Later local checks and native staging

The final combined Go race run and full vet pass, including collector archive
metadata carryover and controller status/recovery changes. The offline checker
passes all 95 Python tests, shell checks, schema examples, eight playbook syntax
checks and Ansible lint across 17 files. Edge checks are now included in the same
CI entrypoint. Earlier context checks and all thirteen context regression tests
passed; the wrap-up handoff records the final documentation check separately.

The native broker compatibility probe built from the already approved Rust
snapshot with two jobs (42.82 seconds). Both public climate objects and their
pinned job specification were copied to a new owned private Ubuntu intake;
all three SHA-256 hashes matched. Total transfer was 38,490,112 bytes. This is
source staging, not candidate preparation or publication.

A separate 993,280-byte service-source archive was prepared, SHA-256
`ed1f2decdb9931effb97e391cf8b4d317c5e8a0248296cc9aaf6684917b0cf55`.
Automatic approval review rejected its transfer to the existing owned Ubuntu
`native-build/web-w1` directory because the previous explicit approval covered
another snapshot. The owner then approved this exact transfer and bounded build.
Transfer, full native Go race tests, vet and all ten command builds succeeded
with two build jobs; see [native Go evidence](w7-native-web.json). Later collector
carryover changes have not been transferred or built natively and require a new
artifact identity before deployment.

## Graceful stop and retained failure

The participant image context is prepared from the approved native inputs, but
its build did not start. Two temporary, never-started extraction containers were
removed. No owned build process remains; existing W1 images/containers were
preserved. [Image checkpoint](w7-participant-image.json) records input hashes.

The actual Ubuntu pipeline-tool installation failed because Python 3.12 lacks
`ensurepip`. It left an incomplete, root-owned version directory at
`/opt/feam/pipeline-releases/0c7703afe7fe3a6fcdb361c97eeae261d3c1142090adfef760c66ea92c7b59df/`.
No `installed.json` or current installation links were activated, and no host
packages were changed. The installer deliberately refuses automatic reuse of
that incomplete directory. Preserve it until explicit scoped reconciliation.
The existing W0 pinned-pip bootstrap is a possible implementation reference;
no replacement installer or retry was performed during wrap-up. An earlier
malformed SSH option failed before connection; both attempts are retained in
private logs. Ubuntu climate candidate preparation/publication remains pending.
