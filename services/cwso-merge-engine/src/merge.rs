use std::collections::{BTreeMap, HashMap};

use serde::{Deserialize, Serialize};
use thiserror::Error;
use tree_sitter::Node;

use crate::parse;
use crate::proto::MergeLanguage;
use crate::sparse_diff::{sparse_diff, SparseDiffMask, SparseRowClass};
use crate::sparse_tensor::encode_sparse_side;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum MergeError {
    #[error("AST semantic conflict")]
    SemanticConflict,
}

/// Per-unit state used in [`ConflictMatrixEntry`]. Mirrors the internal
/// [`NodeState`] but is a stable, serializable, external-facing vocabulary
/// (C042 / Blueprint §5.4 "conflict matrix" contract).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ConflictState {
    Unchanged,
    Deleted,
    Modified,
    /// Both sides introduced a *new* top-level unit under the same
    /// canonical key (e.g. two agents both add a same-named function)
    /// with diverging content -- there is no base-side state for this
    /// case, so both `ours_state`/`theirs_state` report `Inserted`.
    Inserted,
}

/// One row of the Blueprint §5.4 "Conflict Escalation Matrix": a single
/// AST top-level unit (function, struct, class, etc.) on which `ours` and
/// `theirs` collide in a way the algorithmic merge cannot auto-resolve.
///
/// This is *data*, not a formatted message -- see Blueprint §3.3 step 4
/// ("a highly structured JSON conflict report detailing the exact AST
/// node collisions") and §5.4's `merge_concurrent_results` description
/// ("returns a formatted JSON conflict matrix instead of corrupting the
/// file"). Presentation to a human/LLM reviewer is out of scope here.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConflictMatrixEntry {
    /// Canonical AST unit key (see `canonical_raw_key`), e.g. `"function_item:left#1"`.
    pub unit_key: String,
    /// The tree-sitter node kind for this unit, e.g. `"function_item"`.
    pub node_kind: String,
    /// The unit's extracted name, if the grammar exposes one (`None` for
    /// anonymous/unnamed top-level nodes).
    pub node_name: Option<String>,
    pub ours_state: ConflictState,
    pub theirs_state: ConflictState,
    /// Deterministic, stable reason code for *why* this row is a
    /// collision rather than an auto-resolvable edit.
    pub reason_code: String,
}

#[derive(Clone)]
struct AstUnit {
    key: String,
    text: Vec<u8>,
}

#[derive(Clone)]
enum NodeState {
    Unchanged,
    Deleted,
    Modified(Vec<u8>),
}

struct SideDiff {
    states: HashMap<String, NodeState>,
    insertions: HashMap<String, InsertUnit>,
}

#[derive(Clone)]
struct InsertUnit {
    key: String,
    text: Vec<u8>,
    anchor: Option<String>,
}

#[derive(Clone)]
enum MergeDecision {
    Keep(Vec<u8>),
    Delete,
}

pub fn merge_three_way(
    language: MergeLanguage,
    base: &[u8],
    ours: &[u8],
    theirs: &[u8],
) -> std::result::Result<Vec<u8>, MergeError> {
    if ours == theirs {
        return Ok(ours.to_vec());
    }

    if base == ours {
        return Ok(theirs.to_vec());
    }

    if base == theirs {
        return Ok(ours.to_vec());
    }

    let base_units = extract_top_level_units(language, base)?;
    let ours_units = extract_top_level_units(language, ours)?;
    let theirs_units = extract_top_level_units(language, theirs)?;

    merge_units(language, &base_units, &ours_units, &theirs_units, true)
}

/// Builds the Blueprint §5.4 conflict matrix for a three-way merge that
/// [`merge_three_way`] has already determined (or would determine) to be
/// unresolvable.
///
/// This performs its own independent parse + diff pass (always the dense,
/// no-sparse-prefilter path -- correctness over throughput on the rare
/// conflict path) rather than threading state out of `merge_three_way`, so
/// it can be called standalone by an IPC layer purely from `(language,
/// base, ours, theirs)` after `merge_three_way` has already returned
/// `Err(MergeError::SemanticConflict)`.
///
/// Returns every top-level unit where `ours` and `theirs` genuinely
/// collide -- i.e. every unit that would make [`resolve_base_decisions`]
/// or [`merge_insertions`] return `Err` -- not just the first one, so a
/// caller sees the *entire* set of AST node collisions in one report, per
/// Blueprint §3.3 step 4 ("a highly structured JSON conflict report
/// detailing the exact AST node collisions").
///
/// `Ok(vec)` is returned even when `vec` is empty: that happens only when
/// the underlying conflict was not a per-unit collision (e.g. the
/// reassembled merge failed tree-sitter's `has_error()` validation despite
/// no individual unit colliding) and the caller must fall back to a
/// non-matrix conflict report rather than fabricate rows. `Err` propagates
/// only genuine parse failures on one of the three inputs, in which case
/// no matrix can be computed at all.
pub fn conflict_matrix(
    language: MergeLanguage,
    base: &[u8],
    ours: &[u8],
    theirs: &[u8],
) -> std::result::Result<Vec<ConflictMatrixEntry>, MergeError> {
    let base_units = extract_top_level_units(language, base)?;
    let ours_units = extract_top_level_units(language, ours)?;
    let theirs_units = extract_top_level_units(language, theirs)?;

    let ours_diff = build_side_diff(&base_units, &ours_units, None, MergeSide::Ours);
    let theirs_diff = build_side_diff(&base_units, &theirs_units, None, MergeSide::Theirs);

    let mut matrix: BTreeMap<String, ConflictMatrixEntry> = BTreeMap::new();

    for base_unit in &base_units {
        let ours_state = ours_diff
            .states
            .get(&base_unit.key)
            .ok_or(MergeError::SemanticConflict)?;
        let theirs_state = theirs_diff
            .states
            .get(&base_unit.key)
            .ok_or(MergeError::SemanticConflict)?;

        let reason_code = match (ours_state, theirs_state) {
            (NodeState::Modified(o), NodeState::Modified(t)) if o != t => {
                Some("both_modified_diverged")
            }
            (NodeState::Deleted, NodeState::Modified(_))
            | (NodeState::Modified(_), NodeState::Deleted) => Some("delete_modify_conflict"),
            _ => None,
        };

        if let Some(reason_code) = reason_code {
            let (node_kind, node_name) = describe_unit_key(&base_unit.key);
            matrix.insert(
                base_unit.key.clone(),
                ConflictMatrixEntry {
                    unit_key: base_unit.key.clone(),
                    node_kind,
                    node_name,
                    ours_state: conflict_state_tag(ours_state),
                    theirs_state: conflict_state_tag(theirs_state),
                    reason_code: reason_code.to_string(),
                },
            );
        }
    }

    // Insertion collisions: both sides introduce a *new* node under the
    // same canonical key (not present in `base_units`) but with different
    // text or anchor -- e.g. two agents both add a same-named function
    // with different bodies. Not covered by the base-unit loop above
    // since these keys have no base-side state at all.
    for (key, ours_insert) in &ours_diff.insertions {
        if let Some(theirs_insert) = theirs_diff.insertions.get(key) {
            let diverges = theirs_insert.text != ours_insert.text
                || theirs_insert.anchor != ours_insert.anchor;
            if diverges {
                let (node_kind, node_name) = describe_unit_key(key);
                matrix.insert(
                    key.clone(),
                    ConflictMatrixEntry {
                        unit_key: key.clone(),
                        node_kind,
                        node_name,
                        ours_state: ConflictState::Inserted,
                        theirs_state: ConflictState::Inserted,
                        reason_code: "insertion_diverged".to_string(),
                    },
                );
            }
        }
    }

    Ok(matrix.into_values().collect())
}

