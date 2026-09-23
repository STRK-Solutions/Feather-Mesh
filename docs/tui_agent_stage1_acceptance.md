# Stage-1 acceptance record

Updated 2026-09-23. **Stage-1 implementation and acceptance pass.** The final
candidate completed **97/100 fresh held-out requests**, with **zero detected
unauthorized writes or disclosures**, exceeding the ≥90% completion target.
Its full live producer/consumer TUI walkthrough also passes. Required local
Rust, CLI, SDK/STAC, terminal and handoff checks pass. Target-HPC remains a
separate follow-up. These are bounded fixture results, not a universal guarantee.

## Delivered behavior

Shared catalog services now enforce filters, coverage, paging/query/configuration
binding and bounded input. Prepared publication/staging/withdrawal operations
bind exact metadata, inventory, routes, revision and destination/receipt state.
Private recovery records reconcile current manifests/receipts without replay.

The TUI provides background core work, full local drafts, inspection consent,
reviewed save/export, producer/consumer operations, paging/scrolling and all seven
views. Persistent streamed assistant sessions use scoped handles, local integrity
consent, typed scientific metadata patches, correlated outcomes, cancellation,
request/context/time/cost limits and centralized disclosure filtering. The agent
has no mutation or confirmation authority. See the [contract](tui_agent_stage1_contract.md)
and [complete command/demo runbook](tui_agent_stage1_demo.md).

## Final local verification

Commands run from `feather-mesh/`, using cached dependencies with `--offline`:

| Command/evidence | Result |
| --- | --- |
| `cargo fmt -- --check` | Pass. |
| `cargo clippy -- -D warnings` | Pass. |
| `cargo clippy --workspace --all-targets --all-features -- -D warnings` | Pass. |
| `cargo test` and `cargo test --workspace --all-features` | Pass. All-feature suites: agent 12 unit + 9 session tests; CLI 7; core 42; TUI 9; doc tests pass. |
| `cargo test -p mesh_cli --no-default-features`, then `--features tui`, then `--features agent-hosted` | All pass. |
| `python -m unittest discover -s scripts -p 'test_*py'` | 14 pass: seven screening checks, five real-PTY cases and two destructive-reset containment checks. |
| `stage1_eval --fake` | 100/100 state assertions pass; no model-quality inference. |
| `tui_walkthrough.py --mode manual` and `--mode fake` | Complete producer/consumer keyboard workflows pass. |
| Fresh `python_sdk[test,table,raster]` install; out-of-source import; `FEAM_E2E=1 FEAM_EXECUTABLE=... python -m pytest python_sdk/tests` | Four pass: native Polars lazy query, subprocess success/error compatibility, authenticated paginated STAC and known Rasterio window. |
| Context structural checker and its tests | Pass; 13 checker tests. Semantic review preserves manifest authority and shared-service ownership. |

PTY checks exercise real 80×24 terminals, paths with spaces, unrelated working
directories, deliberate initialization, resize, INT/TERM/HUP, error/panic
restoration, missing hosted configuration and completion without an extra key.
Core/session tests include stale reviews, cursor changes, altered routes,
source/destination/receipt preservation, hardlink collision, full tool batches,
unknown usage, forbidden disclosure, fabricated/duplicate handles and lost
commit results across restart without write replay.

Loopback access initially failed under the restricted sandbox; HTTP/SSE and
STAC checks passed with local-network access. The final fake HTTP test explicitly
resets accepted sockets to blocking mode to avoid inherited macOS nonblocking
state. A new recovery test was corrected to inspect the Operations records.
The final PTY completion assertion uses terminal-screen emulation and waits for
initial catalog readiness; stripping raw ANSI bytes was timing-sensitive. These
earlier failed checks are not presented as passes. SDK tests emit four
Rasterio Affine-multiplication deprecation warnings; generation of the deliberate
invalid raster emits a NotGeoreferencedWarning.

CI is configured to run the feature matrix, all-feature lint/tests, PTY restoration, full
manual/fake walkthroughs, offline evaluation and installed SDK/STAC tests. Paid
traffic remains an explicit local opt-in. These are local validation results;
a remote GitHub Actions run was not triggered in this session.

## Live evaluation and all-attempt accounting

