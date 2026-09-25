# Web demo acceptance index

Updated 2026-09-25. **The current Ubuntu participant demo is deployed; final
fresh admin-browser acceptance is unavailable.** U.01–U.06 have bounded evidence.
U.07's operator delivery is complete, but its fresh admin walkthrough and U.G
remain unchecked because the owner cannot complete email-PIN sign-in. See the
[operator handoff](evaluations/web-demo/ubuntu-operator-handoff-20260925.md).

Four admins and seven users are active after exact real Access reconciliation;
all seven have the appropriate role/workspace configuration and the users have
the approved climate release. Two real participant browser sessions passed data
reads and hosted interactions. Eleven live requests settled at US$0.003119 with
no outstanding reservation or unknown live cost. One model handle error was
rejected without writing and remains in the record; a fresh local reviewed copy
and separate explicit-handle denial succeeded. Guided history/no-tool mode and
one observed Help action passed. This is not five-of-five model success, a full
tutorial walkthrough or a quality benchmark.

Ordinary maintenance, repeat converge, spend retention and an independently
verified private Mac archive copy have evidence. The initial three-person PIN,
admin grant/reset/revocation and fake-run evidence remain separate. The fake
unknown cancellation remains unknown. Ten-user capacity, reboot, full recovery,
new held-out benchmarking and cloud/R2 acceptance remain deferred or shelved.

[Workplan](ubuntu_web_dev_demo_workplan.md) owns task status;
[design](ubuntu_web_dev_demo_design.md) owns architectural requirements.
Earlier rows retain their original environment and scope; the final rows provide updated-release evidence.
The [Stage-1 acceptance](tui_agent_stage1_acceptance.md) remains unchanged and
does not establish browser, broker, Ubuntu, cloud or target-HPC acceptance.

