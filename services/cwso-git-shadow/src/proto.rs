//! Wire protocol envelopes.
//!
//! Frame body is one JSON `Envelope<Request>` or `Envelope<Response>`.

use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct Envelope<T> {
    pub id: String,
    #[serde(flatten)]
    pub payload: T,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "op", content = "params")]
pub enum Request {
    #[serde(rename = "stat")]
    Stat,

    #[serde(rename = "create_workspace")]
    CreateWorkspace {
        // For PoC we always start from an empty tree if base_commit_sha is None.
        base_commit_sha: Option<String>,
    },

    #[serde(rename = "list_workspaces")]
    ListWorkspaces,

    #[serde(rename = "get_workspace")]
    GetWorkspace { workspace_uuid: String },

    #[serde(rename = "drop_workspace")]
    DropWorkspace { workspace_uuid: String },

    #[serde(rename = "write_file")]
    WriteFile {
        workspace_uuid: String,
        path: String,
        content_b64: String,
    },

    #[serde(rename = "read_file")]
    ReadFile {
        workspace_uuid: String,
        path: String,
    },

    #[serde(rename = "list_files")]
    ListFiles { workspace_uuid: String },

    #[serde(rename = "commit")]
    Commit {
        workspace_uuid: String,
        message: String,
    },

    #[serde(rename = "query_ast")]
    QueryAst {
        workspace_uuid: String,
        path: String,
        query_type: String, // find_definition | find_references | extract_signature | list_exports | detect_entrypoints
        target_symbol: String,
    },
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Response {
    Ok { ok: bool, result: serde_json::Value },
    Err { ok: bool, error: ErrorObj },
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ErrorObj {
    pub code: String,
    pub message: String,
}

impl Response {
    pub fn ok(value: serde_json::Value) -> Self {
        Response::Ok {
            ok: true,
            result: value,
        }
    }
    pub fn error(code: &str, message: &str) -> Self {
        Response::Err {
            ok: false,
            error: ErrorObj {
                code: code.to_string(),
                message: message.to_string(),
            },
        }
    }
}
