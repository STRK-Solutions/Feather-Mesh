# Adversarial Review of Agent Context for Peer Data Access

Review date: 2026-09-22. Scope: root `AGENTS.md`, the three repository skills, their context-check script, and their interaction with `data_access.md` and `data_access_implementation_workplan.md`.

Follow-up on 2026-09-22: the earlier prototype was preserved on `archive/python-mvp` and removed from the active implementation and routing. That change made the checker resolve the repository independently of the working directory (F5) and removed the prototype-specific wording/regex checks discussed in F6. The findings and line references below describe the pre-change snapshot; the other recommendations were still pending at that point.

Preparation fixes have now been applied and verified; see [completion evidence](#preparation-fix-completion). The findings below remain the historical review, with their original locations and recommendations.

Recommendation: make a bounded agent-context update before beginning implementation. The current instructions accurately describe the existing Rust CLI, but do not reliably direct agents through the architectural transition or require evidence for the new Python/STAC surfaces. Six findings follow, including two reproduced checker defects. P1 means address in the preparation pass; P2 means address before the affected implementation phase.

This review adds no implementation changes and does not rewrite the instruction files. Missing guidance below is a defense against incomplete or stale task context; it does not mean the full workplan itself omits every corresponding requirement.

## Findings

### F1 — P1: No durable routing rule resolves the competing product documents

**Locations:** [AGENTS.md](AGENTS.md#L3), [context-maintainer sources](.codex/skills/feam-agent-context-maintainer/SKILL.md#L10), [older agent workplan](feather-mesh/.agents/feather_mesh_workplan.md#L24).

The entry-point context identifies implementation directories but does not identify the peer-access requirements or explain their relationship to older documents. The maintainer's default reading list includes READMEs, Cargo, and CI, but no explicit feature-contract routing. The workspace README calls the revised PDF the product source of truth; `docs/feam_pdd.md` instead describes an authoritative central database and default managed copy. The older `.agents/feather_mesh_workplan.md` calls the Markdown PDD authoritative and incorrectly says the CLI is a placeholder and services support only teams.

**Failure scenario:** a fresh agent implementing a narrow table/resolver task follows the older agent workplan, extends the authoritative SQLite registry, or requires copying before reading. That can satisfy the generic Rust/CLI guidance while violating peer-manifest authority and first-class direct access.

**Required change:** add a short source-routing rule to `AGENTS.md`: for peer data access, read `data_access.md` and the new implementation workplan; once P0 creates `docs/data_access_contract.md`, use it for settled schema/API decisions. Distinguish confirmed requirements, proposed implementation defaults, and observed current behavior. Explicit user instructions continue to take precedence. Identify older plans as historical for this feature and add a superseded notice to the older `.agents/` plan rather than deleting it.

Update the maintainer skill to preserve such routing links while keeping phase checklists out of `AGENTS.md`. Its prohibition on embedding implementation plans need not prohibit a short pointer to the active feature contract. Do not present planned commands, a proposed SDK directory, or `mesh_stac` as already implemented.

**Verification:** a fresh agent asked which document controls direct peer reads should select the new feature requirements, correctly describe manifests as the intended authority, and still recognize SQLite as the current implementation. The new requirement and plan files are currently untracked; include them in the eventual implementation handoff/change set so these links survive a fresh checkout.

### F2 — P1: Layering rules do not protect the publication and access invariants

**Locations:** [AGENTS engineering rules](AGENTS.md#L39), [Rust skill change workflow](.codex/skills/feam-rust-workflow/SKILL.md#L17), [CLI service boundary](.codex/skills/feam-cli-contract/SKILL.md#L15).

The current rules explain where business logic and SQL belong, but provide no feature-specific checks for authoritative publication, fixed membership, configured peer routes, or mutation safety. A service-layer implementation can obey all those boundaries while discovering unregistered files with a glob, resolving cached physical paths after a link disappears, or treating SQLite and STAC as separately editable publication authorities. Existing copy helpers also remove the destination before copying; location-based guidance alone does not prevent reuse of that unsafe pattern.

**Failure scenario:** a Python or HTTP adapter receives only paths and implements its own discovery/visibility rules; alternatively, Rust exposes a broad recursive Parquet scan and labels it a pinned version. The result passes ordinary positive-path tests but breaks the document's publication gate.

**Required change:** add a concise routing instruction and a focused `feam-peer-data-access` skill, or an equivalent referenced section of an existing skill. One shared cross-interface skill is sufficient; separate near-duplicate skills for each adapter are unnecessary. It should trigger on publication/manifests, project/peer resolution, cache refresh, the supported SDK, STAC, and peer staging changes.

Its durable checks should require:

- One authoritative registered record and atomic publication; SQLite/STAC remain derived for the new workflow.
- Required metadata and physical format checks in core services, including failure when a required inspector is unavailable.
- One Rust resolver for CLI, SDK, and HTTP; fixed versioned inventories; no discovery by filesystem globbing.
- Explicit project anchoring, namespace checks, registered relative paths, and preservation of the configured access route without a cached physical fallback.
- Provider-data preservation and overlap/alias/receipt checks before staging mutations; safe recovery after publication/copy failures.
- Tests for unregistered files, changed working directories, unavailable peers, withdrawal, partial publication, and source/output collisions as applicable to the change.

Keep detailed policies and schemas in the feature contract. Mark draft choices such as nested-link rules and manifest filenames as defaults until P0 settles them.

**Verification:** evaluate proposed changes that use `**/*.parquet`, resolve an unregistered readable TIFF, or reopen a removed peer through a saved physical path. The instructions must clearly reject each and identify the required negative test. A safe source/output collision test must use disposable fixtures.

### F3 — P1: The completion guidance can produce a Rust-only green result

**Locations:** [AGENTS validation](AGENTS.md#L11), [Rust skill validation](.codex/skills/feam-rust-workflow/SKILL.md#L29), [maintainer verification questions](.codex/skills/feam-agent-context-maintainer/SKILL.md#L43).

All listed executable checks are Rust checks, and the maintainer's verification questions cover only the original CLI/service workflow. They do not establish ownership of supported SDK tests, installed-package behavior, STAC conformance/HTTP integration, native format dependencies, optional build configurations, or actual HPC evidence. Current CI likewise runs only Rust checks.

**Failure scenario:** an SDK-only change does not trigger the Rust skill; a feature-disabled build passes Cargo checks while its format validator is missing; static STAC schema tests are accepted as proof of Item Search; local permission mocks are reported as proof of separate-user HPC behavior.

**Required change:** add a concise affected-surface validation rule to `AGENTS.md` and route detailed checks to the relevant skill/contract. Keep the three existing Rust checks. Require the SDK's documented test/install commands for SDK changes, pinned-schema plus real-client HTTP tests for STAC changes, and tested feature configurations for optional format inspectors. Add each command to guidance and CI when its component is introduced, before claiming that phase complete; do not wait solely for P8 documentation cleanup.

State that missing tools/dependencies and unexecuted checks must be reported explicitly. Distinguish automated local verification from target-HPC evidence. An unavailable cluster should leave HPC acceptance pending while independent implementation proceeds. Neither skipped required tests nor static STAC output can establish completion of the missing runtime behavior.

**Verification:** the guidance must answer what proves a native Polars `LazyFrame`, a Rasterio window, authenticated paginated STAC search, and multi-node cache placement. Concrete commands should be added as the components exist; do not invent runnable Python/STAC commands before the package/server is created.

### F4 — P2: CLI contract protection lacks an SDK protocol and migration procedure

**Locations:** [CLI skill change rules](.codex/skills/feam-cli-contract/SKILL.md#L29), [CLI skill validation](.codex/skills/feam-cli-contract/SKILL.md#L37).

The skill protects names, JSON fields, and exit-code meanings, but does not explain how an authorized change becomes a versioned machine interface. The new SDK will depend on structured resolution results and errors. Existing errors are plain stderr text, and existing registry selection defaults to the process working directory.

**Failure scenario:** an agent considers a new JSON success response sufficient, but the SDK must parse human error text, diagnostic output contaminates stdout, or `--project` silently falls back to `registry.db`. Another agent retains a legacy publication path that bypasses the new metadata gate in the name of compatibility.

**Required change:** extend the skill with protocol requirements: versioned result/error schemas, stdout/stderr separation, explicit domain-error/exit-code mappings, project/registry conflict rules, pinned references, and subprocess compatibility checks. Require migration examples and tests for intentional breaking changes. Legacy compatibility must not silently admit unregistered entries into peer discovery. Shared DTO/error definitions belong in core; CLI rendering/process behavior remains in `mesh_cli`.

The user's instruction, “You can include the workplan to change the cli as required,” already authorizes necessary CLI evolution for this work. The existing preservation rule explicitly permits requested changes and is **not** an approval blocker. Keep that rule; clarify how to execute its exception. Persist the feature-specific migration decisions in the feature contract, not as temporary user-conversation history in a general skill.

**Verification:** test success and failure through the SDK subprocess boundary, including spaces in paths, missing executable, incompatible protocol, malformed JSON, and each relevant error category. Confirm ordinary `serve` still means publication and `stac serve` means the HTTP service.

### F5 — P2: The context checker reads the caller's working directory

**Location:** [check_agents_context.sh](.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh#L4).

The script assigns `file="AGENTS.md"`, so it checks the current directory instead of the repository containing the script. This conflicts with the normal workflow of changing into `feather-mesh/` for Cargo commands.

**Reproduction:** invoking the same script by absolute path from the repository root exits 0; invoking it from `feather-mesh/` exits 1 with `missing AGENTS.md`. The root file is present in both cases.

**Required change:** locate the repository root relative to the script, or accept an explicit root argument with a deterministic default. Support an explicit root override for isolated fixture tests. Document invocation from both repository and workspace directories.

**Verification:** invoke the checker from the root, Cargo workspace, and an unrelated temporary directory; each must check the same intended repository unless an explicit override is supplied. A missing file in the explicitly selected root should still fail.

### F6 — P2: The checker confuses wording matches with reliable drift checks

**Locations:** [required patterns](.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh#L11), [contradiction regex](.codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh#L28).

The checker requires one exact English phrase and uses an unstructured regex to detect Python-primary statements. Disposable copies of the real `AGENTS.md` produced these results:

| Mutation | Observed result | Assessment |
| --- | --- | --- |
| Append `` `python_mvp/` is the primary source of truth. `` | Pass | Misses a contradiction because Markdown interrupts the pattern |
| Append SQLite-authority and recursive-discovery instructions | Pass | Feature invariants are outside the checker's coverage |
| Remove layering rules and exit-code documentation guidance | Pass | The purported context anchors remain only keywords |
| Replace the prototype warning with equivalent historical-prototype wording | Fail | Rejects a harmless edit because the exact phrase changed |
| Append “Never choose the source of truth from python_mvp/.” | Fail | Flags a correct negative instruction as a contradiction |

The skill correctly calls this a smoke check; it never promised full semantic validation. The second and third cases demonstrate coverage limits, not a general-purpose security flaw. The false rejections and missed explicit contradiction are concrete reasons to change its implementation.

**Required change:** make the script check structural facts it can establish reliably: required routing/link targets exist, referenced skills are present and loadable, current workspace/command anchors resolve, and mandatory validation guidance is present in a maintainable form. Remove the exact-prose dependency and either replace the simplistic contradiction check with narrowly tested rules or leave semantic precedence to an explicit human/agent checklist. Do not replace it with increasingly broad regexes that claim to understand arbitrary prose.

**Verification:** add regression cases for the mutations above and clearly distinguish automated structural checks from semantic review. A passing result should name its limited scope and never serve as the sole readiness gate.

## Reproduced checks and limits

- Read every repository `AGENTS.md` and `SKILL.md` found by the hidden-file scan: one root instruction file and three repository skills. Global/system skills were outside this repository-readiness review.
- Compared the guidance against the current Cargo workspace, CI, CLI/service source, requirements, and implementation plan. No new runtime features were exercised.
- Ran seven isolated script cases: the actual root, actual Cargo directory, and five mutated copies in temporary directories. The checker and real instruction files were unchanged.
- Did not rerun Cargo tests: this assessment changed no Rust code and makes no new claim about Rust runtime correctness.

## Preparation change set

| File or area | Change before implementation | Completion evidence |
| --- | --- | --- |
| `AGENTS.md` | Add concise feature-contract routing, cross-interface ownership/validation, and invariant-skill routing; retain accurate current commands and crate names | Remains short; all current links resolve; planned components are clearly identified |
| `feam-agent-context-maintainer/SKILL.md` | Include feature-contract precedence, skill routing, supported SDK/API boundaries, and phase-local context refresh in its review checklist | A context refresh preserves requirement links without claiming planned features exist |
| `feam-rust-workflow/SKILL.md` | Route manifest/resolver/cache/staging work to shared invariant checks; include relevant feature configurations and adapter evidence | A core publication/resolver change cannot be signed off by positive Rust tests alone |
| `feam-cli-contract/SKILL.md` | Add authorized migration and SDK protocol procedure; retain exit-code meanings and CLI/core separation | Project/registry and JSON/error compatibility tests are required at the affected phase |
| New `feam-peer-data-access/SKILL.md` or equivalent focused guidance | Encode cross-interface publication/access checks and route detailed policy to the feature contract | Applicable to SDK-only, HTTP-only, and core-only tasks; discoverable through the project's skill mechanism |
| Context-check script | Fix root selection, replace brittle prose checks, add regression fixtures | Root/workspace/temp invocations behave consistently; semantic limitations are explicit |
| Older `.agents/` plan and feature documents | Mark older guidance superseded for this feature; include new documents in the handoff | A fresh checkout does not lose the active specification/plan or select the old baseline |

Use the skill-creator workflow when actually creating or editing skills. Check that a newly added skill is discoverable/loadable in the intended agent environment; do not assume creating a directory updates an already loaded session's catalog.

## What does not need changing

- The existing CLI command list and the `mesh_core`/`mesh_cli` ownership split accurately describe current code. Do not replace that list with unimplemented commands.
- A new Python SDK is compatible with Rust ownership of publication and resolution rules.
- The current `feam` user-facing name and `mesh_cli` binary artifact are compatible with the plan's configurable SDK executable path; a binary rename is not a prerequisite.
- The CLI-preservation rule is not a reason to request permission again. The user has authorized the required changes.
- The maintainer skill's concise-context rule is useful. Route to detailed requirements and skills instead of copying the full workplan into `AGENTS.md`.

The preparation pass is complete when the routing/precedence and invariant guidance are in place, the checker regressions are addressed, and a fresh agent can identify both the current implementation and the required new contracts. The workplan's “ready for implementation” status should then reflect that completed context check; this review alone does not apply the fixes.

## Preparation Fix Completion

The bounded context preparation pass is complete. P0–P8, the supported SDK, STAC HTTP behavior, and target-HPC acceptance remain unimplemented/unverified by this pass.

| Finding | Applied fix | Evidence |
| --- | --- | --- |
| F1 | Concise requirements/defaults/current-behavior routing in AGENTS, feature documents, skills, and READMEs; conditional P0 contract pointer | Active requirement/workplan links resolve; AGENTS retains the current nine commands and identifies SQLite as current behavior. The absent untracked older plan has a superseded routing notice at its former path, without inventing its missing contents. |
| F2 | Shared [peer-access skill](.codex/skills/feam-peer-data-access/SKILL.md) routes core, SDK, and HTTP changes through publication, resolution, and staging invariants | Semantic review rejects glob-based pinned discovery, unregistered readable TIFF resolution, and saved-physical-path fallback; each has a required negative test. Collision tests require disposable fixtures. |
| F3 | Affected-surface evidence in AGENTS/shared skill and phase-local workplan checks | SDK install/native LazyFrame, Rasterio window, real-client authenticated paginated HTTP, inspector configurations, and actual multi-node cache evidence are specified separately. Missing checks/dependencies and pending HPC acceptance must be reported. No nonexistent SDK/STAC commands were invented. |
| F4 | Versioned result/error protocol and authorized CLI migration procedure in [CLI skill](.codex/skills/feam-cli-contract/SKILL.md) | Guidance requires stdout/stderr separation, error/exit mappings, project/registry conflicts, pinned references, migration examples, and subprocess success/failure cases; publication `serve` and proposed HTTP `stac serve` remain distinct. |
| F5 | Script-relative default plus explicit `--root` override | Regression fixtures run from root, Cargo workspace, and an unrelated directory with spaces; explicit missing AGENTS fails. The actual repository was also checked from all three locations. |
| F6 | Structural link/skill/workspace/command/guidance checks, explicit semantic checklist, and CI regression workflow | 13 regression tests pass, including the five review mutation categories. Contradictory appended prose deliberately passes only the structural check and is rejected by the separate semantic review. |

Validation performed:

- Python 3.13.7 with PyYAML 6.0.3 in an isolated temporary environment: `bash .codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh` and `python3 .codex/skills/feam-agent-context-maintainer/scripts/test_check_agents_context.py` pass. Setup and invocation from other directories are documented in the [maintainer skill](.codex/skills/feam-agent-context-maintainer/SKILL.md).
- The skill-creator `quick_validate.py` passes for all four repository skills. A fresh local Codex app-server `skills/list` with `forceReload: true` returns all four enabled exactly once, with zero load errors, from both repository root and Cargo workspace. Each returned SKILL.md is readable. `.agents/skills` is a relative link to the canonical `.codex/skills` directory; this check does not claim the already-loaded conversation catalog was refreshed.
- Agent review using the [semantic checklist](.codex/skills/feam-agent-context-maintainer/references/semantic-review.md) confirms source precedence, adapter ownership, the three rejected access proposals, disposable collision tests, and the separation of local evidence from HPC acceptance. This was an in-session review, not an independent agent run or runtime test.
- `bash -n` and `git diff --check` pass. AGENTS is 67 lines. [Agent-context CI](.github/workflows/agent-context.yml) now installs the checker dependency and runs structural/regression checks on Python 3.11; hosted CI itself has not been run here.

Handoff includes `data_access.md`, `data_access_implementation_workplan.md`, this review, and their existing `map.md` assessment dependency along with the new guidance/checker files. These documents began as untracked files; retain them together in the eventual commit/change set. `map.md` remains a historical assessment, including its explicitly dated references to older untracked documents.

No Rust source changed, so Cargo checks were not rerun. No SDK, STAC runtime, native-format, or HPC checks were executed: those belong to the remaining implementation phases. `docs/data_access_contract.md` remains a P0 deliverable rather than a placeholder presented as a settled contract.
