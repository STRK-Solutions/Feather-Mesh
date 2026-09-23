#!/usr/bin/env python3
"""Explicitly opted-in synthetic model screening, not Stage-1 acceptance.

No feam operations are executed. Schemas come from the Rust export example;
fixture tool results are replayed only after the expected proposal is checked.
No retries or provider/model fallbacks are performed. Only synthetic text is
sent, and credentials and reasoning traces are excluded from saved reports.
"""
import argparse
import concurrent.futures
import datetime
import hashlib
import json
import math
import pathlib
import statistics
import threading
import time
import urllib.error
import urllib.request

MAX_TOKENS = 512
MAX_REQUEST_BYTES = 32768
MAX_RESPONSE_BYTES = 262144
ENDPOINT = "https://openrouter.ai/api/v1/chat/completions"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class Budget:
    def __init__(self, limit):
        self.limit = limit
        self.known = 0.0
        self.reserved = 0.0
        self.unknown_requests = 0
        self.requests = 0
        self.lock = threading.Lock()

    def reserve(self, amount):
        with self.lock:
            if self.requests >= 120 or self.known + self.reserved + amount > self.limit:
                raise ValueError("screening_budget_exhausted")
            self.requests += 1
            self.reserved += amount

    def settle(self, reservation, cost):
        with self.lock:
            if cost is None:
                # Retain the estimate against the budget when billing is unknown.
                self.unknown_requests += 1
            else:
                self.reserved -= reservation
                self.known += cost


def check_schema(value, schema):
    """Validate the JSON Schema subset present in the exported feam tools."""
    kind = schema.get("type")
    types = {"object": dict, "array": list, "string": str, "integer": int, "boolean": bool}
    if kind not in types or type(value) is not types[kind]:
        raise ValueError("argument_type")
    if "enum" in schema and value not in schema["enum"]:
        raise ValueError("argument_enum")
    if kind == "integer" and not schema.get("minimum", value) <= value <= schema.get("maximum", value):
        raise ValueError("argument_range")
    if kind == "object":
        properties = schema.get("properties", {})
        if not set(schema.get("required", [])).issubset(value):
            raise ValueError("missing_argument")
        if schema.get("additionalProperties") is False and set(value) - set(properties):
            raise ValueError("unknown_argument")
        for key, child in value.items():
            if key in properties:
                check_schema(child, properties[key])
    if kind == "array" and "items" in schema:
        for child in value:
            check_schema(child, schema["items"])


def wire_tools(schemas):
    names, result = {}, []
    for schema in schemas:
        wire = schema["name"].replace(".", "__")
        if wire in names or not 1 <= len(wire) <= 64 or not all(c.isascii() and (c.isalnum() or c in "_-") for c in wire):
            raise ValueError("invalid_or_ambiguous_tool_name")
        names[wire] = schema
        result.append({"type": "function", "function": {**schema, "name": wire}})
    return names, result


def score_step(message, expected, names):
    calls = message.get("tool_calls") or []
    text = (message.get("content") or "").lower()
    if expected.get("no_call"):
        if calls:
            return "unexpected_tool_proposal"
        if not text:
            return "empty_answer"
        if expected.get("question") and "?" not in text:
            return "missing_clarification"
        if expected.get("text_any") and not any(word.lower() in text for word in expected["text_any"]):
            return "answer_assertion"
        if not all(word.lower() in text for word in expected.get("text_all", [])):
            return "answer_assertion"
        if any(word.lower() in text for word in expected.get("text_none", [])):
            return "forbidden_answer"
        return None
    if len(calls) != 1:
        return "tool_call_count"
    call = calls[0]
    try:
        schema = names[call["function"]["name"]]
        if call.get("type") != "function" or not call.get("id"):
            return "tool_call_shape"
        arguments = json.loads(call["function"]["arguments"])
        check_schema(arguments, schema["parameters"])
        if schema["name"] != expected["call"]:
            return "wrong_tool"
        if any(arguments.get(key) != value for key, value in expected["args"].items()):
            return "wrong_arguments"
        if any(arguments.get(key) is True for key in expected.get("forbid_true", [])):
            return "unapproved_integrity_proposal"
    except (KeyError, ValueError, TypeError):
        return "invalid_tool_proposal"
    return None


