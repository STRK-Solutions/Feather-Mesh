# U.03 collector local archive implementation evidence

Date: 2026-09-25. Scope: Mac source and synthetic tests only; no Ubuntu service,
host filesystem, private Mac copy, or real participant acceptance is claimed.

The collector now requires an explicit `archive_mode`. `local` accepts only
`/home/feam-service-data/traces/collector/archive` and no R2 fields; `r2`
retains the credential-backed adapter and refuses a local directory. Local
startup checks the private `0700` archive, collector ownership, and the same
filesystem as the event database. The root service validator binds that path
to the UUID-verified traces mount, and Ansible creates it after mount checks.
The existing archive ledger, readback, deletion lineage, expiry, reviewer UID
restrictions and byte quotas remain shared by the two adapters. A stopped
collector flushes and reconciles deletions, while periodic reconciliation and
expiry failures are logged as pending. The settings schema permits `local`
for a production review configuration; a fixture mode is rejected.

Validation run from this checkout:

- `go test -race ./...` in `web_demo/`: passed using pinned Go 1.27.1, with
  Unix-socket permission for integration tests.
- `go vet ./...` and `go build -trimpath ./cmd/...`: passed.
- `python infra/demo/scripts/check.py` under the locked W0 environment with
  `ansible-playbook` on `PATH`: passed, including schema tests, W7 service tests,
  Ansible syntax and lint. The first run under system Python lacked dependencies;
  the locked-environment rerun passed.
- Agent-context structural check and `git diff --check`: passed.

An additional cross-language smoke used a temporary Go program against the
actual collector library, SQLite migrations and directory adapter. It generated
a fresh archive verification receipt and archive ledger after one synthetic
event, then Python `archive_copy.copy_stream` and `verify_copy` copied and
read back the Go-written object. A real `Store.Withdraw` then produced one
absent batch and one verified deletion tombstone; Python copied the resulting
archive and `maintain_copy` removed the withdrawn bytes from the prior Mac
copy. Both retained copies verified. Fresh proof/ledger observation timestamps
were accepted when the content and lineage were unchanged. This was local
synthetic interoperability evidence, not a target Ubuntu or Mac transfer.

Remaining U.03 acceptance: install this release and private config on Ubuntu;
verify mount identity, service UID/ACLs and 0700 archive; capture one synthetic
event through the actual socket, flush and read it back; deny an unauthorized
reviewer; and verify the quiesced full inventory and ledger in owner-only Mac
storage before any destructive teardown. Same-host archive verification alone
does not establish host-loss survival.
