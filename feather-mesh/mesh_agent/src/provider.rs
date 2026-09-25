use std::collections::VecDeque;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

use serde::{Deserialize, Serialize};
use thiserror::Error;

#[cfg(feature = "hosted")]
use crate::config::AgentProfile;

#[derive(Debug, Clone, Default)]
pub struct CancellationToken(Arc<AtomicBool>);

impl CancellationToken {
    pub fn cancel(&self) {
        self.0.store(true, Ordering::Release);
    }

    pub fn is_cancelled(&self) -> bool {
        self.0.load(Ordering::Acquire)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelMessage {
    pub role: String,
    pub content: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ProviderToolCall>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolSchema {
    pub name: String,
    pub description: String,
    pub parameters: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelRequest {
    pub messages: Vec<ModelMessage>,
    pub tools: Vec<ToolSchema>,
    pub max_response_chars: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderToolCall {
    pub id: String,
    pub name: String,
    pub arguments: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderUsage {
    #[serde(default)]
    pub model: Option<String>,
    #[serde(default)]
    pub provider: Option<String>,
    #[serde(default)]
    pub generation_id: Option<String>,
    pub input_tokens: Option<u64>,
    pub output_tokens: Option<u64>,
    pub cost_usd: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ProviderEvent {
    TextDelta(String),
    ToolCall(ProviderToolCall),
    Usage(ProviderUsage),
    Finished,
}

#[derive(Debug, Error, Clone)]
pub enum ProviderError {
    #[error("provider configuration error: {0}")]
    Configuration(String),
    #[error("provider authentication failed")]
    Authentication,
    #[error("provider rate limit exceeded")]
    RateLimited,
    #[error("provider transport error: {0}")]
    Transport(String),
    #[error("provider response exceeded configured limit")]
    TooLarge,
    #[error("provider response was malformed: {0}")]
    Malformed(String),
    #[error("provider stream was cancelled")]
    Cancelled,
    #[error("provider protocol error: {0}")]
    Protocol(String),
}

impl ProviderError {
    pub fn kind(&self) -> &'static str {
        match self {
            Self::Configuration(_) => "provider_configuration",
            Self::Authentication => "provider_authentication",
            Self::RateLimited => "provider_rate_limited",
            Self::Transport(_) => "provider_transport",
            Self::TooLarge => "provider_output_too_large",
            Self::Malformed(_) => "provider_malformed_response",
            Self::Cancelled => "provider_cancelled",
            Self::Protocol(_) => "provider_protocol",
        }
    }
}

/// A provider streams text and only emits a tool call after it has assembled a
/// complete, parseable call. A consumer must still validate the tool name and
/// arguments independently.
pub trait ModelProvider: Send {
    fn set_request_timeout(&mut self, _timeout: std::time::Duration) {}
    fn set_request_id(&mut self, _request_id: Option<String>) {}
    fn stream(
        &mut self,
        request: ModelRequest,
        cancellation: &CancellationToken,
        on_event: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError>;
}

/// Deterministic scripted provider used by tests, demos, and replay mode. It
/// has no network, credential, or feam execution authority.
#[derive(Debug, Clone)]
pub struct FakeProvider {
    scripts: ScriptQueue,
}

type FakeScript = Result<Vec<ProviderEvent>, ProviderError>;
type ScriptQueue = Arc<Mutex<VecDeque<FakeScript>>>;

impl FakeProvider {
    pub fn new(
        scripts: impl IntoIterator<Item = Result<Vec<ProviderEvent>, ProviderError>>,
    ) -> Self {
        Self {
            scripts: Arc::new(Mutex::new(scripts.into_iter().collect())),
        }
    }
}

impl ModelProvider for FakeProvider {
    fn stream(
        &mut self,
        _request: ModelRequest,
        cancellation: &CancellationToken,
        on_event: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        if cancellation.is_cancelled() {
            return Err(ProviderError::Cancelled);
        }
        let script = self
            .scripts
            .lock()
            .map_err(|_| ProviderError::Transport("fake provider lock poisoned".into()))?
            .pop_front()
            .ok_or_else(|| ProviderError::Protocol("fake provider script exhausted".into()))?;
        for event in script? {
            if cancellation.is_cancelled() {
                return Err(ProviderError::Cancelled);
            }
            on_event(event);
        }
        Ok(())
    }
}

#[cfg(feature = "hosted")]
#[derive(Clone)]
pub struct RouterProvider {
    profile: AgentProfile,
    api_key: String,
    client: reqwest::Client,
    request_id: Option<String>,
}

#[cfg(feature = "hosted")]
impl RouterProvider {
    pub fn from_profile(profile: AgentProfile) -> Result<Self, ProviderError> {
        profile
            .validate("selected")
            .map_err(|error| ProviderError::Configuration(error.to_string()))?;
        let api_key = std::env::var(&profile.api_key_env).map_err(|_| {
            ProviderError::Configuration(format!(
                "set the {} environment variable to enable the assistant",
                profile.api_key_env
            ))
        })?;
        if api_key.trim().is_empty() {
            return Err(ProviderError::Configuration(format!(
                "{} is empty",
                profile.api_key_env
            )));
        }
        let mut client = reqwest::Client::builder()
            .pool_max_idle_per_host(0)
            .redirect(reqwest::redirect::Policy::none())
            .use_rustls_tls();
        if let Some(path) = &profile.loopback_ca_file {
            use std::io::Read as _;
            let file = std::fs::File::open(path).map_err(|_| {
                ProviderError::Configuration("loopback CA certificate is unavailable".into())
            })?;
            let mut pem = Vec::new();
            file.take(65_537).read_to_end(&mut pem).map_err(|_| {
                ProviderError::Configuration("loopback CA certificate could not be read".into())
            })?;
            if pem.len() > 65_536 {
                return Err(ProviderError::Configuration(
                    "loopback CA certificate is too large".into(),
                ));
            }
            let certificate = reqwest::Certificate::from_pem(&pem).map_err(|_| {
                ProviderError::Configuration("loopback CA certificate is invalid".into())
            })?;
            client = client.add_root_certificate(certificate).no_proxy();
        }
        let client = client
            .build()
            .map_err(|error| ProviderError::Configuration(error.to_string()))?;
        Ok(Self {
            profile,
            api_key,
            client,
            request_id: None,
        })
    }

    fn endpoint(&self) -> Result<url::Url, ProviderError> {
        let endpoint = format!(
            "{}/chat/completions",
            self.profile.base_url.trim_end_matches('/')
        );
        let url = url::Url::parse(&endpoint)
            .map_err(|error| ProviderError::Configuration(error.to_string()))?;
        if url.scheme() != "https"
            && !matches!(url.host_str(), Some("127.0.0.1") | Some("localhost"))
        {
            return Err(ProviderError::Configuration(
                "hosted router endpoint must use HTTPS".into(),
            ));
        }
        Ok(url)
    }
}

#[cfg(feature = "hosted")]
impl ModelProvider for RouterProvider {
    fn set_request_id(&mut self, request_id: Option<String>) {
        self.request_id = request_id;
    }
    fn set_request_timeout(&mut self, timeout: std::time::Duration) {
        self.profile.max_request_seconds = timeout.as_secs().max(1);
    }
    fn stream(
        &mut self,
        request: ModelRequest,
        cancellation: &CancellationToken,
        on_event: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .map_err(|error| ProviderError::Transport(error.to_string()))?;
        runtime.block_on(async {
            let deadline = tokio::time::Instant::now()
                + std::time::Duration::from_secs(self.profile.max_request_seconds);
            let future = self.stream_async(request, cancellation, on_event);
            tokio::pin!(future);
            loop {
                if cancellation.is_cancelled() {
                    return Err(ProviderError::Cancelled);
                }
                if tokio::time::Instant::now() >= deadline {
                    return Err(ProviderError::Transport(
                        "overall request deadline exceeded".into(),
                    ));
                }
                if let Ok(result) =
                    tokio::time::timeout(std::time::Duration::from_millis(20), &mut future).await
                {
                    return result;
                }
            }
        })
    }
}

#[cfg(feature = "hosted")]
impl RouterProvider {
    async fn stream_async(
        &self,
        request: ModelRequest,
        cancellation: &CancellationToken,
        on_event: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        use futures_util::StreamExt as _;

        let request = crate::disclosure::outbound(request, &self.profile, Some(&self.api_key))?;
        let tool_names = RouterToolNames::new(&request.tools)?;
        let tools = tool_names.schemas(&request.tools)?;
        // Guided turns advertise no tools, but their conversation may contain
        // completed ordinary turns. Translate only those historical names from
        // the closed application schema; keep the response allowlist empty.
        let history_names = if request.tools.is_empty() {
            RouterToolNames::new(&crate::tools::tool_schemas())?
        } else {
            RouterToolNames::new(&request.tools)?
        };
        let messages = history_names.messages(&request.messages)?;
        let tool_choice = if tools.is_empty() { "none" } else { "auto" };
        let mut body = serde_json::json!({
            "model": self.profile.model,
            "max_tokens": self.profile.max_output_tokens,
            "temperature": 0,
            "reasoning": {"enabled": self.profile.reasoning_enabled},
            "messages": messages,
            "tools": tools,
            "tool_choice": tool_choice,
            "stream": true,
            "stream_options": {"include_usage": true}
        });
        {
            body["provider"] = serde_json::json!({
                "only": self.profile.allowed_providers,
                "allow_fallbacks": self.profile.allow_provider_fallbacks,
                "require_parameters": true,
                "max_price": {"prompt": self.profile.max_input_price, "completion": self.profile.max_output_price}
            });
        }
        if self.profile.allowed_providers.is_empty() {
            body["provider"].as_object_mut().unwrap().remove("only");
        }
        if self.profile.max_input_price.is_none() && self.profile.max_output_price.is_none() {
            body["provider"]
                .as_object_mut()
                .unwrap()
                .remove("max_price");
        }
        let encoded = serde_json::to_vec(&body)
            .map_err(|_| ProviderError::Protocol("request serialization failed".into()))?;
        if encoded.len() > self.profile.max_request_bytes {
            return Err(ProviderError::TooLarge);
        }
        let mut outbound = self
            .client
            .post(self.endpoint()?)
            .bearer_auth(&self.api_key)
            .header("Content-Type", "application/json")
            .body(encoded);
        if self.profile.loopback_ca_file.is_some()
            && let Some(request_id) = &self.request_id
        {
            outbound = outbound.header("X-Request-ID", request_id);
        }
        let response = outbound.send().await.map_err(|error| {
            ProviderError::Transport(format!("HTTP request failed: {}", error.without_url()))
        })?;
        let status = response.status();
        if !status.is_success() {
            let mut error_bytes = Vec::new();
            let mut stream = response.bytes_stream();
            while let Some(Ok(chunk)) = stream.next().await {
                if error_bytes.len() + chunk.len() > 4096 {
                    break;
                }
                error_bytes.extend_from_slice(&chunk);
            }
            let message = serde_json::from_slice::<serde_json::Value>(&error_bytes)
                .ok()
                .and_then(|v| {
                    v.pointer("/error/message")
                        .and_then(serde_json::Value::as_str)
                        .map(str::to_owned)
                })
                .map(|s| crate::disclosure::redact(&s, Some(&self.api_key)))
                .unwrap_or_default();
            return Err(match status.as_u16() {
                401 | 403 => ProviderError::Authentication,
                429 => ProviderError::RateLimited,
                _ => ProviderError::Transport(format!("HTTP {}: {}", status.as_u16(), message)),
            });
        }
        let mut parser = SseParser::default();
        let mut tool_calls = std::collections::BTreeMap::<u64, PartialToolCall>::new();
        let mut emitted = 0usize;
        let mut done = false;
        let mut finish_reason = None;
        let mut stream = response.bytes_stream();
        while let Some(chunk) = stream.next().await {
            if cancellation.is_cancelled() {
                return Err(ProviderError::Cancelled);
            }
            let chunk =
                chunk.map_err(|_| ProviderError::Transport("HTTP transport failure".into()))?;
            emitted = emitted.saturating_add(chunk.len());
            if emitted > self.profile.max_response_bytes {
                return Err(ProviderError::TooLarge);
            }
            for event in parser.push(&chunk)? {
                if event == "[DONE]" {
                    done = true;
                    continue;
                }
                let frame: serde_json::Value = serde_json::from_str(&event)
                    .map_err(|_| ProviderError::Malformed("invalid SSE JSON".into()))?;
                if let Some(reason) = frame
                    .pointer("/choices/0/finish_reason")
                    .and_then(serde_json::Value::as_str)
                {
                    finish_reason = Some(reason.to_owned());
                }
                process_router_event(&event, &mut tool_calls, on_event)?;
            }
        }
        for event in parser.finish()? {
            if event != "[DONE]" {
                process_router_event(&event, &mut tool_calls, on_event)?;
            }
        }
        if !done
            || finish_reason
                .as_deref()
                .is_none_or(|r| !["stop", "tool_calls"].contains(&r))
        {
            return Err(ProviderError::Protocol(
                "incomplete or truncated completion; no tool executed".into(),
            ));
        }
        let mut complete_calls = Vec::new();
        let mut ids = std::collections::BTreeSet::new();
        for (_, call) in tool_calls {
            let id = call
                .id
                .ok_or_else(|| ProviderError::Malformed("tool call has no id".into()))?;
            if !ids.insert(id.clone()) {
                return Err(ProviderError::Malformed("duplicate tool call ID".into()));
            }
            let name = call
                .name
                .ok_or_else(|| ProviderError::Malformed("tool call has no name".into()))?;
            let name = tool_names.local_name(&name)?.to_owned();
            let arguments = serde_json::from_str(&call.arguments).map_err(|_| {
                ProviderError::Malformed("tool arguments are not complete JSON".into())
            })?;
            complete_calls.push(ProviderToolCall {
                id,
                name,
                arguments,
            });
        }
        // Emit only after the entire bounded batch is complete and valid.
        for call in complete_calls {
            on_event(ProviderEvent::ToolCall(call));
        }
        on_event(ProviderEvent::Finished);
        Ok(())
    }
}

/// Keep the dotted feam protocol names inside the harness. Router providers
/// require function names containing only ASCII letters, digits, `_`, or `-`.
/// A request-scoped map also prevents ambiguous aliases and invented names.
#[cfg(feature = "hosted")]
struct RouterToolNames {
    to_wire: std::collections::BTreeMap<String, String>,
    to_local: std::collections::BTreeMap<String, String>,
}

#[cfg(feature = "hosted")]
impl RouterToolNames {
    fn new(tools: &[ToolSchema]) -> Result<Self, ProviderError> {
        let mut names = Self {
            to_wire: Default::default(),
            to_local: Default::default(),
        };
        for tool in tools {
            let wire = tool.name.replace('.', "__");
            if wire.is_empty()
                || wire.len() > 64
                || !wire
                    .bytes()
                    .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'_' | b'-'))
                || names.to_local.contains_key(&wire)
            {
                return Err(ProviderError::Configuration(
                    "tool names must map to distinct router function names of 1–64 ASCII letters, digits, underscores, or dashes".into(),
                ));
            }
            names.to_wire.insert(tool.name.clone(), wire.clone());
            names.to_local.insert(wire, tool.name.clone());
        }
        Ok(names)
    }

    fn wire_name(&self, local: &str) -> Result<&str, ProviderError> {
        self.to_wire.get(local).map(String::as_str).ok_or_else(|| {
            ProviderError::Protocol(
                "tool history contains a tool outside the request schema".into(),
            )
        })
    }

    fn local_name(&self, wire: &str) -> Result<&str, ProviderError> {
        self.to_local.get(wire).map(String::as_str).ok_or_else(|| {
            ProviderError::Protocol("provider returned a tool outside the request schema".into())
        })
    }

    fn schemas(&self, tools: &[ToolSchema]) -> Result<Vec<serde_json::Value>, ProviderError> {
        tools
            .iter()
            .map(|tool| {
                Ok(serde_json::json!({
                    "type": "function",
                    "function": {
                        "name": self.wire_name(&tool.name)?,
                        "description": tool.description,
                        "parameters": tool.parameters,
                    },
                }))
            })
            .collect()
    }

    fn messages(&self, messages: &[ModelMessage]) -> Result<Vec<serde_json::Value>, ProviderError> {
        messages
            .iter()
            .map(|message| {
                let mut wire = serde_json::json!({
                    "role": message.role,
                    "content": message.content,
                });
                if let Some(id) = &message.tool_call_id {
                    wire["tool_call_id"] = serde_json::json!(id);
                }
                if let Some(calls) = &message.tool_calls {
                    // The provider-neutral DTO is not the router wire format:
                    // function arguments are a JSON *string* inside `function`.
                    let calls: Result<Vec<_>, ProviderError> = calls
                        .iter()
                        .map(|call| {
                            Ok(serde_json::json!({
                                "id": call.id,
                                "type": "function",
                                "function": {
                                    "name": self.wire_name(&call.name)?,
                                    "arguments": call.arguments.to_string(),
                                },
                            }))
                        })
                        .collect();
                    wire["tool_calls"] = serde_json::json!(calls?);
                }
                Ok(wire)
            })
            .collect()
    }
}

#[cfg(feature = "hosted")]
#[derive(Debug, Default)]
struct PartialToolCall {
    id: Option<String>,
    name: Option<String>,
    arguments: String,
}

#[cfg(feature = "hosted")]
fn process_router_event(
    raw: &str,
    tool_calls: &mut std::collections::BTreeMap<u64, PartialToolCall>,
    on_event: &mut dyn FnMut(ProviderEvent),
) -> Result<(), ProviderError> {
    let value: serde_json::Value =
        serde_json::from_str(raw).map_err(|error| ProviderError::Malformed(error.to_string()))?;
    if value.get("error").is_some() {
        return Err(ProviderError::Transport(
            "provider reported an in-stream error".into(),
        ));
    }
    if let Some(usage) = value.get("usage") {
        on_event(ProviderEvent::Usage(ProviderUsage {
            model: value
                .get("model")
                .and_then(serde_json::Value::as_str)
                .map(str::to_owned),
            provider: value
                .get("provider")
                .and_then(serde_json::Value::as_str)
                .map(str::to_owned),
            generation_id: value
                .get("id")
                .and_then(serde_json::Value::as_str)
                .map(str::to_owned),
            input_tokens: usage
                .get("prompt_tokens")
                .and_then(serde_json::Value::as_u64),
            output_tokens: usage
                .get("completion_tokens")
                .and_then(serde_json::Value::as_u64),
            cost_usd: usage.get("cost").and_then(serde_json::Value::as_f64),
        }));
    }
    let Some(choice) = value
        .get("choices")
        .and_then(serde_json::Value::as_array)
        .and_then(|choices| choices.first())
    else {
        return Ok(());
    };
    let delta = choice.get("delta").unwrap_or(&serde_json::Value::Null);
    if let Some(content) = delta.get("content").and_then(serde_json::Value::as_str) {
        on_event(ProviderEvent::TextDelta(content.into()));
    }
    if let Some(calls) = delta
        .get("tool_calls")
        .and_then(serde_json::Value::as_array)
    {
        for call in calls {
            let index = call
                .get("index")
                .and_then(serde_json::Value::as_u64)
                .unwrap_or(0);
            if index >= 8 {
                return Err(ProviderError::Protocol("too many tool calls".into()));
            }
            let partial = tool_calls.entry(index).or_default();
            if let Some(id) = call.get("id").and_then(serde_json::Value::as_str) {
                if partial.id.as_deref().is_some_and(|old| old != id) {
                    return Err(ProviderError::Malformed("tool call ID changed".into()));
                }
                partial.id = Some(id.into());
            }
            if let Some(name) = call
                .pointer("/function/name")
                .and_then(serde_json::Value::as_str)
            {
                if partial.name.as_deref().is_some_and(|old| old != name) {
                    return Err(ProviderError::Malformed("tool name changed".into()));
                }
                partial.name = Some(name.into());
            }
            if let Some(arguments) = call
                .pointer("/function/arguments")
                .and_then(serde_json::Value::as_str)
            {
                partial.arguments.push_str(arguments);
            }
        }
    }
    Ok(())
}

/// Small SSE parser that handles comments, multi-line `data:` fields, split
/// frames, and split UTF-8 code points without treating chunks as messages.
#[cfg(feature = "hosted")]
#[derive(Debug, Default)]
struct SseParser {
    undecoded: Vec<u8>,
    line: String,
    data: Vec<String>,
}

#[cfg(feature = "hosted")]
impl SseParser {
    fn push(&mut self, bytes: &[u8]) -> Result<Vec<String>, ProviderError> {
        self.undecoded.extend_from_slice(bytes);
        let valid_len = match std::str::from_utf8(&self.undecoded) {
            Ok(_) => self.undecoded.len(),
            Err(error) if error.error_len().is_none() => error.valid_up_to(),
            Err(error) => return Err(ProviderError::Malformed(error.to_string())),
        };
        let text = std::str::from_utf8(&self.undecoded[..valid_len])
            .map_err(|error| ProviderError::Malformed(error.to_string()))?
            .to_owned();
        self.undecoded.drain(..valid_len);
        self.push_text(&text)
    }

    fn finish(&mut self) -> Result<Vec<String>, ProviderError> {
        if !self.undecoded.is_empty() {
            return Err(ProviderError::Malformed(
                "truncated UTF-8 in SSE stream".into(),
            ));
        }
        if !self.line.is_empty() {
            let line = std::mem::take(&mut self.line);
            return self.process_line(&line);
        }
        Ok(Vec::new())
    }

    fn push_text(&mut self, text: &str) -> Result<Vec<String>, ProviderError> {
        let mut emitted = Vec::new();
        for part in text.split_inclusive('\n') {
            if let Some(line) = part.strip_suffix('\n') {
                self.line.push_str(line.strip_suffix('\r').unwrap_or(line));
                let complete = std::mem::take(&mut self.line);
                emitted.extend(self.process_line(&complete)?);
            } else {
                self.line.push_str(part);
            }
        }
        Ok(emitted)
    }

    fn process_line(&mut self, line: &str) -> Result<Vec<String>, ProviderError> {
        if line.is_empty() {
            if self.data.is_empty() {
                return Ok(Vec::new());
            }
            return Ok(vec![std::mem::take(&mut self.data).join("\n")]);
        }
        if line.starts_with(':') {
            return Ok(Vec::new());
        }
        if let Some(data) = line.strip_prefix("data:") {
            self.data
                .push(data.strip_prefix(' ').unwrap_or(data).into());
        }
        Ok(Vec::new())
    }
}

#[cfg(all(test, feature = "hosted"))]
mod hosted_tests {
    use super::*;

    #[test]
    fn router_translates_tool_schemas_and_correlated_history() {
        let tools = crate::tools::tool_schemas();
        let names = RouterToolNames::new(&tools).unwrap();
        let schemas = names.schemas(&tools).unwrap();
        assert!(schemas.iter().all(|schema| {
            let name = schema["function"]["name"].as_str().unwrap();
            !name.contains('.') && names.local_name(name).is_ok()
        }));
        assert_eq!(names.wire_name("help.lookup").unwrap(), "help__lookup");
        assert_eq!(names.local_name("help__lookup").unwrap(), "help.lookup");

        let arguments = serde_json::json!({"topic":"resolve"});
        let history = names
            .messages(&[
                ModelMessage {
                    role: "assistant".into(),
                    content: serde_json::Value::Null,
                    tool_call_id: None,
                    tool_calls: Some(vec![ProviderToolCall {
                        id: "provider-call-7".into(),
                        name: "help.lookup".into(),
                        arguments: arguments.clone(),
                    }]),
                },
                ModelMessage {
                    role: "tool".into(),
                    content: serde_json::json!("synthetic help result"),
                    tool_call_id: Some("provider-call-7".into()),
                    tool_calls: None,
                },
            ])
            .unwrap();
        assert_eq!(
            history[0]["tool_calls"][0],
            serde_json::json!({
                "id":"provider-call-7", "type":"function",
                "function":{"name":"help__lookup", "arguments":arguments.to_string()}
            })
        );
        assert_eq!(history[1]["tool_call_id"], "provider-call-7");
        assert!(history[1].get("tool_calls").is_none());
    }

    #[test]
    fn router_rejects_ambiguous_names_and_unadvertised_calls() {
        let tool = |name: &str| ToolSchema {
            name: name.into(),
            description: "synthetic tool".into(),
            parameters: serde_json::json!({"type":"object"}),
        };
        assert!(RouterToolNames::new(&[tool("help.lookup"), tool("help__lookup")]).is_err());
        assert!(RouterToolNames::new(&[tool("help.lookup"), tool("help.lookup")]).is_err());
        for invalid in ["", "has space", "path/name", &"x".repeat(65)] {
            assert!(RouterToolNames::new(&[tool(invalid)]).is_err());
        }
        let names = RouterToolNames::new(&[tool("help.lookup")]).unwrap();
        assert!(names.local_name("product__withdraw").is_err());
        assert!(names.local_name("help.lookup").is_err());
        assert!(names.wire_name("product.withdraw").is_err());
    }

    #[test]
    fn router_http_round_trip_maps_names_and_preserves_tool_call_ids() {
        router_history_round_trip(false, false);
    }

    #[test]
    fn router_guided_turn_preserves_ordinary_tool_history_without_advertising_tools() {
        router_history_round_trip(true, false);
    }

    #[test]
    fn router_guided_turn_rejects_new_tool_calls() {
        router_history_round_trip(true, true);
    }

    fn router_history_round_trip(guided: bool, returned_tool: bool) {
        use std::io::{BufRead, BufReader, Read, Write};
        use std::net::TcpListener;
        use std::time::{Duration, Instant};

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let address = listener.local_addr().unwrap();
        let server = std::thread::spawn(move || {
            for step in 0..2 {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut socket = loop {
                    match listener.accept() {
                        Ok((socket, _)) => break socket,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < deadline, "router did not send its request");
                            std::thread::sleep(Duration::from_millis(5));
                        }
                        Err(error) => panic!("loopback accept failed: {error}"),
                    }
                };
                socket.set_nonblocking(false).unwrap();
                socket
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                socket
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                let mut reader = BufReader::new(&mut socket);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert!(reader.read_line(&mut line).unwrap() > 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((header, value)) = line.split_once(':')
                        && header.eq_ignore_ascii_case("content-length")
                    {
                        length = Some(value.trim().parse::<usize>().unwrap());
                    }
                }
                let length = length.unwrap();
                assert!(length < 16_384);
                let mut bytes = vec![0; length];
                reader.read_exact(&mut bytes).unwrap();
                let body: serde_json::Value = serde_json::from_slice(&bytes).unwrap();
                if guided && step == 1 {
                    assert_eq!(body["tools"], serde_json::json!([]));
                    assert_eq!(body["tool_choice"], "none");
                } else {
                    assert_eq!(body["tools"][0]["function"]["name"], "help__lookup");
                    assert_eq!(body["tool_choice"], "auto");
                }
                assert_eq!(body["provider"]["allow_fallbacks"], false);
                let events = if step == 0 {
                    vec![
                        serde_json::json!({"choices":[{"delta":{"tool_calls":[{
                            "index":0,"id":"call-preserved-7","type":"function",
                            "function":{"name":"help__lookup","arguments":"{\"topic\":"}
                        }]}}]}),
                        serde_json::json!({"choices":[{"delta":{"tool_calls":[{
                            "index":0,"function":{"arguments":"\"resolve\"}"}
                        }]},"finish_reason":"tool_calls"}]}),
                    ]
                } else {
                    assert_eq!(
                        body["messages"][1]["tool_calls"][0],
                        serde_json::json!({
                            "id":"call-preserved-7","type":"function",
                            "function":{"name":"help__lookup","arguments":"{\"topic\":\"resolve\"}"}
                        })
                    );
                    assert_eq!(body["messages"][2]["tool_call_id"], "call-preserved-7");
                    assert_eq!(
                        body["messages"][2]["content"],
                        "\"An explicit version is required.\""
                    );
                    if returned_tool {
                        vec![serde_json::json!({"choices":[{"delta":{"tool_calls":[{
                            "index":0,"id":"forbidden-guided-call","type":"function",
                            "function":{"name":"help__lookup","arguments":"{\"topic\":\"resolve\"}"}
                        }]},"finish_reason":"tool_calls"}]})]
                    } else {
                        vec![
                            serde_json::json!({"choices":[{"delta":{"content":"Choose an explicit version."},"finish_reason":"stop"}]}),
                        ]
                    }
                };
                let mut response = events
                    .into_iter()
                    .map(|event| format!("data: {event}\n\n"))
                    .collect::<String>();
                response.push_str("data: [DONE]\n\n");
                write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", response.len(), response).unwrap();
            }
        });
        let profile = AgentProfile {
            backend: "router".into(),
            base_url: format!("http://{address}"),
            loopback_ca_file: None,
            model: "synthetic/model".into(),
            api_key_env: "UNUSED_OFFLINE_TEST_KEY".into(),
            context_policy: "synthetic-demo".into(),
            allow_user_text: true,
            max_request_bytes: 131072,
            max_response_bytes: 131072,
            max_output_tokens: 1024,
            allowed_metadata_fields: Vec::new(),
            max_input_price: None,
            max_output_price: None,
            reasoning_enabled: false,
            max_tool_calls: 2,
            max_request_seconds: 5,
            max_context_chars: 8000,
            max_response_chars: 8000,
            max_cost_usd: Some(0.01),
            allowed_providers: vec!["synthetic".into()],
            allow_provider_fallbacks: false,
        };
        let mut provider = RouterProvider {
            request_id: None,
            profile,
            api_key: "synthetic-offline-key".into(),
            client: reqwest::Client::builder()
                .pool_max_idle_per_host(0)
                .redirect(reqwest::redirect::Policy::none())
                .timeout(Duration::from_secs(3))
                .build()
                .unwrap(),
        };
        let tools: Vec<_> = crate::tools::tool_schemas()
            .into_iter()
            .filter(|tool| tool.name == "help.lookup")
            .collect();
        let mut messages = vec![ModelMessage {
            role: "user".into(),
            content: serde_json::json!("Look up resolve help."),
            tool_call_id: None,
            tool_calls: None,
        }];
        let mut events = Vec::new();
        provider
            .stream(
                ModelRequest {
                    messages: messages.clone(),
                    tools: tools.clone(),
                    max_response_chars: 8000,
                },
                &CancellationToken::default(),
                &mut |event| events.push(event),
            )
            .unwrap();
        let call = events
            .into_iter()
            .find_map(|event| match event {
                ProviderEvent::ToolCall(call) => Some(call),
                _ => None,
            })
            .unwrap();
        assert_eq!(call.name, "help.lookup");
        assert_eq!(call.arguments, serde_json::json!({"topic":"resolve"}));
        messages.push(ModelMessage {
            role: "assistant".into(),
            content: serde_json::Value::Null,
            tool_call_id: None,
            tool_calls: Some(vec![call.clone()]),
        });
        messages.push(ModelMessage {
            role: "tool".into(),
            content: serde_json::json!("An explicit version is required."),
            tool_call_id: Some(call.id),
            tool_calls: None,
        });
        let mut text = String::new();
        let mut returned_calls = Vec::new();
        let result = provider.stream(
            ModelRequest {
                messages,
                tools: if guided { Vec::new() } else { tools },
                max_response_chars: 8000,
            },
            &CancellationToken::default(),
            &mut |event| match event {
                ProviderEvent::TextDelta(delta) => text.push_str(&delta),
                ProviderEvent::ToolCall(call) => returned_calls.push(call),
                _ => {}
            },
        );
        server.join().unwrap();
        assert!(returned_calls.is_empty());
        if returned_tool {
            assert!(matches!(result, Err(ProviderError::Protocol(_))));
        } else {
            result.unwrap();
            assert_eq!(text, "Choose an explicit version.");
        }
    }

    #[test]
    fn sse_parser_buffers_split_utf8_and_comments() {
        let mut parser = SseParser::default();
        assert!(parser.push(b": keepalive\n\n").unwrap().is_empty());
        let first = "data: {\"choices\":[{\"delta\":{\"content\":\"\u{00e9}".as_bytes();
        assert!(parser.push(&first[..first.len() - 1]).unwrap().is_empty());
        let rest = [
            first[first.len() - 1],
            b'"',
            b'}',
            b'}',
            b']',
            b'}',
            b'\n',
            b'\n',
        ];
        assert_eq!(parser.push(&rest).unwrap().len(), 1);
    }
}

#[cfg(all(test, feature = "hosted"))]
mod failure_tests {
    use super::*;
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::TcpListener;
    use std::thread;
    use std::time::Duration;
    fn profile(base_url: String) -> AgentProfile {
        serde_json::from_value(serde_json::json!({"backend":"router","base_url":base_url,"model":"synthetic/model","api_key_env":"UNUSED","context_policy":"metadata-only","allow_user_text":true,"max_context_chars":32000,"max_request_seconds":1,"allowed_providers":["fixture"]})).unwrap()
    }
    fn once(
        status: u16,
        body: String,
        delay_ms: u64,
    ) -> (
        RouterProvider,
        std::sync::mpsc::Receiver<String>,
        thread::JoinHandle<()>,
    ) {
        let listener =
            TcpListener::bind("127.0.0.1:0").expect("offline HTTP tests require loopback access");
        let address = listener.local_addr().unwrap();
        let (tx, rx) = std::sync::mpsc::channel();
        let worker = thread::spawn(move || {
            let (mut socket, _) = listener.accept().unwrap();
            socket
                .set_read_timeout(Some(Duration::from_secs(3)))
                .unwrap();
            let mut reader = BufReader::new(&mut socket);
            let mut headers = String::new();
            let mut length = 0;
            loop {
                let mut line = String::new();
                if reader.read_line(&mut line).unwrap() == 0 {
                    return;
                }
                if line == "\r\n" {
                    break;
                }
                if let Some((key, value)) = line.split_once(':')
                    && key.eq_ignore_ascii_case("content-length")
                {
                    length = value.trim().parse().unwrap();
                }
                headers.push_str(&line);
            }
            let mut data = vec![0; length];
            reader.read_exact(&mut data).unwrap();
            headers.push_str(std::str::from_utf8(&data).unwrap());
            let _ = tx.send(headers);
            thread::sleep(Duration::from_millis(delay_ms));
            let _ = write!(
                socket,
                "HTTP/1.1 {status} Fixture\r\nContent-Type: text/event-stream\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
        });
        let provider = RouterProvider {
            request_id: None,
            profile: profile(format!("http://{address}")),
            api_key: "fixture-secret".into(),
            client: reqwest::Client::builder()
                .pool_max_idle_per_host(0)
                .redirect(reqwest::redirect::Policy::none())
                .build()
                .unwrap(),
        };
        (provider, rx, worker)
    }
    fn request() -> ModelRequest {
        ModelRequest {
            messages: vec![ModelMessage {
                role: "user".into(),
                content: serde_json::json!(
                    "Inspect product://climate/observations v1; /private/hidden fixture-secret"
                ),
                tool_call_id: None,
                tool_calls: None,
            }],
            tools: crate::tools::tool_schemas(),
            max_response_chars: 8000,
        }
    }
    fn completion(delta: serde_json::Value, finish: &str) -> String {
        format!(
            "data: {}\n\ndata: [DONE]\n\n",
            serde_json::json!({"choices":[{"delta":delta,"finish_reason":finish}]})
        )
    }
    #[test]
    fn status_errors_and_stream_errors_never_expose_remote_bodies() {
        for status in [401, 403, 429, 500, 503, 302] {
            let (mut provider, rx, worker) =
                once(status, "private server response /private/path".into(), 0);
            let error = provider
                .stream(request(), &CancellationToken::default(), &mut |_| {})
                .unwrap_err();
            assert!(!error.to_string().contains("/private"));
            let captured = rx.recv().unwrap();
            assert!(
                captured
                    .to_lowercase()
                    .contains("authorization: bearer fixture-secret")
            );
            let body = captured.split("\r\n").last().unwrap();
            assert!(!body.contains("fixture-secret"));
            assert!(!body.contains("/private/hidden"));
            worker.join().unwrap();
        }
        let (mut provider, _, worker) = once(
            200,
            "data: {\"error\":{\"message\":\"/private/hidden secret\"}}\n\n".into(),
            0,
        );
        let error = provider
            .stream(request(), &CancellationToken::default(), &mut |_| {})
            .unwrap_err();
        assert!(!error.to_string().contains("hidden"));
        worker.join().unwrap();
    }
    #[test]
    fn truncated_multiple_and_oversized_streams_cannot_emit_tools() {
        let call = serde_json::json!({"tool_calls":[{"index":0,"id":"a","function":{"name":"help__lookup","arguments":"{\"topic\":\"resolve\"}"}}]});
        let multiple = serde_json::json!({"tool_calls":[{"index":0,"id":"a","function":{"name":"help__lookup","arguments":"{}"}},{"index":8,"id":"b","function":{"name":"help__lookup","arguments":"{}"}}]});
        for body in [
            format!(
                "data: {}\n\n",
                serde_json::json!({"choices":[{"delta":call}]})
            ),
            completion(call, "length"),
            completion(multiple, "tool_calls"),
            "x".repeat(140000),
        ] {
            let (mut provider, _, worker) = once(200, body, 0);
            let mut calls = 0;
            assert!(
                provider
                    .stream(
                        request(),
                        &CancellationToken::default(),
                        &mut |e| if matches!(e, ProviderEvent::ToolCall(_)) {
                            calls += 1
                        }
                    )
                    .is_err()
            );
            assert_eq!(calls, 0);
            worker.join().unwrap();
        }
    }
    #[test]
    fn deadline_and_cancellation_interrupt_a_silent_socket() {
        for cancel in [false, true] {
            let (mut provider, _, worker) = once(
                200,
                completion(serde_json::json!({"content":"late"}), "stop"),
                1200,
            );
            let token = CancellationToken::default();
            let worker_token = token.clone();
            let canceller = thread::spawn(move || {
                if cancel {
                    thread::sleep(Duration::from_millis(50));
                    worker_token.cancel();
                }
            });
            let started = std::time::Instant::now();
            let result = provider.stream(request(), &token, &mut |_| {});
            assert!(result.is_err());
            assert!(started.elapsed() < Duration::from_millis(if cancel { 500 } else { 1150 }));
            canceller.join().unwrap();
            worker.join().unwrap();
        }
    }
    #[test]
    fn missing_usage_stays_unknown_and_routing_is_captured() {
        let (mut provider, rx, worker) = once(
            200,
            completion(serde_json::json!({"content":"answer"}), "stop"),
            0,
        );
        let mut usage = false;
        provider
            .stream(request(), &CancellationToken::default(), &mut |e| {
                if matches!(e, ProviderEvent::Usage(_)) {
                    usage = true
                }
            })
            .unwrap();
        assert!(!usage);
        let captured = rx.recv().unwrap();
        assert!(captured.contains("require_parameters"));
        assert!(captured.contains("max_tokens"));
        worker.join().unwrap();
    }
}

/// File-backed deterministic replay for demonstrations, never model-quality
/// evidence. `$selected` expands only to an operation already selected locally.
#[derive(Clone)]
pub struct ReplayProvider {
    turns: Arc<Mutex<VecDeque<Vec<ProviderEvent>>>>,
}
impl ReplayProvider {
    pub fn load(path: &std::path::Path) -> Result<Self, ProviderError> {
        use std::io::Read;
        let file = std::fs::File::open(path)
            .map_err(|_| ProviderError::Configuration("replay file unavailable".into()))?;
        let mut bytes = Vec::new();
        file.take(1024 * 1024 + 1)
            .read_to_end(&mut bytes)
            .map_err(|_| ProviderError::Configuration("replay file unreadable".into()))?;
        if bytes.len() > 1024 * 1024 {
            return Err(ProviderError::TooLarge);
        }
        let turns = serde_json::from_slice(&bytes)
            .map_err(|_| ProviderError::Configuration("invalid replay event file".into()))?;
        Ok(Self {
            turns: Arc::new(Mutex::new(turns)),
        })
    }
}
impl ModelProvider for ReplayProvider {
    fn stream(
        &mut self,
        request: ModelRequest,
        cancellation: &CancellationToken,
        emit: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        if cancellation.is_cancelled() {
            return Err(ProviderError::Cancelled);
        }
        let mut events = self
            .turns
            .lock()
            .map_err(|_| ProviderError::Protocol("replay lock failed".into()))?
            .pop_front()
            .ok_or_else(|| ProviderError::Protocol("replay exhausted".into()))?;
        let handles = request
            .messages
            .iter()
            .rev()
            .filter_map(|m| m.content.as_str())
            .find_map(|s| {
                s.split_once("Locally selected handles for this session: ")
                    .and_then(|(_, handles)| {
                        serde_json::from_str::<serde_json::Value>(handles).ok()
                    })
            });
        for event in &mut events {
            if let ProviderEvent::ToolCall(call) = event
                && call.arguments["handle"] == "$selected"
            {
                let handle = handles
                    .as_ref()
                    .and_then(|h| h.pointer("/operations/0/handle"))
                    .cloned()
                    .ok_or_else(|| {
                        ProviderError::Protocol("replay needs a locally selected handle".into())
                    })?;
                call.arguments["handle"] = handle;
            }
        }
        for event in events {
            if cancellation.is_cancelled() {
                return Err(ProviderError::Cancelled);
            }
            emit(event);
        }
        emit(ProviderEvent::Usage(ProviderUsage {
            model: None,
            provider: None,
            generation_id: None,
            input_tokens: Some(0),
            output_tokens: Some(0),
            cost_usd: Some(0.0),
        }));
        Ok(())
    }
}
#[cfg(feature = "hosted")]
#[derive(Clone)]
pub enum SessionProvider {
    Router(Box<RouterProvider>),
    Replay(ReplayProvider),
}
#[cfg(feature = "hosted")]
impl ModelProvider for SessionProvider {
    fn set_request_id(&mut self, request_id: Option<String>) {
        if let Self::Router(p) = self {
            p.set_request_id(request_id);
        }
    }
    fn set_request_timeout(&mut self, timeout: std::time::Duration) {
        if let Self::Router(p) = self {
            p.set_request_timeout(timeout);
        }
    }
    fn stream(
        &mut self,
        r: ModelRequest,
        c: &CancellationToken,
        e: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        match self {
            Self::Router(p) => p.stream(r, c, e),
            Self::Replay(p) => p.stream(r, c, e),
        }
    }
}
