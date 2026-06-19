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

use std::collections::HashSet;
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
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
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;

    let authz_policy = Arc::new(IpcAuthzPolicy::from_env()?);

    tracing::info!(?socket_path, ?storage_root, "cwso-git-shadow ready");

    let store = Arc::new(ShadowStore::new(storage_root)?);

    for stream in listener.incoming() {
        match stream {
            Ok(s) => {
                let store = Arc::clone(&store);
                let authz_policy = Arc::clone(&authz_policy);
                thread::spawn(move || {
                    if let Err(e) = handle_client(s, store, &authz_policy) {
                        tracing::warn!(error = %e, "client error");
                    }
                });
            }
            Err(e) => tracing::warn!(error = %e, "accept error"),
        }
    }
    Ok(())
}

fn handle_client(
    mut stream: UnixStream,
    store: Arc<ShadowStore>,
    authz_policy: &IpcAuthzPolicy,
) -> Result<()> {
    if !authorize_stream(&stream, authz_policy)? {
        tracing::warn!("rejected unauthorized git-shadow IPC peer");
        return Ok(());
    }

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

#[derive(Debug, Clone, Copy)]
struct PeerCred {
    uid: u32,
    gid: u32,
}

#[derive(Debug, Clone)]
struct IpcAuthzPolicy {
    allowed_uids: HashSet<u32>,
    allowed_gids: HashSet<u32>,
}

impl IpcAuthzPolicy {
    fn from_env() -> Result<Self> {
        let default_uid = unsafe { libc::geteuid() };
        let default_gid = unsafe { libc::getegid() };

        let uid_csv =
            std::env::var("CWSO_IPC_ALLOWED_UIDS").unwrap_or_else(|_| default_uid.to_string());
        let gid_csv =
            std::env::var("CWSO_IPC_ALLOWED_GIDS").unwrap_or_else(|_| default_gid.to_string());

        Ok(Self {
            allowed_uids: parse_id_csv("CWSO_IPC_ALLOWED_UIDS", &uid_csv)?,
            allowed_gids: parse_id_csv("CWSO_IPC_ALLOWED_GIDS", &gid_csv)?,
        })
    }

    fn allows(&self, cred: &PeerCred) -> bool {
        self.allowed_uids.contains(&cred.uid) || self.allowed_gids.contains(&cred.gid)
    }

    #[cfg(test)]
    fn from_allowed(allowed_uids: &[u32], allowed_gids: &[u32]) -> Self {
        Self {
            allowed_uids: allowed_uids.iter().copied().collect(),
            allowed_gids: allowed_gids.iter().copied().collect(),
        }
    }
}

fn parse_id_csv(var_name: &str, value: &str) -> Result<HashSet<u32>> {
    let mut ids = HashSet::new();
    for raw in value.split(',') {
        let trimmed = raw.trim();
        if trimmed.is_empty() {
            continue;
        }
        let parsed: u32 = trimmed
            .parse()
            .with_context(|| format!("invalid {var_name} entry: {trimmed}"))?;
        ids.insert(parsed);
    }

    if ids.is_empty() {
        anyhow::bail!("{var_name} must contain at least one UID/GID");
    }

    Ok(ids)
}

fn authorize_stream(stream: &UnixStream, authz_policy: &IpcAuthzPolicy) -> Result<bool> {
    #[cfg(target_os = "linux")]
    {
        let cred = peer_cred_linux(stream)?;
        return Ok(authz_policy.allows(&cred));
    }

    #[cfg(not(target_os = "linux"))]
    {
        let _ = stream;
        let _ = authz_policy;
        Ok(true)
    }
}

#[cfg(target_os = "linux")]
fn peer_cred_linux(stream: &UnixStream) -> Result<PeerCred> {
    let mut ucred = libc::ucred {
        pid: 0,
        uid: 0,
        gid: 0,
    };
    let mut len = std::mem::size_of::<libc::ucred>() as libc::socklen_t;
    let rc = unsafe {
        libc::getsockopt(
            stream.as_raw_fd(),
            libc::SOL_SOCKET,
            libc::SO_PEERCRED,
            &mut ucred as *mut libc::ucred as *mut libc::c_void,
            &mut len,
        )
    };

    if rc != 0 {
        return Err(std::io::Error::last_os_error()).context("getsockopt SO_PEERCRED failed");
    }

    Ok(PeerCred {
        uid: ucred.uid,
        gid: ucred.gid,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(target_os = "linux")]
    #[test]
    fn authorize_stream_rejects_unauthorized_peer() {
        let (client, server) = UnixStream::pair().expect("create unix pair");
        let _client = client;

        let policy = IpcAuthzPolicy::from_allowed(&[u32::MAX], &[u32::MAX]);
        let authorized = authorize_stream(&server, &policy).expect("authorize stream");

        assert!(
            !authorized,
            "peer must be rejected when UID/GID is not allowlisted"
        );
    }
}
