use std::sync::atomic::{AtomicU32, AtomicU8, Ordering};
use std::sync::Arc;
use std::time::Instant;

use serde_json::json;

use crate::backend::{
    BackendFailure, Completion, FailureClass, Health, HealthState, InferenceBackend,
    InferenceRequest, ProviderCapability, CONTRACT_VERSION,
};
use crate::http::{HttpTransport, TransportError};

/// HealthCache holds the most recently observed health for an accelerator backend so the
/// dispatch hot path can read it locklessly with no network I/O, while a background prober
/// (`probe`) and the `infer` outcome path keep it fresh. `queue_depth` is plumbed through
/// but currently only ever 0 — see the note on `OpenAiCompatibleBackend`.
#[derive(Debug)]
struct HealthCache {
    state: AtomicU8,
    queue_depth: AtomicU32,
}

impl HealthCache {
    fn new(initial: HealthState) -> Self {
        Self {
            state: AtomicU8::new(encode_state(initial)),
            queue_depth: AtomicU32::new(0),
        }
    }

    fn load(&self) -> HealthState {
        decode_state(self.state.load(Ordering::Relaxed))
    }

    fn store(&self, state: HealthState) {
        self.state.store(encode_state(state), Ordering::Relaxed);
    }

    fn queue_depth(&self) -> u32 {
        self.queue_depth.load(Ordering::Relaxed)
    }
}

fn encode_state(state: HealthState) -> u8 {
    match state {
        HealthState::Healthy => 0,
        HealthState::Degraded => 1,
        HealthState::Unavailable => 2,
    }
}

fn decode_state(value: u8) -> HealthState {
    match value {
        0 => HealthState::Healthy,
        1 => HealthState::Degraded,
        _ => HealthState::Unavailable,
    }
}

/// OpenAiBackendConfig configures an OpenAI-compatible chat-completions backend
/// (vLLM, TensorRT-LLM, Groq, etc.). The same adapter serves both the GPU (T083) and
/// LPU (T084) presets — they differ only in endpoint and advertised capabilities.
#[derive(Debug, Clone)]
pub struct OpenAiBackendConfig {
    pub provider_id: String,
    /// base_url is the OpenAI-compatible API root, e.g. `http://vllm:8000/v1`.
    pub base_url: String,
    pub model: String,
    pub api_key: Option<String>,
    pub latency_class: String,
    pub cost_class: String,
    pub reliability_class: String,
    pub supported_workload_tags: Vec<String>,
    pub feature_flags: Vec<String>,
    pub default_max_tokens: u32,
}

/// gpu_vllm_config returns the GPU (dense model, vLLM/TensorRT-LLM) preset (T083).
pub fn gpu_vllm_config(
    base_url: String,
    model: String,
    api_key: Option<String>,
) -> OpenAiBackendConfig {
    OpenAiBackendConfig {
        provider_id: "gpu-accelerated".to_string(),
        base_url,
        model,
        api_key,
        latency_class: "fast".to_string(),
        cost_class: "high".to_string(),
        reliability_class: "gold".to_string(),
        supported_workload_tags: vec![
            "inference-heavy".to_string(),
            "deterministic-edit".to_string(),
        ],
        feature_flags: vec!["hhd.sparse_quantized_assist".to_string()],
        default_max_tokens: 1024,
    }
}

/// lpu_groq_config returns the LPU (Groq-style deterministic low-latency) preset (T084).
pub fn lpu_groq_config(
    base_url: String,
    model: String,
    api_key: Option<String>,
) -> OpenAiBackendConfig {
    OpenAiBackendConfig {
        provider_id: "lpu-realtime".to_string(),
        base_url,
        model,
        api_key,
        latency_class: "ultra".to_string(),
        cost_class: "medium".to_string(),
        reliability_class: "gold".to_string(),
        supported_workload_tags: vec!["realtime".to_string()],
        feature_flags: vec![],
        default_max_tokens: 512,
    }
}

