# CALMS Week-44 local preparation evidence

Measured on Mac on 2026-11-14. This record establishes a reviewed local
implementation and real-source reader check. It does not establish Ubuntu
pipeline admission, candidate approval, promotion, participant assignment or
separate-identity filesystem access.

The locked selection contains one current example from each of the six CALMS
source families. Every source ZIP and all 30 flat archive members are pinned by
byte count and SHA-256. The publication profile is the approved Ottawa-area
WGS84 bbox `[-76, 45, -75, 46]`, nearest-neighbour resampling and a maximum
512×512 output. Week-44 observations use the half-open interval
`[2026-10-26T00:00:00Z, 2026-11-02T00:00:00Z)`; the two baselines explicitly
record the 2006–2025 climatology span.

Observed results:

- All six real archives converted to single-band EPSG:4326 GeoTIFFs and retained
  the approved value domains, nodata and scientific time semantics.
- FEAM validated all six metadata records and published them to a disposable
  local `earth-observation` provider manifest.
- A fresh installed Python SDK resolved all six assets with integrity checks.
  Rasterio compared known 2×2 windows against the source rasters, and
  `pystac-client` found all six items over tokenless loopback HTTP.
- The AAFC catalogue validator, four catalogue tests, ten importer tests and all
  three JSON Schema job validations passed.
- `go test -race ./...`, `go vet ./...` and `go build -trimpath ./cmd/...` passed
  in `web_demo/`.

The production pipeline command was also exercised locally. It refused before
reservation or fetch because this Mac had less than the hard 20 GiB production
free-space minimum. That guard remains unchanged. The three repository jobs are
therefore templates: an Ubuntu operator must bind each `parent_digest` to the
then-current `earth-observation` release, prepare and separately approve/promote
each complete candidate in sequence.

The exact source and derivative hashes are retained in
[`calms-week44-mac-real-source-readers.json`](calms-week44-mac-real-source-readers.json).
