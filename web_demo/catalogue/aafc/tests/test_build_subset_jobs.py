import importlib.util
from pathlib import Path
import unittest


MODULE = Path(__file__).resolve().parents[1] / "build_subset_jobs.py"
SPEC = importlib.util.spec_from_file_location("build_subset_jobs", MODULE)
jobs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(jobs)


class PendingProductsTests(unittest.TestCase):
    def setUp(self):
        self.products = [
            {"product_id": f"product-{index:02d}"}
            for index in range(35)
        ]

    def test_excludes_only_known_already_released_products(self):
        remaining = jobs.pending_products(
            self.products, {"product-03", "product-19"}
        )
        self.assertEqual(len(remaining), 33)
        self.assertNotIn("product-03", {item["product_id"] for item in remaining})
        self.assertNotIn("product-19", {item["product_id"] for item in remaining})

    def test_unknown_duplicate_selection_and_wrong_count_fail(self):
        with self.assertRaisesRegex(ValueError, "unknown product"):
            jobs.pending_products(self.products, {"not-selected"})
        with self.assertRaisesRegex(ValueError, "duplicate product"):
            jobs.pending_products(self.products[:-1] + [self.products[0]], set())
        with self.assertRaisesRegex(ValueError, "35 non-CALMS"):
            jobs.pending_products(self.products[:-1], set())


if __name__ == "__main__":
    unittest.main()
