//! BLAKE3 digest equality with AVX2 / scalar fallback (ADR-009 T127).
//!
//! `std::simd` remains unstable on Rust 1.86; the hot path uses stable `std::arch::x86_64`
//! AVX2 (`_mm256_cmpeq_epi8`) over the 32-byte digest. When AVX-512 VL stabilizes, batch
//! key-hash lanes can widen without changing `sparse_diff` semantics.

use blake3::Hash;

#[inline]
pub fn hashes_equal(a: Hash, b: Hash) -> bool {
    let ab = a.as_bytes();
    let bb = b.as_bytes();
    debug_assert_eq!(ab.len(), 32);
    #[cfg(target_arch = "x86_64")]
    {
        if std::arch::is_x86_feature_detected!("avx2") {
            return unsafe { compare_32_avx2(ab, bb) };
        }
    }
    ab == bb
}

#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
unsafe fn compare_32_avx2(a: &[u8], b: &[u8]) -> bool {
    use std::arch::x86_64::*;
    let va = _mm256_loadu_si256(a.as_ptr() as *const __m256i);
    let vb = _mm256_loadu_si256(b.as_ptr() as *const __m256i);
    let cmp = _mm256_cmpeq_epi8(va, vb);
    _mm256_movemask_epi8(cmp) as u32 == u32::MAX
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn digest_compare_reflexive_and_distinct() {
        let h1 = Hash::from_bytes(blake3::hash(b"one").into());
        let h2 = Hash::from_bytes(blake3::hash(b"two").into());
        assert!(hashes_equal(h1, h1));
        assert!(!hashes_equal(h1, h2));
    }
}
