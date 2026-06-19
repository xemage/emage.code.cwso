//! Completion record and capture queue for the rollout proxy (ADR-010 §3, T132).

use crossbeam_channel::{Receiver, Sender, TrySendError};
use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

use crate::provider::Provider;

/// CompletionRecord is the canonical capture artifact forwarded to the trajectory builder (T133).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CompletionRecord {
    pub request_id: String,
    pub provider: Provider,
    pub prompt_token_ids: Vec<u32>,
    pub sampled_token_ids: Vec<u32>,
    pub logprobs: Vec<f64>,
    pub finish_reason: Option<String>,
    pub timestamp_ns: u64,
}

/// CaptureStore enqueues completion records without blocking the proxy hot path.
#[derive(Debug)]
pub struct CaptureStore {
    sender: Sender<CompletionRecord>,
    receiver: Receiver<CompletionRecord>,
    dropped: AtomicU64,
    store_sender: Option<Sender<CompletionRecord>>,
    store_dropped: Option<Arc<AtomicU64>>,
}

impl CaptureStore {
    pub fn new(capacity: usize) -> Self {
        let (sender, receiver) = crossbeam_channel::bounded(capacity);
        Self {
            sender,
            receiver,
            dropped: AtomicU64::new(0),
            store_sender: None,
            store_dropped: None,
        }
    }

    pub fn with_store_fanout(
        mut self,
        store_sender: Sender<CompletionRecord>,
        store_dropped: Arc<AtomicU64>,
    ) -> Self {
        self.store_sender = Some(store_sender);
        self.store_dropped = Some(store_dropped);
        self
    }

    pub fn try_enqueue(&self, record: CompletionRecord) -> bool {
        if let (Some(store_sender), Some(store_dropped)) = (&self.store_sender, &self.store_dropped)
        {
            match store_sender.try_send(record.clone()) {
                Ok(()) => {}
                Err(TrySendError::Full(_)) => {
                    store_dropped.fetch_add(1, Ordering::Relaxed);
                }
                Err(TrySendError::Disconnected(_)) => {}
            }
        }
        match self.sender.try_send(record) {
            Ok(()) => true,
            Err(TrySendError::Full(_)) => {
                self.dropped.fetch_add(1, Ordering::Relaxed);
                false
            }
            Err(TrySendError::Disconnected(_)) => false,
        }
    }

    pub fn dropped_count(&self) -> u64 {
        self.dropped.load(Ordering::Relaxed)
    }

    pub fn pending_count(&self) -> usize {
        self.receiver.len()
    }

    pub fn try_drain_one(&self) -> Option<CompletionRecord> {
        self.receiver.try_recv().ok()
    }

    pub fn receiver(&self) -> &Receiver<CompletionRecord> {
        &self.receiver
    }
}

pub type SharedCaptureStore = Arc<CaptureStore>;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn enqueue_and_drain_round_trip() {
        let store = CaptureStore::new(8);
        let record = CompletionRecord {
            request_id: "req-1".to_string(),
            provider: Provider::OpenAiChat,
            prompt_token_ids: vec![1, 2],
            sampled_token_ids: vec![3],
            logprobs: vec![-0.1],
            finish_reason: Some("stop".to_string()),
            timestamp_ns: 42,
        };
        assert!(store.try_enqueue(record.clone()));
        assert_eq!(store.try_drain_one(), Some(record));
    }

    #[test]
    fn saturated_queue_increments_drop_counter() {
        let store = CaptureStore::new(1);
        let base = CompletionRecord {
            request_id: "req".to_string(),
            provider: Provider::OpenAiChat,
            prompt_token_ids: vec![],
            sampled_token_ids: vec![],
            logprobs: vec![],
            finish_reason: None,
            timestamp_ns: 0,
        };
        assert!(store.try_enqueue(base.clone()));
        assert!(!store.try_enqueue(base));
        assert_eq!(store.dropped_count(), 1);
    }
}
