import copy
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from validate import InvalidRecord, load_record, validate

EXAMPLES = Path(__file__).resolve().parents[2] / "infra/demo/examples"


class ContractTests(unittest.TestCase):
    def example(self, kind):
        return load_record(EXAMPLES / f"{kind}.json")

    def rejected(self, kind, record):
        with self.assertRaises(InvalidRecord):
            validate(kind, record)

    def test_all_sanitized_examples(self):
        for path in EXAMPLES.glob("*.json"):
            with self.subTest(path=path.name):
                validate(path.stem, load_record(path))

    def test_request_cannot_supply_command_or_mount(self):
        for key in ("command", "mount", "image", "host_path"):
            request = self.example("lifecycle-request")
            request["parameters"][key] = "/host"
            self.rejected("lifecycle-request", request)

    def test_mutation_requires_generation_and_confirmation(self):
        request = self.example("lifecycle-request")
        del request["expected_generation"]
        self.rejected("lifecycle-request", request)
        for method in ("workspace.reset", "workspace.delete"):
            request = self.example("lifecycle-request")
            request["method"] = method
            self.rejected("lifecycle-request", request)

    def test_unknown_protocol_fails(self):
        request = self.example("lifecycle-request")
        request["protocol"] = "feam.web.v2"
        self.rejected("lifecycle-request", request)

    def test_test_settings_cannot_enable_live_or_public(self):
        for key in ("live_inference", "public_admission"):
            settings = self.example("settings")
            settings[key] = True
            self.rejected("settings", settings)

    def test_production_requires_resolved_inputs(self):
        for key in ("live_inference", "public_admission"):
            settings = self.example("settings")
            settings["mode"] = "production"
            settings[key] = True
            settings["model"]["backend"] = "openrouter"
            self.rejected("settings", settings)

    def test_no_raw_key_or_audience_sharing(self):
        settings = self.example("settings")
        settings["model"]["key"] = "synthetic-secret"
        self.rejected("settings", settings)
        settings = self.example("settings")
        settings["identity"]["admin_audience"] = settings["identity"]["portal_audience"]
        self.rejected("settings", settings)

    def test_cannot_change_route_or_add_retries(self):
        for key, value in [("provider", "unreviewed"), ("fallbacks", True), ("automatic_retries", 1)]:
            settings = self.example("settings")
            settings["model"][key] = value
            self.rejected("settings", settings)

    def test_unknown_run_remains_fully_reserved(self):
        ledger = self.example("project-ledger")
        ledger["runs"][0]["outstanding_usd_micros"] = 0
        self.rejected("project-ledger", ledger)

    def test_total_exposure_counts_settled_and_reservations(self):
        ledger = self.example("project-ledger")
        ledger["settled_usd_micros"] = 99000001
        self.rejected("project-ledger", ledger)
        ledger["settled_usd_micros"] = 99000000
        validate("project-ledger", ledger)

    def test_duplicate_allocation_and_activation_rejected(self):
        ledger = self.example("project-ledger")
        ledger["runs"].append(copy.deepcopy(ledger["runs"][0]))
        self.rejected("project-ledger", ledger)
        ledger["runs"][1]["run_id"] = "00000000-0000-4000-8000-000000000009"
        self.rejected("project-ledger", ledger)

    def test_verified_close_transfers_cost_once(self):
        ledger = self.example("project-ledger")
        run = ledger["runs"][0]
        run.update(state="closed", outstanding_usd_micros=0, settled_usd_micros=250000,
                   reconciliation_hash="a" * 64)
        self.rejected("project-ledger", ledger)
        ledger["settled_usd_micros"] = 250000
        validate("project-ledger", ledger)
        run["reconciliation_hash"] = None
        self.rejected("project-ledger", ledger)

    def test_money_is_bounded_integer_not_float_or_boolean(self):
        for value in (-1, 1.5, True, 10**20):
            ledger = self.example("project-ledger")
            ledger["settled_usd_micros"] = value
            self.rejected("project-ledger", ledger)

    def test_stopped_deployment_has_no_public_route(self):
        deployment = self.example("deployment")
        deployment["public_selected"] = True
        self.rejected("deployment", deployment)

    def test_teardown_cannot_erase_unarchived_tail(self):
        deployment = self.example("deployment")
        deployment["state"] = "torn-down"
        deployment["archive"]["pending_batches"] = 1
        self.rejected("deployment", deployment)

    def test_event_requires_provenance_and_no_raw_identity(self):
        event = self.example("event-envelope")
        del event["trust"]
        self.rejected("event-envelope", event)
        event = self.example("event-envelope")
        event["email"] = "synthetic@example.org"
        self.rejected("event-envelope", event)

    def test_duplicate_json_keys_and_oversize_fail(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "input.json"
            for text in ('{"mode":"test","mode":"production"}', ' ' * (1024 * 1024 + 1)):
                path.write_text(text)
                with self.assertRaises(InvalidRecord):
                    load_record(path)

    def test_formats_are_enforced_without_optional_dependencies(self):
        for value in ("not-a-timeZ", "2026-02-30T12:00:00Z", "2026-09-25T25:00:00Z"):
            event = self.example("event-envelope")
            event["occurred_at"] = value
            self.rejected("event-envelope", event)
        for value in ("https://example.org/path", "a..example.org", "*.example.org", "-a.example.org"):
            settings = self.example("settings")
            settings["identity"]["portal_hostname"] = value
            self.rejected("settings", settings)


if __name__ == "__main__":
    unittest.main()
