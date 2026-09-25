# W4 bounded source acquisition proposal

Prepared on 2026-09-25. **Bounded acquisition approved and completed; release
promotion remains unapproved.** The following table retains the pre-acquisition
proposal and observed HEAD metadata. Exact acquired and derived hashes are in the
[MAC reader receipt](w4-mac-real-source-readers.json) and
[private candidate review](w4-mac-candidate-review.json). The real pair was
converted and read privately on Mac; that evidence does not establish Ubuntu
manager admission, publication or participant access.

| Input | Proposed object and scientific subset | Bound and observed metadata |
| --- | --- | --- |
| ECCC daily table | [Exact 2023 Ottawa CSV request](https://climate.weather.gc.ca/climate_data/bulk_data_e.html?format=csv&stationID=49568&Year=2023&Month=1&Day=1&timeframe=2&submit=Download+Data). Station `49568`, Climate ID `6106001`, `OTTAWA INTL A`; 2023-01-01 through 2023-12-31 inclusive. Preserve maximum/minimum/mean temperature, total precipitation, every corresponding quality flag and numeric null. | One request, maximum 2 MiB, 60 seconds. HEAD returned HTTP 200 and attachment `en_climate_daily_ON_6106001_2023_P1D.csv`; byte size and 365-row completeness remain unverified. |
| AAFC raster | [January 1991–2020 maximum-temperature normal](https://agriculture.canada.ca/atlas/data_donnees/climateNormals/data_donnees/tif/monthly/lt_tx/lt_tx_m01_1991_2020.tif). Prepare Ottawa-area WGS84 bbox `[-76.0,45.0,-75.0,46.0]`; at most 512×512 cells, nearest-neighbour resampling with explicit source/target transform, CRS and nodata provenance. | One request, maximum 40 MiB, 120 seconds. HEAD returned HTTP 200, `Content-Length: 38421434`, `Last-Modified: Thu, 28 Mar 2024 16:21:49 GMT`, ETag `"24a43ba-614baeb98bde0"`. Actual format, nodata, source dimensions, units and subset coverage remain unverified until inspection. |

Total acquisition is limited to two files, **42 MiB**, no archives, no credentials,
no source-provided executable content, at most one separately checked HTTPS
redirect to the same approved official source, and 180 seconds total. Abort
partial/oversize downloads. Retain raw inputs privately; publish only derived
Parquet and `.tiff` assets. The proposed complete published bundle remains
limited to **250 MiB**. An exact byte/hash report precedes release approval.

The [official station metadata](https://api.weather.gc.ca/collections/climate-stations/items?STN_ID=49568&limit=1&f=json)
returned station `49568` / Climate ID `6106001`, coordinates
`[-75.66666666666667,45.31666666666667]`, and daily availability beginning
2011-12-15 and ending 2026-09-22. Availability boundaries do not prove there
are no gaps in 2023; conversion verifies the requested daily interval.

The [AAFC catalog](https://open.canada.ca/data/en/dataset/b3fda923-a875-4dde-bb67-873f9684a031)
links its [exact object directory](https://agriculture.canada.ca/atlas/data_donnees/climateNormals/data_donnees/tif/monthly/lt_tx/).
Its January 1991–2020 normal is an aggregate, **not an instantaneous observation**.
Record `datetime: null`, `start_datetime: 1991-01-01T00:00:00Z` and
`end_datetime: 2020-12-31T23:59:59Z`, together with `climatology_month: January`
and the source-defined normal statistic. The acquisition/modification timestamps
are provenance only. Additive Core/CLI/STAC interval support is implemented and
validated with the installed SDK, a real standard client, pinned STAC schema,
inclusive interval search and collection extents. No observation instant is
substituted for this climatology interval.

Both source catalogs identify the [Open Government Licence – Canada](https://open.canada.ca/en/open-government-licence-canada).
Retain attribution to Environment and Climate Change Canada and Agriculture and
Agri-Food Canada, source links, retrieval UTC, upstream identity, source/output
SHA-256, conversion version and scientific limitations. Verify source-specific
metadata against the acquired bytes before approval. No endorsement is claimed.

The previously approved intended audience remains all invited regular users in
the `demo-climate` bundle. Public descriptive metadata may be disclosed to the
selected model; raw data payloads are not automatically disclosed. Research
eligibility remains subject to the existing consent, sanitization and export
review policy. Acquisition approval does not itself approve release promotion,
participant assignments or a public endpoint.

Read-only evidence retained: browser opens of the catalogs/directories returned
403/inaccessible; bounded curl metadata/HEAD requests subsequently succeeded.
The daily-temperature directory exploration stopped at its 512 KiB cap (curl
exit 63); no raster bytes were fetched. A smaller dated AAFC maximum-temperature
series exists, but its 24-hour aggregation also needs truthful interval semantics;
the original normals choice is retained.

The approved download completed with two regular files and no redirects or
archives: ECCC 64,158 bytes and AAFC 38,421,434 bytes. The native source inspection
found EPSG:4269, one float32 band, a 3,600×6,960 source grid and explicit nodata.
The EPSG:4326 nearest-neighbour derivative is 512×512; four sampled cells match
the corresponding source cells. The table preserves all 365 daily rows and
quality flags. Registered assets total 62,046 bytes; the private Mac candidate
totals 76,808 bytes. Its manifest timestamps and exact candidate hash are Mac
evidence; Ubuntu preparation requires a fresh exact-hash approval. The Mac
production manager refused insufficient filesystem headroom, as designed.
