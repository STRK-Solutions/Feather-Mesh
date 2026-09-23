//! Small authenticated, read-only STAC HTTP adapter.
//!
//! The adapter intentionally has no publication logic. Every request rebuilds
//! its view through the shared resolver/STAC projection, so a removed peer or
//! withdrawn version cannot be served from a cached physical path.

use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::Path;

use serde_json::{Value, json};

use crate::peer::{PeerError, PeerResult, Project};
use crate::stac::{CONFORMANCE_CLASSES, STAC_VERSION, StacItem, collections, items, search};

pub fn read_bearer_token(path: impl AsRef<Path>) -> PeerResult<String> {
    let path = path.as_ref();
    let metadata = std::fs::metadata(path)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::MetadataExt;
        if metadata.mode() & 0o077 != 0 {
            return Err(PeerError::Policy(format!(
                "token file '{}' must not be group/world readable",
                path.display()
            )));
        }
    }
    let token = std::fs::read_to_string(path)?.trim().to_string();
    if token.is_empty() {
        return Err(PeerError::Validation {
            field: "token_file".into(),
            message: "must contain a bearer token".into(),
        });
    }
    Ok(token)
}

pub fn serve(project: Project, token: String, bind: SocketAddr) -> PeerResult<()> {
    let listener = TcpListener::bind(bind)?;
    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                let _ = respond(project.clone(), &token, stream);
            }
            Err(error) => return Err(PeerError::Io(error)),
        }
    }
    Ok(())
}

fn respond(project: Project, token: &str, mut stream: TcpStream) -> PeerResult<()> {
    let request = read_request(&mut stream)?;
    if !authorized(&request.headers, token) {
        return write_response(
            &mut stream,
            401,
            json!({"code":"Unauthorized","description":"Bearer token required"}),
        );
    }
    let outcome = route(&project, &request);
    match outcome {
        Ok((status, value)) => write_response(&mut stream, status, value),
        Err(HttpError { status, message }) => write_response(
            &mut stream,
            status,
            json!({"code": status_name(status), "description": message}),
        ),
    }
}

#[derive(Debug)]
struct HttpRequest {
    method: String,
    target: String,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}

fn read_request(stream: &mut TcpStream) -> PeerResult<HttpRequest> {
    let mut bytes = Vec::new();
    let mut buffer = [0_u8; 4096];
    let header_end;
    loop {
        let count = stream.read(&mut buffer)?;
        if count == 0 {
            return Err(PeerError::Policy("empty HTTP request".into()));
        }
        bytes.extend_from_slice(&buffer[..count]);
        if bytes.len() > 128 * 1024 {
            return Err(PeerError::Policy("HTTP request is too large".into()));
        }
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            header_end = position + 4;
            break;
        }
    }
    let header = std::str::from_utf8(&bytes[..header_end])
        .map_err(|_| PeerError::Policy("HTTP request headers are not UTF-8".into()))?;
    let mut lines = header.split("\r\n");
    let start = lines
        .next()
        .ok_or_else(|| PeerError::Policy("HTTP request line is missing".into()))?;
    let mut start_parts = start.split_whitespace();
    let method = start_parts
        .next()
        .ok_or_else(|| PeerError::Policy("HTTP method is missing".into()))?
        .to_string();
    let target = start_parts
        .next()
        .ok_or_else(|| PeerError::Policy("HTTP target is missing".into()))?
        .to_string();
    if start_parts.next().is_none() {
        return Err(PeerError::Policy("HTTP version is missing".into()));
    }
    let mut headers = BTreeMap::new();
    for line in lines {
        if line.is_empty() {
            continue;
        }
        let (key, value) = line
            .split_once(':')
            .ok_or_else(|| PeerError::Policy("malformed HTTP header".into()))?;
        headers.insert(key.trim().to_ascii_lowercase(), value.trim().to_string());
    }
    let length = headers
        .get("content-length")
        .map(|value| {
            value
                .parse::<usize>()
                .map_err(|_| PeerError::Policy("invalid Content-Length".into()))
        })
        .transpose()?
        .unwrap_or(0);
    if length > 64 * 1024 {
        return Err(PeerError::Policy("HTTP body is too large".into()));
    }
    while bytes.len() < header_end + length {
        let count = stream.read(&mut buffer)?;
        if count == 0 {
            return Err(PeerError::Policy("incomplete HTTP request body".into()));
        }
        bytes.extend_from_slice(&buffer[..count]);
    }
    Ok(HttpRequest {
        method,
        target,
        headers,
        body: bytes[header_end..header_end + length].to_vec(),
    })
}

