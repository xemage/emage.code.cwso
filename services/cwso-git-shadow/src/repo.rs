//! Shadow workspace store. Backed by a single bare Git repo (libgit2);
//! each shadow workspace is a `git2::Index` populated on demand from blobs.
//!
//! All blobs are written to the bare repo's ODB. In addition (C021, ADR-012
//! "materialise-to-tmpfs"), every shadow workspace is eagerly materialized
//! onto a real, tmpfs-backed directory (`<storage_root>/<workspace-uuid>/`)
//! at creation time, and every subsequent `write_file` keeps that real path
//! in sync — so ordinary tools (`ls`, `cat`, `pytest`, ...) can reach a
//! shadow workspace's files without going through this process's IPC at
//! all. Tear-down clears the in-memory index for the workspace *and* removes
//! its projected directory. See `materialize_write` for how these real
//! filesystem writes are hardened against a hostile `path` string.

use std::collections::HashMap;
use std::ffi::{CString, OsStr};
use std::io::Write as _;
use std::os::fd::{AsRawFd, FromRawFd, OwnedFd};
use std::os::unix::ffi::OsStrExt;
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
    /// Root of the tmpfs-backed projection tree; each workspace gets
    /// `storage_root/<uuid>/` (see `workspace_dir`).
    storage_root: PathBuf,
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
            storage_root,
        })
    }

    /// Deterministic real, tmpfs-backed path for a workspace's projection:
    /// `<storage_root>/<workspace-uuid>/`. `Uuid::to_string()` only ever
    /// produces hyphenated lowercase hex, so this is not itself
    /// attacker-controlled input (unlike the file `path` strings handled by
    /// `materialize_write`).
    fn workspace_dir(&self, id: Uuid) -> PathBuf {
        self.storage_root.join(id.to_string())
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
        let ws_dir = self.workspace_dir(id);
        // ADR-012: eager materialisation at creation time, not lazy-on-open.
        // Create the projection directory even for a base-less workspace, so
        // a real, listable path exists from the moment `create` returns.
        std::fs::create_dir_all(&ws_dir)
            .with_context(|| format!("create workspace projection dir {ws_dir:?}"))?;
        let mut ws = Workspace {
            files: HashMap::new(),
            base_tree,
        };
        // Seed files map from base tree, if any, materializing each blob to
        // the real workspace directory as we go.
        if let Some(tree_oid) = base_tree {
            let repo = self.repo.lock();
            let tree = repo.find_tree(tree_oid)?;
            let mut walk_err: Option<anyhow::Error> = None;
            tree.walk(git2::TreeWalkMode::PreOrder, |dir, entry| {
                if entry.kind() != Some(ObjectType::Blob) {
                    return git2::TreeWalkResult::Ok;
                }
                let name = entry.name().unwrap_or("");
                let path = if dir.is_empty() {
                    name.to_string()
                } else {
                    format!("{dir}{name}")
                };
                let blob = match repo.find_blob(entry.id()) {
                    Ok(b) => b,
                    Err(e) => {
                        walk_err = Some(e.into());
                        return git2::TreeWalkResult::Abort;
                    }
                };
                // Base-tree paths were themselves already validated by
                // `check_path`/`materialize_write` when they were originally
                // written via `write_file`/`commit` -- but a base tree can in
                // principle be any commit's tree (not only one this store
                // produced), so re-validate here rather than assume that
                // history holds for every tree object we might be asked to
                // seed from.
                if let Err(e) = materialize_write(&ws_dir, &path, blob.content()) {
                    walk_err = Some(e);
                    return git2::TreeWalkResult::Abort;
                }
                ws.files.insert(path, entry.id());
                git2::TreeWalkResult::Ok
            })?;
            if let Some(e) = walk_err {
                return Err(e);
            }
        }
        self.workspaces.lock().insert(id, ws);
        Ok((id, base_tree))
    }

    fn drop_workspace(&self, id: Uuid) -> bool {
        let existed = self.workspaces.lock().remove(&id).is_some();
        // Remove the real, on-disk projection regardless of whether the
        // in-memory entry was found, so no orphaned directory can survive a
        // drop even if the two ever fall out of sync.
        let ws_dir = self.workspace_dir(id);
        match std::fs::remove_dir_all(&ws_dir) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => {
                tracing::warn!(
                    workspace = %id,
                    path = ?ws_dir,
                    error = %e,
                    "failed to remove shadow workspace projection directory"
                );
            }
        }
        existed
    }

    fn write_file(&self, id: Uuid, path: &str, content: &[u8]) -> Result<Oid> {
        check_path(path)?;
        // Fail fast on an unknown workspace before doing any filesystem
        // work below (avoids materializing a real directory tree for a
        // workspace id that was never created).
        if !self.workspaces.lock().contains_key(&id) {
            return Err(anyhow!("no such workspace"));
        }
        // Materialize to the real path *before* mutating the in-memory
        // blob-store state: if this fails (e.g. a path-safety rejection),
        // the two views of the workspace must not diverge -- a partial
        // success here (map updated, disk not) would be exactly the kind of
        // silent inconsistency this projection exists to avoid.
        materialize_write(&self.workspace_dir(id), path, content)?;
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

    fn workspace_meta(&self, id: Uuid) -> Result<serde_json::Value> {
        let wss = self.workspaces.lock();
        let ws = wss.get(&id).ok_or_else(|| anyhow!("no such workspace"))?;
        let mut files: Vec<serde_json::Value> = ws
            .files
            .iter()
            .map(|(path, oid)| {
                json!({
                    "path": path,
                    "blob_oid": oid.to_string(),
                })
            })
            .collect();
        files.sort_by(|a, b| {
            a["path"]
                .as_str()
                .unwrap_or_default()
                .cmp(b["path"].as_str().unwrap_or_default())
        });
        Ok(json!({
            "base_tree_oid": ws.base_tree.map(|o| o.to_string()),
            "files": files,
        }))
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

/// Extends `check_path`'s virtual-path check with the component-level
/// validation a *real filesystem write* needs. `check_path` alone was
/// written to gate an in-memory `HashMap<String, Oid>` key (reject a leading
/// `/` and any `..` substring) -- adequate for that purpose, since a
/// malformed key at worst produces a wrong/missing map entry, never a
/// filesystem escape. `materialize_write` (the only caller of this
/// function) turns `path` into a real path passed to `create_dir_all`/
/// `open`, so "reject the `..` substring" is not by itself a sufficient
/// guarantee -- e.g. it says nothing about empty components, `.`
/// components, or (defensively, since git2 already forbids them in trees it
/// builds) an absolute-looking component embedded past the first character.
/// This function re-parses `p` component-by-component and requires every
/// component to be a plain, non-empty name.
fn secure_relative_path(p: &str) -> Result<PathBuf> {
    check_path(p)?;
    let mut out = PathBuf::new();
    for component in Path::new(p).components() {
        match component {
            std::path::Component::Normal(part) if !part.is_empty() => out.push(part),
            other => {
                return Err(anyhow!("invalid path component ({other:?}) in {p}"));
            }
        }
    }
    if out.as_os_str().is_empty() {
        return Err(anyhow!("empty path"));
    }
    Ok(out)
}

/// Writes `content` to `<workspace_dir>/<rel_path>` on the real,
/// tmpfs-backed filesystem, treating `rel_path` as adversarial input
/// end-to-end (it comes straight off the IPC socket via `write_file`, or
/// indirectly off a base commit's tree via `create`). This is the first
/// code in this service that turns a caller-supplied path into a real
/// filesystem write, so it applies the same principles as
/// `orchestrator/internal/tools/fs_tools.go`'s `pathGuard`/
/// `secureResolveDirs`/`secureOpenLeaf` (T193/T194): validate every
/// component up front, then perform the actual filesystem walk itself in a
/// way that cannot be raced -- not merely checked once and trusted.
///
/// Defense layers, each independently sufficient against the attack class
/// it targets:
///
///  1. `secure_relative_path` rejects `..`, absolute paths, `.` components,
///     and empty components before anything touches disk.
///  2. The actual directory walk and final write (`materialize_write_via_fd_walk`)
///     is fd-anchored: `workspace_dir` is opened once by name (the one
///     trust-anchor lookup -- see that function's doc comment), and every
///     subsequent hop, including the final leaf, is opened with
///     `openat`/`O_NOFOLLOW` relative to the fd returned by the
///     *immediately preceding* hop. There is deliberately no longer a
///     separate "canonicalize once, then open a path string built from the
///     canonicalized result" step: that pattern (this function's
///     pre-SEC-001-fix shape) re-resolves the canonicalized path a second
///     time, by name, from the filesystem root at `open()` time -- a
///     completely independent lookup from the one the canonicalize/
///     containment check just validated, with a real gap in between for a
///     concurrent filesystem change (e.g. an intermediate directory
///     component swapped for a symlink) to land in. Anchoring every hop to
///     an already-open fd removes that second, independently-timed lookup
///     entirely, so there is no window left for such a race to win in --
///     this is a *stronger* guarantee than the removed
///     canonicalize-then-`starts_with` check provided, not a weaker one
///     with the check simply deleted. See `materialize_write_via_fd_walk`
///     for the walk itself.
///
/// Git tree entries can carry symlink mode (`120000`); this function never
/// creates a real OS symlink regardless of the source blob's mode --
/// `content` is always the raw blob bytes written into a plain regular
/// file. A base-tree symlink therefore materializes as an inert text file
/// containing the link-target string, not a working symlink. This is a
/// deliberate, documented limitation of this projection (not an oversight):
/// it removes "a hostile base tree plants a symlink to escape the workspace
/// root" as an attack class entirely, at zero functional cost within this
/// task's read-side-projection scope.
fn materialize_write(workspace_dir: &Path, rel_path: &str, content: &[u8]) -> Result<()> {
    let rel = secure_relative_path(rel_path)?;
    std::fs::create_dir_all(workspace_dir)
        .with_context(|| format!("create workspace root {workspace_dir:?}"))?;
    materialize_write_via_fd_walk(workspace_dir, &rel, content)
        .with_context(|| format!("materialize write to {rel_path:?} under {workspace_dir:?}"))
}

/// Converts a single path component into a NUL-terminated `CString` for use
/// with the raw `openat`/`mkdirat` calls below. `secure_relative_path` has
/// already rejected empty, `.`, and `..` components; this only additionally
/// guards against an embedded NUL byte, which `Path`'s `Normal` component
/// variant does not itself rule out on all platforms.
fn cstring_component(part: &OsStr) -> Result<CString> {
    CString::new(part.as_bytes())
        .map_err(|_| anyhow!("path component contains a NUL byte: {part:?}"))
}

/// Opens `dir` as an `O_DIRECTORY` fd by name. This is the *only*
/// name-based (i.e. not fd-anchored) lookup in the whole write path: it
/// opens `workspace_dir` itself, a `<storage_root>/<workspace-uuid>/` path
/// that this process created and whose UUID component is not
/// attacker-controlled (see `ShadowStore::workspace_dir`'s doc comment).
/// `O_NOFOLLOW` is intentionally not applied here, mirroring
/// `fs_tools.go`'s `rootFd` open for the same reason: the root is the
/// operator/process-controlled trust anchor the whole scheme is anchored
/// to, not attacker-supplied input -- only its descendants (via
/// `rel_path`) are.
fn open_root_dir(dir: &Path) -> Result<OwnedFd> {
    let c = CString::new(dir.as_os_str().as_bytes())
        .map_err(|_| anyhow!("workspace root path contains a NUL byte: {dir:?}"))?;
    let fd = unsafe { libc::open(c.as_ptr(), libc::O_RDONLY | libc::O_DIRECTORY | libc::O_CLOEXEC) };
    if fd < 0 {
        return Err(std::io::Error::last_os_error())
            .with_context(|| format!("open workspace root {dir:?}"));
    }
    Ok(unsafe { OwnedFd::from_raw_fd(fd) })
}

/// Opens `component` as a subdirectory of `parent`, anchored entirely to
/// `parent`'s already-open fd -- never a fresh, name-based lookup from
/// `workspace_dir` or the filesystem root. `O_NOFOLLOW` means that if
/// `component` currently names a symlink -- whether pre-existing or swapped
/// in by a concurrent filesystem operation after some earlier, separately
/// timed check -- the kernel refuses this exact hop atomically (`ELOOP`, or
/// `ENOTDIR` for the `O_DIRECTORY|O_NOFOLLOW` combination against a
/// symlink) instead of silently following it. If `create` is true and the
/// component does not yet exist, `mkdirat` creates it (tolerating a benign
/// `EEXIST` raced against another creator) and the open is retried exactly
/// once; if what's there afterwards still is not a plain, non-symlink
/// directory, the retried `O_NOFOLLOW|O_DIRECTORY` open still correctly
/// rejects it. This -- and `openat_leaf_nofollow` below -- is what closes
/// SEC-001: every hop the write actually takes is committed to an fd
/// obtained from the previous hop, so there is no remaining
/// independently-timed name resolution left for a race to win against.
fn openat_dir_nofollow(parent: &OwnedFd, component: &OsStr, create: bool) -> Result<OwnedFd> {
    let c = cstring_component(component)?;
    let flags = libc::O_RDONLY | libc::O_DIRECTORY | libc::O_NOFOLLOW | libc::O_CLOEXEC;
    let mut fd = unsafe { libc::openat(parent.as_raw_fd(), c.as_ptr(), flags, 0) };
    if fd < 0 {
        let err = std::io::Error::last_os_error();
        if create && err.kind() == std::io::ErrorKind::NotFound {
            let mk = unsafe { libc::mkdirat(parent.as_raw_fd(), c.as_ptr(), 0o755) };
            if mk != 0 {
                let mkerr = std::io::Error::last_os_error();
                if mkerr.raw_os_error() != Some(libc::EEXIST) {
                    return Err(mkerr).with_context(|| format!("mkdirat {component:?}"));
                }
            }
            fd = unsafe { libc::openat(parent.as_raw_fd(), c.as_ptr(), flags, 0) };
        }
        if fd < 0 {
            return Err(std::io::Error::last_os_error())
                .with_context(|| format!("openat {component:?} (O_DIRECTORY|O_NOFOLLOW)"));
        }
    }
    Ok(unsafe { OwnedFd::from_raw_fd(fd) })
}

/// Opens the final path component (the actual write target) anchored to
/// `parent`'s fd, with `O_NOFOLLOW` applied to this hop too, so a symlink
/// planted directly at the leaf name -- not just at an intermediate
/// directory -- is refused by the kernel the same way. The fd this
/// function returns is the *only* fd the caller writes through: there is no
/// subsequent, separately-timed re-open of a path string built from the
/// walk above.
fn openat_leaf_nofollow(parent: &OwnedFd, leaf: &OsStr) -> Result<OwnedFd> {
    let c = cstring_component(leaf)?;
    let flags = libc::O_WRONLY | libc::O_CREAT | libc::O_TRUNC | libc::O_NOFOLLOW | libc::O_CLOEXEC;
    let fd = unsafe { libc::openat(parent.as_raw_fd(), c.as_ptr(), flags, 0o600) };
    if fd < 0 {
        return Err(std::io::Error::last_os_error())
            .with_context(|| format!("openat leaf {leaf:?} (O_NOFOLLOW)"));
    }
    Ok(unsafe { OwnedFd::from_raw_fd(fd) })
}

/// The fd-anchored directory walk + write itself. `rel` is
/// `secure_relative_path`'s output (`workspace_dir`-relative, every
/// component already verified `Normal`/non-empty), so `rel.iter()` yields
/// exactly the path components to walk with no further validation needed
/// here.
///
/// One hop at a time: `workspace_dir` is opened once by name
/// (`open_root_dir`); each intermediate component is then opened relative
/// to the *previous* hop's fd (`openat_dir_nofollow`, dropping the previous
/// fd -- `OwnedFd`'s `Drop` closes it -- once the next one is in hand); and
/// the final component is opened relative to the last directory fd
/// (`openat_leaf_nofollow`) and immediately written to via that same fd
/// (`File::from(OwnedFd)`), never via a path string.
fn materialize_write_via_fd_walk(workspace_dir: &Path, rel: &Path, content: &[u8]) -> Result<()> {
    let components: Vec<&OsStr> = rel.iter().collect();
    let (dirs, leaf_slice) = components
        .split_at(components.len().saturating_sub(1));
    let leaf = *leaf_slice
        .first()
        .ok_or_else(|| anyhow!("empty relative path"))?;

    let mut current = open_root_dir(workspace_dir)?;
    for component in dirs {
        current = openat_dir_nofollow(&current, component, true)?;
    }

    let leaf_fd = openat_leaf_nofollow(&current, leaf)?;
    let mut file = std::fs::File::from(leaf_fd);
    file.write_all(content)
        .with_context(|| format!("write {rel:?} under {workspace_dir:?}"))?;
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
        Request::GetWorkspace { workspace_uuid } => {
            let id = Uuid::parse_str(&workspace_uuid)?;
            store.workspace_meta(id)
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
    fn workspace_meta_reports_base_tree_and_files() {
        let s = store();
        let (seed_id, _) = s.create(None).unwrap();
        s.write_file(seed_id, "seed.txt", b"seed").unwrap();
        let (tree_oid, commit_oid) = s.commit(seed_id, "seed").unwrap();
        let (id, _) = s.create(Some(commit_oid.to_string())).unwrap();
        s.write_file(id, "z.txt", b"z").unwrap();
        s.write_file(id, "a.txt", b"a").unwrap();
        let meta = s.workspace_meta(id).unwrap();
        assert_eq!(
            meta["base_tree_oid"].as_str(),
            Some(tree_oid.to_string().as_str())
        );
        let files = meta["files"].as_array().expect("files");
        assert_eq!(files.len(), 3);
        assert_eq!(files[0]["path"].as_str(), Some("a.txt"));
        assert_eq!(files[1]["path"].as_str(), Some("seed.txt"));
        assert_eq!(files[2]["path"].as_str(), Some("z.txt"));
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

    // --- C021: filesystem projection lifecycle ---------------------------

    #[test]
    fn create_materializes_empty_workspace_dir_with_no_base() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let dir = s.workspace_dir(id);
        assert!(
            dir.is_dir(),
            "expected a real, listable projection dir to exist immediately after create"
        );
    }

    #[test]
    fn create_materializes_seeded_files_to_real_path() {
        let s = store();
        let (seed_id, _) = s.create(None).unwrap();
        s.write_file(seed_id, "a/b/seed.txt", b"seed-content")
            .unwrap();
        let (_, commit_oid) = s.commit(seed_id, "seed").unwrap();

        let (id, _) = s.create(Some(commit_oid.to_string())).unwrap();
        let real_path = s.workspace_dir(id).join("a/b/seed.txt");
        assert!(real_path.is_file(), "expected real file at {real_path:?}");
        assert_eq!(std::fs::read(&real_path).unwrap(), b"seed-content");
    }

    #[test]
    fn write_file_keeps_real_path_in_sync() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "hello.txt", b"v1").unwrap();
        let real_path = s.workspace_dir(id).join("hello.txt");
        assert_eq!(std::fs::read(&real_path).unwrap(), b"v1");

        s.write_file(id, "hello.txt", b"v2").unwrap();
        assert_eq!(
            std::fs::read(&real_path).unwrap(),
            b"v2",
            "real path must reflect the latest in-memory write"
        );
    }

    #[test]
    fn drop_workspace_removes_real_directory() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "hello.txt", b"x").unwrap();
        let dir = s.workspace_dir(id);
        assert!(dir.is_dir());

        assert!(s.drop_workspace(id));
        assert!(
            !dir.exists(),
            "projection dir must be gone after drop_workspace, not just the map entry"
        );
    }

    #[test]
    fn drop_workspace_on_unknown_id_is_harmless() {
        let s = store();
        let id = Uuid::new_v4();
        assert!(!s.drop_workspace(id));
        assert!(!s.workspace_dir(id).exists());
    }

    // --- C021: path-safety hardening for real filesystem writes ----------

    #[test]
    fn write_file_rejects_leading_dot_component_even_without_dotdot_substring() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        // "./evil.txt" contains no ".." substring and does not start with
        // '/', so `check_path` alone would let it through unchanged (a
        // leading "." component is *not* silently normalized away by
        // `Path::components()`, unlike an interior one) --
        // `secure_relative_path`'s component-level check, added for real
        // filesystem writes, is what rejects it.
        assert!(s.write_file(id, "./evil.txt", b"x").is_err());
        assert!(!s.workspace_dir(id).join("evil.txt").exists());
    }

    #[test]
    fn write_file_refuses_to_follow_symlink_planted_at_leaf() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        // Simulate a symlink somehow present at the target leaf, pointing
        // outside the workspace projection directory.
        let outside = tempfile::tempdir().unwrap();
        let secret = outside.path().join("secret.txt");
        std::fs::write(&secret, b"do-not-overwrite").unwrap();
        let leaf = ws_dir.join("evil.txt");
        std::os::unix::fs::symlink(&secret, &leaf).unwrap();

        assert!(
            s.write_file(id, "evil.txt", b"pwned").is_err(),
            "O_NOFOLLOW must cause the open to fail rather than write through the symlink"
        );
        assert_eq!(
            std::fs::read(&secret).unwrap(),
            b"do-not-overwrite",
            "the symlink target outside the workspace must be untouched"
        );
    }

    // --- SEC-001: TOCTOU fix (fd-anchored path walk in materialize_write) ---

    #[test]
    fn write_file_refuses_to_follow_symlink_planted_at_intermediate_component() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        // Pre-plant (not concurrently -- the malicious state already
        // exists at the moment write_file is called) a symlink at an
        // INTERMEDIATE directory component ("sub"), pointing outside the
        // workspace projection directory. This exercises the same
        // fd-anchored, O_NOFOLLOW-at-every-hop code path
        // (`openat_dir_nofollow`) that a genuine concurrent symlink-swap
        // race (see `write_file_race_against_symlink_swap_never_escapes_
        // workspace` below) would also hit, deterministically and without
        // relying on thread timing.
        //
        // NOTE for reviewers: this specific *static* scenario was also
        // already rejected by the pre-fix `canonicalize`-then-`starts_with`
        // check (canonicalize resolves the symlink and finds it outside
        // the root, synchronously, in the same call) -- so this test alone
        // does not distinguish pre-fix from post-fix behavior. It is
        // included as direct regression coverage for the new
        // `openat_dir_nofollow` hop specifically (the intermediate-
        // component case had no dedicated test before this fix, only the
        // leaf case did), and as a deterministic exercise of the same code
        // path the genuine race below stresses. The stress test below is
        // what actually exercises the timing gap the pre-fix code left
        // open.
        let outside = tempfile::tempdir().unwrap();
        std::os::unix::fs::symlink(outside.path(), ws_dir.join("sub")).unwrap();

        assert!(
            s.write_file(id, "sub/evil.txt", b"pwned").is_err(),
            "an intermediate path component that is a symlink must cause the write to fail closed"
        );
        assert!(
            !outside.path().join("evil.txt").exists(),
            "no file must be created outside the workspace root through a symlinked intermediate component"
        );
    }

    #[test]
    fn write_file_race_against_symlink_swap_never_escapes_workspace() {
        // Best-effort, non-flaky-by-design race test, mirroring
        // fs_tools_test.go's
        // TestWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace
        // (T195): this does NOT assert that the race window is hit on any
        // particular iteration (that would be flaky by construction) --
        // only that across many iterations of a concurrent, real
        // directory-vs-symlink swap at "race", write_file NEVER results in
        // content landing outside the workspace root. This is the test
        // that actually exercises the check-then-use timing gap the
        // pre-fix `canonicalize`-then-`open` code left open (see
        // `materialize_write`'s doc comment): supporting empirical evidence
        // alongside the deterministic tests above and the structural
        // reasoning in `materialize_write_via_fd_walk`'s doc comment, not
        // the sole proof.
        use std::sync::atomic::{AtomicBool, Ordering};
        use std::sync::Arc;

        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);
        let race_dir = ws_dir.join("race");
        std::fs::create_dir(&race_dir).unwrap();

        let outside = tempfile::tempdir().unwrap();
        let outside_path = outside.path().to_path_buf();

        let stop = Arc::new(AtomicBool::new(false));
        let escape = Arc::new(AtomicBool::new(false));

        let racer_stop = stop.clone();
        let link_path = race_dir.clone();
        let symlink_target = outside_path.clone();
        let racer = std::thread::spawn(move || {
            while !racer_stop.load(Ordering::Relaxed) {
                let _ = std::fs::remove_dir(&link_path);
                let _ = std::fs::remove_file(&link_path);
                let _ = std::os::unix::fs::symlink(&symlink_target, &link_path);
                let _ = std::fs::remove_file(&link_path);
                let _ = std::fs::create_dir(&link_path);
            }
        });

        const ITERATIONS: usize = 2000;
        for _ in 0..ITERATIONS {
            let _ = s.write_file(id, "race/pwned.txt", b"owned");
            if outside_path.join("pwned.txt").exists() {
                escape.store(true, Ordering::Relaxed);
                break;
            }
            let _ = std::fs::remove_file(race_dir.join("pwned.txt"));
        }

        stop.store(true, Ordering::Relaxed);
        racer.join().unwrap();

        assert!(
            !escape.load(Ordering::Relaxed),
            "write_file wrote a file outside the workspace during concurrent symlink swap"
        );
    }

    #[test]
    fn base_tree_symlink_entry_materializes_as_plain_file_not_real_symlink() {
        let s = store();
        // Build a tree containing an entry with git's symlink mode (120000)
        // directly via TreeBuilder, bypassing write_file/commit (which only
        // ever produce mode 100644 entries) -- this simulates an unusual or
        // hostile base tree handed to `create`.
        let commit_oid = {
            let repo = s.repo.lock();
            let blob_oid = repo.blob(b"../../etc/passwd").unwrap();
            let mut tb = repo.treebuilder(None).unwrap();
            tb.insert("link.txt", blob_oid, 0o120000).unwrap();
            let tree_oid = tb.write().unwrap();
            let tree = repo.find_tree(tree_oid).unwrap();
            let sig = git2::Signature::now("test", "test@example.invalid").unwrap();
            repo.commit(None, &sig, &sig, "seed", &tree, &[]).unwrap()
        };

        let (id, _) = s.create(Some(commit_oid.to_string())).unwrap();
        let real_path = s.workspace_dir(id).join("link.txt");
        let meta = std::fs::symlink_metadata(&real_path).unwrap();
        assert!(
            !meta.file_type().is_symlink(),
            "a base-tree symlink-mode entry must never materialize as a real OS symlink"
        );
        assert_eq!(std::fs::read(&real_path).unwrap(), b"../../etc/passwd");
    }
}

// Suppress unused warnings on _path holders.
#[allow(dead_code)]
fn _ensure_path_use(_p: &Path) {}
