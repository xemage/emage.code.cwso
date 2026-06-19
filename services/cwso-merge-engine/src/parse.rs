use anyhow::{anyhow, Context, Result};
use tree_sitter::Parser;

use crate::proto::MergeLanguage;

pub fn supported_languages() -> Vec<&'static str> {
    vec![
        MergeLanguage::Go.as_wire(),
        MergeLanguage::Rust.as_wire(),
        MergeLanguage::Python.as_wire(),
        MergeLanguage::TypeScript.as_wire(),
    ]
}

pub fn validate_parse(language: MergeLanguage, source: &[u8]) -> Result<()> {
    let (_, tree) = parse_tree(language, source)?;

    if tree.root_node().has_error() {
        return Err(anyhow!("tree-sitter parse produced syntax errors"));
    }
    Ok(())
}

pub fn parse_tree(language: MergeLanguage, source: &[u8]) -> Result<(String, tree_sitter::Tree)> {
    let source_text = std::str::from_utf8(source)
        .context("source is not valid UTF-8")?
        .to_string();
    let mut parser = Parser::new();
    parser
        .set_language(&ts_language(language))
        .map_err(|_| anyhow!("failed to set parser language"))?;

    let tree = parser
        .parse(&source_text, None)
        .ok_or_else(|| anyhow!("tree-sitter parse returned no tree"))?;

    Ok((source_text, tree))
}

fn ts_language(language: MergeLanguage) -> tree_sitter::Language {
    match language {
        MergeLanguage::Go => tree_sitter_go::language(),
        MergeLanguage::Rust => tree_sitter_rust::language(),
        MergeLanguage::Python => tree_sitter_python::language(),
        MergeLanguage::TypeScript => tree_sitter_typescript::language_typescript(),
    }
}
