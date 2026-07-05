# Services

The service layer defines API-style functions that expose key Feather Mesh workflows for `mesh_cli` to call.

```
services/
├── registry_service.rs  # Registry workflows built on repositories
├── requests.rs          # Service request DTOs for CLI-facing workflows
└── mod.rs               # Service module exports
```

Services validate request DTOs, coordinate repositories and models, and return workflow-level results while hiding persistence details from `mesh_cli`.

Keep CLI-facing workflow functions here, not in repositories or raw database code.
