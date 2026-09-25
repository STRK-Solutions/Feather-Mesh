# Services

The service layer owns reusable Feather Mesh workflows. CLI, TUI, SDK, and STAC
surfaces must translate to these rules rather than create competing publication,
visibility, or mutation behavior.

```text
services/
├── peer_access.rs             # Project, manifest, validation, resolve, and stage rules
├── catalog_service.rs         # Project-scoped catalog queries and peer coverage
├── interactive_operations.rs # Prepared/rechecked interactive mutations
├── operation_journal.rs       # Minimal private recovery records
├── registry_service.rs        # Legacy SQLite registry workflows
├── requests.rs                # Legacy registry request DTOs
└── mod.rs                     # Module exports
```

The peer workflow treats provider manifests as authoritative and does not use
legacy SQLite as a publication source. The legacy registry service coordinates
repositories and models while hiding persistence details from `mesh_cli`.

Keep workflow rules here, SQL and row mapping in `repositories/`, reusable data
types in the appropriate core modules, and terminal formatting/process behavior
in `mesh_cli`.
