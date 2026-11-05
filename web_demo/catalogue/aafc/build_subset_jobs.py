#!/usr/bin/env python3
"""Build closed FEAM pipeline jobs from the locked AAFC subset selection.

The script performs no network access. It verifies the exact private intake
bytes and hashes before emitting jobs in the pipeline's maximum batch size of
two products. Raw inputs and generated jobs must remain outside serving/.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid


JOB_NAMESPACE = uuid.UUID("c4b03ca7-d095-4cab-bf45-5abed2e6e594")
PEER_BY_SUITE = {
    "agroclimate": "climate",
    "nasdi": "climate",
    "calms": "earth-observation",
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def source_record(selection: dict, product: dict) -> dict:
    return {
        "id": product["source_id"],
        "importer": "aafc-geotiff",
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
    }


def product_record(product: dict) -> dict:
    suite = "Agroclimate" if product["suite"] == "agroclimate" else "NASDI"
    return {
        "source_id": product["source_id"],
        "id": product["product_id"],
        "version": product["version"],
        "name": f"{suite} {product['title']} bounded demo",
        "description": (
            "Ottawa-area WGS84 derivative of the selected AAFC "
            f"{suite} {product['title']} source."
        ),
        "intended_use": "Web-demo catalogue and bounded raster-read demonstration.",
        "limitations": (
            "Demonstration subset only; not a national replacement for the "
            "Agriculture and Agri-Food Canada source raster."
        ),
        "producer": "Agriculture and Agri-Food Canada",
        "contact": "open-data@example.invalid",
        "asset_id": "raster",
    }


def validate_input(input_dir: Path, product: dict) -> None:
    path = input_dir / product["source_id"]
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"missing regular input for {product['source_id']}")
    if path.stat().st_size != product["expected_bytes"]:
        raise ValueError(f"byte count mismatch for {product['source_id']}")
    if sha256(path) != product["sha256"]:
        raise ValueError(f"SHA-256 mismatch for {product['source_id']}")


def pending_products(products: list[dict], excluded_product_ids: set[str]) -> list[dict]:
    if len(products) != 35:
        raise ValueError("the locked selection must contain 35 non-CALMS products")
    product_ids = [product["product_id"] for product in products]
    if len(set(product_ids)) != len(product_ids):
        raise ValueError("the locked selection contains duplicate product ids")
    unknown = excluded_product_ids - set(product_ids)
    if unknown:
        raise ValueError(f"cannot exclude unknown product ids: {', '.join(sorted(unknown))}")
    return [product for product in products if product["product_id"] not in excluded_product_ids]


def build_jobs(
    selection: dict,
    input_dir: Path,
    output_dir: Path,
    excluded_product_ids: set[str] | None = None,
) -> list[Path]:
    if selection.get("kind") != "feam.aafc-demo-subset-selection.v1":
        raise ValueError("unsupported subset selection")
    products = pending_products(
        selection.get("products", []), excluded_product_ids or set()
    )
    for product in products:
        validate_input(input_dir, product)

    output_dir.mkdir(parents=True, exist_ok=True)
    if any(output_dir.iterdir()):
        raise ValueError("job output directory must be empty")

    written: list[Path] = []
    for offset in range(0, len(products), 2):
        batch = products[offset : offset + 2]
        peers = {PEER_BY_SUITE[product["suite"]] for product in batch}
        if len(peers) != 1:
            raise ValueError("a pipeline job cannot mix climate and earth-observation peers")
        peer = peers.pop()
        batch_number = offset // 2 + 1
        identity = ",".join(product["product_id"] for product in batch)
        job = {
            "protocol": "feam.pipeline.v1",
            "id": str(uuid.uuid5(JOB_NAMESPACE, identity)),
            "bundle": peer,
            "namespace": peer,
            "parent_digest": "",
            "importer_version": "2",
            "sources": [source_record(selection, product) for product in batch],
            "products": [product_record(product) for product in batch],
            "limits": {
                "source_bytes": sum(product["expected_bytes"] for product in batch),
                "output_bytes": 8388608,
                "candidate_bytes": 16777216,
                "elapsed_seconds": 300,
                "redirects": 3,
                "archive_entries": 0,
                "expanded_bytes": 0,
            },
            "policy": {
                "audience": "invited-users",
                "model_metadata": True,
                "model_payload": False,
                "research_eligible": True,
            },
        }
        destination = output_dir / f"aafc-subsets-{batch_number:02d}.json"
        destination.write_text(json.dumps(job, indent=2) + "\n", encoding="utf-8")
        written.append(destination)
    return written


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--selection", required=True, type=Path)
    parser.add_argument("--input-dir", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument(
        "--exclude-product-id",
        action="append",
        default=[],
        help="omit a product version already present in the current release; repeat as needed",
    )
    args = parser.parse_args()
    selection = json.loads(args.selection.read_text(encoding="utf-8"))
    excluded = set(args.exclude_product_id)
    if len(excluded) != len(args.exclude_product_id):
        raise ValueError("duplicate --exclude-product-id")
    written = build_jobs(selection, args.input_dir, args.output_dir, excluded)
    selected_count = len(selection["products"]) - len(excluded)
    print(f"wrote {len(written)} jobs for {selected_count} bounded products")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
