#!/usr/bin/env python3
"""Validate the repository-owned AAFC source-product catalogue.

This is intentionally a catalogue validator, not a FEAM publication validator.
It verifies that the reusable product vocabulary is internally consistent while
leaving asset inspection and authoritative serving-manifest creation to FEAM.
"""

from __future__ import annotations

import json
import re
from datetime import datetime
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).parent
ID = re.compile(r"^[a-z][a-z0-9-]*$")
EXPECTED_PRODUCT_COUNTS = {"agroclimate": 21, "nasdi": 14, "calms": 6}
EXPECTED_PEERS = {
    "climate": {"agroclimate", "nasdi"},
    "earth-observation": {"calms"},
}
SHA256 = re.compile(r"^[a-f0-9]{64}$")


def load_json(name: str) -> dict:
    with (ROOT / name).open(encoding="utf-8") as source:
        return json.load(source)


def fail(message: str) -> None:
    raise ValueError(message)


def validate_suite(suite: dict, expected_id: str) -> dict[str, str | None]:
    if suite.get("schema_version") != 1 or suite.get("kind") != "feam.source-product-suite.v1":
        fail(f"{expected_id}: unsupported suite manifest version")
    if suite.get("id") != expected_id or suite.get("publication_boundary") != "catalogue_only":
        fail(f"{expected_id}: wrong suite identity or publication boundary")
    expected_peer = "earth-observation" if expected_id == "calms" else "climate"
    if suite.get("peer") != expected_peer:
        fail(f"{expected_id}: expected peer {expected_peer!r}")
    profile = suite.get("default_publication_profile", {})
    if profile.get("data_kind") != "raster" or profile.get("data_format") != "geotiff":
        fail(f"{expected_id}: catalogue products must declare their intended raster publication profile")

    references: dict[str, str | None] = {}
    product_count = 0
    for category in suite.get("categories", []):
        category_id = category.get("id", "")
        if not ID.fullmatch(category_id):
            fail(f"{expected_id}: invalid category id {category_id!r}")
        for product in category.get("products", []):
            product_id = product.get("id", "")
            if not ID.fullmatch(product_id):
                fail(f"{expected_id}: invalid product id {product_id!r}")
            reference = f"{expected_id}/{category_id}/{product_id}"
            if reference in references:
                fail(f"{expected_id}: duplicate product reference {reference}")
            references[reference] = product.get("semantic_concept_id")
            variant_ids: set[str] = set()
            for variant in product.get("variants", []):
                variant_id = variant.get("id", "")
                if not ID.fullmatch(variant_id):
                    fail(f"{expected_id}: invalid variant id {variant_id!r}")
                if variant_id in variant_ids:
                    fail(f"{expected_id}: duplicate variant id {variant_id!r} in {reference}")
                variant_ids.add(variant_id)
            product_count += 1
    if product_count != EXPECTED_PRODUCT_COUNTS[expected_id]:
        fail(f"{expected_id}: expected {EXPECTED_PRODUCT_COUNTS[expected_id]} products, found {product_count}")
    return references


def parse_utc(value: str) -> datetime:
    if not isinstance(value, str) or not value.endswith("Z"):
        fail(f"subset selection: scientific time must be UTC, found {value!r}")
    return datetime.fromisoformat(value)


