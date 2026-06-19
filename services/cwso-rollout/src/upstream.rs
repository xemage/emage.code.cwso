//! Upstream HTTP client for forwarding normalized requests (T132).

use bytes::Bytes;
use http_body_util::{BodyExt, Full};
use hyper::body::Incoming;
use hyper::{Method, Request, StatusCode};
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;
use serde_json::Value;
use std::time::Duration;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum UpstreamError {
    #[error("upstream request failed: {0}")]
    Request(String),
    #[error("upstream returned {0}")]
    Status(StatusCode),
}

pub struct UpstreamClient {
    client: Client<HttpConnector, Full<Bytes>>,
    base_url: String,
    api_key: Option<String>,
}

impl UpstreamClient {
    pub fn new(base_url: String, api_key: Option<String>, timeout_ms: u64) -> Self {
        let mut connector = HttpConnector::new();
        connector.set_connect_timeout(Some(Duration::from_millis(timeout_ms)));
        connector.set_nodelay(true);
        let client = Client::builder(TokioExecutor::new()).build(connector);
        Self {
            client,
            base_url,
            api_key,
        }
    }

    pub async fn post_json(&self, url: &str, body: &Value) -> Result<Vec<u8>, UpstreamError> {
        let payload =
            serde_json::to_vec(body).map_err(|error| UpstreamError::Request(error.to_string()))?;
        let uri = url
            .parse::<hyper::Uri>()
            .map_err(|error| UpstreamError::Request(error.to_string()))?;

        let mut request = Request::builder()
            .method(Method::POST)
            .uri(uri)
            .header("content-type", "application/json")
            .body(Full::new(Bytes::from(payload)))
            .map_err(|error| UpstreamError::Request(error.to_string()))?;

        if let Some(token) = &self.api_key {
            request.headers_mut().insert(
                hyper::header::AUTHORIZATION,
                hyper::header::HeaderValue::from_str(&format!("Bearer {token}"))
                    .map_err(|error| UpstreamError::Request(error.to_string()))?,
            );
        }

        let response = self
            .client
            .request(request)
            .await
            .map_err(|error| UpstreamError::Request(error.to_string()))?;

        let status = response.status();
        let body = response
            .into_body()
            .collect()
            .await
            .map_err(|error| UpstreamError::Request(error.to_string()))?
            .to_bytes();

        if !status.is_success() {
            return Err(UpstreamError::Status(status));
        }
        Ok(body.to_vec())
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }
}

#[allow(dead_code)]
fn _incoming_marker(_: Incoming) {}
