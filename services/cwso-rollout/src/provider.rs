//! Provider detection and request/response normalization (Polar §3.2, T132).

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use thiserror::Error;

/// Provider identifies the client-facing API shape.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Provider {
    OpenAiChat,
    AnthropicMessages,
    GoogleGenerateContent,
    Unknown,
}

impl Provider {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::OpenAiChat => "openai_chat",
            Self::AnthropicMessages => "anthropic_messages",
            Self::GoogleGenerateContent => "google_generate_content",
            Self::Unknown => "unknown",
        }
    }

    pub fn from_wire_str(value: &str) -> Self {
        match value {
            "openai_chat" => Self::OpenAiChat,
            "anthropic_messages" => Self::AnthropicMessages,
            "google_generate_content" => Self::GoogleGenerateContent,
            _ => Self::Unknown,
        }
    }
}

/// DetectProvider classifies an inbound proxy request by path.
pub fn detect_provider(path: &str) -> Provider {
    let path = path.trim_end_matches('/');
    if path.ends_with("/v1/chat/completions") || path == "/v1/chat/completions" {
        Provider::OpenAiChat
    } else if path.ends_with("/v1/messages") || path == "/v1/messages" {
        Provider::AnthropicMessages
    } else if path.contains(":generateContent") {
        Provider::GoogleGenerateContent
    } else {
        Provider::Unknown
    }
}

#[derive(Debug, Error, PartialEq, Eq)]
pub enum NormalizeError {
    #[error("unsupported provider for normalize")]
    UnsupportedProvider,
    #[error("request body is not valid JSON")]
    InvalidJson,
}

/// NormalizeRequest transforms a client request into OpenAI Chat Completions shape and forces
/// `logprobs=true` when the upstream supports it. Returns `(normalized_body, client_wants_stream)`.
pub fn normalize_request(provider: Provider, body: &[u8]) -> Result<(Value, bool), NormalizeError> {
    match provider {
        Provider::OpenAiChat => normalize_openai_chat(body),
        Provider::AnthropicMessages => normalize_anthropic(body),
        Provider::GoogleGenerateContent => normalize_google(body),
        Provider::Unknown => Err(NormalizeError::UnsupportedProvider),
    }
}

fn normalize_openai_chat(body: &[u8]) -> Result<(Value, bool), NormalizeError> {
    let mut value: Value = serde_json::from_slice(body).map_err(|_| NormalizeError::InvalidJson)?;
    let wants_stream = value
        .get("stream")
        .and_then(Value::as_bool)
        .unwrap_or(false);

    if let Some(obj) = value.as_object_mut() {
        obj.insert("logprobs".to_string(), json!(true));
        obj.insert("stream".to_string(), json!(false));
    }
    Ok((value, wants_stream))
}

fn normalize_anthropic(body: &[u8]) -> Result<(Value, bool), NormalizeError> {
    let anthropic: Value = serde_json::from_slice(body).map_err(|_| NormalizeError::InvalidJson)?;
    let wants_stream = anthropic
        .get("stream")
        .and_then(Value::as_bool)
        .unwrap_or(false);

    let model = anthropic
        .get("model")
        .and_then(Value::as_str)
        .unwrap_or("claude-3-5-sonnet-latest");
    let max_tokens = anthropic
        .get("max_tokens")
        .and_then(Value::as_u64)
        .unwrap_or(1024);

    let mut messages = Vec::new();
    if let Some(system) = anthropic.get("system").and_then(Value::as_str) {
        messages.push(json!({"role": "system", "content": system}));
    }
    if let Some(items) = anthropic.get("messages").and_then(Value::as_array) {
        for item in items {
            let role = item.get("role").and_then(Value::as_str).unwrap_or("user");
            let content = extract_anthropic_content(item.get("content"));
            messages.push(json!({"role": role, "content": content}));
        }
    }

    let normalized = json!({
        "model": model,
        "messages": messages,
        "max_tokens": max_tokens,
        "logprobs": true,
        "stream": false,
    });
    Ok((normalized, wants_stream))
}

fn extract_anthropic_content(content: Option<&Value>) -> Value {
    match content {
        Some(Value::String(text)) => json!(text),
        Some(Value::Array(blocks)) => {
            let text: String = blocks
                .iter()
                .filter_map(|block| block.get("text").and_then(Value::as_str))
                .collect::<Vec<_>>()
                .join("");
            json!(text)
        }
        Some(other) => other.clone(),
        None => json!(""),
    }
}