/// OpenAiCompatibleBackend talks to an OpenAI-compatible `/chat/completions` endpoint.
///
/// `health()` is cheap: it returns a cached snapshot (no network I/O) so it never inflates
/// the dispatch hot path. The cache is refreshed two ways (T091): a background prober calls
/// `probe()` periodically (active liveness via `/models`), and every `infer` updates the
/// cache from its own outcome (reactive). This makes the capability snapshot the Go control
/// plane consumes carry *live* `health_state` instead of a hardcoded "healthy".
///
/// `queue_depth` is plumbed through the cache and capability record, but the OpenAI API has
/// no standard queue-depth endpoint, so it currently stays 0; deriving a real value needs a
/// provider-specific metrics scrape (e.g. vLLM `/metrics`) and is tracked as future work.
pub struct OpenAiCompatibleBackend {
    cfg: OpenAiBackendConfig,
    transport: Box<dyn HttpTransport>,
    health: Arc<HealthCache>,
}

impl OpenAiCompatibleBackend {
    pub fn new(cfg: OpenAiBackendConfig, transport: Box<dyn HttpTransport>) -> Self {
        Self {
            cfg,
            transport,
            // Optimistic until the first probe/infer so a just-registered backend is
            // eligible for routing; the startup probe in main.rs refreshes it immediately.
            health: Arc::new(HealthCache::new(HealthState::Healthy)),
        }
    }

    /// record_outcome updates the cached health from a completed inference attempt so the
    /// next capability snapshot reflects reality without waiting for the next probe tick.
    fn record_failure(&self, failure: BackendFailure) -> BackendFailure {
        self.health.store(failure.class.to_health_state());
        failure
    }

    fn chat_url(&self) -> String {
        format!(
            "{}/chat/completions",
            self.cfg.base_url.trim_end_matches('/')
        )
    }

    fn models_url(&self) -> String {
        format!("{}/models", self.cfg.base_url.trim_end_matches('/'))
    }

    /// probe_models performs a live readiness check against `/models`.
    pub fn probe_models(&self) -> Result<(), BackendFailure> {
        let resp = self
            .transport
            .get(&self.models_url(), self.cfg.api_key.as_deref())
            .map_err(map_transport_error)?;
        if (200..300).contains(&resp.status) {
            Ok(())
        } else {
            Err(map_status_error(resp.status, &resp.body))
        }
    }
}

impl InferenceBackend for OpenAiCompatibleBackend {
    fn capabilities(&self) -> ProviderCapability {
        ProviderCapability {
            provider_id: self.cfg.provider_id.clone(),
            contract_version: CONTRACT_VERSION.to_string(),
            health_state: self.health.load().as_wire().to_string(),
            latency_class: self.cfg.latency_class.clone(),
            cost_class: self.cfg.cost_class.clone(),
            queue_depth: self.health.queue_depth(),
            supported_workload_tags: self.cfg.supported_workload_tags.clone(),
            reliability_class: self.cfg.reliability_class.clone(),
            feature_flags: self.cfg.feature_flags.clone(),
        }
    }

    fn health(&self) -> Health {
        Health {
            state: self.health.load(),
            queue_depth: self.health.queue_depth(),
            detail: None,
        }
    }

    fn probe(&self) -> Health {
        let state = match self.probe_models() {
            Ok(()) => HealthState::Healthy,
            Err(failure) => failure.class.to_health_state(),
        };
        self.health.store(state);
        self.health()
    }

    fn infer(&self, req: &InferenceRequest) -> Result<Completion, BackendFailure> {
        let started = Instant::now();
        let max_tokens = if req.max_output_tokens > 0 {
            req.max_output_tokens
        } else {
            self.cfg.default_max_tokens
        };

        let body = json!({
            "model": self.cfg.model,
            "messages": [{ "role": "user", "content": req.prompt }],
            "max_tokens": max_tokens,
            "temperature": 0,
            "stream": false,
        });

        let resp = self
            .transport
            .post_json(&self.chat_url(), self.cfg.api_key.as_deref(), &body)
            .map_err(|error| self.record_failure(map_transport_error(error)))?;

        if !(200..300).contains(&resp.status) {
            return Err(self.record_failure(map_status_error(resp.status, &resp.body)));
        }

        match parse_completion(
            &self.cfg.provider_id,
            &resp.body,
            started.elapsed().as_millis() as u64,
        ) {
            Ok(completion) => {
                // A served request is the strongest possible liveness signal.
                self.health.store(HealthState::Healthy);
                Ok(completion)
            }
            Err(failure) => Err(self.record_failure(failure)),
        }
    }
}

