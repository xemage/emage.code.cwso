//! T728 (SEC-T696-07 a): pins the explicit wasmtime feature list.
//!
//! `cwso-sparse/Cargo.toml` turns wasmtime's default features off and lists
//! them by hand, minus `cache` and `profiling` (see the comment there). With
//! `default-features = false` a feature that a newer wasmtime adds to its
//! defaults is silently not enabled, and a feature it renames or drops goes
//! unnoticed. These tests make every wasmtime bump re-diff the list:
//!
//! * `wasmtime_version_is_the_reviewed_one` fails on any change of the locked
//!   wasmtime version until `REVIEWED_VERSION` is raised, which is the moment
//!   to repeat the review below.
//! * `explicit_features_are_upstream_defaults_minus_excluded` reads the
//!   locked wasmtime's own `default` feature set through `cargo metadata` and
//!   fails unless the explicit list equals it minus `EXCLUDED`, and every
//!   excluded name is still an upstream default.
//!
//! Review on a bump: compare the new `default` set with the list, decide for
//! each new feature whether to enable it, update `Cargo.toml` and then
//! `REVIEWED_VERSION`.
//!
//! (SEC-T696-07 b, fuel or epoch interruption, stays tracked: required before
//! untrusted modules are accepted; not changed here.)

use std::collections::BTreeSet;
use std::process::Command;

/// The wasmtime version whose default feature set was last reviewed.
const REVIEWED_VERSION: &str = "36.0.20";

/// Upstream default features deliberately left out (reasons in Cargo.toml).
const EXCLUDED: [&str; 2] = ["cache", "profiling"];

fn metadata() -> serde_json::Value {
    let cargo = std::env::var("CARGO").unwrap_or_else(|_| "cargo".to_string());
    let manifest = concat!(env!("CARGO_MANIFEST_DIR"), "/Cargo.toml");
    let out = Command::new(cargo)
        .args(["metadata", "--format-version", "1", "--locked"])
        .args(["--manifest-path", manifest])
        .output()
        .expect("run cargo metadata");
    assert!(
        out.status.success(),
        "cargo metadata failed: {}",
        String::from_utf8_lossy(&out.stderr)
    );
    serde_json::from_slice(&out.stdout).expect("parse cargo metadata")
}

fn package<'a>(meta: &'a serde_json::Value, name: &str) -> &'a serde_json::Value {
    let found: Vec<_> = meta["packages"]
        .as_array()
        .expect("packages array")
        .iter()
        .filter(|p| p["name"] == name)
        .collect();
    assert_eq!(found.len(), 1, "expected exactly one `{name}` package");
    found[0]
}

fn names(value: &serde_json::Value) -> BTreeSet<String> {
    value
        .as_array()
        .expect("array of feature names")
        .iter()
        .map(|v| v.as_str().expect("feature name").to_string())
        .collect()
}

/// (uses_default_features, explicit features) of cwso-sparse's wasmtime dep.
fn declared(meta: &serde_json::Value) -> (bool, BTreeSet<String>) {
    let deps = package(meta, "cwso-sparse")["dependencies"]
        .as_array()
        .expect("dependencies array");
    let dep = deps
        .iter()
        .find(|d| d["name"] == "wasmtime")
        .expect("cwso-sparse depends on wasmtime");
    let defaults = dep["uses_default_features"].as_bool().expect("bool");
    (defaults, names(&dep["features"]))
}

#[test]
fn wasmtime_version_is_the_reviewed_one() {
    let meta = metadata();
    let locked = package(&meta, "wasmtime")["version"]
        .as_str()
        .expect("version")
        .to_string();
    assert_eq!(
        locked, REVIEWED_VERSION,
        "wasmtime changed to {locked}: re-diff its default features against the \
         explicit list in cwso-sparse/Cargo.toml, then raise REVIEWED_VERSION"
    );
}

#[test]
fn explicit_features_are_upstream_defaults_minus_excluded() {
    let meta = metadata();
    let upstream = names(&package(&meta, "wasmtime")["features"]["default"]);
    let (uses_defaults, explicit) = declared(&meta);
    assert!(
        !uses_defaults,
        "wasmtime must keep default-features = false"
    );
    for name in EXCLUDED {
        assert!(
            upstream.contains(name),
            "`{name}` is no longer a wasmtime default; review EXCLUDED"
        );
    }
    let expected: BTreeSet<String> = upstream
        .iter()
        .filter(|f| !EXCLUDED.contains(&f.as_str()))
        .cloned()
        .collect();
    assert_eq!(
        explicit, expected,
        "explicit wasmtime features differ from upstream defaults minus {EXCLUDED:?}"
    );
}
