//! `.cwsl` — CWSO Weight Slice container + SHA-256-pinned, mmap'd loader (ADR-008 §5, T121).
//!
//! A skill slice is an unstructured-pruned ternary (`{-1,0,+1}`) weight matrix for one skill
//! domain, packed by the [`crate::gemm`] kernel's 2-bit format plus a per-output-row `f32` scale.
//! On disk it is content-addressed: the file name is its SHA-256 and a manifest pins
//! `skill_domain → sha256`. The loader maps the file **read-only** so the OS shares the resident
//! weight pages across every agent referencing the same slice (the copy-on-write story); only the
//! small scale vector is materialised. Integrity is verified against the pinned hash before use.
//!
//! Layout (little-endian):
//! ```text
//!   offset  size                 field
//!   0       4                    magic "CWSL"
//!   4       2                    format_version (u16)
//!   6       1                    quantization (u8; 0 = ternary 1.58-bit)
//!   7       1                    reserved (u8, 0)
//!   8       4                    n  (u32, output features)
//!   12      4                    k  (u32, input features)
//!   16      4                    scale_count (u32, == n)
//!   20      8                    packed_len (u64, == n * ceil(k/4))
//!   28      scale_count*4        scales (f32)
//!   …       packed_len           packed ternary weights (2-bit, 4/byte)
//! ```

#![allow(dead_code)] // public slice/loader API consumed by the agent lifecycle (T122).

use std::collections::HashMap;
use std::path::{Path, PathBuf};

use memmap2::Mmap;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use thiserror::Error;

use crate::gemm::{packed_row_bytes, GemmError, TernaryView};

pub const CWSL_MAGIC: &[u8; 4] = b"CWSL";
pub const CWSL_VERSION: u16 = 1;
pub const QUANT_TERNARY_1_58: u8 = 0;
pub const HEADER_LEN: usize = 28;

#[derive(Debug, Error)]
pub enum SliceError {
    #[error("io error: {0}")]
    Io(String),
    #[error("file too small: {got} bytes (need at least {need})")]
    Truncated { got: usize, need: usize },
    #[error("bad magic: expected CWSL")]
    BadMagic,
    #[error("unsupported format version {0} (expected {CWSL_VERSION})")]
    UnsupportedVersion(u16),
    #[error("unsupported quantization {0}")]
    UnsupportedQuantization(u8),
    #[error("declared length {declared} != file length {actual}")]
    LengthMismatch { declared: usize, actual: usize },
    #[error("integrity check failed: expected sha256 {expected}, got {actual}")]
    IntegrityMismatch { expected: String, actual: String },
    #[error("invalid pinned sha256 {0:?}: must be 64 lowercase hex chars")]
    BadPinnedHash(String),
    #[error("weights invalid: {0}")]
    Gemm(#[from] GemmError),
    #[error("manifest error: {0}")]
    Manifest(String),
    #[error("unknown skill domain {0:?}")]
    UnknownDomain(String),
}

/// Parsed `.cwsl` header.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct SliceHeader {
    pub format_version: u16,
    pub quantization: u8,
    pub n: usize,
    pub k: usize,
    pub scale_count: usize,
    pub packed_len: usize,
}

impl SliceHeader {
    /// Total on-disk size implied by this header.
    pub fn total_len(&self) -> usize {
        HEADER_LEN + self.scale_count * 4 + self.packed_len
    }

    fn parse(bytes: &[u8]) -> Result<Self, SliceError> {
        if bytes.len() < HEADER_LEN {
            return Err(SliceError::Truncated {
                got: bytes.len(),
                need: HEADER_LEN,
            });
        }
        if &bytes[0..4] != CWSL_MAGIC {
            return Err(SliceError::BadMagic);
        }
        let format_version = u16::from_le_bytes([bytes[4], bytes[5]]);
        if format_version != CWSL_VERSION {
            return Err(SliceError::UnsupportedVersion(format_version));
        }
        let quantization = bytes[6];
        if quantization != QUANT_TERNARY_1_58 {
            return Err(SliceError::UnsupportedQuantization(quantization));
        }
        let n = u32::from_le_bytes([bytes[8], bytes[9], bytes[10], bytes[11]]) as usize;
        let k = u32::from_le_bytes([bytes[12], bytes[13], bytes[14], bytes[15]]) as usize;
        let scale_count = u32::from_le_bytes([bytes[16], bytes[17], bytes[18], bytes[19]]) as usize;
        let packed_len = u64::from_le_bytes([
            bytes[20], bytes[21], bytes[22], bytes[23], bytes[24], bytes[25], bytes[26], bytes[27],
        ]) as usize;
        Ok(Self {
            format_version,
            quantization,
            n,
            k,
            scale_count,
            packed_len,
        })
    }
}

