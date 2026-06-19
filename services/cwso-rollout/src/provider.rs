//! Provider detection and request/response normalization (Polar §3.2, T132, T147).

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use thiserror::Error;

/// Provider identifies the client-facing API shape.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Provider {
    OpenAiChat,
    OpenAiResponses,
    AnthropicMessages,
    GoogleGenerateContent,
    Unknown,
}

impl Provider {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::OpenAiChat => "openai_chat",
            Self::OpenAiResponses => "openai_responses",
            Self::AnthropicMessages => "anthropic_messages",
            Self::GoogleGenerateContent => "google_generate_content",
            Self::Unknown => "unknown",
        }
    }

    pub fn from_wire_str(value: &str) -> Self {
        match value {
            "openai_chat" => Self::OpenAiChat,
            "openai_responses" => Self::OpenAiResponses,
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
    } else if path.ends_with("/v1/responses") || path == "/v1/responses" {
        Provider::OpenAiResponses
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
        Provider::OpenAiResponses => normalize_openai_responses(body),
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

fn normalize_openai_responses(body: &[u8]) -> Result<(Value, bool), NormalizeError> {
    let responses: Value = serde_json::from_slice(body).map_err(|_| NormalizeError::InvalidJson)?;
    let wants_stream = responses
        .get("stream")
        .and_then(Value::as_bool)
        .unwrap_or(false);

    let model = responses
        .get("model")
        .and_then(Value::as_str)
        .unwrap_or("gpt-4o");
    let max_tokens = responses
        .get("max_output_tokens")
        .or_else(|| responses.get("max_tokens"))
        .and_then(Value::as_u64)
        .unwrap_or(1024);

    let mut messages = Vec::new();
    if let Some(instructions) = responses.get("instructions").and_then(Value::as_str) {
        messages.push(json!({"role": "system", "content": instructions}));
    }
    append_responses_input(&mut messages, responses.get("input"));

    let normalized = json!({
        "model": model,
        "messages": messages,
        "max_tokens": max_tokens,
        "logprobs": true,
        "stream": false,
    });
    Ok((normalized, wants_stream))
}

fn append_responses_input(messages: &mut Vec<Value>, input: Option<&Value>) {
    match input {
        Some(Value::String(text)) => {
            messages.push(json!({"role": "user", "content": text}));
        }
        Some(Value::Array(items)) => {
            for item in items {
                append_responses_input_item(messages, item);
            }
        }
        _ => {}
    }
}

fn append_responses_input_item(messages: &mut Vec<Value>, item: &Value) {
    if let Some(text) = item.as_str() {
        messages.push(json!({"role": "user", "content": text}));
        return;
    }
    let role = item.get("role").and_then(Value::as_str).unwrap_or("user");
    let content = match item.get("content") {
        Some(Value::String(text)) => json!(text),
        Some(Value::Array(blocks)) => {
            let text: String = blocks
                .iter()
                .filter_map(|block| {
                    block
                        .get("text")
                        .and_then(Value::as_str)
                        .or_else(|| block.get("input_text").and_then(Value::as_str))
                })
                .collect::<Vec<_>>()
                .join("");
            json!(text)
        }
        Some(other) => other.clone(),
        None => json!(""),
    };
    messages.push(json!({"role": role, "content": content}));
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
    let wants_stream = google
        .get("stream")
        .and_then(Value::as_bool)
        .unwrap_or(false);

    let mut messages = Vec::new();
    if let Some(contents) = google.get("contents").and_then(Value::as_array) {
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
        return Ok((synthetic_sse(provider, upstream_body)?, "text/event-stream"));
    }

    match provider {
        Provider::OpenAiChat => Ok((upstream_body.to_vec(), "application/json")),
        Provider::OpenAiResponses => Ok((
            openai_responses_from_openai(upstream_body)?.to_vec(),
            "application/json",
        )),
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

fn parse_upstream(upstream_body: &[u8]) -> Result<Value, NormalizeError> {
    serde_json::from_slice(upstream_body).map_err(|_| NormalizeError::InvalidJson)
}

fn upstream_text(parsed: &Value) -> &str {
    parsed
        .pointer("/choices/0/message/content")
        .and_then(Value::as_str)
        .unwrap_or("")
}

fn upstream_id<'a>(parsed: &'a Value, default: &'a str) -> &'a str {
    parsed.get("id").and_then(Value::as_str).unwrap_or(default)
}

fn synthetic_sse(provider: Provider, upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    match provider {
        Provider::OpenAiChat => synthetic_sse_openai_chat(upstream_body),
        Provider::OpenAiResponses => synthetic_sse_openai_responses(upstream_body),
        Provider::AnthropicMessages => synthetic_sse_anthropic(upstream_body),
        Provider::GoogleGenerateContent => synthetic_sse_google(upstream_body),
        Provider::Unknown => Err(NormalizeError::UnsupportedProvider),
    }
}

fn synthetic_sse_openai_chat(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let content = upstream_text(&parsed);
    let id = upstream_id(&parsed, "chatcmpl-synthetic");

    let chunk = json!({
        "id": id,
        "object": "chat.completion.chunk",
        "choices": [{
            "index": 0,
            "delta": {"content": content},
            "finish_reason": parsed.pointer("/choices/0/finish_reason"),
        }]
    });
    Ok(format_sse_data(&[chunk], true))
}

fn synthetic_sse_openai_responses(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let content = upstream_text(&parsed);
    let id = upstream_id(&parsed, "resp-synthetic");
    let model = parsed.get("model").cloned().unwrap_or(json!(""));

    let created = json!({
        "type": "response.created",
        "response": {"id": id, "object": "response", "status": "in_progress", "model": model}
    });
    let delta = json!({
        "type": "response.output_text.delta",
        "item_id": format!("{id}_msg"),
        "output_index": 0,
        "content_index": 0,
        "delta": content
    });
    let completed = json!({
        "type": "response.completed",
        "response": openai_responses_value(&parsed, id)?
    });
    Ok(format_sse_events(&[
        ("response.created", &created),
        ("response.output_text.delta", &delta),
        ("response.completed", &completed),
    ]))
}

fn synthetic_sse_anthropic(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let content = upstream_text(&parsed);
    let id = upstream_id(&parsed, "msg-synthetic");
    let model = parsed.get("model").cloned().unwrap_or(json!(""));

    let message_start = json!({
        "type": "message_start",
        "message": {
            "id": id,
            "type": "message",
            "role": "assistant",
            "content": [],
            "model": model,
            "stop_reason": null,
            "stop_sequence": null,
            "usage": {"input_tokens": 0, "output_tokens": 0}
        }
    });
    let block_delta = json!({
        "type": "content_block_delta",
        "index": 0,
        "delta": {"type": "text_delta", "text": content}
    });
    let message_delta = json!({
        "type": "message_delta",
        "delta": {"stop_reason": "end_turn", "stop_sequence": null},
        "usage": {"output_tokens": 1}
    });
    Ok(format_sse_events(&[
        ("message_start", &message_start),
        ("content_block_delta", &block_delta),
        ("message_delta", &message_delta),
        ("message_stop", &json!({"type": "message_stop"})),
    ]))
}

fn synthetic_sse_google(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let content = upstream_text(&parsed);
    let chunk = json!({
        "candidates": [{
            "content": {"parts": [{"text": content}], "role": "model"},
            "finishReason": parsed.pointer("/choices/0/finish_reason").cloned().unwrap_or(json!("STOP")),
        }]
    });
    Ok(format_sse_data(&[chunk], true))
}

fn format_sse_data(chunks: &[Value], include_done: bool) -> Vec<u8> {
    let mut out = String::new();
    for chunk in chunks {
        out.push_str("data: ");
        out.push_str(&serde_json::to_string(chunk).unwrap_or_else(|_| "{}".to_string()));
        out.push_str("\n\n");
    }
    if include_done {
        out.push_str("data: [DONE]\n\n");
    }
    out.into_bytes()
}

fn format_sse_events(events: &[(&str, &Value)]) -> Vec<u8> {
    let mut out = String::new();
    for (event, payload) in events {
        out.push_str("event: ");
        out.push_str(event);
        out.push('\n');
        out.push_str("data: ");
        out.push_str(&serde_json::to_string(payload).unwrap_or_else(|_| "{}".to_string()));
        out.push_str("\n\n");
    }
    out.into_bytes()
}

fn openai_responses_value(parsed: &Value, id: &str) -> Result<Value, NormalizeError> {
    let text = upstream_text(parsed);
    Ok(json!({
        "id": id,
        "object": "response",
        "status": "completed",
        "model": parsed.get("model").cloned().unwrap_or(json!("")),
        "output": [{
            "type": "message",
            "id": format!("{id}_msg"),
            "role": "assistant",
            "status": "completed",
            "content": [{
                "type": "output_text",
                "text": text,
                "annotations": []
            }]
        }],
        "usage": parsed.get("usage").cloned().unwrap_or(json!({})),
    }))
}

fn openai_responses_from_openai(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let id = upstream_id(&parsed, "resp-synthetic");
    let out = openai_responses_value(&parsed, id)?;
    serde_json::to_vec(&out).map_err(|_| NormalizeError::InvalidJson)
}

fn anthropic_from_openai(upstream_body: &[u8]) -> Result<Vec<u8>, NormalizeError> {
    let parsed = parse_upstream(upstream_body)?;
    let text = upstream_text(&parsed);
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
    let parsed = parse_upstream(upstream_body)?;
    let text = upstream_text(&parsed);

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
    let parsed = parse_upstream(upstream_body)?;

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

    const UPSTREAM: &[u8] = br#"{"id":"x","model":"gpt-4","choices":[{"message":{"content":"hello"},"finish_reason":"stop","logprobs":{"content":[{"token_id":9,"logprob":-0.2}]}}],"usage":{"prompt_tokens":2}}"#;

    #[test]
    fn detect_openai_path() {
        assert_eq!(
            detect_provider("/v1/chat/completions"),
            Provider::OpenAiChat
        );
    }

    #[test]
    fn detect_openai_responses_path() {
        assert_eq!(detect_provider("/v1/responses"), Provider::OpenAiResponses);
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
    fn openai_responses_normalize_maps_input_and_instructions() {
        let body = br#"{"model":"gpt-4o","instructions":"sys","input":"hi","stream":true,"max_output_tokens":128}"#;
        let (normalized, wants_stream) =
            normalize_request(Provider::OpenAiResponses, body).expect("normalize");
        assert!(wants_stream);
        assert_eq!(normalized["messages"][0]["role"], "system");
        assert_eq!(normalized["messages"][0]["content"], "sys");
        assert_eq!(normalized["messages"][1]["role"], "user");
        assert_eq!(normalized["messages"][1]["content"], "hi");
        assert_eq!(normalized["max_tokens"], json!(128));
        assert_eq!(normalized["logprobs"], json!(true));
    }

    #[test]
    fn openai_responses_denormalize_maps_output_array() {
        let (body, content_type) =
            denormalize_response(Provider::OpenAiResponses, UPSTREAM, false).expect("denorm");
        assert_eq!(content_type, "application/json");
        let parsed: Value = serde_json::from_slice(&body).expect("json");
        assert_eq!(parsed["object"], "response");
        assert_eq!(parsed["output"][0]["content"][0]["text"], "hello");
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
    fn synthetic_sse_openai_chat_format() {
        let (body, content_type) =
            denormalize_response(Provider::OpenAiChat, UPSTREAM, true).expect("denorm");
        assert_eq!(content_type, "text/event-stream");
        let text = String::from_utf8(body).expect("utf8");
        assert!(text.contains("chat.completion.chunk"));
        assert!(text.contains("[DONE]"));
        assert!(text.contains("hello"));
    }

    #[test]
    fn synthetic_sse_openai_responses_format() {
        let (body, _) =
            denormalize_response(Provider::OpenAiResponses, UPSTREAM, true).expect("denorm");
        let text = String::from_utf8(body).expect("utf8");
        assert!(text.contains("event: response.output_text.delta"));
        assert!(text.contains("response.completed"));
        assert!(text.contains("hello"));
    }

    #[test]
    fn synthetic_sse_anthropic_format() {
        let (body, _) =
            denormalize_response(Provider::AnthropicMessages, UPSTREAM, true).expect("denorm");
        let text = String::from_utf8(body).expect("utf8");
        assert!(text.contains("event: content_block_delta"));
        assert!(text.contains("text_delta"));
        assert!(text.contains("hello"));
    }

    #[test]
    fn synthetic_sse_google_format() {
        let (body, _) =
            denormalize_response(Provider::GoogleGenerateContent, UPSTREAM, true).expect("denorm");
        let text = String::from_utf8(body).expect("utf8");
        assert!(text.contains("data:"));
        assert!(text.contains("candidates"));
        assert!(text.contains("hello"));
    }

    #[test]
    fn extract_capture_reads_logprobs() {
        let record =
            extract_capture_fields("req", Provider::OpenAiChat, UPSTREAM, 1).expect("extract");
        assert_eq!(record.request_id, "req");
        assert_eq!(record.sampled_token_ids, vec![9]);
        assert_eq!(record.logprobs, vec![-0.2]);
        assert_eq!(record.prompt_token_ids.len(), 2);
    }

    #[test]
    fn extract_capture_for_responses_provider() {
        let record = extract_capture_fields("req-resp", Provider::OpenAiResponses, UPSTREAM, 2)
            .expect("extract");
        assert_eq!(record.provider, Provider::OpenAiResponses);
        assert_eq!(record.sampled_token_ids, vec![9]);
    }
}
