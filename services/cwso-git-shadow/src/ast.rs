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
    } else if path.ends_with(".ts")
        || path.ends_with(".tsx")
        || path.ends_with(".js")
        || path.ends_with(".jsx")
    {
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

pub fn query(lang: Lang, src: &[u8], query_type: &str, target: &str) -> Result<serde_json::Value> {
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
            hits = resolve_references(lang, root, src, target);
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

// --- Scope/binding resolution for `find_references` (C040 / DEBT-REGISTER B6) ---
//
// The `query_ast` wire protocol (`proto::Request::QueryAst`) passes only a bare
// `target_symbol: String` -- no source position -- so a *specific* binding can never be
// selected by the caller. What we CAN do honestly, without guessing, is: build a real
// lexical scope tree (mirroring each grammar's actual scoping rules) and only report an
// identifier occurrence as a reference when it resolves -- via ordinary nearest-enclosing-
// scope lookup -- to *some* real declaration of `target_symbol`. An occurrence whose text
// happens to match but which has no visible binding in its own scope chain (e.g. a
// same-named local variable in an unrelated sibling function) is excluded rather than
// reported, which is exactly the "false positive across shadowed names" class of bug this
// task fixes.
//
// Member/attribute/field/property access (`obj.foo`, `self.x`, `f.Get()`) is a *separate*
// resolution problem: which declaration `foo` binds to depends on the runtime type of the
// receiver, which requires type inference. Type inference is explicitly out of scope for
// v1.0 (single-file scope/binding only, per the C040 brief) and is deferred to v1.1. Rather
// than guess which of several same-named methods on different receiver types a call site
// refers to, those occurrences are excluded ("unresolved") -- documented here and in the
// per-grammar handling below, not silently merged into the wrong receiver's hits. This is
// also why "same method name on different receivers" fixtures deliberately assert on
// *definition* sites only (which are unambiguous: the declared receiver/self type is
// present in the source, not inferred) and never on call-site hits.
//
// Node-kind note (verified against tree-sitter-go 0.21, tree-sitter-rust 0.21,
// tree-sitter-typescript 0.21, tree-sitter-python 0.21 via direct S-expression dumps, not
// assumed): Go's selector field and method name use `field_identifier`, Rust's field/method
// access uses `field_identifier`, and TypeScript's member/property access and method name
// use `property_identifier` -- all distinct from the `identifier` / `type_identifier` kinds
// this resolver walks, so member access is *structurally* excluded there without extra
// logic. Python is the one grammar where attribute access (`attribute` field of an
// `attribute` node) reuses the plain `identifier` kind, so it needs an explicit exclusion
// check (`is_member_access_name`).

/// Node kinds that introduce a new lexical scope frame, per language. Deliberately minimal:
/// enough to model the four fixture scenarios (nested-scope shadowing, method definitions on
/// different receivers, shadowed imports, orphan/out-of-scope references) without attempting
/// full-language scoping fidelity, consistent with "single-file scope/binding only" (C040).
fn is_scope_boundary(lang: Lang, kind: &str) -> bool {
    match lang {
        Lang::Go => matches!(
            kind,
            "source_file"
                | "function_declaration"
                | "method_declaration"
                | "func_literal"
                | "block"
        ),
        Lang::Rust => matches!(
            kind,
            "source_file" | "function_item" | "block" | "closure_expression"
        ),
        Lang::Python => matches!(kind, "module" | "function_definition" | "lambda"),
        Lang::TypeScript => matches!(
            kind,
            "program"
                | "function_declaration"
                | "method_definition"
                | "arrow_function"
                | "statement_block"
        ),
    }
}

/// Whether `n` is a "method-like" definition: bound to a receiver/container type rather than
/// callable as a bare name (you cannot reference `Get` bare, only `receiver.Get()`). Method
/// names must never be added to the *enclosing* lexical scope, or a bare occurrence of the
/// same text elsewhere would spuriously resolve against a method that isn't callable that way.
fn is_method_like(lang: Lang, n: Node<'_>) -> bool {
    match lang {
        Lang::Go => n.kind() == "method_declaration",
        Lang::Rust => {
            n.kind() == "function_item"
                && n.parent()
                    .map(|p| p.kind() == "declaration_list")
                    .unwrap_or(false)
                && n.parent()
                    .and_then(|p| p.parent())
                    .map(|gp| gp.kind() == "impl_item")
                    .unwrap_or(false)
        }
        Lang::Python => {
            n.kind() == "function_definition"
                && n.parent().map(|p| p.kind() == "block").unwrap_or(false)
                && n.parent()
                    .and_then(|p| p.parent())
                    .map(|gp| gp.kind() == "class_definition")
                    .unwrap_or(false)
        }
        Lang::TypeScript => n.kind() == "method_definition",
    }
}

/// The name node of a definition-like node (function/type/class/method), regardless of
/// whether it contributes to the enclosing scope. Definition occurrences are always
/// unambiguous (the declaration itself, not a resolved usage), so they are always reported
/// as hits when their name matches -- never subject to scope-chain resolution.
fn definition_name_node<'a>(lang: Lang, n: Node<'a>) -> Option<Node<'a>> {
    if is_method_like(lang, n) {
        return n.child_by_field_name("name");
    }
    if definition_kinds(lang).contains(&n.kind()) {
        return n.child_by_field_name(name_field(lang));
    }
    None
}

