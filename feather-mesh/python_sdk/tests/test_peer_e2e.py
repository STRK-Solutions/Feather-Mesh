"""Real CLI/SDK/Polars/Rasterio/STAC integration; enabled by FEAM_E2E=1 in CI."""

from __future__ import annotations

import json
import os
import socket
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path

import pytest

if os.environ.get("FEAM_E2E") != "1":
    pytest.skip("set FEAM_E2E=1 with pyarrow, polars, rasterio, and pystac-client installed", allow_module_level=True)

import numpy as np
import polars as pl
import pyarrow as pa
import pyarrow.parquet as pq
import rasterio
from jsonschema import validate as validate_json_schema
from pystac_client import Client
from rasterio.transform import from_bounds
from rasterio.windows import Window

from feam import Project


EXE = os.environ["FEAM_EXECUTABLE"]


def cli(*arguments: str, cwd: Path | None = None) -> dict:
    completed = subprocess.run([EXE, *arguments], cwd=cwd, check=True, text=True, capture_output=True)
    return json.loads(completed.stdout)


def metadata_table() -> dict:
    return {
        "schema_version": 1,
        "namespace": "climate",
        "product_id": "observations",
        "name": "Observations",
        "version": "v1",
        "data_kind": "table",
        "data_format": "parquet",
        "description": "known values",
        "intended_use": "test lazy access",
        "limitations": "none",
        "owner_team": "Climate",
        "producer": "Climate Lab",
        "contact": "climate@example.test",
        "usage_policy": "internal",
        "classification": "internal",
        "quality": "production",
        "assets": [
            {"id": "part-000", "path": "datasets/observations/v1/part-000.parquet", "role": "data", "media_type": "application/vnd.apache.parquet"},
            {"id": "part-001", "path": "datasets/observations/v1/part-001.parquet", "role": "data", "media_type": "application/vnd.apache.parquet"},
        ],
        "lineage": [],
        "table": {
            "column_meanings": {"station_id": "identifier", "temperature": "daily temperature"},
            "column_units": {"station_id": "not_applicable", "temperature": "celsius"},
            "partition_columns": [],
        },
    }


def metadata_raster(version: str, filename: str) -> dict:
    return {
        "schema_version": 1,
        "namespace": "climate",
        "product_id": "temperature",
        "name": "Temperature",
        "version": version,
        "data_kind": "raster",
        "data_format": "geotiff",
        "description": "known pixels",
        "intended_use": "test windows",
        "limitations": "none",
        "owner_team": "Climate",
        "producer": "Climate Lab",
        "contact": "climate@example.test",
        "usage_policy": "internal",
        "classification": "internal",
        "quality": "production",
        "assets": [{"id": "data", "path": f"datasets/temperature/{version}/{filename}", "role": "data", "media_type": "image/tiff; application=geotiff"}],
        "lineage": [],
        "raster": {"datetime": "2026-01-01T00:00:00Z", "bbox": [-76, 45, -75, 46], "semantics": {"band_1": "temperature"}},
    }


def publish(provider: Path, serving: Path, metadata: dict, filename: str) -> None:
    metadata_path = provider / filename
    metadata_path.write_text(json.dumps(metadata), encoding="utf-8")
    cli("--project", str(provider), "--format", "json", "serve", str(serving), "--metadata", str(metadata_path))


