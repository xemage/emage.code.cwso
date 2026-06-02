use std::collections::HashSet;
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;

use anyhow::{Context, Result};
use serde_json::json;

use crate::backend::CONTRACT_VERSION;
use crate::proto::{Envelope, Request, Response};
use crate::registry::{BackendRegistry, RegistryError};

const FRAME_HEADER: usize = 4;
const FRAME_MAX: usize = 8 * 1024 * 1024;

pub fn run(socket_path: PathBuf, registry: Arc<BackendRegistry>) -> Result<()> {
    if let Some(parent) = socket_path.parent() {
        std::fs::create_dir_all(parent).ok();
    }

    let _ = std::fs::remove_file(&socket_path);
    let listener = UnixListener::bind(&socket_path)
        .with_context(|| format!("bind cwso-hal socket {socket_path:?}"))?;

    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;

    let authz_policy = Arc::new(IpcAuthzPolicy::from_env()?);

    tracing::info!(?socket_path, providers = ?registry.provider_ids(), "cwso-hal ready");

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                let authz_policy = Arc::clone(&authz_policy);
                let registry = Arc::clone(&registry);
                thread::spawn(move || {
                    if let Err(error) = handle_client(stream, &authz_policy, &registry) {
                        tracing::warn!(error = %error, "cwso-hal client error");
                    }
                });
            }
            Err(error) => {
                tracing::warn!(error = %error, "cwso-hal accept error");
            }
        }
    }
    Ok(())
}

fn handle_client(
    mut stream: UnixStream,
    authz_policy: &IpcAuthzPolicy,
    registry: &BackendRegistry,
) -> Result<()> {
    if !authorize_stream(&stream, authz_policy)? {
        tracing::warn!("rejected unauthorized cwso-hal IPC peer");
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
        let response = dispatch(registry, request.payload);
        let envelope = Envelope::<Response> {
            id,
            payload: response,
        };
        write_frame(&mut stream, &serde_json::to_vec(&envelope)?)?;
    }
}

