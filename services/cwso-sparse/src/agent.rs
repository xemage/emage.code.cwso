//! Ephemeral sparse Wasm micro-agent lifecycle (ADR-008, T122).
//!
//! Resolves a skill-domain slice from the manifest, mmap-pins it, instantiates a wasmtime
//! sandbox module with a memory cap and a tight `{ternary_gemm}` host-call allowlist, and
//! tracks agents until explicitly dropped.

use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Instant;

use sha2::{Digest, Sha256};
use thiserror::Error;
use wasmtime::{Engine, Linker, Module, ResourceLimiter, Store, StoreLimitsBuilder};

use crate::slice::{MappedSlice, SliceError, SliceManifest};

/// Minimal valid WebAssembly module (empty module, version 1). Used when no orchestration
/// module path is configured (tests / CI).
pub const MINIMAL_WASM: &[u8] = b"\x00asm\x01\x00\x00\x00";

const QUANT_TERNARY_158: &str = "1.58-bit";
const DEFAULT_HOST_RAM_CAP_MB: u32 = 4096;

#[derive(Debug, Error)]
pub enum AgentError {
    #[error("agent registry disabled (no slice manifest configured)")]
    Disabled,
    #[error("unknown skill domain {0:?}")]
    UnknownDomain(String),
    #[error("unsupported quantization {0:?} (only {QUANT_TERNARY_158} is supported)")]
    UnsupportedQuantization(String),
    #[error("max_ram_mb must be > 0")]
    InvalidRAM,
    #[error("max_ram_mb {requested} exceeds host cap {cap}")]
    RAMCapExceeded { requested: u32, cap: u32 },
    #[error("unknown agent {0:?}")]
    UnknownAgent(String),
    #[error("slice error: {0}")]
    Slice(#[from] SliceError),
    #[error("wasm error: {0}")]
    Wasm(String),
    #[error("io error: {0}")]
    Io(String),
    #[error("wasm module integrity check failed: expected sha256 {expected}, got {actual}")]
    ModuleIntegrity { expected: String, actual: String },
}

/// Sidecar configuration for agent lifecycle (from environment at boot).
#[derive(Debug, Clone)]
pub struct AgentConfig {
    pub manifest_path: PathBuf,
    pub wasm_module_path: Option<PathBuf>,
    pub wasm_expected_sha256: Option<String>,
    pub host_ram_cap_mb: u32,
}

impl AgentConfig {
    pub fn from_env() -> Result<Option<Self>, AgentError> {
        let manifest = match std::env::var("CWSO_SPARSE_SLICE_MANIFEST") {
            Ok(v) if !v.trim().is_empty() => PathBuf::from(v),
            _ => return Ok(None),
        };
        let wasm_module_path = std::env::var("CWSO_SPARSE_WASM_MODULE_PATH")
            .ok()
            .filter(|s| !s.trim().is_empty())
            .map(PathBuf::from);
        let wasm_expected_sha256 = std::env::var("CWSO_SPARSE_WASM_MODULE_SHA256")
            .ok()
            .map(|s| s.trim().to_lowercase())
            .filter(|s| !s.is_empty());
        if wasm_module_path.is_some() && wasm_expected_sha256.is_none() {
            return Err(AgentError::Wasm(
                "CWSO_SPARSE_WASM_MODULE_SHA256 is required when a wasm module path is set".into(),
            ));
        }
        let host_ram_cap_mb = std::env::var("CWSO_SPARSE_HOST_RAM_CAP_MB")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(DEFAULT_HOST_RAM_CAP_MB);
        Ok(Some(Self {
            manifest_path: manifest,
            wasm_module_path,
            wasm_expected_sha256,
            host_ram_cap_mb,
        }))
    }
}

/// Runtime snapshot returned by `agent_stat` and included in create responses.
#[derive(Debug, Clone)]
pub struct AgentSnapshot {
    pub agent_id: String,
    pub skill_domain: String,
    pub slice_sha256: String,
    pub state: String,
    pub resident_ram_mb: f64,
    pub tokens_per_sec: f64,
    pub cold_start_ms: f64,
    pub target_ast_node: Option<String>,
}

struct AgentInner {
    skill_domain: String,
    slice_sha256: String,
    target_ast_node: Option<String>,
    max_ram_mb: u32,
    cold_start_ms: f64,
    state: String,
    tokens_per_sec: f64,
    slice: MappedSlice,
    _engine: Engine,
    _store: Store<HostState>,
    _instance: wasmtime::Instance,
}

struct HostState {
    limits: wasmtime::StoreLimits,
}

impl ResourceLimiter for HostState {
    fn memory_growing(
        &mut self,
        current: usize,
        desired: usize,
        maximum: Option<usize>,
    ) -> anyhow::Result<bool> {
        self.limits.memory_growing(current, desired, maximum)
    }

