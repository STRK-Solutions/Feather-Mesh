# Archive metadata across compute recreation

The collector's archive ledger survives disposable compute through the
encrypted operator vault. It contains pseudonymous object metadata and review
split assignments; it contains no event bodies, account mapping, capabilities
or model credentials. Keep this ledger private. Do not reset the archive
budget by initializing a new empty collector database against an existing
archive.

Migration 2 adds durable archive byte reservations and original batch expiry
dates. Existing version1 databases require explicit `-mode migrate`; ordinary
startup never migrates them. The old binary refuses the new schema. Keep the
original database when migrating and use the reviewed rollback procedure.

For the first deployment, initialize the database and then run:

```bash
collector -config PRIVATE_CONFIG -mode initialize-archive-ledger
```

This checks the existing local metadata against fresh archive GET/hash/absence
results and a complete bounded `research/` listing. On an empty local database,
the archive must be empty. Missing archive access, extra objects, a changed
hash or listing/page bound stops initialization. It records the verified
baseline once. `serve` refuses a database without this baseline or an explicit
import. For an established version1 deployment, stop capture, migrate, flush
pending records and use the same mode to verify its existing metadata before
starting the new binary.

Before removing an old deployment, drain capture, flush pending archive and
deletion work, and export both proofs under the collector service identity:

```bash
umask 077
collector -config PRIVATE_CONFIG -mode archive-verify --include-inventory > archive-proof.json
collector -config PRIVATE_CONFIG -mode export-archive-ledger > archive-ledger.json
sha256sum archive-ledger.json
```

Seal the exact private files in the operator vault and verify a private Mac
copy of every retained object before deleting Ubuntu state. The lifecycle
operator orchestrator performs this capture; the shell
example shows the equivalent explicit steps. Each operation has a 60-second
deadline. The ledger is at most 1 MiB; a larger history fails closed and
requires a separately reviewed bounded archival/index procedure. Keep the
source host until all gates succeed. The prefix must have one active writer;
stop/revoke the old writer before moving authority to another host.

The `feam.archive-ledger.v1` JSON has `exported_at`, `objects`, `deletions`,
`splits` and `sha256`. Each object retains its ID, kind, participant pseudonym,
object key, SHA256, state, bytes, creation/expiry timestamps and reviewer.
Deletion entries retain the pseudonym, reason, creation time and archive
state. Split entries retain dimension, value and train/held-out assignment.
The internal digest covers compact Go JSON with the `sha256` field omitted;
the import CLI additionally requires SHA256 of the exact file bytes, including
its trailing newline. The operator vault binds that file hash; it should not
re-serialize the ledger before importing it.

On a fresh deployment, initialize the collector databases, restore only the
private metadata file, then import it before starting capture:

```bash
collector -config NEW_PRIVATE_CONFIG -mode import-archive-ledger \
  --ledger RESTORED_PRIVATE_LEDGER --confirm-ledger-sha256 EXACT_FILE_SHA256
collector -config NEW_PRIVATE_CONFIG -mode serve
```

Import requires an untouched initialized database. It verifies the canonical
ledger hash, every current archive object's actual bytes and SHA256, expected
absence of deleted objects, the full prefix listing, and the configured
archive cap including tombstones. Only then does one SQLite transaction
import metadata and its baseline receipt. Failed import leaves no partial
baseline. It never overwrites a nonempty database and never restores event
contents. If the operation's completion is uncertain, inspect the baseline
and run archive verification; do not discard the database or replay it into
another new database while capture is active.

Carried objects count against new upload reservations until verified deletion.
Withdrawal deletes both old and new participant objects. Retention uses the
original deadlines; recreation does not extend them. An object already past
its recorded deadline may be absent after verified local expiry or an R2
lifecycle rule. Unexpired absence fails verification. Deletion tombstones also
expire after 30 days, while the minimal encrypted withdrawal floor remains
in the operator ledger to prevent capture from being resurrected. Repeat
withdrawal never re-uploads an expired tombstone. Train/held-out assignments
also carry forward, preventing recreation from changing previous splits.

The local fixture tests cover recreation at a full budget, held-out split
rejection, withdrawal and retention of imported objects, expired tombstones,
tampered or incomplete metadata, unknown archive keys, a missing object,
an already populated destination and interrupted verification. Signed R2
pagination is tested against a local HTTP transport fixture. Actual R2
credentials, retrieval after host deletion and cohort content remain separate
deployment evidence.
