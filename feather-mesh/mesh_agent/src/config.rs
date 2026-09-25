use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use thiserror::Error;

pub const AGENT_CONFIG_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Error)]
pub enum ProfileError {
    #[error("agent configuration error: {0}")]
    Io(#[from] std::io::Error),
    #[error("agent configuration parse error: {0}")]
    Toml(#[from] toml::de::Error),
    #[error("invalid agent profile '{profile}': {message}")]
    Invalid { profile: String, message: String },
    #[error("agent profile '{0}' does not exist")]
    Missing(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AgentConfig {
    pub schema_version: u32,
    #[serde(default)]
    pub default_profile: Option<String>,
    pub profiles: BTreeMap<String, AgentProfile>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AgentProfile {
    pub backend: String,
    pub base_url: String,
    /// Additional trust only for the installed HTTPS IPv4-loopback broker
    /// adapter. This never extends trust for external router endpoints.
    #[serde(default)]
    pub loopback_ca_file: Option<PathBuf>,
    pub model: String,
    pub api_key_env: String,
    /// An explicit name for the locally enforced serialization policy.
    pub context_policy: String,
    #[serde(default = "default_max_tool_calls")]
    pub max_tool_calls: u8,
    #[serde(default = "default_request_seconds")]
    pub max_request_seconds: u64,
    #[serde(default = "default_context_chars")]
    pub max_context_chars: usize,
    #[serde(default = "default_response_chars")]
    pub max_response_chars: usize,
    #[serde(default)]
    pub max_cost_usd: Option<f64>,
    #[serde(default)]
    pub allowed_providers: Vec<String>,
    #[serde(default)]
    pub allow_provider_fallbacks: bool,
    /// Sending the user's typed request is a deliberate profile policy. Tool
    /// results are still separately filtered by the harness.
    #[serde(default)]
    pub allow_user_text: bool,
    #[serde(default = "default_request_bytes")]
    pub max_request_bytes: usize,
    #[serde(default = "default_request_bytes")]
    pub max_response_bytes: usize,
    #[serde(default = "default_output_tokens")]
    pub max_output_tokens: u32,
    #[serde(default)]
    pub allowed_metadata_fields: Vec<String>,
    #[serde(default)]
    pub max_input_price: Option<f64>,
    #[serde(default)]
    pub max_output_price: Option<f64>,
    #[serde(default)]
    pub reasoning_enabled: bool,
}

fn default_request_bytes() -> usize {
    128 * 1024
}
fn default_output_tokens() -> u32 {
    1024
}

fn default_max_tool_calls() -> u8 {
    8
}
fn default_request_seconds() -> u64 {
    120
}
fn default_context_chars() -> usize {
    8_000
}
fn default_response_chars() -> usize {
    8_000
}

impl AgentConfig {
    pub fn load(path: impl AsRef<Path>) -> Result<Self, ProfileError> {
        let config: Self = toml::from_str(&fs::read_to_string(path)?)?;
        if config.schema_version != AGENT_CONFIG_SCHEMA_VERSION {
            return Err(ProfileError::Invalid {
                profile: "<config>".into(),
                message: format!("schema_version must be {AGENT_CONFIG_SCHEMA_VERSION}"),
            });
        }
        for (name, profile) in &config.profiles {
            profile.validate(name)?;
        }
        Ok(config)
    }

    pub fn selected(&self, requested: Option<&str>) -> Result<(&str, &AgentProfile), ProfileError> {
        let name = requested
            .map(str::to_owned)
            .or_else(|| self.default_profile.clone())
            .ok_or_else(|| ProfileError::Missing("no default_profile configured".into()))?;
        self.profiles
            .get_key_value(&name)
            .map(|(name, profile)| (name.as_str(), profile))
            .ok_or(ProfileError::Missing(name))
    }

    pub fn default_path() -> Option<PathBuf> {
        let base = std::env::var_os("XDG_CONFIG_HOME")
            .map(PathBuf::from)
            .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".config")))?;
        Some(base.join("feam/agent.toml"))
    }
}

impl AgentProfile {
    pub fn validate(&self, profile: &str) -> Result<(), ProfileError> {
        if self.backend != "router" {
            return Err(invalid(profile, "backend must be 'router' in stage 1"));
        }
        if self.model.trim().is_empty() || self.api_key_env.trim().is_empty() {
            return Err(invalid(profile, "model and api_key_env must not be blank"));
        }
        if !(1..=8).contains(&self.max_tool_calls) {
            return Err(invalid(profile, "max_tool_calls must be between 1 and 8"));
        }
        if self.max_request_seconds == 0 || self.max_request_seconds > 120 {
            return Err(invalid(
                profile,
                "max_request_seconds must be between 1 and 120",
            ));
        }
        if self.max_context_chars == 0 || self.max_response_chars == 0 {
            return Err(invalid(
                profile,
                "context and response limits must be positive",
            ));
        }
        if self
            .max_cost_usd
            .is_some_and(|value| !value.is_finite() || value <= 0.0)
        {
            return Err(invalid(
                profile,
                "max_cost_usd must be a positive finite value",
            ));
        }
        if !["synthetic-demo", "metadata-only"].contains(&self.context_policy.as_str()) {
            return Err(invalid(
                profile,
                "context_policy must be synthetic-demo or metadata-only",
            ));
        }
        if !(1024..=1024 * 1024).contains(&self.max_request_bytes)
            || !(1024..=1024 * 1024).contains(&self.max_response_bytes)
            || !(1..=4096).contains(&self.max_output_tokens)
            || self.max_context_chars > 256 * 1024
            || self.max_response_chars > 256 * 1024
        {
            return Err(invalid(
                profile,
                "request/response/context/output bounds exceed supported limits",
            ));
        }
        for field in &self.allowed_metadata_fields {
            if ![
                "name",
                "description",
                "limitations",
                "intended_use",
                "usage_policy",
                "lineage",
                "owner_team",
            ]
            .contains(&field.as_str())
            {
                return Err(invalid(
                    profile,
                    "metadata field is not in the disclosure allowlist",
                ));
            }
        }
        for price in [self.max_input_price, self.max_output_price]
            .into_iter()
            .flatten()
        {
            if !price.is_finite() || price < 0.0 {
                return Err(invalid(
                    profile,
                    "price caps must be finite and nonnegative",
                ));
            }
        }
        validate_endpoint(&self.base_url).map_err(|message| invalid(profile, message))?;
        if let Some(path) = &self.loopback_ca_file {
            let authority = self
                .base_url
                .strip_prefix("https://")
                .map(|rest| rest.split('/').next().unwrap_or_default());
            let exact_loopback = authority.is_some_and(|authority| {
                authority == "127.0.0.1"
                    || authority
                        .strip_prefix("127.0.0.1:")
                        .is_some_and(|port| port.parse::<u16>().is_ok_and(|value| value > 0))
            });
            if !path.is_absolute() || !exact_loopback {
                return Err(invalid(
                    profile,
                    "loopback_ca_file requires an absolute certificate path and HTTPS 127.0.0.1 endpoint",
                ));
            }
        }
        Ok(())
    }
}

fn invalid(profile: &str, message: impl Into<String>) -> ProfileError {
    ProfileError::Invalid {
        profile: profile.into(),
        message: message.into(),
    }
}

fn validate_endpoint(value: &str) -> Result<(), String> {
    let secure = value.starts_with("https://");
    let loopback = ["http://127.0.0.1", "http://localhost"]
        .iter()
        .any(|prefix| {
            value.strip_prefix(prefix).is_some_and(|rest| {
                rest.is_empty() || rest.starts_with(':') || rest.starts_with('/')
            })
        });
    if !secure && !loopback {
        return Err("base_url must use HTTPS (except loopback development endpoints)".into());
    }
    let authority = value
        .split_once("://")
        .map(|(_, rest)| rest.split('/').next().unwrap_or_default())
        .unwrap_or_default();
    if value.contains('?')
        || value.contains('#')
        || authority.is_empty()
        || authority.contains('@')
        || authority.contains('#')
        || authority.contains('?')
    {
        return Err("base_url must have a host and no credentials, fragment, or query".into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_a_bounded_tls_router_profile() {
        let profile = AgentProfile {
            backend: "router".into(),
            base_url: "https://openrouter.ai/api/v1".into(),
            loopback_ca_file: None,
            model: "example/model".into(),
            api_key_env: "FEAM_ROUTER_API_KEY".into(),
            context_policy: "synthetic-demo".into(),
            max_tool_calls: 8,
            max_request_seconds: 120,
            max_context_chars: 8000,
            max_response_chars: 8000,
            max_cost_usd: Some(1.0),
            allowed_providers: vec!["example".into()],
            allow_provider_fallbacks: false,
            allow_user_text: true,
            max_request_bytes: 131072,
            max_response_bytes: 131072,
            max_output_tokens: 1024,
            allowed_metadata_fields: Vec::new(),
            max_input_price: None,
            max_output_price: None,
            reasoning_enabled: false,
        };
        profile.validate("demo").unwrap();
        assert!(
            AgentProfile {
                base_url: "http://router.test".into(),
                ..profile
            }
            .validate("demo")
            .is_err()
        );
    }

    #[test]
    fn extra_ca_is_scoped_to_exact_https_loopback() {
        let mut profile: AgentProfile = serde_json::from_value(serde_json::json!({
            "backend":"router", "base_url":"https://127.0.0.1:8443/v1",
            "model":"fixture", "api_key_env":"UNUSED", "context_policy":"metadata-only",
            "loopback_ca_file":"/run/feam/model/ca.pem"
        }))
        .unwrap();
        profile.validate("phase1-demo").unwrap();
        for endpoint in [
            "https://openrouter.ai/api/v1",
            "http://127.0.0.1:8443/v1",
            "https://localhost:8443/v1",
            "https://127.0.0.1.invalid/v1",
            "https://127.0.0.1:8443@outside.invalid/v1",
            "https://127.0.0.1:0/v1",
        ] {
            profile.base_url = endpoint.into();
            assert!(profile.validate("phase1-demo").is_err(), "{endpoint}");
        }
        profile.base_url = "https://127.0.0.1:8443/v1".into();
        profile.loopback_ca_file = Some("relative.pem".into());
        assert!(profile.validate("phase1-demo").is_err());
    }
}
