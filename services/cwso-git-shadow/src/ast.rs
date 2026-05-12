//! AST queries via tree-sitter.
//!
//! Phase 2+ supports Go, Python, Rust, and TypeScript (T029: Rust+TS added).
//! Implements the five query types specified in `query_ast`:
//!   find_definition | find_references | extract_signature
//!   | list_exports | detect_entrypoints

use anyhow::{anyhow, Result};
use serde_json::json;
use tree_sitter::{Language, Node, Parser};

#[derive(Copy, Clone, Debug)]
pub enum Lang {
    Go,
    Python,
    Rust,
    TypeScript,
}

pub fn detect_language(path: &str) -> Option<Lang> {
    if path.ends_with(".go") {
        Some(Lang::Go)
    } else if path.ends_with(".py") {
        Some(Lang::Python)
    } else if path.ends_with(".rs") {
        Some(Lang::Rust)
    } else if path.ends_with(".ts") || path.ends_with(".tsx") || path.ends_with(".js") || path.ends_with(".jsx") {
        Some(Lang::TypeScript)
    } else {
        None
    }
}

pub fn supported_languages() -> Vec<&'static str> {
    vec!["go", "python", "rust", "typescript"]
}

fn ts_language(lang: Lang) -> Language {
    match lang {
        Lang::Go => tree_sitter_go::language(),
        Lang::Python => tree_sitter_python::language(),
        Lang::Rust => tree_sitter_rust::language(),
        Lang::TypeScript => tree_sitter_typescript::language_typescript(),
    }
}

/// Definition node kind names per language.
fn definition_kinds(lang: Lang) -> &'static [&'static str] {
    match lang {
        Lang::Go => &[
            "function_declaration",
            "method_declaration",
            "type_declaration",
            "const_declaration",
            "var_declaration",
        ],
        Lang::Python => &["function_definition", "class_definition"],
        Lang::Rust => &[
            "function_item",
            "impl_item",
            "struct_item",
            "enum_item",
            "trait_item",
            "const_item",
            "static_item",
        ],
        Lang::TypeScript => &[
            "function_declaration",
            "class_declaration",
            "interface_declaration",
            "type_alias_declaration",
            "enum_declaration",
            "module",
        ],
    }
}

/// Identifier node kind name per language (for the symbol-name child lookup).
fn name_field(lang: Lang) -> &'static str {
    match lang {
        Lang::Go | Lang::Python | Lang::Rust | Lang::TypeScript => "name",
    }
}

pub fn query(
    lang: Lang,
    src: &[u8],
    query_type: &str,
    target: &str,
) -> Result<serde_json::Value> {
    let mut parser = Parser::new();
    parser
        .set_language(&ts_language(lang))
        .map_err(|e| anyhow!("set_language: {e}"))?;
    let tree = parser
        .parse(src, None)
        .ok_or_else(|| anyhow!("parse failed"))?;
    let root = tree.root_node();

    let mut hits: Vec<serde_json::Value> = Vec::new();

    match query_type {
        "find_definition" => {
            walk(&root, src, &mut |n| {
                if definition_kinds(lang).contains(&n.kind()) {
                    if let Some(name) = node_name(n, src, name_field(lang)) {
                        if name == target {
                            hits.push(node_to_json(n, src));
                        }
                    }
                }
            });
        }
        "find_references" => {
            walk(&root, src, &mut |n| {
                if n.kind() == "identifier" || n.kind() == "type_identifier" {
                    if let Ok(text) = n.utf8_text(src) {
                        if text == target {
                            hits.push(node_to_json(n, src));
                        }
                    }
                }
            });
        }
        "extract_signature" => {
            walk(&root, src, &mut |n| {
                if definition_kinds(lang).contains(&n.kind()) {
                    if let Some(name) = node_name(n, src, name_field(lang)) {
                        if name == target {
                            // For Go: capture the full first line of the declaration.
                            // For Python: include def line through the colon.
                            let snippet = signature_snippet(n, src);
                            hits.push(json!({
                                "kind": n.kind(),
                                "signature": snippet,
                                "start_row": n.start_position().row,
                            }));
                        }
                    }
                }
            });
        }
        "list_exports" => {
            // Heuristic per language. PoC quality.
            walk(&root, src, &mut |n| match lang {
                Lang::Go => {
                    if definition_kinds(lang).contains(&n.kind()) {
                        if let Some(name) = node_name(n, src, name_field(lang)) {
                            if name
                                .chars()
                                .next()
                                .map(|c| c.is_ascii_uppercase())
                                .unwrap_or(false)
                            {
                                hits.push(json!({ "kind": n.kind(), "name": name }));
                            }
                        }
                    }
                }
                Lang::Python => {
                    if definition_kinds(lang).contains(&n.kind()) {
                        if let Some(name) = node_name(n, src, name_field(lang)) {
                            if !name.starts_with('_') {
                                hits.push(json!({ "kind": n.kind(), "name": name }));
                            }
                        }
                    }
                }
                Lang::Rust | Lang::TypeScript => {
                    if definition_kinds(lang).contains(&n.kind()) {
                        if let Some(name) = node_name(n, src, name_field(lang)) {
                            hits.push(json!({ "kind": n.kind(), "name": name }));
                        }
                    }
                }
            });
            // For list_exports, target is ignored.
            let _ = target;
        }
        "detect_entrypoints" => {
            walk(&root, src, &mut |n| match lang {
                Lang::Go => {
                    if n.kind() == "function_declaration" {
                        if let Some(name) = node_name(n, src, name_field(lang)) {
                            if name == "main" {
                                hits.push(node_to_json(n, src));
                            }
                        }
                    }
                }
                Lang::Python | Lang::TypeScript => {
                    // Look for `if __name__ == "__main__":`
                    if n.kind() == "if_statement" {
                        if let Ok(text) = n.utf8_text(src) {
                            if text.contains("__name__") && text.contains("__main__") {
                                hits.push(node_to_json(n, src));
                            }
                        }
                    }
                }
                Lang::Rust => {
                    if n.kind() == "function_item" {
                        if let Some(name) = node_name(n, src, name_field(lang)) {
                            if name == "main" {
                                hits.push(node_to_json(n, src));
                            }
                        }
                    }
                }
            });
            let _ = target;
        }
        other => return Err(anyhow!("unknown query_type: {other}")),
    }

    Ok(json!({
        "language": match lang {
            Lang::Go => "go",
            Lang::Python => "python",
            Lang::Rust => "rust",
            Lang::TypeScript => "typescript",
        },
        "query_type": query_type,
        "target_symbol": target,
        "hits": hits,
    }))
}

