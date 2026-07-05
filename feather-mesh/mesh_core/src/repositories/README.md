# Repositories

The repository layer owns database access and row mapping.

```
repositories/
├── *_repository.rs  # SQL queries and row-to-model mapping
└── mod.rs           # Repository module exports
```

Repositories accept `NewX` insert models, execute SQL using a `rusqlite::Connection`, and return persisted entity models. They are also the boundary where typed domain values are converted to and from SQLite text columns.

Keep SQL and database row mapping here, not in `models/` or `services/`.
