use std::collections::{BTreeMap, HashMap};

use thiserror::Error;
use tree_sitter::Node;

use crate::parse;
use crate::proto::MergeLanguage;

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

    let base_order: Vec<String> = base_units.iter().map(|u| u.key.clone()).collect();
    let ours_diff = build_side_diff(&base_units, &ours_units);
    let theirs_diff = build_side_diff(&base_units, &theirs_units);
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

fn build_side_diff(base_units: &[AstUnit], side_units: &[AstUnit]) -> SideDiff {
    let mut side_map: HashMap<String, Vec<u8>> = HashMap::new();
    for unit in side_units {
        side_map.insert(unit.key.clone(), unit.text.clone());
    }

    let mut states = HashMap::new();
    for base in base_units {
        match side_map.get(&base.key) {
            None => {
                states.insert(base.key.clone(), NodeState::Deleted);
            }
            Some(side_text) if *side_text == base.text => {
                states.insert(base.key.clone(), NodeState::Unchanged);
            }
            Some(side_text) => {
                states.insert(base.key.clone(), NodeState::Modified(side_text.clone()));
            }
        }
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
}
