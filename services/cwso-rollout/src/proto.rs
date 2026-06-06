//! Framed-JSON IPC envelope types (same wire format as cwso-hal / cwso-sparse).

use serde::{Deserialize, Serialize};
use serde_json::Value;

pub const SERVICE: &str = "cwso-rollout";
pub const CONTRACT_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Envelope<T> {
    pub id: String,
    #[serde(flatten)]
    pub payload: T,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "op", content = "params")]
pub enum Request {
    #[serde(rename = "stat")]
    Stat,
    #[serde(rename = "capture_stats")]
    CaptureStats,
    #[serde(rename = "drain_capture")]
    DrainCapture { limit: u32 },
    #[serde(rename = "prefix_prewarm")]
    PrefixPrewarm { prefix_key: String },
    #[serde(rename = "prefix_stats")]
    PrefixStats,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Response {
    Ok { ok: bool, result: Value },
    Err { ok: bool, error: ErrorObj },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorObj {
    pub code: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason_code: Option<String>,
    pub message: String,
}

impl Response {
    pub fn ok(result: Value) -> Self {
        Self::Ok { ok: true, result }
    }

    pub fn error(code: &str, message: &str) -> Self {
        Self::Err {
            ok: false,
            error: ErrorObj {
                code: code.to_string(),
                reason_code: None,
                message: message.to_string(),
            },
        }
    }
}
