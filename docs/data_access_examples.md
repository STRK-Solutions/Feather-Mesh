# Peer data-access examples

All data access is anchored to an explicit project root and a pinned version.
The examples assume a shared filesystem peer link has been configured
administratively in `.feam/project.toml`.

## Batch table job

```python
from feam import Project
import polars as pl

project = Project.open("/work/client-project", executable="/opt/feam/bin/feam")
query = (
    project.scan_table("product://climate/observations", version="v3")
    .filter(pl.col("station_id") == "OTTAWA")
    .select("date", "temperature")
)
query.sink_parquet("/scratch/job-123/ottawa.parquet")
```

`scan_table` returns a native lazy `polars.LazyFrame` over an exact registered
Parquet list; it uses no glob. Laziness reduces unnecessary input reads but does
not bound output memory. Use a Parquet sink, streaming-capable operations, and
the scheduler allocation appropriate to the query. A peer link, permission, or
file can change after query construction, so `.collect()`/`sink_parquet()` can
still fail at execution time.

## Notebook raster discovery and windowed read

Start metadata discovery in the same filesystem context as the notebook. The
token is a per-instance secret stored outside shared project files.

```bash
umask 077
printf '%s\n' "$FEAM_STAC_TOKEN" > /tmp/feam-stac-token
feam --project /work/client-project stac serve --token-file /tmp/feam-stac-token
```

```python
from feam import Project
import rasterio
from rasterio.windows import Window
from pystac_client import Client

client = Client.open(
    "http://127.0.0.1:8080",
    headers={"Authorization": f"Bearer {token}"},
)
item = next(client.search(
    collections=["climate--temperature"],
    bbox=[-76, 45, -75, 46],
    datetime="2026-01-01T00:00:00Z/2026-01-01T00:00:00Z",
).items())

project = Project.open("/work/client-project")
asset = project.resolve_asset("product://climate/temperature", version="v1", asset="data")
with rasterio.open(asset.path) as dataset:
    pixels = dataset.read(1, window=Window(0, 0, 512, 512))
```

STAC returns metadata and encoded local `file:` URIs; it does not proxy raster
bytes. Resolve the product/version/asset through Feather Mesh before opening it
with Rasterio, and record `item.id`, the resolved manifest revision, version,
asset digest (if supplied), project root, cache status, and job ID with outputs.

## Explicit staging

Direct reads are preferred for standard shared-filesystem jobs. Stage only when
a private snapshot or storage locality is genuinely required:

```bash
feam --project /work/client-project consume \
  product://climate/observations --version v3 --out /scratch/job-123/observations
```

The command copies exactly the registered inventory and commits its provenance
receipt with the output. It rejects source/output aliases and overlaps. Removing
a peer link stops new Feather Mesh resolutions but cannot revoke already-open
Rasterio handles or data already loaded into a process.
