//! Transport-security helpers for upstream endpoints (reuse HAL policy, T132).

use thiserror::Error;

/// EndpointSecurityError explains why an upstream URL was rejected.
#[derive(Debug, Error, PartialEq, Eq)]
pub enum EndpointSecurityError {
    #[error("could not parse endpoint url {0:?}")]
    Unparseable(String),
    #[error("unsupported endpoint url scheme {0:?}; expected http or https")]
    UnsupportedScheme(String),
    #[error(
        "refusing plaintext http to non-loopback host {host:?}: the bearer API key would be \
         sent in cleartext. Use https, or set CWSO_ROLLOUT_ALLOW_INSECURE_ENDPOINTS=true."
    )]
    InsecureNonLoopback { host: String },
}

pub fn validate_endpoint(
    base_url: &str,
    allow_insecure: bool,
) -> Result<bool, EndpointSecurityError> {
    let (scheme, host) = split_scheme_host(base_url)
        .ok_or_else(|| EndpointSecurityError::Unparseable(base_url.to_string()))?;

    match scheme.as_str() {
        "https" => Ok(false),
        "http" => {
            if is_loopback(&host) {
                Ok(false)
            } else if allow_insecure {
                Ok(true)
            } else {
                Err(EndpointSecurityError::InsecureNonLoopback { host })
            }
        }
        other => Err(EndpointSecurityError::UnsupportedScheme(other.to_string())),
    }
}

/// redact_authorization returns `[REDACTED]` when the header name is authorization (case-insensitive).
pub fn redact_authorization(name: &str, value: &str) -> String {
    if name.eq_ignore_ascii_case("authorization") {
        "[REDACTED]".to_string()
    } else {
        value.to_string()
    }
}

fn split_scheme_host(url: &str) -> Option<(String, String)> {
    let (scheme, rest) = url.split_once("://")?;
    if scheme.is_empty() {
        return None;
    }
    let authority = rest
        .split(['/', '?', '#'])
        .next()
        .filter(|s| !s.is_empty())?;
    let host_port = authority.rsplit_once('@').map_or(authority, |(_, h)| h);
    let host = if let Some(after_bracket) = host_port.strip_prefix('[') {
        let end = after_bracket.find(']')?;
        after_bracket[..end].to_string()
    } else {
        host_port.split(':').next().unwrap_or(host_port).to_string()
    };
    if host.is_empty() {
        return None;
    }
    Some((scheme.to_ascii_lowercase(), host.to_ascii_lowercase()))
}

fn is_loopback(host: &str) -> bool {
    host == "localhost" || host.ends_with(".localhost") || host == "::1" || host.starts_with("127.")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn redacts_bearer_tokens() {
        assert_eq!(
            redact_authorization("Authorization", "Bearer secret"),
            "[REDACTED]"
        );
        assert_eq!(redact_authorization("X-Request-Id", "abc"), "abc");
    }

    #[test]
    fn rejects_plaintext_remote_upstream() {
        let err = validate_endpoint("http://remote.example/v1", false).unwrap_err();
        assert!(matches!(
            err,
            EndpointSecurityError::InsecureNonLoopback { .. }
        ));
    }
}