/// Whether a definition's name should be added to the *enclosing* scope's binding table
/// (true for bare-callable functions/types/classes; false for methods -- see `is_method_like`).
fn contributes_to_enclosing_scope(lang: Lang, n: Node<'_>) -> bool {
    definition_name_node(lang, n).is_some() && !is_method_like(lang, n)
}

/// Whether `n` (kind `identifier`) is the attribute/member name on the right of a `.` in
/// Python (`obj.foo`, `self.x`) -- resolving these requires the receiver's type, which is
/// out of scope for v1.0. Go/Rust/TypeScript don't need this check: their member/property
/// access nodes use `field_identifier` / `property_identifier`, already excluded by kind.
fn is_member_access_name(lang: Lang, n: Node<'_>) -> bool {
    if !matches!(lang, Lang::Python) {
        return false;
    }
    if let Some(parent) = n.parent() {
        if parent.kind() == "attribute" {
            if let Some(attr_field) = parent.child_by_field_name("attribute") {
                return attr_field.id() == n.id();
            }
        }
    }
    false
}

/// Recursively collect every `identifier` leaf under `n` into `scope` -- used for binding
/// patterns (`let x = ...`, `x, y := ...`, simple/tuple destructuring) where the bound
/// name(s) are nested under a field rather than being the field node itself.
fn collect_identifier_leaves(
    n: Node<'_>,
    src: &[u8],
    scope: &mut std::collections::HashSet<String>,
) {
    if n.kind() == "identifier" {
        if let Ok(t) = n.utf8_text(src) {
            scope.insert(t.to_string());
        }
        return;
    }
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        collect_identifier_leaves(child, src, scope);
    }
}

fn insert_field_names(
    n: Node<'_>,
    field: &str,
    src: &[u8],
    scope: &mut std::collections::HashSet<String>,
) {
    if let Some(f) = n.child_by_field_name(field) {
        collect_identifier_leaves(f, src, scope);
    }
}

