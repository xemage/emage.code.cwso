//! LRU KV prefix cache for rollout proxy prewarm (T135).

use std::collections::{HashMap, VecDeque};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;

#[derive(Debug, Clone, Copy)]
pub struct PrefixCacheStats {
    pub entries: usize,
    pub hits: u64,
    pub misses: u64,
    pub hit_rate: f64,
}

pub struct PrefixCache {
    max_entries: usize,
    order: Mutex<VecDeque<String>>,
    entries: Mutex<HashMap<String, ()>>,
    hits: AtomicU64,
    misses: AtomicU64,
}

impl PrefixCache {
    pub fn new(max_entries: usize) -> Self {
        let capacity = max_entries.max(1);
        Self {
            max_entries: capacity,
            order: Mutex::new(VecDeque::with_capacity(capacity)),
            entries: Mutex::new(HashMap::with_capacity(capacity)),
            hits: AtomicU64::new(0),
            misses: AtomicU64::new(0),
        }
    }

    pub fn from_env() -> Self {
        let max_entries = std::env::var("CWSO_ROLLOUT_PREFIX_CACHE_SIZE")
            .ok()
            .and_then(|raw| raw.parse().ok())
            .unwrap_or(128);
        Self::new(max_entries)
    }

    pub fn prewarm(&self, prefix_key: &str) -> bool {
        if prefix_key.is_empty() {
            return false;
        }
        let mut entries = self.entries.lock().expect("prefix cache entries");
        let mut order = self.order.lock().expect("prefix cache order");
        if entries.contains_key(prefix_key) {
            self.touch(&mut order, prefix_key);
            self.hits.fetch_add(1, Ordering::Relaxed);
            return true;
        }
        self.misses.fetch_add(1, Ordering::Relaxed);
        while entries.len() >= self.max_entries {
            if let Some(evicted) = order.pop_front() {
                entries.remove(&evicted);
            } else {
                break;
            }
        }
        entries.insert(prefix_key.to_string(), ());
        order.push_back(prefix_key.to_string());
        false
    }

    pub fn stats(&self) -> PrefixCacheStats {
        let entries = self.entries.lock().expect("prefix cache entries").len();
        let hits = self.hits.load(Ordering::Relaxed);
        let misses = self.misses.load(Ordering::Relaxed);
        let total = hits + misses;
        let hit_rate = if total == 0 {
            0.0
        } else {
            hits as f64 / total as f64
        };
        PrefixCacheStats {
            entries,
            hits,
            misses,
            hit_rate,
        }
    }

    fn touch(&self, order: &mut VecDeque<String>, key: &str) {
        if let Some(pos) = order.iter().position(|entry| entry == key) {
            if let Some(existing) = order.remove(pos) {
                order.push_back(existing);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn prewarm_miss_then_hit() {
        let cache = PrefixCache::new(4);
        assert!(!cache.prewarm("abc"));
        assert!(cache.prewarm("abc"));
        let stats = cache.stats();
        assert_eq!(stats.hits, 1);
        assert_eq!(stats.misses, 1);
        assert_eq!(stats.entries, 1);
    }

    #[test]
    fn evicts_oldest_entry() {
        let cache = PrefixCache::new(2);
        assert!(!cache.prewarm("a"));
        assert!(!cache.prewarm("b"));
        assert!(!cache.prewarm("c"));
        let entries = cache.entries.lock().unwrap();
        assert!(!entries.contains_key("a"));
        assert!(entries.contains_key("b"));
        assert!(entries.contains_key("c"));
    }
}
