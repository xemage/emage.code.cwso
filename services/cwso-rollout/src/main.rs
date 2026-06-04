//! cwso-rollout — Phase 9 LLM reverse proxy sidecar (ADR-010, T132).
//!
//! Hosts a `hyper` reverse proxy for OpenAI/Anthropic/Google model routes with a four-step
//! capture pipeline and framed-JSON UDS control plane matching cwso-hal / cwso-sparse.

use std::path::PathBuf;
use std::sync::Arc;

use anyhow::Result;
use tokio::runtime::Runtime;

mod capture;
mod config;
mod ipc;
mod proto;
mod provider;
mod proxy;
mod record;
mod security;
mod upstream;

use capture::CapturePipeline;
use config::SidecarConfig;
use record::CaptureStore;

const CAPTURE_QUEUE_DEFAULT: usize = 4096;

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .json()
        .init();

    let sidecar = SidecarConfig::from_env()?;
    let queue_capacity = sidecar
        .proxy
        .as_ref()
        .map(|proxy| proxy.capture_queue_capacity)
        .unwrap_or(CAPTURE_QUEUE_DEFAULT);
    let store = Arc::new(CaptureStore::new(queue_capacity));

    let socket_path: PathBuf = sidecar.socket_path.into();
    let store_ipc = Arc::clone(&store);
    std::thread::spawn(move || {
        if let Err(error) = ipc::run(socket_path, store_ipc) {
            tracing::error!(error = %error, "cwso-rollout IPC server exited");
        }
    });

    if let Some(proxy_config) = sidecar.proxy {
        let runtime = Runtime::new()?;
        let pipeline = Arc::new(CapturePipeline::new(
            proxy_config.clone(),
            Arc::clone(&store),
        ));
        let bind = proxy_config.http_bind.clone();
        runtime.block_on(async move {
            tracing::info!(%bind, upstream = %proxy_config.upstream_url, "starting rollout proxy");
            proxy::serve(&bind, pipeline).await
        })?;
    } else {
        tracing::info!("CWSO_ROLLOUT_PROXY_ENABLED=false; IPC-only mode");
        std::thread::park();
    }

    Ok(())
}
