use std::collections::HashSet;
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;

use anyhow::{Context, Result};
use base64::engine::general_purpose::STANDARD as B64;
use base64::Engine;
use serde_json::json;

use crate::merge::{merge_three_way, MergeError};
use crate::parse;
use crate::proto::{Envelope, Request, Response};

const FRAME_HEADER: usize = 4;
const FRAME_MAX: usize = 8 * 1024 * 1024;

pub fn run(socket_path: PathBuf) -> Result<()> {
    if let Some(parent) = socket_path.parent() {
        std::fs::create_dir_all(parent).ok();
    }

    let _ = std::fs::remove_file(&socket_path);
    let listener = UnixListener::bind(&socket_path)
        .with_context(|| format!("bind merge-engine socket {socket_path:?}"))?;

    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;

    let authz_policy = Arc::new(IpcAuthzPolicy::from_env()?);

    tracing::info!(?socket_path, "cwso-merge-engine ready");

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                let authz_policy = Arc::clone(&authz_policy);
                thread::spawn(move || {
                    if let Err(error) = handle_client(stream, &authz_policy) {
                        tracing::warn!(error = %error, "merge-engine client error");
                    }
                });
            }
            Err(error) => {
                tracing::warn!(error = %error, "merge-engine accept error");
            }
        }
    }
    Ok(())
}

fn handle_client(mut stream: UnixStream, authz_policy: &IpcAuthzPolicy) -> Result<()> {
    if !authorize_stream(&stream, authz_policy)? {
        tracing::warn!("rejected unauthorized merge-engine IPC peer");
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
        let response = dispatch(request.payload);
        let envelope = Envelope::<Response> {
            id,
            payload: response,
        };
        write_frame(&mut stream, &serde_json::to_vec(&envelope)?)?;
    }
}

fn dispatch(request: Request) -> Response {
    match request {
        Request::Stat => Response::ok(json!({
            "service": "cwso-merge-engine",
            "supported_languages": parse::supported_languages(),
        })),
        Request::MergeThreeWay {
            language,
            base_b64,
            ours_b64,
            theirs_b64,
        } => {
            let decoded = (|| -> anyhow::Result<(Vec<u8>, Vec<u8>, Vec<u8>)> {
                let base = B64.decode(base_b64.as_bytes())?;
                let ours = B64.decode(ours_b64.as_bytes())?;
                let theirs = B64.decode(theirs_b64.as_bytes())?;
                Ok((base, ours, theirs))
            })();

            let (base, ours, theirs) = match decoded {
                Ok(v) => v,
                Err(error) => {
                    return Response::error_with_meta(
                        "invalid_input",
                        Some("policy_conflict"),
                        Some("invalid_payload_encoding"),
                        &error.to_string(),
                    )
                }
            };

            match merge_three_way(language, &base, &ours, &theirs) {
                Ok(merged) => Response::ok(json!({ "merged_b64": B64.encode(merged) })),
                Err(MergeError::SemanticConflict) => Response::error_with_meta(
                    "merge_conflict",
                    Some("semantic_conflict"),
                    Some("ast_overlap_conflict"),
                    "AST semantic overlap conflict",
                ),
            }
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

    fn b64(input: &str) -> String {
        B64.encode(input.as_bytes())
    }

    #[test]
    fn merge_conflict_includes_semantic_class_and_reason() {
        let response = dispatch(Request::MergeThreeWay {
            language: crate::proto::MergeLanguage::Go,
            base_b64: b64("package main\n\nfunc value() int {\n\treturn 1\n}\n"),
            ours_b64: b64("package main\n\nfunc value() int {\n\treturn 2\n}\n"),
            theirs_b64: b64("package main\n\nfunc value() int {\n\treturn 3\n}\n"),
        });

        match response {
            Response::Err { ok, error } => {
                assert!(!ok);
                assert_eq!(error.code, "merge_conflict");
                assert_eq!(error.class.as_deref(), Some("semantic_conflict"));
                assert_eq!(error.reason_code.as_deref(), Some("ast_overlap_conflict"));
            }
            Response::Ok { .. } => panic!("expected conflict response"),
        }
    }

    #[test]
    fn invalid_payload_includes_policy_class_and_reason() {
        let response = dispatch(Request::MergeThreeWay {
            language: crate::proto::MergeLanguage::Go,
            base_b64: "%%%not-base64%%%".to_string(),
            ours_b64: b64("package main\nfunc main() {}\n"),
            theirs_b64: b64("package main\nfunc main() {}\n"),
        });

        match response {
            Response::Err { ok, error } => {
                assert!(!ok);
                assert_eq!(error.code, "invalid_input");
                assert_eq!(error.class.as_deref(), Some("policy_conflict"));
                assert_eq!(
                    error.reason_code.as_deref(),
                    Some("invalid_payload_encoding")
                );
            }
            Response::Ok { .. } => panic!("expected invalid_input response"),
        }
    }

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
