use std::time::Duration;

/// HttpResponse is a transport-agnostic HTTP result (status + raw body).
#[derive(Debug, Clone)]
pub struct HttpResponse {
    pub status: u16,
    pub body: String,
}

/// TransportError categorizes low-level transport failures (no HTTP status reached).
#[derive(Debug, Clone, thiserror::Error)]
pub enum TransportError {
    #[error("request timed out: {0}")]
    Timeout(String),
    #[error("endpoint unreachable: {0}")]
    Unreachable(String),
    #[error("transport error: {0}")]
    Other(String),
}

/// HttpTransport abstracts the HTTP client so adapters can be unit-tested offline with a
/// mock and run against real OpenAI-compatible servers in production.
pub trait HttpTransport: Send + Sync {
    /// post_json sends a JSON body and returns the HTTP response (any status), or a
    /// transport error when no response was received.
    fn post_json(
        &self,
        url: &str,
        bearer: Option<&str>,
        body: &serde_json::Value,
    ) -> Result<HttpResponse, TransportError>;

    /// get performs a GET (used for health probes).
    fn get(&self, url: &str, bearer: Option<&str>) -> Result<HttpResponse, TransportError>;
}

/// UreqTransport is the production blocking HTTP client.
pub struct UreqTransport {
    agent: ureq::Agent,
}

impl UreqTransport {
    pub fn new(timeout: Duration) -> Self {
        let agent = ureq::AgentBuilder::new()
            .timeout(timeout)
            .user_agent("cwso-hal/0.1")
            .build();
        Self { agent }
    }
}

impl HttpTransport for UreqTransport {
    fn post_json(
        &self,
        url: &str,
        bearer: Option<&str>,
        body: &serde_json::Value,
    ) -> Result<HttpResponse, TransportError> {
        let mut req = self.agent.post(url).set("content-type", "application/json");
        if let Some(token) = bearer {
            req = req.set("authorization", &format!("Bearer {token}"));
        }
        classify(req.send_json(body.clone()))
    }

    fn get(&self, url: &str, bearer: Option<&str>) -> Result<HttpResponse, TransportError> {
        let mut req = self.agent.get(url);
        if let Some(token) = bearer {
            req = req.set("authorization", &format!("Bearer {token}"));
        }
        classify(req.call())
    }
}

/// classify normalizes a ureq result into HttpResponse / TransportError. A non-2xx HTTP
/// status is still a response (ureq surfaces it as `Error::Status`), so it is returned as
/// `Ok(HttpResponse)` with the status preserved for adapter-level mapping.
fn classify(result: Result<ureq::Response, ureq::Error>) -> Result<HttpResponse, TransportError> {
    match result {
        Ok(resp) => {
            let status = resp.status();
            let body = resp.into_string().unwrap_or_default();
            Ok(HttpResponse { status, body })
        }
        Err(ureq::Error::Status(code, resp)) => {
            let body = resp.into_string().unwrap_or_default();
            Ok(HttpResponse { status: code, body })
        }
        Err(ureq::Error::Transport(transport)) => {
            let message = transport.to_string();
            let lower = message.to_lowercase();
            if lower.contains("timed out") || lower.contains("timeout") {
                Err(TransportError::Timeout(message))
            } else if lower.contains("dns")
                || lower.contains("connect")
                || lower.contains("refused")
                || lower.contains("resolve")
            {
                Err(TransportError::Unreachable(message))
            } else {
                Err(TransportError::Other(message))
            }
        }
    }
}

#[cfg(test)]
pub mod testing {
    use super::*;
    use std::sync::Mutex;

    /// MockTransport returns canned responses and records requests for assertions.
    pub struct MockTransport {
        pub post_result: Mutex<Vec<Result<HttpResponse, TransportError>>>,
        pub get_result: Mutex<Vec<Result<HttpResponse, TransportError>>>,
        pub last_post_body: Mutex<Option<serde_json::Value>>,
        pub last_bearer: Mutex<Option<String>>,
    }

    impl MockTransport {
        pub fn new() -> Self {
            Self {
                post_result: Mutex::new(Vec::new()),
                get_result: Mutex::new(Vec::new()),
                last_post_body: Mutex::new(None),
                last_bearer: Mutex::new(None),
            }
        }

        pub fn with_post(self, result: Result<HttpResponse, TransportError>) -> Self {
            self.post_result.lock().unwrap().push(result);
            self
        }

        pub fn with_get(self, result: Result<HttpResponse, TransportError>) -> Self {
            self.get_result.lock().unwrap().push(result);
            self
        }

        fn pop(
            queue: &Mutex<Vec<Result<HttpResponse, TransportError>>>,
        ) -> Result<HttpResponse, TransportError> {
            let mut guard = queue.lock().unwrap();
            if guard.is_empty() {
                return Err(TransportError::Other(
                    "mock: no canned response".to_string(),
                ));
            }
            guard.remove(0)
        }
    }

    impl HttpTransport for MockTransport {
        fn post_json(
            &self,
            _url: &str,
            bearer: Option<&str>,
            body: &serde_json::Value,
        ) -> Result<HttpResponse, TransportError> {
            *self.last_post_body.lock().unwrap() = Some(body.clone());
            *self.last_bearer.lock().unwrap() = bearer.map(ToString::to_string);
            Self::pop(&self.post_result)
        }

        fn get(&self, _url: &str, _bearer: Option<&str>) -> Result<HttpResponse, TransportError> {
            Self::pop(&self.get_result)
        }
    }
}
