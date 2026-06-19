//! Deterministic 1.58-bit ternary GEMM kernel (BitNet b1.58, weights ∈ {-1, 0, +1}).
//!
//! This is the native compute primitive behind the sparse micro-agent tier (ADR-008): a thin
//! sandboxed orchestration module calls this kernel through a single bounds-checked host-call
//! (`ternary_gemm`). Because weights are ternary, the inner product is pure add/subtract/skip —
//! no multiply — and the reduction runs in a fixed (k-ascending) order so the result is
//! byte-identical across runs for a given `(weights, activations)`.
//!
//! Packing: each weight is a 2-bit code, 4 weights per byte, least-significant pair first:
//!   `00` → 0, `01` → +1, `10` → −1 (`11` is invalid). A row of `k` weights occupies
//!   `ceil(k/4)` bytes. Output `Y[m,n]` for activations `A[m,k]` and weights `W[n,k]` is
//!   `Y[i,j] = scale[j] * Σ_k A[i,k] · W[j,k]`.

use thiserror::Error;

pub const TERNARY_ZERO: u8 = 0b00;
pub const TERNARY_POS: u8 = 0b01;
pub const TERNARY_NEG: u8 = 0b10;

const TERNARY_MASK: u8 = 0b11;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum GemmError {
    #[error("dimension mismatch: {0}")]
    Dimension(String),
    #[error("invalid ternary value {value} at index {index} (expected -1, 0, or +1)")]
    NonTernary { value: i64, index: usize },
    #[error("invalid ternary code {code:#04b} at weight index {index}")]
    Encoding { code: u8, index: usize },
}

/// Number of packed bytes needed for `k` ternary weights (4 per byte).
#[inline]
pub fn packed_row_bytes(k: usize) -> usize {
    k.div_ceil(4)
}

/// Pack ternary values (-1, 0, +1) into 2-bit codes, 4 per byte (LSB-first).
//
// Part of the kernel's public packing API; consumed by tests now and by the `.cwsl` slice
// packer (T121). `allow(dead_code)` keeps the binary build warning-free until then.
#[allow(dead_code)]
pub fn pack_ternary(values: &[i8]) -> Result<Vec<u8>, GemmError> {
    let mut out = vec![0u8; packed_row_bytes(values.len())];
    for (index, &value) in values.iter().enumerate() {
        let code = match value {
            0 => TERNARY_ZERO,
            1 => TERNARY_POS,
            -1 => TERNARY_NEG,
            other => {
                return Err(GemmError::NonTernary {
                    value: other as i64,
                    index,
                })
            }
        };
        out[index / 4] |= code << ((index % 4) * 2);
    }
    Ok(out)
}

#[inline]
fn decode_weight(code: u8, index: usize) -> Result<i32, GemmError> {
    match code {
        TERNARY_ZERO => Ok(0),
        TERNARY_POS => Ok(1),
        TERNARY_NEG => Ok(-1),
        other => Err(GemmError::Encoding { code: other, index }),
    }
}

/// Validate that `scales` (length n) and `packed` (length n·ceil(k/4)) describe an `[n, k]`
/// ternary matrix. Shared by the owning `TernaryWeights` and the borrowed `TernaryView`.
fn validate_dims(
    n: usize,
    k: usize,
    scales_len: usize,
    packed_len: usize,
) -> Result<(), GemmError> {
    if scales_len != n {
        return Err(GemmError::Dimension(format!(
            "scales length {scales_len} != n {n}"
        )));
    }
    let expected = n
        .checked_mul(packed_row_bytes(k))
        .ok_or_else(|| GemmError::Dimension(format!("n {n} * row_bytes overflow for k {k}")))?;
    if packed_len != expected {
        return Err(GemmError::Dimension(format!(
            "packed length {packed_len} != expected {expected} (n={n}, k={k})"
        )));
    }
    Ok(())
}

/// A borrowed view of an `[n, k]` ternary matrix: the scale vector and packed weights live
/// elsewhere (e.g. an mmap'd `.cwsl` slice). This is what makes weight sharing real — the
/// kernel runs directly over the resident, read-only bytes without copying them per agent.
#[derive(Debug, Clone, Copy)]
pub struct TernaryView<'a> {
    n: usize,
    k: usize,
    scales: &'a [f32],
    packed: &'a [u8],
}

impl<'a> TernaryView<'a> {
    /// Construct a view from borrowed slices, validating shape invariants.
    pub fn new(n: usize, k: usize, scales: &'a [f32], packed: &'a [u8]) -> Result<Self, GemmError> {
        validate_dims(n, k, scales.len(), packed.len())?;
        Ok(Self {
            n,
            k,
            scales,
            packed,
        })
    }