    fn table_growing(
        &mut self,
        current: usize,
        desired: usize,
        maximum: Option<usize>,
    ) -> anyhow::Result<bool> {
        self.limits.table_growing(current, desired, maximum)
    }
}

/// Concurrency-safe sparse-agent store owned by the sidecar IPC server.
pub struct AgentRegistry {
    cfg: AgentConfig,
    manifest: SliceManifest,
    engine: Engine,
    wasm_bytes: Vec<u8>,
    agents: Mutex<HashMap<String, Arc<AgentInner>>>,
    next_id: Mutex<u64>,
}

impl AgentRegistry {
    pub fn new(cfg: AgentConfig) -> Result<Self, AgentError> {
        let manifest = SliceManifest::from_json_file(&cfg.manifest_path)?;
        let wasm_bytes = load_wasm_module(&cfg)?;
        let engine = Engine::default();
        Ok(Self {
            cfg,
            manifest,
            engine,
            wasm_bytes,
            agents: Mutex::new(HashMap::new()),
            next_id: Mutex::new(0),
        })
    }

    pub fn create_agent(
        &self,
        skill_domain: &str,
        quantization: &str,
        max_ram_mb: u32,
        target_ast_node: Option<String>,
    ) -> Result<AgentSnapshot, AgentError> {
        validate_create(
            skill_domain,
            quantization,
            max_ram_mb,
            self.cfg.host_ram_cap_mb,
        )?;

        let slice = self.manifest.load_slice(skill_domain)?;
        let slice_sha256 = slice.sha256().to_string();
        let started = Instant::now();

        let max_bytes = (max_ram_mb as u64).saturating_mul(1024 * 1024) as usize;
        let limits = StoreLimitsBuilder::new()
            .memory_size(max_bytes)
            .trap_on_grow_failure(true)
            .build();

        let module = Module::new(&self.engine, &self.wasm_bytes)
            .map_err(|e| AgentError::Wasm(e.to_string()))?;

        let mut store = Store::new(&self.engine, HostState { limits });
        store.limiter(|s| s as &mut dyn ResourceLimiter);

        let linker = Linker::new(&self.engine);
        let instance = linker
            .instantiate(&mut store, &module)
            .map_err(|e| AgentError::Wasm(e.to_string()))?;

        let cold_start_ms = started.elapsed().as_secs_f64() * 1000.0;
        let resident_ram_mb = estimate_resident_mb(&slice, max_ram_mb);
        let agent_id = self.next_agent_id();

        let inner = Arc::new(AgentInner {
            skill_domain: skill_domain.to_string(),
            slice_sha256: slice_sha256.clone(),
            target_ast_node: target_ast_node.clone(),
            max_ram_mb,
            cold_start_ms,
            state: "ready".to_string(),
            tokens_per_sec: 0.0,
            slice,
            _engine: self.engine.clone(),
            _store: store,
            _instance: instance,
        });

        self.agents
            .lock()
            .map_err(|_| AgentError::Wasm("agent registry lock poisoned".into()))?
            .insert(agent_id.clone(), inner);

        Ok(AgentSnapshot {
            agent_id,
            skill_domain: skill_domain.to_string(),
            slice_sha256,
            state: "ready".to_string(),
            resident_ram_mb,
            tokens_per_sec: 0.0,
            cold_start_ms,
            target_ast_node,
        })
    }

    pub fn drop_agent(&self, agent_id: &str) -> Result<(), AgentError> {
        let mut guard = self
            .agents
            .lock()
            .map_err(|_| AgentError::Wasm("agent registry lock poisoned".into()))?;
        if guard.remove(agent_id).is_none() {
            return Err(AgentError::UnknownAgent(agent_id.to_string()));
        }
        Ok(())
    }

    pub fn agent_stat(&self, agent_id: &str) -> Result<AgentSnapshot, AgentError> {
        let guard = self
            .agents
            .lock()
            .map_err(|_| AgentError::Wasm("agent registry lock poisoned".into()))?;
        let inner = guard
            .get(agent_id)
            .ok_or_else(|| AgentError::UnknownAgent(agent_id.to_string()))?;
        Ok(AgentSnapshot {
            agent_id: agent_id.to_string(),
            skill_domain: inner.skill_domain.clone(),
            slice_sha256: inner.slice_sha256.clone(),
            state: inner.state.clone(),
            resident_ram_mb: estimate_resident_mb(&inner.slice, inner.max_ram_mb),
            tokens_per_sec: inner.tokens_per_sec,
            cold_start_ms: inner.cold_start_ms,
            target_ast_node: inner.target_ast_node.clone(),
        })
    }

