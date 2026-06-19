use std::collections::{BTreeMap, HashMap};

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
}
