use serde::{Deserialize, Serialize};

/// Wire envelope shared with the orchestrator (matches cwso-hal / cwso-git-shadow framing:
/// 4-byte big-endian length prefix + JSON body).
#[derive(Debug, Serialize, Deserialize)]
pub struct Envelope<T> {
    pub id: String,
    #[serde(flatten)]
    pub payload: T,
}

/// Request is the cwso-sparse IPC surface (JSON, tagged by `op`).
#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "op", content = "params")]
pub enum Request {
    #[serde(rename = "stat")]
    Stat,
    /// The single sandboxed compute host-call (ADR-008): run a deterministic ternary GEMM.
    /// `packed_b64` is the base64 of the 2-bit-packed `[n, k]` weight matrix; `activations`
    /// is row-major `[m, k]`. Returns row-major `[m, n]` output.
    #[serde(rename = "ternary_gemm")]
    TernaryGemm {
        n: usize,
        k: usize,
        m: usize,
        scales: Vec<f32>,
        packed_b64: String,
        activations: Vec<f32>,
    },
    /// Instantiate an ephemeral sparse micro-agent over a pinned skill slice (T122).
    #[serde(rename = "create_agent")]
    CreateAgent {
        skill_domain: String,
        #[serde(default = "default_quantization")]
        quantization: String,
        #[serde(default = "default_max_ram_mb")]
        max_ram_mb: u32,
        #[serde(default)]
        target_ast_node: Option<String>,
    },
    #[serde(rename = "drop_agent")]
    DropAgent { agent_id: String },
    #[serde(rename = "agent_stat")]
    AgentStat { agent_id: String },
}

fn default_quantization() -> String {
    "1.58-bit".to_string()
}

fn default_max_ram_mb() -> u32 {
    512
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason_code: Option<String>,
    pub message: String,
}

impl Response {
    pub fn ok(result: serde_json::Value) -> Self {
        Self::Ok { ok: true, result }
    }

    pub fn error(code: &str, message: &str) -> Self {
        Self::error_with_reason(code, None, message)
    }

    pub fn error_with_reason(code: &str, reason_code: Option<&str>, message: &str) -> Self {
        Self::Err {
            ok: false,
            error: ErrorObj {
                code: code.to_string(),
                reason_code: reason_code.map(ToString::to_string),
                message: message.to_string(),
            },
        }
    }
}