    fn next_agent_id(&self) -> String {
        let mut n = self.next_id.lock().unwrap_or_else(|e| e.into_inner());
        *n += 1;
        format!("sa-{:012x}", *n)
    }
}

fn validate_create(
    _skill_domain: &str,
    quantization: &str,
    max_ram_mb: u32,
    host_cap: u32,
) -> Result<(), AgentError> {
    let q = quantization.trim();
    if q.is_empty() || q == QUANT_TERNARY_158 {
        // default
    } else {
        return Err(AgentError::UnsupportedQuantization(q.to_string()));
    }
    if max_ram_mb == 0 {
        return Err(AgentError::InvalidRAM);
    }
    if max_ram_mb > host_cap {
        return Err(AgentError::RAMCapExceeded {
            requested: max_ram_mb,
            cap: host_cap,
        });
    }
    Ok(())
}

fn load_wasm_module(cfg: &AgentConfig) -> Result<Vec<u8>, AgentError> {
    let bytes = match &cfg.wasm_module_path {
        Some(path) => std::fs::read(path).map_err(|e| AgentError::Io(e.to_string()))?,
        None => MINIMAL_WASM.to_vec(),
    };
    if let Some(expected) = &cfg.wasm_expected_sha256 {
        let mut hasher = Sha256::new();
        hasher.update(&bytes);
        let actual = hex::encode(hasher.finalize());
        if !actual.eq_ignore_ascii_case(expected) {
            return Err(AgentError::ModuleIntegrity {
                expected: expected.clone(),
                actual,
            });
        }
    }
    Ok(bytes)
}

fn estimate_resident_mb(slice: &MappedSlice, max_ram_mb: u32) -> f64 {
    let slice_bytes = slice.header().total_len() as f64;
    // Marginal per-agent RAM ≈ activation budget (capped) + tiny metadata; weight pages are shared.
    let activation_budget = (max_ram_mb as f64).min(64.0);
    (slice_bytes / (1024.0 * 1024.0) * 0.01) + activation_budget * 0.05
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gemm::pack_ternary;
    use crate::slice::{content_hash, serialize};
    use std::io::Write;

    fn write_fixture_manifest(dir: &std::path::Path) -> (String, String) {
        let mut packed = Vec::new();
        for row in [[1i8, 0, 0], [0, 1, 0], [0, 0, 1]] {
            packed.extend(pack_ternary(&row).unwrap());
        }
        let bytes = serialize(3, 3, &[1.0, 1.0, 1.0], &packed).unwrap();
        let sha = content_hash(&bytes);
        let slice_path = dir.join("react-hooks.cwsl");
        std::fs::write(&slice_path, &bytes).unwrap();
        let manifest_json = format!(
            r#"{{"slices":[{{"skill_domain":"react-hooks","path":"react-hooks.cwsl","sha256":"{sha}"}}]}}"#
        );
        let manifest_path = dir.join("manifest.json");
        std::fs::write(&manifest_path, manifest_json).unwrap();
        (manifest_path.to_string_lossy().into(), sha)
    }

    #[test]
    fn create_agent_instantiates_and_reports_metrics() {
        let dir = std::env::temp_dir().join(format!("cwsl-agent-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let (manifest_path, sha) = write_fixture_manifest(&dir);

        let cfg = AgentConfig {
            manifest_path: manifest_path.into(),
            wasm_module_path: None,
            wasm_expected_sha256: None,
            host_ram_cap_mb: 512,
        };
        let reg = AgentRegistry::new(cfg).unwrap();
        let snap = reg
            .create_agent("react-hooks", "1.58-bit", 128, Some("Class: Foo".into()))
            .unwrap();
        assert!(snap.agent_id.starts_with("sa-"));
        assert_eq!(snap.skill_domain, "react-hooks");
        assert_eq!(snap.slice_sha256, sha);
        assert_eq!(snap.state, "ready");
        assert!(snap.cold_start_ms >= 0.0);
        assert!(snap.resident_ram_mb > 0.0);

        let stat = reg.agent_stat(&snap.agent_id).unwrap();
        assert_eq!(stat.agent_id, snap.agent_id);

        reg.drop_agent(&snap.agent_id).unwrap();
        assert!(reg.agent_stat(&snap.agent_id).is_err());

        std::fs::remove_dir_all(&dir).ok();
    }

    #[test]
    fn create_rejects_unknown_domain_and_bad_quantization() {
        let dir = std::env::temp_dir().join(format!("cwsl-agent-bad-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let (manifest_path, _) = write_fixture_manifest(&dir);
        let cfg = AgentConfig {
            manifest_path: manifest_path.into(),
            wasm_module_path: None,
            wasm_expected_sha256: None,
            host_ram_cap_mb: 512,
        };
        let reg = AgentRegistry::new(cfg).unwrap();
        assert!(matches!(
            reg.create_agent("missing", "1.58-bit", 64, None),
            Err(AgentError::Slice(_))
        ));
        assert!(matches!(
            reg.create_agent("react-hooks", "int8", 64, None),
            Err(AgentError::UnsupportedQuantization(_))
        ));
        std::fs::remove_dir_all(&dir).ok();
    }
}
