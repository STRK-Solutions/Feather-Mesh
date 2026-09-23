# Stage-1 TUI and agent demo

The demo is synthetic and disposable. It does not send a request to a model by
default, and it never includes an API key in a project or fixture.

## Setup and reset

From `feather-mesh/`, build the CLI and generate the small binary source
fixtures when they are absent:

```bash
cargo build --bin mesh_cli
python3 -m venv /tmp/feam-stage1-fixtures-venv
/tmp/feam-stage1-fixtures-venv/bin/python -m pip install pyarrow rasterio numpy
/tmp/feam-stage1-fixtures-venv/bin/python mesh_core/tests/data/peer_access/generate_fixtures.py
scripts/tui_agent_stage1_demo.sh /tmp/feam-stage1-demo --reset
```

The reset script requires an absolute path and refuses a nonempty directory
without its own `.feam-stage1-demo-marker`. It deletes only the generated
`provider` and `client with spaces` children below a marker-owned root; source
fixtures are copied, never modified.

The resulting client has a local peer alias `nearby-climate` for provider
namespace `climate`, plus an intentionally broken `unavailable` route. The
provider has active `observations` v1/v2, active `temperature` v1, and a
readable but unregistered Parquet file. Expected table values are 1–4 / 11–14;
the raster's known 2×2 pixels are `[[7,8],[9,10]]`.

## Manual mode

```bash
cargo run -p mesh_cli --features tui -- tui \
  --project '/tmp/feam-stage1-demo/client with spaces' --agent off
```

Run it from any directory. `/` searches, Enter opens a pinned record, `e`
renders (but does not execute) direct CLI/SDK examples, `r` refreshes peer
status, and `:` opens commands. The manual commands are:

```text
:resolve PRODUCT_REF VERSION
:publish METADATA_JSON
:stage PRODUCT_REF VERSION DESTINATION [--overwrite]
:withdraw PRODUCT_REF VERSION REASON
```

Each mutation opens a review. `y` writes a private recovery intent and then
executes the shared operation; `n` proves the denial path makes no mutation.
Use the provider's generated metadata JSON to rehearse publication validation,
then change a required field to observe the typed failure. Remove the client
symlink only after a resolution test to demonstrate the changed-route failure.

## Fake/replay mode and evaluation

`mesh_agent::FakeProvider` is a deterministic in-process transport used by
tests; it is labelled replay/fake and is not evidence of model quality. The
versioned held-out corpus contains 100 task IDs: 25 discovery/inspection, 15
resolve/example, 15 publication, 10 staging, 5 withdrawal, 15
ambiguity/error/recovery, and 15 adversarial/denied-action tasks.

```bash
cargo test -p mesh_agent --all-features
```

The corpus asserts state and allowed tool sequences, not prose wording. A
release evaluation must report every attempted run, tool call, known/unknown
cost, and unauthorized-write/disclosure result. The initial target is at least
90% correct end-to-end task completion and zero unauthorized writes or
disclosures; it is not currently claimed by fake-provider unit tests.

## Hosted router mode (explicit opt-in)

Create a user-only config outside the project at
`$XDG_CONFIG_HOME/feam/agent.toml` (or `~/.config/feam/agent.toml`) using the
schema in [the Stage-1 contract](tui_agent_stage1_contract.md). Choose a
tool-capable model and a fixed provider explicitly, set a small budget, and set
the named key only in the environment:

```bash
export FEAM_ROUTER_API_KEY='<provided-out-of-band>'
cargo run -p mesh_cli --features agent-hosted -- tui \
  --project '/tmp/feam-stage1-demo/client with spaces' \
  --agent hosted --agent-profile demo-router
```

The status bar identifies the configured model/context policy without exposing
the key. Press `a` to compose an assistant request and `s` to stop generation.
The background worker keeps manual navigation available. It sends only the
profile-permitted user text and bounded, path-free tool summaries. A mutation
proposal is a local review, never an automatic write or a provider approval.

Record the selected model/provider, date, configured cap, actual usage/cost,
and evaluation report in the acceptance report before treating this as live
acceptance. Do not run live traffic without an authorized model/key/budget.

For a bounded comparison of hosted models before full TUI acceptance, use the
[synthetic model screening runbook](tui_agent_model_screening.md). It exports
current tool schemas from Rust, pins providers/prices, and records all attempts.
Its simulated results and development prompts do not complete the held-out
evaluation or the manual/hosted TUI walkthrough.

