//! Pure `sparse_diff` kernel (ADR-009). AVX2 accelerates payload-hash equality on x86_64.

use std::collections::BTreeMap;

use blake3::Hash;

use crate::sparse_tensor::SparseAstTensor;

mod simd;

/// Per-key classification for the sparse pre-filter (photonic-ready mask).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum SparseRowClass {
    BothUnchanged,
    OursOnly,
    TheirsOnly,
    BothModified,
    DisjointInsert,
}

pub type SparseDiffMask = BTreeMap<String, SparseRowClass>;

/// Compare sparse tensors against the ordered dense `base_keys` list.
///
/// Keys absent from both sparse tensors are unchanged on both sides relative to `base`.
/// Insertions (rows present in a sparse tensor but not in `base_keys`) are classified separately.
pub fn sparse_diff(
    base_keys: &[String],
    ours: &SparseAstTensor,
    theirs: &SparseAstTensor,
) -> SparseDiffMask {
    let base_set: BTreeMap<&str, ()> = base_keys.iter().map(|k| (k.as_str(), ())).collect();
    let mut mask = BTreeMap::new();

    for key in base_keys {
        let o = ours.row(key);
        let t = theirs.row(key);
        let class = match (o, t) {
            (None, None) => SparseRowClass::BothUnchanged,
            (Some(_), None) => SparseRowClass::OursOnly,
            (None, Some(_)) => SparseRowClass::TheirsOnly,
            (Some(_), Some(_)) => SparseRowClass::BothModified,
        };
        mask.insert(key.clone(), class);
    }

    let mut insertion_keys: Vec<String> = ours
        .rows
        .keys()
        .chain(theirs.rows.keys())
        .filter(|k| !base_set.contains_key(k.as_str()))
        .cloned()
        .collect();
    insertion_keys.sort();
    insertion_keys.dedup();

    for key in insertion_keys {
        let class = match (ours.row(&key), theirs.row(&key)) {
            (Some(_), None) => SparseRowClass::OursOnly,
            (None, Some(_)) => SparseRowClass::TheirsOnly,
            (Some(ours_row), Some(theirs_row)) => {
                if simd::hashes_equal(ours_row.payload_hash, theirs_row.payload_hash) {
                    SparseRowClass::BothModified
                } else {
                    SparseRowClass::DisjointInsert
                }
            }
            (None, None) => continue,
        };
        mask.insert(key, class);
    }

    mask
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;

    use super::*;
    use crate::sparse_tensor::encode_sparse_side;

    fn units(pairs: &[(&str, &str)]) -> BTreeMap<String, Vec<u8>> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.as_bytes().to_vec()))
            .collect()
    }

    #[test]
    fn unchanged_base_keys_marked_both_unchanged() {
        let base = units(&[
            ("function:main", "fn main() {}"),
            ("function:other", "fn other()"),
        ]);
        let ours = encode_sparse_side(&base, &base);
        let theirs = encode_sparse_side(&base, &base);
        let keys: Vec<String> = base.keys().cloned().collect();
        let mask = sparse_diff(&keys, &ours, &theirs);
        assert_eq!(
            mask.get("function:main"),
            Some(&SparseRowClass::BothUnchanged)
        );
        assert_eq!(
            mask.get("function:other"),
            Some(&SparseRowClass::BothUnchanged)
        );
    }

    #[test]
    fn ours_only_change_classified() {
        let base = units(&[("a", "1"), ("b", "2")]);
        let mut side = base.clone();
        side.insert("b".into(), b"22".to_vec());
        let ours = encode_sparse_side(&base, &side);
        let theirs = encode_sparse_side(&base, &base);
        let keys: Vec<String> = base.keys().cloned().collect();
        let mask = sparse_diff(&keys, &ours, &theirs);
        assert_eq!(mask.get("a"), Some(&SparseRowClass::BothUnchanged));
        assert_eq!(mask.get("b"), Some(&SparseRowClass::OursOnly));
    }

    #[test]
    fn both_modified_when_both_sides_change_same_key() {
        let base = units(&[("k", "base")]);
        let mut ours_side = base.clone();
        ours_side.insert("k".into(), b"ours".to_vec());
        let mut theirs_side = base.clone();
        theirs_side.insert("k".into(), b"theirs".to_vec());
        let ours = encode_sparse_side(&base, &ours_side);
        let theirs = encode_sparse_side(&base, &theirs_side);
        let mask = sparse_diff(&["k".to_string()], &ours, &theirs);
        assert_eq!(mask.get("k"), Some(&SparseRowClass::BothModified));
    }

    #[test]
    fn identical_side_change_hashes_still_both_modified() {
        let base = units(&[("k", "base")]);
        let mut side = base.clone();
        side.insert("k".into(), b"same".to_vec());
        let ours = encode_sparse_side(&base, &side);
        let theirs = encode_sparse_side(&base, &side);
        let mask = sparse_diff(&["k".to_string()], &ours, &theirs);
        assert_eq!(mask.get("k"), Some(&SparseRowClass::BothModified));
    }

    #[test]
    fn disjoint_insertions_on_new_keys() {
        let base = units(&[("a", "1")]);
        let mut ours_side = base.clone();
        ours_side.insert("insert_ours".into(), b"x".to_vec());
        let mut theirs_side = base.clone();
        theirs_side.insert("insert_theirs".into(), b"y".to_vec());
        let ours = encode_sparse_side(&base, &ours_side);
        let theirs = encode_sparse_side(&base, &theirs_side);
        let keys: Vec<String> = base.keys().cloned().collect();
        let mask = sparse_diff(&keys, &ours, &theirs);
        assert_eq!(mask.get("insert_ours"), Some(&SparseRowClass::OursOnly));
        assert_eq!(mask.get("insert_theirs"), Some(&SparseRowClass::TheirsOnly));
    }

    #[test]
    fn same_new_key_different_payloads_disjoint_insert() {
        let base = units(&[]);
        let mut ours_side = BTreeMap::new();
        ours_side.insert("new".into(), b"1".to_vec());
        let mut theirs_side = BTreeMap::new();
        theirs_side.insert("new".into(), b"2".to_vec());
        let ours = encode_sparse_side(&base, &ours_side);
        let theirs = encode_sparse_side(&base, &theirs_side);
        let mask = sparse_diff(&[], &ours, &theirs);
        assert_eq!(mask.get("new"), Some(&SparseRowClass::DisjointInsert));
    }

    #[test]
    fn mask_keys_sorted_lexicographically() {
        let base = units(&[("b", "2"), ("a", "1")]);
        let ours = encode_sparse_side(&base, &base);
        let theirs = encode_sparse_side(&base, &base);
        let keys = vec!["b".to_string(), "a".to_string()];
        let mask = sparse_diff(&keys, &ours, &theirs);
        let ordered: Vec<_> = mask.keys().cloned().collect();
        assert_eq!(ordered, vec!["a".to_string(), "b".to_string()]);
    }

    #[test]
    fn simd_hash_equality_matches_scalar_reference() {
        let h1 = Hash::from_bytes(blake3::hash(b"alpha").into());
        let h2 = Hash::from_bytes(blake3::hash(b"beta").into());
        assert!(super::simd::hashes_equal(h1, h1));
        assert!(!super::simd::hashes_equal(h1, h2));
    }
}
