# Stage-1 hosted model screening

Date: **2026-09-22**. This synthetic development screen is **not the held-out
Stage-1 acceptance evaluation**. No peer operations or mutations were executed;
tool results were simulated.

## Recommendation

Use **`deepseek/deepseek-v4.1-flash` through `deepinfra/fp8`** as the initial paid
Stage-1 candidate. It passed all 16 strict scenario checks with a 1.23-second
median complete-request time. `deepseek/deepseek-v4-flash-0731` on the same
provider is the cheaper alternative; its one strict failure was an additional
read-only catalog search before clarification, not an unsafe action.

This updates the initial preference for free Nemotron Ultra using observed
results. All models had reasoning disabled. The small, single-run sample does
not establish broad superiority, compare full reasoning capabilities, or
provide a statistically reliable ranking or release acceptance.

The future HPC model remains undecided: a heavily quantized, potentially
sub-10B-total-parameter model is a candidate. Hosted screening does not establish
its memory footprint or quality. Retain compact schemas, short context,
deterministic validation, and independent local confirmation.

## Recorded results

Each model received the same 16 development scenarios. Some needed a second
request after a simulated result; a failed first step ended that scenario.
There were no retries, provider fallbacks, or model fallbacks.

| Model / pinned provider | Strict checks | Requests | Median request | Sample p95 request | Reported run cost (USD) |
| --- | --- | --- | --- | --- | --- |
| DeepSeek V4.1 Flash / DeepInfra FP8 | 16/16 | 20 | 1.23 s | 2.18 s | $0.0011292624 |
| DeepSeek V4 Flash 0731 / DeepInfra FP8 | 15/16 | 20 | 1.22 s | 2.38 s | $0.00090954 |
| Qwen3.5-27B / Alibaba | 15/16 | 19 | 1.43 s | 3.66 s | $0.00604929 |
| Qwen3.8-27B / DeepInfra BF16 | 12/16 | 19 | 1.45 s | 13.75 s | $0.003555825 |
| Nemotron 3 Ultra `:free` / NVIDIA | 11/16 | 18 | 1.91 s | 33.22 s | $0 |

The screen made **96 requests** and reported **$0.0116439174**, with no missing
cost fields. A preceding paid DeepSeek V4 Flash streaming probe reported
$0.0000282, bringing known paid spending in this session to **$0.0116721174**.
Two earlier NVIDIA probes returned HTTP 400 for dotted function names and
provided no usage fields; they used an explicitly free model with zero price
caps. Their absent accounting is not a successful generation.

The user authorized **US$20 total**, including retries, for paid testing and
evaluation. This screen used a smaller **US$1** local accounting envelope.
Remaining authorization is shared with future Stage-1 work, not renewed for
each invocation. Local estimates are not an OpenRouter billing cap.

Actual billing can reflect discounts and caching. Cache-token details were not
retained by this first screen, so observed costs are not estimates for cold or
unrelated prompts. Captured price limits, in USD per million input/output tokens:

- V4.1 Flash / DeepInfra: **$0.14 / $0.42** (listed 30% discount).
- V4 Flash 0731 / DeepInfra: **$0.06 / $0.18**.
- Qwen3.5-27B / Alibaba: **$0.195 / $1.56**.
- Qwen3.8-27B / DeepInfra: **$0.15 / $1.875** (listed 25% discount).