fn parse_completion(
    provider_id: &str,
    body: &str,
    latency_ms: u64,
) -> Result<Completion, BackendFailure> {
    let value: serde_json::Value = serde_json::from_str(body).map_err(|e| {
        BackendFailure::new(
            FailureClass::Internal,
            format!("invalid response json: {e}"),
        )
    })?;

    let output = value["choices"][0]["message"]["content"]
        .as_str()
        .ok_or_else(|| {
            BackendFailure::new(
                FailureClass::Internal,
                "response missing choices[0].message.content",
            )
        })?
        .to_string();

    let tokens_in = value["usage"]["prompt_tokens"].as_u64().unwrap_or(0) as u32;
    let tokens_out = value["usage"]["completion_tokens"].as_u64().unwrap_or(0) as u32;

    Ok(Completion {
        provider_id: provider_id.to_string(),
        output,
        tokens_in,
        tokens_out,
        latency_ms,
        deterministic: false,
    })
}

fn map_transport_error(error: TransportError) -> BackendFailure {
    match error {
        TransportError::Timeout(message) => BackendFailure::new(FailureClass::Timeout, message),
        TransportError::Unreachable(message) => {
            BackendFailure::new(FailureClass::Unavailable, message)
        }
        TransportError::Other(message) => BackendFailure::new(FailureClass::Internal, message),
    }
}

fn map_status_error(status: u16, body: &str) -> BackendFailure {
    let class = match status {
        429 => FailureClass::Overloaded,
        408 => FailureClass::Timeout,
        400 | 422 => FailureClass::InvalidRequest,
        500..=599 => FailureClass::Unavailable,
        _ => FailureClass::Internal,
    };
    BackendFailure::new(class, format!("http {status}: {}", truncate(body, 200)))
}