fn normalize_google(body: &[u8]) -> Result<(Value, bool), NormalizeError> {
    let google: Value = serde_json::from_slice(body).map_err(|_| NormalizeError::InvalidJson)?;
    let wants_stream = false;

    let mut messages = Vec::new();
    if let Some(contents) = google
        .get("contents")
        .and_then(Value::as_array)
        .or_else(|| google.get("contents").and_then(Value::as_array))
    {
        for item in contents {
            let role = item.get("role").and_then(Value::as_str).unwrap_or("user");
            let text = item
                .get("parts")
                .and_then(Value::as_array)
                .and_then(|parts| parts.first())
                .and_then(|part| part.get("text"))
                .and_then(Value::as_str)
                .unwrap_or("");
            messages.push(json!({"role": role, "content": text}));
        }
    }

    let model = google
        .get("model")
        .and_then(Value::as_str)
        .unwrap_or("gemini-pro");

    let normalized = json!({
        "model": model,
        "messages": messages,
        "logprobs": true,
        "stream": false,
    });
    Ok((normalized, wants_stream))
}

/// DenormalizeResponse transforms an upstream OpenAI-shaped completion back to the client's
/// provider format. When `client_wants_stream` is true, returns synthetic SSE bytes.
pub fn denormalize_response(
    provider: Provider,
    upstream_body: &[u8],
    client_wants_stream: bool,
) -> Result<(Vec<u8>, &'static str), NormalizeError> {
    if client_wants_stream {
        return Ok((synthetic_sse(upstream_body)?, "text/event-stream"));
    }

    match provider {
        Provider::OpenAiChat => Ok((upstream_body.to_vec(), "application/json")),
        Provider::AnthropicMessages => Ok((
            anthropic_from_openai(upstream_body)?.to_vec(),
            "application/json",
        )),
        Provider::GoogleGenerateContent => Ok((
            google_from_openai(upstream_body)?.to_vec(),
            "application/json",
        )),
        Provider::Unknown => Err(NormalizeError::UnsupportedProvider),
    }
}

fn synthetic_sse(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed: Value =
        serde_json::from_slice(upstream_body).map_err(|_| NormalizeError::InvalidJson)?;
    let content = parsed
        .pointer("/choices/0/message/content")
        .and_then(Value::as_str)
        .unwrap_or("");
    let id = parsed
        .get("id")
        .and_then(Value::as_str)
        .unwrap_or("chatcmpl-synthetic");

    let chunk = json!({
        "id": id,
        "object": "chat.completion.chunk",
        "choices": [{
            "index": 0,
            "delta": {"content": content},
            "finish_reason": parsed.pointer("/choices/0/finish_reason"),
        }]
    });
    let done = "data: [DONE]\n\n";
    Ok(format!(
        "data: {}\n\n{done}",
        serde_json::to_string(&chunk).unwrap_or_else(|_| "{}".to_string())
    )
    .into_bytes())
}

fn anthropic_from_openai(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed: Value =
        serde_json::from_slice(upstream_body).map_err(|_| NormalizeError::InvalidJson)?;
    let text = parsed
        .pointer("/choices/0/message/content")
        .and_then(Value::as_str)
        .unwrap_or("");
    let stop = parsed
        .pointer("/choices/0/finish_reason")
        .and_then(Value::as_str)
        .map(|reason| match reason {
            "stop" => "end_turn",
            other => other,
        })
        .unwrap_or("end_turn");

    let out = json!({
        "id": parsed.get("id").cloned().unwrap_or(json!("msg-synthetic")),
        "type": "message",
        "role": "assistant",
        "content": [{"type": "text", "text": text}],
        "model": parsed.get("model").cloned().unwrap_or(json!("")),
        "stop_reason": stop,
        "usage": parsed.get("usage").cloned().unwrap_or(json!({})),
    });
    serde_json::to_vec(&out).map_err(|_| NormalizeError::InvalidJson)
}

fn google_from_openai(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed: Value =
        serde_json::from_slice(upstream_body).map_err(|_| NormalizeError::InvalidJson)?;
    let text = parsed
        .pointer("/choices/0/message/content")
        .and_then(Value::as_str)
        .unwrap_or("");

    let out = json!({
        "candidates": [{
            "content": {
                "parts": [{"text": text}],
                "role": "model",
            },
            "finishReason": parsed.pointer("/choices/0/finish_reason").cloned().unwrap_or(json!("STOP")),
        }],
        "usageMetadata": parsed.get("usage").cloned().unwrap_or(json!({})),
    });
    serde_json::to_vec(&out).map_err(|_| NormalizeError::InvalidJson)
}