| Environment | Evidence | What it establishes |
| --- | --- | --- |
| MAC | [W0 record](evaluations/web-demo/w0-baseline.md) | Checkout baseline, contracts, configuration validation and toolchain checks. |
| LINUX | [W0 VM proof and native artifact](evaluations/web-demo/w0-linux-artifacts.json) | Owner-authorized Ubuntu 24.04 x86-64 QEMU full VM passed systemd/cgroup/loop/remount checks. TCG is functional evidence only; VM is stopped and retained. Remote CI jobs are defined but not run. |
| UBUNTU | [W0 preflight/build](evaluations/web-demo/w0-baseline.md#actual-ubuntu-preflight), [owner-run privileged evidence](evaluations/web-demo/w0-privileged-preflight.json) | Read-only preflight and Ansible entrypoint (`changed=0`); native hosted FEAM release and 18 offline contract tests passed in isolated scope. Reviewed privileged inventory closes W0.06/W0.G with a dedicated provisioning scope. No demo runtime or capacity claim. |
| MAC / LINUX | [W1 execution record](evaluations/web-demo/w1-progress.md), [artifact pins](evaluations/web-demo/w1-artifacts.json) | Go race/vet/native build and offline checks pass. Disposable QEMU proof covers resource/socket limits, mount faults, eager allocation and repair, persistence/reset, lingering recovery and real browser manual/fake exchanges. Functional evidence only. |
| UBUNTU | [W1 acceptance](evaluations/web-demo/w1-ubuntu-acceptance.json), [browser](evaluations/web-demo/w1-ubuntu-browser.json), [enforcement](evaluations/web-demo/w1-ubuntu-enforcement.json) | Owner-run scoped provisioning and full manual/fake walkthroughs pass. Actual Chromium terminal, resource limits, identity/socket denial, private persistence/reset and `changed=0` converge pass. Lingering requires no login; existing Docker and host policies are preserved. Public edge, live inference, capacity/cloud and shared-host reboot are later gates. |
| MAC → UBUNTU | [Deployment access verification](ubuntu_web_dev_demo_workplan.md#ubuntu-deployment-access-verified-2026-09-25) | After owner setup, dedicated-key SSH returns `feam-deploy` and `sudo -n id -u` returns `0`, exit 0 without prompting. Authorized Ubuntu work can proceed without manual sudo handoffs. This proves administrative access only; no new Ansible converge, application, reboot or W2–W10 acceptance is claimed. |
| EDGE | [Domain activation and hostname decision](ubuntu_web_dev_demo_workplan.md#domain-activation-and-hostname-switch-2026-09-25), [Zero Trust onboarding](ubuntu_web_dev_demo_workplan.md#zero-trust-onboarding-confirmation-2026-09-25) | Owner confirms `613202690.xyz` is Active in Cloudflare and Zero Trust onboarding is complete; public DNS through two resolvers returns the assigned nameservers. DNS routes, HTTPS, Tunnel, Access and application readiness remain unverified; W8 is not complete. |
| MAC / Codex connection (historical prerequisite) | [Cloudflare connection record](ubuntu_web_dev_demo_workplan.md#cloudflare-connected-to-codex-2026-09-25) | Owner reports connection setup complete; the agent confirms Cloudflare tools are available in this session. At that prerequisite checkpoint no API request or mutation had been made; later account/R2 checks are below. Terraform/runtime authentication and W8 EDGE acceptance remain unverified. |
| CLOUD | Not run | No deployment, public exposure, paid resources or invitations. |

Later work has these additional bounded results:

| Environment | Evidence | What it establishes |
| --- | --- | --- |
| MAC | [W2–W7 progress](evaluations/web-demo/w2-w7-progress.md), [W4](evaluations/web-demo/w4-local-evidence.md), [W5](evaluations/web-demo/w5-local-evidence.md), [W6](evaluations/web-demo/w6-local-evidence.md), [W7 services](evaluations/web-demo/w7-services-local-evidence.md) | Final Go race suite and vet pass; 95 offline Python tests, eight Ansible syntax checks, lint across 17 files and 12 Terraform mock tests pass. Context checks pass. Rust/PTY capture, real local climate readers, fake broker/accounting and operator recovery are separately recorded. These are not deployed service or live-model acceptance. |
| UBUNTU | [Finite storage pool](evaluations/web-demo/w3-ubuntu-storage.json), [native FEAM build](evaluations/web-demo/w7-native-feam.json), [native Go build](evaluations/web-demo/w7-native-web.json), [image context](evaluations/web-demo/w7-participant-image.json) | All sixteen filesystems fully reserved/mounted, repeat converge unchanged. Approved Rust and ten Go command binaries built natively; Go race/vet passed for that snapshot. Current collector carryover is newer. Image build and participant services remain pending under U; clean full-VM integration is deferred. Pipeline tools failed on missing ensurepip; no current installation was activated. |
| UBUNTU capacity model | [Cloud volume format model](evaluations/web-demo/cloud-volume-format-model.json) | Temporary sparse 320-GiB filesystem geometry supports the bounded pool plan; the owned temporary file was removed. It was not mounted and does not establish actual cloud capacity or statvfs admission. |
| EDGE archive | [Archive setup](evaluations/web-demo/w6-r2-setup.json) | Approved private Standard ENAM bucket, 30-day research retention, no public domains; synthetic PUT/list/delete and empty-bucket check. Account MFA verified; S3 credentials received and checked locally. Ubuntu credential transfer, live collector readback and archive-survival checks were unexecuted and are now shelved. U.03 plans the local archive/Mac-copy path. |
| CLOUD account read-only | [Progress](evaluations/web-demo/w2-w7-progress.md) | Supplied DigitalOcean credential authenticates; no existing droplets/volumes, approved size absent from account listing. No compute resources created; DigitalOcean shelved by owner. |

Current functional takeover evidence:

| Environment | Evidence | What it establishes |
| --- | --- | --- |
| UBUNTU | [U.01 prerequisites/image](evaluations/web-demo/u01-ubuntu-prerequisites.json) | Repaired Python venv, pinned imports, participant image, installed SDK/native readers and synthetic hosted startup. Current reused Rust/adapter build inputs match. W1 preserved. |
| UBUNTU | [U.02 promoted release](evaluations/web-demo/u02-ubuntu-release.json), [browser grant/readback](evaluations/web-demo/u02-browser-grant.json) | Exact owner-approved immutable release promoted once; source/output hashes, real Polars/Rasterio and loopback STAC pass. A real admin browser grant gave A only the read-only release; installed SDK table and Rasterio window checks passed from its workspace. B had no mount and peer resolution was denied. U.02 is complete. |
| MAC / UBUNTU | [U.03 local archive](evaluations/web-demo/u03-collector-local-archive.md), [deployed capture/stop/copy](evaluations/web-demo/u03-fake-capture-archive.json), [W1 recovery](evaluations/web-demo/u03-w1-runtime-recovery.json), [copy lifecycle](../infra/demo/SITE_LIFECYCLE.md) | U.03 passes: two zero-cost fake requests, one intentionally canceled unknown-cost request, correlated synthetic broker events, ordinary stop and eight verified Ubuntu archive objects. Four non-reviewer categories cannot read the archive/events database or reach research export. A separate owner-only Mac copy verified all eight objects and provenance ledger; local withdrawal/expiry tests pass. The Ansible runtime reconverge interrupted a W1 container, then the same generation/container and private terminal were restored without reset. No destructive teardown or live-provider acceptance is claimed. |
| UBUNTU private | [U.04 final build](evaluations/web-demo/u04-native-web-final.json), [six services](evaluations/web-demo/u04-private-services.json), [two workspaces](evaluations/web-demo/u04-private-workspaces.json), [browser form repair](evaluations/web-demo/u05-browser-form-repair.json), [reviewed reset](evaluations/web-demo/u04-browser-reset-stage.json) | Six private services, A/B terminals, exact isolation/cross-owner denials, one confirmed B reset and A's reviewed TUI staging passed. Native Chromium form POST initially failed 403 with `Origin: null`; scoped gateway repair passed real browser grant and installed SDK reading for A, while B remained ungranted. U.03 now covers the fake capture/ordinary stop/copy. The updated-release and live maintenance evidence is linked below. |
| MAC / EDGE + UBUNTU | [Public activation](evaluations/web-demo/u05-public-activation-proposal.md), [browser acceptance](evaluations/web-demo/u05-browser-acceptance.json), [activation checks](evaluations/web-demo/u05-public-activation.json), [runtime repairs](evaluations/web-demo/u05-runtime-repairs.json) | U.05 functional edge acceptance passes: four exact proxied routes, HTTPS redirect, protected Ubuntu ingress, three real PIN dashboards, 25/25 authenticated/negative HTTP and WebSocket probes, and real browser account-disable closing the current WebSocket in 0.506 seconds. Old JWTs returned 403. Connector boot enablement remains off, no direct 80/443 listener exists. At that checkpoint the second alias was disabled; the owner-authorized full-roster restoration and actual reconciliation are recorded below. Initial redirect-field, wrong-IdP, form-Origin and HTTP/2 Unix-WebSocket failures and scoped repairs remain recorded. Explicit expired-assertion probing is deferred. |
| MAC operator / UBUNTU | [Updated release](evaluations/web-demo/u07-updated-release.json) | Exact native builds/image, tested audited grants, eleven active accounts, seven grants, real two-user browser/data and role isolation. Normal SSO renewal resolved stale application tokens; fresh admin PIN unavailable. |
| UBUNTU / OpenRouter | [Live functional smoke](evaluations/web-demo/u06-live-functional-smoke.json) | Owner's later US$50 shared allocation supersedes the original US$1 proposal. Eleven requests settled at US$0.003119, actual pinned route and correlated capture. Model handle failure, local recovery, denial and limited guide proof remain explicit. |
| MAC / UBUNTU | [Live maintenance/archive](evaluations/web-demo/u07-live-maintenance.json), [operator handoff](evaluations/web-demo/ubuntu-operator-handoff-20260925.md) | Ordinary restart/convergence preserves the same allocation, accounting, files and grants. Retained archives/provenance have independently verified private Mac copies. Fresh admin acceptance and U.G remain unchecked. |

Publish only sanitized observations and hashes. Raw preflight output, host
addresses, UUIDs, inventory, roster and credentials belong in the ignored
operator directory or encrypted operator storage. Logs from synthetic local
checks may be retained under this evidence directory after review.
