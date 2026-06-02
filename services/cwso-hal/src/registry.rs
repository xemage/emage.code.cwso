use std::collections::{BTreeMap, HashSet};

use crate::backend::{
    Completion, Health, HealthState, InferenceBackend, InferenceRequest, ProviderCapability,
};
use crate::cpu::{CpuBaselineBackend, CPU_BASELINE_PROVIDER_ID};

/// AttemptRecord is one entry in the per-dispatch audit trail.
#[derive(Debug, Clone)]
pub struct AttemptRecord {
    pub provider_id: String,
    /// outcome is "served", a failure-class wire string, "skipped_unhealthy", or
    /// "unknown_provider".
    pub outcome: String,
}

/// FallbackOutcome records how a dispatch resolved, including any fallback hops taken.
#[derive(Debug, Clone)]
pub struct FallbackOutcome {
    pub completion: Completion,
    pub served_by: String,
    pub requested_provider: String,
    pub fallback_count: u32,
    pub attempts: Vec<AttemptRecord>,
}

/// RegistryError is returned when no backend could serve the request.
#[derive(Debug, thiserror::Error)]
pub enum RegistryError {
    #[error("no backend available to serve the request")]
    Exhausted { attempts: Vec<AttemptRecord> },
}

/// BackendRegistry owns the available backends and performs deterministic
/// dispatch-with-fallback, mirroring the Go policy engine's `FallbackOnFailure`: try the
/// selected provider, then each provider in the ranked fallback chain, and finally the
/// CPU baseline as the terminal-safe hop. A non-retryable failure (e.g. invalid request)
/// stops the walk immediately rather than masking the error with the baseline.
pub struct BackendRegistry {
    backends: BTreeMap<String, Box<dyn InferenceBackend>>,
}

impl BackendRegistry {
    pub fn new() -> Self {
        Self {
            backends: BTreeMap::new(),
        }
    }

    /// with_cpu_baseline returns a registry that always contains the terminal-safe baseline.
    pub fn with_cpu_baseline() -> Self {
        let mut registry = Self::new();
        registry.register(Box::new(CpuBaselineBackend::new()));
        registry
    }

    pub fn register(&mut self, backend: Box<dyn InferenceBackend>) {
        let id = backend.capabilities().provider_id;
        self.backends.insert(id, backend);
    }

    pub fn provider_ids(&self) -> Vec<String> {
        self.backends.keys().cloned().collect()
    }

    pub fn capabilities(&self) -> Vec<ProviderCapability> {
        self.backends.values().map(|b| b.capabilities()).collect()
    }

    pub fn health(&self, provider_id: &str) -> Option<Health> {
        self.backends.get(provider_id).map(|b| b.health())
    }

    /// dispatch attempts the selected provider, then the fallback chain, then the CPU
    /// baseline, returning the first successful completion or an exhaustion error.
    pub fn dispatch(
        &self,
        selected_provider: &str,
        fallback_chain: &[String],
        req: &InferenceRequest,
    ) -> Result<FallbackOutcome, RegistryError> {
        let order = build_order(selected_provider, fallback_chain);
        let requested_provider = selected_provider.to_string();
        let mut attempts: Vec<AttemptRecord> = Vec::new();

        for provider_id in &order {
            let backend = match self.backends.get(provider_id) {
                Some(backend) => backend.as_ref(),
                None => {
                    attempts.push(AttemptRecord {
                        provider_id: provider_id.clone(),
                        outcome: "unknown_provider".to_string(),
                    });
                    continue;
                }
            };

            if backend.health().state == HealthState::Unavailable {
                attempts.push(AttemptRecord {
                    provider_id: provider_id.clone(),
                    outcome: "skipped_unhealthy".to_string(),
                });
                continue;
            }

            match backend.infer(req) {
                Ok(completion) => {
                    attempts.push(AttemptRecord {
                        provider_id: provider_id.clone(),
                        outcome: "served".to_string(),
                    });
                    let fallback_count = (attempts.len() as u32).saturating_sub(1);
                    return Ok(FallbackOutcome {
                        completion,
                        served_by: provider_id.clone(),
                        requested_provider,
                        fallback_count,
                        attempts,
                    });
                }
                Err(failure) => {
                    attempts.push(AttemptRecord {
                        provider_id: provider_id.clone(),
                        outcome: failure.class.as_wire().to_string(),
                    });
                    if !failure.class.retryable_via_fallback() {
                        // Terminal failure: do not mask a caller error with the baseline.
                        return Err(RegistryError::Exhausted { attempts });
                    }
                }
            }
        }

        Err(RegistryError::Exhausted { attempts })
    }
}

impl Default for BackendRegistry {
    fn default() -> Self {
        Self::with_cpu_baseline()
    }
}

