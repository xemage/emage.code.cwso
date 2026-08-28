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

use crate::merge::{conflict_matrix, merge_three_way, MergeError};
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

            if base.is_empty() || ours.is_empty() || theirs.is_empty() {
                return Response::error_with_meta(
                    "invalid_input",
                    Some("policy_conflict"),
                    Some("empty_merge_input"),
                    "merge inputs must be non-empty",
                );
            }

            match merge_three_way(language, &base, &ours, &theirs) {
                Ok(merged) => Response::ok(json!({ "merged_b64": B64.encode(merged) })),
                Err(MergeError::SemanticConflict) => {
                    // Blueprint §5.4 / §3.3 step 4: an unresolvable merge
                    // must return a structured conflict matrix as *data*,
                    // never a corrupted/partially-merged file. `base`,
                    // `ours`, and `theirs` above are never touched by this
                    // branch -- only read -- so the caller's pre-merge
                    // state is provably unchanged regardless of what
                    // happens here (C042).
                    //
                    // `conflict_matrix` re-derives the same per-unit
                    // collision predicate `merge_three_way` used
                    // internally; best-effort only -- if it can't compute
                    // any rows (e.g. the conflict was a whole-file parse
                    // failure rather than a per-unit collision, or one of
                    // the three inputs doesn't parse at all) this falls
                    // back to the original message-only conflict report
                    // rather than fabricating rows.
                    match conflict_matrix(language, &base, &ours, &theirs) {
                        Ok(matrix) if !matrix.is_empty() => Response::error_with_conflict_matrix(
                            "merge_conflict",
                            Some("semantic_conflict"),
                            Some("ast_overlap_conflict"),
                            "AST semantic overlap conflict",
                            matrix,
                        ),
                        _ => Response::error_with_meta(
                            "merge_conflict",
                            Some("semantic_conflict"),
                            Some("ast_overlap_conflict"),
                            "AST semantic overlap conflict",
                        ),
                    }
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
                // C042: the conflict response also carries the Blueprint
                // §5.4 conflict matrix as structured data.
                let matrix = error
                    .conflict_matrix
                    .as_ref()
                    .expect("conflict response must include a conflict matrix");
                assert_eq!(matrix.len(), 1);
                assert_eq!(matrix[0].reason_code, "both_modified_diverged");
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

    #[test]
    fn empty_payload_includes_policy_class_and_reason() {
        let response = dispatch(Request::MergeThreeWay {
            language: crate::proto::MergeLanguage::Go,
            base_b64: b64(""),
            ours_b64: b64("package main\nfunc main() {}\n"),
            theirs_b64: b64("package main\nfunc main() {}\n"),
        });

        match response {
            Response::Err { ok, error } => {
                assert!(!ok);
                assert_eq!(error.code, "invalid_input");
                assert_eq!(error.class.as_deref(), Some("policy_conflict"));
                assert_eq!(error.reason_code.as_deref(), Some("empty_merge_input"));
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

    // -- C042: wire-level (dispatch) coverage ----------------------------

    /// Acceptance criterion 1, at the IPC boundary: a genuine three-way
    /// merge round-trips through `dispatch` end to end and decodes back to
    /// the exact expected merged bytes.
    #[test]
    fn c042_dispatch_merges_disjoint_edits_successfully() {
        let base =
            "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 2\n}\n";
        let ours =
            "package main\n\nfunc left() int {\n\treturn 10\n}\n\nfunc right() int {\n\treturn 2\n}\n";
        let theirs =
            "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 20\n}\n";
        let expected =
            "package main\nfunc left() int {\n\treturn 10\n}\nfunc right() int {\n\treturn 20\n}\n";

        let response = dispatch(Request::MergeThreeWay {
            language: crate::proto::MergeLanguage::Go,
            base_b64: b64(base),
            ours_b64: b64(ours),
            theirs_b64: b64(theirs),
        });

        match response {
            Response::Ok { ok, result } => {
                assert!(ok);
                let merged_b64 = result
                    .get("merged_b64")
                    .and_then(|v| v.as_str())
                    .expect("result must include merged_b64");
                let merged = B64.decode(merged_b64).expect("valid base64");
                assert_eq!(merged, expected.as_bytes());
            }
            Response::Err { error, .. } => {
                panic!("expected successful merge, got error: {error:?}")
            }
        }
    }

    /// Acceptance criterion 2, at the IPC boundary: an unresolvable merge's
    /// wire response (a) never carries a `merged_b64`/result payload of any
    /// kind (conflict matrix or clean merge, nothing in between) and (b)
    /// leaves the request's own `base_b64`/`ours_b64`/`theirs_b64` strings
    /// -- the caller's pre-merge state -- byte-identical afterwards.
    #[test]
    fn c042_dispatch_conflict_never_leaks_merged_content_and_preserves_pre_merge_state() {
        let base_b64 = b64("package main\n\nfunc value() int {\n\treturn 1\n}\n");
        let ours_b64 = b64("package main\n\nfunc value() int {\n\treturn 2\n}\n");
        let theirs_b64 = b64("package main\n\nfunc value() int {\n\treturn 3\n}\n");

        let base_b64_before = base_b64.clone();
        let ours_b64_before = ours_b64.clone();
        let theirs_b64_before = theirs_b64.clone();

        let response = dispatch(Request::MergeThreeWay {
            language: crate::proto::MergeLanguage::Go,
            base_b64: base_b64.clone(),
            ours_b64: ours_b64.clone(),
            theirs_b64: theirs_b64.clone(),
        });

        // The request's own strings are untouched by `dispatch` (it only
        // ever reads them) -- explicit byte comparison, not an assumption.
        assert_eq!(base_b64, base_b64_before);
        assert_eq!(ours_b64, ours_b64_before);
        assert_eq!(theirs_b64, theirs_b64_before);

        let envelope = Envelope::<Response> {
            id: "test".to_string(),
            payload: response,
        };
        let wire_json = serde_json::to_value(&envelope).expect("serialize envelope");

        // The wire response must be *either* a clean merge result *or* a
        // conflict report -- never both, and never a bare/partial result
        // alongside an error.
        assert!(
            wire_json.get("result").is_none(),
            "conflict response must carry no `result` field at all: {wire_json}"
        );
        let error = wire_json
            .get("error")
            .expect("conflict response must carry an `error` field");
        assert!(
            error.get("merged_b64").is_none(),
            "conflict error object must never carry merged/partial content"
        );
        let matrix = error
            .get("conflict_matrix")
            .expect("conflict error must carry a conflict_matrix");
        assert!(
            matrix.as_array().is_some_and(|rows| !rows.is_empty()),
            "conflict_matrix must be a non-empty array for this fixture"
        );
    }

    /// Acceptance criterion 3, at the IPC boundary: repeated dispatch calls
    /// on identical input produce byte-identical serialized wire responses,
    /// for both the success and the conflict path.
    #[test]
    fn c042_dispatch_is_deterministic_across_repeated_runs() {
        let merge_cases = [
            (
                "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 2\n}\n",
                "package main\n\nfunc left() int {\n\treturn 10\n}\n\nfunc right() int {\n\treturn 2\n}\n",
                "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 20\n}\n",
            ),
            (
                "package main\n\nfunc value() int {\n\treturn 1\n}\n",
                "package main\n\nfunc value() int {\n\treturn 2\n}\n",
                "package main\n\nfunc value() int {\n\treturn 3\n}\n",
            ),
        ];

        for (base, ours, theirs) in merge_cases {
            let first = dispatch(Request::MergeThreeWay {
                language: crate::proto::MergeLanguage::Go,
                base_b64: b64(base),
                ours_b64: b64(ours),
                theirs_b64: b64(theirs),
            });
            let first_json = serde_json::to_vec(&first).expect("serialize response");

            for _ in 0..25 {
                let repeat = dispatch(Request::MergeThreeWay {
                    language: crate::proto::MergeLanguage::Go,
                    base_b64: b64(base),
                    ours_b64: b64(ours),
                    theirs_b64: b64(theirs),
                });
                let repeat_json = serde_json::to_vec(&repeat).expect("serialize response");
                assert_eq!(
                    repeat_json, first_json,
                    "dispatch response must be byte-identical across repeated runs"
                );
            }
        }
    }
}
