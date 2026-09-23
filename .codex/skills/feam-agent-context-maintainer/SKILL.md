---
name: feam-agent-context-maintainer
description: Use when AGENTS.md is missing, stale, contradictory, or needs self-healing; when repo structure, Cargo workspace layout, CI, commands, tests, or source-of-truth docs change; or when asked to update Feather Mesh AI-SDLC, agent, onboarding, or project skill context.
---

# Feather Mesh Agent Context Maintainer

Use this skill to keep project agent context small, accurate, and durable.

## Sources To Read

Read only the high-signal files needed for the drift being investigated:

- [AGENTS.md](../../../AGENTS.md)
- `README.md`
- `feather-mesh/README.md`
- `feather-mesh/Cargo.toml`
- [CI workflows](../../../.github/workflows), including Rust and agent-context checks
- Changed files relevant to the current task
- For peer data access, [requirements](../../../data_access.md), the [workplan](../../../data_access_implementation_workplan.md), and [feam-peer-data-access](../feam-peer-data-access/SKILL.md). Read the existing [contract](../../../docs/data_access_contract.md) for settled decisions. Confirmed requirements govern over proposed defaults; source/tests describe observed behavior. Explicit user instructions take precedence. Older plans are historical for this feature.

Do not load long project artifacts, PDFs, or generated outputs unless the task specifically targets them.

## Self-Healing Rules

- Patch `AGENTS.md` when it is missing, stale, or contradicted by durable repo facts.
- Keep `AGENTS.md` as a routing and decision guide, not a duplicate README.
- Prefer durable facts: source-of-truth directories, crate boundaries, validation commands, tested contracts, and ownership rules.
- Remove stale statements instead of adding caveats around them.
- Keep the file concise; target under 120 lines.
- Do not include temporary branch status, implementation plans, sprint notes, or speculative roadmap.
- Preserve concise links to active feature requirements, contracts, and applicable skills; these routing pointers are not embedded phase checklists. Mark absent commands, SDK directories, and adapter crates as planned until source evidence supports them.
- Keep SDK/HTTP adapters dependent on shared Rust publication/resolution rules, core DTOs/errors, and their own runtime tests. Refresh guidance and CI when each component/configuration is introduced, before that phase is complete; do not wait solely for final documentation cleanup.
- Include active requirement/plan files and their local dependencies in the handoff/change set. Check untracked files explicitly so a fresh checkout retains routing targets.

## Drift Check

The structural checker requires Python 3.11+ and the dependency in [scripts/requirements.txt](scripts/requirements.txt). Install it in a virtual environment, then run from the repository root:

```bash
python3 -m venv /tmp/feam-context-venv
/tmp/feam-context-venv/bin/python -m pip install -r .codex/skills/feam-agent-context-maintainer/scripts/requirements.txt
export PATH="/tmp/feam-context-venv/bin:$PATH"
.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh
python3 .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py
```

From the Cargo workspace use `../.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh`. From any other directory invoke the script by absolute path. The default root is relative to the script; `--root /path/to/fixture` selects an explicit root and must fail when that root lacks `AGENTS.md`.

The checker checks local inline Markdown link targets, required routing links, skill YAML/name/body structure, Cargo members, current CLI source anchors, ownership/affected-surface table rows, and the three mandatory Rust commands. Table keys are structural IDs; prose may change freely. Keep those checks/tests aligned when the documented structure changes. The context checker is structural tooling; installed SDK/runtime evidence is recorded separately.

A pass reports **structural checks only**. It does not interpret contradictory prose, prove feature readiness, or run runtime tests. Review [references/semantic-review.md](references/semantic-review.md) separately; regressions deliberately show that contradictory appended prose can pass structural checks.

## Skill Discovery

Repository skill sources live in `.codex/skills/`, with `.agents/skills` linking to that directory for current Codex repository discovery. Keep the link relative and avoid duplicate skill copies. [Official skill discovery documentation](https://learn.chatgpt.com/docs/build-skills) describes repository locations and symlink support.

After adding a skill, validate its frontmatter/body with the skill-creator validator and check its linked resources. Confirm it appears enabled with no load errors using a fresh skill catalog or the local app-server `skills/list` request with this repository's `cwds` and `forceReload: true`. Verify from both root and Cargo workspace. Read the returned `SKILL.md` path. Creating a directory does not prove the already-loaded conversation catalog refreshed; use the explicit AGENTS link in that session. [App-server protocol](https://learn.chatgpt.com/docs/app-server) documents catalog refresh.

## Validation

After updating context, verify that a future agent could answer:

- Where should a CLI behavior change go?
- What tests should run after service logic changes?
- Which workspace owns the implementation?
- Where are stable CLI exit codes documented and tested?
- Which source governs direct peer reads, which defaults are unsettled, and what does current code actually implement?
- Do CLI, SDK-only, HTTP-only, and core-only tasks find the shared invariant skill?
- What proves a native Polars lazy query, a known Rasterio window, and authenticated paginated STAC HTTP behavior?
- Which supported native-inspector feature configurations ran, and did missing inspectors fail publication?
- Which checks were unavailable or skipped, and which separate-user/multi-node HPC requirements remain pending?

Use the semantic checklist in addition to structural checks. Neither Rust-only success nor static STAC output establishes missing adapter behavior.