/// build_order produces the deterministic attempt order: selected → fallback chain →
/// cpu-baseline, de-duplicated while preserving first occurrence.
fn build_order(selected: &str, fallback_chain: &[String]) -> Vec<String> {
    let mut order: Vec<String> = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();

    let candidates = std::iter::once(selected)
        .chain(fallback_chain.iter().map(String::as_str))
        .chain(std::iter::once(CPU_BASELINE_PROVIDER_ID));

    for candidate in candidates {
        if !candidate.is_empty() && seen.insert(candidate.to_string()) {
            order.push(candidate.to_string());
        }
    }
    order
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::{
        BackendFailure, Completion, FailureClass, HealthState, ProviderCapability,
    };

    /// StubBackend is a configurable test backend.
    struct StubBackend {
        id: String,
        health: HealthState,
        failure: Option<FailureClass>,
    }

    impl StubBackend {
        fn serving(id: &str) -> Self {
            Self {
                id: id.to_string(),
                health: HealthState::Healthy,
                failure: None,
            }
        }
        fn failing(id: &str, class: FailureClass) -> Self {
            Self {
                id: id.to_string(),
                health: HealthState::Healthy,
                failure: Some(class),
            }
        }
        fn unavailable(id: &str) -> Self {
            Self {
                id: id.to_string(),
                health: HealthState::Unavailable,
                failure: None,
            }
        }
    }

    impl InferenceBackend for StubBackend {
        fn capabilities(&self) -> ProviderCapability {
            ProviderCapability {
                provider_id: self.id.clone(),
                contract_version: "dispatch.provider/v2".to_string(),
                health_state: self.health.as_wire().to_string(),
                latency_class: "fast".to_string(),
                cost_class: "high".to_string(),
                queue_depth: 0,
                supported_workload_tags: vec!["inference-heavy".to_string()],
                reliability_class: "gold".to_string(),
                feature_flags: vec![],
            }
        }
        fn health(&self) -> Health {
            Health {
                state: self.health,
                queue_depth: 0,
                detail: None,
            }
        }
        fn infer(&self, _req: &InferenceRequest) -> Result<Completion, BackendFailure> {
            match self.failure {
                Some(class) => Err(BackendFailure::new(class, "stub failure")),
                None => Ok(Completion {
                    provider_id: self.id.clone(),
                    output: format!("served by {}", self.id),
                    tokens_in: 1,
                    tokens_out: 1,
                    latency_ms: 0,
                    deterministic: false,
                }),
            }
        }
    }

    fn req() -> InferenceRequest {
        InferenceRequest {
            request_id: "r".to_string(),
            workload_tags: vec![],
            prompt: "do work".to_string(),
            context_tokens: 0,
            max_output_tokens: 0,
            latency_class: "batch".to_string(),
        }
    }

    #[test]
    fn selected_backend_serves_with_no_fallback() {
        let mut reg = BackendRegistry::with_cpu_baseline();
        reg.register(Box::new(StubBackend::serving("gpu-a")));

        let outcome = reg.dispatch("gpu-a", &[], &req()).expect("dispatch");
        assert_eq!(outcome.served_by, "gpu-a");
        assert_eq!(outcome.fallback_count, 0);
    }

    #[test]
    fn retryable_failure_falls_back_to_baseline() {
        let mut reg = BackendRegistry::with_cpu_baseline();
        reg.register(Box::new(StubBackend::failing(
            "gpu-a",
            FailureClass::Unavailable,
        )));

        let outcome = reg.dispatch("gpu-a", &[], &req()).expect("dispatch");
        assert_eq!(outcome.served_by, CPU_BASELINE_PROVIDER_ID);
        assert!(outcome.fallback_count >= 1);
        assert_eq!(outcome.requested_provider, "gpu-a");
    }

    #[test]
    fn fallback_chain_is_honored_in_order() {
        let mut reg = BackendRegistry::with_cpu_baseline();
        reg.register(Box::new(StubBackend::failing(
            "gpu-a",
            FailureClass::Timeout,
        )));
        reg.register(Box::new(StubBackend::serving("lpu-b")));

        let outcome = reg
            .dispatch("gpu-a", &["lpu-b".to_string()], &req())
            .expect("dispatch");
        assert_eq!(outcome.served_by, "lpu-b");
    }

    #[test]
    fn unhealthy_backend_is_skipped() {
        let mut reg = BackendRegistry::with_cpu_baseline();
        reg.register(Box::new(StubBackend::unavailable("gpu-a")));

        let outcome = reg.dispatch("gpu-a", &[], &req()).expect("dispatch");
        assert_eq!(outcome.served_by, CPU_BASELINE_PROVIDER_ID);
        assert_eq!(outcome.attempts[0].outcome, "skipped_unhealthy");
    }

    #[test]
    fn non_retryable_failure_stops_walk() {
        let mut reg = BackendRegistry::with_cpu_baseline();
        reg.register(Box::new(StubBackend::failing(
            "gpu-a",
            FailureClass::InvalidRequest,
        )));

        let err = reg
            .dispatch("gpu-a", &[], &req())
            .expect_err("must not mask invalid request");
        match err {
            RegistryError::Exhausted { attempts } => {
                assert_eq!(attempts.last().unwrap().outcome, "invalid_request");
                // baseline must not have been reached
                assert!(attempts
                    .iter()
                    .all(|a| a.provider_id != CPU_BASELINE_PROVIDER_ID));
            }
        }
    }

    #[test]
    fn unknown_selected_provider_falls_back_to_baseline() {
        let reg = BackendRegistry::with_cpu_baseline();
        let outcome = reg
            .dispatch("does-not-exist", &[], &req())
            .expect("dispatch");
        assert_eq!(outcome.served_by, CPU_BASELINE_PROVIDER_ID);
        assert_eq!(outcome.attempts[0].outcome, "unknown_provider");
    }

    #[test]
    fn build_order_dedupes_and_appends_baseline() {
        let order = build_order("gpu-a", &["gpu-a".to_string(), "lpu-b".to_string()]);
        assert_eq!(order, vec!["gpu-a", "lpu-b", CPU_BASELINE_PROVIDER_ID]);
    }
}