fn authorized(headers: &BTreeMap<String, String>, token: &str) -> bool {
    headers
        .get("authorization")
        .is_some_and(|value| value.strip_prefix("Bearer ") == Some(token))
}

#[derive(Debug)]
struct HttpError {
    status: u16,
    message: String,
}
impl HttpError {
    fn bad(message: impl Into<String>) -> Self {
        Self {
            status: 400,
            message: message.into(),
        }
    }
}

fn route(project: &Project, request: &HttpRequest) -> Result<(u16, Value), HttpError> {
    let (path, raw_query) = request
        .target
        .split_once('?')
        .unwrap_or((&request.target, ""));
    let query = query_map(raw_query).map_err(HttpError::bad)?;
    let base = format!(
        "http://{}",
        request
            .headers
            .get("host")
            .map(String::as_str)
            .unwrap_or("127.0.0.1")
    );
    match (request.method.as_str(), path) {
        ("GET", "/") => Ok((200, landing(&base))),
        ("GET", "/conformance") => Ok((200, json!({"conformsTo": CONFORMANCE_CLASSES}))),
        ("GET", "/collections") => Ok((
            200,
            json!({"collections": collections(project).map_err(internal)?, "links": []}),
        )),
        ("GET", "/search") | ("POST", "/search") => {
            let parameters = if request.method == "POST" {
                json_parameters(&request.body)?
            } else {
                query
            };
            Ok((200, search_response(project, &parameters, &base)?))
        }
        ("GET", collection_path) if collection_path.starts_with("/collections/") => {
            collection_route(project, collection_path, &query, &base)
        }
        _ => Err(HttpError {
            status: 404,
            message: "STAC endpoint not found".into(),
        }),
    }
}

fn landing(base: &str) -> Value {
    json!({
        "stac_version": STAC_VERSION, "type": "Catalog", "id": "feam", "title": "Feather Mesh STAC API",
        "description": "Read-only project-scoped registered raster metadata", "conformsTo": CONFORMANCE_CLASSES,
        "links": [
            {"rel":"self", "href": format!("{base}/"), "type":"application/json"},
            {"rel":"conformance", "href": format!("{base}/conformance"), "type":"application/json"},
            {"rel":"data", "href": format!("{base}/collections"), "type":"application/json"},
            {"rel":"search", "href": format!("{base}/search"), "type":"application/geo+json"}
        ]
    })
}

fn collection_route(
    project: &Project,
    path: &str,
    query: &BTreeMap<String, String>,
    base: &str,
) -> Result<(u16, Value), HttpError> {
    let suffix = path.trim_start_matches("/collections/");
    let Some((collection, item_suffix)) = suffix.split_once("/items") else {
        let collection = collections(project)
            .map_err(internal)?
            .into_iter()
            .find(|value| value.get("id").and_then(Value::as_str) == Some(suffix))
            .ok_or_else(|| HttpError {
                status: 404,
                message: "collection not found".into(),
            })?;
        return Ok((200, collection));
    };
    let matching = search(
        &items(project).map_err(internal)?,
        Some(collection),
        None,
        None,
    );
    if item_suffix.is_empty() {
        let mut parameters = query.clone();
        parameters.insert("collections".into(), collection.to_string());
        return Ok((200, search_response(project, &parameters, base)?));
    }
    let id = item_suffix.strip_prefix('/').ok_or_else(|| HttpError {
        status: 404,
        message: "item endpoint not found".into(),
    })?;
    let item = matching
        .into_iter()
        .find(|item| item.id == id)
        .ok_or_else(|| HttpError {
            status: 404,
            message: "item not found".into(),
        })?;
    Ok((200, item.value))
}