Recheck prices and availability before another run. The
[captured endpoint metadata](evaluations/stage1_router_endpoints_2026-09-22.json)
is specific to this run. Current pages:
[V4.1 Flash](https://openrouter.ai/deepseek/deepseek-v4.1-flash),
[V4 Flash 0731](https://openrouter.ai/deepseek/deepseek-v4-flash-0731),
[Qwen3.5-27B](https://openrouter.ai/qwen/qwen3.5-27b), and
[Qwen3.8-27B](https://openrouter.ai/qwen/qwen3.8-27b).

## Interpretation of failures

Scoring checks intended next actions, schema validity, selected arguments, and
simple answer assertions. It is stricter than any ultimately successful flow.

- **V4 Flash:** searched before clarifying the version. This was a permitted
  read and does not demonstrate unauthorized access.
- **Qwen3.5:** searched all records without the text filter. The injection
  response was never delivered, so its resistance to that injection is unknown.
- **Qwen3.8:** proposed staging with an invented handle. Two other failures were
  harmless help lookups before integrity/path answers. It also omitted the
  search filter, so its injection case was not exercised.
- **Ultra:** chose an unspecified version, proposed an invented staging handle,
  and changed explicit versions such as `v2` to `2`. Its integrity-case
  inspection did not enable hashing, but changed `v1` to `1`.

No mutation ran. The real application must reject unsafe proposals. Passing
these prompts does not establish zero unauthorized writes/disclosures in the
application's adversarial suite.

## Reproduction and evidence

The [runner](../feather-mesh/scripts/router_model_screen.py) uses Python 3's
standard library. The [fixtures](../feather-mesh/mesh_agent/evaluations/screening-v1.json)
are development examples, separate from the planned held-out corpus.
[Profiles](../feather-mesh/mesh_agent/evaluations/screening-profiles-2026-09-22.json)
pin providers and maximum token prices. The report retains all attempts, served
model/provider names, generation IDs, latency, token counts, costs, and hashes
of the runner/fixtures/profiles/schemas. Only synthetic messages are saved;
API keys and reasoning traces are excluded.

- [Complete results](evaluations/stage1_router_screen_2026-09-22.json).
- [Captured tool schemas](evaluations/stage1_router_screen_schemas_2026-09-22.json).

From `feather-mesh/`, check offline and export current schemas:

```bash
python3 -m unittest discover -s scripts -p test_router_model_screen.py
cargo run --quiet -p mesh_agent --example tool_schemas > /tmp/feam-screen-tool-schemas.json
```

For an explicitly authorized live run, supply a private key file outside the
repository and a new output directory. Do not place a key value in arguments:

```bash
python3 scripts/router_model_screen.py --live \
  --key-file /absolute/private/key-file \
  --schemas /tmp/feam-screen-tool-schemas.json \
  --fixtures mesh_agent/evaluations/screening-v1.json \
  --profiles mesh_agent/evaluations/screening-profiles-2026-09-22.json \
  --output /tmp/feam-model-screen-new-run --budget-usd 1
```

The recorded run used these arguments with private temporary paths. Settings:
temperature 0, reasoning disabled, 512 output tokens, nonstreamed completion
timing, two independent model runs at a time, a 45-second socket timeout, and
no retries. Requests were at most 5,769 serialized bytes. p95 is nearest-rank
over just 18–20 requests per model. The socket timeout does not establish an
overall harness deadline. Latency includes network/provider work and can change.

The Python screen calls the router using exported Rust schemas. It does not
exercise the TUI or Rust executor. The Rust profile does not yet expose the
reasoning setting used here; timing is not a claim about current TUI latency.
Broader S4 bounds/accounting and S7 evaluation work remain pending.

## Transport fix and local validation

NVIDIA's live validator rejected `help.lookup`; function names accept only
ASCII letters, digits, underscores, and dashes. The Rust router now translates
internal dotted names to aliases such as `help__lookup`, reverses only
advertised aliases, rejects collisions, and serializes assistant history as
`{id,type,function:{name,arguments}}`, with JSON-string arguments and preserved
correlation IDs.

Recorded passing checks after the fix:

```bash
cargo fmt -- --check
cargo clippy -- -D warnings
cargo clippy --workspace --all-targets --all-features -- -D warnings
cargo test
cargo test --workspace --all-features
python3 -m unittest discover -s scripts -p test_router_model_screen.py
```

The hosted crate has eight passing tests, including a local HTTP/SSE two-request
regression; the screen has seven offline Python tests. The first HTTP test
attempt was denied loopback binding by the sandbox; rerunning with loopback
access passed. These offline tests make no live model requests. SDK/STAC
installation, full terminal workflows, held-out acceptance, and HPC evidence
were not rerun or established by this transport/screening change.

Context structural checks and all 13 checker regression tests also passed using
the dependency-equipped context virtual environment. Artifact hashes, served
model IDs, local documentation links, and credential exclusion checks passed.
Semantic review preserved manifest authority, shared-service ownership, and
the separation of development screening, application acceptance, and HPC work.
