from pathlib import Path
import json
p=Path('provider')
p.mkdir(parents=True, exist_ok=True)
obs_v1={
  "schema_version":1,
  "namespace":"climate",
  "product_id":"observations",
  "name":"Demo observations",
  "version":"v1",
  "data_kind":"table",
  "data_format":"parquet",
  "description":"Synthetic Stage-1 table",
  "intended_use":"TUI and SDK demo",
  "limitations":"none",
  "owner_team":"Climate",
  "producer":"Feather Mesh demo",
  "contact":"demo@example.test",
  "usage_policy":"internal",
  "classification":"internal",
  "quality":"production",
  "assets":[
    {"id":"part-000","path":"datasets/observations/v1/part-000.parquet","role":"data","media_type":"application/vnd.apache.parquet"},
    {"id":"part-001","path":"datasets/observations/v1/part-001.parquet","role":"data","media_type":"application/vnd.apache.parquet"}
  ],
  "lineage":[],
  "table":{
    "column_meanings":{"station_id":"synthetic station identifier","temperature":"synthetic temperature"},
    "column_units":{"station_id":"not_applicable","temperature":"celsius"},
    "partition_columns":[]
  }
}
obs_v2=obs_v1.copy()
obs_v2['version']='v2'
obs_v2['assets']=[
  {"id":"part-000","path":"datasets/observations/v2/part-000.parquet","role":"data","media_type":"application/vnd.apache.parquet"},
  {"id":"part-001","path":"datasets/observations/v2/part-001.parquet","role":"data","media_type":"application/vnd.apache.parquet"}
]

temp_v1={
  "schema_version":1,
  "namespace":"climate",
  "product_id":"temperature",
  "name":"Demo temperature",
  "version":"v1",
  "data_kind":"raster",
  "data_format":"geotiff",
  "description":"Synthetic Stage-1 raster",
  "intended_use":"TUI and Rasterio demo",
  "limitations":"none",
  "owner_team":"Climate",
  "producer":"Feather Mesh demo",
  "contact":"demo@example.test",
  "usage_policy":"internal",
  "classification":"internal",
  "quality":"production",
  "assets":[
    {"id":"data","path":"datasets/temperature/v1/temperature.tiff","role":"data","media_type":"image/tiff; application=geotiff"}
  ],
  "lineage":[],
  "raster":{"datetime":"2026-01-01T00:00:00Z","bbox":[-76.0,45.0,-75.0,46.0],"semantics":{"band_1":"synthetic temperature"}}
}

(p/'observations-v1.json').write_text(json.dumps(obs_v1,indent=2))
(p/'observations-v2.json').write_text(json.dumps(obs_v2,indent=2))
(p/'temperature-v1.json').write_text(json.dumps(temp_v1,indent=2))
print('wrote manifests')
