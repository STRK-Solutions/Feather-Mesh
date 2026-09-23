//! Bounded, manifest-backed catalog discovery shared by the TUI and agent.
//!
//! The source of truth remains provider manifests. This module intentionally
//! does not name cache files or SQLite tables so a later catalog index can
//! implement the same query/result contract.

use std::fmt::Write as _;

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use super::peer_access::{
    DataFormat, DataKind, PeerCoverage, PeerError, PeerResult, list_registered_with_coverage,
};
use super::peer_access::{Lifecycle, Project, reference};

pub const CATALOG_PROTOCOL_VERSION: &str = "feam.catalog.v1";
pub const MAX_CATALOG_PAGE_SIZE: usize = 100;

fn default_limit() -> usize {
    25
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CatalogQuery {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub namespace: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub data_kind: Option<DataKind>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub data_format: Option<DataFormat>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub quality: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub owner_team: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub classification: Option<String>,
    #[serde(default = "default_limit")]
    pub limit: usize,
    /// Opaque cursor returned by a previous page from the same snapshot.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cursor: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CatalogEntry {
    pub reference: String,
    pub version: String,
    pub name: String,
    pub manifest_revision: u64,
    pub data_kind: DataKind,
    pub data_format: DataFormat,
    pub description: String,
    pub limitations: String,
    pub owner_team: String,
    pub quality: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CatalogPage {
    pub protocol: String,
    pub entries: Vec<CatalogEntry>,
    pub coverage: Vec<PeerCoverage>,
    pub snapshot_fingerprint: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub next_cursor: Option<String>,
    pub result_limit: usize,
    /// True when more matching records existed than the requested page allows.
    pub truncated: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct CatalogCursor {
    snapshot_fingerprint: String,
    after_reference: String,
    after_version: String,
}

/// Finds active, registered versions with deterministic ordering and explicit
/// coverage. It reads live manifests, so a cursor becomes invalid when a peer
/// revision or route availability changes; callers restart rather than mixing
/// pages from different views.
pub fn query_catalog(project: &Project, query: CatalogQuery) -> PeerResult<CatalogPage> {
    if query.limit == 0 || query.limit > MAX_CATALOG_PAGE_SIZE {
        return Err(PeerError::Validation {
            field: "limit".into(),
            message: format!("must be between 1 and {MAX_CATALOG_PAGE_SIZE}"),
        });
    }
    let listing = list_registered_with_coverage(project)?;
    let mut bound_query = query.clone();
    bound_query.cursor = None;
    let fingerprint = format!(
        "{:x}",
        Sha256::digest(serde_json::to_vec(&(
            snapshot_fingerprint(&listing.coverage),
            super::peer_access::configuration_fingerprint(project)?,
            project.root(),
            bound_query,
        ))?)
    );
    let cursor = query.cursor.as_deref().map(decode_cursor).transpose()?;
    if let Some(cursor) = &cursor
        && cursor.snapshot_fingerprint != fingerprint
    {
        return Err(PeerError::Conflict(
            "catalog changed between pages; restart the query".into(),
        ));
    }

    let needle = query.text.as_deref().map(str::to_ascii_lowercase);
    let mut matches = Vec::new();
    for (namespace, product, revision) in listing.entries {
        for version in product.versions {
            if version.lifecycle != Lifecycle::Active
                || !matches_query(
                    &namespace,
                    &product.id,
                    &product.name,
                    &version,
                    &query,
                    needle.as_deref(),
                )
            {
                continue;
            }
            matches.push(CatalogEntry {
                reference: reference(&namespace, &product.id),
                version: version.version,
                name: product.name.clone(),
                manifest_revision: revision,
                data_kind: version.data_kind,
                data_format: version.data_format,
                description: version.description,
                limitations: version.limitations,
                owner_team: version.owner_team,
                quality: version.quality,
            });
        }
    }
    matches.sort_by(|left, right| {
        (&left.reference, &left.version, &left.name).cmp(&(
            &right.reference,
            &right.version,
            &right.name,
        ))
    });
    if let Some(cursor) = cursor {
        matches.retain(|entry| {
            (&entry.reference, &entry.version) > (&cursor.after_reference, &cursor.after_version)
        });
    }

    let truncated = matches.len() > query.limit;
    let entries: Vec<_> = matches.into_iter().take(query.limit).collect();
    let next_cursor = truncated.then(|| {
        let last = entries
            .last()
            .expect("truncated catalog page has one entry");
        encode_cursor(&CatalogCursor {
            snapshot_fingerprint: fingerprint.clone(),
            after_reference: last.reference.clone(),
            after_version: last.version.clone(),
        })
    });
    Ok(CatalogPage {
        protocol: CATALOG_PROTOCOL_VERSION.into(),
        entries,
        coverage: listing.coverage,
        snapshot_fingerprint: fingerprint,
        next_cursor,
        result_limit: query.limit,
        truncated,
    })
}

pub(crate) fn matches_query(
    namespace: &str,
    product_id: &str,
    name: &str,
    version: &super::peer_access::PublishedVersion,
    query: &CatalogQuery,
    needle: Option<&str>,
) -> bool {
    if query
        .namespace
        .as_deref()
        .is_some_and(|value| value != namespace)
        || query
            .data_kind
            .is_some_and(|value| value != version.data_kind)
        || query
            .data_format
            .is_some_and(|value| value != version.data_format)
        || query
            .quality
            .as_deref()
            .is_some_and(|value| value != version.quality)
        || query
            .owner_team
            .as_deref()
            .is_some_and(|value| value != version.owner_team)
        || query
            .classification
            .as_deref()
            .is_some_and(|value| value != version.classification)
    {
        return false;
    }
    needle.is_none_or(|needle| {
        format!(
            "{namespace} {product_id} {name} {} {} {}",
            version.version, version.description, version.owner_team
        )
        .to_ascii_lowercase()
        .contains(needle)
    })
}

fn snapshot_fingerprint(coverage: &[PeerCoverage]) -> String {
    let mut rows: Vec<_> = coverage
        .iter()
        .map(|coverage| {
            format!(
                "{}|{:?}|{}|{}",
                coverage.namespace,
                coverage.revision,
                coverage.available,
                coverage.error.as_deref().unwrap_or_default()
            )
        })
        .collect();
    rows.sort();
    let mut hash = Sha256::new();
    for row in rows {
        hash.update(row.as_bytes());
        hash.update([0]);
    }
    format!("{:x}", hash.finalize())
}

fn encode_cursor(cursor: &CatalogCursor) -> String {
    let bytes = serde_json::to_vec(cursor).expect("catalog cursor serializes");
    let mut encoded = String::with_capacity(3 + bytes.len() * 2);
    encoded.push_str("v1:");
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}").expect("writing to String cannot fail");
    }
    encoded
}

fn decode_cursor(value: &str) -> PeerResult<CatalogCursor> {
    if value.len() > 4096 || !value.is_ascii() {
        return Err(PeerError::Validation {
            field: "cursor".into(),
            message: "invalid or oversized cursor".into(),
        });
    }
    let encoded = value
        .strip_prefix("v1:")
        .ok_or_else(|| PeerError::Validation {
            field: "cursor".into(),
            message: "unsupported cursor version".into(),
        })?;
    if encoded.len() % 2 != 0 {
        return Err(PeerError::Validation {
            field: "cursor".into(),
            message: "malformed cursor".into(),
        });
    }
    let bytes: Result<Vec<_>, _> = (0..encoded.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&encoded[index..index + 2], 16))
        .collect();
    let bytes = bytes.map_err(|_| PeerError::Validation {
        field: "cursor".into(),
        message: "malformed cursor".into(),
    })?;
    serde_json::from_slice(&bytes).map_err(|_| PeerError::Validation {
        field: "cursor".into(),
        message: "malformed cursor".into(),
    })
}