fn conflict_state_tag(state: &NodeState) -> ConflictState {
    match state {
        NodeState::Unchanged => ConflictState::Unchanged,
        NodeState::Deleted => ConflictState::Deleted,
        NodeState::Modified(_) => ConflictState::Modified,
    }
}

/// Recovers `(node_kind, node_name)` from a canonical unit key produced by
/// `canonical_raw_key` + `extract_top_level_units`'s `"{raw_key}#{ordinal}"`
/// suffixing. Keys look like `"function_item:left#1"` (named) or
/// `"comment:@3#1"` (anonymous, `@{ordinal}` placeholder name).
fn describe_unit_key(key: &str) -> (String, Option<String>) {
    let without_ordinal = match key.rfind('#') {
        Some(idx) => &key[..idx],
        None => key,
    };
    match without_ordinal.split_once(':') {
        Some((kind, name)) if !name.starts_with('@') => (kind.to_string(), Some(name.to_string())),
        Some((kind, _)) => (kind.to_string(), None),
        None => (without_ordinal.to_string(), None),
    }
}

/// ADR-006 dense path (no sparse pre-filter). Used by the T129 conformance suite.
#[cfg(test)]
fn merge_three_way_dense(
    language: MergeLanguage,
    base: &[u8],
    ours: &[u8],
    theirs: &[u8],
) -> std::result::Result<Vec<u8>, MergeError> {
    if ours == theirs {
        return Ok(ours.to_vec());
    }

    if base == ours {
        return Ok(theirs.to_vec());
    }

    if base == theirs {
        return Ok(ours.to_vec());
    }

    let base_units = extract_top_level_units(language, base)?;
    let ours_units = extract_top_level_units(language, ours)?;
    let theirs_units = extract_top_level_units(language, theirs)?;

    merge_units(language, &base_units, &ours_units, &theirs_units, false)
}

fn merge_units(
    language: MergeLanguage,
    base_units: &[AstUnit],
    ours_units: &[AstUnit],
    theirs_units: &[AstUnit],
    use_sparse_prefilter: bool,
) -> std::result::Result<Vec<u8>, MergeError> {
    let base_order: Vec<String> = base_units.iter().map(|u| u.key.clone()).collect();
    let mask = if use_sparse_prefilter {
        Some(sparse_prefilter_mask(
            base_units,
            ours_units,
            theirs_units,
            &base_order,
        ))
    } else {
        None
    };
    let ours_diff = build_side_diff(base_units, ours_units, mask.as_ref(), MergeSide::Ours);
    let theirs_diff = build_side_diff(base_units, theirs_units, mask.as_ref(), MergeSide::Theirs);
    let decisions = resolve_base_decisions(&base_units, &ours_diff.states, &theirs_diff.states)?;
    let merged_insertions = merge_insertions(&ours_diff.insertions, &theirs_diff.insertions)?;
    let merged = assemble_output(&base_order, &decisions, &merged_insertions);

    parse::validate_parse(language, &merged).map_err(|_| MergeError::SemanticConflict)?;
    Ok(merged)
}

fn extract_top_level_units(
    language: MergeLanguage,
    source: &[u8],
) -> std::result::Result<Vec<AstUnit>, MergeError> {
    let (source_text, tree) =
        parse::parse_tree(language, source).map_err(|_| MergeError::SemanticConflict)?;
    let root = tree.root_node();
    let mut raw_units = Vec::new();

    for idx in 0..root.named_child_count() {
        if let Some(node) = root.named_child(idx) {
            if node.kind() == "comment" {
                continue;
            }
            let raw_key = canonical_raw_key(node, &source_text, idx as usize);
            let text = source_text.as_bytes()[node.start_byte()..node.end_byte()].to_vec();
            raw_units.push((raw_key, text));
        }
    }

    let mut counts: HashMap<String, usize> = HashMap::new();
    let mut units = Vec::new();
    for (raw_key, text) in raw_units {
        let entry = counts.entry(raw_key.clone()).or_insert(0);
        *entry += 1;
        let key = format!("{}#{}", raw_key, *entry);
        units.push(AstUnit { key, text });
    }

    Ok(units)
}

fn canonical_raw_key(node: Node<'_>, source_text: &str, ordinal: usize) -> String {
    if let Some(named) = node.child_by_field_name("name") {
        let name = source_slice(source_text, named).trim();
        if !name.is_empty() {
            return format!("{}:{}", node.kind(), name);
        }
    }

    if is_import_kind(node.kind()) {
        if let Some(path) = extract_string_literal(source_slice(source_text, node)) {
            return format!("{}:{}", node.kind(), path);
        }
    }

    format!("{}:@{}", node.kind(), ordinal)
}

