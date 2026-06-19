use std::collections::HashSet;
use std::io::{Read, Write};
use std::os::fd::AsRawFd;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;

use anyhow::{Context, Result};
use base64::Engine;
use serde_json::json;

use crate::agent::{AgentError, AgentRegistry};
use crate::gemm::TernaryWeights;
use crate::proto::{Envelope, Request, Response};

pub const SERVICE: &str = "cwso-sparse";
pub const CONTRACT_VERSION: u32 = 2;

const FRAME_HEADER: usize = 4;
const FRAME_MAX: usize = 8 * 1024 * 1024;

pub fn run(socket_path: PathBuf, agents: Option<Arc<AgentRegistry>>) -> Result<()> {
    if let Some(parent) = socket_path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    let _ = std::fs::remove_file(&socket_path);
    let listener = UnixListener::bind(&socket_path)
        .with_context(|| format!("bind cwso-sparse socket {socket_path:?}"))?;

    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;

    let authz_policy = Arc::new(IpcAuthzPolicy::from_env()?);
    tracing::info!(
        ?socket_path,
        agents_enabled = agents.is_some(),
        "cwso-sparse ready"
    );

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                let authz_policy = Arc::clone(&authz_policy);
                let agents = agents.as_ref().map(Arc::clone);
                thread::spawn(move || {
                    if let Err(error) = handle_client(stream, &authz_policy, agents.as_deref()) {
                        tracing::warn!(error = %error, "cwso-sparse client error");
                    }
                });
            }
            Err(error) => tracing::warn!(error = %error, "cwso-sparse accept error"),
        }
    }
    Ok(())
}

fn handle_client(
    mut stream: UnixStream,
    authz_policy: &IpcAuthzPolicy,
    agents: Option<&AgentRegistry>,
) -> Result<()> {
    if !authorize_stream(&stream, authz_policy)? {
        tracing::warn!("rejected unauthorized cwso-sparse IPC peer");
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
        let response = dispatch(agents, request.payload);
        let envelope = Envelope::<Response> {
            id,
            payload: response,
        };
        write_frame(&mut stream, &serde_json::to_vec(&envelope)?)?;
    }
}

/// dispatch executes one request.
pub fn dispatch(agents: Option<&AgentRegistry>, request: Request) -> Response {
    match request {
        Request::Stat => Response::ok(json!({
            "service": SERVICE,
            "contract_version": CONTRACT_VERSION,
            "agents_enabled": agents.is_some(),
        })),
        Request::TernaryGemm {
            n,
            k,
            m,
            scales,
            packed_b64,
            activations,
        } => ternary_gemm(n, k, m, scales, packed_b64, activations),
        Request::CreateAgent {
            skill_domain,
            quantization,
            max_ram_mb,
            target_ast_node,
        } => create_agent(
            agents,
            skill_domain,
            quantization,
            max_ram_mb,
            target_ast_node,
        ),
        Request::DropAgent { agent_id } => drop_agent(agents, agent_id),
        Request::AgentStat { agent_id } => agent_stat(agents, agent_id),
    }
}

fn create_agent(
    agents: Option<&AgentRegistry>,
    skill_domain: String,
    quantization: String,
    max_ram_mb: u32,
    target_ast_node: Option<String>,
) -> Response {
    let Some(reg) = agents else {
        return Response::error_with_reason(
            "disabled",
            Some("agents_disabled"),
            "sparse agent lifecycle is not configured (set CWSO_SPARSE_SLICE_MANIFEST)",
        );
    };
    match reg.create_agent(&skill_domain, &quantization, max_ram_mb, target_ast_node) {
        Ok(snap) => Response::ok(json!({
            "wasm_agent_id": snap.agent_id,
            "skill_domain": snap.skill_domain,
            "slice_sha256": snap.slice_sha256,
            "state": snap.state,
            "cold_start_ms": snap.cold_start_ms,
            "resident_ram_mb": snap.resident_ram_mb,
            "tokens_per_sec": snap.tokens_per_sec,
            "target_ast_node": snap.target_ast_node,
        })),
        Err(error) => agent_error_response(error),
    }
}

fn drop_agent(agents: Option<&AgentRegistry>, agent_id: String) -> Response {
    let Some(reg) = agents else {
        return Response::error_with_reason(
            "disabled",
            Some("agents_disabled"),
            "sparse agent lifecycle is not configured",
        );
    };
    match reg.drop_agent(&agent_id) {
        Ok(()) => Response::ok(json!({ "dropped": true, "wasm_agent_id": agent_id })),
        Err(error) => agent_error_response(error),
    }
}

fn agent_stat(agents: Option<&AgentRegistry>, agent_id: String) -> Response {
    let Some(reg) = agents else {
        return Response::error_with_reason(
            "disabled",
            Some("agents_disabled"),
            "sparse agent lifecycle is not configured",
        );
    };
    match reg.agent_stat(&agent_id) {
        Ok(snap) => Response::ok(json!({
            "wasm_agent_id": snap.agent_id,
            "skill_domain": snap.skill_domain,
            "slice_sha256": snap.slice_sha256,
            "state": snap.state,
            "cold_start_ms": snap.cold_start_ms,
            "resident_ram_mb": snap.resident_ram_mb,
            "tokens_per_sec": snap.tokens_per_sec,
            "target_ast_node": snap.target_ast_node,
        })),
        Err(error) => agent_error_response(error),
    }
}

