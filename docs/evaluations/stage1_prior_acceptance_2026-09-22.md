# Stage-1 acceptance record

Status: **Stage 1 incomplete: local implementation and verification gaps remain, alongside live-router acceptance. Existing local checks are recorded below. Target-HPC acceptance is a separate follow-up.**

## Local evidence recorded

| Date | Command | Result |
| --- | --- | --- |
| 2026-09-22 | `cargo test -p mesh_core --test peer_access_tests` | Passed: 2 tests before Stage-1 edits. |
| 2026-09-22 | `cargo test -p mesh_cli --test cli_workflow_tests` | Passed: 5 tests before Stage-1 edits. |
| 2026-09-22 | `cargo check --workspace --all-features` | Passed after adding `mesh_core` catalog/operations, `mesh_tui`, and `mesh_agent`. |
| 2026-09-22 | `cargo test -p mesh_core -p mesh_agent -p mesh_tui --all-features` | Passed: core catalog/journal, fake-provider/harness/SSE parser, and terminal-render tests. |
| 2026-09-22 | `scripts/tui_agent_stage1_demo.sh /tmp/feam-stage1-demo --reset` | Passed: creates provider/client synthetic fixture pair, versions, unregistered asset, and unavailable peer. |
| 2026-09-22 | `cargo fmt -- --check` | Passed. |
| 2026-09-22 | `cargo clippy --workspace --all-targets --all-features -- -D warnings` | Passed. |
| 2026-09-22 | `cargo test --workspace --all-features` | Passed: `mesh_agent` 5, `mesh_cli` 6, `mesh_core` 25, and `mesh_tui` 2 unit/integration tests (plus doc tests). |
| 2026-09-22 | CLI-only/manual/hosted feature matrix | Passed: `cargo test -p mesh_cli --no-default-features`, then with `--features tui`, then with `--features agent-hosted`. |
| 2026-09-22 | `expect` pseudo-terminal launch/quit of `mesh_cli --project '/tmp/feam-stage1-demo/client with spaces' tui --agent off` | Passed: a real pseudo-terminal entered and restored the alternate screen after `q`; terminal path-with-spaces behavior was exercised. Resize and signal/panic paths remain unexecuted. |
| 2026-09-22 | Fresh `/tmp/feam-stage1-sdk-venv` install, out-of-tree `import feam`, and `FEAM_E2E=1 ... pytest python_sdk/tests` | Passed: 4 tests, including native Polars lazy query and authenticated paginated STAC/Rasterio window round trip. |
| 2026-09-22 | Context checker and its Python tests | Passed: `check_agents_context.sh`; 13 Python tests. Semantic review confirmed manifest/direct-read authority and route/staging invariants remain intact. |

The SDK run emitted four Rasterio `PendingDeprecationWarning` warnings about
Affine multiplication; it otherwise passed. The package build/test artefacts
created by that temporary verification environment are not retained in this
change set.

The local checks do not cover every phase-level negative/recovery case. In
particular, a resize/signal/panic pseudo-terminal test, fake HTTP/SSE transport
coverage, complete manual producer/consumer/restart flows, agent mutation
adversarial/reconnect cases, performance/RSS measurements, and a clean-checkout
run remain explicit local gaps.

The [workplan phase checklists](../../tui_agent_harness_stage1_workplan.md#5-delivery-phases)
also identify unfinished implementation: background core operations, complete
manual forms/reviews and restart reconciliation, streamed/session-aware agent
interaction, enforced disclosure and local-consent controls, mutation-handle
wiring, and a distinct evaluation corpus with an executable assertion-based
runner. The current 100 task IDs repeat category templates; their distribution
test is not a completed model evaluation. The status review did not rerun Rust,
SDK, terminal, or provider acceptance tests.

Documentation validation on 2026-09-22 passed with these commands from the
repository root:

```bash
PATH="/tmp/feam-context-venv/bin:$PATH" bash .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh
PATH="/tmp/feam-context-venv/bin:$PATH" /tmp/feam-context-venv/bin/python .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py
```

The structural check and all 13 checker tests passed. The default interpreter
first failed because PyYAML was unavailable; the existing virtual environment
provided PyYAML 6.0.3. A separate local-link/anchor and whitespace check passed
for all 39 local links in the two edited documents. Semantic review preserved
manifest authority, shared-service ownership, and the separation of local,
live-router, and later HPC evidence. These are documentation checks only.

## Pending live-router acceptance

Credentials and a total paid-testing budget have now been supplied; the
synthetic model/provider screening is recorded below. Full Stage-1 acceptance
still requires a real authenticated Rust/TUI streaming/tool-call walkthrough
and at least 100 distinct held-out tasks with authoritative state assertions,
including provider/model/date/usage/cost. The development screen does not
complete either requirement.

## Separate target-HPC acceptance

Target-HPC terminal/SSH evidence, scheduler and CPU/GPU resource measurements,
separate identities, target-filesystem behavior, and multi-node peer-access
checks remain pending under the [HPC checklist](../data_access_hpc_checklist.md)
and later design milestones. They do not block Stage 1 under the
[workplan scope boundary](../../tui_agent_harness_stage1_workplan.md#2-scope-and-completion-boundary).
Local TUI/harness footprint and responsiveness measurements remain Stage-1
requirements.

The historical fake-provider/parser checks cover only their exercised local
cases. The subsequent router work below adds one successful HTTP boundary case;
neither establishes the full application acceptance or HPC operation.

## Subsequent router screening and transport check

The [model screening report](../tui_agent_model_screening.md) records a separate
2026-09-22 run after the earlier evidence above. Credentials authenticated
successfully; the authorized paid testing ceiling is US$20 total, including
retries. Five exact model/provider profiles were screened on 16 synthetic
development tasks each, with simulated tool results. The 96 requests reported
US$0.0116439174; a preceding paid streaming probe brings known paid spending to
US$0.0116721174. No key, private key path, or reasoning trace is retained here.

The router now translates dotted tool names and serializes correlated tool
history in the provider wire format. Default and all-feature tests, required
format/lint checks, eight hosted-crate tests (including a local HTTP/SSE round
trip), and seven offline screening tests passed. The HTTP test first failed
because sandbox loopback binding was denied, then passed with loopback access.

This supersedes the earlier absence of any HTTP boundary evidence only for the
tested successful round trip. The full transport failure suite, live Rust/TUI
walkthrough, distinct held-out evaluation, disclosure/budget enforcement, and
other implementation gaps remain pending. The reported model scores are strict
development checks, not application acceptance or HPC evidence. See the report
for exact commands, profiles, timing conditions, failure interpretation, and
full synthetic results.