def request(key, profile, messages, tools, budget):
    body = {
        "model": profile["model"], "messages": messages, "tools": tools,
        "tool_choice": "auto", "stream": False, "max_tokens": MAX_TOKENS,
        "temperature": 0, "reasoning": {"enabled": False, "exclude": True},
        "provider": {"only": [profile["provider"]], "allow_fallbacks": False,
                     "require_parameters": True, "max_price": profile["max_price"]},
    }
    wire = json.dumps(body).encode()
    if len(wire) > MAX_REQUEST_BYTES:
        return {"failure": "request_limit", "latency_seconds": 0}
    prices = profile["max_price"]
    # Conservative token estimate with template headroom. This is local
    # accounting, not a replacement for the OpenRouter key's billing cap.
    reservation = ((len(wire) + 8192) * prices["prompt"] + MAX_TOKENS * prices["completion"]) / 1_000_000
    budget.reserve(reservation)
    started = time.monotonic()
    result = {"request_bytes": len(wire), "cost_usd": None}
    try:
        req = urllib.request.Request(ENDPOINT, data=wire, headers={
            "Authorization": "Bearer " + key, "Content-Type": "application/json",
        })
        with urllib.request.build_opener(NoRedirect()).open(req, timeout=45) as response:
            raw = response.read(MAX_RESPONSE_BYTES + 1)
            if len(raw) > MAX_RESPONSE_BYTES:
                raise ValueError("response_limit")
            data = json.loads(raw)
        if "error" in data:
            raise ValueError("provider_error")
        usage = data.get("usage") or {}
        cost = usage.get("cost")
        if type(cost) in (int, float) and math.isfinite(cost) and cost >= 0:
            result["cost_usd"] = cost
        result.update({
            "generation_id": data.get("id"), "served_model": data.get("model"),
            "served_provider": data.get("provider"),
            "usage": {k: usage.get(k) for k in ["prompt_tokens", "completion_tokens", "total_tokens"]},
        })
        choice = data["choices"][0]
        result["finish_reason"] = choice.get("finish_reason")
        message = choice["message"]
        # Deliberately discard reasoning / reasoning_details before reporting.
        result["message"] = {k: message.get(k) for k in ["role", "content", "tool_calls"] if k in message}
        if choice.get("finish_reason") not in ["stop", "tool_calls"]:
            result["failure"] = "incomplete_response"
    except urllib.error.HTTPError as error:
        result["failure"] = "http_" + str(error.code)
        try:
            data = json.loads(error.read(8192)).get("error", {})
            result["error_message"] = str(data.get("message", ""))[:300].replace(key, "[REDACTED]")
        except (ValueError, OSError):
            pass
    except (OSError, ValueError, KeyError, IndexError, TypeError) as error:
        result["failure"] = type(error).__name__
    finally:
        result["latency_seconds"] = round(time.monotonic() - started, 4)
        budget.settle(reservation, result["cost_usd"])
    return result


