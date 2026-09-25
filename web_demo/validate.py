#!/usr/bin/env python3
"""Validate W0 records offline. This is not a runtime authorization or ledger writer."""
import argparse
from datetime import datetime
import json
from pathlib import Path
import re

from jsonschema import Draft202012Validator, FormatChecker

SCHEMAS = Path(__file__).resolve().parent / "schemas"
MAX_BYTES = 1024 * 1024
FORMATS = FormatChecker()


@FORMATS.checks("date-time", raises=ValueError)
def utc_timestamp(value):
    # jsonschema's optional format extras are not assumed to be installed.
    if not isinstance(value, str):
        return True  # The schema's type constraint handles this case.
    if not re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z", value):
        return False
    datetime.fromisoformat(value)
    return True


@FORMATS.checks("hostname")
def ascii_hostname(value):
    if not isinstance(value, str):
        return True
    return (len(value) <= 253 and "." in value
            and all(re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", label)
                    for label in value.split(".")))


class InvalidRecord(ValueError):
    pass


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise InvalidRecord("duplicate JSON key")
        result[key] = value
    return result


def load_record(path):
    with Path(path).open("rb") as source:
        data = source.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise InvalidRecord("record exceeds one MiB")
    try:
        return json.loads(data, object_pairs_hook=unique_object)
    except (UnicodeError, json.JSONDecodeError, RecursionError) as exc:
        raise InvalidRecord("invalid JSON encoding or nesting") from exc


def validate(kind, record):
    if kind not in {p.name.removesuffix(".schema.json") for p in SCHEMAS.glob("*.schema.json")}:
        raise InvalidRecord("unknown record kind")
    schema = load_record(SCHEMAS / f"{kind}.schema.json")
    Draft202012Validator.check_schema(schema)
    validator = Draft202012Validator(schema, format_checker=FORMATS)
    if next(validator.iter_errors(record), None) is not None:
        # Do not echo rejected values: private records can contain secrets.
        raise InvalidRecord("record does not match the closed v1 schema")
    if kind == "settings":
        identity = record["identity"]
        if identity["portal_hostname"] == identity["admin_hostname"]:
            raise InvalidRecord("portal and admin origins must be separate")
        audiences = [identity[k] for k in ("portal_audience", "admin_audience", "workspace_audience")]
        if len(set(audiences)) != len(audiences):
            raise InvalidRecord("application audiences must be distinct")
        if record["live_inference"] and (record["research"]["retention_days"] is None
                                         or not record["research"]["reviewer_ids"]
                                         or record["research"]["archive"] == "unconfigured"):
            raise InvalidRecord("live capture policy is incomplete")
    elif kind == "project-ledger":
        runs = record["runs"]
        if len({run["run_id"] for run in runs}) != len(runs):
            raise InvalidRecord("duplicate run allocation")
        activations = [(r["deployment_id"], r["activation_generation"]) for r in runs]
        if len(set(activations)) != len(activations):
            raise InvalidRecord("duplicate deployment activation allocation")
        for run in runs:
            if run["state"] == "closed":
                if (run["outstanding_usd_micros"] != 0 or run["reconciliation_hash"] is None
                        or run["settled_usd_micros"] > run["allocation_usd_micros"]):
                    raise InvalidRecord("closed allocation lacks verified reconciliation")
            elif (run["outstanding_usd_micros"] != run["allocation_usd_micros"]
                  or run["settled_usd_micros"] != 0 or run["reconciliation_hash"] is not None):
                raise InvalidRecord("unclosed runs must retain the full reservation")
        closed_spend = sum(r["settled_usd_micros"] for r in runs if r["state"] == "closed")
        if record["settled_usd_micros"] < closed_spend:
            raise InvalidRecord("project settlement omits closed runs")
        exposure = record["settled_usd_micros"] + sum(r["outstanding_usd_micros"] for r in runs)
        if exposure > record["allowance_usd_micros"]:
            raise InvalidRecord("project exposure exceeds allowance")
    elif kind == "deployment":
        if record["public_selected"] and (record["desired"] != "running" or record["state"] != "running"):
            raise InvalidRecord("public route requires a running deployment")
        if record["state"] == "torn-down" and (record["desired"] != "stopped"
                                              or record["archive"]["pending_batches"] != 0):
            raise InvalidRecord("teardown cannot discard an unarchived tail")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind")
    parser.add_argument("record", type=Path)
    args = parser.parse_args()
    try:
        validate(args.kind, load_record(args.record))
    except (InvalidRecord, OSError) as exc:
        parser.exit(1, f"validation failed: {exc if isinstance(exc, InvalidRecord) else 'cannot read file'}\n")
    print("valid v1 record; deployment readiness and authorization are separate checks")


if __name__ == "__main__":
    main()
