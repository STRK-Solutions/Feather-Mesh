# TUI and agent harness — Stage 1 contract

Contract version: **1**. This document defines the implemented Stage-1
application boundary. It complements, and never replaces, the authoritative
peer publication/resolution rules in [the peer data-access contract](data_access_contract.md).

## Entry points and build modes

`feam tui --project ROOT` is project-scoped and never enters legacy
`registry.db` mode. It requires an interactive terminal. If the root has no
`.feam/project.toml`, the UI offers the explicit `:init NAMESPACE [SERVING_DIR]`
action; merely opening a directory does not create a project.

| Mode | Build command | Behavior |
| --- | --- | --- |
| CLI only | `cargo test -p mesh_cli --no-default-features` | The command is recognized but explains that the optional TUI was not built. |
| Manual TUI | `cargo test -p mesh_cli --no-default-features --features tui` | Ratatui/Crossterm UI with no HTTP/router dependency. |
| Hosted assistant | `cargo test -p mesh_cli --no-default-features --features agent-hosted` | Manual UI plus `mesh_agent` router transport. Live requests remain opt-in. |

`mesh_cli` owns command parsing and process errors; `mesh_tui` owns terminal
lifecycle, focus, rendering, review screens, and local confirmations;
`mesh_agent` owns provider translation, bounded tool orchestration, and
serialization policy; `mesh_core::services` owns catalog/operation DTOs and
all peer rules. Neither the TUI nor the agent can edit a manifest directly.

## Menu and view contract

Catalog renders the product/version list beside selected product metadata.
Lineage renders the same selectable list beside the product's version lineage. Peers, Operations,
Help, Teams, and Cache are exclusive full-width views; Catalog-only open,
example, and paging controls do nothing there. Each view and the modal review screen retain independent
scroll positions, and asynchronous view results update their originating view
without stealing focus.

With `agent-hosted`, Assistant is an additional menu even when its profile is
off or invalid. Prompt submission selects it. Streaming and completed output,
user prompts, run state, and tool summaries remain available when navigating
away and back; completion in another menu marks Assistant unread instead of
changing focus. Retained rendered transcript text is bounded to 1 MiB by
removing the oldest complete entries, resets on project/profile changes, and
is not persisted automatically. Agent-produced drafts remain in Catalog and
do not replace the Assistant transcript. All rendered content remains subject
to terminal-control sanitization.

## Catalog contract

`feam.catalog.v1` exposes `CatalogQuery` and `CatalogPage`. The initial
implementation reads live configured manifests through the shared peer layer,
returns active versions only, sorts by qualified reference and version, and
limits each page to 1–100 records (default 25). `CatalogPage.coverage` reports
every attempted peer route; an empty entry list never means all peers were
available. The cursor is opaque and bound to a revision/availability
fingerprint including the query, canonical project and route configuration. If that fingerprint changes, the next page fails with a restart
requirement rather than combining different views.

The interface has no SQLite paths or schema. A later shared catalog can replace
the manifest reader only if it preserves qualified identity, limits,
freshness/coverage, and current-route checks before access.

## Review-bound operations and recovery

`PreparedPublication`, `PreparedStage`, and `PreparedWithdrawal` bind a review
to the project configuration fingerprint, expected manifest revision, exact
draft/inventory, and (for stage) destination/overwrite choice. Execution:

1. reopens the project;
2. checks the configuration and current manifest/inventory again;
3. performs publish/withdraw revision checks while holding the existing
   manifest lock, or re-resolves immediately before staging; and
4. returns `committed`, `failed_before_commit`,
   `committed_with_followup_error`, or `outcome_unknown`.

Before any TUI mutation, a private `OperationJournal` syncs a minimal intent
record. A result is recorded afterward. A journal failure after a commit is
reported as a follow-up/reconciliation problem, never as a rollback. Pending
intents are not idempotency keys and never authorize retry; users reconcile
against the manifest or stage receipt and submit a fresh review. The journal
does not store credentials or full model content, and requires restrictive
permissions on Unix. If a locked-down environment rejects the normal user state
directory, the TUI uses a stable private temporary directory keyed to the user home.
`FEAM_TUI_JOURNAL_DIR` can select a durable external private directory. Temporary
state can be removed by the OS; choose persistent storage for durable recovery.
Startup and `:recover` inspect unresolved intents and current manifests/receipts
without replaying writes. Records are limited to 1 MiB on write and read; an oversized intent fails before
mutation. Completed retention is 100 records; unresolved records
are retained, with a 1,000-record read limit requiring local maintenance.

## Agent profile and disclosure policy

