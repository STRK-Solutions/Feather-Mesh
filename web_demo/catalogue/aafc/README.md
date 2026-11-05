# AAFC source-product catalogue

This directory describes the *shapes* of Agriculture and Agri-Food Canada
(AAFC) products that the web demo may later prepare.  It is deliberately not a
provider release and none of these files is a `serving/manifest.json`.

The distinction matters:

| File type | Purpose | May name a real mounted asset? |
| --- | --- | --- |
| Source-product catalogue (this directory) | Stable product families, categories, cadence variants, semantics, and source documentation | No |
| FEAM `serving/manifest.json` | The exact, validated versioned asset inventory a participant can resolve | Yes |

Creating a catalogue entry never downloads, converts, publishes, mounts, or
grants access to data.  To add a real demo example, create a bounded pipeline
job that references the catalogue ID, prepare a derivative privately, validate
it through FEAM, and obtain the required candidate-hash approval.  The
pipeline—not a hand-edited JSON file—then creates the authoritative serving
manifest.

## Files

- `catalogue.json` is the root index and records cross-suite semantic concepts.
- `catalogue.json` also assigns suites to two publication peers: `climate`
  contains Agroclimate and NASDI; `earth-observation` contains CALMS.
- `agroclimate.manifest.json`, `nasdi.manifest.json`, and
  `calms.manifest.json` are the three source-suite catalogues.
- `subset-selection.json` locks one exact direct GeoTIFF for every Agroclimate
  and NASDI product, plus the common bounded publication profile. CALMS is not
  included in this selection.
- `build_subset_jobs.py` verifies a private source intake against the locked
  sizes and SHA-256 values, then emits the closed pipeline jobs in batches of
  at most two products.
- `validate_catalogue.py` checks the repository-owned structural invariants
  using only Python's standard library.

Run the validation from the repository root:

```bash
python3 web_demo/catalogue/aafc/validate_catalogue.py
```

The selected national inputs total 41.69 MiB and are transient pipeline
inputs, not demo assets. The 35 publishable outputs are Ottawa-area
`[-76, 45, -75, 46]` WGS84 rasters capped at 512×512 cells. To build jobs from
an already acquired private intake whose filenames equal the selection's
`source_id` values:

```bash
python3 web_demo/catalogue/aafc/build_subset_jobs.py \
  --selection web_demo/catalogue/aafc/subset-selection.json \
  --input-dir /private/intake/aafc \
  --output-dir /private/jobs/aafc-subsets
```

The generated jobs and source intake stay private. On the Ubuntu web-demo
host, process the climate jobs sequentially. The generator deliberately leaves each
`parent_digest` empty because later release digests do not exist yet; before
each submission, the operator must bind that job to the then-current approved
release digest, then review and promote its candidate. A local
direct-converter run is useful for development, but it is not evidence that
the Ubuntu pipeline or promotion gate passed.

To resume the current live `climate` release, exclude its eight already
published product versions. This produces 14 templates containing only the 27
remaining interval products:

```bash
python3 web_demo/catalogue/aafc/build_subset_jobs.py \
  --selection web_demo/catalogue/aafc/subset-selection.json \
  --input-dir /private/intake/aafc \
  --output-dir /private/jobs/aafc-interval-subsets \
  --exclude-product-id blended-drought-indicator-long-term \
  --exclude-product-id blended-drought-indicator-short-term \
  --exclude-product-id difference-from-normal-soil-moisture \
  --exclude-product-id moisture-anomaly-index \
  --exclude-product-id palmer-hydrological-drought-index \
  --exclude-product-id palmer-modified-drought-index \
  --exclude-product-id percent-of-normal-soil-moisture \
  --exclude-product-id total-soil-moisture
```

The exclusion is only a resume aid: it rejects unknown or repeated IDs and
does not withdraw or overwrite anything in the current release.

The generated climate jobs use the `climate` bundle and namespace. CALMS is
reserved for a later archive-unpack stage and its jobs must use the separate
`earth-observation` bundle and namespace; a pipeline job must never mix products from the two
peers.

To browse it in the local Feather Mesh TUI, run from `feather-mesh/` with an
existing FEAM project root:

```bash
cargo run -p mesh_cli --features tui -- tui \
  --project /absolute/path/to/client-project \
  --agent off \
  --source-catalogue ../web_demo/catalogue/aafc/catalogue.json
```

The TUI opens the `Sources` tab. Arrow keys or `j`/`k` select an entry, and
`PgUp`/`PgDn` scroll its details. This is still descriptive browsing only.

## Adding a product later

1. Add the product under the appropriate `categories[].products[]` list with a
   stable lowercase `id`.
2. Give it an honest `publication_readiness` value.  `requires_archive_unpack`
   is a source-preparation requirement; it is not permission to publish an
   archive as a GeoTIFF.
3. Add a cadence variant when the product has independently meaningful monthly,
   weekly, or other temporal forms.
4. Add a cross-suite semantic concept only when it genuinely identifies the
   same scientific measure.  It is a relationship, never a parent-child
   publication relationship.
5. Update the expected counts in `validate_catalogue.py` when the
   specification itself changes, then run the validator.

The supplied data-product specifications remain the source for product
definitions.  Catalogue metadata purposely does not invent an authoritative
release version, checksum, observation time, spatial bounds, or asset path.