/// Go `var_declaration` / `const_declaration` wrap either a single `var_spec`/`const_spec`
/// or a `var_spec_list`/`const_spec_list` of them; recurse to find all of them either way.
fn go_collect_spec_names(n: Node<'_>, src: &[u8], scope: &mut std::collections::HashSet<String>) {
    if n.kind() == "var_spec" || n.kind() == "const_spec" {
        if let Some(name) = n.child_by_field_name("name") {
            if let Ok(t) = name.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        go_collect_spec_names(child, src, scope);
    }
}

/// Go `import_declaration` wraps `import_spec` (or `import_spec_list` of them). Only
/// explicitly aliased imports are bound here -- an unaliased import's bound name is the
/// last path component of its string literal, which we'd have to parse out of a string
/// rather than read structurally; rather than guess, unaliased imports are left unresolved.
fn go_collect_import_names(n: Node<'_>, src: &[u8], scope: &mut std::collections::HashSet<String>) {
    if n.kind() == "import_spec" {
        if let Some(name) = n.child_by_field_name("name") {
            if let Ok(t) = name.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        go_collect_import_names(child, src, scope);
    }
}

/// The name actually bound by a Rust `use` path fragment: `use_as_clause` binds its alias,
/// a bare `scoped_identifier` binds its final segment, a bare `identifier` binds itself.
/// Glob (`use_wildcard`) and grouped (`use_list`) imports bind an unenumerable/ambiguous set
/// of names and are left unresolved rather than guessed.
fn rust_use_bound_name(n: Node<'_>) -> Option<Node<'_>> {
    match n.kind() {
        "use_as_clause" => n.child_by_field_name("alias"),
        "scoped_identifier" => n.child_by_field_name("name"),
        "identifier" => Some(n),
        _ => None,
    }
}

fn insert_rust_use_binding(n: Node<'_>, src: &[u8], scope: &mut std::collections::HashSet<String>) {
    if let Some(arg) = n.child_by_field_name("argument") {
        if let Some(name_node) = rust_use_bound_name(arg) {
            if let Ok(t) = name_node.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
}

/// The name bound by one child of a Python `import_statement`/`import_from_statement`:
/// `aliased_import` binds its alias; a single-component `dotted_name` binds that component.
/// Multi-component dotted imports (`import a.b`) are left unresolved rather than guessing
/// which component Python actually binds at module scope.
fn python_bound_name_from_import_child(child: Node<'_>) -> Option<Node<'_>> {
    match child.kind() {
        "dotted_name" => {
            if child.child_count() == 1 {
                let c = child.child(0)?;
                if c.kind() == "identifier" {
                    return Some(c);
                }
            }
            None
        }
        "aliased_import" => child.child_by_field_name("alias"),
        _ => None,
    }
}

fn insert_python_import_names(
    n: Node<'_>,
    src: &[u8],
    scope: &mut std::collections::HashSet<String>,
) {
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        if let Some(name_node) = python_bound_name_from_import_child(child) {
            if let Ok(t) = name_node.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
}

fn insert_python_import_from_names(
    n: Node<'_>,
    src: &[u8],
    scope: &mut std::collections::HashSet<String>,
) {
    let module_name_id = n.child_by_field_name("module_name").map(|m| m.id());
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        if Some(child.id()) == module_name_id {
            continue;
        }
        if let Some(name_node) = python_bound_name_from_import_child(child) {
            if let Ok(t) = name_node.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
}

/// TypeScript default import: a bare `identifier` child directly under `import_clause`
/// (distinct from `named_imports`' `import_specifier` children, handled separately).
fn insert_ts_import_clause(n: Node<'_>, src: &[u8], scope: &mut std::collections::HashSet<String>) {
    let mut cursor = n.walk();
    for child in n.children(&mut cursor) {
        if child.kind() == "identifier" {
            if let Ok(t) = child.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
    }
}

fn insert_ts_import_specifier(
    n: Node<'_>,
    src: &[u8],
    scope: &mut std::collections::HashSet<String>,
) {
    let bound = n
        .child_by_field_name("alias")
        .or_else(|| n.child_by_field_name("name"));
    if let Some(name_node) = bound {
        if let Ok(t) = name_node.utf8_text(src) {
            scope.insert(t.to_string());
        }
    }
}

/// Binding rules for one node, per language. Definitions (function/type/class/method) are
/// handled first and uniformly across all four languages via `definition_name_node` /
/// `contributes_to_enclosing_scope`; everything else is language-specific local/parameter/
/// import binding syntax.
fn add_binding_from_node(
    n: Node<'_>,
    src: &[u8],
    lang: Lang,
    scope: &mut std::collections::HashSet<String>,
) {
    if let Some(name_node) = definition_name_node(lang, n) {
        if contributes_to_enclosing_scope(lang, n) {
            if let Ok(t) = name_node.utf8_text(src) {
                scope.insert(t.to_string());
            }
        }
        return;
    }
    match lang {
        Lang::Go => match n.kind() {
            "short_var_declaration" => insert_field_names(n, "left", src, scope),
            "var_declaration" | "const_declaration" => go_collect_spec_names(n, src, scope),
            "import_declaration" => go_collect_import_names(n, src, scope),
            "parameter_declaration" => insert_field_names(n, "name", src, scope),
            _ => {}
        },
        Lang::Rust => match n.kind() {
            "let_declaration" => insert_field_names(n, "pattern", src, scope),
            "use_declaration" => insert_rust_use_binding(n, src, scope),
            "parameter" => insert_field_names(n, "pattern", src, scope),
            "self_parameter" => {
                scope.insert("self".to_string());
            }
            _ => {}
        },
        Lang::Python => match n.kind() {
            "assignment" => insert_field_names(n, "left", src, scope),
            "for_statement" => insert_field_names(n, "left", src, scope),
            "import_statement" => insert_python_import_names(n, src, scope),
            "import_from_statement" => insert_python_import_from_names(n, src, scope),
            _ => {}
        },
        Lang::TypeScript => match n.kind() {
            "variable_declarator" => insert_field_names(n, "name", src, scope),
            "required_parameter" | "optional_parameter" => {
                insert_field_names(n, "pattern", src, scope)
            }
            "import_clause" => insert_ts_import_clause(n, src, scope),
            "import_specifier" => insert_ts_import_specifier(n, src, scope),
            _ => {}
        },
    }
}

/// Collect every binding declared directly within `node`'s own scope -- i.e. recurse into
/// every descendant *except* nested scope-boundary nodes, whose bindings belong to their own
/// (separately pushed) frame, not this one. Called once per scope push.
fn collect_bindings(
    node: Node<'_>,
    src: &[u8],
    lang: Lang,
    scope: &mut std::collections::HashSet<String>,
) {
    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        add_binding_from_node(child, src, lang, scope);
        if !is_scope_boundary(lang, child.kind()) {
            collect_bindings(child, src, lang, scope);
        }
    }
}

/// The core resolver walk: emits a hit for (a) any definition-name occurrence matching
/// `target` (always unambiguous, never scope-gated) and (b) any plain identifier/
/// type_identifier occurrence matching `target` that resolves against a real binding
/// somewhere in the current lexical scope chain. Occurrences with no visible binding, and
/// member/attribute access names, are honestly excluded rather than guessed.
fn resolve_references_walk<'a>(
    node: Node<'a>,
    src: &[u8],
    lang: Lang,
    target: &str,
    scopes: &mut Vec<std::collections::HashSet<String>>,
    hits: &mut Vec<serde_json::Value>,
) {
    let kind = node.kind();
    let def_name = definition_name_node(lang, node);

    if let Some(name_node) = def_name {
        if let Ok(text) = name_node.utf8_text(src) {
            if text == target {
                hits.push(node_to_json(&name_node, src));
            }
        }
    }

    let pushed = is_scope_boundary(lang, kind);
    if pushed {
        scopes.push(std::collections::HashSet::new());
        collect_bindings(node, src, lang, scopes.last_mut().unwrap());
    }

    if def_name.is_none()
        && (kind == "identifier" || kind == "type_identifier")
        && !is_member_access_name(lang, node)
    {
        if let Ok(text) = node.utf8_text(src) {
            if text == target && scopes.iter().any(|s| s.contains(target)) {
                hits.push(node_to_json(&node, src));
            }
        }
    }

    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        if let Some(dn) = def_name {
            if dn.id() == child.id() {
                // Already emitted above as the definition's own name; don't also
                // re-process it as a plain reference occurrence (would double-count).
                continue;
            }
        }
        resolve_references_walk(child, src, lang, target, scopes, hits);
    }

    if pushed {
        scopes.pop();
    }
}

fn resolve_references(
    lang: Lang,
    root: Node<'_>,
    src: &[u8],
    target: &str,
) -> Vec<serde_json::Value> {
    let mut scopes: Vec<std::collections::HashSet<String>> = Vec::new();
    let mut hits = Vec::new();
    resolve_references_walk(root, src, lang, target, &mut scopes, &mut hits);
    hits
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

    #[test]
    fn rust_find_definition() {
        let src = b"fn helper() {}\nfn main() {}\n";
        let res = query(Lang::Rust, src, "find_definition", "main").unwrap();
        let hits = res["hits"].as_array().unwrap();
        assert_eq!(hits.len(), 1);
    }

    #[test]
    fn typescript_find_definition() {
        let src = b"export function greet(name: string): string { return `hi ${name}`; }\n";
        let res = query(Lang::TypeScript, src, "find_definition", "greet").unwrap();
        let hits = res["hits"].as_array().unwrap();
        assert_eq!(hits.len(), 1);
    }

    // --- C040 shadowed-name fixture set: find_references scope/binding resolution ---
    //
    // Each fixture below covers, per grammar: (1) an identifier orphaned outside the scope
    // of its only declaration -- the concrete "false positive" the old text-match algorithm
    // produced and this resolver excludes; (2) the same identifier declared in nested
    // scopes, with every legitimate occurrence still resolving (no false negatives either);
    // (3) the same method name defined on two different receiver/container types, proving
    // definitions are reported (unambiguous) while call sites are not guessed at either
    // receiver; (4) an imported name shadowed by a local re-import/re-declaration.

    fn row_of(src: &str, needle: &str, occurrence: usize) -> u64 {
        let idx = src
            .match_indices(needle)
            .nth(occurrence)
            .unwrap_or_else(|| {
                panic!("substring {needle:?} occurrence {occurrence} not found in fixture")
            })
            .0;
        src[..idx].matches('\n').count() as u64
    }

    fn hit_rows(res: &serde_json::Value) -> Vec<u64> {
        res["hits"]
            .as_array()
            .unwrap()
            .iter()
            .map(|h| h["start_row"].as_u64().unwrap())
            .collect()
    }

    const GO_SHADOW_FIXTURE: &str = r#"package main

import shadowedimp "fmt"

func useLocalOnly() int {
	orphan := 5
	return orphan
}

func readsUnrelatedOrphan() int {
	return orphan
}

func shadowExample() int {
	shadow := 1
	{
		shadow := 2
		return shadow
	}
	return shadow
}

type Foo struct{}

func (f Foo) Get() int { return 1 }

type Bar struct{}

func (b Bar) Get() int { return 2 }

func callsBothReceivers() int {
	f := Foo{}
	b := Bar{}
	return f.Get() + b.Get()
}

func usesImportAlias() {
	shadowedimp.Println("x")
}

func localShadowsImport() {
	shadowedimp := 42
	_ = shadowedimp
}
"#;

    #[test]
    fn go_find_references_excludes_orphan_reference_outside_declaring_scope() {
        let res = query(
            Lang::Go,
            GO_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "orphan",
        )
        .unwrap();
        // Old text-match returned 3 hits (decl + 2 uses). Scope resolution must exclude the
        // use in readsUnrelatedOrphan, where `orphan` has no visible binding at all.
        assert_eq!(res["hits"].as_array().unwrap().len(), 2);
        let excluded_row = row_of(GO_SHADOW_FIXTURE, "return orphan", 1);
        assert!(!hit_rows(&res).contains(&excluded_row));
    }

    #[test]
    fn go_find_references_resolves_nested_shadowed_variable_without_conflation() {
        let res = query(
            Lang::Go,
            GO_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "shadow",
        )
        .unwrap();
        // outer decl, inner decl, inner use, outer use -- all four are legitimate, real
        // occurrences (no orphans here), so none should be dropped by the resolver either.
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }

    #[test]
    fn go_find_references_method_definitions_on_different_receivers_not_conflated_with_calls() {
        let res = query(
            Lang::Go,
            GO_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "Get",
        )
        .unwrap();
        let hits = res["hits"].as_array().unwrap();
        // Both definitions (Foo.Get, Bar.Get) are unambiguous declarations, so both are
        // reported. Call sites (f.Get(), b.Get()) use Go's `field_identifier` kind, distinct
        // from `identifier`/`type_identifier`; resolving them to the correct receiver would
        // need type inference (out of scope for v1.0), so they are excluded, not guessed.
        assert_eq!(hits.len(), 2);
        for h in hits {
            assert_eq!(h["kind"], "field_identifier");
        }
    }

    #[test]
    fn go_find_references_local_variable_shadows_import_alias_within_its_scope() {
        let res = query(
            Lang::Go,
            GO_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "shadowedimp",
        )
        .unwrap();
        // usesImportAlias's selector use (resolves against the file-level import) + the
        // local `shadowedimp := 42` decl and its use inside localShadowsImport (resolves
        // against the local binding, which shadows the import within that function only).
        // The import declaration itself uses Go's `package_identifier` kind, not
        // `identifier`, so it does not produce a hit of its own (parity with old behavior).
        assert_eq!(res["hits"].as_array().unwrap().len(), 3);
    }

    const RUST_SHADOW_FIXTURE: &str = r#"fn use_local_only() -> i32 {
    let orphan = 5;
    orphan
}

fn reads_unrelated_orphan() -> i32 {
    return orphan;
}

fn shadow_example() -> i32 {
    let shadow = 1;
    {
        let shadow = 2;
        return shadow;
    }
    shadow
}

struct Foo;
impl Foo {
    fn get(&self) -> i32 { 1 }
}

struct Bar;
impl Bar {
    fn get(&self) -> i32 { 2 }
}

fn calls_both_receivers() -> i32 {
    let f = Foo;
    let b = Bar;
    f.get() + b.get()
}

use std::collections::HashMap as Map;

fn uses_import_alias() -> Map<i32, i32> {
    Map::new()
}

fn local_shadows_import() -> i32 {
    let Map = 42;
    Map
}
"#;

    #[test]
    fn rust_find_references_excludes_orphan_reference_outside_declaring_scope() {
        let res = query(
            Lang::Rust,
            RUST_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "orphan",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 2);
        let excluded_row = row_of(RUST_SHADOW_FIXTURE, "return orphan", 0);
        assert!(!hit_rows(&res).contains(&excluded_row));
    }

    #[test]
    fn rust_find_references_resolves_nested_shadowed_variable_without_conflation() {
        let res = query(
            Lang::Rust,
            RUST_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "shadow",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }

    #[test]
    fn rust_find_references_method_definitions_on_different_receivers_not_conflated_with_calls() {
        let res = query(
            Lang::Rust,
            RUST_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "get",
        )
        .unwrap();
        let hits = res["hits"].as_array().unwrap();
        // Both `fn get` definitions (impl Foo, impl Bar) are unambiguous, so both are
        // reported. Call sites (f.get(), b.get()) use Rust's `field_expression` /
        // `field_identifier`, distinct from plain `identifier`; resolving them to the
        // correct receiver would need type inference (out of scope for v1.0), so they are
        // excluded, not guessed.
        assert_eq!(hits.len(), 2);
        for h in hits {
            assert_eq!(h["kind"], "identifier");
        }
    }

    #[test]
    fn rust_find_references_local_binding_shadows_use_alias_within_its_scope() {
        let res = query(
            Lang::Rust,
            RUST_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "Map",
        )
        .unwrap();
        // `use ... as Map` decl (1) + uses_import_alias's return-type and `Map::new()` path
        // (2, both resolve against the module-level `use` alias) + local_shadows_import's
        // `let Map = 42;` decl and its trailing use (2, resolve against the local binding,
        // which shadows the `use` alias within that function only) = 5.
        assert_eq!(res["hits"].as_array().unwrap().len(), 5);
    }

    const PYTHON_SHADOW_FIXTURE: &str = "def use_local_only():\n    orphan = 5\n    return orphan\n\n\ndef reads_unrelated_orphan():\n    return orphan\n\n\ndef shadow_example():\n    shadow = 1\n    def inner():\n        shadow = 2\n        return shadow\n    return shadow\n\n\nclass A:\n    def foo(self):\n        return self.x\n\n\nclass B:\n    def foo(self):\n        return self.x\n\n\ndef calls_both_receivers():\n    a = A()\n    b = B()\n    return a.foo() + b.foo()\n\n\nimport os\n\n\ndef uses_import():\n    return os\n\n\ndef local_shadows_import():\n    import sys as os\n    return os\n";

    #[test]
    fn python_find_references_excludes_orphan_reference_outside_declaring_scope() {
        let res = query(
            Lang::Python,
            PYTHON_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "orphan",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 2);
        let excluded_row = row_of(PYTHON_SHADOW_FIXTURE, "return orphan", 1);
        assert!(!hit_rows(&res).contains(&excluded_row));
    }

    #[test]
    fn python_find_references_resolves_nested_shadowed_variable_without_conflation() {
        let res = query(
            Lang::Python,
            PYTHON_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "shadow",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }

    #[test]
    fn python_find_references_method_definitions_on_different_receivers_not_conflated_with_calls() {
        let res = query(
            Lang::Python,
            PYTHON_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "foo",
        )
        .unwrap();
        let hits = res["hits"].as_array().unwrap();
        // Both `def foo` definitions (class A, class B) are unambiguous, so both are
        // reported. Call sites (a.foo(), b.foo()) are the `attribute` field of an
        // `attribute` node -- Python is the one grammar where member access reuses the
        // plain `identifier` kind, so it needs the explicit `is_member_access_name` check
        // rather than a kind-based exclusion. Resolving them to the correct receiver would
        // need type inference (out of scope for v1.0), so they are excluded, not guessed.
        assert_eq!(hits.len(), 2);
    }

    #[test]
    fn python_find_references_attribute_access_is_never_guessed() {
        // `self.x` appears in both class A's and class B's `foo` -- if attribute access were
        // (incorrectly) resolved like a plain lexical reference, it would false-positive
        // against nothing in particular. There is no lexically-scoped `x` anywhere in this
        // fixture, so the honest answer is zero hits, not a guess at either receiver's field.
        let res = query(
            Lang::Python,
            PYTHON_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "x",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 0);
    }

    #[test]
    fn python_find_references_local_import_alias_shadows_module_import_within_its_scope() {
        let res = query(
            Lang::Python,
            PYTHON_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "os",
        )
        .unwrap();
        // module-level `import os` (1, the dotted_name identifier itself resolves against
        // the scope it just populated) + uses_import's `return os` (1, resolves against the
        // module import) + local_shadows_import's `import sys as os` alias decl and its
        // `return os` (2, resolve against the local, function-scoped import that shadows the
        // module-level one only within that function) = 4.
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }

    const TS_SHADOW_FIXTURE: &str = r#"function useLocalOnly(): number {
    const orphan = 5;
    return orphan;
}

function readsUnrelatedOrphan(): number {
    return orphan;
}

function shadowExample(): number {
    let shadow = 1;
    function inner(): number {
        let shadow = 2;
        return shadow;
    }
    return shadow;
}

class A {
    get(): number { return 1; }
}

class B {
    get(): number { return 2; }
}

function callsBothReceivers(): number {
    const a = new A();
    const b = new B();
    return a.get() + b.get();
}

import { foo as importedAlias } from "./mod";

function usesImportAlias(): void {
    console.log(importedAlias);
}

function localShadowsImport(): void {
    const importedAlias = 42;
    console.log(importedAlias);
}
"#;

    #[test]
    fn typescript_find_references_excludes_orphan_reference_outside_declaring_scope() {
        let res = query(
            Lang::TypeScript,
            TS_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "orphan",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 2);
        let excluded_row = row_of(TS_SHADOW_FIXTURE, "return orphan", 1);
        assert!(!hit_rows(&res).contains(&excluded_row));
    }

    #[test]
    fn typescript_find_references_resolves_nested_shadowed_variable_without_conflation() {
        let res = query(
            Lang::TypeScript,
            TS_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "shadow",
        )
        .unwrap();
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }

    #[test]
    fn typescript_find_references_method_definitions_on_different_receivers_not_conflated_with_calls(
    ) {
        let res = query(
            Lang::TypeScript,
            TS_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "get",
        )
        .unwrap();
        let hits = res["hits"].as_array().unwrap();
        // Both `get()` method definitions (class A, class B) are unambiguous, so both are
        // reported. Call sites (a.get(), b.get()) use TypeScript's `member_expression` /
        // `property_identifier`, distinct from plain `identifier`; resolving them to the
        // correct receiver would need type inference (out of scope for v1.0), so they are
        // excluded, not guessed.
        assert_eq!(hits.len(), 2);
        for h in hits {
            assert_eq!(h["kind"], "property_identifier");
        }
    }

    #[test]
    fn typescript_find_references_local_const_shadows_import_alias_within_its_scope() {
        let res = query(
            Lang::TypeScript,
            TS_SHADOW_FIXTURE.as_bytes(),
            "find_references",
            "importedAlias",
        )
        .unwrap();
        // `import { foo as importedAlias }` decl (1) + usesImportAlias's usage (1, resolves
        // against the module-level import alias) + localShadowsImport's `const
        // importedAlias = 42;` decl and its usage (2, resolve against the local binding,
        // which shadows the import alias within that function only) = 4.
        assert_eq!(res["hits"].as_array().unwrap().len(), 4);
    }
}