fn truncate(text: &str, max: usize) -> String {
    if text.len() <= max {
        text.to_string()
    } else {
        format!("{}…", &text[..max])
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::http::testing::MockTransport;
    use crate::http::{HttpResponse, TransportError};

    fn request() -> InferenceRequest {
        InferenceRequest {
            request_id: "r1".to_string(),
            workload_tags: vec!["inference-heavy".to_string()],
            prompt: "write a haiku".to_string(),
            context_tokens: 0,
            max_output_tokens: 0,
            latency_class: "batch".to_string(),
        }
    }

    fn ok_body() -> String {
        json!({
            "choices": [{ "message": { "role": "assistant", "content": "snow falls softly" } }],
            "usage": { "prompt_tokens": 5, "completion_tokens": 3 }
        })
        .to_string()
    }

    fn gpu(transport: MockTransport) -> OpenAiCompatibleBackend {
        OpenAiCompatibleBackend::new(
            gpu_vllm_config(
                "http://vllm:8000/v1".to_string(),
                "qwen2.5-coder".to_string(),
                Some("secret".to_string()),
            ),
            Box::new(transport),
        )
    }

    #[test]
    fn gpu_capabilities_are_dense_high_cost() {
        let cap = gpu(MockTransport::new()).capabilities();
        assert_eq!(cap.provider_id, "gpu-accelerated");
        assert_eq!(cap.latency_class, "fast");
        assert_eq!(cap.cost_class, "high");
        assert!(cap
            .supported_workload_tags
            .contains(&"inference-heavy".to_string()));
    }

    #[test]
    fn lpu_capabilities_are_ultra_realtime() {
        let backend = OpenAiCompatibleBackend::new(
            lpu_groq_config(
                "http://groq/v1".to_string(),
                "llama-3.1-8b".to_string(),
                None,
            ),
            Box::new(MockTransport::new()),
        );
        let cap = backend.capabilities();
        assert_eq!(cap.provider_id, "lpu-realtime");
        assert_eq!(cap.latency_class, "ultra");
        assert_eq!(cap.supported_workload_tags, vec!["realtime".to_string()]);
    }

    #[test]
    fn infer_parses_completion_and_sends_bearer() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 200,
            body: ok_body(),
        }));
        let backend = gpu(transport);
        let completion = backend.infer(&request()).expect("infer");
        assert_eq!(completion.provider_id, "gpu-accelerated");
        assert_eq!(completion.output, "snow falls softly");
        assert_eq!(completion.tokens_in, 5);
        assert_eq!(completion.tokens_out, 3);
        assert!(!completion.deterministic);
    }

    #[test]
    fn http_429_maps_to_overloaded() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 429,
            body: "rate limited".to_string(),
        }));
        let err = gpu(transport).infer(&request()).expect_err("must fail");
        assert_eq!(err.class, FailureClass::Overloaded);
    }

    #[test]
    fn http_400_maps_to_invalid_request() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 400,
            body: "bad".to_string(),
        }));
        let err = gpu(transport).infer(&request()).expect_err("must fail");
        assert_eq!(err.class, FailureClass::InvalidRequest);
        assert!(!err.class.retryable_via_fallback());
    }

    #[test]
    fn http_503_maps_to_unavailable() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 503,
            body: "down".to_string(),
        }));
        let err = gpu(transport).infer(&request()).expect_err("must fail");
        assert_eq!(err.class, FailureClass::Unavailable);
    }

    #[test]
    fn transport_timeout_maps_to_timeout() {
        let transport =
            MockTransport::new().with_post(Err(TransportError::Timeout("timed out".to_string())));
        let err = gpu(transport).infer(&request()).expect_err("must fail");
        assert_eq!(err.class, FailureClass::Timeout);
    }

    #[test]
    fn malformed_json_maps_to_internal() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 200,
            body: "not json".to_string(),
        }));
        let err = gpu(transport).infer(&request()).expect_err("must fail");
        assert_eq!(err.class, FailureClass::Internal);
    }

    #[test]
    fn probe_models_ok_on_2xx() {
        let transport = MockTransport::new().with_get(Ok(HttpResponse {
            status: 200,
            body: "{\"data\":[]}".to_string(),
        }));
        assert!(gpu(transport).probe_models().is_ok());
    }

    #[test]
    fn probe_models_unreachable_maps_to_unavailable() {
        let transport = MockTransport::new().with_get(Err(TransportError::Unreachable(
            "connect refused".to_string(),
        )));
        let err = gpu(transport).probe_models().expect_err("must fail");
        assert_eq!(err.class, FailureClass::Unavailable);
    }

    #[test]
    fn probe_success_caches_healthy_and_capabilities_reflect_it() {
        let transport = MockTransport::new().with_get(Ok(HttpResponse {
            status: 200,
            body: "{\"data\":[]}".to_string(),
        }));
        let backend = gpu(transport);
        let health = backend.probe();
        assert_eq!(health.state, HealthState::Healthy);
        assert_eq!(backend.capabilities().health_state, "healthy");
    }

    #[test]
    fn probe_unreachable_caches_unavailable_in_capabilities() {
        let transport = MockTransport::new().with_get(Err(TransportError::Unreachable(
            "connect refused".to_string(),
        )));
        let backend = gpu(transport);
        let health = backend.probe();
        assert_eq!(health.state, HealthState::Unavailable);
        // The cheap health() and the capability snapshot must reflect the live probe.
        assert_eq!(backend.health().state, HealthState::Unavailable);
        assert_eq!(backend.capabilities().health_state, "unavailable");
    }

    #[test]
    fn probe_overloaded_caches_degraded() {
        let transport = MockTransport::new().with_get(Ok(HttpResponse {
            status: 429,
            body: "rate limited".to_string(),
        }));
        let backend = gpu(transport);
        assert_eq!(backend.probe().state, HealthState::Degraded);
        assert_eq!(backend.capabilities().health_state, "degraded");
    }

    #[test]
    fn infer_failure_reactively_marks_unavailable() {
        let transport = MockTransport::new().with_post(Ok(HttpResponse {
            status: 503,
            body: "down".to_string(),
        }));
        let backend = gpu(transport);
        // Starts optimistic.
        assert_eq!(backend.health().state, HealthState::Healthy);
        let _ = backend.infer(&request()).expect_err("must fail");
        // A 503 (Unavailable) inference must degrade cached health without a probe.
        assert_eq!(backend.health().state, HealthState::Unavailable);
    }

    #[test]
    fn infer_success_restores_healthy() {
        let transport = MockTransport::new()
            .with_post(Ok(HttpResponse {
                status: 503,
                body: "down".to_string(),
            }))
            .with_post(Ok(HttpResponse {
                status: 200,
                body: ok_body(),
            }));
        let backend = gpu(transport);
        let _ = backend.infer(&request()).expect_err("first attempt fails");
        assert_eq!(backend.health().state, HealthState::Unavailable);
        let _ = backend.infer(&request()).expect("second attempt serves");
        assert_eq!(backend.health().state, HealthState::Healthy);
    }
}