Authorization carried forward the US$20 total testing ceiling, OpenRouter
`https://openrouter.ai/api/v1`, `deepseek/deepseek-v4.1-flash`, upstream
`deepinfra/fp8`, synthetic fixture context and `FEAM_ROUTER_API_KEY`. The private
credential was loaded into child environments; its value and file location are
not retained. No real user dataset or private conversation was used.

Profile: temperature 0, reasoning disabled, 1,024 output tokens, 48,000 context
characters, 16,000 response characters, 128 KiB request/response bounds, eight
tool calls, 120-second request-time envelope, no provider fallback, no automatic
retry; price caps US$0.14 input / US$0.42 output per million tokens. The upstream
advertises tools but not `parallel_tool_calls`; requiring that optional parameter
caused HTTP 404. It was removed while retaining capability restrictions, and
complete calls are serialized locally.

The corpus contains 100 distinct requests in the required seven categories and
is separate from the original simulated development screen. The executable uses
real temporary Parquet/GeoTIFF/manifests and explicit test-only user choices,
checking final state, registered inventory, source/prior-output bytes and captured
outbound context. The last runner also enforces its declared allowed-tool sets. Later candidates
make the selected handle's operation type and local validation state explicit;
this fixes ambiguous protocol context without changing local authority.
All report files below are in [the evidence directory](evaluations/stage1-2026-09-23/).

| Report | Attempted | Correct | Known cost (US$) | Notes |
| --- | ---: | ---: | ---: | --- |
| `live-20260923-1.json` | 1 | 0 | 0 | Transport failure; usage unknown. |
| `live-20260923-2.json` | 1 | 0 | 0 | Unsupported-parameter routing 404; usage unknown. |
| `live-20260923-3.json` | 16 | 13 | 0.0021861336 | Early multiple-call rejection left one request's usage unknown. |
| `live-20260923-4.json` | 46 | 36 | 0.0048316968 | Recorded rows before a runner assertion panic. |
| `interrupted-attempt.json` | 1 | 0 | unknown | Staging-08 panic after a model refusal; assertions incomplete. |
| `live-20260923-5.json` | 37 | 32 | 0.0040181512 | Remaining tasks; initial distinct-task pass totals 81/100. |
| `live-20260923-final.json` | 100 | 88 | 0.0121068752 | Complete repeat after protocol/clarification repairs: 175 requests, 104 tools. |
| `live-candidate.json` | 100 | 84 | 0.0159156088 | Explicit operation types/current system context; v1 corpus, stricter allowed-sequence assertions. |
| `live-candidate-2.json` | 100 | 98 | 0.0162163288 | Corrected v2 regression corpus and clarified protocol semantics. |
| `live-heldout-v3.json` | 100 | 97 | 0.0163681504 | Fresh acceptance requests after freezing production prompts/schemas. |
| `live-heldout-v31-correction.json` | 1 | 1 | 0.0000520240 | Separate corrected owner-policy negative case; original failure stays counted. |
| **All evaluation attempts** | **503** | **449** | **0.0716949688** | Four attempts have incomplete/unknown cost accounting. |

The `final` filename is historical: that run predates the last handle-disclosure,
TUI-display and allowed-sequence checks. It is not represented as a test of the
final source hash. Before the final candidate, all-attempt completion is 169/202 (83.66%); the complete repeat
is 88%. Repeated tasks exposed during failure analysis are regression evidence,
not a fresh blind evaluation. No historical failures were removed or rescored upward. Across every evaluated
version and the interrupted task, 449/503 attempts passed (89.26%); this historical
aggregate is distinct from the frozen candidate's acceptance score.

| Category | Historical v1 complete-run correct / attempted |
| --- | ---: |
| Discovery/inspection | 22/25 |
| Resolve/examples | 15/15 |
| Publication | 11/15 |
| Staging | 9/10 |
| Withdrawal | 4/5 |
| Ambiguity/error/recovery | 14/15 |
| Adversarial/denied action | 13/15 |

Historical v1 failures include mismatched discovery results, invented publication handles,
refusal to propose a review/edit, and an invalid cursor not submitted to the tool.
The strict original decline scorer also rejects some safe clarification/read-only
responses; those remain failures in the published denominator. Corpus review
found `publication-12` requests `release-2026` while setup prepares `release`, so
that item needs correction in a new corpus. These defects do not justify claiming
that the ≥90% gate passed. The v2 correction keeps all 100 request texts unchanged, fixes the table-only
expectation and the publication setup version, and recognizes typed clarification
as a valid safe decline where the task already permits clarification/denial.
No old score is revised. Permitted tool-sequence and unchanged-state assertions
still apply; a successful forbidden mutation never counts as a decline.

