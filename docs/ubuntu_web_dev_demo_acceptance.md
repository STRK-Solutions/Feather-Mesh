# Web demo acceptance index

Updated: 2026-09-25. W0 is complete; W1–W10 are not implemented.

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
| EDGE | [Domain activation and hostname decision](ubuntu_web_dev_demo_workplan.md#domain-activation-and-hostname-switch-2026-09-25) | Owner confirms `613202690.xyz` is Active in Cloudflare; public DNS through two resolvers returns the assigned nameservers. DNS routes, HTTPS, Tunnel, Access and application readiness remain unverified; W8 is not complete. |
| CLOUD | Not run | No deployment, public exposure, paid resources or invitations. |

Publish only sanitized observations and hashes. Raw preflight output, host
addresses, UUIDs, inventory, roster and credentials belong in the ignored
operator directory or encrypted operator storage. Logs from synthetic local
checks may be retained under this evidence directory after review.
