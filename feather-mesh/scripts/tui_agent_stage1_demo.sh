#!/usr/bin/env bash
# Create or reset a disposable Stage-1 provider/client demo. This script only
# removes its fixed children after confirming its own marker in the requested
# directory; it never touches source fixtures.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 DEMO_ROOT [--reset]" >&2
  exit 2
fi

demo_root_input=$1
reset=${2:-}
if [[ "$demo_root_input" != /* ]]; then
  echo "DEMO_ROOT must be an absolute path" >&2
  exit 2
fi
if [[ -n "$reset" && "$reset" != "--reset" ]]; then
  echo "only --reset is supported" >&2
  exit 2
fi

if [[ -L "$demo_root_input" || "$demo_root_input" == "/" ]]; then
  echo "refusing a symlink or filesystem root" >&2
  exit 2
fi
mkdir -p -- "$demo_root_input"
demo_root=$(cd -- "$demo_root_input" && pwd -P)
marker="$demo_root/.feam-stage1-demo-marker"
if [[ -L "$marker" ]]; then
  echo "refusing a symlink marker" >&2
  exit 2
fi
if [[ -e "$marker" ]]; then
  if [[ ! -f "$marker" || "$(cat "$marker")" != "feam-stage1-demo-v1" ]]; then
    echo "refusing an invalid demo marker" >&2
    exit 2
  fi
  if [[ -n "$reset" ]]; then
    rm -rf -- "$demo_root/provider" "$demo_root/client with spaces"
  fi
else
  if [[ -n "$(find "$demo_root" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    echo "refusing nonempty unmarked demo root: $demo_root" >&2
    exit 2
  fi
  printf '%s\n' 'feam-stage1-demo-v1' > "$marker"
fi

workspace=${FEAM_WORKSPACE:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)}
feam=${FEAM_EXECUTABLE:-$workspace/target/debug/mesh_cli}
fixtures="$workspace/mesh_core/tests/data/peer_access"
if [[ ! -x "$feam" ]]; then
  echo "build the CLI first: cargo build --bin mesh_cli" >&2
  exit 2
fi
if [[ ! -f "$fixtures/table/part-000.parquet" ]]; then
  echo "generate fixtures first: python mesh_core/tests/data/peer_access/generate_fixtures.py" >&2
  exit 2
fi

provider="$demo_root/provider"
client="$demo_root/client with spaces"
if [[ -e "$provider" || -e "$client" ]]; then
  echo "demo exists; rerun with --reset" >&2
  exit 2
fi

"$feam" --project "$provider" init --namespace climate --serving-dir serving --owner-team Climate
mkdir -p "$provider/serving/datasets/observations/v1" \
  "$provider/serving/datasets/observations/v2" \
  "$provider/serving/datasets/guided-observations/v1" \
  "$provider/serving/datasets/temperature/v1"
cp "$fixtures/table/part-000.parquet" "$provider/serving/datasets/observations/v1/part-000.parquet"
cp "$fixtures/table/part-001.parquet" "$provider/serving/datasets/observations/v1/part-001.parquet"
cp "$fixtures/table/part-000.parquet" "$provider/serving/datasets/observations/v2/part-000.parquet"
cp "$fixtures/table/part-001.parquet" "$provider/serving/datasets/observations/v2/part-001.parquet"
cp "$fixtures/table/unregistered.parquet" "$provider/serving/datasets/observations/v1/unregistered.parquet"
cp "$fixtures/table/part-000.parquet" "$provider/serving/datasets/guided-observations/v1/data.parquet"
cp "$fixtures/raster/temperature.tiff" "$provider/serving/datasets/temperature/v1/temperature.tiff"

cat > "$provider/observations-v1.json" <<'JSON'
{"schema_version":1,"namespace":"climate","product_id":"observations","name":"Demo observations","version":"v1","data_kind":"table","data_format":"parquet","description":"Synthetic Stage-1 table","intended_use":"TUI and SDK demo","limitations":"none","owner_team":"Climate","producer":"Feather Mesh demo","contact":"demo@example.test","usage_policy":"internal","classification":"internal","quality":"production","assets":[{"id":"part-000","path":"datasets/observations/v1/part-000.parquet","role":"data","media_type":"application/vnd.apache.parquet"},{"id":"part-001","path":"datasets/observations/v1/part-001.parquet","role":"data","media_type":"application/vnd.apache.parquet"}],"lineage":[],"table":{"column_meanings":{"station_id":"synthetic station identifier","temperature":"synthetic temperature"},"column_units":{"station_id":"not_applicable","temperature":"celsius"},"partition_columns":[]}}
JSON
sed 's#"version":"v1"#"version":"v2"#; s#observations/v1#observations/v2#g' "$provider/observations-v1.json" > "$provider/observations-v2.json"
cat > "$provider/temperature-v1.json" <<'JSON'
{"schema_version":1,"namespace":"climate","product_id":"temperature","name":"Demo temperature","version":"v1","data_kind":"raster","data_format":"geotiff","description":"Synthetic Stage-1 raster","intended_use":"TUI and Rasterio demo","limitations":"none","owner_team":"Climate","producer":"Feather Mesh demo","contact":"demo@example.test","usage_policy":"internal","classification":"internal","quality":"production","assets":[{"id":"data","path":"datasets/temperature/v1/temperature.tiff","role":"data","media_type":"image/tiff; application=geotiff"}],"lineage":[],"raster":{"datetime":"2026-01-01T00:00:00Z","bbox":[-76.0,45.0,-75.0,46.0],"semantics":{"band_1":"synthetic temperature"}}}
JSON
cat > "$provider/guided-product.json" <<'JSON'
{"schema_version":1,"namespace":"climate","product_id":"guided-observations","name":"Guided observations","version":"v1","data_kind":"table","data_format":"parquet","description":"Guided tutorial table","intended_use":"Disposable guided tutorial practice","limitations":"none","owner_team":"Climate","producer":"Feather Mesh demo","contact":"demo@example.test","usage_policy":"internal","classification":"internal","quality":"production","assets":[{"id":"data","path":"datasets/guided-observations/v1/data.parquet","role":"data","media_type":"application/vnd.apache.parquet"}],"lineage":[],"table":{"column_meanings":{"station_id":"synthetic station identifier","temperature":"synthetic temperature"},"column_units":{"station_id":"not_applicable","temperature":"celsius"},"partition_columns":[]}}
JSON

"$feam" --project "$provider" serve "$provider/serving" --metadata "$provider/observations-v1.json"
"$feam" --project "$provider" serve "$provider/serving" --metadata "$provider/observations-v2.json"
"$feam" --project "$provider" serve "$provider/serving" --metadata "$provider/temperature-v1.json"

"$feam" --project "$client" init --namespace consumer
mkdir -p "$client/peers"
ln -s "$provider/serving" "$client/peers/nearby-climate"
cat > "$client/.feam/project.toml" <<'TOML'
schema_version = 1
namespace = "consumer"
owner_teams = ["consumer"]

[[peers]]
alias = "nearby-climate"
namespace = "climate"
path = "peers/nearby-climate"

[[peers]]
alias = "intentionally-unavailable"
namespace = "unavailable"
path = "peers/missing"
TOML
"$feam" --project "$client" refresh

printf '%s\n' "Stage-1 demo created: $demo_root"
printf '%s\n' "Client project: $client"
printf '%s\n' "Run manual TUI: cargo run -p mesh_cli --features tui -- tui --project '$client' --agent off"
