use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct Envelope<T> {
    pub id: String,
    #[serde(flatten)]
    pub payload: T,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MergeLanguage {
    Go,
    Rust,
    Python,
    TypeScript,
}

impl MergeLanguage {
    pub fn as_wire(self) -> &'static str {
        match self {
            MergeLanguage::Go => "go",
            MergeLanguage::Rust => "rust",
            MergeLanguage::Python => "python",
            MergeLanguage::TypeScript => "typescript",
        }
    }
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "op", content = "params")]
pub enum Request {
    #[serde(rename = "stat")]
    Stat,
    #[serde(rename = "merge_three_way")]
    MergeThreeWay {
        language: MergeLanguage,
        base_b64: String,
        ours_b64: String,
        theirs_b64: String,
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub class: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason_code: Option<String>,
    pub message: String,
}

impl Response {
    pub fn ok(result: serde_json::Value) -> Self {
        Self::Ok { ok: true, result }
    }

    pub fn error(code: &str, message: &str) -> Self {
        Self::error_with_meta(code, None, None, message)
    }

    pub fn error_with_meta(
        code: &str,
        class: Option<&str>,
        reason_code: Option<&str>,
        message: &str,
    ) -> Self {
        Self::Err {
            ok: false,
            error: ErrorObj {
                code: code.to_string(),
                class: class.map(ToString::to_string),
                reason_code: reason_code.map(ToString::to_string),
                message: message.to_string(),
            },
        }
    }
}
