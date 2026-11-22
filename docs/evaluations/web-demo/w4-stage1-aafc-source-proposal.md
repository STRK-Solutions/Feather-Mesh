# W4 Stage 1 AAFC direct-GeoTIFF source proposal

Prepared on 2026-10-28. This is a source-selection and private-acquisition
record, not a release approval, production job, participant grant, or evidence
of Ubuntu deployment acceptance.

Stage 1 adds three small, direct AAFC GeoTIFF derivatives to the `climate`
peer. The pipeline fetches the exact objects into private
staging, verifies their SHA-256 values, creates a bounded Ottawa-area WGS84
derivative, validates it through FEAM, and publishes it only after a separate
exact-candidate-hash approval. Raw inputs, job drafts, source reports and
provenance remain outside the mounted `serving/` tree.

## Approved source candidates

| Product ID | Exact source | Private intake SHA-256 | Bytes | Scientific time | Published semantics |
| --- | --- | --- | ---: | --- | --- |
| `aafc-spi-30d` | [SPI 30-day, 2026-10-19](https://agriculture.canada.ca/atlas/data_donnees/agClimate/data_donnees/tif/spi/30/spi_30_20261019.tif) | `335be1d814c841ab94d34e0eb683eef29b4e67dc2f1ba90e12d7b36c7a3c13f2` | 340,277 | 30-day interval ending 2026-10-19; the exact UTC bounds are confirmed during operator review from the AAFC period definition. | Standardized Precipitation Index; dimensionless standard deviations relative to the long-term mean. |
| `nasdi-monthly-precipitation` | [NASDI accumulated precipitation, 2026-09](https://agriculture.canada.ca/atlas/data_donnees/nasdi/data_donnees/monthly/accumulated_precipitation/nasdi_pcp_mm_202609.tif) | `32a5afac16739fc97d350a94ecbd5aafaadd145346af84e3af5970dff4bca196` | 3,529,633 | 2026-09-01 through 2026-09-30 UTC, pending source-tag confirmation during conversion review. | Accumulated precipitation, millimetres. |
| `nasdi-weekly-precipitation` | [NASDI accumulated precipitation, 2026-W42](https://agriculture.canada.ca/atlas/data_donnees/nasdi/data_donnees/weekly/accumulated_precipitation/nasdi_pcp_mm_2026W42.tif) | `6c95e8d7959cb7252e80aa66cff44fbf214cd613af1777ac0c2e521840899cfa` | 3,634,851 | ISO week 42, 2026: 2026-10-12 through 2026-10-18 UTC, pending source-tag confirmation during conversion review. | Accumulated precipitation, millimetres. |

All three sources use the [Open Government Licence – Canada](https://open.canada.ca/en/open-government-licence-canada) and attribute Agriculture and Agri-Food Canada. The SPI specification describes a 30-day period code as the thirty days prior to the raster date; it must remain an interval rather than be labelled as an instantaneous measurement.

## Bounds and safety limits

- Source files are fetched only by the private pipeline identity from the exact
  HTTPS URLs above; no redirects to another host, archives, credentials or
  unpinned source versions are permitted.
- The Ottawa-area WGS84 subset is `[-76.0, 45.0, -75.0, 46.0]`, resampled only
  with nearest-neighbour into a maximum 512×512 output grid.
- Each derivative is a demonstrative subset, not a national or scientific
  replacement product. Its metadata must say so.
- Direct-source maximums are 1 MiB for SPI and 4 MiB for each NASDI source;
  the candidate release uses the existing 250 MiB ceiling but should remain
  comfortably below 10 MiB.
- Native inspection observed one float32 band and explicit nodata in every
  selected file. SPI is a 1,872×929 Web-Mercator source at 5 km. The two NASDI
  sources are 935×824 10 km sources. The converter must still perform its normal
  GDAL/Rasterio CRS and bounds validation before publication.

## Release sequencing

The closed job contract admits one or two sources per job. Prepare an approved
successor of the current `climate` release in two jobs:

1. job A: `aafc-spi-30d` and `nasdi-monthly-precipitation`;
2. job B: copies the released job-A snapshot and adds
   `nasdi-weekly-precipitation`.

Both use importer version `2`, which adds regular scientific intervals while
preserving version-1 job semantics. CALMS remains Stage 2: its selected
Best-Quality Maximum-NDVI source is an archive and is deliberately excluded from
this direct-GeoTIFF proposal.

## Required evidence before promotion

1. A reviewer confirms the exact interval boundaries, CRS transform, nodata,
   output source-cell samples, output hashes and licence/attribution against the
   private inputs.
2. `feam validate-metadata` and `feam serve` succeed for every product, yielding
   one exact manifest inventory.
3. A fresh client route resolves each registered product and Rasterio reads a
   known window with expected CRS, nodata and pixels.
4. An administrator approves the exact candidate tree hash before promotion.
5. Ubuntu-only acceptance verifies the mounted serving tree is read-only and
   that raw inputs/reports/provenance are unreadable from participant sandboxes.
