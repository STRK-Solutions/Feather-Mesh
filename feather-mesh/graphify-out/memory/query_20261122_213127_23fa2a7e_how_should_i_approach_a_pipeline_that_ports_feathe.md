---
type: "query"
date: "2026-11-22T21:31:27.853753+00:00"
question: "How should I approach a pipeline that ports Feather Mesh and the web-demo data manifests to an HPC cluster?"
contributor: "graphify"
outcome: "useful"
source_nodes: ["CatalogueManifest", "Filesystem", "Staging", "Cache", "Asset", "Publication", "Project", "Peer"]
---

# Q: How should I approach a pipeline that ports Feather Mesh and the web-demo data manifests to an HPC cluster?

## Answer

Expanded from the original query via graph vocab: asset, cache, catalog, catalogue, data, filesystem, job, manifest, peer, project, publication, staging. The graph shows that the Rust workspace already owns project-scoped peer access, publication, resolution, staging, cache behavior, Python SDK access, and descriptive source-catalogue loading. The source catalogue is descriptive only and cannot claim publication authority. The recommended first HPC slice is a scheduler-invoked single-node workflow on the target shared filesystem that consumes a reviewed dataset job, uses a private pinned-source inbox, prepares and validates through FEAM, promotes an immutable release, and is then tested from separate producer and consumer identities and from two nodes. The web control plane and persistent services should remain a later site-specific layer.

## Outcome

- Signal: useful

## Source Nodes

- CatalogueManifest
- Filesystem
- Staging
- Cache
- Asset
- Publication
- Project
- Peer