fn agent_error_response(error: AgentError) -> Response {
    let (code, reason) = match &error {
        AgentError::UnknownDomain(_) => ("invalid_input", Some("unknown_skill_domain")),
        AgentError::UnsupportedQuantization(_) => {
            ("invalid_input", Some("unsupported_quantization"))
        }
        AgentError::InvalidRAM | AgentError::RAMCapExceeded { .. } => {
            ("invalid_input", Some("invalid_max_ram_mb"))
        }
        AgentError::UnknownAgent(_) => ("not_found", Some("unknown_agent")),
        AgentError::Disabled => ("disabled", Some("agents_disabled")),
        AgentError::ModuleIntegrity { .. } => ("invalid_input", Some("module_integrity")),
        AgentError::Slice(_) | AgentError::Io(_) | AgentError::Wasm(_) => {
            ("invalid_input", Some("create_failed"))
        }
    };
    Response::error_with_reason(code, reason, &error.to_string())
}

fn ternary_gemm(
    n: usize,
    k: usize,
    m: usize,
    scales: Vec<f32>,
    packed_b64: String,
    activations: Vec<f32>,
) -> Response {
    let packed = match base64::engine::general_purpose::STANDARD.decode(packed_b64.as_bytes()) {
        Ok(bytes) => bytes,
        Err(error) => {
            return Response::error_with_reason(
                "invalid_input",
                Some("bad_base64"),
                &format!("packed_b64 is not valid base64: {error}"),
            )
        }
    };

    let weights = match TernaryWeights::new(n, k, scales, packed) {
        Ok(weights) => weights,
        Err(error) => {
            return Response::error_with_reason(
                "invalid_input",
                Some("bad_weights"),
                &error.to_string(),
            )
        }
    };

    match weights.gemm(&activations, m) {
        Ok(output) => Response::ok(json!({ "m": m, "n": n, "output": output })),
        Err(error) => {
            Response::error_with_reason("invalid_input", Some("gemm_failed"), &error.to_string())
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
    use crate::gemm::pack_ternary;

    fn unwrap_ok(response: Response) -> serde_json::Value {
        match response {
            Response::Ok { ok, result } => {
                assert!(ok);
                result
            }
            Response::Err { error, .. } => panic!("expected ok, got error {error:?}"),
        }
    }

    fn unwrap_err(response: Response) -> crate::proto::ErrorObj {
        match response {
            Response::Err { ok, error } => {
                assert!(!ok);
                error
            }
            Response::Ok { .. } => panic!("expected error"),
        }
    }

    #[test]
    fn stat_reports_service_and_contract() {
        let result = unwrap_ok(dispatch(None, Request::Stat));
        assert_eq!(result["service"], SERVICE);
        assert_eq!(result["contract_version"], CONTRACT_VERSION);
    }

    #[test]
    fn ternary_gemm_op_computes_identity() {
        // Weights are packed row-by-row (each row padded to packed_row_bytes(k)); a 3x3
        // ternary identity must be packed as three separate rows, not one flat buffer.
        let mut packed = Vec::new();
        for row in [[1i8, 0, 0], [0, 1, 0], [0, 0, 1]] {
            packed.extend(pack_ternary(&row).unwrap());
        }
        let b64 = base64::engine::general_purpose::STANDARD.encode(&packed);
        let result = unwrap_ok(dispatch(
            None,
            Request::TernaryGemm {
                n: 3,
                k: 3,
                m: 1,
                scales: vec![1.0, 1.0, 1.0],
                packed_b64: b64,
                activations: vec![5.0, 6.0, 7.0],
            },
        ));
        assert_eq!(result["output"], json!([5.0, 6.0, 7.0]));
        assert_eq!(result["m"], 1);
        assert_eq!(result["n"], 3);
    }

    #[test]
    fn ternary_gemm_op_rejects_bad_base64() {
        let error = unwrap_err(dispatch(
            None,
            Request::TernaryGemm {
                n: 1,
                k: 1,
                m: 1,
                scales: vec![1.0],
                packed_b64: "not base64!!!".to_string(),
                activations: vec![1.0],
            },
        ));
        assert_eq!(error.code, "invalid_input");
        assert_eq!(error.reason_code.as_deref(), Some("bad_base64"));
    }

    #[test]
    fn ternary_gemm_op_rejects_shape_mismatch() {
        let packed = pack_ternary(&[1, 0]).unwrap();
        let b64 = base64::engine::general_purpose::STANDARD.encode(&packed);
        let error = unwrap_err(dispatch(
            None,
            Request::TernaryGemm {
                n: 1,
                k: 2,
                m: 1,
                scales: vec![1.0],
                packed_b64: b64,
                activations: vec![1.0, 2.0, 3.0], // wrong length for m*k=2
            },
        ));
        assert_eq!(error.code, "invalid_input");
        assert_eq!(error.reason_code.as_deref(), Some("gemm_failed"));
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
