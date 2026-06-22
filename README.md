<p align="center">
    <img src="./imgs/logo.png" width="400">
</p>

# Feather Mesh

This STRK-Solutions repository contains the capstone work for Feather Mesh, an HPC-oriented data catalog / data mesh project. The repo includes an MVP Python CLI, a Rust workspace for the Feather Mesh implementation, supporting diagrams, and project documentation.

This root README is intentionally high level. For setup, usage, and implementation details, use the README inside each subproject directory.

## Repository Overview

### `python_mvp/`

Contains the Python MVP for the data registry CLI. This is the lighter-weight prototype used to model registry behavior such as serving, searching, and inspecting mock data products.

Start here if you want to:

- run the MVP CLI quickly
- inspect the mock registry data and Python packaging setup
- understand the early prototype workflow

See [`python_mvp/README.md`](/python_mvp/README.md) for installation, commands, and usage examples.

### `feather-mesh/`

Contains the Rust workspace for Feather Mesh, the HPC-native middleware layer for publishing, discovering, and consuming reusable data products. This area holds the main systems-oriented implementation work.

Key parts of this directory include:

- `mesh_core/` for the shared Rust library and core domain logic
- `mesh_cli/` for the command-line interface built on top of the core library

See [`feather-mesh/README.md`](/feather-mesh/README.md) for workspace layout, crate boundary notes, build guidance, and the `cargo test` command.

### `diagrams/`

Stores architecture, schema, and workflow visuals used to explain the system design and project direction. These files are useful for understanding the conceptual model without digging into code first.

### `imgs/`

Holds image assets used by the documentation and project branding, including the Feather Mesh logo files referenced by README content.

### `.github/`

Contains GitHub-specific project automation, currently including workflow configuration for repository checks or CI-related tasks.

### `proposal.md`

Contains project-level written documentation separate from the implementation directories. Use this for broader context on the capstone proposal and framing.

### `Feather_Mesh_PDD_Revised.pdf`

The product definition document, outlining an in-depth overview of the Feather Mesh product.

## Where To Go Next

- If you want the Python prototype, start in [`python_mvp/README.md`](/python_mvp/README.md).
- If you want the Rust implementation, start in [`feather-mesh/README.md`](/feather-mesh/README.md).
- If you want architecture context, browse `diagrams/`, `proposal.md`, `Feather_Mesh_PDD_Revised.pdf`, and [`feather-mesh/README.md`](/feather-mesh/README.md).