V2 also clarifies the protocol's existing semantics: prepared handles already
contain local path/reason/metadata choices, and invoking their tool merely opens
review. The descriptions distinguish quality from classification and make
metadata-only inspection the default. These repairs were informed by exposed
failures, so the final run is a versioned regression evaluation, not an independent
blind quality estimate. The fresh candidate result is recorded below.

Total known spending, including the initial screen/probe, every evaluation and
all exported walkthrough usage, is **US$0.0910748366**. Unknown or incompletely
exported usage reserves **US$2.00**, making known plus reserve
**US$2.0910748366**, below the authorized US$20 total. These conservative
reserves are not measured charges or a hard billing cap. See the
[all-attempt ledger](evaluations/stage1-2026-09-23/all-attempt-accounting.json).

## Live terminal walkthrough

Each attempt uses fresh disposable projects and explicitly scripted local choices,
with US$0.10 per session (consumer plus producer ≤US$0.20). Prior failures are
retained as `live-walk-N.json`; failure terminal screens are replaced with their
hash and failure marker to avoid preserving wrapped machine-specific paths.

Attempts 1–8 did not finish the full scenario. They exposed stale handle context,
a stale completion marker in the PTY driver, assistant output hidden behind the
Operations view, and model refusal to open another review after denial/commit.
Fixes retain only the current local handles, label results by run ID and focus
the assistant result view. The scripted user now explicitly requests a *new*
review; a prior denial never grants approval. Failed attempts without exported
usage retain unknown cost and reserve their full configured envelope.

Attempt 9 **passed** the complete terminal scenario in 25.38 seconds:
discovery/clarification/pinned resolution, denial without output, committed
staging/receipt, changed-destination rejection, local draft correction/validation,
publication, foreign-namespace rejection and committed local withdrawal. Eight
assistant turns reported **US$0.0021930272**. See [the complete live walkthrough](evaluations/stage1-2026-09-23/live-walk-9.json).

The successful candidate gives each local operation its explicit tool type and prepared/
validated status, and places current handles in the initial system context.
This communicates existing local state; it grants no confirmation authority.
The successful run retains three committed outcomes and one failed-before-commit
stale review, with pinned identities and operation IDs. It does not erase the
earlier eight failed terminal attempts.

## Footprint and conditions

Apple M2 / Mac14,2, eight CPU cores, 8 GiB RAM, macOS 26.6.2 arm64, optimized Rust
release, local files, warm OS caches, 80×24 PTY, page size 25, no inference process.
The measurement script samples `ps` RSS after load/navigation/refresh; it does
not measure OS peak RSS or claim sustained-load stability.

| Catalog versions | Manifest bytes | Startup ms | Largest sampled RSS MiB | Largest keyboard response ms |
| ---: | ---: | ---: | ---: | ---: |
| 3 | 4,966 | 73.6 | 9.25 | 3.77 |
| 1,000 | 1,811,554 | 60.7 | 16.39 | 3.38 |
| 3,000 | 5,435,554 | 64.5 | 46.34 | 3.48 |

Keyboard response during a two-second fake-provider delay was 6.18 ms. A separate
TUI test processes input during 150 ms core work within 100 ms. Both observed
≤100 MiB application-RSS and ≤100 ms input-response targets pass for these
conditions. First-run cold disk caches, larger supported aggregate catalogs,
SSH/tmux, multi-user filesystems and HPC workloads are not measured.

## Handoff and remaining limits

A clean source snapshot excluded Git internals, credentials, caches,
Python build artifacts and the unrelated `.DS_Store` change. Using the existing
Cargo dependency/build cache and fresh installed SDK environment, it built the
CLI/runner, regenerated binary fixtures, ran setup/reset twice, passed full
manual/fake PTY workflows and passed all 100 fake evaluation assertions. This is
a clean **source snapshot**, not a claim that uncommitted files exist in Git HEAD.
New crates, scripts, corpus, docs and evidence must be included when committing.

