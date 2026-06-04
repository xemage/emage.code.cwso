//! Hyper reverse proxy server for model API routes (T132).

use std::convert::Infallible;
use std::sync::Arc;

use bytes::Bytes;
use http_body_util::{combinators::BoxBody, BodyExt, Full};
use hyper::body::Incoming;
use hyper::service::service_fn;
use hyper::{Method, Request, Response, StatusCode};
use hyper_util::rt::TokioIo;
use tokio::net::TcpListener;
use tracing::{info, warn};

use crate::capture::{CaptureError, CapturePipeline};

type BoxResponse = Response<BoxBody<Bytes, hyper::Error>>;

pub async fn serve(bind: &str, pipeline: Arc<CapturePipeline>) -> anyhow::Result<()> {
    let listener = TcpListener::bind(bind).await?;
    info!(%bind, "cwso-rollout proxy listening");

    loop {
        let (stream, _) = listener.accept().await?;
        let pipeline = Arc::clone(&pipeline);
        tokio::spawn(async move {
            let io = TokioIo::new(stream);
            let service = service_fn(move |req| {
                let pipeline = Arc::clone(&pipeline);
                async move { handle_request(req, pipeline).await }
            });
            if let Err(error) = hyper::server::conn::http1::Builder::new()
                .serve_connection(io, service)
                .await
            {
                warn!(error = %error, "proxy connection error");
            }
        });
    }
}

async fn handle_request(
    req: Request<Incoming>,
    pipeline: Arc<CapturePipeline>,
) -> Result<BoxResponse, Infallible> {
    if req.method() != Method::POST {
        return Ok(error_response(
            StatusCode::METHOD_NOT_ALLOWED,
            "only POST is supported",
        ));
    }

    let path = req.uri().path().to_string();
    let body = match req.into_body().collect().await {
        Ok(collected) => collected.to_bytes().to_vec(),
        Err(error) => {
            return Ok(error_response(
                StatusCode::BAD_REQUEST,
                &format!("failed to read body: {error}"),
            ));
        }
    };

    match pipeline.handle(&path, &body).await {
        Ok((status, response_body, content_type)) => Ok(json_response(
            StatusCode::from_u16(status).unwrap_or(StatusCode::OK),
            response_body,
            content_type,
        )),
        Err(CaptureError::UnsupportedProvider) => Ok(error_response(
            StatusCode::NOT_FOUND,
            "unsupported provider route",
        )),
        Err(CaptureError::Normalize(error)) => Ok(error_response(
            StatusCode::BAD_REQUEST,
            &format!("normalize error: {error}"),
        )),
        Err(CaptureError::Upstream(error)) => {
            warn!(error = %error, "upstream failure");
            Ok(error_response(StatusCode::BAD_GATEWAY, &error.to_string()))
        }
    }
}

fn json_response(status: StatusCode, body: Vec<u8>, content_type: &str) -> BoxResponse {
    Response::builder()
        .status(status)
        .header("content-type", content_type)
        .body(full_body(body))
        .unwrap_or_else(|_| {
            error_response(StatusCode::INTERNAL_SERVER_ERROR, "response build failed")
        })
}

fn error_response(status: StatusCode, message: &str) -> BoxResponse {
    let body = format!(r#"{{"error":{{"message":"{message}"}}}}"#);
    Response::builder()
        .status(status)
        .header("content-type", "application/json")
        .body(full_body(body.into_bytes()))
        .unwrap_or_else(|_| {
            Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .body(full_body(Vec::new()))
                .expect("empty fallback response")
        })
}

fn full_body(body: Vec<u8>) -> BoxBody<Bytes, hyper::Error> {
    Full::new(Bytes::from(body))
        .map_err(|never| match never {})
        .boxed()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::ProxyConfig;
    use crate::record::CaptureStore;
    use std::sync::Arc;

    use hyper_util::rt::TokioExecutor;

    async fn spawn_mock_upstream() -> String {
        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            loop {
                let (stream, _) = listener.accept().await.expect("accept");
                let io = TokioIo::new(stream);
                tokio::spawn(async move {
                    let service = service_fn(|_req: Request<Incoming>| async {
                        let body = br#"{"id":"mock","choices":[{"message":{"content":"proxy-ok"},"finish_reason":"stop"}]}"#;
                        Ok::<_, hyper::Error>(Response::new(Full::new(Bytes::from_static(body))))
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
    async fn proxy_endpoint_returns_openai_shape() {
        let upstream = spawn_mock_upstream().await;
        let store = Arc::new(CaptureStore::new(8));
        let config = ProxyConfig {
            http_bind: "127.0.0.1:0".to_string(),
            upstream_url: upstream,
            upstream_api_key: None,
            capture_enabled: false,
            capture_queue_capacity: 8,
            http_timeout_ms: 5_000,
            allow_insecure_endpoints: false,
        };
        let pipeline = Arc::new(CapturePipeline::new(config, store));
        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind proxy");
        let proxy_addr = listener.local_addr().expect("addr");
        let pipeline_task = Arc::clone(&pipeline);
        tokio::spawn(async move {
            loop {
                let (stream, _) = listener.accept().await.expect("accept");
                let io = TokioIo::new(stream);
                let pipeline = Arc::clone(&pipeline_task);
                tokio::spawn(async move {
                    let service = service_fn(move |req| {
                        let pipeline = Arc::clone(&pipeline);
                        async move { handle_request(req, pipeline).await }
                    });
                    let _ = hyper::server::conn::http1::Builder::new()
                        .serve_connection(io, service)
                        .await;
                });
            }
        });

        let client = hyper_util::client::legacy::Client::builder(TokioExecutor::new())
            .build_http::<Full<Bytes>>();
        let body = Bytes::from_static(br#"{"model":"gpt-4","messages":[]}"#);
        let request = Request::builder()
            .method(Method::POST)
            .uri(format!("http://{proxy_addr}/v1/chat/completions"))
            .header("content-type", "application/json")
            .body(Full::new(body))
            .expect("request");
        let response = client.request(request).await.expect("response");
        assert_eq!(response.status(), StatusCode::OK);
        let bytes = response
            .into_body()
            .collect()
            .await
            .expect("body")
            .to_bytes();
        assert!(String::from_utf8_lossy(&bytes).contains("proxy-ok"));
    }
}
