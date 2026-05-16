//! cwso-git-shadow — Phase 2 PoC sidecar.
//!
//! Owns:
//!   * In-memory bare Git repo on tmpfs
//!   * Per-shadow-workspace virtual filesystem backed by libgit2 blobs
//!   * Tree-sitter AST queries (Phase 2: Go, Python; Rust+TS via T029)
//!
//! Wire protocol: framed JSON over a Unix domain socket.
//! Frame = 4-byte big-endian length + JSON body.
//!
//! POC-DEBT (P2-1): OverlayFS bind-mount layer is deferred to Phase 4 with
//! sandbox runners. Today, sub-agents access the virtual FS via orchestrator
//! → sidecar IPC instead of an OS mount.

use std::io::{Read, Write};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;

use anyhow::{Context, Result};

mod ast;
mod proto;
mod repo;

use proto::{Envelope, Request, Response};
use repo::ShadowStore;

const SOCKET_PATH_DEFAULT: &str = "/run/cwso/git-shadow.sock";
const FRAME_HEADER: usize = 4;
const FRAME_MAX: usize = 8 * 1024 * 1024; // 8 MiB

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .json()
        .init();

    let socket_path: PathBuf = std::env::var("CWSO_GIT_SHADOW_SOCKET")
        .unwrap_or_else(|_| SOCKET_PATH_DEFAULT.to_string())
        .into();
    let storage_root: PathBuf = std::env::var("CWSO_GIT_SHADOW_STORAGE")
        .unwrap_or_else(|_| "/var/lib/cwso/shadow".to_string())
        .into();

    if let Some(parent) = socket_path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    std::fs::create_dir_all(&storage_root)
        .with_context(|| format!("create storage root {storage_root:?}"))?;

    // Best-effort cleanup of stale socket.
    let _ = std::fs::remove_file(&socket_path);
    let listener =
        UnixListener::bind(&socket_path).with_context(|| format!("bind {socket_path:?}"))?;
    // POC-DEBT P2-5: socket perms are 0o666 because the orchestrator and the
    // sidecar run under different UIDs in their respective containers and the
    // socket is exposed only on a private compose-managed bind volume. T029
    // tightens this by aligning UIDs and using 0o660 with a shared GID.
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o666))?;

    tracing::info!(?socket_path, ?storage_root, "cwso-git-shadow ready");

    let store = Arc::new(ShadowStore::new(storage_root)?);

    for stream in listener.incoming() {
        match stream {
            Ok(s) => {
                let store = Arc::clone(&store);
                thread::spawn(move || {
                    if let Err(e) = handle_client(s, store) {
                        tracing::warn!(error = %e, "client error");
                    }
                });
            }
            Err(e) => tracing::warn!(error = %e, "accept error"),
        }
    }
    Ok(())
}

fn handle_client(mut stream: UnixStream, store: Arc<ShadowStore>) -> Result<()> {
    loop {
        let frame = match read_frame(&mut stream)? {
            Some(f) => f,
            None => return Ok(()),
        };
        let env: Envelope<Request> = match serde_json::from_slice(&frame) {
            Ok(v) => v,
            Err(e) => {
                let resp = Envelope::<Response> {
                    id: String::new(),
                    payload: Response::error("parse_error", &e.to_string()),
                };
                write_frame(&mut stream, &serde_json::to_vec(&resp)?)?;
                continue;
            }
        };
        let id = env.id.clone();
        let resp = repo::dispatch(&store, env.payload);
        let out = Envelope::<Response> { id, payload: resp };
        write_frame(&mut stream, &serde_json::to_vec(&out)?)?;
    }
}

fn read_frame(s: &mut UnixStream) -> Result<Option<Vec<u8>>> {
    let mut hdr = [0u8; FRAME_HEADER];
    match s.read_exact(&mut hdr) {
        Ok(()) => {}
        Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(e) => return Err(e.into()),
    }
    let len = u32::from_be_bytes(hdr) as usize;
    if len == 0 || len > FRAME_MAX {
        anyhow::bail!("frame size out of range: {len}");
    }
    let mut body = vec![0u8; len];
    s.read_exact(&mut body)?;
    Ok(Some(body))
}

fn write_frame(s: &mut UnixStream, body: &[u8]) -> Result<()> {
    if body.len() > FRAME_MAX {
        anyhow::bail!("response too large: {}", body.len());
    }
    let hdr = (body.len() as u32).to_be_bytes();
    s.write_all(&hdr)?;
    s.write_all(body)?;
    s.flush()?;
    Ok(())
}
