//! Sidecar configuration from environment (rollout-architecture-v1.md §9).

use anyhow::{Context, Result};
use std::time::Duration;

use crate::security::{validate_endpoint, EndpointSecurityError};

const SOCKET_DEFAULT: &str = "/run/cwso/rollout.sock";
const HTTP_BIND_DEFAULT: &str = "127.0.0.1:8787";
const CAPTURE_QUEUE_DEFAULT: usize = 4096;
const HTTP_TIMEOUT_MS_DEFAULT: u64 = 120_000;

#[derive(Debug, Clone)]
pub struct ProxyConfig {
    pub http_bind: String,
    pub upstream_url: String,
    pub upstream_api_key: Option<String>,
    pub capture_enabled: bool,
    pub capture_queue_capacity: usize,
    pub http_timeout_ms: u64,
    pub allow_insecure_endpoints: bool,
}

#[derive(Debug, Clone)]
pub struct SidecarConfig {
    pub socket_path: String,
    pub proxy: Option<ProxyConfig>,
}

impl SidecarConfig {
    pub fn from_env() -> Result<Self> {
        let socket_path =
            std::env::var("CWSO_ROLLOUT_SOCKET").unwrap_or_else(|_| SOCKET_DEFAULT.to_string());

        let proxy_enabled = env_bool("CWSO_ROLLOUT_PROXY_ENABLED", false);
        let proxy = if proxy_enabled {
            Some(load_proxy_config()?)
        } else {
            None
        };

        Ok(Self { socket_path, proxy })
    }
}

fn load_proxy_config() -> Result<ProxyConfig> {
    let upstream_url = std::env::var("CWSO_ROLLOUT_UPSTREAM_URL")
        .context("CWSO_ROLLOUT_UPSTREAM_URL is required when proxy is enabled")?;
    let allow_insecure = env_bool("CWSO_ROLLOUT_ALLOW_INSECURE_ENDPOINTS", false);
    match validate_endpoint(&upstream_url, allow_insecure) {
        Ok(false) => {}
        Ok(true) => tracing::warn!(
            upstream_url = %upstream_url,
            "rollout upstream uses plaintext http to a non-loopback host"
        ),
        Err(EndpointSecurityError::InsecureNonLoopback { host }) => {
            anyhow::bail!(
                "refusing insecure rollout upstream host {host:?}; use https or \
                 CWSO_ROLLOUT_ALLOW_INSECURE_ENDPOINTS=true"
            );
        }
        Err(other) => anyhow::bail!("invalid CWSO_ROLLOUT_UPSTREAM_URL: {other}"),
    }

    Ok(ProxyConfig {
        http_bind: std::env::var("CWSO_ROLLOUT_HTTP_BIND")
            .unwrap_or_else(|_| HTTP_BIND_DEFAULT.to_string()),
        upstream_url: trim_trailing_slash(upstream_url),
        upstream_api_key: std::env::var("CWSO_ROLLOUT_UPSTREAM_API_KEY")
            .ok()
            .filter(|value| !value.is_empty()),
        capture_enabled: env_bool("CWSO_ROLLOUT_CAPTURE_ENABLED", true),
        capture_queue_capacity: env_usize(
            "CWSO_ROLLOUT_CAPTURE_QUEUE_CAPACITY",
            CAPTURE_QUEUE_DEFAULT,
        ),
        http_timeout_ms: env_u64("CWSO_ROLLOUT_HTTP_TIMEOUT_MS", HTTP_TIMEOUT_MS_DEFAULT),
        allow_insecure_endpoints: allow_insecure,
    })
}

impl ProxyConfig {
    pub fn http_timeout(&self) -> Duration {
        Duration::from_millis(self.http_timeout_ms)
    }

    pub fn upstream_chat_path(&self) -> String {
        format!("{}/v1/chat/completions", self.upstream_url)
    }

    pub fn upstream_responses_path(&self) -> String {
        format!("{}/v1/responses", self.upstream_url)
    }

    /// All client providers normalize to OpenAI Chat Completions for upstream capture.
    pub fn upstream_path_for(&self, provider: crate::provider::Provider) -> String {
        match provider {
            crate::provider::Provider::OpenAiChat
            | crate::provider::Provider::OpenAiResponses
            | crate::provider::Provider::AnthropicMessages
            | crate::provider::Provider::GoogleGenerateContent => self.upstream_chat_path(),
            crate::provider::Provider::Unknown => self.upstream_chat_path(),
        }
    }
}

fn trim_trailing_slash(url: String) -> String {
    url.trim_end_matches('/').to_string()
}

fn env_bool(key: &str, default: bool) -> bool {
    std::env::var(key)
        .ok()
        .map(|value| {
            matches!(
                value.trim().to_ascii_lowercase().as_str(),
                "1" | "true" | "yes" | "on"
            )
        })
        .unwrap_or(default)
}

fn env_u64(key: &str, default: u64) -> u64 {
    std::env::var(key)
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}

fn env_usize(key: &str, default: usize) -> usize {
    std::env::var(key)
        .ok()
        .and_then(|value| value.parse::<usize>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn upstream_chat_path_joins_cleanly() {
        let cfg = ProxyConfig {
            http_bind: HTTP_BIND_DEFAULT.to_string(),
            upstream_url: "http://127.0.0.1:8000".to_string(),
            upstream_api_key: None,
            capture_enabled: true,
            capture_queue_capacity: 8,
            http_timeout_ms: 1_000,
            allow_insecure_endpoints: false,
        };
        assert_eq!(
            cfg.upstream_chat_path(),
            "http://127.0.0.1:8000/v1/chat/completions"
        );
    }

    #[test]
    fn upstream_path_for_responses_uses_chat_after_normalize() {
        use crate::provider::Provider;
        let cfg = ProxyConfig {
            http_bind: HTTP_BIND_DEFAULT.to_string(),
            upstream_url: "http://127.0.0.1:8000".to_string(),
            upstream_api_key: None,
            capture_enabled: true,
            capture_queue_capacity: 8,
            http_timeout_ms: 1_000,
            allow_insecure_endpoints: false,
        };
        assert_eq!(
            cfg.upstream_path_for(Provider::OpenAiResponses),
            cfg.upstream_chat_path()
        );
    }
}
