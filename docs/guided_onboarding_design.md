**Guided Onboarding — Design for Web Demo (TUI-centric)**

Overview
--------

This design replaces the previous lightweight onboarding doc and grounds the guided onboarding workflow in the repository's existing Stage-1 demo artifacts, test harnesses, and agent contract. It targets the web demo and the TUI wrapper (`mesh_cli` with `tui` feature). The goal is a minimal, secure, resumable guided flow that exercises publication → consumer discovery → table preview while remaining testable via PTY and existing fake-provider fixtures.

Key references (in-repo)
-----------------------

- Stage-1 demo runbook: `docs/tui_agent_stage1_demo.md` — fixture generation, manual commands, hosted-agent opt-in, and PTY test guidance.
- Stage-1 contract: `docs/tui_agent_stage1_contract.md` — agent profile schema and tool exposure rules used in hosted mode.
- Existing test harnesses and scripts: `scripts/tui_walkthrough.py`, `examples/stage1_eval`, `scripts/tui_agent_stage1_demo.sh`.

Guiding principles
------------------

- Reuse existing flows: leverage current `:draft`, `:publish`, `:resolve`, `:stage` and `:recover` commands and backend services.
- Minimal invasiveness: initial implementation is controller-only + non-invasive UI panels in `mesh_tui` plus tests; no breaking API changes.
- Evidence-first: each step requires verification (hashes, fingerprints, successful reads) before advancing checkpoint.
- Privacy-safe agent use: agent coaching uses bounded, path-free summaries per `tui_agent_stage1_contract.md` and the SDLC `v41flash/OpenRouter` note.

User flow (TUI screens)
----------------------

1. Welcome / Mode selection
- Prompt: `Guided Onboarding — Choose mode` (Publish / Consume / Both)

2. Project binding
- Prefills from `:project` and `.feam/project.toml`; fields: `project_root`, `serving_root`, `namespace`, `owner_team`.
- Actions: `Validate` (runs `:draft validate` style checks), `Save`.

3. Asset selection
- Filesystem browser limited to `serving_root` with checkboxes for assets; confirmation creates an in-memory `Draft`.

4. Metadata editor & validation
- JSON form with helpers for required fields. Runs same validators as `:draft validate` (local hashing optional with consent).

5. Publish review
- Shows preflight summary (inventory fingerprint, size, destination path). `Confirm publish` executes a standard `:publish` review and commit flow.

6. Add peer (consumer)
- Form to add peer alias and local path; reads manifest(s) and displays available products.

7. Discover & select version
- Refresh discovery; select product + version. UI displays inventory fingerprint and metadata.

8. Table preview
- Column selector, row limit, run preview. Uses new `TablePreview` service to read up to N rows safely in background.

9. Summary & export
- Summarize created product/version, published fingerprint, and produce exportable `walkthrough-report.json` for demo evidence.

Implementation components
------------------------

- `mesh_tui/src/tutorial.rs` — controller and state machine for the guided flow. Responsibilities: manage `TutorialState`, produce UI panels, call core services, checkpoint state, and report progress.
- UI panels in `mesh_tui/src/ui/*.rs` (or within `tutorial.rs`) — minimal panels for the nine screens above. Keep panels small and test-friendly.
- `mesh_core/src/services/table_preview.rs` — service implementing `TablePreviewRequest` and `TablePreviewResult`. Use `polars` or `pyarrow` bindings consistent with existing code. Limit reads to 100 rows and 64 KiB display.
- Background worker integration — reuse existing executor pattern in `mesh_tui` (or add a small threadpool) with progress/cancellation channels.
- Checkpoint persistence — private JSON under `.feam/tutorials/<project-hash>.json` containing evidence entries (fingerprints, timestamps, step status). No file contents.

Testing and QA
--------------

- Unit tests: `mesh_core` tests for `table_preview` and validators.
- PTY walkthrough: `mesh_tui/tests/tutorial_pty.rs` driving the TUI using `examples/terminal_probe` with synthetic fixtures created by `scripts/tui_agent_stage1_demo.sh`.
- Evaluation: use `mesh_agent` fake provider and the held-out corpus for automated runs as in `docs/tui_agent_stage1_demo.md`.

Workplan (concrete tasks)
------------------------

Phase 1 — Scaffolding (low-risk)
1. Add `docs/guided_onboarding_design.md` (this file) and remove previous doc — DONE.
2. Create `mesh_tui/src/tutorial.rs` scaffolding: `TutorialState`, `TutorialStep`, checkpoint I/O, and placeholder UI hooks — IN-PROGRESS.
3. Add `docs/tutorial_assistant_harness.md` — temporary harness for assistant-driven demo runs — DONE.

Phase 2 — Core features
4. Implement `TablePreview` service in `mesh_core` (request/result types + safe preview reader) — NOT-STARTED.
5. Wire background worker integration in `mesh_tui` to run validation/publish/preview tasks — NOT-STARTED.
6. Implement minimal UI panels and wire them to controller actions (no heavy styling) — NOT-STARTED.
7. Add checkpoint persistence and `:tutorial [start|resume|reset]` command dispatch — NOT-STARTED.

Phase 3 — Tests & demo
8. Add `mesh_tui/tests/tutorial_pty.rs` PTY walkthrough using fake provider fixtures — NOT-STARTED.
9. Add unit tests for `TablePreview` and controller state transitions — NOT-STARTED.
10. Add stage1 CI job to run PTY walkthrough in fake mode — NOT-STARTED.

Phase 4 — Polish and PR
11. Review security: ensure no credentials in repo; agent model usage aligned with `v41flash/OpenRouter` note — NOT-STARTED.
12. Remove temporary harness and docs after controller + PTY tests exist — NOT-STARTED.
13. Open PR with stubs, tests, and demo harness — NOT-STARTED.

Deliverables
------------

- `docs/guided_onboarding_design.md` (this file) — repo-grounded design.
- `mesh_tui/src/tutorial.rs` — controller stubs and placeholder UI wiring.
- `mesh_core/src/services/table_preview.rs` — preview service and tests.
- `mesh_tui/tests/tutorial_pty.rs` — PTY walkthrough harness.
- CI job definition to run PTY demo in fake mode.

Ownership and timeline
----------------------

- Suggested owner: `mesh_tui` lead (or assigned contributor). Estimated time: 2–4 days for Phase 1–2 scaffolding and preview; additional 2–4 days for tests and CI depending on availability of fixture generation and runner.

Risks and mitigations
---------------------

- Risk: heavy IO operations may block the TUI — mitigate by running all IO in cancellable background threads and reporting progress.
- Risk: model/coaching misconfiguration — mitigate via explicit opt-in and staging-only default.
- Risk: PTY flakiness on Windows — use cross-platform PTY harness patterns already present in `scripts/tui_walkthrough.py` and `examples/terminal_probe`.

Next action
-----------

I'll scaffold `mesh_tui/src/tutorial.rs` with types, checkpoint I/O, and placeholder functions plus a minimal README for the workplan. Confirm and I'll create the files and update the todo list accordingly.
