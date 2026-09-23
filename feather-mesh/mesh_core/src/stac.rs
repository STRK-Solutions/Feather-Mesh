//! Derived STAC 1.1.0 projection for registered raster versions.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use serde_json::{Map, Value, json};

use crate::peer::{
    DataKind, Lifecycle, PeerError, PeerResult, Project, ResolvedProduct, list_registered,
    reference, resolve,
};

pub const STAC_VERSION: &str = "1.1.0";
pub const STAC_API_VERSION: &str = "1.0.0";
pub const CONFORMANCE_CLASSES: &[&str] = &[
    "https://api.stacspec.org/v1.0.0/core",
    "https://api.stacspec.org/v1.0.0/collections",
    "https://api.stacspec.org/v1.0.0/ogcapi-features",
    "https://api.stacspec.org/v1.0.0/item-search",
    "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
    "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
];

#[derive(Debug, Clone)]
pub struct StacItem {
    pub id: String,
    pub collection_id: String,
    pub namespace: String,
    pub product_id: String,
    pub version: String,
    pub manifest_revision: u64,
    pub bbox: [f64; 4],
    pub datetime: String,
    pub value: Value,
}

pub fn collection_id(namespace: &str, product_id: &str) -> String {
    format!("{namespace}--{product_id}")
}

pub fn item_id(namespace: &str, product_id: &str, version: &str) -> String {
    format!("{namespace}--{product_id}--{version}")
}

pub fn collections(project: &Project) -> PeerResult<Vec<Value>> {
    let mut values = BTreeMap::new();
    for item in items(project)? {
        values.entry(item.collection_id.clone()).or_insert_with(|| {
            json!({
                "stac_version": STAC_VERSION,
                "type": "Collection",
                "id": item.collection_id,
                "title": format!("{} raster product", item.product_id),
                "description": format!("Registered raster product product://{}/{}", item.namespace, item.product_id),
                "license": "proprietary",
                "extent": {"spatial": {"bbox": [item.bbox]}, "temporal": {"interval": [[item.datetime, item.datetime]]}},
                "links": [],
                "summaries": {"feam:namespace": [item.namespace], "feam:product_id": [item.product_id]}
            })
        });
    }
    Ok(values.into_values().collect())
}

pub fn items(project: &Project) -> PeerResult<Vec<StacItem>> {
    let mut result = Vec::new();
    for (namespace, product, revision) in list_registered(project)? {
        for version in product.versions {
            if version.lifecycle != Lifecycle::Active || version.data_kind != DataKind::Raster {
                continue;
            }
            let resolved = match resolve(
                project,
                &reference(&namespace, &product.id),
                &version.version,
                None,
                false,
            ) {
                Ok(resolved) => resolved,
                // A removed/broken route is not discoverable through this projection.
                Err(PeerError::PeerUnavailable(_)) | Err(PeerError::Withdrawn(_)) => continue,
                Err(err) => return Err(err),
            };
            result.push(item_from_resolved(resolved, revision)?);
        }
    }
    result.sort_by(|left, right| left.id.cmp(&right.id));
    Ok(result)
}

pub fn item_from_resolved(
    resolved: ResolvedProduct,
    manifest_revision: u64,
) -> PeerResult<StacItem> {
    let descriptor = resolved
        .raster
        .as_ref()
        .ok_or_else(|| PeerError::Validation {
            field: "data_kind".into(),
            message: "STAC projection requires a raster descriptor".into(),
        })?;
    let id = item_id(&resolved.namespace, &resolved.product_id, &resolved.version);
    let collection = collection_id(&resolved.namespace, &resolved.product_id);
    let mut assets = Map::new();
    for asset in &resolved.assets {
        assets.insert(
            asset.id.clone(),
            json!({
                "href": file_uri(&asset.project_access_path),
                "type": asset.media_type,
                "roles": [asset.role],
                "title": asset.id,
                "feam:asset_id": asset.id,
                "feam:product": reference(&resolved.namespace, &resolved.product_id),
                "feam:version": resolved.version,
            }),
        );
    }
    let [west, south, east, north] = descriptor.bbox;
    let geometry = json!({
        "type": "Polygon",
        "coordinates": [[[west, south], [east, south], [east, north], [west, north], [west, south]]]
    });
    let value = json!({
        "stac_version": STAC_VERSION,
        "type": "Feature",
        "id": id,
        "collection": collection,
        "bbox": descriptor.bbox,
        "geometry": geometry,
        "properties": {
            "datetime": descriptor.datetime,
            "proj:shape": [descriptor.height, descriptor.width],
            "proj:epsg": descriptor.crs.as_deref().and_then(|crs| crs.strip_prefix("EPSG:")).and_then(|code| code.parse::<u16>().ok()),
            "feam:native_crs": descriptor.crs,
            "raster:bands": [{"data_type": descriptor.color_type, "nodata": descriptor.nodata}],
            "feam:namespace": resolved.namespace,
            "feam:product_id": resolved.product_id,
            "feam:version": resolved.version,
            "feam:manifest_revision": manifest_revision,
            "feam:owner_team": resolved.owner_team,
            "feam:producer": resolved.producer,
            "feam:usage_policy": resolved.usage_policy,
            "feam:lineage": resolved.lineage,
        },
        "assets": assets,
        "links": []
    });
    validate_item(&value)?;
    Ok(StacItem {
        id,
        collection_id: collection,
        namespace: resolved.namespace,
        product_id: resolved.product_id,
        version: resolved.version,
        manifest_revision,
        bbox: descriptor.bbox,
        datetime: descriptor.datetime.clone(),
        value,
    })
}