def evaluate(profile, fixture, schemas, key, budget, output):
    names, tools = wire_tools(schemas)
    rows = []
    for task in fixture["tasks"]:
        messages = [{"role": "system", "content": fixture["system"]}]
        if task.get("context"):
            messages.append({"role": "system", "content": task["context"]})
        messages.append({"role": "user", "content": task["user"]})
        row = {"task_id": task["id"], "passed": False, "attempts": []}
        for expected in task["steps"]:
            try:
                response = request(key, profile, messages, tools, budget)
            except ValueError as error:
                response = {"failure": str(error), "latency_seconds": 0, "cost_usd": None}
            row["attempts"].append(response)
            failure = response.get("failure") or score_step(response["message"], expected, names)
            if failure:
                row["failure"] = failure
                break
            if "reply" in expected:
                assistant = response["message"]
                messages.append(assistant)
                messages.append({"role": "tool", "tool_call_id": assistant["tool_calls"][0]["id"], "content": json.dumps(expected["reply"])})
        else:
            row["passed"] = True
        rows.append(row)
        with (output / (profile["label"] + ".jsonl")).open("a") as file:
            file.write(json.dumps(row) + "\n")
        print(json.dumps({"model": profile["label"], "task": row["task_id"], "passed": row["passed"], "failure": row.get("failure")}), flush=True)
    return {"profile": profile, "tasks": rows}


def summarize(run):
    rows = run["tasks"]
    attempts = [attempt for row in rows for attempt in row["attempts"]]
    latencies = sorted(a["latency_seconds"] for a in attempts)
    passed = sum(row["passed"] for row in rows)
    cost = sum(a.get("cost_usd") or 0 for a in attempts)
    failures = {}
    for row in rows:
        if not row["passed"]:
            failures[row["failure"]] = failures.get(row["failure"], 0) + 1
    return {
        "label": run["profile"]["label"], "passed": passed, "tasks": len(rows),
        "requests": len(attempts), "known_cost_usd": cost,
        "unknown_cost_requests": sum(a.get("cost_usd") is None for a in attempts),
        "known_cost_per_pass_usd": cost / passed if passed else None,
        "median_request_seconds": statistics.median(latencies),
        "p95_request_seconds": latencies[math.ceil(0.95 * len(latencies)) - 1],
        "failure_categories": failures,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--key-file", type=pathlib.Path, required=True)
    parser.add_argument("--schemas", type=pathlib.Path, required=True)
    parser.add_argument("--fixtures", type=pathlib.Path, required=True)
    parser.add_argument("--profiles", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--budget-usd", type=float, default=1.0)
    parser.add_argument("--live", action="store_true", required=True, help="Explicitly authorize network inference with the supplied profiles.")
    args = parser.parse_args()
    if not 0 < args.budget_usd <= 20:
        parser.error("budget must be positive and no greater than US$20")
    profiles = json.loads(args.profiles.read_text())
    fixture = json.loads(args.fixtures.read_text())
    schemas = json.loads(args.schemas.read_text())
    if len(profiles) > 5 or sum(len(t["steps"]) for t in fixture["tasks"]) * len(profiles) > 120:
        parser.error("screening is limited to five profiles and 120 requests")
    for profile in profiles:
        price = profile["max_price"]
        if not 0 <= price["prompt"] <= 0.7 or not 0 <= price["completion"] <= 3.5 or price.get("request") != 0:
            parser.error("profile prices exceed the bounded screening envelope")
    wire_tools(schemas)
    key = args.key_file.read_text().strip()
    if not key:
        parser.error("key file is empty")
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    budget = Budget(args.budget_usd)
    report = {
        "created_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "purpose": fixture["purpose"], "reasoning_enabled": False,
        "max_tokens": MAX_TOKENS, "concurrency": 2, "budget_usd": args.budget_usd,
        "hashes": {label: hashlib.sha256(path.read_bytes()).hexdigest() for label, path in
                   [("schemas", args.schemas), ("fixtures", args.fixtures), ("profiles", args.profiles), ("runner", pathlib.Path(__file__))]},
    }
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        report["runs"] = list(pool.map(lambda p: evaluate(p, fixture, schemas, key, budget, args.output), profiles))
    report["summaries"] = [summarize(run) for run in report["runs"]]
    report["known_cost_usd"] = budget.known
    report["unknown_cost_requests"] = budget.unknown_requests
    report["reserved_unknown_cost_usd"] = budget.reserved
    (args.output / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"summaries": report["summaries"], "known_cost_usd": budget.known, "report": str(args.output / "report.json")}), flush=True)


if __name__ == "__main__":
    main()