Profiles live outside projects, by default in
`$XDG_CONFIG_HOME/feam/agent.toml` or `~/.config/feam/agent.toml`. Schema
version 1 requires a named `router` profile with HTTPS endpoint (loopback HTTP
is development-only), explicit model, API-key environment variable, context
policy, 1–8 tool calls, request/context/output bounds, and optional cost
envelope. Redirects are disabled. API keys are read only from the named
environment variable and are excluded from UI output, journal, model context,
and logs.

Example, with placeholders rather than a usable credential:

```toml
schema_version = 1
default_profile = "demo-router"

[profiles.demo-router]
backend = "router"
base_url = "https://openrouter.ai/api/v1"
model = "<selected-tool-capable-model>"
api_key_env = "FEAM_ROUTER_API_KEY"
context_policy = "synthetic-demo"
allow_user_text = true
max_tool_calls = 8
max_request_seconds = 120
max_context_chars = 48000
max_response_chars = 16000
max_request_bytes = 131072
max_response_bytes = 131072
max_output_tokens = 1024
max_input_price = 0.14
max_output_price = 0.42
reasoning_enabled = false
max_cost_usd = 1.00
allowed_providers = ["<selected-provider>"]
allow_provider_fallbacks = false
```

Both `metadata-only` and `synthetic-demo` enforce the same structural allowlist;
`synthetic-demo` never bypasses filtering. The profile's `allow_user_text` gate
is enforced before a hosted request. `allowed_metadata_fields` explicitly opts
into selected free-text metadata fields. User text, metadata, tool results,
history and errors pass through centralized outbound filtering; local paths,
credential-like tokens and the actual router key are removed. Structural summaries
retain qualified identities, versions, asset IDs/roles/sizes and outcomes. Tool
results, metadata, errors, and user text are separate channels. Context from a
project is not sent merely because a model asks for it.

## `feam.agent.tools.v1`

All model calls use the versioned `feam.agent.tools.v1` schemas. The model
cannot provide a project root, approval flag, arbitrary filesystem path, or
shell command. Stage-1 tool names are:

- Read-only: `catalog.search`, `product.inspect`, `product.resolve`,
  `peers.status`, `catalog.refresh`, `help.lookup`, and `user.clarify`.
- Review-only mutation proposals: `publication.validate`,
  `publication.publish`, `product.stage`, and `product.withdraw`.

These are internal protocol names. The router adapter maps `.` to `__` for
provider function names (for example, `help__lookup`), rejects ambiguous aliases,
and maps only advertised names back into the harness. Assistant tool-call
history uses the router's nested `function` object, JSON-string arguments, and
the original provider call ID so subsequent tool results remain correlated.

The provider can suggest a tool call only. Complete batches of at most eight
calls are validated before dispatch and executed serially. The harness rejects unknown fields/tools/fabricated handles, enforces a
maximum of eight calls, one argument repair, request/output/cost limits, and
stops repeated calls that make no progress. It does not execute partial SSE
arguments. Mutation calls only open a local review; confirmation never appears
in provider JSON, does not survive project/session changes, and cannot replay a
write after a reconnect.

OpenRouter streaming uses `stream: true`, tool schemas on each request,
explicit provider restrictions, `require_parameters: true`, price caps when configured,
and accounting frames. It ignores SSE comments, buffers split UTF-8/frames,
waits for complete JSON tool arguments, and treats stream error events as
errors. The current adapter behavior follows OpenRouter's [tool-calling guide](https://openrouter.ai/docs/guides/features/tool-calling), [streaming guide](https://openrouter.ai/docs/api_reference/streaming), and [provider-routing guide](https://openrouter.ai/docs/guides/routing/provider-selection).

## Explicit limits

The TUI serializes core calls through its controller and keeps terminal input
active while hosted work runs in a background worker. Stopping a model request
cancels generation/prevents new calls; an already-started synchronous peer
mutation is shown as finishing because the core service has no tested safe
cancellation point. The catalog remains manifest-backed. It loads and sorts bounded manifests:
8 MiB per manifest (including publication output), 32 MiB of catalog input,
256 KiB project config and at most 64 peer routes. Oversized input yields an
explicit error or unavailable coverage instead of silent truncation. Pages are
not streaming disk indexes. Local release measurements and their conditions
are in the acceptance record; they do not establish target-HPC performance.

There are no automatic transport retries. Missing usage leaves cost unknown and
stops further requests in a budgeted session. Known costs accumulate across
turns; price-based reservations protect each next request. These are conservative
application limits, not a router billing guarantee. `:usage` shows the most recent
100 run records, including known/unknown cost; `:export NAME` preserves the view.

Publication inspection requires a local I/O review. Full integrity resolution
requires a one-use `:integrity-consent REF VERSION`; provider arguments cannot
grant it. Publication binds validated scientific metadata, inventory and source
state; staging binds destination and receipt state. There remains an ordinary
filesystem race between final checks and external changes during I/O. No remote
filesystem transaction or exactly-once crash guarantee is claimed.
