# Feather Mesh

Feather Mesh (`feam`) is an HPC-oriented catalog and data-access layer for
scientific teams. It helps teams publish versioned data products with useful
metadata, discover products through configured project relationships, and
reuse the exact versions they select. The data stays under its provider's
control on the filesystem.

## How it works

1. A provider publishes an explicit inventory and metadata for a product
   version. The provider's `serving/manifest.json` is the authoritative record
   for the project-scoped peer workflow.
2. A client project declares which peer routes it can use. Feather Mesh only
   discovers products registered by those providers and reachable through
   those routes.
3. A consumer resolves a product identifier and pinned version to inspect its
   registered files and metadata. Resolve is a direct read; it does not copy
   the data.
4. If a separate copy is needed, the consumer explicitly stages the product.
   Staging records provenance. Actual access to the files is still governed by
   operating-system and filesystem permissions.

```text
Provider project                         Client project
  data + metadata                           configured peer route
         |                                          |
  serving/manifest.json  <--- discover/resolve ---+
         |                                          |
  provider-owned files  <--- direct read ----------+
                                                    +--- explicit stage ---> client output
```

## Important distinction

The current project-scoped peer workflow and the legacy SQLite registry are
separate:

- `feam --project ROOT ...` uses project configuration and provider manifests.
- `feam --registry PATH ...` selects the legacy SQLite workflow.
- SQLite is not the authority for project-scoped peer products.
- Feather Mesh is not a replacement filesystem, scheduler, or arbitrary
  network file-transfer service.

## Start here

- [Repository orientation map](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/map.md)
  explains terminology, architecture, and how to navigate the code.
- [Workspace README](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/feather-mesh/README.md)
  covers installation and CLI examples.
- [Peer data-access contract](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/data_access_contract.md)
  defines publication, manifests, resolution, and staging.
- [Agent context](agent-context.md) gives AI coding assistants a concise routing
  guide and the project's important behavioral boundaries.
- [Web demo design](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/ubuntu_web_dev_demo_design.md)
  and [web demo workplan](https://github.com/STRK-Solutions/Feather-Mesh/blob/main/docs/ubuntu_web_dev_demo_workplan.md)
  describe the separate, planned browser demo. This documentation site is
  static and does not imply that the web demo has been deployed.

## Current interfaces

The Rust workspace provides the `feam` CLI, shared `mesh_core` services, an
optional terminal UI, and an optional bounded assistant harness. A supported
Python SDK and a derived, loopback-only STAC adapter are also available. These
interfaces use shared core rules where applicable; they do not make separate
catalog authorities.
