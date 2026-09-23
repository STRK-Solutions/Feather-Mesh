//! Public peer-data access DTOs and services.
//!
//! Implementation remains in `services::peer_access`; this facade keeps the
//! adapter-facing API separate from the legacy SQLite service names.

pub use crate::services::peer_access::*;
