use std::time::Instant;

use sha2::{Digest, Sha256};

use crate::backend::{
    BackendFailure, Completion, Health, HealthState, InferenceBackend, InferenceRequest,
    ProviderCapability, CONTRACT_VERSION,
};

/// Stable provider id for the terminal-safe baseline backend.
pub const CPU_BASELINE_PROVIDER_ID: &str = "cpu-baseline";

/// CpuBaselineBackend is the terminal-safe fallback adapter. It is always healthy,
/// requires no accelerator, and produces a deterministic completion that is a pure
/// function of the request — so identical requests always yield identical output. This
/// makes it the guaranteed last hop of any fallback chain and a stable target for
/// replay/testing until real accelerator adapters (T083/T084) land.
#[derive(Debug, Default, Clone)]
pub struct CpuBaselineBackend;

impl CpuBaselineBackend {
    pub fn new() -> Self {
        Self
    }
}

impl InferenceBackend for CpuBaselineBackend {
    fn capabilities(&self) -> ProviderCapability {
        ProviderCapability {
            provider_id: CPU_BASELINE_PROVIDER_ID.to_string(),
            contract_version: CONTRACT_VERSION.to_string(),
            health_state: HealthState::Healthy.as_wire().to_string(),
            latency_class: "baseline".to_string(),
            cost_class: "low".to_string(),
            queue_depth: 0,
            supported_workload_tags: vec!["default".to_string()],
            reliability_class: "standard".to_string(),
            feature_flags: vec![],
        }
    }

    fn health(&self) -> Health {
        Health {
            state: HealthState::Healthy,
            queue_depth: 0,
            detail: None,
        }
    }

    fn infer(&self, req: &InferenceRequest) -> Result<Completion, BackendFailure> {
        let started = Instant::now();

        // Deterministic, dependency-free "inference": derive a stable fingerprint from
        // the request so output is reproducible without any model weights.
        let mut hasher = Sha256::new();
        hasher.update(req.prompt.as_bytes());
        for tag in &req.workload_tags {
            hasher.update([0x1f]);
            hasher.update(tag.as_bytes());
        }
        let digest = hasher.finalize();
        let fingerprint = hex_lower(&digest[..8]);

        let request_label = if req.request_id.is_empty() {
            "anonymous"
        } else {
            req.request_id.as_str()
        };
        let output = format!(
            "cpu-baseline deterministic completion for request {request_label} [fingerprint={fingerprint}]"
        );

        let tokens_in = estimate_tokens(&req.prompt).saturating_add(req.context_tokens);
        let tokens_out = estimate_tokens(&output);

        Ok(Completion {
            provider_id: CPU_BASELINE_PROVIDER_ID.to_string(),
            output,
            tokens_in,
            tokens_out,
            latency_ms: started.elapsed().as_millis() as u64,
            deterministic: true,
        })
    }
}

/// estimate_tokens approximates token count with a ~4-chars-per-token heuristic,
/// returning at least 1 for any non-empty text.
fn estimate_tokens(text: &str) -> u32 {
    let approx = (text.len() as u32) / 4;
    if !text.is_empty() && approx == 0 {
        1
    } else {
        approx
    }
}

fn hex_lower(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push(char::from_digit((byte >> 4) as u32, 16).unwrap_or('0'));
        out.push(char::from_digit((byte & 0x0f) as u32, 16).unwrap_or('0'));
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn request(prompt: &str) -> InferenceRequest {
        InferenceRequest {
            request_id: "req-1".to_string(),
            workload_tags: vec!["default".to_string()],
            prompt: prompt.to_string(),
            context_tokens: 10,
            max_output_tokens: 0,
            latency_class: "batch".to_string(),
        }
    }

    #[test]
    fn capabilities_are_baseline() {
        let cap = CpuBaselineBackend::new().capabilities();
        assert_eq!(cap.provider_id, CPU_BASELINE_PROVIDER_ID);
        assert_eq!(cap.contract_version, CONTRACT_VERSION);
        assert_eq!(cap.latency_class, "baseline");
        assert_eq!(cap.supported_workload_tags, vec!["default".to_string()]);
    }

    #[test]
    fn baseline_is_always_healthy() {
        assert_eq!(
            CpuBaselineBackend::new().health().state,
            HealthState::Healthy
        );
    }

    #[test]
    fn inference_is_deterministic() {
        let backend = CpuBaselineBackend::new();
        let a = backend.infer(&request("hello world")).expect("infer a");
        let b = backend.infer(&request("hello world")).expect("infer b");
        assert_eq!(a.output, b.output);
        assert_eq!(a.tokens_in, b.tokens_in);
        assert_eq!(a.tokens_out, b.tokens_out);
        assert!(a.deterministic);
        assert_eq!(a.provider_id, CPU_BASELINE_PROVIDER_ID);
    }

    #[test]
    fn different_prompts_yield_different_output() {
        let backend = CpuBaselineBackend::new();
        let a = backend.infer(&request("alpha")).expect("infer a");
        let b = backend.infer(&request("beta")).expect("infer b");
        assert_ne!(a.output, b.output);
    }
}
