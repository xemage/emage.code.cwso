//! Framed-JSON UDS control plane for cwso-rollout (ADR-010, T132).

use std::collections::HashSet;
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;

use anyhow::{Context, Result};
use serde_json::json;

use crate::prefix_cache::PrefixCache;
use crate::proto::{Envelope, Request, Response, CONTRACT_VERSION, SERVICE};
use crate::record::SharedCaptureStore;

const FRAME_HEADER: usize = 4;
const FRAME_MAX: usize = 8 * 1024 * 1024;

pub fn run(
    socket_path: PathBuf,
    store: SharedCaptureStore,
    prefix_cache: Arc<PrefixCache>,
) -> Result<()> {
    if let Some(parent) = socket_path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    let _ = std::fs::remove_file(&socket_path);
    let listener = UnixListener::bind(&socket_path)
        .with_context(|| format!("bind cwso-rollout socket {socket_path:?}"))?;

    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;

    let authz_policy = Arc::new(IpcAuthzPolicy::from_env()?);
    tracing::info!(?socket_path, "cwso-rollout IPC ready");

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                let authz_policy = Arc::clone(&authz_policy);
                let store = Arc::clone(&store);
                let prefix_cache = Arc::clone(&prefix_cache);
                thread::spawn(move || {
                    if let Err(error) = handle_client(stream, &authz_policy, &store, &prefix_cache)
                    {
                        tracing::warn!(error = %error, "cwso-rollout IPC client error");
                    }
                });
            }
            Err(error) => tracing::warn!(error = %error, "cwso-rollout accept error"),
        }
    }
    Ok(())
}

fn handle_client(
    mut stream: UnixStream,
    authz_policy: &IpcAuthzPolicy,
    store: &SharedCaptureStore,
    prefix_cache: &PrefixCache,
) -> Result<()> {
    if !authorize_stream(&stream, authz_policy)? {
        tracing::warn!("rejected unauthorized cwso-rollout IPC peer");
        return Ok(());
    }

    loop {
        let frame = match read_frame(&mut stream)? {
            Some(frame) => frame,
            None => return Ok(()),
        };

        let request: Envelope<Request> = match serde_json::from_slice(&frame) {
            Ok(request) => request,
            Err(error) => {
                let parse_error = Envelope::<Response> {
                    id: String::new(),
                    payload: Response::error("parse_error", &error.to_string()),
                };
                write_frame(&mut stream, &serde_json::to_vec(&parse_error)?)?;
                continue;
            }
        };

        let id = request.id.clone();
        let response = dispatch(store, prefix_cache, request.payload);
        let envelope = Envelope::<Response> {
            id,
            payload: response,
        };
        write_frame(&mut stream, &serde_json::to_vec(&envelope)?)?;
    }
}

pub fn dispatch(
    store: &SharedCaptureStore,
    prefix_cache: &PrefixCache,
    request: Request,
) -> Response {
    match request {
        Request::Stat => Response::ok(json!({
            "service": SERVICE,
            "contract_version": CONTRACT_VERSION,
        })),
        Request::CaptureStats => Response::ok(json!({
            "pending": store.pending_count(),
            "dropped": store.dropped_count(),
        })),
        Request::DrainCapture { limit } => {
            let limit = limit.max(1) as usize;
            let mut records = Vec::new();
            for _ in 0..limit {
                match store.try_drain_one() {
                    Some(record) => records.push(record),
                    None => break,
                }
            }
            Response::ok(json!({ "records": records }))
        }
        Request::PrefixPrewarm { prefix_key } => {
            let cache_hit = prefix_cache.prewarm(&prefix_key);
            Response::ok(json!({ "cache_hit": cache_hit }))
        }
        Request::PrefixStats => {
            let stats = prefix_cache.stats();
            Response::ok(json!({
                "entries": stats.entries,
                "hits": stats.hits,
                "misses": stats.misses,
                "hit_rate": stats.hit_rate,
            }))
        }
    }
}

fn read_frame(stream: &mut UnixStream) -> Result<Option<Vec<u8>>> {
    let mut header = [0u8; FRAME_HEADER];
    match stream.read_exact(&mut header) {
        Ok(()) => {}
        Err(error) if error.kind() == std::io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(error) => return Err(error.into()),
    }

    let body_len = u32::from_be_bytes(header) as usize;
    if body_len == 0 || body_len > FRAME_MAX {
        anyhow::bail!("frame size out of range: {body_len}");
    }

    let mut body = vec![0u8; body_len];
    stream.read_exact(&mut body)?;
    Ok(Some(body))
}

fn write_frame(stream: &mut UnixStream, body: &[u8]) -> Result<()> {
    if body.len() > FRAME_MAX {
        anyhow::bail!("response too large: {}", body.len());
    }
    let header = (body.len() as u32).to_be_bytes();
    stream.write_all(&header)?;
    stream.write_all(body)?;
    stream.flush()?;
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
    use crate::prefix_cache::PrefixCache;
    use crate::record::CaptureStore;

    #[test]
    fn stat_reports_service() {
        let store = Arc::new(CaptureStore::new(4));
        let prefix_cache = PrefixCache::new(8);
        let response = dispatch(&store, &prefix_cache, Request::Stat);
        match response {
            Response::Ok { ok, result } => {
                assert!(ok);
                assert_eq!(result["service"], SERVICE);
            }
            Response::Err { .. } => panic!("expected ok"),
        }
    }

    #[test]
    fn prefix_prewarm_reports_cache_hit() {
        let store = Arc::new(CaptureStore::new(4));
        let prefix_cache = PrefixCache::new(8);
        let miss = dispatch(
            &store,
            &prefix_cache,
            Request::PrefixPrewarm {
                prefix_key: "abc".to_string(),
            },
        );
        let hit = dispatch(
            &store,
            &prefix_cache,
            Request::PrefixPrewarm {
                prefix_key: "abc".to_string(),
            },
        );
        match (miss, hit) {
            (
                Response::Ok {
                    ok: true,
                    result: miss_result,
                },
                Response::Ok {
                    ok: true,
                    result: hit_result,
                },
            ) => {
                assert_eq!(miss_result["cache_hit"], false);
                assert_eq!(hit_result["cache_hit"], true);
            }
            _ => panic!("expected ok responses"),
        }
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn authorize_stream_rejects_unauthorized_peer() {
        let (client, server) = UnixStream::pair().expect("pair");
        let _client = client;
        let policy = IpcAuthzPolicy::from_allowed(&[u32::MAX], &[u32::MAX]);
        let authorized = authorize_stream(&server, &policy).expect("authorize");
        assert!(!authorized);
    }
}