fn search_response(
    project: &Project,
    parameters: &BTreeMap<String, String>,
    base: &str,
) -> Result<Value, HttpError> {
    let all = items(project).map_err(internal)?;
    let collection = parameters.get("collections").map(String::as_str);
    let bbox = parameters
        .get("bbox")
        .map(|value| parse_bbox(value))
        .transpose()?;
    let datetime = parameters.get("datetime").map(String::as_str);
    let limit = parameters
        .get("limit")
        .map(|value| {
            value
                .parse::<usize>()
                .map_err(|_| HttpError::bad("limit must be a positive integer"))
        })
        .transpose()?
        .unwrap_or(10);
    if limit == 0 || limit > 1_000 {
        return Err(HttpError::bad("limit must be between 1 and 1000"));
    }
    let filtered = search(&all, collection, bbox, datetime);
    let snapshot = snapshot(&all);
    let offset = if let Some(cursor) = parameters.get("cursor") {
        let cursor = decode_cursor(cursor)?;
        if cursor.snapshot != snapshot
            || cursor.collection.as_deref() != collection
            || cursor.bbox.as_deref() != parameters.get("bbox").map(String::as_str)
            || cursor.datetime.as_deref() != datetime
        {
            return Err(HttpError {
                status: 409,
                message: "cursor catalog snapshot or query changed; restart search".into(),
            });
        }
        cursor.offset
    } else {
        0
    };
    if offset > filtered.len() {
        return Err(HttpError::bad("cursor offset is invalid"));
    }
    let page: Vec<Value> = filtered
        .iter()
        .skip(offset)
        .take(limit)
        .map(|item| item.value.clone())
        .collect();
    let mut links = vec![
        json!({"rel":"self", "href": format!("{base}/search"), "type":"application/geo+json"}),
    ];
    if offset + page.len() < filtered.len() {
        let cursor = Cursor {
            snapshot,
            offset: offset + page.len(),
            collection: collection.map(str::to_string),
            bbox: parameters.get("bbox").cloned(),
            datetime: datetime.map(str::to_string),
        };
        let mut next_query = vec![
            format!("cursor={}", encode_cursor(&cursor)),
            format!("limit={limit}"),
        ];
        if let Some(collection) = collection {
            next_query.push(format!("collections={collection}"));
        }
        if let Some(bbox) = parameters.get("bbox") {
            next_query.push(format!("bbox={bbox}"));
        }
        if let Some(datetime) = datetime {
            next_query.push(format!("datetime={datetime}"));
        }
        links.push(json!({"rel":"next", "href": format!("{base}/search?{}", next_query.join("&")), "type":"application/geo+json"}));
    }
    Ok(
        json!({"type":"FeatureCollection", "features": page, "links": links, "numberMatched": filtered.len(), "numberReturned": page.len()}),
    )
}