/// ExtractCaptureFields pulls token/logprob fields from an upstream OpenAI completion body.
pub fn extract_capture_fields(
    request_id: &str,
    provider: Provider,
    upstream_body: &[u8],
    timestamp_ns: u64,
) -> Result<super::record::CompletionRecord, NormalizeError> {
    let parsed: Value =
        serde_json::from_slice(upstream_body).map_err(|_| NormalizeError::InvalidJson)?;

    let finish_reason = parsed
        .pointer("/choices/0/finish_reason")
        .and_then(Value::as_str)
        .map(str::to_string);

    let mut logprobs = Vec::new();
    let mut sampled_token_ids = Vec::new();

    if let Some(content_items) = parsed
        .pointer("/choices/0/logprobs/content")
        .and_then(Value::as_array)
    {
        for item in content_items {
            if let Some(token) = item.get("token").and_then(Value::as_str) {
                let _ = token;
            }
            if let Some(id) = item.get("token_id").and_then(Value::as_u64) {
                sampled_token_ids.push(id as u32);
            }
            if let Some(lp) = item.get("logprob").and_then(Value::as_f64) {
                logprobs.push(lp);
            }
        }
    }

    Ok(super::record::CompletionRecord {
        request_id: request_id.to_string(),
        provider,
        prompt_token_ids: parsed
            .pointer("/usage/prompt_tokens")
            .and_then(Value::as_u64)
            .map(|count| (0..count).map(|i| i as u32).collect())
            .unwrap_or_default(),
        sampled_token_ids,
        logprobs,
        finish_reason,
        timestamp_ns,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detect_openai_path() {
        assert_eq!(
            detect_provider("/v1/chat/completions"),
            Provider::OpenAiChat
        );
    }

    #[test]
    fn detect_anthropic_path() {
        assert_eq!(detect_provider("/v1/messages"), Provider::AnthropicMessages);
    }

    #[test]
    fn openai_normalize_forces_logprobs_and_disables_upstream_stream() {
        let body = br#"{"model":"gpt-4","messages":[],"stream":true}"#;
        let (normalized, wants_stream) =
            normalize_request(Provider::OpenAiChat, body).expect("normalize");
        assert!(wants_stream);
        assert_eq!(normalized["logprobs"], json!(true));
        assert_eq!(normalized["stream"], json!(false));
    }

    #[test]
    fn anthropic_normalize_maps_messages() {
        let body = br#"{"model":"claude","max_tokens":64,"system":"sys","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"stream":true}"#;
        let (normalized, wants_stream) =
            normalize_request(Provider::AnthropicMessages, body).expect("normalize");
        assert!(wants_stream);
        assert_eq!(normalized["messages"][0]["role"], "system");
        assert_eq!(normalized["messages"][1]["content"], "hi");
        assert_eq!(normalized["logprobs"], json!(true));
    }

    #[test]
    fn synthetic_sse_wraps_upstream_completion() {
        let upstream =
            br#"{"id":"x","choices":[{"message":{"content":"hello"},"finish_reason":"stop"}]}"#;
        let (body, content_type) =
            denormalize_response(Provider::OpenAiChat, upstream, true).expect("denorm");
        assert_eq!(content_type, "text/event-stream");
        let text = String::from_utf8(body).expect("utf8");
        assert!(text.contains("data:"));
        assert!(text.contains("[DONE]"));
        assert!(text.contains("hello"));
    }

    #[test]
    fn extract_capture_reads_logprobs() {
        let upstream = br#"{"choices":[{"finish_reason":"stop","logprobs":{"content":[{"token_id":9,"logprob":-0.2}]}}],"usage":{"prompt_tokens":2}}"#;
        let record =
            extract_capture_fields("req", Provider::OpenAiChat, upstream, 1).expect("extract");
        assert_eq!(record.request_id, "req");
        assert_eq!(record.sampled_token_ids, vec![9]);
        assert_eq!(record.logprobs, vec![-0.2]);
        assert_eq!(record.prompt_token_ids.len(), 2);
    }
}
