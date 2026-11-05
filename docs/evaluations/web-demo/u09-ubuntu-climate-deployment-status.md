# Ubuntu climate-data deployment status

Status as of 2026-10-31.

## Purpose

Make small, real AAFC climate raster samples available in the Ubuntu web-demo
workspaces. Do not publish full national datasets or raw pipeline inputs.

## Already live on Ubuntu

- The installed version-1 pipeline prepared, reviewed, approved, and promoted
  four cumulative two-product jobs.
- The current `climate` release contains eight bounded Ottawa-area WGS84
  GeoTIFF derivatives (236,795 bytes total): drought/soil-moisture products
  with truthful instant timestamps.
- The exact release digest is
  `01820c04781a72b9d3dd22492b8e2dafbb033dcc685b38eee41ad80f879f00a5`.
- It is assigned and verified as ready in all seven existing participant
  workspaces. The older `demo-climate` grant was retained.
- Raw selected sources and conversion reports stay private; participants see
  only the approved release's serving subtree.

## Prepared locally, not yet deployed to Ubuntu

- AAFC catalogue/manifests describe 35 climate products: 21 Agroclimate and
  14 NASDI. CALMS remains deferred under `earth-observation`.
- Fourteen hash-pinned, two-product-or-fewer jobs are ready for the remaining
  27 climate products. They omit the eight already live products.
- Version-2 pipeline/converter support preserves normal weekly/monthly/SPI
  periods as truthful `start_datetime`/`end_datetime` intervals.
- Local Go race tests, converter tests, catalogue tests, and offline Ansible
  syntax/lint checks passed. This is not Ubuntu deployment evidence.

## Remaining Ubuntu work

1. Restore private deployment prerequisites on the Mac: the Ubuntu inventory,
   SSH key, and existing private service-variable/configuration files.
2. Build the reviewed Go services natively on Ubuntu and install the reviewed
   version-2 pipeline binary plus Python converter through the existing Ansible
   deployment path.
3. Verify that a version-2 interval job is accepted by the live pipeline.
4. Process the 14 remaining jobs sequentially: bind each to the latest release,
   prepare, review the exact candidate, approve, and promote.
5. Verify the final 35-product `climate` release. Obtain explicit approval
   before replacing the seven workspaces' current eight-product grant with the
   successor release.

The private prepared operator bundle is intentionally ignored by Git at
`.local/demo-deployment/pipeline-v2/`. See [the pipeline guide](../../../web_demo/PIPELINE.md)
and [the Ubuntu participant-services runbook](../../../infra/demo/W7.md) for
the deployed workflow.
