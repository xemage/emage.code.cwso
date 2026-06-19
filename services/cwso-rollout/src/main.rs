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
mod prefix_cache;
mod proto;
mod provider;
mod proxy;
mod record;
mod security;
mod store;
mod upstream;

use capture::CapturePipeline;
use config::SidecarConfig;
use prefix_cache::PrefixCache;
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
    let mut capture_store = CaptureStore::new(queue_capacity);
    let store_writer = store::StoreConfig::from_env()?;
    let mut _store_join = None;
    if let Some(store_config) = store_writer {
        let (store_handle, join) = store::spawn_store(store_config)?;
        capture_store =
            capture_store.with_store_fanout(store_handle.sender, Arc::clone(&store_handle.dropped));
        _store_join = Some(join);
        tracing::info!(
            written = store_handle
                .written
                .load(std::sync::atomic::Ordering::Relaxed),
            "trajectory Parquet store enabled"
        );
    }
    let store = Arc::new(capture_store);
    let prefix_cache = Arc::new(PrefixCache::from_env());

    let socket_path: PathBuf = sidecar.socket_path.into();
    let store_ipc = Arc::clone(&store);
    let prefix_ipc = Arc::clone(&prefix_cache);
    std::thread::spawn(move || {
        if let Err(error) = ipc::run(socket_path, store_ipc, prefix_ipc) {
            tracing::error!(error = %error, "cwso-rollout IPC server exited");
        }
    });

    if let Some(proxy_config) = sidecar.proxy {
        let runtime = Runtime::new()?;
        let pipeline = Arc::new(CapturePipeline::new(
            proxy_config.clone(),
            Arc::clone(&store),
            Arc::clone(&prefix_cache),
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
