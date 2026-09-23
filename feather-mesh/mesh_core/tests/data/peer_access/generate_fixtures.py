"""Generate deterministic, disposable peer-access test assets.

Run only in a fixture checkout: it intentionally replaces the files below this
directory so stale binary fixtures cannot mask changed expectations.
"""

from pathlib import Path

import pyarrow as pa
import pyarrow.parquet as pq
import numpy as np
import rasterio
from rasterio.transform import from_bounds


ROOT = Path(__file__).resolve().parent


def write_table(path: Path, station_ids: list[int]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    table = pa.table({"station_id": station_ids, "temperature": [value + 10 for value in station_ids]})
    pq.write_table(table, path, compression="none")


def main() -> None:
    write_table(ROOT / "table/part-000.parquet", [1, 2])
    write_table(ROOT / "table/part-001.parquet", [3, 4])
    write_table(ROOT / "table/unregistered.parquet", [99])
    invalid = ROOT / "invalid"
    invalid.mkdir(parents=True, exist_ok=True)
    pq.write_table(pa.table({"station_id": [5], "humidity": [80]}), invalid / "incompatible.parquet")
    (invalid / "renamed.csv.parquet").write_text("station_id,temperature\n1,11\n", encoding="utf-8")
    (invalid / "corrupt.parquet").write_bytes(b"PAR1not-a-valid-footer")
    raster = ROOT / "raster/temperature.tiff"
    raster.parent.mkdir(parents=True, exist_ok=True)
    with rasterio.open(
        raster,
        "w",
        driver="GTiff",
        width=2,
        height=2,
        count=1,
        dtype="uint8",
        crs="EPSG:4326",
        transform=from_bounds(-76, 45, -75, 46, 2, 2),
        nodata=255,
    ) as dataset:
        dataset.write(np.array([[7, 8], [9, 10]], dtype="uint8"), 1)
    with rasterio.open(invalid / "plain.tiff", "w", driver="GTiff", width=2, height=2, count=1, dtype="uint8") as dataset:
        dataset.write(np.zeros((2, 2), dtype="uint8"), 1)


if __name__ == "__main__":
    main()
