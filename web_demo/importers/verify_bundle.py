#!/usr/bin/env python3
"""Read-only native-reader verification of a private, FEAM-registered candidate.

Creates only an isolated temporary consumer. It neither promotes nor grants a
release. Run with the supported installed SDK and exact-loopback socket access.
"""
import argparse
import csv
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import platform
import socket
import subprocess
import tempfile
import time
import urllib.request
import zipfile

import feam
import polars as pl
from pystac_client import Client
import rasterio
from rasterio.warp import transform
from rasterio.windows import Window


def digest(path):
    sha = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            sha.update(chunk)
    return sha.hexdigest()


@contextmanager
def raster_source(path, source):
    if source["importer"] != "aafc-calms-zip":
        yield path
        return
    with tempfile.TemporaryDirectory(prefix="feam-calms-verification-") as temporary:
        target = Path(temporary) / "source.tif"
        archive = source["archive"]
        with zipfile.ZipFile(path) as package, package.open(archive["primary_member"]) as incoming, target.open("xb") as output:
            for block in iter(lambda: incoming.read(1024 * 1024), b""):
                output.write(block)
        yield target


def verify(provider, raw, job, executable):
    results = {"protocol": "feam.pipeline.verification.v1", "environment": platform.system(), "status": "private-candidate-readers-verified",
               "verified_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
               "promoted": False, "assigned": False, "namespace": job["namespace"], "sources": [], "products": []}
    for source in job["sources"]:
        path = raw / source["id"]
        assert digest(path) == source["sha256"]
        results["sources"].append({"id": source["id"], "url": source["url"], "sha256": source["sha256"], "bytes": path.stat().st_size,
                                   "retrieved_at": source["retrieved_at"], "license": source["license"], "attribution": source["attribution"]})
    with tempfile.TemporaryDirectory(prefix="feam-candidate-consumer-") as temporary:
        client = Path(temporary) / "consumer with spaces"
        subprocess.run([executable, "--project", str(client), "--format", "json", "init", "--namespace", "candidate-check"], check=True, capture_output=True)
        (client / "peers").mkdir()
        (client / "peers" / "approved").symlink_to(provider / "serving", target_is_directory=True)
        # These are administrative peer routes in a new disposable client only.
        (client / ".feam/project.toml").write_text(f'schema_version = 1\nnamespace = "candidate-check"\n[[peers]]\nalias = "approved"\nnamespace = "{job["namespace"]}"\npath = "peers/approved"\n')
        subprocess.run([executable, "--project", str(client), "--format", "json", "refresh"], check=True, capture_output=True)
        project = feam.Project.open(client, executable=executable)
        for product in job["products"]:
            source = next(s for s in job["sources"] if s["id"] == product["source_id"])
            reference = f'product://{job["namespace"]}/{product["id"]}'
            result = {"product": reference, "version": product["version"]}
            asset = project.resolve_asset(reference, version=product["version"], asset=product["asset_id"])
            result.update(output_sha256=digest(asset.path), output_bytes=asset.path.stat().st_size)
            if source["importer"] == "eccc-csv":
                lazy = project.scan_table(reference, version=product["version"])
                assert isinstance(lazy, pl.LazyFrame)
                frame = lazy.collect()
                with (raw / source["id"]).open(encoding="utf-8-sig", newline="") as stream:
                    rows = list(csv.DictReader(stream))
                assert frame.height == len(rows) == 365
                actual = lazy.filter(pl.col("date") == pl.date(2023, 1, 1)).select("maximum_temperature", "minimum_temperature", "mean_temperature", "total_precipitation").collect().row(0)
                expected = tuple(float(rows[0][key]) for key in ("Max Temp (°C)", "Min Temp (°C)", "Mean Temp (°C)", "Total Precip (mm)"))
                assert actual == expected
                assert frame["total_precipitation_flag"][-1] == rows[-1]["Total Precip Flag"] == "T"
                result.update(rows=frame.height, known_date="2023-01-01", known_values=list(actual), last_precipitation_flag="T", native_lazy_frame=True)
            else:
                with raster_source(raw / source["id"], source) as original_path, rasterio.open(asset.path) as out, rasterio.open(original_path) as original:
                    assert out.crs.to_epsg() == 4326
                    assert out.nodata == original.nodata
                    window = out.read(1, window=Window(255, 255, 2, 2))
                    for row in (255, 256):
                        for column in (255, 256):
                            lon, lat = out.xy(row, column)
                            x, y = transform(out.crs, original.crs, [lon], [lat])
                            i, k = original.index(x[0], y[0])
                            expected = original.read(1, window=Window(k, i, 1, 1))[0, 0]
                            domain = source.get("value_domain")
                            if domain and not any(value_range["min"] <= expected <= value_range["max"] for value_range in domain["ranges"]):
                                expected = original.nodata
                            assert expected == window[row - 255, column - 255]
                    result.update(shape=list(out.shape), crs=out.crs.to_string(), source_crs=original.crs.to_string(), nodata=out.nodata, known_window=window.tolist(), source_pixel_comparison=True)
            results["products"].append(result)
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        process = subprocess.Popen([executable, "--project", str(client), "stac", "serve", "--addr", f"127.0.0.1:{port}"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            for _ in range(50):
                try:
                    with urllib.request.urlopen(f"http://127.0.0.1:{port}/", timeout=.25):
                        break
                except OSError:
                    time.sleep(.1)
            else:
                raise RuntimeError("private STAC readiness timeout")
            catalog = Client.open(f"http://127.0.0.1:{port}")
            found = list(catalog.search(method="GET", datetime="1900-01-01T00:00:00Z/2100-01-01T00:00:00Z", limit=100).items())
            expected = {(product["id"], product["version"]) for product in job["products"]}
            observed = {(item.properties["feam:product_id"], item.properties["feam:version"]) for item in found}
            assert expected.issubset(observed)
            results["stac"] = {"client": "pystac-client", "loopback": "127.0.0.1", "authentication": "none", "matched_items": len(found),
                               "expected_products": sorted(f"{product}/{version}" for product, version in expected)}
        finally:
            process.terminate()
            process.wait(timeout=5)
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("provider", "raw", "job", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--executable", required=True)
    args = parser.parse_args()
    job = json.loads(args.job.read_bytes())
    report = verify(args.provider, args.raw, job, args.executable)
    args.output.write_text(json.dumps(report, sort_keys=True, indent=2) + "\n")
    print("PASS: private native-reader and STAC interval verification; promotion and host acceptance remain separate")


if __name__ == "__main__":
    main()
