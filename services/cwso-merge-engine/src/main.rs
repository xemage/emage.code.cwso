use anyhow::Result;
use std::path::PathBuf;

mod ipc;
mod merge;
mod parse;
mod proto;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/merge-engine.sock";

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .json()
        .init();

    let socket_path: PathBuf = std::env::var("CWSO_MERGE_ENGINE_SOCKET")
        .unwrap_or_else(|_| SOCKET_PATH_DEFAULT.to_string())
        .into();

    ipc::run(socket_path)
}