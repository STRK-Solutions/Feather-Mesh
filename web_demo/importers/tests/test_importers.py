import copy
import csv
from datetime import date
import importlib.util
from pathlib import Path
import tempfile
import unittest

import numpy as np
import polars as pl
import rasterio
from rasterio.transform import from_origin

MODULE = Path(__file__).resolve().parents[1] / "convert.py"
SPEC = importlib.util.spec_from_file_location("climate_converter", MODULE)
converter = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(converter)


class ImportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="feam-import-tests-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def csv(self, rows=None):
        rows = rows or [["6106001", "2023-01-01", "2.5", "E", "-1", "", "0.5", "", "", "M"],
                        ["6106001", "2023-01-02", "3", "", "-2", "", "0", "", "0", "T"]]
        path = self.root / "private.csv"
        with path.open("w", newline="", encoding="utf-8") as stream:
            writer = csv.writer(stream)
            writer.writerow(["Climate ID", "Date/Time", "Max Temp (°C)", "Max Temp Flag", "Min Temp (°C)", "Min Temp Flag",
                             "Mean Temp (°C)", "Mean Temp Flag", "Total Precip (mm)", "Total Precip Flag"])
            writer.writerows(rows)
        return path

    def test_nulls_flags_units_dates_and_known_lazy_query(self):
        destination = self.root / "table.parquet"
        report = converter.csv_to_parquet(self.csv(), destination, {"station": "6106001", "start_date": "2023-01-01", "end_date": "2023-01-02"})
        lazy = pl.scan_parquet([str(destination)], glob=False)
        self.assertIsInstance(lazy, pl.LazyFrame)
        result = lazy.filter(pl.col("maximum_temperature") > 2.7).select("date", "total_precipitation_flag").collect()
        self.assertEqual(result.rows(), [(date(2023, 1, 2), "T")])
        all_rows = lazy.collect()
        self.assertIsNone(all_rows["total_precipitation"][0])
        self.assertEqual(all_rows["total_precipitation_flag"][0], "M")
        self.assertEqual(report["table"]["column_units"]["maximum_temperature"], "degree_Celsius")
        self.assertEqual(report["conversion"]["row_count"], 2)

    def test_bad_station_date_incomplete_duplicate_and_nonfinite(self):
        for change in ["station", "date", "duplicate", "nonfinite", "incomplete"]:
            source = self.csv()
            with source.open() as stream:
                rows = list(csv.reader(stream))
            if change == "station": rows[1][0] = "different"
            if change == "date": rows[1][1] = "2024-01-01"
            if change == "duplicate": rows[2][1] = rows[1][1]
            if change == "nonfinite": rows[1][2] = "NaN"
            if change == "incomplete": rows.pop()
            with source.open("w", newline="") as stream: csv.writer(stream).writerows(rows)
            with self.subTest(change=change), self.assertRaises(ValueError):
                converter.csv_to_parquet(source, self.root / "bad.parquet", {"station": "6106001", "start_date": "2023-01-01", "end_date": "2023-01-02"})

    def raster(self, crs="EPSG:4326", nodata=-9999):
        source = self.root / "raw.tif"
        transform = from_origin(-76, 46, .5, .5) if crs == "EPSG:4326" else from_origin(-8460281, 5780349, 100000, 100000)
        with rasterio.open(source, "w", driver="GTiff", width=2, height=2, count=1, dtype="float32", crs=crs, transform=transform, nodata=nodata) as dataset:
            dataset.write(np.array([[10, 20], [30, -9999]], dtype=np.float32), 1)
        return source

    def spec(self):
        return {"bbox": [-76, 45, -75, 46], "resampling": "nearest", "variable": "synthetic temperature", "unit": "degree_Celsius",
                "scientific_time": {"kind": "instant", "start": "2023-01-01T12:00:00Z", "end": "2023-01-01T12:00:00Z", "evidence": "synthetic fixture definition"}}

    def test_known_pixels_nodata_georeferencing_time(self):
        destination = self.root / "published.tiff"
        report = converter.geotiff_to_wgs84(self.raster(), destination, self.spec())
        with rasterio.open(destination) as dataset:
            self.assertEqual(dataset.crs.to_epsg(), 4326)
            self.assertEqual(dataset.nodata, -9999)
            self.assertEqual(dataset.read(1, window=((0, 1), (0, 2))).tolist(), [[10, 20]])
            self.assertEqual(list(dataset.bounds), [-76, 45, -75, 46])
        self.assertEqual(report["raster"]["datetime"], "2023-01-01T12:00:00Z")
        self.assertEqual(report["conversion"]["resampling"], "nearest")

    def test_reprojection_changes_transform_and_records_provenance(self):
        spec = self.spec()
        spec["bbox"] = [-75.9, 44.9, -75.1, 45.5]
        output = self.root / "derived.tiff"
        report = converter.geotiff_to_wgs84(self.raster("EPSG:3857"), output, spec)
        self.assertEqual(report["conversion"]["source_crs"], "EPSG:3857")
        self.assertNotEqual(report["conversion"]["source_transform"], report["conversion"]["target_transform"])
        with rasterio.open(output) as dataset:
            self.assertEqual(dataset.crs.to_epsg(), 4326)
            self.assertEqual(dataset.shape, (512, 512))
            self.assertTrue(set(np.unique(dataset.read(1))).issubset({10, 20, 30, -9999}))

    def test_climatology_retains_interval_and_never_invents_instant(self):
        spec = self.spec()
        spec["scientific_time"] = {"kind": "climatology-interval", "start": "1991-01-01T00:00:00Z", "end": "2020-12-31T23:59:59Z", "evidence": "synthetic normal interval"}
        report = converter.geotiff_to_wgs84(self.raster(), self.root / "normal.tiff", spec)
        self.assertIsNone(report["raster"]["datetime"])
        self.assertEqual(report["raster"]["start_datetime"], spec["scientific_time"]["start"])
        self.assertEqual(report["raster"]["end_datetime"], spec["scientific_time"]["end"])
        self.assertEqual(report["conversion"]["scientific_time"], spec["scientific_time"])

    def test_corrupt_renamed_missing_georeference_and_nodata_fail(self):
        source = self.root / "not-really.tiff"
        source.write_text("not a TIFF")
        with self.assertRaises(rasterio.errors.RasterioIOError):
            converter.geotiff_to_wgs84(source, self.root / "bad.tiff", self.spec())
        for crs, nodata in [(None, -9999), ("EPSG:4326", None)]:
            with self.subTest(crs=crs, nodata=nodata), self.assertRaises(ValueError):
                converter.geotiff_to_wgs84(self.raster(crs, nodata), self.root / "bad.tiff", self.spec())


if __name__ == "__main__":
    unittest.main()