/// CLI product grouping preserves the peer protocol while sharing all filters
/// with version pages. Withdrawn versions never produce empty product rows.
pub fn query_products(
    project: &Project,
    query: &CatalogQuery,
) -> PeerResult<Vec<serde_json::Value>> {
    let needle = query.text.as_deref().map(str::to_ascii_lowercase);
    let mut products = Vec::new();
    for (namespace, product, revision) in list_registered_with_coverage(project)?.entries {
        let versions: Vec<_> = product
            .versions
            .iter()
            .filter(|v| {
                v.lifecycle == Lifecycle::Active
                    && matches_query(
                        &namespace,
                        &product.id,
                        &product.name,
                        v,
                        query,
                        needle.as_deref(),
                    )
            })
            .map(|v| v.version.clone())
            .collect();
        if !versions.is_empty() {
            products.push(serde_json::json!({"reference": reference(&namespace, &product.id), "name": product.name,
                "versions": versions, "manifest_revision": revision}));
        }
    }
    products.sort_by_key(|p| p["reference"].as_str().unwrap_or_default().to_owned());
    Ok(products)
}

pub fn catalog_teams(project: &Project) -> PeerResult<std::collections::BTreeSet<String>> {
    Ok(list_registered_with_coverage(project)?
        .entries
        .into_iter()
        .flat_map(|(_, p, _)| p.versions)
        .filter(|v| v.lifecycle == Lifecycle::Active)
        .map(|v| v.owner_team)
        .collect())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cursor_round_trips_without_exposing_json_to_callers() {
        let original = CatalogCursor {
            snapshot_fingerprint: "f".into(),
            after_reference: "product://climate/observations".into(),
            after_version: "v1".into(),
        };
        let encoded = encode_cursor(&original);
        assert!(encoded.starts_with("v1:"));
        assert_eq!(decode_cursor(&encoded).unwrap().after_version, "v1");
    }
}