/// Serialize a ternary slice into the `.cwsl` byte container. `packed` must already be the
/// kernel's 2-bit packing of an `[n, k]` matrix (`n * ceil(k/4)` bytes); `scales` has length `n`.
pub fn serialize(n: usize, k: usize, scales: &[f32], packed: &[u8]) -> Result<Vec<u8>, SliceError> {
    // Validate by constructing a view (reuses the kernel's invariants).
    TernaryView::new(n, k, scales, packed)?;
    let n_u32 = u32::try_from(n).map_err(|_| SliceError::Manifest("n exceeds u32".into()))?;
    let k_u32 = u32::try_from(k).map_err(|_| SliceError::Manifest("k exceeds u32".into()))?;

    let mut out = Vec::with_capacity(HEADER_LEN + scales.len() * 4 + packed.len());
    out.extend_from_slice(CWSL_MAGIC);
    out.extend_from_slice(&CWSL_VERSION.to_le_bytes());
    out.push(QUANT_TERNARY_1_58);
    out.push(0); // reserved
    out.extend_from_slice(&n_u32.to_le_bytes());
    out.extend_from_slice(&k_u32.to_le_bytes());
    out.extend_from_slice(&n_u32.to_le_bytes()); // scale_count == n
    out.extend_from_slice(&(packed.len() as u64).to_le_bytes());
    for scale in scales {
        out.extend_from_slice(&scale.to_le_bytes());
    }
    out.extend_from_slice(packed);
    Ok(out)
}

/// SHA-256 content hash of a `.cwsl` byte buffer, as lowercase hex (the content address).
pub fn content_hash(bytes: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    hex::encode(hasher.finalize())
}

/// A SHA-256-pinned, memory-mapped `.cwsl` slice. The large packed-weight region stays resident
/// in the read-only mmap (shared across agents); the small scale vector is materialised once.
#[derive(Debug)]
pub struct MappedSlice {
    _mmap: Mmap,
    header: SliceHeader,
    sha256: String,
    scales: Vec<f32>,
    packed_offset: usize,
}

impl MappedSlice {
    /// Open and verify a slice file. `expected_sha256` is the pinned content hash (64 lowercase
    /// hex chars); the file is rejected unless its bytes hash to exactly that value.
    pub fn open(path: impl AsRef<Path>, expected_sha256: &str) -> Result<Self, SliceError> {
        if !is_sha256_hex(expected_sha256) {
            return Err(SliceError::BadPinnedHash(expected_sha256.to_string()));
        }
        let file = std::fs::File::open(path.as_ref()).map_err(|e| SliceError::Io(e.to_string()))?;
        // SAFETY: read-only mapping; the slice is treated as immutable bytes and never written.
        let mmap = unsafe { Mmap::map(&file) }.map_err(|e| SliceError::Io(e.to_string()))?;

        let header = SliceHeader::parse(&mmap)?;
        let declared = header.total_len();
        if declared != mmap.len() {
            return Err(SliceError::LengthMismatch {
                declared,
                actual: mmap.len(),
            });
        }

        let actual = content_hash(&mmap);
        if !actual.eq_ignore_ascii_case(expected_sha256) {
            return Err(SliceError::IntegrityMismatch {
                expected: expected_sha256.to_string(),
                actual,
            });
        }

        // Validate dimensions against the kernel contract before trusting the body.
        let scales_end = HEADER_LEN + header.scale_count * 4;
        let expected_packed = header
            .n
            .checked_mul(packed_row_bytes(header.k))
            .ok_or_else(|| SliceError::Manifest("n * row_bytes overflow".into()))?;
        if header.scale_count != header.n || header.packed_len != expected_packed {
            return Err(SliceError::Gemm(GemmError::Dimension(format!(
                "header dims inconsistent (n={}, k={}, scale_count={}, packed_len={})",
                header.n, header.k, header.scale_count, header.packed_len
            ))));
        }

        // Materialise the small scale vector; leave packed weights in the shared mmap.
        let mut scales = Vec::with_capacity(header.scale_count);
        for chunk in mmap[HEADER_LEN..scales_end].chunks_exact(4) {
            scales.push(f32::from_le_bytes([chunk[0], chunk[1], chunk[2], chunk[3]]));
        }

        Ok(Self {
            _mmap: mmap,
            header,
            sha256: actual,
            scales,
            packed_offset: scales_end,
        })
    }

