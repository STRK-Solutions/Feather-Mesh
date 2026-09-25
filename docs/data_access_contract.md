# Peer data-access contract

Contract version: **1**. This document is the settled contract for the peer
data-access implementation. It implements the confirmed requirements in
[`data_access.md`](../data_access.md); older SQLite-only plans do not define
this interface.

## Scope and compatibility

`feam` remains the public executable name (the Cargo binary remains
`mesh_cli`). The existing SQLite commands remain available only with an
explicit `--registry PATH`. They neither publish into nor discover from peer
manifests. Every peer operation requires `--project ROOT`; it never defaults to
the current directory or creates `registry.db`.

The peer JSON protocol is `feam.peer.v1`. In `--format json` peer mode stdout
contains exactly one JSON value. Failures write one JSON object to stderr:

```json
{"protocol":"feam.peer.v1","error":{"kind":"dataset_not_registered","message":"..."}}
```

Unknown JSON fields are rejected in manifests and publication metadata so a
new writer cannot silently alter an older reader's interpretation. Readers
reject unsupported schema/protocol versions without modifying the project or a
cache. Existing legacy JSON output is unchanged.

## Project and route configuration

Project configuration is `.feam/project.toml`, TOML schema version 1:

```toml
schema_version = 1
namespace = "client"
serving_dir = "serving" # optional; relative to project root
owner_teams = ["Climate"] # provider teams authorized to publish

[[peers]]
alias = "local-climate"       # a local name only
namespace = "climate"         # expected provider namespace
path = "peers/climate"        # relative to the project root or absolute
```

`feam init --project ROOT --namespace NAME [--serving-dir PATH]` creates this
file and refuses to replace an existing file. A project can publish and consume.
Peer links and grants are administrative operations; their configuration must
name the expected provider namespace. Configuration paths are anchored to the
project root, not the process working directory. A configured peer root may be
outside the project. Two configured peers may not advertise the same namespace.

The derived cache is advisory only. It is stored below the user-selected
`FEAM_CACHE_DIR`, or a per-process directory under the OS temporary directory;
it is never a shared project SQLite/WAL database. `refresh` writes whole JSON
snapshots atomically and records refresh time/revision/error. New direct access
always reopens the configured peer route and its manifest; a cache can only
describe a last-known, explicitly stale discovery result. Removing or retargeting
a peer route makes new access fail even if an old physical path remains readable.

## Identity, lifecycle, and manifest

A product identity is `product://<namespace>/<dataset-id>`. Direct reads and
staging require a version. A provider writes one authoritative,
`serving_dir/manifest.json` with `schema_version: 1`, a provider `namespace`, a
monotonically increasing `revision`, and products/immutable version records.
Asset paths are serving-root-relative. A manifest is the publication authority;
SQLite, cache, and STAC are derived views and must never be edited as a way to
publish a dataset.

Each version has lifecycle `active` or `withdrawn`. Withdrawal appends a
tombstone with reason and time; it never deletes provider bytes or provenance.
Published version metadata and inventory are immutable. Repeating a publication
with the same qualified product/version is a `publication_conflict`, including
after an uncertain client-side response; clients should reopen the manifest and
compare the record rather than write a duplicate.

Publication serializes writers through a same-directory create-new lock file.
While holding it the service reloads and validates the current revision, writes
and syncs a temporary manifest in the manifest directory, and atomically renames
it into place. The lock has a 30-second timeout and stale-lock recovery is an
explicit operator action. Failure before rename leaves the old manifest visible.
If a derived projection fails after rename, publication remains committed and
the response reports a rebuildable projection failure.

## Publication metadata and inspectors

The common service used by `serve` and `validate-metadata` requires:

- namespace, stable `product_id`, display `name`, and `version`;
- description, intended use, limitations (or the literal `none`), owner team,
  producer, contact, usage policy, classification, and quality tier;
- an explicit typed asset inventory with IDs, roles, relative paths, and media
  types; every path must be a regular file below the serving root;
- publication timestamp, and `lineage`, which may be an explicit empty list;
- table column meanings/units (or an explicit `unknown`/`not_applicable`) and
  declared partition columns; or raster observation time, spatial extent, and
  per-asset scientific semantics.

`data_kind` (`table` or `raster`) and `data_format` (`parquet` or `geotiff`)
are distinct from the legacy `asset_type`. Publication records a byte size and
SHA-256 digest for each asset unless the producer explicitly records
`digest_opt_out`; membership pinning does not assert immutable bytes. Resolve
can request digest verification; normal resolves do not read whole files.

