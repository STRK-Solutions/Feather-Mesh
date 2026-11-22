#!/usr/bin/env python3
"""Build three closed CALMS archive jobs from the locked Week-44 selection.

This script performs no network access. It verifies exact private source
archives before emitting at most two products per cumulative release job.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid


JOB_NAMESPACE = uuid.UUID("7464bf64-c9eb-47cf-85f8-c8f77e68ddba")
EXPECTED_PRODUCTS = (
    ("days-of-week-maximum-ndvi", "quality-control-cloud-and-snow-flags"),
    ("weekly-best-quality-maximum-ndvi", "weekly-best-quality-maximum-ndvi-anomalies"),
    ("count-of-mean-weekly-best-quality-maximum-ndvi", "mean-weekly-best-quality-maximum-ndvi"),
)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def validate_input(input_dir: Path, product: dict) -> None:
    path = input_dir / product["source_id"]
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"missing regular input for {product['source_id']}")
    if path.stat().st_size != product["expected_bytes"]:
        raise ValueError(f"byte count mismatch for {product['source_id']}")
    if sha256(path) != product["sha256"]:
        raise ValueError(f"SHA-256 mismatch for {product['source_id']}")


def source_record(selection: dict, product: dict) -> dict:
    return {
        "id": product["source_id"],
        "importer": "aafc-calms-zip",
        "url": product["url"],
        "sha256": product["sha256"],
        "max_bytes": product["expected_bytes"],
        "upstream_version": product["upstream_version"],
        "retrieved_at": selection["retrieved_at"],
        "license": selection["license"],
        "attribution": selection["attribution"],
        "station": "",
        "start_date": "",
        "end_date": "",
        "scientific_time": product["scientific_time"],
        "bbox": selection["subset"]["bbox"],
        "resampling": selection["subset"]["resampling"],
        "variable": product["variable"],
        "unit": product["unit"],
        "archive": product["archive"],
        "value_domain": product["value_domain"],
    }


def product_record(selection: dict, product: dict) -> dict:
    return {
        "source_id": product["source_id"],
        "id": product["product_id"],
        "version": product["version"],
        "name": f"CALMS {product['title']} bounded demo",
        "description": (
            "Ottawa-area WGS84 derivative of the selected Agriculture and "
            f"Agri-Food Canada CALMS {product['title']} source."
        ),
        "intended_use": "Web-demo catalogue and bounded raster-read demonstration.",
        "limitations": (
            "Demonstration subset only; not a national replacement for the "
            "Agriculture and Agri-Food Canada CALMS source raster."
        ),
        "producer": "Agriculture and Agri-Food Canada",
        "contact": selection["contact"],
        "asset_id": "raster",
    }


def validate_selection(selection: dict) -> list[dict]:
    if selection.get("kind") != "feam.aafc-calms-demo-subset-selection.v1":
        raise ValueError("unsupported CALMS subset selection")
    products = selection.get("products", [])
    by_id = {product.get("product_id"): product for product in products}
    expected = {product_id for pair in EXPECTED_PRODUCTS for product_id in pair}
    if len(products) != 6 or set(by_id) != expected:
        raise ValueError("locked CALMS selection must contain the six expected products exactly once")
    if len({product["source_id"] for product in products}) != len(products):
        raise ValueError("locked CALMS selection contains duplicate source ids")
    return products


def build_jobs(selection: dict, input_dir: Path, output_dir: Path) -> list[Path]:
    products = validate_selection(selection)
    by_id = {product["product_id"]: product for product in products}
    for product in products:
        validate_input(input_dir, product)
    output_dir.mkdir(parents=True, exist_ok=True)
    if any(output_dir.iterdir()):
        raise ValueError("job output directory must be empty")

    written = []
    for index, product_ids in enumerate(EXPECTED_PRODUCTS, 1):
        batch = [by_id[product_id] for product_id in product_ids]
        expanded = sum(
            member["bytes"]
            for product in batch
            for member in product["archive"]["members"]
        )
        identity = ",".join(product_ids)
        job = {
            "protocol": "feam.pipeline.v1",
            "id": str(uuid.uuid5(JOB_NAMESPACE, identity)),
            "bundle": "earth-observation",
            "namespace": "earth-observation",
            "parent_digest": "",
            "importer_version": "3",
            "sources": [source_record(selection, product) for product in batch],
            "products": [product_record(selection, product) for product in batch],
            "limits": {
                "source_bytes": max(product["expected_bytes"] for product in batch),
                "output_bytes": 8 * 1024 * 1024,
                "candidate_bytes": 64 * 1024 * 1024,
                "elapsed_seconds": 300,
                "redirects": 3,
                "archive_entries": sum(len(product["archive"]["members"]) for product in batch),
                "expanded_bytes": expanded,
            },
            "policy": {
                "audience": "invited-users",
                "model_metadata": True,
                "model_payload": False,
                "research_eligible": True,
            },
        }
        destination = output_dir / f"calms-week44-{index:02d}.json"
        destination.write_text(json.dumps(job, indent=2) + "\n", encoding="utf-8")
        written.append(destination)
    return written


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--selection", required=True, type=Path)
    parser.add_argument("--input-dir", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args()
    selection = json.loads(args.selection.read_text(encoding="utf-8"))
    written = build_jobs(selection, args.input_dir, args.output_dir)
    print(f"wrote {len(written)} jobs for six bounded CALMS products")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
