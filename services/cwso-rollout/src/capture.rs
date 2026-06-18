//! Four-step capture pipeline orchestration (Polar §3.2, T132).

use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::Value;
use thiserror::Error;
use uuid::Uuid;

use crate::config::ProxyConfig;
use crate::prefix_cache::PrefixCache;
use crate::provider::{
    denormalize_response, detect_provider, extract_capture_fields, normalize_request,
    NormalizeError, Provider,
};
use crate::record::SharedCaptureStore;
use crate::upstream::{UpstreamClient, UpstreamError};

#[derive(Debug, Error)]
pub enum CaptureError {
    #[error("unsupported provider route")]
    UnsupportedProvider,
    #[error("normalize failed: {0}")]
    Normalize(#[from] NormalizeError),
    #[error("upstream failed: {0}")]
    Upstream(#[from] UpstreamError),
}

pub struct CapturePipeline {
    config: ProxyConfig,
    store: SharedCaptureStore,
    upstream: UpstreamClient,
    prefix_cache: Arc<PrefixCache>,
}

impl CapturePipeline {
    pub fn new(
        config: ProxyConfig,
        store: SharedCaptureStore,
        prefix_cache: Arc<PrefixCache>,
    ) -> Self {
        let upstream = UpstreamClient::new(
            config.upstream_url.clone(),
            config.upstream_api_key.clone(),
            config.http_timeout_ms,
        );
        Self {
            config,
            store,
            upstream,
            prefix_cache,
        }
    }

    /// handle executes detect → normalize → forward+store → denormalize.
    pub async fn handle(
        &self,
        path: &str,
        body: &[u8],
    ) -> Result<(u16, Vec<u8>, &'static str), CaptureError> {
        let provider = detect_provider(path);
        if provider == Provider::Unknown {
            return Err(CaptureError::UnsupportedProvider);
        }

        let (mut normalized, client_wants_stream) = normalize_request(provider, body)?;
        self.maybe_apply_kv_differential_prompting(&mut normalized);
        let request_id = Uuid::new_v4().to_string();
        let upstream_path = self.config.upstream_path_for(provider);
        let upstream_body = self.upstream.post_json(&upstream_path, &normalized).await?;

        if self.config.capture_enabled {
            let timestamp_ns = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_nanos() as u64)
                .unwrap_or(0);
            if let Ok(record) =
                extract_capture_fields(&request_id, provider, &upstream_body, timestamp_ns)
            {
                let _ = self.store.try_enqueue(record);
            }
        }

        let (response_body, content_type) =
            denormalize_response(provider, &upstream_body, client_wants_stream)?;
        Ok((200, response_body, content_type))
    }

    pub fn store(&self) -> &SharedCaptureStore {
        &self.store
    }

    fn maybe_apply_kv_differential_prompting(&self, normalized: &mut Value) {
        if !self.config.kv_differential_prompting_enabled {
            return;
        }
        let Some(obj) = normalized.as_object_mut() else {
            return;
        };
        let Some(prefix_key) = obj
            .get("prefix_key")
            .and_then(Value::as_str)
            .map(str::to_string)
        else {
            return;
        };

        // Differential prompting applies only when the prefix key already exists in the LRU.
        let cache_hit = self.prefix_cache.prewarm(&prefix_key);
        if !cache_hit {
            return;
        }

        let prefix_token_count = obj
            .get("cached_prefix_token_count")
            .and_then(Value::as_u64)
            .unwrap_or(0) as usize;

        if prefix_token_count > 0 {
            if let Some(prompt_tokens) = obj
                .get_mut("prompt_token_ids")
                .and_then(Value::as_array_mut)
            {
                let drain = prefix_token_count.min(prompt_tokens.len());
                prompt_tokens.drain(0..drain);
            }
        }

        obj.insert("cache_salt".to_string(), Value::String(prefix_key));
        obj.remove("prefix_key");
        obj.remove("cached_prefix_token_count");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::ProxyConfig;
    use crate::record::CaptureStore;
    use http_body_util::BodyExt;
    use hyper::body::Incoming;
    use hyper::service::service_fn;
    use hyper::{Request, Response};
    use hyper_util::rt::TokioIo;
    use std::sync::Arc;
    use tokio::net::TcpListener;

    async fn spawn_mock_upstream() -> String {
        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            loop {
                let (stream, _) = listener.accept().await.expect("accept");
                let io = TokioIo::new(stream);
                tokio::task::spawn(async move {
                    let service = service_fn(|_req: Request<Incoming>| async {
                        let body = br#"{"id":"mock","choices":[{"message":{"content":"ok"},"finish_reason":"stop","logprobs":{"content":[{"token_id":1,"logprob":-0.01}]}}],"usage":{"prompt_tokens":1}}"#;
                        Ok::<_, hyper::Error>(Response::new(http_body_util::Full::new(
                            bytes::Bytes::from_static(body),
                        )))
                    });
                    let _ = hyper::server::conn::http1::Builder::new()
                        .serve_connection(io, service)
                        .await;
                });
            }
        });
        format!("http://{addr}")
    }