def validate_subset_selection(selection: dict, references: dict[str, str | None]) -> None:
    if selection.get("schema_version") != 1 or selection.get("kind") != "feam.aafc-demo-subset-selection.v1":
        fail("subset selection: unsupported version")
    if selection.get("publication_boundary") != "pipeline_input_selection":
        fail("subset selection: must not be mistaken for a serving manifest")
    subset = selection.get("subset", {})
    if subset != {
        "bbox": [-76.0, 45.0, -75.0, 46.0],
        "target_crs": "EPSG:4326",
        "maximum_width": 512,
        "maximum_height": 512,
        "resampling": "nearest",
    }:
        fail("subset selection: the bounded Ottawa publication profile changed")

    expected = {
        (suite, product)
        for reference in references
        for suite, _category, product in [reference.split("/")]
        if suite in {"agroclimate", "nasdi"}
    }
    products = selection.get("products", [])
    actual = {(product.get("suite"), product.get("product_id")) for product in products}
    if len(products) != 35 or actual != expected:
        fail("subset selection: every Agroclimate and NASDI product must occur exactly once")
    if len({product.get("source_id") for product in products}) != len(products):
        fail("subset selection: source ids must be unique")

    total_bytes = 0
    for product in products:
        source_id = product.get("source_id", "")
        if not ID.fullmatch(source_id):
            fail(f"subset selection: invalid source id {source_id!r}")
        url = urlparse(product.get("url", ""))
        if (
            url.scheme != "https"
            or url.hostname != "agriculture.canada.ca"
            or not url.path.startswith("/atlas/data_donnees/")
            or not url.path.endswith(".tif")
            or url.query
            or url.fragment
        ):
            fail(f"subset selection: unapproved source URL for {source_id}")
        expected_bytes = product.get("expected_bytes")
        if not isinstance(expected_bytes, int) or not 0 < expected_bytes <= 64 * 1024 * 1024:
            fail(f"subset selection: invalid byte bound for {source_id}")
        total_bytes += expected_bytes
        if not SHA256.fullmatch(product.get("sha256", "")):
            fail(f"subset selection: invalid SHA-256 for {source_id}")
        if not re.fullmatch(r"v[a-z0-9]+", product.get("version", "")):
            fail(f"subset selection: invalid FEAM version for {source_id}")
        scientific = product.get("scientific_time", {})
        if scientific.get("kind") not in {"instant", "interval", "climatology-interval"} or not scientific.get("evidence"):
            fail(f"subset selection: incomplete scientific time for {source_id}")
        start, end = parse_utc(scientific.get("start", "")), parse_utc(scientific.get("end", ""))
        if scientific["kind"] == "instant" and start != end:
            fail(f"subset selection: instant bounds differ for {source_id}")
        if scientific["kind"] != "instant" and start >= end:
            fail(f"subset selection: interval is empty for {source_id}")
        if not product.get("variable") or not product.get("unit"):
            fail(f"subset selection: missing raster semantics for {source_id}")
    if total_bytes > 64 * 1024 * 1024:
        fail("subset selection: transient source intake exceeds the 64 MiB demo bound")


def main() -> int:
    index = load_json("catalogue.json")
    if index.get("schema_version") != 1 or index.get("kind") != "feam.source-product-catalogue.v1":
        fail("catalogue: unsupported version")
    if index.get("publication_boundary") != "catalogue_only":
        fail("catalogue: must not be mistaken for a serving manifest")

    references: dict[str, str | None] = {}
    declared_suites = index.get("suites", [])
    if {entry.get("id") for entry in declared_suites} != set(EXPECTED_PRODUCT_COUNTS):
        fail("catalogue: suite index must contain Agroclimate, NASDI, and CALMS exactly once")
    for entry in declared_suites:
        manifest = entry.get("manifest", "")
        if Path(manifest).name != manifest or not manifest.endswith(".manifest.json"):
            fail(f"catalogue: unsafe or invalid suite manifest path {manifest!r}")
        references.update(validate_suite(load_json(manifest), entry["id"]))

    peers = index.get("peers", [])
    peer_suites = {peer.get("id"): set(peer.get("suites", [])) for peer in peers}
    if peer_suites != EXPECTED_PEERS:
        fail(f"catalogue: peers must map to {EXPECTED_PEERS}")
    for peer in peers:
        peer_id = peer.get("id")
        if peer.get("bundle") != peer_id or peer.get("namespace") != peer_id:
            fail(f"catalogue: peer {peer_id!r} must use matching bundle and namespace")

    declared_concepts: set[str] = set()
    concept_references: dict[str, set[str]] = {}
    for concept in index.get("semantic_concepts", []):
        concept_id = concept.get("id", "")
        if not ID.fullmatch(concept_id):
            fail(f"catalogue: invalid semantic concept id {concept_id!r}")
        if concept_id in declared_concepts:
            fail(f"catalogue: duplicate semantic concept {concept_id}")
        declared_concepts.add(concept_id)
        concept_references[concept_id] = set(concept.get("product_references", []))
        for reference in concept.get("product_references", []):
            if reference not in references:
                fail(f"catalogue: semantic concept {concept_id} references unknown product {reference}")
            if references[reference] != concept_id:
                fail(f"catalogue: semantic concept {concept_id} disagrees with {reference}")
    for reference, concept_id in references.items():
        if concept_id is not None and concept_id not in declared_concepts:
            fail(f"catalogue: {reference} names an undeclared semantic concept {concept_id}")
        if concept_id is not None and reference not in concept_references[concept_id]:
            fail(f"catalogue: semantic concept {concept_id} omits product {reference}")

    validate_subset_selection(load_json("subset-selection.json"), references)

    print("AAFC source-product catalogue is valid")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
