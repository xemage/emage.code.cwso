use serde::{Deserialize, Serialize};

/// HAL provider contract version advertised by every backend (design T081).
pub const CONTRACT_VERSION: &str = "dispatch.provider/v2";

/// HealthState mirrors the orchestrator capability-registry health vocabulary so
/// snapshots round-trip cleanly between the Rust HAL and the Go control plane.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum HealthState {
    Healthy,
    Degraded,
    Unavailable,
}

impl HealthState {
    pub fn as_wire(self) -> &'static str {
        match self {
            HealthState::Healthy => "healthy",
            HealthState::Degraded => "degraded",
            HealthState::Unavailable => "unavailable",
        }
    }
}

/// Health is a backend's current readiness snapshot.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Health {
    pub state: HealthState,
    pub queue_depth: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
}

/// ProviderCapability is the policy-facing capability record, aligned field-for-field
/// with the Go `dispatch.ProviderCapability` struct.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderCapability {
    pub provider_id: String,
    pub contract_version: String,
    pub health_state: String,
    pub latency_class: String,
    pub cost_class: String,
    pub queue_depth: u32,
    pub supported_workload_tags: Vec<String>,
    pub reliability_class: String,
    pub feature_flags: Vec<String>,
}

/// InferenceRequest is one unit of work routed to a backend.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InferenceRequest {
    #[serde(default)]
    pub request_id: String,
    #[serde(default)]
    pub workload_tags: Vec<String>,
    pub prompt: String,
    #[serde(default)]
    pub context_tokens: u32,
    #[serde(default)]
    pub max_output_tokens: u32,
    #[serde(default)]
    pub latency_class: String,
}

/// Completion is a successful backend result.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Completion {
    pub provider_id: String,
    pub output: String,
    pub tokens_in: u32,
    pub tokens_out: u32,
    pub latency_ms: u64,
    pub deterministic: bool,
}

/// FailureClass categorizes a backend failure so the control plane can decide whether a
/// different backend should be attempted.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FailureClass {
    Unavailable,
    Timeout,
    Overloaded,
    InvalidRequest,
    Internal,
}

impl FailureClass {
    pub fn as_wire(self) -> &'static str {
        match self {
            FailureClass::Unavailable => "unavailable",
            FailureClass::Timeout => "timeout",
            FailureClass::Overloaded => "overloaded",
            FailureClass::InvalidRequest => "invalid_request",
            FailureClass::Internal => "internal",
        }
    }

    /// retryable_via_fallback reports whether attempting a different backend may succeed.
    /// `InvalidRequest` is terminal — the request itself is malformed, so masking it with
    /// the baseline would hide a caller bug.
    pub fn retryable_via_fallback(self) -> bool {
        !matches!(self, FailureClass::InvalidRequest)
    }

    /// to_health_state maps an observed failure to the health it implies for the backend.
    /// Transient pressure (timeout/overloaded) degrades rather than disables the backend;
    /// connectivity / server faults mark it unavailable so the router skips it. A malformed
    /// request says nothing bad about the backend, so it is treated as (at worst) degraded.
    pub fn to_health_state(self) -> HealthState {
        match self {
            FailureClass::Timeout | FailureClass::Overloaded => HealthState::Degraded,
            FailureClass::Unavailable | FailureClass::Internal => HealthState::Unavailable,
            FailureClass::InvalidRequest => HealthState::Degraded,
        }
    }
}

/// BackendFailure is the error type returned by an adapter's `infer`.
#[derive(Debug, Clone, thiserror::Error)]
#[error("{class} backend failure: {message}")]
pub struct BackendFailure {
    pub class: FailureClass,
    pub message: String,
}

impl std::fmt::Display for FailureClass {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_wire())
    }
}

impl BackendFailure {
    pub fn new(class: FailureClass, message: impl Into<String>) -> Self {
        Self {
            class,
            message: message.into(),
        }
    }
}

/// InferenceBackend is the Hardware Abstraction Layer contract every adapter implements.
/// Adapters may be in-process (e.g. the CPU baseline) or thin shims over an
/// out-of-process engine; the control plane treats them uniformly.
pub trait InferenceBackend: Send + Sync {
    /// capabilities returns the policy-facing capability record for routing.
    fn capabilities(&self) -> ProviderCapability;

    /// health returns the backend's current readiness. This MUST be cheap (no network
    /// I/O): it is read on the dispatch hot path, so it should return a cached snapshot.
    fn health(&self) -> Health;

    /// probe performs an *active* readiness check (may do network I/O), refreshes the
    /// backend's cached health, and returns the fresh snapshot. It is intended to be
    /// called periodically by a background prober — never on the dispatch hot path. The
    /// default delegates to the cheap `health()` for backends with no remote dependency
    /// (e.g. the CPU baseline), which are always ready.
    fn probe(&self) -> Health {
        self.health()
    }

    /// infer executes one request, returning a completion or a classified failure.
    fn infer(&self, req: &InferenceRequest) -> Result<Completion, BackendFailure>;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn failure_class_wire_roundtrip() {
        for (class, wire) in [
            (FailureClass::Unavailable, "unavailable"),
            (FailureClass::Timeout, "timeout"),
            (FailureClass::Overloaded, "overloaded"),
            (FailureClass::InvalidRequest, "invalid_request"),
            (FailureClass::Internal, "internal"),
        ] {
            assert_eq!(class.as_wire(), wire);
            assert_eq!(class.to_string(), wire);
        }
    }

    #[test]
    fn only_invalid_request_is_non_retryable() {
        assert!(!FailureClass::InvalidRequest.retryable_via_fallback());
        assert!(FailureClass::Unavailable.retryable_via_fallback());
        assert!(FailureClass::Timeout.retryable_via_fallback());
        assert!(FailureClass::Overloaded.retryable_via_fallback());
        assert!(FailureClass::Internal.retryable_via_fallback());
    }

    #[test]
    fn health_state_wire_values() {
        assert_eq!(HealthState::Healthy.as_wire(), "healthy");
        assert_eq!(HealthState::Degraded.as_wire(), "degraded");
        assert_eq!(HealthState::Unavailable.as_wire(), "unavailable");
    }

    #[test]
    fn failure_class_maps_to_health_state() {
        assert_eq!(
            FailureClass::Timeout.to_health_state(),
            HealthState::Degraded
        );
        assert_eq!(
            FailureClass::Overloaded.to_health_state(),
            HealthState::Degraded
        );
        assert_eq!(
            FailureClass::Unavailable.to_health_state(),
            HealthState::Unavailable
        );
        assert_eq!(
            FailureClass::Internal.to_health_state(),
            HealthState::Unavailable
        );
        assert_eq!(
            FailureClass::InvalidRequest.to_health_state(),
            HealthState::Degraded
        );
    }
}