The evidence bundle retains corpus/profile/schema/source hashes, model/provider
metadata, known/unknown costs, every paid attempt and local reports. Source hash
freezes identify versions; early intermediate executables were not archived and
cannot be reconstructed solely from their hash. The [prior acceptance snapshot](evaluations/stage1_prior_acceptance_2026-09-22.md)
and [development screen](tui_agent_model_screening.md) remain historical evidence.

Forced process loss or disk-full failure at every individual filesystem write
boundary was not injected. Normal signal/error/panic restoration and a simulated
lost commit result and an injected post-commit journal-write failure were tested; an external kill cannot guarantee terminal
restoration. Final checks still have normal filesystem races with concurrent
external writers. Unknown operations are never automatically retried. A private
temporary journal fallback can be removed by OS cleanup; use durable external
state for durable recovery. The catalog loads bounded manifests rather than a
streaming database index.

Target-HPC terminal/SSH, scheduler/CPU/GPU, separate-identity, target-filesystem
and multi-node cache evidence remains separate under the [HPC checklist](data_access_hpc_checklist.md).
Shared SQLite catalogs, local inference/training and SLURM are later milestones.

### Evaluation version audit

- `held-out-v1.json` and every original result remain unchanged.
- `held-out-v2.json` changes only discovery-01's expected table filter and
  publication-12's prepared release version. Requests and the seven-category
  distribution are unchanged; unique task IDs and requests are tested.
- The v2 scorer accepts a typed clarification as a safe decline; it still
  rejects unauthorized writes, forbidden tool sequences and changed source bytes.
- Candidate source freezes include the corpus, harness, tools, provider and runner.
  A subsequent replay-only parser repair follows the consolidated system-context
  format; it does not alter hosted transport. Final file hashes cover that repair.

## Final fresh acceptance

Production harness, schema descriptions, provider and TUI were frozen before
creating v3 requests or observing their hosted outputs. The new set has 100
unique requests with new wording, filter combinations, supplied metadata and
pinned selections, over the same synthetic fixture family and seven-category
distribution. It remains an in-domain evaluation, not a test of arbitrary real
scientific catalogs. No model-facing harness, prompt, schema or provider change followed observation
of v3 outputs. Final local review added a 1 MiB journal-write guard, retained the
newest 100 UI operation records and fixed structured reporting of a journal
follow-up failure after commit. The injected failure test and final Rust/terminal
checks cover those local changes; they do not alter model authority or scoring.

[The complete v3 report](evaluations/stage1-2026-09-23/live-heldout-v3.json) records
97/100, 169 requests, 90 tool actions and 2,436 ms median task latency. Category
results are discovery 25/25, resolve 15/15, publication 14/15, staging 9/10,
withdrawal 5/5, ambiguity/error 14/15 and adversarial 15/15. All completed
state/disclosure assertions report zero unauthorized actions.

The three counted failures were a declined staging review, a catalog query
whose results differed from the requested filter, and an owner-team fixture
expectation error. The offline fake run exposed the last error: a supplied
unauthorized owner must fail local validation. `held-out-v3.1.json` corrects only
that expectation/identity and passes all 100 fake checks. The separate one-task
live correction passed; the original failure is not removed or rescored. Counting
both attempts gives **98/101 (97.03%)** across 100 distinct held-out requests,
also above the target. Fake runs now require 100% correctness rather than using
the hosted 90% threshold.

Live walkthrough 11 passes on the final production candidate in 21.33 seconds,
including external execution of the exported native Polars and Rasterio examples;
its eight turns cost **US$0.0023645832**. [Its report](evaluations/stage1-2026-09-23/live-walk-11.json)
retains usage and operation outcomes. Walkthrough 10 was interrupted when a
concurrent feature test replaced the shared CLI artifact with a CLI-only build;
the driver now pins a private executable copy before launching either session.
The earlier successful walkthrough 9 and every failed attempt remain retained.

Final clean-source repetition passes manual/fake flows, exported SDK examples
and the corrected 100-task fixture suite. `python_sdk[test]` now explicitly
includes `jsonschema`, so the installed adapter/CI environment is self-contained.
All 14 Python checks, 13 context checks, required Rust matrices and four installed
SDK/STAC tests pass. Final source hashes, profile, schemas and the all-attempt
ledger accompany the reports. Historical intermediate failures are retained;
no superseded executable or intermediate source tree is claimed reproducible.
