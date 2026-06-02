use anyhow::Result;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;

mod backend;
mod cpu;
mod http;
mod ipc;
mod openai;
mod proto;
mod registry;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/hal.sock";
const HTTP_TIMEOUT_MS_DEFAULT: u64 = 30_000;

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .json()
        .init();

    let socket_path: PathBuf = std::env::var("CWSO_HAL_SOCKET")
        .unwrap_or_else(|_| SOCKET_PATH_DEFAULT.to_string())
        .into();

    let http_timeout =
        Duration::from_millis(env_u64("CWSO_HAL_HTTP_TIMEOUT_MS", HTTP_TIMEOUT_MS_DEFAULT));

    // The terminal-safe CPU baseline is always registered. Accelerator adapters are
    // registered only when their endpoint is configured, so the default deployment (and
    // CI/e2e, which have no model servers) runs baseline-only and safe.
    let mut registry = registry::BackendRegistry::with_cpu_baseline();
    register_openai_from_env(&mut registry, http_timeout, "GPU", openai::gpu_vllm_config);
    register_openai_from_env(&mut registry, http_timeout, "LPU", openai::lpu_groq_config);

    ipc::run(socket_path, Arc::new(registry))
}

/// register_openai_from_env registers an OpenAI-compatible accelerator backend when its
/// `CWSO_HAL_<PREFIX>_BASE_URL` and `_MODEL` env vars are set. A failed startup probe is
/// logged but non-fatal — liveness is enforced at inference time via fallback.
fn register_openai_from_env(
    registry: &mut registry::BackendRegistry,
    timeout: Duration,
    prefix: &str,
    make_cfg: fn(String, String, Option<String>) -> openai::OpenAiBackendConfig,
) {
    let base_url = std::env::var(format!("CWSO_HAL_{prefix}_BASE_URL")).ok();
    let model = std::env::var(format!("CWSO_HAL_{prefix}_MODEL")).ok();
    let (base_url, model) = match (base_url, model) {
        (Some(b), Some(m)) if !b.is_empty() && !m.is_empty() => (b, m),
        _ => return,
    };
    let api_key = std::env::var(format!("CWSO_HAL_{prefix}_API_KEY"))
        .ok()
        .filter(|value| !value.is_empty());

    let cfg = make_cfg(base_url, model, api_key);
    let provider_id = cfg.provider_id.clone();
    let backend =
        openai::OpenAiCompatibleBackend::new(cfg, Box::new(http::UreqTransport::new(timeout)));

    match backend.probe_models() {
        Ok(()) => tracing::info!(provider = %provider_id, "hal accelerator registered (probe ok)"),
        Err(error) => tracing::warn!(
            provider = %provider_id,
            error = %error,
            "hal accelerator registered (startup probe failed; relying on infer fallback)"
        ),
    }
    registry.register(Box::new(backend));
}

fn env_u64(key: &str, default: u64) -> u64 {
    std::env::var(key)
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}