fn source_slice<'a>(source_text: &'a str, node: Node<'_>) -> &'a str {
    &source_text[node.start_byte()..node.end_byte()]
}

fn is_import_kind(kind: &str) -> bool {
    kind.contains("import") || kind == "use_declaration"
}

fn extract_string_literal(text: &str) -> Option<String> {
    let bytes = text.as_bytes();
    let mut start = None;
    let mut quote = b'\0';

    for (idx, byte) in bytes.iter().enumerate() {
        if start.is_none() && (*byte == b'\'' || *byte == b'"') {
            start = Some(idx + 1);
            quote = *byte;
            continue;
        }

        if let Some(from) = start {
            if *byte == quote {
                return Some(text[from..idx].to_string());
            }
        }
    }

    None
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum MergeSide {
    Ours,
    Theirs,
}

fn units_to_map(units: &[AstUnit]) -> BTreeMap<String, Vec<u8>> {
    units
        .iter()
        .map(|u| (u.key.clone(), u.text.clone()))
        .collect()
}

fn sparse_prefilter_mask(
    base_units: &[AstUnit],
    ours_units: &[AstUnit],
    theirs_units: &[AstUnit],
    base_order: &[String],
) -> SparseDiffMask {
    let base_map = units_to_map(base_units);
    let ours_map = units_to_map(ours_units);
    let theirs_map = units_to_map(theirs_units);
    let ours_sparse = encode_sparse_side(&base_map, &ours_map);
    let theirs_sparse = encode_sparse_side(&base_map, &theirs_map);
    sparse_diff(base_order, &ours_sparse, &theirs_sparse)
}

fn build_side_diff(
    base_units: &[AstUnit],
    side_units: &[AstUnit],
    mask: Option<&SparseDiffMask>,
    side: MergeSide,
) -> SideDiff {
    let mut side_map: HashMap<String, Vec<u8>> = HashMap::new();
    for unit in side_units {
        side_map.insert(unit.key.clone(), unit.text.clone());
    }

    let mut states = HashMap::new();
    for base in base_units {
        let state = match mask {
            None => lookup_side_state(base, &side_map),
            Some(mask) => match mask.get(&base.key) {
                Some(SparseRowClass::BothUnchanged) => NodeState::Unchanged,
                Some(SparseRowClass::OursOnly) if side == MergeSide::Theirs => NodeState::Unchanged,
                Some(SparseRowClass::TheirsOnly) if side == MergeSide::Ours => NodeState::Unchanged,
                Some(
                    SparseRowClass::OursOnly
                    | SparseRowClass::TheirsOnly
                    | SparseRowClass::BothModified,
                ) => lookup_side_state(base, &side_map),
                Some(SparseRowClass::DisjointInsert) => lookup_side_state(base, &side_map),
                None => lookup_side_state(base, &side_map),
            },
        };
        states.insert(base.key.clone(), state);
    }

    let mut base_key_set: HashMap<&str, usize> = HashMap::new();
    for (idx, unit) in base_units.iter().enumerate() {
        base_key_set.insert(unit.key.as_str(), idx);
    }

    let mut insertions = HashMap::new();
    for (idx, unit) in side_units.iter().enumerate() {
        if base_key_set.contains_key(unit.key.as_str()) {
            continue;
        }

        let anchor = find_anchor(side_units, &base_key_set, idx);
        insertions.insert(
            unit.key.clone(),
            InsertUnit {
                key: unit.key.clone(),
                text: unit.text.clone(),
                anchor,
            },
        );
    }

    SideDiff { states, insertions }
}

fn lookup_side_state(base: &AstUnit, side_map: &HashMap<String, Vec<u8>>) -> NodeState {
    match side_map.get(&base.key) {
        None => NodeState::Deleted,
        Some(side_text) if *side_text == base.text => NodeState::Unchanged,
        Some(side_text) => NodeState::Modified(side_text.clone()),
    }
}

fn find_anchor(
    side_units: &[AstUnit],
    base_key_set: &HashMap<&str, usize>,
    idx: usize,
) -> Option<String> {
    let mut cursor = idx as isize - 1;
    while cursor >= 0 {
        let candidate = &side_units[cursor as usize];
        if base_key_set.contains_key(candidate.key.as_str()) {
            return Some(candidate.key.clone());
        }
        cursor -= 1;
    }
    None
}

fn resolve_base_decisions(
    base_units: &[AstUnit],
    ours_states: &HashMap<String, NodeState>,
    theirs_states: &HashMap<String, NodeState>,
) -> std::result::Result<BTreeMap<String, MergeDecision>, MergeError> {
    let mut decisions = BTreeMap::new();

    for base in base_units {
        let ours = ours_states
            .get(&base.key)
            .ok_or(MergeError::SemanticConflict)?;
        let theirs = theirs_states
            .get(&base.key)
            .ok_or(MergeError::SemanticConflict)?;

        let decision = match (ours, theirs) {
            (NodeState::Unchanged, NodeState::Unchanged) => MergeDecision::Keep(base.text.clone()),
            (NodeState::Modified(text), NodeState::Unchanged)
            | (NodeState::Unchanged, NodeState::Modified(text)) => {
                MergeDecision::Keep(text.clone())
            }
            (NodeState::Deleted, NodeState::Unchanged)
            | (NodeState::Unchanged, NodeState::Deleted)
            | (NodeState::Deleted, NodeState::Deleted) => MergeDecision::Delete,
            (NodeState::Modified(ours_text), NodeState::Modified(theirs_text)) => {
                if ours_text == theirs_text {
                    MergeDecision::Keep(ours_text.clone())
                } else {
                    return Err(MergeError::SemanticConflict);
                }
            }
            (NodeState::Deleted, NodeState::Modified(_))
            | (NodeState::Modified(_), NodeState::Deleted) => {
                return Err(MergeError::SemanticConflict);
            }
        };

        decisions.insert(base.key.clone(), decision);
    }

    Ok(decisions)
}

fn merge_insertions(
    ours: &HashMap<String, InsertUnit>,
    theirs: &HashMap<String, InsertUnit>,
) -> std::result::Result<HashMap<Option<String>, Vec<InsertUnit>>, MergeError> {
    let mut combined: HashMap<String, InsertUnit> = HashMap::new();
    for insert in ours.values() {
        combined.insert(insert.key.clone(), insert.clone());
    }

    for insert in theirs.values() {
        match combined.get(&insert.key) {
            None => {
                combined.insert(insert.key.clone(), insert.clone());
            }
            Some(existing) => {
                if existing.text != insert.text || existing.anchor != insert.anchor {
                    return Err(MergeError::SemanticConflict);
                }
            }
        }
    }

    let mut anchored: HashMap<Option<String>, Vec<InsertUnit>> = HashMap::new();
    for insert in combined.into_values() {
        anchored
            .entry(insert.anchor.clone())
            .or_default()
            .push(insert);
    }

    for values in anchored.values_mut() {
        values.sort_by(|a, b| a.key.cmp(&b.key));
    }

    Ok(anchored)
}

fn assemble_output(
    base_order: &[String],
    decisions: &BTreeMap<String, MergeDecision>,
    insertions: &HashMap<Option<String>, Vec<InsertUnit>>,
) -> Vec<u8> {
    let mut output = Vec::new();
    append_insertions(&mut output, insertions.get(&None));

    for key in base_order {
        if let Some(decision) = decisions.get(key) {
            match decision {
                MergeDecision::Keep(text) => append_unit(&mut output, text),
                MergeDecision::Delete => {}
            }
        }
        append_insertions(&mut output, insertions.get(&Some(key.clone())));
    }

    output
}

fn append_insertions(output: &mut Vec<u8>, maybe_units: Option<&Vec<InsertUnit>>) {
    if let Some(units) = maybe_units {
        for unit in units {
            append_unit(output, &unit.text);
        }
    }
}

fn append_unit(output: &mut Vec<u8>, text: &[u8]) {
    let trimmed = trim_ascii_whitespace(text);
    if trimmed.is_empty() {
        return;
    }

    if !output.is_empty() && !output.ends_with(b"\n") {
        output.push(b'\n');
    }

    output.extend_from_slice(trimmed);
    if !output.ends_with(b"\n") {
        output.push(b'\n');
    }
}

fn trim_ascii_whitespace(input: &[u8]) -> &[u8] {
    let mut start = 0;
    let mut end = input.len();

    while start < end && input[start].is_ascii_whitespace() {
        start += 1;
    }
    while end > start && input[end - 1].is_ascii_whitespace() {
        end -= 1;
    }

    &input[start..end]
}

#[cfg(test)]
mod tests {
    use super::*;

    use crate::parse;

    #[derive(Clone, Copy)]
    struct TrivialFixture {
        lang: MergeLanguage,
        base: &'static str,
        ours_change: &'static str,
        theirs_change: &'static str,
    }

    struct SemanticFixture {
        lang: MergeLanguage,
        base: &'static str,
        ours: &'static str,
        theirs: &'static str,
        expected: &'static str,
        overlap_theirs: &'static str,
    }

    fn trivial_fixtures() -> Vec<TrivialFixture> {
        vec![
            TrivialFixture {
                lang: MergeLanguage::Go,
                base: "package main\n\nfunc value() int {\n\treturn 1\n}\n",
                ours_change: "package main\n\nfunc value() int {\n\treturn 2\n}\n",
                theirs_change: "package main\n\nfunc value() int {\n\treturn 3\n}\n",
            },
            TrivialFixture {
                lang: MergeLanguage::Rust,
                base: "fn value() -> i32 {\n    1\n}\n",
                ours_change: "fn value() -> i32 {\n    2\n}\n",
                theirs_change: "fn value() -> i32 {\n    3\n}\n",
            },
            TrivialFixture {
                lang: MergeLanguage::Python,
                base: "def value():\n    return 1\n",
                ours_change: "def value():\n    return 2\n",
                theirs_change: "def value():\n    return 3\n",
            },
            TrivialFixture {
                lang: MergeLanguage::TypeScript,
                base: "export function value(): number {\n  return 1;\n}\n",
                ours_change: "export function value(): number {\n  return 2;\n}\n",
                theirs_change: "export function value(): number {\n  return 3;\n}\n",
            },
        ]
    }

    fn semantic_fixtures() -> Vec<SemanticFixture> {
        vec![
            SemanticFixture {
                lang: MergeLanguage::Go,
                base: "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 2\n}\n",
                ours: "package main\n\nfunc left() int {\n\treturn 10\n}\n\nfunc right() int {\n\treturn 2\n}\n",
                theirs: "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 20\n}\n",
                expected: "package main\nfunc left() int {\n\treturn 10\n}\nfunc right() int {\n\treturn 20\n}\n",
                overlap_theirs: "package main\n\nfunc left() int {\n\treturn 99\n}\n\nfunc right() int {\n\treturn 2\n}\n",
            },
            SemanticFixture {
                lang: MergeLanguage::Rust,
                base: "fn left() -> i32 {\n    1\n}\n\nfn right() -> i32 {\n    2\n}\n",
                ours: "fn left() -> i32 {\n    10\n}\n\nfn right() -> i32 {\n    2\n}\n",
                theirs: "fn left() -> i32 {\n    1\n}\n\nfn right() -> i32 {\n    20\n}\n",
                expected: "fn left() -> i32 {\n    10\n}\nfn right() -> i32 {\n    20\n}\n",
                overlap_theirs: "fn left() -> i32 {\n    99\n}\n\nfn right() -> i32 {\n    2\n}\n",
            },
            SemanticFixture {
                lang: MergeLanguage::Python,
                base: "def left():\n    return 1\n\ndef right():\n    return 2\n",
                ours: "def left():\n    return 10\n\ndef right():\n    return 2\n",
                theirs: "def left():\n    return 1\n\ndef right():\n    return 20\n",
                expected: "def left():\n    return 10\ndef right():\n    return 20\n",
                overlap_theirs: "def left():\n    return 99\n\ndef right():\n    return 2\n",
            },
            SemanticFixture {
                lang: MergeLanguage::TypeScript,
                base: "export function left(): number {\n  return 1;\n}\n\nexport function right(): number {\n  return 2;\n}\n",
                ours: "export function left(): number {\n  return 10;\n}\n\nexport function right(): number {\n  return 2;\n}\n",
                theirs: "export function left(): number {\n  return 1;\n}\n\nexport function right(): number {\n  return 20;\n}\n",
                expected: "export function left(): number {\n  return 10;\n}\nexport function right(): number {\n  return 20;\n}\n",
                overlap_theirs: "export function left(): number {\n  return 99;\n}\n\nexport function right(): number {\n  return 2;\n}\n",
            },
        ]
    }

    #[test]
    fn merge_noop_identical_bytes_all_languages() {
        for fixture in trivial_fixtures() {
            let merged = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.base.as_bytes(),
                fixture.base.as_bytes(),
            )
            .expect("merge must succeed");
            assert_eq!(merged, fixture.base.as_bytes());
        }
    }

    #[test]
    fn merge_one_side_modified_all_languages() {
        for fixture in trivial_fixtures() {
            let merged_theirs = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.base.as_bytes(),
                fixture.theirs_change.as_bytes(),
            )
            .expect("merge must take theirs");
            assert_eq!(merged_theirs, fixture.theirs_change.as_bytes());

            let merged_ours = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours_change.as_bytes(),
                fixture.base.as_bytes(),
            )
            .expect("merge must take ours");
            assert_eq!(merged_ours, fixture.ours_change.as_bytes());
        }
    }

    #[test]
    fn merge_both_sides_same_change_all_languages() {
        for fixture in trivial_fixtures() {
            let merged = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours_change.as_bytes(),
                fixture.ours_change.as_bytes(),
            )
            .expect("merge must take common change");
            assert_eq!(merged, fixture.ours_change.as_bytes());
        }
    }

    #[test]
    fn merge_non_trivial_collision_returns_semantic_conflict() {
        for fixture in trivial_fixtures() {
            let err = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours_change.as_bytes(),
                fixture.theirs_change.as_bytes(),
            )
            .expect_err("collision must be deferred");
            assert_eq!(err, MergeError::SemanticConflict);
        }
    }

    #[test]
    fn deterministic_output_over_repeated_runs() {
        for fixture in semantic_fixtures() {
            let expected = fixture.expected.as_bytes().to_vec();
            for _ in 0..100 {
                let merged = merge_three_way(
                    fixture.lang,
                    fixture.base.as_bytes(),
                    fixture.ours.as_bytes(),
                    fixture.theirs.as_bytes(),
                )
                .expect("merge must succeed");
                assert_eq!(merged, expected);
            }
        }
    }

    #[test]
    fn semantic_disjoint_edits_auto_resolve_all_languages() {
        for fixture in semantic_fixtures() {
            let merged = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours.as_bytes(),
                fixture.theirs.as_bytes(),
            )
            .expect("disjoint edits must merge");
            assert_eq!(merged, fixture.expected.as_bytes());
            parse::validate_parse(fixture.lang, &merged).expect("merged output must parse");
        }
    }

    #[test]
    fn semantic_overlap_returns_explicit_conflict_all_languages() {
        for fixture in semantic_fixtures() {
            let err = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours.as_bytes(),
                fixture.overlap_theirs.as_bytes(),
            )
            .expect_err("overlap must conflict");
            assert_eq!(err, MergeError::SemanticConflict);
        }
    }

    #[test]
    fn sparse_prefilter_seeds_unchanged_without_side_byte_compare() {
        let base_units = vec![
            AstUnit {
                key: "function:left#1".into(),
                text: b"fn left() {}".to_vec(),
            },
            AstUnit {
                key: "function:middle#1".into(),
                text: b"fn middle() {}".to_vec(),
            },
            AstUnit {
                key: "function:right#1".into(),
                text: b"fn right() {}".to_vec(),
            },
        ];
        let ours_units = vec![
            AstUnit {
                key: "function:left#1".into(),
                text: b"fn left() { 10 }".to_vec(),
            },
            AstUnit {
                key: "function:middle#1".into(),
                text: b"fn middle() {}".to_vec(),
            },
            AstUnit {
                key: "function:right#1".into(),
                text: b"fn right() {}".to_vec(),
            },
        ];
        let theirs_units = vec![
            AstUnit {
                key: "function:left#1".into(),
                text: b"fn left() {}".to_vec(),
            },
            AstUnit {
                key: "function:middle#1".into(),
                text: b"fn middle() {}".to_vec(),
            },
            AstUnit {
                key: "function:right#1".into(),
                text: b"fn right() { 20 }".to_vec(),
            },
        ];
        let base_order: Vec<String> = base_units.iter().map(|u| u.key.clone()).collect();
        let mask = sparse_prefilter_mask(&base_units, &ours_units, &theirs_units, &base_order);
        assert_eq!(
            mask.get("function:middle#1"),
            Some(&SparseRowClass::BothUnchanged)
        );
        assert_eq!(mask.get("function:left#1"), Some(&SparseRowClass::OursOnly));
        assert_eq!(
            mask.get("function:right#1"),
            Some(&SparseRowClass::TheirsOnly)
        );

        let ours_diff = build_side_diff(&base_units, &ours_units, Some(&mask), MergeSide::Ours);
        let theirs_diff =
            build_side_diff(&base_units, &theirs_units, Some(&mask), MergeSide::Theirs);
        assert!(matches!(
            ours_diff.states.get("function:middle#1"),
            Some(NodeState::Unchanged)
        ));
        assert!(matches!(
            theirs_diff.states.get("function:middle#1"),
            Some(NodeState::Unchanged)
        ));
    }

    #[test]
    fn sparse_prefilter_integration_matches_disjoint_semantic_merge() {
        for fixture in semantic_fixtures() {
            let merged = merge_three_way(
                fixture.lang,
                fixture.base.as_bytes(),
                fixture.ours.as_bytes(),
                fixture.theirs.as_bytes(),
            )
            .expect("sparse pre-filter path must preserve merge semantics");
            assert_eq!(merged, fixture.expected.as_bytes());
        }
    }

    #[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
    enum SideTag {
        Unchanged,
        Deleted,
        Modified,
    }

    #[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
    enum DecisionTag {
        Keep,
        Delete,
    }

    #[derive(Clone, Debug, PartialEq, Eq)]
    struct ConflictMatrixRow {
        ours: SideTag,
        theirs: SideTag,
        decision: DecisionTag,
    }

    fn side_tag(state: &NodeState) -> SideTag {
        match state {
            NodeState::Unchanged => SideTag::Unchanged,
            NodeState::Deleted => SideTag::Deleted,
            NodeState::Modified(_) => SideTag::Modified,
        }
    }

    fn decision_tag(decision: &MergeDecision) -> DecisionTag {
        match decision {
            MergeDecision::Keep(_) => DecisionTag::Keep,
            MergeDecision::Delete => DecisionTag::Delete,
        }
    }

    fn merge_conflict_matrix(
        language: MergeLanguage,
        base: &[u8],
        ours: &[u8],
        theirs: &[u8],
        use_sparse_prefilter: bool,
    ) -> std::result::Result<BTreeMap<String, ConflictMatrixRow>, MergeError> {
        let base_units = extract_top_level_units(language, base)?;
        let ours_units = extract_top_level_units(language, ours)?;
        let theirs_units = extract_top_level_units(language, theirs)?;
        let base_order: Vec<String> = base_units.iter().map(|u| u.key.clone()).collect();
        let mask = if use_sparse_prefilter {
            Some(sparse_prefilter_mask(
                &base_units,
                &ours_units,
                &theirs_units,
                &base_order,
            ))
        } else {
            None
        };
        let ours_diff = build_side_diff(&base_units, &ours_units, mask.as_ref(), MergeSide::Ours);
        let theirs_diff =
            build_side_diff(&base_units, &theirs_units, mask.as_ref(), MergeSide::Theirs);
        let decisions =
            resolve_base_decisions(&base_units, &ours_diff.states, &theirs_diff.states)?;
        let mut matrix = BTreeMap::new();
        for base in &base_units {
            let ours_state = ours_diff
                .states
                .get(&base.key)
                .ok_or(MergeError::SemanticConflict)?;
            let theirs_state = theirs_diff
                .states
                .get(&base.key)
                .ok_or(MergeError::SemanticConflict)?;
            let decision = decisions
                .get(&base.key)
                .ok_or(MergeError::SemanticConflict)?;
            matrix.insert(
                base.key.clone(),
                ConflictMatrixRow {
                    ours: side_tag(ours_state),
                    theirs: side_tag(theirs_state),
                    decision: decision_tag(decision),
                },
            );
        }
        Ok(matrix)
    }

    struct CorpusCase {
        label: &'static str,
        lang: MergeLanguage,
        base: &'static str,
        ours: &'static str,
        theirs: &'static str,
    }

    fn insertion_corpus() -> Vec<CorpusCase> {
        vec![
            CorpusCase {
                label: "go-disjoint-insertions",
                lang: MergeLanguage::Go,
                base: "package main\n\nfunc keep() int {\n\treturn 1\n}\n",
                ours: "package main\n\nfunc keep() int {\n\treturn 1\n}\n\nfunc oursOnly() int {\n\treturn 2\n}\n",
                theirs: "package main\n\nfunc keep() int {\n\treturn 1\n}\n\nfunc theirsOnly() int {\n\treturn 3\n}\n",
            },
            CorpusCase {
                label: "rust-disjoint-insertions",
                lang: MergeLanguage::Rust,
                base: "fn keep() -> i32 {\n    1\n}\n",
                ours: "fn keep() -> i32 {\n    1\n}\n\nfn ours_only() -> i32 {\n    2\n}\n",
                theirs: "fn keep() -> i32 {\n    1\n}\n\nfn theirs_only() -> i32 {\n    3\n}\n",
            },
            CorpusCase {
                label: "python-disjoint-insertions",
                lang: MergeLanguage::Python,
                base: "def keep():\n    return 1\n",
                ours: "def keep():\n    return 1\n\ndef ours_only():\n    return 2\n",
                theirs: "def keep():\n    return 1\n\ndef theirs_only():\n    return 3\n",
            },
            CorpusCase {
                label: "typescript-disjoint-insertions",
                lang: MergeLanguage::TypeScript,
                base: "export function keep(): number {\n  return 1;\n}\n",
                ours: "export function keep(): number {\n  return 1;\n}\n\nexport function oursOnly(): number {\n  return 2;\n}\n",
                theirs: "export function keep(): number {\n  return 1;\n}\n\nexport function theirsOnly(): number {\n  return 3;\n}\n",
            },
        ]
    }

    fn full_merge_corpus() -> Vec<CorpusCase> {
        let mut cases = Vec::new();

        for fixture in trivial_fixtures() {
            for (label, ours, theirs) in [
                ("trivial-identical", fixture.base, fixture.base),
                ("trivial-theirs-only", fixture.base, fixture.theirs_change),
                ("trivial-ours-only", fixture.ours_change, fixture.base),
                (
                    "trivial-both-same-change",
                    fixture.ours_change,
                    fixture.ours_change,
                ),
                (
                    "trivial-collision",
                    fixture.ours_change,
                    fixture.theirs_change,
                ),
                (
                    "trivial-collision-swapped",
                    fixture.theirs_change,
                    fixture.ours_change,
                ),
            ] {
                cases.push(CorpusCase {
                    label,
                    lang: fixture.lang,
                    base: fixture.base,
                    ours,
                    theirs,
                });
            }
        }

        for fixture in semantic_fixtures() {
            for (label, ours, theirs) in [
                ("semantic-disjoint", fixture.ours, fixture.theirs),
                (
                    "semantic-overlap-conflict",
                    fixture.ours,
                    fixture.overlap_theirs,
                ),
                ("semantic-ours-only", fixture.ours, fixture.base),
                ("semantic-theirs-only", fixture.base, fixture.theirs),
                ("semantic-both-expected", fixture.expected, fixture.expected),
            ] {
                cases.push(CorpusCase {
                    label,
                    lang: fixture.lang,
                    base: fixture.base,
                    ours,
                    theirs,
                });
            }
        }

        cases.extend(insertion_corpus());
        cases
    }

    /// Synthetic Go file with `unit_count` top-level functions; two disjoint edits on ours/theirs.
    fn synthesize_large_go_three_way(unit_count: usize) -> (Vec<u8>, Vec<u8>, Vec<u8>) {
        let mut base = String::from("package main\n\n");
        for i in 0..unit_count {
            base.push_str(&format!("func unit_{i}() int {{\n\treturn {i}\n}}\n\n"));
        }
        let mut ours = base.clone();
        ours = ours.replace(
            "func unit_0() int {\n\treturn 0\n}",
            "func unit_0() int {\n\treturn 100\n}",
        );
        let mut theirs = base.clone();
        theirs = theirs.replace(
            "func unit_1() int {\n\treturn 1\n}",
            "func unit_1() int {\n\treturn 200\n}",
        );
        (base.into_bytes(), ours.into_bytes(), theirs.into_bytes())
    }

    #[test]
    fn large_repo_sparse_dense_equivalence() {
        const UNITS: usize = 2000;
        let (base, ours, theirs) = synthesize_large_go_three_way(UNITS);
        let dense = merge_three_way_dense(MergeLanguage::Go, &base, &ours, &theirs);
        let sparse = merge_three_way(MergeLanguage::Go, &base, &ours, &theirs);
        assert_eq!(
            dense, sparse,
            "large-repo sparse path must match dense merge"
        );
    }

    #[test]
    #[ignore = "manual benchmark; cargo test -p cwso-merge-engine large_repo_merge_prefilter_benchmark --release -- --ignored --nocapture"]
    fn large_repo_merge_prefilter_benchmark() {
        use std::time::{Duration, Instant};

        const UNITS: usize = 2000;
        const ITERS: usize = 15;
        let (base, ours, theirs) = synthesize_large_go_three_way(UNITS);

        let mut dense_samples = Vec::with_capacity(ITERS);
        let mut sparse_samples = Vec::with_capacity(ITERS);
        for _ in 0..ITERS {
            let t0 = Instant::now();
            let _ = merge_three_way_dense(MergeLanguage::Go, &base, &ours, &theirs);
            dense_samples.push(t0.elapsed());

            let t1 = Instant::now();
            let _ = merge_three_way(MergeLanguage::Go, &base, &ours, &theirs);
            sparse_samples.push(t1.elapsed());
        }

        dense_samples.sort();
        sparse_samples.sort();
        let median = |v: &[Duration]| v[v.len() / 2];
        let dense_med = median(&dense_samples);
        let sparse_med = median(&sparse_samples);
        let speedup = dense_med.as_secs_f64() / sparse_med.as_secs_f64();
        eprintln!(
            "large-repo benchmark ({UNITS} units, {ITERS} iters): dense_median={dense_med:?} sparse_median={sparse_med:?} ratio={speedup:.2}x"
        );
        let dense_out =
            merge_three_way_dense(MergeLanguage::Go, &base, &ours, &theirs).expect("dense");
        let sparse_out = merge_three_way(MergeLanguage::Go, &base, &ours, &theirs).expect("sparse");
        assert_eq!(
            dense_out, sparse_out,
            "benchmark fixture must stay sparse≡dense"
        );
    }

    #[test]
    fn sparse_dense_conformance_full_corpus() {
        for case in full_merge_corpus() {
            let base = case.base.as_bytes();
            let ours = case.ours.as_bytes();
            let theirs = case.theirs.as_bytes();

            let dense = merge_three_way_dense(case.lang, base, ours, theirs);
            let sparse = merge_three_way(case.lang, base, ours, theirs);

            assert_eq!(
                dense, sparse,
                "corpus case {}: sparse pre-filter must match dense merge output",
                case.label
            );

            match (&dense, &sparse) {
                (Ok(_), Ok(_)) => {
                    let dense_matrix = merge_conflict_matrix(case.lang, base, ours, theirs, false)
                        .expect("dense matrix");
                    let sparse_matrix = merge_conflict_matrix(case.lang, base, ours, theirs, true)
                        .expect("sparse matrix");
                    assert_eq!(
                        dense_matrix, sparse_matrix,
                        "corpus case {}: conflict matrix must match",
                        case.label
                    );
                }
                (Err(MergeError::SemanticConflict), Err(MergeError::SemanticConflict)) => {}
                _ => panic!(
                    "corpus case {}: dense/sparse error classification diverged",
                    case.label
                ),
            }
        }
    }

    // -- C042: ancestor-based three-way merge + conflict matrix ---------

    /// Acceptance criterion 1: a genuine three-way merge (disjoint edits on
    /// two different top-level functions, resolved against their real
    /// common ancestor `base`) succeeds and produces the exact expected
    /// merged output.
    #[test]
    fn c042_genuine_three_way_merge_succeeds_with_real_output() {
        let base = "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 2\n}\n";
        let ours = "package main\n\nfunc left() int {\n\treturn 10\n}\n\nfunc right() int {\n\treturn 2\n}\n";
        let theirs = "package main\n\nfunc left() int {\n\treturn 1\n}\n\nfunc right() int {\n\treturn 20\n}\n";
        let expected =
            "package main\nfunc left() int {\n\treturn 10\n}\nfunc right() int {\n\treturn 20\n}\n";

        let merged = merge_three_way(
            MergeLanguage::Go,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        )
        .expect("disjoint three-way merge against real common ancestor must succeed");

        assert_eq!(
            merged,
            expected.as_bytes(),
            "merged output must reflect both sides' disjoint edits"
        );
        parse::validate_parse(MergeLanguage::Go, &merged).expect("merged output must parse");
    }

    /// Acceptance criterion 2 (matrix side): an unresolvable merge with
    /// *multiple* independently colliding units returns a conflict matrix
    /// that reports every collision, not just the first (unlike the
    /// short-circuiting `Err` from `merge_three_way` itself).
    #[test]
    fn c042_conflict_matrix_reports_every_colliding_unit() {
        let base = "package main\n\nfunc alpha() int {\n\treturn 1\n}\n\nfunc beta() int {\n\treturn 2\n}\n\nfunc gamma() int {\n\treturn 3\n}\n";
        let ours = "package main\n\nfunc alpha() int {\n\treturn 100\n}\n\nfunc beta() int {\n\treturn 200\n}\n\nfunc gamma() int {\n\treturn 30\n}\n";
        let theirs = "package main\n\nfunc alpha() int {\n\treturn 101\n}\n\nfunc beta() int {\n\treturn 201\n}\n\nfunc gamma() int {\n\treturn 3\n}\n";

        // Sanity: this must actually be a merge failure (alpha and beta
        // diverge; gamma is a disjoint ours-only edit, auto-resolvable).
        let merge_result = merge_three_way(
            MergeLanguage::Go,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        );
        assert_eq!(merge_result, Err(MergeError::SemanticConflict));

        let matrix = conflict_matrix(
            MergeLanguage::Go,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        )
        .expect("matrix computation must succeed on well-formed input");

        let keys: Vec<&str> = matrix.iter().map(|e| e.unit_key.as_str()).collect();
        assert_eq!(
            keys,
            vec![
                "function_declaration:alpha#1",
                "function_declaration:beta#1"
            ],
            "matrix must report both colliding units (alpha, beta) and exclude the \
             disjointly-edited, auto-resolvable gamma"
        );

        for entry in &matrix {
            assert_eq!(entry.reason_code, "both_modified_diverged");
            assert_eq!(entry.ours_state, ConflictState::Modified);
            assert_eq!(entry.theirs_state, ConflictState::Modified);
        }
        assert_eq!(matrix[0].node_kind, "function_declaration");
        assert_eq!(matrix[0].node_name.as_deref(), Some("alpha"));
        assert_eq!(matrix[1].node_name.as_deref(), Some("beta"));
    }

    /// A delete/modify collision (one side removes a unit the other side
    /// edits) is reported with its own distinct reason code.
    #[test]
    fn c042_conflict_matrix_reports_delete_modify_conflict() {
        let base = "fn value() -> i32 {\n    1\n}\n";
        let ours = ""; // ours deletes the only top-level unit
        let theirs = "fn value() -> i32 {\n    2\n}\n"; // theirs modifies it

        let matrix = conflict_matrix(
            MergeLanguage::Rust,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        )
        .expect("matrix computation must succeed");

        assert_eq!(matrix.len(), 1);
        assert_eq!(matrix[0].reason_code, "delete_modify_conflict");
        assert_eq!(matrix[0].ours_state, ConflictState::Deleted);
        assert_eq!(matrix[0].theirs_state, ConflictState::Modified);
        assert_eq!(matrix[0].node_name.as_deref(), Some("value"));
    }

    /// Both sides independently insert a *new*, same-named unit with
    /// diverging bodies -- a collision with no base-side state at all.
    #[test]
    fn c042_conflict_matrix_reports_insertion_diverged() {
        let base = "def keep():\n    return 1\n";
        let ours = "def keep():\n    return 1\n\ndef newFn():\n    return 2\n";
        let theirs = "def keep():\n    return 1\n\ndef newFn():\n    return 3\n";

        let merge_result = merge_three_way(
            MergeLanguage::Python,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        );
        assert_eq!(merge_result, Err(MergeError::SemanticConflict));

        let matrix = conflict_matrix(
            MergeLanguage::Python,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        )
        .expect("matrix computation must succeed");

        assert_eq!(matrix.len(), 1);
        assert_eq!(matrix[0].reason_code, "insertion_diverged");
        assert_eq!(matrix[0].ours_state, ConflictState::Inserted);
        assert_eq!(matrix[0].theirs_state, ConflictState::Inserted);
        assert_eq!(matrix[0].node_name.as_deref(), Some("newFn"));
    }

    /// Acceptance criterion 3: the conflict matrix itself is deterministic
    /// -- repeated runs on identical input produce byte-identical
    /// serialized output, not just an equal-by-construction Rust value.
    #[test]
    fn c042_conflict_matrix_is_deterministic_across_repeated_runs() {
        let base = "package main\n\nfunc alpha() int {\n\treturn 1\n}\n\nfunc beta() int {\n\treturn 2\n}\n";
        let ours = "package main\n\nfunc alpha() int {\n\treturn 10\n}\n\nfunc beta() int {\n\treturn 20\n}\n";
        let theirs = "package main\n\nfunc alpha() int {\n\treturn 11\n}\n\nfunc beta() int {\n\treturn 21\n}\n";

        let first = conflict_matrix(
            MergeLanguage::Go,
            base.as_bytes(),
            ours.as_bytes(),
            theirs.as_bytes(),
        )
        .expect("matrix computation must succeed");
        let first_json = serde_json::to_vec(&first).expect("serialize matrix");
        assert!(!first.is_empty(), "fixture must produce a non-empty matrix");

        for _ in 0..100 {
            let repeat = conflict_matrix(
                MergeLanguage::Go,
                base.as_bytes(),
                ours.as_bytes(),
                theirs.as_bytes(),
            )
            .expect("matrix computation must succeed");
            let repeat_json = serde_json::to_vec(&repeat).expect("serialize matrix");
            assert_eq!(
                repeat_json, first_json,
                "conflict matrix must be byte-identical across repeated runs"
            );
        }
    }

    /// Acceptance criterion 2 (no-corruption side): calling both
    /// `merge_three_way` and `conflict_matrix` on an unresolvable case
    /// never mutates the caller's `base`/`ours`/`theirs` buffers -- the
    /// pre-merge state is provably intact by direct byte comparison
    /// before and after both calls (not merely assumed from Rust's
    /// `&[u8]` immutability).
    #[test]
    fn c042_pre_merge_state_provably_unchanged_after_conflict() {
        let base = b"package main\n\nfunc value() int {\n\treturn 1\n}\n".to_vec();
        let ours = b"package main\n\nfunc value() int {\n\treturn 2\n}\n".to_vec();
        let theirs = b"package main\n\nfunc value() int {\n\treturn 3\n}\n".to_vec();

        let base_before = base.clone();
        let ours_before = ours.clone();
        let theirs_before = theirs.clone();

        let merge_err = merge_three_way(MergeLanguage::Go, &base, &ours, &theirs)
            .expect_err("this fixture must be an unresolvable collision");
        assert_eq!(merge_err, MergeError::SemanticConflict);

        let matrix = conflict_matrix(MergeLanguage::Go, &base, &ours, &theirs)
            .expect("matrix computation must succeed");
        assert_eq!(matrix.len(), 1);
        assert_eq!(matrix[0].reason_code, "both_modified_diverged");

        assert_eq!(
            base, base_before,
            "base buffer must be byte-identical post-conflict"
        );
        assert_eq!(
            ours, ours_before,
            "ours buffer must be byte-identical post-conflict"
        );
        assert_eq!(
            theirs, theirs_before,
            "theirs buffer must be byte-identical post-conflict"
        );
    }
}
