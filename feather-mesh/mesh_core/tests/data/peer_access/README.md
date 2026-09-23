# Peer-access fixtures

`generate_fixtures.py` creates the small binary test fixtures used by the
Python/STAC integration suite. It is deliberately deterministic:

```bash
cd feather-mesh
python3 -m venv /tmp/feam-fixtures-venv
/tmp/feam-fixtures-venv/bin/python -m pip install pyarrow rasterio numpy
/tmp/feam-fixtures-venv/bin/python mesh_core/tests/data/peer_access/generate_fixtures.py
```

Expected table values are `station_id` 1–4 and `temperature` 11–14 across the
two Parquet shards. `temperature.tiff` is a 2×2 EPSG:4326 GeoTIFF with pixels
`[[7, 8], [9, 10]]`, nodata `-9999`, and bounds `[-76, 45, -75, 46]`.

The generator also writes a renamed CSV, a corrupt Parquet footer, an
incompatible shard, and a plain TIFF without GeoTIFF tags. Traversal, broken
links, cycles, namespace mismatch, and source/output alias cases are created in
temporary test directories because committing those filesystem objects would be
non-portable. Rust tests independently generate equivalent valid Parquet and
GeoTIFF samples using the pinned `parquet 60.0.0` and `tiff 0.10.3` inspectors.
