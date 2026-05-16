use std::io::{Read, Write};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
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
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o666))?;

    tracing::info!(?socket_path, "cwso-merge-engine ready");

    for stream in listener.incoming() {
        match stream {
            Ok(stream) => {
                thread::spawn(move || {
                    if let Err(error) = handle_client(stream) {
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

fn handle_client(mut stream: UnixStream) -> Result<()> {
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
                Err(error) => return Response::error("invalid_input", &error.to_string()),
            };

            match merge_three_way(language, &base, &ours, &theirs) {
                Ok(merged) => Response::ok(json!({ "merged_b64": B64.encode(merged) })),
                Err(MergeError::SemanticConflict) => {
                    Response::error("unimplemented_conflict", "AST semantic overlap conflict")
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
