# Web demo acceptance index

Updated: 2026-09-25. W0 and [W1](evaluations/web-demo/w1-progress.md) are complete, including [actual-Ubuntu acceptance](evaluations/web-demo/w1-ubuntu-acceptance.json). W2–W10 remain pending.

[Workplan](ubuntu_web_dev_demo_workplan.md) owns task status;
[design](ubuntu_web_dev_demo_design.md) owns architectural requirements.
Evidence below is newly measured unless explicitly labelled historical.
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
| MAC / Codex connection | [Cloudflare connection record](ubuntu_web_dev_demo_workplan.md#cloudflare-connected-to-codex-2026-09-25) | Owner reports connection setup complete; the agent confirms Cloudflare tools are available in this session. No account/zone API request or mutation was made. Permissions, Terraform/runtime authentication and W8 EDGE acceptance remain unverified. |
| CLOUD | Not run | No deployment, public exposure, paid resources or invitations. |

Publish only sanitized observations and hashes. Raw preflight output, host
addresses, UUIDs, inventory, roster and credentials belong in the ignored
operator directory or encrypted operator storage. Logs from synthetic local
checks may be retained under this evidence directory after review.