Tables are Parquet only. The service opens every declared footer with the Apache
Parquet Rust reader, records its physical schema and row count, and rejects
incompatible shard schemas or undeclared partition/physical column conflicts.
Footer validation does not validate every data page. Rasters are GeoTIFF only.
The Rust TIFF decoder must open every asset and the service requires GeoTIFF
georeferencing tags plus producer-supplied scientific time/extent; an extension
alone is never accepted. If an inspector is not available or rejects a file,
publication fails with `unsupported_format` and no manifest change.

## Filesystem and resolution rules

The resolver returns only assets named in the pinned active manifest record:
namespace/product/version, manifest revision, kind/format, inventory IDs,
project-route paths, role/media type/size/digest, descriptors, metadata,
lineage, refresh information, and integrity result. It never scans a directory,
expands a glob, or returns a provider-absolute fallback path.

Every configured route and registered asset is checked at access time. Absolute
and traversal paths, broken links, link cycles, non-files, and nested links
escaping the canonical serving root are rejected. Canonical paths prove route
safety but returned paths preserve the configured client route. A readable but
unregistered requested path is `dataset_not_registered`, distinct from an
`asset_unavailable`, stale cache, or `peer_unavailable` error.

Staging (`consume --project`) copies the resolved inventory only. Before writing,
it rejects source/output equality, hard-link or symlink aliases,
ancestor/descendant overlap, dangling output links, and receipt collisions. It
copies to a sibling temporary location, writes a receipt, then atomically swaps
the new output into place. Existing output is retained until all data and the
receipt are ready. A receipt-write failure removes the temporary data and returns
failure; no complete-success response is emitted. Receipts record identity,
version, revision, assets, digests, and timestamp.

## Errors and exits

Core errors have stable machine kinds. The CLI maps them without leaking this
mapping into `mesh_core`: validation/metadata/format errors → 3; missing product,
version, or registered inventory → 4; peer route, lifecycle, permission, and
destination-policy errors → 5; I/O, lock timeout, malformed remote manifest,
and projection failures → 1. Clap argument errors remain 2. New kinds include
`bad_metadata`, `unsupported_format`, `dataset_not_registered`,
`peer_unavailable`, `stale_catalog`, `withdrawn_version`, `integrity_failed`,
and `publication_conflict`.

## Python and STAC adapters

The supported SDK invokes an installed `feam` executable using an argument array
with `--project` and `--format json`; `FEAM_EXECUTABLE` or `Project.open(...,
executable=...)` selects it. It checks `feam.peer.v1`, maps error kinds to typed
exceptions, and never rescans peers. `scan_table` calls
`polars.scan_parquet(paths, glob=False)` and returns the native `LazyFrame`.
Its paths can fail later when `.collect()` runs because peer links, bytes, or
permissions can change after construction.

The STAC projection is derived from active raster manifest records. It pins STAC
core **1.1.0** and STAC API **1.0.0** conformance URIs: Core, Collections,
OGC API Features/GeoJSON, and Item Search. A namespace-qualified product is a
Collection; version-distinguishing granules are Items; data/mask/quality files
are named Assets. Items include GeoJSON geometry/bbox, RFC 3339 observation time,
projection/raster properties where known, and `feam:` ownership/version/
provenance properties. Projection remains native CRS when already WGS84; other
CRS transforms are rejected until a deterministic transformer is introduced.

`feam stac serve --project ROOT [--addr 127.0.0.1:PORT]` is an unauthenticated,
metadata-only service. The core rejects every bind address except the exact IPv4
loopback address `127.0.0.1`; the port remains configurable and defaults to 8080.
The intended deployment trusts processes able to connect on the same HPC node;
there is no Feather Mesh token, login, or per-user STAC authorization. Filesystem
permissions still govern opening the returned raster assets. The service exposes
landing, conformance, collections, collection/items, Item Search and deterministic,
revision-bound pagination. A changed peer/revision causes an old cursor to fail
with restart-required rather than leak a withdrawn record. Responses contain
encoded local `file:` URIs preserving client routes plus resolvable identity;
raster bytes are never proxied. The client and service must share the filesystem
view.

This is an intentional CLI break from the earlier authenticated prototype.
`--token-file` is removed and rejected as an unknown argument; clients must stop
sending bearer credentials. Deployments needing access from another node require
a separately approved transport design and must not weaken the loopback invariant.

## Validation environment

Rust builds use the pure-Rust `parquet 60.0.0` and `tiff 0.10.3` inspectors.
Fixture generation/integration requires Python 3.11+, `pyarrow`, `rasterio`,
`polars`, and `pystac-client`; see `mesh_core/tests/data/peer_access/README.md`
and `python_sdk/README.md`. Target-HPC validation additionally requires the
actual shared filesystem, separate producer/consumer identities, and two nodes;
local tests cannot prove that deployment evidence.