    /// Construct without re-validating dimensions. Callers (the owning `TernaryWeights` and the
    /// `.cwsl` loader, which both validate up front) guarantee the invariants.
    pub(crate) fn new_unchecked(n: usize, k: usize, scales: &'a [f32], packed: &'a [u8]) -> Self {
        Self {
            n,
            k,
            scales,
            packed,
        }
    }

    #[allow(dead_code)]
    pub fn n(&self) -> usize {
        self.n
    }

    #[allow(dead_code)]
    pub fn k(&self) -> usize {
        self.k
    }

    /// Compute `Y[m,n] = scale ∘ (A[m,k] · Wᵀ)` for row-major activations `A` of `m` rows.
    /// Deterministic: fixed k-ascending reduction, no parallel float nondeterminism.
    pub fn gemm(&self, activations: &[f32], m: usize) -> Result<Vec<f32>, GemmError> {
        if activations.len() != m * self.k {
            return Err(GemmError::Dimension(format!(
                "activations length {} != m*k {}",
                activations.len(),
                m * self.k
            )));
        }
        let row_bytes = packed_row_bytes(self.k);
        let mut out = vec![0.0f32; m * self.n];
        for i in 0..m {
            let a_row = &activations[i * self.k..i * self.k + self.k];
            for j in 0..self.n {
                let w_row = &self.packed[j * row_bytes..j * row_bytes + row_bytes];
                let mut acc = 0.0f32;
                for (kk, &a) in a_row.iter().enumerate() {
                    let code = (w_row[kk / 4] >> ((kk % 4) * 2)) & TERNARY_MASK;
                    match decode_weight(code, kk)? {
                        1 => acc += a,
                        -1 => acc -= a,
                        _ => {}
                    }
                }
                out[i * self.n + j] = self.scales[j] * acc;
            }
        }
        Ok(out)
    }
}

/// A pruned ternary weight matrix of shape `[n, k]` (output features × input features) with a
/// per-output-row `f32` scale. This is the in-memory form of a packed skill slice (T121 will
/// add the on-disk `.cwsl` container + mmap loader; this kernel is the consumer).
#[derive(Debug, Clone)]
pub struct TernaryWeights {
    n: usize,
    k: usize,
    scales: Vec<f32>,
    packed: Vec<u8>,
}

impl TernaryWeights {
    /// Construct from a pre-packed buffer, validating shape invariants.
    pub fn new(n: usize, k: usize, scales: Vec<f32>, packed: Vec<u8>) -> Result<Self, GemmError> {
        validate_dims(n, k, scales.len(), packed.len())?;
        Ok(Self {
            n,
            k,
            scales,
            packed,
        })
    }

    /// Construct from a dense row-major `[n, k]` slice of ternary values (test/builder helper;
    /// also the basis for the T121 slice packer).
    #[allow(dead_code)]
    pub fn from_dense(
        n: usize,
        k: usize,
        values: &[i8],
        scales: Vec<f32>,
    ) -> Result<Self, GemmError> {
        if values.len() != n * k {
            return Err(GemmError::Dimension(format!(
                "values length {} != n*k {}",
                values.len(),
                n * k
            )));
        }
        let row_bytes = packed_row_bytes(k);
        let mut packed = vec![0u8; n * row_bytes];
        for row in 0..n {
            let row_packed = pack_ternary(&values[row * k..row * k + k])?;
            packed[row * row_bytes..row * row_bytes + row_bytes].copy_from_slice(&row_packed);
        }
        Self::new(n, k, scales, packed)
    }

    #[allow(dead_code)]
    pub fn n(&self) -> usize {
        self.n
    }

    #[allow(dead_code)]
    pub fn k(&self) -> usize {
        self.k
    }

