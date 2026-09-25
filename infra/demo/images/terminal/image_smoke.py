"""Real installed SDK and pinned demo reader smoke, executed inside the image."""
from pathlib import Path
import json
import subprocess
import polars as pl
import rasterio
from rasterio.windows import Window
from feam import Project

root = Path('/workspace/demo/client with spaces')
project = Project.open(root)
lazy = project.scan_table('product://climate/observations', version='v1')
assert isinstance(lazy, pl.LazyFrame)
assert lazy.filter(pl.col('station_id') > 2).select('temperature').collect().to_dicts() == [{'temperature': 13}, {'temperature': 14}]
asset = project.resolve_asset('product://climate/temperature', version='v1', asset='data')
assert 'peers/nearby-climate' in str(asset.path)
with rasterio.open(asset.path) as dataset:
    assert dataset.read(1, window=Window(0,0,2,2)).tolist() == [[7,8],[9,10]]
    assert dataset.crs.to_epsg() == 4326 and dataset.nodata == 255
link = root/'peers/nearby-climate'
assert link.is_symlink() and str(link.resolve()) == '/workspace/demo/provider/serving'
print(json.dumps({'sdk_installed': True, 'native_lazyframe': True, 'known_raster_window': True, 'final_path_symlink': True}))
