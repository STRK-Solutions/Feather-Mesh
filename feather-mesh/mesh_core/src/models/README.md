# Models

The model layer is split by database lifecycle and domain validation concepts.

```
models/
├── metadata/  # Typed metadata values and validation helpers
├── entities/  # Persisted rows returned from the database
├── new/       # Insertable NewX structs used before persistence
└── mod.rs     # Module exports
```

`entities/` contains persisted models such as `DataProduct`, `Metadata`, and `Team`. These include database-managed fields like primary keys and `created_at` timestamps.

`new/` contains insert models such as `NewDataProduct`, `NewMetadata`, and `NewTeam`. These exclude database-managed fields and are intended for database insertion.

`metadata/` contains typed metadata values such as `AssetType`, `DataQuality`, and `AccessClassification`, plus validation helpers used before persistence.
