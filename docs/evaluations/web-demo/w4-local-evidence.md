# W4 private pipeline and grant evidence

Measured 2026-09-25 on Mac. This record does not establish Ubuntu storage,
separate-identity ACLs, image mounts, public Access or cloud acceptance.

Implemented local boundaries:

- Closed, duplicate-key-rejecting source jobs pin official HTTPS objects, SHA-256,
  scientific subset/time, byte/time/redirect/archive bounds and audience policy.
  Fetch validates every DNS answer and redirect, forbids private/reserved addresses,
  disables environment proxies and never exposes partial downloads.
- Fixed offline ECCC/AAFC converters produce only Parquet and GeoTIFF. They preserve
  daily nulls/quality flags and explicit raster nodata, source/target transforms,
  CRS, units, resampling and source/output hashes. Climatology keeps its interval.
- A dedicated SQLite manager reserves complete candidates, serializes bundle
  writers, preserves parent snapshots, invokes FEAM with explicit project roots,
  requires exact-hash approval and atomically promotes complete releases. Unknown
  publication and rename outcomes reconcile without replay. Integrity and current
  withdrawal floors gate every release resolution.
- Unix release resolution authenticates controller/gateway kernel UIDs. Admin grant
  forms validate exact releases, CAS grant/auth versions, pause admission and close
  streams. A durable outbox only completes after controller reconfiguration success;
  stale acknowledgments cannot erase newer changes. Admin stop/reset/delete remain
  separate from participant terminal access and retain destructive confirmation.
- The image's fixed release attachment helper preserves practice peers, rejects
  conflicting namespaces, unsafe paths/symlinks and ambiguous managed sections,
  removes revoked routes and refreshes using FEAM. Release promotion uses optional
  fixed Linux ACLs: mapped UID 200999 can read only the serving tree; source,
  provenance and project internals remain owner-only. Linux ACL execution is a
  separate deployed check, not proved by Mac ACL-plan tests.

Validation completed:

| Surface | Observed result |
| --- | --- |
| Rust | `cargo test -p mesh_core` passed; CLI build/test passed; `cargo fmt -- --check`, `cargo clippy -- -D warnings` and default workspace `cargo test` passed after interval changes. Broker all-feature checks are tracked separately. |
| Installed SDK/STAC | Fresh installed SDK outside source tree; `FEAM_E2E=1 … python -m pytest python_sdk/tests -q`: 5 passed. Includes native Polars, pinned STAC schema, standard tokenless exact-loopback pagination, interval search/extents, non-loopback rejection and known Rasterio window. |
| Actual real pair | `verify_bundle.py` passed using the fresh SDK environment and the previously approved private source files; rerun after the platform label fix also passed. See linked source-reader receipt and candidate review. |
| Importers | 6 native Python tests passed, covering nulls/flags/completeness, corrupt/renamed raster inputs, missing georeference/nodata, actual reprojection and truthful climatology intervals. Rasterio reports Affine pending-deprecation warnings. |
| Pipeline | `go test -race -tags=pipeline_integration ./internal/pipeline` passed: real FEAM publication/successor integrity, approval mismatch, immutable release, stale/withdrawn IPC, post-approval mutation, unknown commit and orphan promotion reconciliation. Unit tests cover closed bounds, source/address policy, partial/checksum errors, reservation/writer races, symlinks and fail-closed startup. Additional real CLI tests reject missing metadata, namespace conflicts, corrupt/renamed Parquet and incompatible shards; a failing cache projection leaves the committed manifest and direct resolver intact. Missing Python native packages fail actual conversion. Durable queue, withdrawal and authoritative retention tests also passed. |
| Grants/status | `go test -race ./internal/control ./internal/gateway ./internal/pipeline` and `go vet` for those packages plus `cmd/pipeline` passed. Tests include stale snapshots/forms/acks, disabled accounts, failed release resolution, stream closure, failed controller dispatch then restart retry, revocation without source availability, admin lifecycle/terminal boundaries and stale status suppression. |
| Image attachment | 3 attachment tests and 3 existing hosted-image tests passed. Image build, real mount identities and sandbox native reads remain distinct. |

Retained failed attempts and limits:

- Default sandbox denied Unix/loopback test binds; authorized local-socket reruns
  passed. A prior default GOPATH/cache write failure was resolved with explicit
  private tool/cache paths. No network data fetch was used for those tests.
- Initial tiny Mac fixture admission failed the production free-space threshold.
  Fixture-only configuration disables that host threshold; the production command
  retains its 15%/20 GiB guard, and the real Mac manager correctly refused admission.
  Bounded real conversion/FEAM reads were run separately and remain private proof.
- macOS refused renaming a sealed directory root; the implementation seals all
  children first and seals the root immediately after atomic rename. Reconciliation
  covers a crash between rename and durable release registration.
- Production retention currently reserves retained bytes and refuses exhausted
  budgets; no automatic pruning of potentially assigned releases is implemented.
  Explicit admin pruning now requires exact retained-release review and live
  authoritative pins; current/revoked/withdrawal-protected/uncertain releases are
  refused. Partial deletion stays reserved for operator inspection.
- The dashboard queue and separate source/candidate review steps are implemented.
  Five-minute review tokens reject changed jobs or another administrator. Real
  FEAM withdrawal tests passed: complete parent copy, tombstones, monotonic floor,
  preserved unaffected versions, later versions retaining tombstones and refused
  republishing of an already withdrawn version. Bundle grant revocation is durable
  and idempotent; old mounts remain retention pins until acknowledgment.
- Native `setfacl`/`getfacl` execution, source preparation under UID 2106, filesystem
  headroom, audience mounts, denied B shell paths, write/symlink denial, recreation
  and Ubuntu SDK/STAC reads must be recorded from the deployed Ubuntu run.

The [source proposal](w4-source-proposal.md), [MAC native-reader receipt](w4-mac-real-source-readers.json)
and [private candidate review](w4-mac-candidate-review.json) retain the exact source,
output and candidate hashes. Acquisition approval does not approve promotion.

Final integration additions: `go test -race -tags=pipeline_integration
./internal/pipeline ./internal/control ./internal/gateway` and `go vet` for these
packages plus gateway/pipeline commands passed. The pipeline now has 18 Mac
integration/unit test functions, plus a separate Linux ACL execution test. The
final owner-only retention notice/status/review tests passed; live broker summary
shows unavailable explicitly and distinguishes recording configuration from actual
collector delivery. The approved real source job's canonical hash remains
`5b34ed3fa00c5ccf8d8d6e822ee3ca5c2f1d62fa825dc6bf8cbae96c83d39a10` after
adding the optional withdrawal schema. An initial missing-package test used
`--help`, which correctly avoided lazy imports; it was corrected to invoke actual
conversion and then passed. Default system Python lacks `jsonschema`; schema
checks use the fresh locked reader environment instead.
