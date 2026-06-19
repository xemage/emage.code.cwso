//! CSR-style sparse AST tensor encoding (ADR-009 / sparse-ast-tensor-encoding-v1).

use std::collections::BTreeMap;

use blake3::Hash;
use thiserror::Error;

pub const SPAT_MAGIC: &[u8; 4] = b"SPAT";
pub const SPAT_VERSION: u16 = 1;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum SparseTensorError {
    #[error("invalid SPAT magic")]
    BadMagic,
    #[error("unsupported SPAT version {0}")]
    UnsupportedVersion(u16),
    #[error("truncated SPAT buffer")]
    Truncated,
    #[error("row payload length mismatch for key {key}")]
    PayloadLen { key: String },
}

/// One non-zero (changed vs base) top-level AST unit row.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SparseRow {
    pub payload: Vec<u8>,
    pub payload_hash: Hash,
}

/// Sparse side tensor: only rows that differ from `base` (structural zeros omitted).
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct SparseAstTensor {
    /// Rows sorted by key (deterministic iteration order).
    pub rows: BTreeMap<String, SparseRow>,
}

impl SparseAstTensor {
    pub fn row(&self, key: &str) -> Option<&SparseRow> {
        self.rows.get(key)
    }

    pub fn insert_row(&mut self, key: String, payload: Vec<u8>) {
        let payload_hash = Hash::from_bytes(blake3::hash(&payload).into());
        self.rows.insert(
            key,
            SparseRow {
                payload,
                payload_hash,
            },
        );
    }
}

/// Build a sparse tensor for `side` relative to `base` keyed units (`key`, text bytes).
pub fn encode_sparse_side(
    base: &BTreeMap<String, Vec<u8>>,
    side: &BTreeMap<String, Vec<u8>>,
) -> SparseAstTensor {
    let mut tensor = SparseAstTensor::default();
    for (key, side_text) in side {
        let unchanged = base
            .get(key)
            .is_some_and(|base_text| base_text == side_text);
        if unchanged {
            continue;
        }
        tensor.insert_row(key.clone(), side_text.clone());
    }
    tensor
}

/// Serialize `SparseAstTensor` to the v1 SPAT wire bytes.
pub fn serialize_spat(tensor: &SparseAstTensor) -> Vec<u8> {
    let row_count = tensor.rows.len() as u32;
    let mut keys_blob = Vec::new();
    let mut lens = Vec::new();
    let mut payloads = Vec::new();
    for (key, row) in &tensor.rows {
        let key_bytes = key.as_bytes();
        keys_blob.extend_from_slice(&(key_bytes.len() as u32).to_le_bytes());
        keys_blob.extend_from_slice(key_bytes);
        lens.push(row.payload.len() as u32);
        payloads.extend_from_slice(&row.payload);
    }
    let mut out = Vec::new();
    out.extend_from_slice(SPAT_MAGIC);
    out.extend_from_slice(&SPAT_VERSION.to_le_bytes());
    out.extend_from_slice(&row_count.to_le_bytes());
    out.extend_from_slice(&(keys_blob.len() as u32).to_le_bytes());
    out.extend_from_slice(&keys_blob);
    for len in &lens {
        out.extend_from_slice(&len.to_le_bytes());
    }
    out.extend_from_slice(&payloads);
    out
}

/// Parse v1 SPAT bytes into `SparseAstTensor`.
pub fn parse_spat(bytes: &[u8]) -> Result<SparseAstTensor, SparseTensorError> {
    let mut cursor = bytes;
    let magic: [u8; 4] = take_fixed(&mut cursor, SparseTensorError::Truncated)?;
    if &magic != SPAT_MAGIC {
        return Err(SparseTensorError::BadMagic);
    }
    let version = take_u16(&mut cursor)?;
    if version != SPAT_VERSION {
        return Err(SparseTensorError::UnsupportedVersion(version));
    }
    let row_count = take_u32(&mut cursor)? as usize;
    let keys_len = take_u32(&mut cursor)? as usize;
    let keys_blob = take_slice(&mut cursor, keys_len)?;
    let lens = (0..row_count)
        .map(|_| take_u32(&mut cursor))
        .collect::<Result<Vec<_>, _>>()?;

    let mut keys = Vec::with_capacity(row_count);
    let mut keys_cursor = keys_blob;
    for _ in 0..row_count {
        let key_len = take_u32(&mut keys_cursor)? as usize;
        let key_bytes = take_slice(&mut keys_cursor, key_len)?;
        let key = std::str::from_utf8(key_bytes)
            .map_err(|_| SparseTensorError::Truncated)?
            .to_string();
        keys.push(key);
    }

    let mut tensor = SparseAstTensor::default();
    for (key, &payload_len) in keys.iter().zip(lens.iter()) {
        let payload = take_vec(&mut cursor, payload_len as usize)?;
        if payload.len() != payload_len as usize {
            return Err(SparseTensorError::PayloadLen { key: key.clone() });
        }
        tensor.insert_row(key.clone(), payload);
    }
    if !cursor.is_empty() {
        return Err(SparseTensorError::Truncated);
    }
    Ok(tensor)
}

fn take_fixed<const N: usize>(
    cursor: &mut &[u8],
    err: SparseTensorError,
) -> Result<[u8; N], SparseTensorError> {
    if cursor.len() < N {
        return Err(err);
    }
    let (head, tail) = cursor.split_at(N);
    *cursor = tail;
    Ok(head.try_into().expect("split_at N"))
}

fn take_u16(cursor: &mut &[u8]) -> Result<u16, SparseTensorError> {
    let bytes = take_fixed(cursor, SparseTensorError::Truncated)?;
    Ok(u16::from_le_bytes(bytes))
}

fn take_u32(cursor: &mut &[u8]) -> Result<u32, SparseTensorError> {
    let bytes = take_fixed(cursor, SparseTensorError::Truncated)?;
    Ok(u32::from_le_bytes(bytes))
}

fn take_slice<'a>(cursor: &mut &'a [u8], len: usize) -> Result<&'a [u8], SparseTensorError> {
    if cursor.len() < len {
        return Err(SparseTensorError::Truncated);
    }
    let (head, tail) = cursor.split_at(len);
    *cursor = tail;
    Ok(head)
}

fn take_vec(cursor: &mut &[u8], len: usize) -> Result<Vec<u8>, SparseTensorError> {
    Ok(take_slice(cursor, len)?.to_vec())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn spat_round_trip_preserves_rows() {
        let mut tensor = SparseAstTensor::default();
        tensor.insert_row("function:main".into(), b"fn main() {}".to_vec());
        tensor.insert_row("function:other".into(), b"fn other() {}".to_vec());
        let wire = serialize_spat(&tensor);
        let parsed = parse_spat(&wire).expect("parse");
        assert_eq!(parsed, tensor);
    }

    #[test]
    fn encode_sparse_side_omits_unchanged_rows() {
        let mut base = BTreeMap::new();
        base.insert("a".into(), b"1".to_vec());
        base.insert("b".into(), b"2".to_vec());
        let mut side = base.clone();
        side.insert("b".into(), b"22".to_vec());
        let sparse = encode_sparse_side(&base, &side);
        assert_eq!(sparse.rows.len(), 1);
        assert!(sparse.row("a").is_none());
        assert!(sparse.row("b").is_some());
    }
}
