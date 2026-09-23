//! Single outbound boundary for user text, history, metadata and tool errors.
use crate::{
    AgentProfile,
    provider::{ModelMessage, ModelRequest, ProviderError},
};
use serde_json::Value;

/// Local paths and credential-looking tokens have no place in router context.
/// Explicitly allowed prose is untrusted data; the executor still enforces scope.
pub fn redact(text: &str, secret: Option<&str>) -> String {
    let text = secret
        .filter(|s| !s.is_empty())
        .map_or_else(|| text.to_owned(), |s| text.replace(s, "[credential]"));
    text.split_inclusive(char::is_whitespace)
        .map(|part| {
            let token = part.trim();
            let suffix = &part[part.trim_end().len()..];
            let stripped = token.trim_start_matches(['"', '\'', '(', '[', '{']);
            if (stripped.contains('/')
                && !stripped.starts_with("product://")
                && !stripped.starts_with("feam."))
                || stripped.starts_with('~')
                || stripped.contains(":\\")
                || stripped.starts_with("sk-")
                || stripped.starts_with("Bearer")
                || stripped.contains("API_KEY=")
            {
                format!("[redacted]{suffix}")
            } else {
                part.chars()
                    .filter(|c| !c.is_control() || c.is_whitespace())
                    .collect()
            }
        })
        .collect()
}

/// Free-form metadata requires explicit field permission. Paths, contact,
/// producer, raw errors, and physical inventory are never admitted.
pub fn tool_summary(value: &Value, profile: &AgentProfile) -> Value {
    const STRUCTURAL: &[&str] = &[
        "operations",
        "drafts",
        "summary",
        "overwrite",
        "question",
        "choices",
        "protocol",
        "tool",
        "status",
        "result",
        "error",
        "kind",
        "operation_id",
        "handle",
        "reference",
        "version",
        "manifest_revision",
        "expected_manifest_revision",
        "data_kind",
        "data_format",
        "quality",
        "classification",
        "entries",
        "coverage",
        "namespace",
        "available",
        "revision",
        "observed_at",
        "freshness",
        "integrity_verified",
        "asset_count",
        "estimated_asset_bytes",
        "assets",
        "id",
        "role",
        "media_type",
        "size",
        "next_cursor",
        "snapshot_fingerprint",
        "result_limit",
        "truncated",
        "topic",
        "excerpt",
        "state",
        "completed_at",
        "input_tokens",
        "output_tokens",
        "cost_usd",
        "missing_fields",
    ];
    fn walk(value: &Value, profile: &AgentProfile) -> Value {
        match value {
            Value::Object(map) => Value::Object(
                map.iter()
                    .filter(|(k, _)| {
                        STRUCTURAL.contains(&k.as_str())
                            || profile.allowed_metadata_fields.contains(k)
                    })
                    .map(|(k, v)| (k.clone(), walk(v, profile)))
                    .collect(),
            ),
            Value::Array(values) => {
                Value::Array(values.iter().take(100).map(|v| walk(v, profile)).collect())
            }
            Value::String(text) => {
                Value::String(redact(&text.chars().take(1024).collect::<String>(), None))
            }
            other => other.clone(),
        }
    }
    walk(value, profile)
}

pub fn outbound(
    mut request: ModelRequest,
    profile: &AgentProfile,
    secret: Option<&str>,
) -> Result<ModelRequest, ProviderError> {
    for ModelMessage {
        role,
        content,
        tool_calls,
        ..
    } in &mut request.messages
    {
        if role == "user" && !profile.allow_user_text {
            return Err(ProviderError::Configuration(
                "user text disclosure is disabled".into(),
            ));
        }
        if role == "tool" {
            let value = content
                .as_str()
                .and_then(|s| serde_json::from_str::<Value>(s).ok())
                .unwrap_or_else(|| content.clone());
            let mut allowed = tool_summary(&value, profile);
            scrub_value(&mut allowed, secret);
            *content = Value::String(allowed.to_string());
        } else if role == "system" {
            if let Some(key) = secret.filter(|key| !key.is_empty())
                && let Some(text) = content.as_str()
            {
                *content = Value::String(text.replace(key, "[credential]"));
            }
        } else if role != "system"
            && let Some(text) = content.as_str()
        {
            *content = Value::String(redact(text, secret));
        }
        if let Some(calls) = tool_calls {
            for call in calls {
                scrub_value(&mut call.arguments, secret);
            }
        }
    }
    let bytes = serde_json::to_vec(&request)
        .map_err(|_| ProviderError::Protocol("request serialization failed".into()))?;
    if bytes.len() > profile.max_request_bytes
        || String::from_utf8_lossy(&bytes).chars().count() > profile.max_context_chars
    {
        return Err(ProviderError::TooLarge);
    }
    Ok(request)
}
fn scrub_value(value: &mut Value, secret: Option<&str>) {
    match value {
        Value::String(s) => *s = redact(s, secret),
        Value::Array(a) => a.iter_mut().for_each(|v| scrub_value(v, secret)),
        Value::Object(m) => m.values_mut().for_each(|v| scrub_value(v, secret)),
        _ => (),
    }
}
