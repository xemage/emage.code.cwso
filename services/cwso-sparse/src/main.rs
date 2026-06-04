//! cwso-sparse — the data-side host for the ephemeral sparse Wasm micro-agent tier (ADR-008).
//!
//! Phase 7 / Feature B. This binary owns the deterministic native ternary GEMM kernel and
//! exposes it over a framed-JSON Unix-domain-socket protocol (same wire format and peer-auth
//! envelope as cwso-hal). The wasmtime module-instantiation layer that drives this kernel from a
//! sandboxed orchestration module lands with the agent lifecycle (T122); this crate establishes
//! the sidecar, the protocol, and the bounds-checked `ternary_gemm` host-call.

use anyhow::Result;
use std::path::PathBuf;

mod agent;
mod gemm;
mod ipc;
mod proto;
mod slice;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/sparse.sock";

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .json()
        .init();

    let socket_path: PathBuf = std::env::var("CWSO_SPARSE_SOCKET")
        .unwrap_or_else(|_| SOCKET_PATH_DEFAULT.to_string())
        .into();

    let agents = match agent::AgentConfig::from_env()? {
        Some(cfg) => Some(std::sync::Arc::new(agent::AgentRegistry::new(cfg)?)),
        None => None,
    };

    ipc::run(socket_path, agents)
}
