# Feather Mesh Python SDK

The SDK is a small supported adapter over the installed `feam` executable. It
does not discover peers or scan directories itself: Rust validates the project,
peer route, manifest revision, lifecycle, and exact inventory before Python
receives paths.

Install from a clean environment, outside this source directory:

```bash
python3 -m venv /tmp/feam-sdk-venv
/tmp/feam-sdk-venv/bin/python -m pip install ./python_sdk[table,raster,test]
FEAM_EXECUTABLE="$(pwd)/target/debug/mesh_cli" \
  /tmp/feam-sdk-venv/bin/python -m pytest python_sdk/tests
```

`FEAM_EXECUTABLE` may instead point to an installed `feam`; callers can also
pass `executable=` to `Project.open`. The Cargo artifact remains `mesh_cli`.

```python
from feam import Project

project = Project.open("/work/client-project")
table = project.resolve_table("product://climate/observations", version="v3")
query = project.scan_table("product://climate/observations", version="v3")
result = query.filter(...).select(...).collect()
```

`query` is a native `polars.LazyFrame` built with `glob=False` from the pinned
registered paths. It is intentionally lazy: a removed peer link, changed bytes,
or permission change after construction can surface during `collect()`. Raster
callers use `resolve_asset(...)` and pass its local path to Rasterio.

The protocol is `feam.peer.v1`. Success is one stdout JSON object; failure is a
structured stderr object. The SDK rejects missing executables, malformed output,
and incompatible protocols, and maps Rust error kinds to typed exceptions.