/// Local, pinned profile validation. It verifies the fields that the supported
/// STAC 1.1.0 profile emits before a schema validator/client is involved.
pub fn validate_item(item: &Value) -> PeerResult<()> {
    let object = item
        .as_object()
        .ok_or_else(|| invalid("item", "must be an object"))?;
    required_string(object, "stac_version")?;
    if object.get("stac_version").and_then(Value::as_str) != Some(STAC_VERSION) {
        return Err(invalid("stac_version", "must be 1.1.0"));
    }
    if object.get("type").and_then(Value::as_str) != Some("Feature") {
        return Err(invalid("type", "must be Feature"));
    }
    required_string(object, "id")?;
    required_string(object, "collection")?;
    if object
        .get("bbox")
        .and_then(Value::as_array)
        .is_none_or(|bbox| bbox.len() != 4)
    {
        return Err(invalid("bbox", "must contain four coordinates"));
    }
    let properties = object
        .get("properties")
        .and_then(Value::as_object)
        .ok_or_else(|| invalid("properties", "must be an object"))?;
    required_string(properties, "datetime")?;
    if object
        .get("geometry")
        .and_then(Value::as_object)
        .and_then(|geometry| geometry.get("type"))
        .and_then(Value::as_str)
        != Some("Polygon")
    {
        return Err(invalid("geometry", "must be a polygon"));
    }
    if object
        .get("assets")
        .and_then(Value::as_object)
        .is_none_or(Map::is_empty)
    {
        return Err(invalid("assets", "must contain a registered asset"));
    }
    Ok(())
}

pub fn file_uri(path: &Path) -> String {
    let raw = path.to_string_lossy();
    let mut encoded = String::new();
    for byte in raw.as_bytes() {
        if byte.is_ascii_alphanumeric() || matches!(*byte, b'-' | b'.' | b'_' | b'~' | b'/') {
            encoded.push(*byte as char);
        } else {
            use std::fmt::Write;
            let _ = write!(encoded, "%{byte:02X}");
        }
    }
    format!("file://{encoded}")
}

pub fn path_from_file_uri(value: &str) -> PeerResult<PathBuf> {
    let encoded = value
        .strip_prefix("file://")
        .ok_or_else(|| invalid("asset.href", "must be a file URI"))?;
    let mut bytes = Vec::with_capacity(encoded.len());
    let source = encoded.as_bytes();
    let mut index = 0;
    while index < source.len() {
        if source[index] == b'%' {
            if index + 2 >= source.len() {
                return Err(invalid(
                    "asset.href",
                    "contains an incomplete percent escape",
                ));
            }
            let high = hex(source[index + 1])
                .ok_or_else(|| invalid("asset.href", "contains an invalid percent escape"))?;
            let low = hex(source[index + 2])
                .ok_or_else(|| invalid("asset.href", "contains an invalid percent escape"))?;
            bytes.push(high * 16 + low);
            index += 3;
        } else {
            bytes.push(source[index]);
            index += 1;
        }
    }
    let decoded = String::from_utf8(bytes).map_err(|_| invalid("asset.href", "is not UTF-8"))?;
    let path = PathBuf::from(decoded);
    if !path.is_absolute() {
        return Err(invalid("asset.href", "must contain an absolute local path"));
    }
    Ok(path)
}

pub fn search(
    items: &[StacItem],
    collection: Option<&str>,
    bbox: Option<[f64; 4]>,
    datetime: Option<&str>,
) -> Vec<StacItem> {
    items
        .iter()
        .filter(|item| {
            collection.is_none_or(|collection| item.collection_id == collection)
                && bbox.is_none_or(|query| intersects(item.bbox, query))
                && datetime.is_none_or(|query| matches_datetime(&item.datetime, query))
        })
        .cloned()
        .collect()
}

fn intersects(
    [left, bottom, right, top]: [f64; 4],
    [q_left, q_bottom, q_right, q_top]: [f64; 4],
) -> bool {
    left <= q_right && right >= q_left && bottom <= q_top && top >= q_bottom
}

fn matches_datetime(value: &str, query: &str) -> bool {
    if let Some((start, end)) = query.split_once('/') {
        (start == ".." || value >= start) && (end == ".." || value <= end)
    } else {
        value == query
    }
}

fn required_string(object: &Map<String, Value>, key: &str) -> PeerResult<()> {
    if object
        .get(key)
        .and_then(Value::as_str)
        .is_some_and(|value| !value.is_empty())
    {
        Ok(())
    } else {
        Err(invalid(key, "is required"))
    }
}
fn invalid(field: &str, message: &str) -> PeerError {
    PeerError::Validation {
        field: field.into(),
        message: message.into(),
    }
}
fn hex(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn local_file_uris_round_trip_spaces_unicode_and_reserved_characters() {
        let path = Path::new("/tmp/with spaces/é#?.tiff");
        let uri = file_uri(path);
        assert_eq!(uri, "file:///tmp/with%20spaces/%C3%A9%23%3F.tiff");
        assert_eq!(path_from_file_uri(&uri).unwrap(), path);
    }
}