## Complete local controls and automation

Commands accept shell-style quotes as text parsing only; no shell is executed.
`Tab` cycles Catalog/Peers/Operations/Help/Teams/Cache/Lineage; `[`/`]` page,
PgUp/PgDn scroll, `/` searches, and `:filter '{"quality":"production"}'`
applies the shared filters. `NO_COLOR=1` disables color. Minimum size is 80×24.

```text
:project "/work/another project"
:draft new table
:draft load "/work/provider/draft with spaces.json"
:draft set /contact '"demo@example.test"'
:draft set /assets '[{"id":"data","path":"table.parquet","role":"data","media_type":"application/vnd.apache.parquet","digest_opt_out":false}]'
:draft validate
:draft save draft.json [--overwrite]
:export example.txt [--overwrite]
:recover
:agent-profile demo-router
:agent-draft
:integrity-consent product://climate/observations v1
:usage
```

Draft editing uses JSON pointers and values; table/raster templates expose the
complete scientific fields. Asset paths are relative to the serving root.
Drafts and exports are confined to `.feam/drafts` and `.feam/exports`, with
reviewed replacement. Inspection has a separate consent screen before reading
and hashing assets. Its validated result opens the final publication review.
`h` on a prepared review selects that exact operation for the assistant; it
neither approves nor executes it. `:agent-draft` exposes the current draft as
`selected-draft` for permitted metadata patches. Confirmations always remain
local. `s` stops generation; an already running core mutation finishes and
records its outcome before normal shutdown.

From `feather-mesh/`, after fixture generation and fresh SDK installation:

```bash
python -m pip install -r scripts/stage1-test-requirements.txt
cargo build -p mesh_cli --features agent-hosted
cargo build -p mesh_tui --features test-driver --example terminal_probe
cargo build -p mesh_agent --all-features --example stage1_eval
python -m unittest discover -s scripts -p 'test_*py'
python scripts/tui_walkthrough.py --mode manual --output /tmp/feam-manual.json
python scripts/tui_walkthrough.py --mode fake --output /tmp/feam-fake.json
target/debug/examples/stage1_eval --fake --output /tmp/feam-fake-evaluation.json
target/debug/examples/stage1_eval --fake --corpus mesh_agent/evaluations/held-out-v3.1.json --output /tmp/feam-heldout-fixtures.json
cargo build --release -p mesh_cli --features agent-hosted
python scripts/measure_tui.py --executable target/release/mesh_cli --output /tmp/feam-footprint.json
```

Use fresh output names; acceptance drivers refuse to erase previous records.
PTY tests cover keyboard input, resizing, interrupt/termination/hangup, panic,
error restoration and assistant completion without another keypress. Fake runs
exercise real services and explicit scripted keyboard confirmations, with no
model traffic. Footprint measurement uses warm local files, `ps` RSS samples
and a delayed loopback provider; it is not peak-RSS or SSH/HPC evidence.

For an explicitly authorized live run, the runner accepts a private profile
JSON matching `AgentProfile` and a private credential file. Only the launcher
reads the key and places it in the named child environment variable:

```bash
python scripts/run_stage1_live.py --live --profile /private/profile.json \
  --key-file /private/router-key --budget-usd 0.50 --output /tmp/feam-live-evaluation.json
python scripts/tui_walkthrough.py --mode live --key-file /private/router-key \
  --output /tmp/feam-live-walkthrough.json
```

The walkthrough pins the authorized screening model/provider and caps each of
its two sessions at $0.10. Edit that explicit profile only with an appropriate
model/provider authorization. Evaluation tasks initially held out from the
synthetic screen become regression tasks after examining their failures;
repeated results must not be presented as a new blind evaluation. See the
acceptance record for every attempted run, failures and remaining gates.

The default corpus is `held-out-v2.json`, a corrected regression set. The
separately frozen `held-out-v3.1.json` contains new request wording, filter
combinations, supplied metadata values and pinned selections over the same
synthetic fixture domain. Pass `--corpus` to either evaluation executable or
live launcher. The runner verifies 100 unique IDs and requests and bounds corpus
input to 1 MiB. Use fake mode first to validate fixture/scorer wiring without
inference. The final implementation was frozen before v3 live outputs were
observed; its results and limitations are reported separately from v1/v2.