fn walk<'a, F: FnMut(&Node<'a>)>(n: &Node<'a>, src: &[u8], cb: &mut F) {
    cb(n);
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        walk(&child, src, cb);
    }
}

fn node_name<'a>(n: &Node<'a>, src: &[u8], field: &str) -> Option<String> {
    let name_node = n.child_by_field_name(field)?;
    name_node.utf8_text(src).ok().map(|s| s.to_string())
}

fn node_to_json(n: &Node<'_>, src: &[u8]) -> serde_json::Value {
    let text = n.utf8_text(src).unwrap_or("");
    let snippet: String = text.chars().take(120).collect();
    json!({
        "kind": n.kind(),
        "start_row": n.start_position().row,
        "start_col": n.start_position().column,
        "end_row": n.end_position().row,
        "end_col": n.end_position().column,
        "snippet": snippet,
    })
}

fn signature_snippet(n: &Node<'_>, src: &[u8]) -> String {
    let text = n.utf8_text(src).unwrap_or("");
    if let Some(idx) = text.find('{') {
        text[..idx].trim().to_string()
    } else if let Some(idx) = text.find(':') {
        text[..=idx].trim().to_string()
    } else {
        text.lines().next().unwrap_or("").to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_find_definition() {
        let src = b"package main\nfunc Hello() string { return \"hi\" }\n";
        let res = query(Lang::Go, src, "find_definition", "Hello").unwrap();
        let hits = res.get("hits").unwrap().as_array().unwrap();
        assert_eq!(hits.len(), 1);
    }

    #[test]
    fn go_extract_signature() {
        let src = b"package main\nfunc Hello(name string) (string, error) { return \"hi\", nil }\n";
        let res = query(Lang::Go, src, "extract_signature", "Hello").unwrap();
        let sig = res["hits"][0]["signature"].as_str().unwrap();
        assert!(sig.contains("Hello"));
        assert!(sig.contains("string"));
    }

    #[test]
    fn python_list_exports() {
        let src = b"def foo():\n    pass\n\ndef _hidden():\n    pass\n\nclass Bar:\n    pass\n";
        let res = query(Lang::Python, src, "list_exports", "").unwrap();
        let names: Vec<&str> = res["hits"]
            .as_array()
            .unwrap()
            .iter()
            .map(|v| v["name"].as_str().unwrap())
            .collect();
        assert!(names.contains(&"foo"));
        assert!(names.contains(&"Bar"));
        assert!(!names.contains(&"_hidden"));
    }

    #[test]
    fn go_detect_entrypoint() {
        let src = b"package main\nfunc main() { println(\"hi\") }\n";
        let res = query(Lang::Go, src, "detect_entrypoints", "").unwrap();
        let hits = res["hits"].as_array().unwrap();
        assert_eq!(hits.len(), 1);
    }
}