    pub fn header(&self) -> &SliceHeader {
        &self.header
    }

    pub fn sha256(&self) -> &str {
        &self.sha256
    }

    /// Zero-copy kernel view: scales from the materialised vector, packed weights borrowed
    /// directly from the resident mmap.
    pub fn view(&self) -> TernaryView<'_> {
        let packed = &self._mmap[self.packed_offset..self.packed_offset + self.header.packed_len];
        TernaryView::new_unchecked(self.header.n, self.header.k, &self.scales, packed)
    }
}

fn is_sha256_hex(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|b| b.is_ascii_hexdigit())
}

// --- Manifest -------------------------------------------------------------------------------

#[derive(Debug, Clone, Deserialize)]
pub struct ManifestEntry {
    pub skill_domain: String,
    pub path: String,
    pub sha256: String,
}

#[derive(Debug, Clone, Deserialize)]
struct ManifestFile {
    slices: Vec<ManifestEntry>,
}

/// Maps `skill_domain → (slice path, pinned sha256)`. Paths are resolved relative to the
/// manifest's directory.
#[derive(Debug, Clone)]
pub struct SliceManifest {
    base_dir: PathBuf,
    entries: HashMap<String, ManifestEntry>,
}

impl SliceManifest {
    pub fn from_json_file(path: impl AsRef<Path>) -> Result<Self, SliceError> {
        let path = path.as_ref();
        let raw = std::fs::read_to_string(path).map_err(|e| SliceError::Io(e.to_string()))?;
        let parsed: ManifestFile =
            serde_json::from_str(&raw).map_err(|e| SliceError::Manifest(e.to_string()))?;
        let base_dir = path
            .parent()
            .map(Path::to_path_buf)
            .unwrap_or_else(|| PathBuf::from("."));
        let mut entries = HashMap::new();
        for entry in parsed.slices {
            if !is_sha256_hex(&entry.sha256) {
                return Err(SliceError::BadPinnedHash(entry.sha256));
            }
            entries.insert(entry.skill_domain.clone(), entry);
        }
        Ok(Self { base_dir, entries })
    }

    pub fn domains(&self) -> Vec<&str> {
        let mut out: Vec<&str> = self.entries.keys().map(String::as_str).collect();
        out.sort_unstable();
        out
    }

    pub fn get(&self, skill_domain: &str) -> Option<&ManifestEntry> {
        self.entries.get(skill_domain)
    }

    /// Resolve, open, and integrity-verify the slice for a skill domain.
    pub fn load_slice(&self, skill_domain: &str) -> Result<MappedSlice, SliceError> {
        let entry = self
            .get(skill_domain)
            .ok_or_else(|| SliceError::UnknownDomain(skill_domain.to_string()))?;
        let path = self.base_dir.join(&entry.path);
        MappedSlice::open(path, &entry.sha256)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gemm::pack_ternary;
    use std::io::Write;

    fn sample_slice() -> (usize, usize, Vec<f32>, Vec<u8>) {
        // 3x3 ternary identity → gemm returns the scaled input.
        let mut packed = Vec::new();
        for row in [[1i8, 0, 0], [0, 1, 0], [0, 0, 1]] {
            packed.extend(pack_ternary(&row).unwrap());
        }
        (3, 3, vec![1.0, 2.0, 0.5], packed)
    }

    fn write_temp(bytes: &[u8], name: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!("cwsl-{}-{name}", std::process::id()));
        let mut f = std::fs::File::create(&path).unwrap();
        f.write_all(bytes).unwrap();
        f.flush().unwrap();
        path
    }

