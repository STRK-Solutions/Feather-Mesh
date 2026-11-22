import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import unittest


MODULE = Path(__file__).resolve().parents[1] / "build_calms_jobs.py"
SPEC = importlib.util.spec_from_file_location("build_calms_jobs", MODULE)
jobs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(jobs)


class BuildCalmsJobsTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="feam-calms-jobs-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.inputs = self.root / "inputs"
        self.inputs.mkdir()
        products = []
        for index, product_id in enumerate(product for pair in jobs.EXPECTED_PRODUCTS for product in pair):
            source_id = f"source-{index}"
            body = f"archive-{index}".encode()
            (self.inputs / source_id).write_bytes(body)
            products.append({
                "product_id": product_id,
                "source_id": source_id,
                "title": product_id,
                "url": f"https://agriculture.canada.ca/{source_id}.zip",
                "expected_bytes": len(body),
                "sha256": hashlib.sha256(body).hexdigest(),
                "upstream_version": "week44",
                "version": "v2026w44",
                "scientific_time": {"kind": "interval", "start": "2026-10-26T00:00:00Z", "end": "2026-11-02T00:00:00Z", "evidence": "fixture"},
                "variable": product_id,
                "unit": "1",
                "value_domain": {"ranges": [{"min": 0, "max": 1}], "labels": {}},
                "archive": {"primary_member": f"{source_id}.tif", "members": [
                    {"name": f"{source_id}.tif", "bytes": 10 + index, "sha256": "a" * 64}
                ]},
            })
        self.selection = {
            "kind": "feam.aafc-calms-demo-subset-selection.v1",
            "retrieved_at": "2026-11-14T00:00:00Z",
            "license": "https://open.canada.ca/en/open-government-licence-canada",
            "attribution": "AAFC",
            "contact": "test@example.invalid",
            "subset": {"bbox": [-76, 45, -75, 46], "resampling": "nearest"},
            "products": products,
        }

    def test_emits_three_closed_two_product_jobs_with_exact_archive_limits(self):
        output = self.root / "jobs"
        written = jobs.build_jobs(self.selection, self.inputs, output)
        self.assertEqual(len(written), 3)
        decoded = [json.loads(path.read_text()) for path in written]
        self.assertEqual([len(job["products"]) for job in decoded], [2, 2, 2])
        self.assertTrue(all(job["bundle"] == job["namespace"] == "earth-observation" for job in decoded))
        self.assertTrue(all(job["importer_version"] == "3" for job in decoded))
        self.assertTrue(all(job["limits"]["archive_entries"] == 2 for job in decoded))
        self.assertEqual(decoded[0]["limits"]["expanded_bytes"], 21)

    def test_rejects_missing_product_changed_bytes_and_nonempty_output(self):
        with self.assertRaisesRegex(ValueError, "six expected"):
            jobs.build_jobs({**self.selection, "products": self.selection["products"][:-1]}, self.inputs, self.root / "missing")
        first = self.inputs / self.selection["products"][0]["source_id"]
        first.write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "byte count mismatch"):
            jobs.build_jobs(self.selection, self.inputs, self.root / "changed")
        first.write_bytes(b"archive-0")
        output = self.root / "not-empty"
        output.mkdir()
        (output / "existing").write_text("preserve")
        with self.assertRaisesRegex(ValueError, "must be empty"):
            jobs.build_jobs(self.selection, self.inputs, output)


if __name__ == "__main__":
    unittest.main()
