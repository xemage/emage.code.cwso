use anyhow::Result;
use std::path::PathBuf;
use std::sync::Arc;

mod backend;
mod cpu;
mod ipc;
mod proto;
mod registry;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/hal.sock";

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

    // Phase 6 (T082): only the terminal-safe CPU baseline adapter is registered. Real
    // accelerator adapters (GPU/LPU/SSM) land in T083/T084 and register alongside it.
    let registry = Arc::new(registry::BackendRegistry::with_cpu_baseline());

    ipc::run(socket_path, registry)
}
