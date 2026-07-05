---
name: feam-agent-context-maintainer
description: Use when AGENTS.md is missing, stale, contradictory, or needs self-healing; when repo structure, Cargo workspace layout, CI, commands, tests, or source-of-truth docs change; or when asked to update Feather Mesh AI-SDLC, agent, onboarding, or project skill context.
---

# Feather Mesh Agent Context Maintainer

Use this skill to keep project agent context small, accurate, and durable.

## Sources To Read

Read only the high-signal files needed for the drift being investigated:

- `AGENTS.md`
- `README.md`
- `feather-mesh/README.md`
- `feather-mesh/Cargo.toml`
- `.github/workflows/rust.yml`
- Changed files relevant to the current task

Do not load long project artifacts, PDFs, generated outputs, or the Python MVP unless the task specifically targets them.

## Self-Healing Rules

- Patch `AGENTS.md` when it is missing, stale, or contradicted by durable repo facts.
- Keep `AGENTS.md` as a routing and decision guide, not a duplicate README.
- Prefer durable facts: source-of-truth directories, crate boundaries, validation commands, tested contracts, and ownership rules.
- Remove stale statements instead of adding caveats around them.
- Keep the file concise; target under 120 lines.
- Do not include temporary branch status, implementation plans, sprint notes, or speculative roadmap.
- Keep Python MVP context explicitly secondary unless it becomes the active implementation source of truth.

## Drift Check

When useful, run:

```bash
.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh
```

Treat the script as a smoke check. Passing does not prove the context is complete; it only verifies the most important anchors exist.

## Validation

After updating context, verify that a future agent could answer:

- Where should a CLI behavior change go?
- What tests should run after service logic changes?
- Is `python_mvp/` the source of truth?
- Where are stable CLI exit codes documented and tested?
