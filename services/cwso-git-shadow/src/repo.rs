//! Shadow workspace store. Backed by a single bare Git repo (libgit2);
//! each shadow workspace is a `git2::Index` populated on demand from blobs.
//!
//! All blobs are written to the bare repo's ODB; nothing touches a working
//! tree. Tear-down clears the in-memory index for the workspace.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use anyhow::{anyhow, Context, Result};
use base64::{engine::general_purpose::STANDARD as B64, Engine as _};
use git2::{IndexEntry, IndexTime, ObjectType, Oid, Repository, RepositoryInitOptions};
use parking_lot::Mutex;
use serde_json::json;
use uuid::Uuid;

use crate::ast;
use crate::proto::{Request, Response};

pub struct ShadowStore {
    repo: Mutex<Repository>,
    workspaces: Mutex<HashMap<Uuid, Workspace>>,
}

struct Workspace {
    /// path within the shadow repo (workspace_uuid → in-memory file map)
    files: HashMap<String, Oid>,
    base_tree: Option<Oid>,
}

impl ShadowStore {
    pub fn new(storage_root: PathBuf) -> Result<Self> {
        std::fs::create_dir_all(&storage_root)?;
        let repo_path = storage_root.join("bare.git");
        let repo = if repo_path.join("HEAD").exists() {
            Repository::open_bare(&repo_path)?
        } else {
            let mut opts = RepositoryInitOptions::new();
            opts.bare(true).initial_head("main");
            Repository::init_opts(&repo_path, &opts)?
        };
        Ok(Self {
            repo: Mutex::new(repo),
            workspaces: Mutex::new(HashMap::new()),
        })
    }

    fn create(&self, base: Option<String>) -> Result<(Uuid, Option<Oid>)> {
        let base_tree = if let Some(sha) = base {
            let oid = Oid::from_str(&sha).with_context(|| format!("bad sha {sha}"))?;
            let repo = self.repo.lock();
            let commit = repo.find_commit(oid)?;
            Some(commit.tree_id())
        } else {
            None
        };
        let id = Uuid::new_v4();
        let mut ws = Workspace {
            files: HashMap::new(),
            base_tree,
        };
        // Seed files map from base tree, if any.
        if let Some(tree_oid) = base_tree {
            let repo = self.repo.lock();
            let tree = repo.find_tree(tree_oid)?;
            tree.walk(git2::TreeWalkMode::PreOrder, |dir, entry| {
                if entry.kind() == Some(ObjectType::Blob) {
                    let name = entry.name().unwrap_or("");
                    let path = if dir.is_empty() {
                        name.to_string()
                    } else {
                        format!("{dir}{name}")
                    };
                    ws.files.insert(path, entry.id());
                }
                git2::TreeWalkResult::Ok
            })?;
        }
        self.workspaces.lock().insert(id, ws);
        Ok((id, base_tree))
    }

    fn drop_workspace(&self, id: Uuid) -> bool {
        self.workspaces.lock().remove(&id).is_some()
    }

    fn write_file(&self, id: Uuid, path: &str, content: &[u8]) -> Result<Oid> {
        check_path(path)?;
        let oid = self.repo.lock().blob(content)?;
        let mut wss = self.workspaces.lock();
        let ws = wss
            .get_mut(&id)
            .ok_or_else(|| anyhow!("no such workspace"))?;
        ws.files.insert(path.to_string(), oid);
        Ok(oid)
    }

    fn read_file(&self, id: Uuid, path: &str) -> Result<Vec<u8>> {
        check_path(path)?;
        let wss = self.workspaces.lock();
        let ws = wss.get(&id).ok_or_else(|| anyhow!("no such workspace"))?;
        let oid = *ws
            .files
            .get(path)
            .ok_or_else(|| anyhow!("path not found in workspace: {path}"))?;
        drop(wss);
        let repo = self.repo.lock();
        let blob = repo.find_blob(oid)?;
        Ok(blob.content().to_vec())
    }

    fn list_files(&self, id: Uuid) -> Result<Vec<String>> {
        let wss = self.workspaces.lock();
        let ws = wss.get(&id).ok_or_else(|| anyhow!("no such workspace"))?;
        let mut out: Vec<String> = ws.files.keys().cloned().collect();
        out.sort();
        Ok(out)
    }

    fn commit(&self, id: Uuid, message: &str) -> Result<(Oid, Oid)> {
        let wss = self.workspaces.lock();
        let ws = wss.get(&id).ok_or_else(|| anyhow!("no such workspace"))?;
        let entries: Vec<(String, Oid)> = ws.files.iter().map(|(k, v)| (k.clone(), *v)).collect();
        drop(wss);

        let repo = self.repo.lock();
        let mut idx = git2::Index::new()?;
        for (path, oid) in &entries {
            let mut entry = IndexEntry {
                ctime: IndexTime::new(0, 0),
                mtime: IndexTime::new(0, 0),
                dev: 0,
                ino: 0,
                mode: 0o100644,
                uid: 0,
                gid: 0,
                file_size: 0,
                id: *oid,
                flags: 0,
                flags_extended: 0,
                path: path.as_bytes().to_vec(),
            };
            // file_size for index hint
            let blob = repo.find_blob(*oid)?;
            entry.file_size = blob.content().len() as u32;
            idx.add(&entry)?;
        }
        let tree_oid = idx.write_tree_to(&repo)?;
        let tree = repo.find_tree(tree_oid)?;
        let sig = git2::Signature::now("cwso-shadow", "shadow@cwso.invalid")?;
        // Parent: workspace base_commit if present? For PoC we orphan-commit.
        let parents: Vec<git2::Commit> = vec![];
        let parent_refs: Vec<&git2::Commit> = parents.iter().collect();
        // POC-DEBT P2-4: orphan commits per workspace; chained history added in T029.
        let commit_oid = repo.commit(None, &sig, &sig, message, &tree, &parent_refs)?;
        Ok((tree_oid, commit_oid))
    }

