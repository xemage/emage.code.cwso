//! Transport-security policy for accelerator endpoints (T093).
//!
//! Accelerator adapters send a bearer API key (`authorization: Bearer …`) to an
//! OpenAI-compatible endpoint. Over plaintext `http://` to a remote host that key — and the
//! prompt/response payload — travels in the clear. This module enforces that non-loopback
//! endpoints use `https://`, refusing to register an insecure remote endpoint unless the
//! operator explicitly opts in via `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true`.
//!
//! Loopback `http://` (e.g. a vLLM sidecar on `localhost`) is always allowed: the traffic
//! never leaves the host, which matches the common single-node deployment.

/// EndpointSecurityError explains why an accelerator endpoint URL was rejected.
#[derive(Debug, thiserror::Error, PartialEq, Eq)]
pub enum EndpointSecurityError {
    #[error("could not parse endpoint url {0:?}")]
    Unparseable(String),
    #[error("unsupported endpoint url scheme {0:?}; expected http or https")]
    UnsupportedScheme(String),
    #[error(
        "refusing plaintext http to non-loopback host {host:?}: the bearer API key would be \
         sent in cleartext. Use https, or set CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true to override."
    )]
    InsecureNonLoopback { host: String },
}

/// validate_endpoint enforces the transport-security policy for an accelerator base URL.
///
/// Returns:
/// * `Ok(false)` — endpoint is secure (https, or http to loopback); register normally.
/// * `Ok(true)`  — endpoint is insecure (http to a non-loopback host) but allowed because
///   `allow_insecure` is set; the caller SHOULD log a warning.
/// * `Err(..)`   — endpoint is rejected; the caller MUST NOT register it.
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

/// split_scheme_host extracts the lowercased `(scheme, host)` from a URL, dropping any
/// userinfo, port, path, query, and fragment. IPv6 literals in `[..]` are unwrapped.
fn split_scheme_host(url: &str) -> Option<(String, String)> {
    let (scheme, rest) = url.split_once("://")?;
    if scheme.is_empty() {
        return None;
    }

    // Authority ends at the first '/', '?', or '#'.
    let authority = rest
        .split(['/', '?', '#'])
        .next()
        .filter(|s| !s.is_empty())?;

    // Strip optional userinfo (user:pass@).
    let host_port = authority.rsplit_once('@').map_or(authority, |(_, h)| h);

    let host = if let Some(after_bracket) = host_port.strip_prefix('[') {
        // IPv6 literal: take everything up to the closing ']'.
        let end = after_bracket.find(']')?;
        after_bracket[..end].to_string()
    } else {
        // host[:port] — host is everything before the first ':'.
        host_port.split(':').next().unwrap_or(host_port).to_string()
    };

    if host.is_empty() {
        return None;
    }
    Some((scheme.to_ascii_lowercase(), host.to_ascii_lowercase()))
}

/// is_loopback reports whether a host refers to the local machine and therefore never
/// exposes plaintext traffic to the network.
fn is_loopback(host: &str) -> bool {
    host == "localhost" || host.ends_with(".localhost") || host == "::1" || host.starts_with("127.")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn https_is_always_allowed() {
        assert_eq!(
            validate_endpoint("https://api.groq.com/openai/v1", false),
            Ok(false)
        );
        assert_eq!(
            validate_endpoint("https://vllm.internal:8000/v1", false),
            Ok(false)
        );
    }

    #[test]
    fn http_loopback_is_allowed() {
        for url in [
            "http://localhost:8000/v1",
            "http://127.0.0.1:8000/v1",
            "http://127.5.5.5/v1",
            "http://[::1]:8000/v1",
            "http://svc.localhost/v1",
        ] {
            assert_eq!(validate_endpoint(url, false), Ok(false), "url={url}");
        }
    }

    #[test]
    fn http_non_loopback_is_rejected_by_default() {
        let err = validate_endpoint("http://vllm.internal:8000/v1", false).unwrap_err();
        assert_eq!(
            err,
            EndpointSecurityError::InsecureNonLoopback {
                host: "vllm.internal".to_string()
            }
        );
    }

    #[test]
    fn http_non_loopback_allowed_with_override() {
        assert_eq!(validate_endpoint("http://10.0.0.5:8000/v1", true), Ok(true));
    }

    #[test]
    fn userinfo_and_port_are_stripped() {
        let err = validate_endpoint("http://user:pass@remote.example:9000/v1", false).unwrap_err();
        assert_eq!(
            err,
            EndpointSecurityError::InsecureNonLoopback {
                host: "remote.example".to_string()
            }
        );
    }

    #[test]
    fn unsupported_scheme_is_rejected() {
        assert_eq!(
            validate_endpoint("ftp://example.com/v1", true),
            Err(EndpointSecurityError::UnsupportedScheme("ftp".to_string()))
        );
    }

    #[test]
    fn unparseable_url_is_rejected() {
        assert_eq!(
            validate_endpoint("not-a-url", false),
            Err(EndpointSecurityError::Unparseable("not-a-url".to_string()))
        );
    }

    #[test]
    fn ipv6_non_loopback_is_rejected() {
        let err = validate_endpoint("http://[2001:db8::1]:8000/v1", false).unwrap_err();
        assert_eq!(
            err,
            EndpointSecurityError::InsecureNonLoopback {
                host: "2001:db8::1".to_string()
            }
        );
    }
}
