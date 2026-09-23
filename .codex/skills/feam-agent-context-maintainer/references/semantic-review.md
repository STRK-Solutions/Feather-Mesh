# Semantic Context Review

Run this review after the structural checker. It is an agent/human decision checklist, not a natural-language regex specification. A structurally valid document can still give wrong instructions.

- Route a direct peer-read task from `AGENTS.md` to `data_access.md`, the workplan, and the shared peer-access skill. Confirmed requirements require provider registration, manifests as intended authority, and first-class direct reads. SQLite is the observed current implementation. The future P0 contract settles schemas/defaults; older PDDs/plans cannot override the feature requirements or explicit user instructions.
- Review proposed `**/*.parquet` discovery: reject it for pinned access and require a test proving an added unregistered shard never enters the result. Inventory preparation by a provider is distinct from publication.
- Review resolving an unregistered but readable TIFF: reject it, including explicit path lookup, and require a public-boundary not-registered failure test.
- Review reopening a removed peer via its saved physical path: reject it and require a test where the physical file remains readable but new access through Feather Mesh fails.
- Review staging collision tests: require disposable temporary sources/destinations, hard-link/symlink identity and overlap checks, and preservation of source and prior output bytes after failure. Never exercise destructive tests on provider data.
- Read ownership and exit-code guidance for meaning, not just table/link presence. Core owns workflows/shared DTOs/domain errors; repositories own SQL; CLI owns rendering/process behavior. SDK/HTTP cannot implement separate publication or visibility rules. A requested CLI migration needs documented schema/error/exit-code compatibility and tests, not another approval solely because of preservation guidance.
- Require installed-package/subprocess evidence for SDK changes, native `LazyFrame` execution over fixed membership, known Rasterio window values, pinned STAC schemas plus real authenticated paginated client tests, and documented inspector feature configurations. Commands enter guidance/CI as each component arrives. Missing dependencies/tests and actual HPC evidence must remain explicit; local mocks cannot complete multi-node acceptance.

## Checker Regression Interpretation

The review's five prose mutations have deliberately different structural and semantic outcomes:

| Mutation | Structural result | Semantic assessment |
| --- | --- | --- |
| Append a Markdown-wrapped claim that `python_mvp/` is primary | Pass, with structural-only scope notice | Reject: conflicts with current Rust implementation/routing. |
| Append SQLite-authority and recursive-discovery instructions for peer access | Pass, with scope notice | Reject: conflicts with manifest authority/fixed inventories. |
| Remove ownership rows or exit-code routing | Fail for missing structure | Also restore and review their meaning; keeping IDs with wrong prose could still pass. |
| Rephrase the historical prototype note without changing meaning | Pass | Accept; exact English wording is not a contract. |
| Append “Never choose the source of truth from python_mvp/.” | Pass | Accept; negation is not a contradiction. |

When a new skill is substantial, independent forward-testing can add confidence if delegation is authorized. At minimum evaluate the cases above from the instructions and record the conclusions alongside structural, loadability, and runtime evidence without conflating them.