    #[test]
    fn serialize_parse_roundtrip() {
        let (n, k, scales, packed) = sample_slice();
        let bytes = serialize(n, k, &scales, &packed).unwrap();
        let header = SliceHeader::parse(&bytes).unwrap();
        assert_eq!(header.n, 3);
        assert_eq!(header.k, 3);
        assert_eq!(header.scale_count, 3);
        assert_eq!(header.packed_len, packed.len());
        assert_eq!(header.total_len(), bytes.len());
    }

    #[test]
    fn open_verifies_hash_and_runs_gemm_from_mmap() {
        let (n, k, scales, packed) = sample_slice();
        let bytes = serialize(n, k, &scales, &packed).unwrap();
        let sha = content_hash(&bytes);
        let path = write_temp(&bytes, "ok.cwsl");

        let slice = MappedSlice::open(&path, &sha).unwrap();
        assert_eq!(slice.sha256(), sha);
        assert_eq!(slice.header().n, 3);
        let out = slice.view().gemm(&[4.0, 5.0, 6.0], 1).unwrap();
        assert_eq!(out, vec![4.0, 10.0, 3.0]); // scaled identity: [1*4, 2*5, 0.5*6]
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn open_rejects_integrity_mismatch() {
        let (n, k, scales, packed) = sample_slice();
        let bytes = serialize(n, k, &scales, &packed).unwrap();
        let wrong = "0".repeat(64);
        let path = write_temp(&bytes, "badhash.cwsl");
        let err = MappedSlice::open(&path, &wrong).unwrap_err();
        assert!(matches!(err, SliceError::IntegrityMismatch { .. }));
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn open_rejects_non_hex_pin() {
        let path = write_temp(b"whatever", "pin.cwsl");
        let err = MappedSlice::open(&path, "not-a-hash").unwrap_err();
        assert!(matches!(err, SliceError::BadPinnedHash(_)));
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn open_rejects_truncated_file() {
        let bytes = vec![b'C', b'W', b'S', b'L', 1, 0];
        let sha = content_hash(&bytes);
        let path = write_temp(&bytes, "trunc.cwsl");
        let err = MappedSlice::open(&path, &sha).unwrap_err();
        assert!(matches!(err, SliceError::Truncated { .. }));
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn parse_rejects_bad_magic() {
        let mut bytes = vec![0u8; HEADER_LEN];
        bytes[0..4].copy_from_slice(b"XXXX");
        assert!(matches!(
            SliceHeader::parse(&bytes),
            Err(SliceError::BadMagic)
        ));
    }

    #[test]
    fn parse_rejects_unsupported_version() {
        let (n, k, scales, packed) = sample_slice();
        let mut bytes = serialize(n, k, &scales, &packed).unwrap();
        bytes[4] = 9; // bump version to 9
        assert!(matches!(
            SliceHeader::parse(&bytes),
            Err(SliceError::UnsupportedVersion(9))
        ));
    }

    #[test]
    fn open_rejects_length_mismatch() {
        let (n, k, scales, packed) = sample_slice();
        let mut bytes = serialize(n, k, &scales, &packed).unwrap();
        bytes.push(0xFF); // extra trailing byte → declared != actual
        let sha = content_hash(&bytes);
        let path = write_temp(&bytes, "len.cwsl");
        let err = MappedSlice::open(&path, &sha).unwrap_err();
        assert!(matches!(err, SliceError::LengthMismatch { .. }));
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn manifest_loads_and_resolves_slice() {
        let (n, k, scales, packed) = sample_slice();
        let bytes = serialize(n, k, &scales, &packed).unwrap();
        let sha = content_hash(&bytes);

        let dir = std::env::temp_dir().join(format!("cwsl-manifest-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let slice_path = dir.join("react-hooks.cwsl");
        std::fs::write(&slice_path, &bytes).unwrap();
        let manifest_json = format!(
            r#"{{"slices":[{{"skill_domain":"react-hooks","path":"react-hooks.cwsl","sha256":"{sha}"}}]}}"#
        );
        let manifest_path = dir.join("manifest.json");
        std::fs::write(&manifest_path, manifest_json).unwrap();

        let manifest = SliceManifest::from_json_file(&manifest_path).unwrap();
        assert_eq!(manifest.domains(), vec!["react-hooks"]);
        let slice = manifest.load_slice("react-hooks").unwrap();
        assert_eq!(slice.header().n, 3);
        assert!(manifest.load_slice("missing").is_err());

        std::fs::remove_dir_all(&dir).ok();
    }
}
