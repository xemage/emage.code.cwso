use anyhow::Result;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

mod backend;
mod cpu;
mod http;
mod ipc;
mod openai;
mod proto;
mod registry;
mod security;

use backend::InferenceBackend;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/hal.sock";
const HTTP_TIMEOUT_MS_DEFAULT: u64 = 30_000;
const HEALTH_PROBE_SECONDS_DEFAULT: u64 = 10;

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

    let registry = Arc::new(registry);

    // Active health probing (T091): a background prober refreshes each backend's cached
    // health on an interval so capability snapshots carry live health_state, while the
    // dispatch hot path keeps reading the cheap cached value.
    let probe_interval = Duration::from_secs(env_u64(
        "CWSO_HAL_HEALTH_PROBE_SECONDS",
        HEALTH_PROBE_SECONDS_DEFAULT,
    ));
    spawn_health_prober(Arc::clone(&registry), probe_interval);

    ipc::run(socket_path, registry)
}

/// spawn_health_prober runs an active health probe across all backends every `interval`.
/// Probing happens off the dispatch hot path; only the cached snapshot is read during
/// routing. The CPU baseline's probe is a cheap no-op (always healthy).
fn spawn_health_prober(registry: Arc<registry::BackendRegistry>, interval: Duration) {
    thread::spawn(move || loop {
        thread::sleep(interval);
        for (provider, health) in registry.probe_all() {
            tracing::debug!(
                provider = %provider,
                state = %health.state.as_wire(),
                queue_depth = health.queue_depth,
                "hal health probe"
            );
        }
    });
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

    // Transport-security gate (T093): refuse to ship a bearer token over plaintext http to
    // a non-loopback host unless explicitly overridden.
    let allow_insecure = env_bool("CWSO_HAL_ALLOW_INSECURE_ENDPOINTS", false);
    match security::validate_endpoint(&base_url, allow_insecure) {
        Ok(false) => {}
        Ok(true) => tracing::warn!(
            prefix = %prefix,
            base_url = %base_url,
            "registering accelerator over PLAINTEXT http to a non-loopback host \
             (CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true); the bearer API key is sent in cleartext"
        ),
        Err(error) => {
            tracing::error!(
                prefix = %prefix,
                base_url = %base_url,
                error = %error,
                "refusing to register accelerator endpoint with insecure transport; \
                 use https or set CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true"
            );
            return;
        }
    }

    let cfg = make_cfg(base_url, model, api_key);
    let provider_id = cfg.provider_id.clone();
    let backend =
        openai::OpenAiCompatibleBackend::new(cfg, Box::new(http::UreqTransport::new(timeout)));

    // Seed the cached health from a startup probe so the first capability snapshot reflects
    // reality; the background prober keeps it fresh thereafter. A failed probe is non-fatal
    // (the backend stays registered and is re-probed / reactively healed via infer).
    let health = backend.probe();
    match health.state {
        backend::HealthState::Healthy => {
            tracing::info!(provider = %provider_id, "hal accelerator registered (probe ok)")
        }
        state => tracing::warn!(
            provider = %provider_id,
            state = %state.as_wire(),
            "hal accelerator registered (startup probe unhealthy; relying on probe/infer recovery)"
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