    #[tokio::test]
    async fn pipeline_forwards_and_captures() {
        let upstream = spawn_mock_upstream().await;
        let store = Arc::new(CaptureStore::new(16));
        let config = ProxyConfig {
            http_bind: "127.0.0.1:0".to_string(),
            upstream_url: upstream,
            upstream_api_key: None,
            capture_enabled: true,
            kv_differential_prompting_enabled: false,
            capture_queue_capacity: 16,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline =
            CapturePipeline::new(config, Arc::clone(&store), Arc::new(PrefixCache::new(8)));
        let body = br#"{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}"#;
        let (status, response, content_type) = pipeline
            .handle("/v1/chat/completions", body)
            .await
            .expect("handle");
        assert_eq!(status, 200);
        assert_eq!(content_type, "application/json");
        assert!(String::from_utf8_lossy(&response).contains("ok"));

        let record = store.try_drain_one().expect("captured");
        assert_eq!(record.sampled_token_ids, vec![1]);
    }

    #[tokio::test]
    async fn pipeline_routes_responses_path() {
        let upstream = spawn_mock_upstream().await;
        let store = Arc::new(CaptureStore::new(16));
        let config = ProxyConfig {
            http_bind: "127.0.0.1:0".to_string(),
            upstream_url: upstream,
            upstream_api_key: None,
            capture_enabled: true,
            kv_differential_prompting_enabled: false,
            capture_queue_capacity: 16,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline =
            CapturePipeline::new(config, Arc::clone(&store), Arc::new(PrefixCache::new(8)));
        let body = br#"{"model":"gpt-4o","input":"hi","stream":false}"#;
        let (status, response, content_type) = pipeline
            .handle("/v1/responses", body)
            .await
            .expect("handle");
        assert_eq!(status, 200);
        assert_eq!(content_type, "application/json");
        let text = String::from_utf8(response).expect("utf8");
        assert!(text.contains("\"object\":\"response\""));
        assert!(text.contains("ok"));

        let record = store.try_drain_one().expect("captured");
        assert_eq!(record.provider, Provider::OpenAiResponses);
        assert_eq!(record.sampled_token_ids, vec![1]);
    }

    #[tokio::test]
    async fn differential_prompting_strips_prefix_tokens_on_cache_hit() {
        let observed = Arc::new(tokio::sync::Mutex::new(Vec::<u8>::new()));
        let observed_ref = Arc::clone(&observed);

        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            loop {
                let (stream, _) = listener.accept().await.expect("accept");
                let io = TokioIo::new(stream);
                let observed = Arc::clone(&observed_ref);
                tokio::task::spawn(async move {
                    let service = service_fn(move |req: Request<Incoming>| {
                        let observed = Arc::clone(&observed);
                        async move {
                            let bytes =
                                req.into_body().collect().await.expect("collect").to_bytes();
                            *observed.lock().await = bytes.to_vec();
                            let body = br#"{"id":"mock","choices":[{"message":{"content":"ok"},"finish_reason":"stop","logprobs":{"content":[{"token_id":1,"logprob":-0.01}]}}],"usage":{"prompt_tokens":1}}"#;
                            Ok::<_, hyper::Error>(Response::new(http_body_util::Full::new(
                                bytes::Bytes::from_static(body),
                            )))
                        }
                    });
                    let _ = hyper::server::conn::http1::Builder::new()
                        .serve_connection(io, service)
                        .await;
                });
            }
        });

        let store = Arc::new(CaptureStore::new(16));
        let prefix_cache = Arc::new(PrefixCache::new(8));
        assert!(!prefix_cache.prewarm("hot-prefix"));
        let config = ProxyConfig {
            http_bind: "127.0.0.1:0".to_string(),
            upstream_url: format!("http://{addr}"),
            upstream_api_key: None,
            capture_enabled: true,
            kv_differential_prompting_enabled: true,
            capture_queue_capacity: 16,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline = CapturePipeline::new(config, Arc::clone(&store), Arc::clone(&prefix_cache));
        let body = br#"{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"prompt_token_ids":[10,11,12,13],"prefix_key":"hot-prefix","cached_prefix_token_count":2}"#;

        let _ = pipeline
            .handle("/v1/chat/completions", body)
            .await
            .expect("handle");

        let forwarded = observed.lock().await.clone();
        let parsed: Value = serde_json::from_slice(&forwarded).expect("json");
        assert_eq!(parsed["prompt_token_ids"], serde_json::json!([12, 13]));
        assert_eq!(parsed["cache_salt"], serde_json::json!("hot-prefix"));
        assert!(parsed.get("prefix_key").is_none());
        assert!(parsed.get("cached_prefix_token_count").is_none());
    }
}