    fn query_ast(
        &self,
        id: Uuid,
        path: &str,
        query_type: &str,
        target: &str,
    ) -> Result<serde_json::Value> {
        let bytes = self.read_file(id, path)?;
        let lang =
            ast::detect_language(path).ok_or_else(|| anyhow!("unsupported language for {path}"))?;
        ast::query(lang, &bytes, query_type, target)
    }

    fn stat(&self) -> serde_json::Value {
        let wss = self.workspaces.lock();
        json!({
            "workspaces": wss.len(),
            "supported_languages": ast::supported_languages(),
        })
    }
}

fn check_path(p: &str) -> Result<()> {
    if p.is_empty() {
        return Err(anyhow!("empty path"));
    }
    if p.starts_with('/') || p.contains("..") {
        return Err(anyhow!("invalid path: {p}"));
    }
    Ok(())
}

pub fn dispatch(store: &Arc<ShadowStore>, req: Request) -> Response {
    let result: Result<serde_json::Value> = (|| match req {
        Request::Stat => Ok(store.stat()),
        Request::CreateWorkspace { base_commit_sha } => {
            let (id, base_tree) = store.create(base_commit_sha)?;
            Ok(json!({
                "workspace_uuid": id.to_string(),
                "base_tree_oid": base_tree.map(|o| o.to_string()),
            }))
        }
        Request::ListWorkspaces => {
            let wss = store.workspaces.lock();
            let ids: Vec<String> = wss.keys().map(|u| u.to_string()).collect();
            Ok(json!({ "workspace_uuids": ids }))
        }
        Request::DropWorkspace { workspace_uuid } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            Ok(json!({ "dropped": store.drop_workspace(id) }))
        }
        Request::WriteFile {
            workspace_uuid,
            path,
            content_b64,
        } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            let bytes = B64.decode(content_b64.as_bytes())?;
            let oid = store.write_file(id, &path, &bytes)?;
            Ok(json!({ "blob_oid": oid.to_string(), "size": bytes.len() }))
        }
        Request::ReadFile {
            workspace_uuid,
            path,
        } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            let bytes = store.read_file(id, &path)?;
            Ok(json!({ "content_b64": B64.encode(&bytes), "size": bytes.len() }))
        }
        Request::ListFiles { workspace_uuid } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            Ok(json!({ "files": store.list_files(id)? }))
        }
        Request::Commit {
            workspace_uuid,
            message,
        } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            let (tree_oid, commit_oid) = store.commit(id, &message)?;
            Ok(json!({
                "tree_oid": tree_oid.to_string(),
                "commit_oid": commit_oid.to_string(),
            }))
        }
        Request::QueryAst {
            workspace_uuid,
            path,
            query_type,
            target_symbol,
        } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            store.query_ast(id, &path, &query_type, &target_symbol)
        }
    })();

    match result {
        Ok(v) => Response::ok(v),
        Err(e) => Response::error("op_failed", &e.to_string()),
    }
}

// Allow tests inside this module to read internal state.
#[cfg(test)]
mod tests {
    use super::*;

    fn store() -> Arc<ShadowStore> {
        let dir = tempfile::tempdir().unwrap();
        Arc::new(ShadowStore::new(dir.into_path()).unwrap())
    }

    #[test]
    fn create_write_read_roundtrip() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "hello.txt", b"world").unwrap();
        let got = s.read_file(id, "hello.txt").unwrap();
        assert_eq!(got, b"world");
    }

    #[test]
    fn rejects_path_traversal() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        assert!(s.write_file(id, "../etc/passwd", b"x").is_err());
        assert!(s.write_file(id, "/abs/path", b"x").is_err());
    }

    #[test]
    fn parallel_workspaces_are_isolated() {
        let s = store();
        let (a, _) = s.create(None).unwrap();
        let (b, _) = s.create(None).unwrap();
        s.write_file(a, "f.txt", b"AAA").unwrap();
        s.write_file(b, "f.txt", b"BBB").unwrap();
        assert_eq!(s.read_file(a, "f.txt").unwrap(), b"AAA");
        assert_eq!(s.read_file(b, "f.txt").unwrap(), b"BBB");
    }

    #[test]
    fn commit_produces_tree_oid() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "a/b/c.txt", b"hi").unwrap();
        let (tree, commit) = s.commit(id, "test").unwrap();
        assert!(!tree.is_zero());
        assert!(!commit.is_zero());
    }
}

// Suppress unused warnings on _path holders.
#[allow(dead_code)]
fn _ensure_path_use(_p: &Path) {}