@pytest.fixture()
def peer_projects(tmp_path: Path):
    provider = tmp_path / "provider"
    client = tmp_path / "client with spaces"
    cli("--project", str(provider), "--format", "json", "init", "--namespace", "climate", "--serving-dir", "serving", "--owner-team", "Climate")
    serving = provider / "serving"
    table = serving / "datasets/observations/v1"
    table.mkdir(parents=True)
    pq.write_table(pa.table({"station_id": [1, 2], "temperature": [11, 12]}), table / "part-000.parquet")
    pq.write_table(pa.table({"station_id": [3, 4], "temperature": [13, 14]}), table / "part-001.parquet")
    pq.write_table(pa.table({"station_id": [99], "temperature": [99]}), table / "unregistered.parquet")
    publish(provider, serving, metadata_table(), "table.json")
    for version, filename in [("v1", "temperature.tiff"), ("v2", "temperature-v2.tiff")]:
        target = serving / f"datasets/temperature/{version}/{filename}"
        target.parent.mkdir(parents=True)
        with rasterio.open(target, "w", driver="GTiff", width=2, height=2, count=1, dtype="uint8", crs="EPSG:4326", transform=from_bounds(-76, 45, -75, 46, 2, 2), nodata=255) as dataset:
            dataset.write(np.array([[7, 8], [9, 10]], dtype="uint8"), 1)
        publish(provider, serving, metadata_raster(version, filename), f"raster-{version}.json")
    cli("--project", str(client), "--format", "json", "init", "--namespace", "consumer")
    peers = client / "peers"
    peers.mkdir()
    (peers / "local-alias").symlink_to(serving, target_is_directory=True)
    (client / ".feam/project.toml").write_text(
        "schema_version = 1\nnamespace = 'consumer'\n\n[[peers]]\nalias = 'local-alias'\nnamespace = 'climate'\npath = 'peers/local-alias'\n",
        encoding="utf-8",
    )
    cli("--project", str(client), "--format", "json", "refresh", cwd=tmp_path)
    return provider, client, serving


def test_sdk_returns_native_lazy_frame_with_only_registered_shards(peer_projects):
    _, client, serving = peer_projects
    project = Project.open(client, executable=EXE)
    table = project.resolve_table("product://climate/observations", version="v1")
    assert len(table.paths) == 2
    assert all("unregistered" not in str(path) for path in table.paths)
    query = project.scan_table("product://climate/observations", version="v1")
    assert isinstance(query, pl.LazyFrame)
    rows = query.filter(pl.col("station_id") > 1).select("station_id", "temperature").collect().to_dicts()
    assert rows == [{"station_id": 2, "temperature": 12}, {"station_id": 3, "temperature": 13}, {"station_id": 4, "temperature": 14}]
    pq.write_table(pa.table({"station_id": [100], "temperature": [100]}), serving / "datasets/observations/v1/late-unregistered.parquet")
    assert project.scan_table("product://climate/observations", version="v1").collect().height == 4


def test_loopback_stac_paginates_and_round_trips_identity_to_rasterio(peer_projects):
    provider, client, _ = peer_projects
    with socket.socket() as socket_probe:
        socket_probe.bind(("127.0.0.1", 0))
        port = socket_probe.getsockname()[1]
    process = subprocess.Popen([EXE, "--project", str(client), "stac", "serve", "--addr", f"127.0.0.1:{port}"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    base = f"http://127.0.0.1:{port}"
    try:
        for _ in range(50):
            try:
                with urllib.request.urlopen(base + "/", timeout=0.25) as response:
                    assert response.status == 200
                    break
            except (urllib.error.URLError, ConnectionError):
                time.sleep(0.1)
        else:
            raise AssertionError("STAC server did not start")
        catalog = Client.open(base)
        result = catalog.search(collections=["climate--temperature"], bbox=[-76, 45, -75, 46], datetime="2026-01-01T00:00:00Z/2026-01-01T00:00:00Z", limit=1, max_items=2)
        found = list(result.items())
        assert len(found) == 2
        schema = json.loads((Path(__file__).parents[2] / "mesh_core/tests/data/peer_access/stac-item-profile-v1.1.0.json").read_text())
        validate_json_schema(found[0].to_dict(), schema)
        empty = catalog.search(collections=["climate--temperature"], bbox=[0, 0, 1, 1], max_items=10)
        assert list(empty.items()) == []
        with urllib.request.urlopen(base + "/search?collections=climate--temperature&limit=1", timeout=1) as response:
            next_link = next(link["href"] for link in json.load(response)["links"] if link["rel"] == "next")
        cli("--project", str(provider), "--format", "json", "withdraw", "product://climate/temperature", "--version", "v2", "--reason", "pagination test")
        with pytest.raises(urllib.error.HTTPError) as changed:
            urllib.request.urlopen(next_link, timeout=1)
        assert changed.value.code == 409
        asset = Project.open(client, executable=EXE).resolve_asset("product://climate/temperature", version="v1", asset="data")
        with rasterio.open(asset.path) as dataset:
            assert dataset.crs.to_string() == "EPSG:4326"
            assert dataset.nodata == 255
            assert dataset.read(1, window=Window(0, 0, 2, 1)).tolist() == [[7, 8]]
    finally:
        process.terminate()
        process.wait(timeout=5)