    /// Borrow this matrix as a zero-copy [`TernaryView`].
    pub fn as_view(&self) -> TernaryView<'_> {
        TernaryView::new_unchecked(self.n, self.k, &self.scales, &self.packed)
    }

    /// Compute `Y[m,n] = scale ∘ (A[m,k] · Wᵀ)` for row-major activations `A` of `m` rows.
    pub fn gemm(&self, activations: &[f32], m: usize) -> Result<Vec<f32>, GemmError> {
        self.as_view().gemm(activations, m)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Reference dense matmul over i8 weights — independent of the packed kernel.
    fn reference(
        n: usize,
        k: usize,
        weights: &[i8],
        scales: &[f32],
        a: &[f32],
        m: usize,
    ) -> Vec<f32> {
        let mut out = vec![0.0f32; m * n];
        for i in 0..m {
            for j in 0..n {
                let mut acc = 0.0f32;
                for kk in 0..k {
                    acc += a[i * k + kk] * weights[j * k + kk] as f32;
                }
                out[i * n + j] = scales[j] * acc;
            }
        }
        out
    }

    #[test]
    fn pack_unpack_roundtrip_via_gemm_identity() {
        // 3x3 ternary identity → output is the scaled input row.
        let weights: [i8; 9] = [1, 0, 0, 0, 1, 0, 0, 0, 1];
        let w = TernaryWeights::from_dense(3, 3, &weights, vec![1.0, 1.0, 1.0]).unwrap();
        let out = w.gemm(&[2.0, -3.0, 4.0], 1).unwrap();
        assert_eq!(out, vec![2.0, -3.0, 4.0]);
    }

    #[test]
    fn packs_four_weights_per_byte_lsb_first() {
        // [+1, -1, 0, +1] → 01,10,00,01 packed LSB-first = 0b01_00_10_01 = 0x49.
        let packed = pack_ternary(&[1, -1, 0, 1]).unwrap();
        assert_eq!(packed, vec![0b01_00_10_01]);
        // k=5 needs 2 bytes.
        assert_eq!(packed_row_bytes(5), 2);
    }

    #[test]
    fn rejects_non_ternary_weight() {
        let err = pack_ternary(&[1, 2, 0]).unwrap_err();
        assert_eq!(err, GemmError::NonTernary { value: 2, index: 1 });
    }

    #[test]
    fn gemm_matches_reference_on_random_like_matrix() {
        let n = 4;
        let k = 7; // not a multiple of 4 → exercises tail byte
        let weights: [i8; 28] = [
            1, -1, 0, 1, 0, -1, 1, // row 0
            0, 0, 1, -1, 1, 0, -1, // row 1
            -1, 1, -1, 0, 0, 1, 0, // row 2
            1, 1, 1, -1, -1, -1, 0, // row 3
        ];
        let scales = vec![0.5, 2.0, 1.0, -1.5];
        let a = vec![1.0, 2.0, 3.0, -1.0, 0.5, -2.0, 4.0];
        let w = TernaryWeights::from_dense(n, k, &weights, scales.clone()).unwrap();
        let got = w.gemm(&a, 1).unwrap();
        let want = reference(n, k, &weights, &scales, &a, 1);
        assert_eq!(got, want);
    }

    #[test]
    fn gemm_handles_multiple_activation_rows() {
        let weights: [i8; 6] = [1, -1, 0, 0, 1, 1];
        let scales = vec![1.0, 1.0];
        let a = vec![1.0, 2.0, 3.0, 10.0, 20.0, 30.0]; // m=2, k=3
        let w = TernaryWeights::from_dense(2, 3, &weights, scales).unwrap();
        let got = w.gemm(&a, 2).unwrap();
        // row0: [1*1 + 2*-1 + 3*0, 1*0 + 2*1 + 3*1] = [-1, 5]
        // row1: [10 - 20, 20 + 30] = [-10, 50]
        assert_eq!(got, vec![-1.0, 5.0, -10.0, 50.0]);
    }

    #[test]
    fn gemm_is_deterministic_across_runs() {
        let weights: [i8; 12] = [1, -1, 0, 1, 0, 1, -1, 0, -1, -1, 1, 0];
        let w = TernaryWeights::from_dense(3, 4, &weights, vec![1.25, -0.75, 2.0]).unwrap();
        let a = vec![0.1, 0.2, 0.3, 0.4];
        let first = w.gemm(&a, 1).unwrap();
        for _ in 0..1000 {
            assert_eq!(w.gemm(&a, 1).unwrap(), first);
        }
    }

    #[test]
    fn gemm_rejects_activation_dimension_mismatch() {
        let w = TernaryWeights::from_dense(2, 3, &[1, 0, -1, 0, 1, 0], vec![1.0, 1.0]).unwrap();
        let err = w.gemm(&[1.0, 2.0], 1).unwrap_err();
        assert!(matches!(err, GemmError::Dimension(_)));
    }

    #[test]
    fn new_rejects_bad_scale_length() {
        let err = TernaryWeights::new(2, 4, vec![1.0], vec![0; 2]).unwrap_err();
        assert!(matches!(err, GemmError::Dimension(_)));
    }

    #[test]
    fn new_rejects_bad_packed_length() {
        let err = TernaryWeights::new(2, 4, vec![1.0, 1.0], vec![0; 3]).unwrap_err();
        assert!(matches!(err, GemmError::Dimension(_)));
    }

    #[test]
    fn gemm_detects_invalid_encoding() {
        // Hand-craft a packed buffer containing the invalid 0b11 code.
        let w = TernaryWeights::new(1, 1, vec![1.0], vec![0b11]).unwrap();
        let err = w.gemm(&[1.0], 1).unwrap_err();
        assert_eq!(
            err,
            GemmError::Encoding {
                code: 0b11,
                index: 0
            }
        );
    }
}
