#!/usr/bin/env python3
"""Fixed offline climate converters. No network, shell, arbitrary plugins or archives.

The Go pipeline validates and pins the job and invokes this file with an argument
array. Inputs/provenance stay private; only Parquet/GeoTIFF enter serving/.
"""
from __future__ import annotations

import argparse
import csv
from datetime import date, datetime
import hashlib
import json
import math
from pathlib import Path
import sys

VERSION = "1"
FIELDS = {
    "Max Temp (°C)": ("maximum_temperature", "degree_Celsius"),
    "Min Temp (°C)": ("minimum_temperature", "degree_Celsius"),
    "Mean Temp (°C)": ("mean_temperature", "degree_Celsius"),
    "Total Precip (mm)": ("total_precipitation", "mm"),
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def csv_to_parquet(source: Path, destination: Path, spec: dict) -> dict:
    import polars as pl

    start, end = date.fromisoformat(spec["start_date"]), date.fromisoformat(spec["end_date"])
    if not 0 <= (end - start).days <= 366:
        raise ValueError("date range exceeds one year")
    rows, seen = [], set()
    with source.open(encoding="utf-8-sig", newline="") as stream:
        reader = csv.DictReader(stream)
        required = {"Climate ID", "Date/Time", *FIELDS}
        required.update(column.replace(" (°C)", "").replace(" (mm)", "") + " Flag" for column in FIELDS)
        if not reader.fieldnames or len(set(reader.fieldnames)) != len(reader.fieldnames) or not required.issubset(reader.fieldnames):
            raise ValueError("unsupported ECCC CSV columns")
        for row in reader:
            if None in row or any(v is None for v in row.values()):
                raise ValueError("malformed CSV record")
            observation = date.fromisoformat(row["Date/Time"])
            if row["Climate ID"] != spec["station"] or not start <= observation <= end or observation in seen:
                raise ValueError("unexpected station/date or duplicate daily observation")
            seen.add(observation)
            record = {"station_id": row["Climate ID"], "date": observation, "data_quality": row.get("Data Quality")}
            for column, (name, _) in FIELDS.items():
                value = row[column]
                number = None if value == "" else float(value)
                if number is not None and not math.isfinite(number):
                    raise ValueError("nonfinite observation")
                record[name] = number
                flag = column.replace(" (°C)", "").replace(" (mm)", "") + " Flag"
                record[name + "_flag"] = row[flag]  # Preserve blanks and quality flags exactly.
            rows.append(record)
            if len(rows) > 367:
                raise ValueError("row bound exceeded")
    if not rows or len(rows) != (end - start).days + 1:
        raise ValueError("requested daily interval is incomplete; choose an explicit available interval")
    schema = {"station_id": pl.String, "date": pl.Date, "data_quality": pl.String}
    units = {"station_id": "not_applicable", "date": "ISO 8601 calendar date in source local time", "data_quality": "not_applicable"}
    meanings = {"station_id": "ECCC Climate ID exactly as provided", "date": "ECCC Date/Time local daily observation date", "data_quality": "Unmodified general ECCC Data Quality flag; null if source column absent"}
    for name, unit in FIELDS.values():
        schema[name], schema[name + "_flag"] = pl.Float64, pl.String
        units[name], units[name + "_flag"] = unit, "not_applicable"
        meanings[name], meanings[name + "_flag"] = name.replace("_", " "), "Unmodified ECCC quality flag for " + name
    frame = pl.DataFrame(rows, schema=schema).sort("date")
    frame.write_parquet(destination, compression="zstd", statistics=True)
    # Reopen with the same native lazy reader used by the SDK; verify physical rows.
    reopened = pl.scan_parquet([str(destination)], glob=False).collect()
    if not reopened.equals(frame):
        raise ValueError("Parquet round-trip mismatch")
    return {"data_kind": "table", "data_format": "parquet", "table": {"column_meanings": meanings, "column_units": units, "partition_columns": []},
            "conversion": {"row_count": len(rows), "date_start": start.isoformat(), "date_end": end.isoformat(), "station_id": spec["station"], "null_policy": "empty numeric cell -> null; quality flags unchanged", "units": units}}


def geotiff_to_wgs84(source: Path, destination: Path, spec: dict) -> dict:
    import numpy as np
    import rasterio
    from rasterio.enums import Resampling
    from rasterio.transform import from_bounds
    from rasterio.warp import reproject, transform_bounds

    bbox = spec["bbox"]
    if len(bbox) != 4 or not all(math.isfinite(v) for v in bbox) or not (-180 <= bbox[0] < bbox[2] <= 180 and -90 <= bbox[1] < bbox[3] <= 90):
        raise ValueError("invalid WGS84 subset")
    scientific = spec["scientific_time"]
    if scientific["kind"] not in {"instant", "climatology-interval"} or not scientific["evidence"]:
        raise ValueError("source scientific time evidence is required")
    start, end = datetime.fromisoformat(scientific["start"]), datetime.fromisoformat(scientific["end"])
    if not scientific["start"].endswith("Z") or not scientific["end"].endswith("Z") or end < start:
        raise ValueError("invalid scientific interval")
    if spec["resampling"] != "nearest":
        raise ValueError("unapproved resampling")
    with rasterio.open(source) as src:
        if src.driver != "GTiff" or not src.crs or src.transform.is_identity or src.count != 1 or src.nodata is None or not math.isfinite(src.nodata) or src.width * src.height > 32_000_000 or src.dtypes[0] not in {"uint8", "int16", "uint16", "int32", "uint32", "float32", "float64"}:
            raise ValueError("unsupported, unreferenced or oversized source GeoTIFF")
        if len(src.files) != 1 or Path(src.files[0]).resolve() != source.resolve():
            raise ValueError("external raster dependencies are forbidden")
        coverage = transform_bounds(src.crs, "EPSG:4326", *src.bounds, densify_pts=21)
        if bbox[0] < coverage[0] or bbox[1] < coverage[1] or bbox[2] > coverage[2] or bbox[3] > coverage[3]:
            raise ValueError("requested subset exceeds source coverage")
        # Bounded fixed target grid: at most 512x512 samples, nearest-neighbour.
        # Preserve the source's exact grid when already WGS84 with this extent.
        exact = src.crs.to_epsg() == 4326 and list(src.bounds) == bbox and src.width <= 512 and src.height <= 512
        width, height = (src.width, src.height) if exact else (512, 512)
        target_transform = src.transform if exact else from_bounds(*bbox, width=width, height=height)
        output = np.full((height, width), src.nodata, dtype=src.dtypes[0])
        reproject(source=rasterio.band(src, 1), destination=output, src_transform=src.transform, src_crs=src.crs,
                  src_nodata=src.nodata, dst_transform=target_transform, dst_crs="EPSG:4326", dst_nodata=src.nodata,
                  resampling=Resampling.nearest, num_threads=1, warp_mem_limit=32)
        with rasterio.open(destination, "w", driver="GTiff", width=width, height=height, count=1, dtype=output.dtype,
                           crs="EPSG:4326", transform=target_transform, nodata=src.nodata, compress="deflate") as dst:
            dst.write(output, 1)
            dst.update_tags(scientific_time_kind=scientific["kind"], scientific_start=scientific["start"], scientific_end=scientific["end"])
        provenance = {"source_crs": src.crs.to_string(), "source_transform": list(src.transform), "source_nodata": src.nodata,
                      "target_crs": "EPSG:4326", "target_transform": list(target_transform), "width": width, "height": height,
                      "resampling": "nearest", "scientific_time": scientific, "source_scale": src.scales[0], "source_offset": src.offsets[0]}
        # Applying a source scale/offset silently would change physical semantics.
        if src.scales != (1.0,) or src.offsets != (0.0,):
            raise ValueError("scaled source needs an explicitly reviewed converter")
    with rasterio.open(destination) as verified:
        if verified.crs.to_epsg() != 4326 or not np.array_equal(verified.read(1), output):
            raise ValueError("GeoTIFF round-trip mismatch")
    result = {"data_kind": "raster", "data_format": "geotiff", "conversion": provenance}
    result["raster"] = {"datetime": scientific["start"] if scientific["kind"] == "instant" else None, "bbox": bbox,
                        "semantics": {"band_1": spec["variable"], "units": spec["unit"], "scientific_time_evidence": scientific["evidence"], "time_kind": scientific["kind"]}}
    if scientific["kind"] != "instant":
        result["raster"].update(start_datetime=scientific["start"], end_datetime=scientific["end"])
    return result


def convert(job: dict, inputs: Path, output: Path, reports: Path) -> None:
    if job["protocol"] != "feam.pipeline.v1" or job["importer_version"] != VERSION:
        raise ValueError("unsupported converter job")
    sources = {source["id"]: source for source in job["sources"]}
    for product in job["products"]:
        spec = sources[product["source_id"]]
        raw = inputs / spec["id"]
        if raw.is_symlink() or not raw.is_file() or raw.stat().st_size > spec["max_bytes"] or sha256(raw) != spec["sha256"]:
            raise ValueError("unverified source")
        extension = ".parquet" if spec["importer"] == "eccc-csv" else ".tiff"
        relative = Path("datasets") / product["id"] / product["version"] / (product["asset_id"] + extension)
        target = output / relative
        target.parent.mkdir(parents=True, exist_ok=False)
        converter = {"eccc-csv": csv_to_parquet, "aafc-geotiff": geotiff_to_wgs84}[spec["importer"]]
        result = converter(raw, target, spec)
        if target.stat().st_size > job["limits"]["output_bytes"]:
            raise ValueError("prepared output exceeds bound")
        metadata = {"schema_version": 1, "namespace": job["namespace"], "product_id": product["id"], "name": product["name"],
                    "version": product["version"], "description": product["description"], "intended_use": product["intended_use"],
                    "limitations": product["limitations"], "owner_team": "Demo", "producer": product["producer"], "contact": product["contact"],
                    "usage_policy": spec["license"] + "; " + spec["attribution"], "classification": "public", "quality": "experimental", "lineage": [],
                    "data_kind": result["data_kind"], "data_format": result["data_format"],
                    "assets": [{"id": product["asset_id"], "path": relative.as_posix(), "role": "data", "media_type": "application/vnd.apache.parquet" if extension == ".parquet" else "image/tiff; application=geotiff"}]}
        for key in ("table", "raster"):
            if key in result:
                metadata[key] = result[key]
        provenance = {"protocol": "feam.pipeline.provenance.v1", "importer_version": VERSION, "source": spec,
                      "output_path": relative.as_posix(), "output_sha256": sha256(target), "output_bytes": target.stat().st_size,
                      "conversion": result["conversion"]}
        (reports / (product["id"] + ".provenance.json")).write_text(json.dumps(provenance, sort_keys=True, indent=2) + "\n")
        (reports / (product["id"] + ".metadata.json")).write_text(json.dumps(metadata, sort_keys=True, indent=2) + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--job", required=True, type=Path)
    parser.add_argument("--input-dir", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--reports-dir", required=True, type=Path)
    args = parser.parse_args()
    try:
        convert(json.loads(args.job.read_bytes()), args.input_dir, args.output_dir, args.reports_dir)
    except Exception as error:
        # No raw rows, paths, credentials or traceback at the subprocess boundary.
        print(json.dumps({"protocol": "feam.pipeline.v1", "error": "conversion_failed", "kind": type(error).__name__}), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
