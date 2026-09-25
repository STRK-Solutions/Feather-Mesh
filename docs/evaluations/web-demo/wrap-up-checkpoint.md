# Web demo review checkpoint — 2026-09-25

Historical implementation checkpoint. The later [Ubuntu functional milestone](../../ubuntu_web_dev_demo_workplan.md#ubuntu-functional-milestone) supersedes the resume order and release prerequisites below: Cloudflare access/OpenRouter remain, R2/cloud hosts are shelved, and lengthy acceptance tests are deferred. The recorded results and failures below are unchanged.

Work stopped at the owner's request on `feat/complete-ubuntu-web-demo`.
Changes remain uncommitted; no commit or push was performed. The unrelated
`.DS_Store` change is preserved. No subagent or owned build process remains active.
The existing W1 runtime was left unchanged; graceful wrap-up stopped development,
not the previously running private W1 demo.

[Workplan](../../ubuntu_web_dev_demo_workplan.md) owns task checkboxes;
[acceptance index](../../ubuntu_web_dev_demo_acceptance.md) separates environments;
[execution record](w2-w7-progress.md) retains attempts and approvals.

## State to evaluate

| Area | Completed work and evidence | Remaining acceptance |
| --- | --- | --- |
| W0–W1 | Complete, including actual Ubuntu browser manual/fake flows, isolation, persistence/reset and unchanged repeat converge. | Later shared-host reboot and capacity gates remain separate. |
| W2–W3 | Identity/control DB, gateway, reconciler and finite lifecycle controller implemented/tested. Ubuntu pool expanded to 12 slots and 269 GiB across 16 filesystems; zero-change repeat converge. | Actual multi-identity service/socket/container integration, aggregate limits and recovery. |
| W4 | Bounded import/conversion/publication/grant code, real local Parquet/raster checks, approved climate acquisition and hash-verified Ubuntu source staging. | Pipeline dependencies, exact Ubuntu candidate approval/promotion and mounted readers. |
| W5 | Broker, private model adapter, durable accounting, cancellation/queueing and fake upstream tests. US$100 durable encrypted project ledger initialized with owner-confirmed zero prior charges. | Ubuntu end-to-end broker proof, finite allocation/model key and bounded live evaluation. No run allocation or live spending. |
| W6 | Capture, redaction, review/export, archive verification and metadata carryover implemented/tested. Private R2 bucket/30-day policy and synthetic PUT/list/delete verified; MFA enabled. | Received S3 credential scope/access, Ubuntu collector readback, deletion/retention, deployed capture and archive survival. |
| W7 | Service/Ansible/lifecycle recipes, encrypted operator vault, separate Terraform modules and native Rust/Go builds. Participant image context prepared. | Current native artifact, image build/smoke, full-VM recreation, Ubuntu service lifecycle and teardown proof. |
| W8 | Edge Terraform, connector role/preflight and negative probe suite pass local checks. Account/zone read access and MFA verified. | Public DNS/Tunnel/Access/HTTPS, actual login/revocation, browser and live hosted acceptance. |
| W9–W10 | Cloud recipes and local guards prepared; DigitalOcean explicitly shelved. | Ubuntu ten-user/reboot proof; deferred cloud lifecycle; cohort/operator acceptance and invitations. |

Beyond W0/W1, completed task IDs are W2.01, W2.02, W2.05, W3.01, W4.01,
W4.04, W5.01, W5.04, W6.01, W6.04 and W7.01. Other implemented components
remain unchecked where a task's required environment proof is incomplete.
**No W2–W10 phase exit gate is complete.**

## Final verification

- MAC: `go test -race ./...`, `go vet ./...` and Go formatting passed, including the final collector carryover changes.
- MAC: `python infra/demo/scripts/check.py` passed 95 Python tests, shell/schema checks, eight Ansible playbook syntax checks and production-profile lint across 17 files. This now includes the edge suite.
- MAC: Terraform 1.14.5 formatting/validation and mock tests passed: edge 9, research storage 1, cloud host 2. The initial sandboxed provider handshake failed; rerunning with local socket permission passed. No live Terraform plan/apply ran.
- Earlier session Rust format/Clippy/tests, feature combinations, PTY/capture and local dataset-reader checks are retained in the component evidence. They were not rerun during this documentation checkpoint because Rust/data code did not change.
- UBUNTU: approved source snapshots built native Rust and all ten Go commands; native Go race/vet passed. The approved Go snapshot predates the final collector carryover and cannot represent the current full release. See [Rust](w7-native-feam.json), [Go](w7-native-web.json) and [image inputs](w7-participant-image.json).
- Final context structural checks, all 131 local links/anchors across seven changed documents, four evidence JSON files and `git diff --check` passed. All 96 unique workplan task IDs are preserved.

## Retained failure and safe stopping point

The Ubuntu pipeline installer failed on missing Python 3.12 `ensurepip`.
Its root-owned partial directory is
`/opt/feam/pipeline-releases/0c7703afe7fe3a6fcdb361c97eeae261d3c1142090adfef760c66ea92c7b59df/`.
It has no successful installation receipt; no current pipeline links were
activated and no host packages were changed. Preserve the directory for scoped
reconciliation; the installer rejects automatic retry into incomplete state.

The image context is retained on Ubuntu, but the image build did not start.
Two temporary extraction containers were never started and were removed.
No participant services, public route, billable inference or paid compute were
started. The previously verified W1 runtime and storage were preserved.

## Resume after review

1. Resolve the pipeline dependency and reconcile the partial installation. Freeze a current native source snapshot, including collector carryover, with a new digest.
2. Build/smoke the participant image and complete full-VM/private Ubuntu service integration. Existing W1 slot contents need deliberate preservation/reclamation before fresh controller initialization.
3. Copy the received R2 credential privately to the collector's intended owner/configuration and prove live readback, retention/deletion and archival. Local file presence is not token-scope or S3-access evidence.
4. Prepare/review the exact Ubuntu dataset release and the bounded public staging/model configuration at their existing approval gates. Then complete actual Access/browser, live hosted and ten-user checks.
5. Keep DigitalOcean deferred until the owner explicitly resumes it. No new permission request is pending merely to review this checkpoint.

Private inputs and logs remain in ignored `.local/demo-deployment/` storage.
The vault is revision 3; its key is outside the repository. The durable project
ledger and keys remain on the Mac; no budget keys or R2 credential were copied
to Ubuntu. Git alone is not a backup of those operator inputs.
