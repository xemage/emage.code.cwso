//! Four-step capture pipeline orchestration (Polar §3.2, T132).

use std::time::{SystemTime, UNIX_EPOCH};

use thiserror::Error;
use uuid::Uuid;

use crate::config::ProxyConfig;
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
}

impl CapturePipeline {
    pub fn new(config: ProxyConfig, store: SharedCaptureStore) -> Self {
        let upstream = UpstreamClient::new(
            config.upstream_url.clone(),
            config.upstream_api_key.clone(),
            config.http_timeout_ms,
        );
        Self {
            config,
            store,
            upstream,
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

        let (normalized, client_wants_stream) = normalize_request(provider, body)?;
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
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::ProxyConfig;
    use crate::record::CaptureStore;
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
            capture_queue_capacity: 16,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline = CapturePipeline::new(config, Arc::clone(&store));
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
            capture_queue_capacity: 16,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline = CapturePipeline::new(config, Arc::clone(&store));
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
}
