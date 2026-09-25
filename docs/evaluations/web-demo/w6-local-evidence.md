# W6 local structured-capture evidence — 2026-09-25

Environment: macOS ARM64, pinned Go 1.27.1, Rust stable workspace, synthetic
SQLite/filesystem/Unix/HTTP fixtures. No cohort events, provider request, R2
upload, paid resource or target Ubuntu process is represented by this record.

Implemented surfaces are documented in [COLLECTOR.md](../../../web_demo/COLLECTOR.md):
closed typed records and provenance, capability-scoped append endpoints,
kernel-UID trusted endpoints, pre-persistence redaction, transaction/dedup/gap
handling, bounded collection, index recovery, private R2 signing/readback,
pending uploads, withdrawal/deletion lineage, restricted reviewed export and
participant/dataset/task split separation. The policy uses the owner-confirmed
30-day retention, with private policy/identity mapping separate from the corpus.

Final local validation passed:

- `go test -race ./internal/collector ./cmd/collector ./cmd/capture-fixture` and
  matching `go vet`, including actual SQLite `max_page_count` exhaustion,
  duplicate/gap retries, redaction, corrupt readback, failed export upload
  recovery, restricted provenance, withdrawal recovery and a kernel-UID broker
  recording exchange over a Unix socket.
- `cargo fmt -- --check`, all-workspace/all-target/all-feature Clippy with
  warnings denied, and all-workspace/all-feature tests. After the final capture
  framing/feature fix, focused all-feature `mesh_tui` tests and CLI `tui` and
  `agent-hosted` tests passed; no-default CLI tests also passed.
- Seven actual PTY tests passed, including two new structured-capture fixtures
  for edits, denial, approval, typed local outcome, exact duplicate submission,
  no private entered text in events, collector gap status, paused mutation and
  restored terminal state on failure/exit.
- [Manual walkthrough](w6-mac-manual.json): eight offline checks passed.
  [Fake walkthrough](w6-mac-fake.json): nine offline checks passed. These retain
  actual FEAM staging/publication/withdrawal outcomes and external native
  Polars/Rasterio example execution. The fake walkthrough does not establish
  provider or deployed capture coverage.
- Both event records in the checked-in
  [synthetic reviewed export](../../../web_demo/testdata/synthetic-reviewed-export.json)
  validated against the closed event-record schema. Its verifier/reviewer and
  receipt are explicitly synthetic fixture data. They test export labeling and
  negative retention, not human review or a real independently observed result.

Failures encountered and corrected remain part of the evidence: the directory
archive correctly refused a nonprivate temporary directory; tests now create
explicit `0700` archive roots. A Unix transport test exposed EOF-sensitive ACK
handling; framing now requires a bounded exact `Content-Length` response.
The manual-only Rust feature build exposed a socket2 conversion supplied only
by transitive feature unification; conversion now uses standard `OwnedFd`.
Initial PTY assertions assumed no duplicate transport attempts and inspected
raw ANSI bytes; they now verify exact retries, render the screen and wait for
the visible durable-recording ready state before acting.

Still pending: actual Linux/Ubuntu service identities and ACLs, deployed
request-to-independent-receipt coverage including reset, real R2 retention and
retrieval after deleting disposable compute, and Saif's human-reviewed eligible
real-cohort export. W6.06/W6.G remain acceptance gates. No training was started.