fn dispatch(registry: &BackendRegistry, request: Request) -> Response {
    match request {
        Request::Stat => Response::ok(json!({
            "service": "cwso-hal",
            "contract_version": CONTRACT_VERSION,
            "providers": registry.provider_ids(),
        })),
        Request::Capabilities => match serde_json::to_value(registry.capabilities()) {
            Ok(value) => Response::ok(json!({ "providers": value })),
            Err(error) => Response::error("internal", &error.to_string()),
        },
        Request::Health { provider_id } => match registry.health(&provider_id) {
            Some(health) => match serde_json::to_value(health) {
                Ok(value) => Response::ok(json!({ "provider_id": provider_id, "health": value })),
                Err(error) => Response::error("internal", &error.to_string()),
            },
            None => Response::error_with_meta(
                "unknown_provider",
                Some("invalid_request"),
                Some("provider_not_registered"),
                &format!("no backend registered for {provider_id}"),
            ),
        },
        Request::Infer {
            selected_provider,
            fallback_chain,
            request,
        } => {
            if request.prompt.trim().is_empty() {
                return Response::error_with_meta(
                    "invalid_input",
                    Some("invalid_request"),
                    Some("empty_prompt"),
                    "request.prompt must be non-empty",
                );
            }

            match registry.dispatch(&selected_provider, &fallback_chain, &request) {
                Ok(outcome) => {
                    let completion =
                        serde_json::to_value(&outcome.completion).unwrap_or_else(|_| json!({}));
                    let attempts: Vec<_> = outcome
                        .attempts
                        .iter()
                        .map(|attempt| {
                            json!({ "provider_id": attempt.provider_id, "outcome": attempt.outcome })
                        })
                        .collect();
                    Response::ok(json!({
                        "served_by": outcome.served_by,
                        "requested_provider": outcome.requested_provider,
                        "fallback_count": outcome.fallback_count,
                        "completion": completion,
                        "attempts": attempts,
                    }))
                }
                Err(RegistryError::Exhausted { attempts }) => {
                    let attempts: Vec<_> = attempts
                        .iter()
                        .map(|attempt| {
                            json!({ "provider_id": attempt.provider_id, "outcome": attempt.outcome })
                        })
                        .collect();
                    Response::error_with_meta(
                        "dispatch_failed",
                        Some("unavailable"),
                        Some("all_backends_exhausted"),
                        &format!(
                            "all backends exhausted: {}",
                            serde_json::to_string(&attempts).unwrap_or_default()
                        ),
                    )
                }
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
    use crate::proto::Response;

    fn registry() -> BackendRegistry {
        BackendRegistry::with_cpu_baseline()
    }

    fn unwrap_ok(response: Response) -> serde_json::Value {
        match response {
            Response::Ok { ok, result } => {
                assert!(ok);
                result
            }
            Response::Err { error, .. } => panic!("expected ok, got error {error:?}"),
        }
    }

    #[test]
    fn stat_reports_service_and_providers() {
        let result = unwrap_ok(dispatch(&registry(), Request::Stat));
        assert_eq!(result["service"], "cwso-hal");
        assert_eq!(result["contract_version"], CONTRACT_VERSION);
        assert!(result["providers"]
            .as_array()
            .unwrap()
            .iter()
            .any(|p| p == "cpu-baseline"));
    }

    #[test]
    fn capabilities_lists_baseline() {
        let result = unwrap_ok(dispatch(&registry(), Request::Capabilities));
        let providers = result["providers"].as_array().unwrap();
        assert!(providers.iter().any(|p| p["provider_id"] == "cpu-baseline"));
    }

    #[test]
    fn health_for_known_provider() {
        let result = unwrap_ok(dispatch(
            &registry(),
            Request::Health {
                provider_id: "cpu-baseline".to_string(),
            },
        ));
        assert_eq!(result["health"]["state"], "healthy");
    }

    #[test]
    fn health_for_unknown_provider_is_error() {
        let response = dispatch(
            &registry(),
            Request::Health {
                provider_id: "ghost".to_string(),
            },
        );
        match response {
            Response::Err { ok, error } => {
                assert!(!ok);
                assert_eq!(error.code, "unknown_provider");
                assert_eq!(
                    error.reason_code.as_deref(),
                    Some("provider_not_registered")
                );
            }
            Response::Ok { .. } => panic!("expected error"),
        }
    }

    #[test]
    fn infer_routes_to_baseline_and_is_deterministic() {
        let request = crate::backend::InferenceRequest {
            request_id: "req-9".to_string(),
            workload_tags: vec![],
            prompt: "summarize this".to_string(),
            context_tokens: 0,
            max_output_tokens: 0,
            latency_class: "batch".to_string(),
        };
        let make = || {
            dispatch(
                &registry(),
                Request::Infer {
                    selected_provider: "cpu-baseline".to_string(),
                    fallback_chain: vec![],
                    request: request.clone(),
                },
            )
        };
        let a = unwrap_ok(make());
        let b = unwrap_ok(make());
        assert_eq!(a["served_by"], "cpu-baseline");
        assert_eq!(a["fallback_count"], 0);
        assert_eq!(a["completion"]["output"], b["completion"]["output"]);
    }

    #[test]
    fn infer_with_empty_prompt_is_rejected() {
        let request = crate::backend::InferenceRequest {
            request_id: String::new(),
            workload_tags: vec![],
            prompt: "   ".to_string(),
            context_tokens: 0,
            max_output_tokens: 0,
            latency_class: String::new(),
        };
        let response = dispatch(
            &registry(),
            Request::Infer {
                selected_provider: "cpu-baseline".to_string(),
                fallback_chain: vec![],
                request,
            },
        );
        match response {
            Response::Err { error, .. } => {
                assert_eq!(error.code, "invalid_input");
                assert_eq!(error.reason_code.as_deref(), Some("empty_prompt"));
            }
            Response::Ok { .. } => panic!("expected invalid_input"),
        }
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn authorize_stream_rejects_unauthorized_peer() {
        let (client, server) = UnixStream::pair().expect("create unix pair");
        let _client = client;
        let policy = IpcAuthzPolicy::from_allowed(&[u32::MAX], &[u32::MAX]);
        let authorized = authorize_stream(&server, &policy).expect("authorize stream");
        assert!(!authorized, "peer must be rejected when not allowlisted");
    }
}