#[derive(serde::Serialize, serde::Deserialize)]
struct Cursor {
    snapshot: String,
    offset: usize,
    collection: Option<String>,
    bbox: Option<String>,
    datetime: Option<String>,
}
fn snapshot(items: &[StacItem]) -> String {
    let mut material = String::new();
    for item in items {
        material.push_str(&item.id);
        material.push(':');
        material.push_str(&item.manifest_revision.to_string());
        material.push(';');
    }
    hex_encode(material.as_bytes())
}
fn encode_cursor(cursor: &Cursor) -> String {
    hex_encode(&serde_json::to_vec(cursor).expect("cursor serializes"))
}
fn decode_cursor(value: &str) -> Result<Cursor, HttpError> {
    serde_json::from_slice(&hex_decode(value).ok_or_else(|| HttpError::bad("cursor is malformed"))?)
        .map_err(|_| HttpError::bad("cursor is malformed"))
}
fn hex_encode(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn hex_decode(value: &str) -> Option<Vec<u8>> {
    if !value.len().is_multiple_of(2) {
        return None;
    }
    (0..value.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&value[index..index + 2], 16).ok())
        .collect()
}
fn parse_bbox(value: &str) -> Result<[f64; 4], HttpError> {
    let values: Vec<f64> = value
        .split(',')
        .map(|part| {
            part.parse::<f64>()
                .map_err(|_| HttpError::bad("bbox must contain four numbers"))
        })
        .collect::<Result<_, _>>()?;
    if values.len() != 4 || values[0] > values[2] || values[1] > values[3] {
        return Err(HttpError::bad("bbox must be west,south,east,north"));
    }
    Ok([values[0], values[1], values[2], values[3]])
}
fn query_map(query: &str) -> Result<BTreeMap<String, String>, String> {
    let mut result = BTreeMap::new();
    if query.is_empty() {
        return Ok(result);
    }
    for piece in query.split('&') {
        let (key, value) = piece
            .split_once('=')
            .ok_or("query parameter is missing '='")?;
        result.insert(percent_decode(key)?, percent_decode(value)?);
    }
    Ok(result)
}
fn json_parameters(body: &[u8]) -> Result<BTreeMap<String, String>, HttpError> {
    let value: Value = serde_json::from_slice(body)
        .map_err(|_| HttpError::bad("search JSON body is malformed"))?;
    let object = value
        .as_object()
        .ok_or_else(|| HttpError::bad("search JSON body must be an object"))?;
    let mut result = BTreeMap::new();
    for key in ["datetime", "limit", "cursor"] {
        if let Some(value) = object.get(key) {
            result.insert(
                key.into(),
                value
                    .as_str()
                    .map(str::to_string)
                    .unwrap_or_else(|| value.to_string()),
            );
        }
    }
    if let Some(collections) = object.get("collections") {
        let collection = match collections {
            Value::String(value) => Some(value.clone()),
            Value::Array(values) if values.len() == 1 => values[0].as_str().map(str::to_string),
            Value::Array(_) => {
                return Err(HttpError::bad(
                    "only one collection is supported by this query",
                ));
            }
            _ => None,
        }
        .ok_or_else(|| HttpError::bad("collections must be a string or one-element array"))?;
        result.insert("collections".into(), collection);
    }
    if let Some(bbox) = object.get("bbox").and_then(Value::as_array) {
        result.insert(
            "bbox".into(),
            bbox.iter()
                .map(Value::to_string)
                .collect::<Vec<_>>()
                .join(","),
        );
    }
    Ok(result)
}
fn percent_decode(value: &str) -> Result<String, String> {
    let bytes = value.as_bytes();
    let mut result = Vec::new();
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'%' {
            if index + 2 >= bytes.len() {
                return Err("incomplete percent escape".into());
            }
            let byte = u8::from_str_radix(&value[index + 1..index + 3], 16)
                .map_err(|_| "invalid percent escape")?;
            result.push(byte);
            index += 3;
        } else {
            result.push(if bytes[index] == b'+' {
                b' '
            } else {
                bytes[index]
            });
            index += 1;
        }
    }
    String::from_utf8(result).map_err(|_| "query is not UTF-8".into())
}
fn internal(error: PeerError) -> HttpError {
    HttpError {
        status: 500,
        message: error.to_string(),
    }
}
fn status_name(status: u16) -> &'static str {
    match status {
        400 => "BadRequest",
        401 => "Unauthorized",
        404 => "NotFound",
        409 => "Conflict",
        _ => "InternalServerError",
    }
}
fn write_response(stream: &mut TcpStream, status: u16, value: Value) -> PeerResult<()> {
    let body = serde_json::to_vec(&value)?;
    let response = format!(
        "HTTP/1.1 {status} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        status_name(status),
        body.len()
    );
    stream.write_all(response.as_bytes())?;
    stream.write_all(&body)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn cursor_is_opaque_and_round_trips() {
        let cursor = Cursor {
            snapshot: "abc".into(),
            offset: 2,
            collection: Some("climate--temp".into()),
            bbox: None,
            datetime: Some("2026-01-01T00:00:00Z".into()),
        };
        let encoded = encode_cursor(&cursor);
        assert!(!encoded.contains("climate"));
        assert_eq!(decode_cursor(&encoded).unwrap().offset, 2);
    }
}
