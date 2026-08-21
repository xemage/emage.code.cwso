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
use std::sync::{Arc, OnceLock};

use anyhow::{anyhow, Context, Result};
use base64::{engine::general_purpose::STANDARD as B64, Engine as _};
use git2::{IndexEntry, IndexTime, ObjectType, Oid, Repository, RepositoryInitOptions};
use parking_lot::Mutex;
use serde_json::json;
use uuid::Uuid;

use crate::ast;
use crate::proto::{Request, Response};
use crate::writeback::WriteBackEngine;

pub struct ShadowStore {
    repo: Mutex<Repository>,
    workspaces: Mutex<HashMap<Uuid, Workspace>>,
    /// Root of the tmpfs-backed projection tree; each workspace gets
    /// `storage_root/<uuid>/` (see `workspace_dir`).
    storage_root: PathBuf,
    /// C022 write-back engine (`docs/decisions/ADR-012-shadow-workspace-
    /// filesystem-projection.md`). Populated exactly once, shortly after
    /// construction in `main`, via `attach_writeback` -- see that method's
    /// doc comment for why a `OnceLock` rather than a constructor argument.
    /// Every code path that reads it treats "not attached" (this module's
    /// own unit tests construct a bare `ShadowStore` and never attach one)
    /// as "write-back is inactive," never as an error.
    writeback: OnceLock<Arc<WriteBackEngine>>,
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
            writeback: OnceLock::new(),
        })
    }

    /// Wires up the C022 write-back engine after construction. Must be
    /// called at most once (a second call is a no-op: `OnceLock::set`
    /// simply reports failure, which is intentionally ignored rather than
    /// panicking — there is no scenario in this service where re-attaching
    /// a second engine instance would be correct, so silently keeping the
    /// first one is the safe default). Not part of `new` itself because
    /// `WriteBackEngine::spawn` needs an `Arc<ShadowStore>` to hand its
    /// background threads, which does not exist yet inside `new`'s own
    /// constructor body — see `main.rs` for the two-step wiring.
    pub fn attach_writeback(&self, engine: Arc<WriteBackEngine>) {
        let _ = self.writeback.set(engine);
    }

    /// Deterministic real, tmpfs-backed path for a workspace's projection:
    /// `<storage_root>/<workspace-uuid>/`. `Uuid::to_string()` only ever
    /// produces hyphenated lowercase hex, so this is not itself
    /// attacker-controlled input (unlike the file `path` strings handled by
    /// `materialize_write`). `pub(crate)` so `writeback.rs` can resolve a
    /// workspace's real path without duplicating this logic.
    pub(crate) fn workspace_dir(&self, id: Uuid) -> PathBuf {
        self.storage_root.join(id.to_string())
    }

    // `pub(crate)`, not `fn`, solely so `writeback.rs`'s own unit tests
    // (a different module) can create a workspace to exercise write-back
    // against; `dispatch` below remains the only externally reachable
    // (IPC) caller.
    pub(crate) fn create(&self, base: Option<String>) -> Result<(Uuid, Option<Oid>)> {
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
        // C022: register this workspace's real, projected directory for
        // filesystem write-back (recursively adds inotify watches and
        // seeds write-back's view of pre-existing content -- see
        // `WriteBackEngine::register_workspace`'s doc comment). Done last,
        // after the workspace is visible in `self.workspaces`, so a
        // write-back event racing in immediately after registration always
        // finds a live workspace to apply itself to.
        if let Some(engine) = self.writeback.get() {
            engine.register_workspace(id, &ws_dir);
        }
        Ok((id, base_tree))
    }

    fn drop_workspace(&self, id: Uuid) -> bool {
        // C022: tear down this workspace's watches *before* removing the
        // in-memory entry and the real directory, so no further write-back
        // event for this workspace is dispatched after this point (the
        // `workspace_exists` guard in `writeback.rs`'s `handle_event` is a
        // second, independent backstop if a stray event still races in).
        if let Some(engine) = self.writeback.get() {
            engine.unregister_workspace(id);
        }
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

    // --- C022: write-back support (called only from `writeback.rs`) ------
    //
    // These methods are the *only* way `writeback.rs` touches `ShadowStore`
    // state. Each one only updates the in-memory `files` map (and, for a
    // write, the git ODB) -- never the real, projected filesystem path; see
    // `writeback.rs`'s module doc comment ("Why write-back never writes
    // back to disk") for why that direction is one-way by design.

    /// Whether `id` is still a live workspace. Used by write-back to no-op
    /// safely against a concurrent `drop_workspace` rather than erroring.
    pub(crate) fn workspace_exists(&self, id: Uuid) -> bool {
        self.workspaces.lock().contains_key(&id)
    }

    /// Records that `rel_path` now holds `content` on the real, projected
    /// filesystem: writes `content` as a blob (idempotent -- writing
    /// identical content again yields the same `Oid`) and updates the
    /// workspace's `files` map. Silently no-ops if the workspace no longer
    /// exists (a race against `drop_workspace`, not an error). Fails
    /// closed (returns `Err`, does not touch the map) if `rel_path` itself
    /// somehow isn't a valid workspace-relative logical path -- defense in
    /// depth: every real caller in `writeback.rs` builds `rel_path` from a
    /// combination of an already-registered watch's own `rel_dir` (which
    /// this store produced) and a kernel-reported directory-entry name
    /// (which cannot contain `/` or be `.`/`..`), so this should never
    /// actually trip, but a state-corrupting map key is exactly the kind of
    /// silent failure this task's brief singles out as unacceptable, so it
    /// is checked rather than assumed.
    pub(crate) fn wb_apply_write(&self, id: Uuid, rel_path: &str, content: &[u8]) -> Result<()> {
        secure_relative_path(rel_path)
            .with_context(|| format!("write-back rejected unsafe logical path: {rel_path}"))?;
        let oid = self.repo.lock().blob(content)?;
        if let Some(ws) = self.workspaces.lock().get_mut(&id) {
            ws.files.insert(rel_path.to_string(), oid);
        }
        Ok(())
    }

    /// Records that `rel_path` no longer exists on the real, projected
    /// filesystem. Silently no-ops if the workspace or the path is already
    /// gone (both are legitimate, harmless races, not errors).
    pub(crate) fn wb_apply_delete(&self, id: Uuid, rel_path: &str) {
        if let Some(ws) = self.workspaces.lock().get_mut(&id) {
            ws.files.remove(rel_path);
        }
    }

    /// Removes every `files` entry whose logical path falls under
    /// `rel_dir` (a whole real subdirectory having been deleted or moved
    /// away). `rel_dir == ""` means the workspace root itself -- every
    /// entry is cleared in that case, since `""` is logically a prefix of
    /// every path in the workspace.
    pub(crate) fn wb_apply_delete_prefix(&self, id: Uuid, rel_dir: &str) {
        if let Some(ws) = self.workspaces.lock().get_mut(&id) {
            if rel_dir.is_empty() {
                ws.files.clear();
                return;
            }
            let prefix = format!("{rel_dir}/");
            ws.files.retain(|path, _| !path.starts_with(&prefix));
        }
    }

    /// Snapshot of every currently known workspace id, for the
    /// reconciliation pass to iterate without holding `workspaces`' lock
    /// for the duration of each workspace's (potentially slow) filesystem
    /// walk.
    pub(crate) fn workspace_ids_snapshot(&self) -> Vec<Uuid> {
        self.workspaces.lock().keys().copied().collect()
    }

    /// Read-only snapshot of a workspace's current `files` map, for the
    /// reconciliation pass to diff against the real filesystem without
    /// holding the lock for that same duration. `None` if the workspace no
    /// longer exists (a race against `drop_workspace`).
    pub(crate) fn workspace_files_snapshot(&self, id: Uuid) -> Option<HashMap<String, Oid>> {
        self.workspaces.lock().get(&id).map(|ws| ws.files.clone())
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

/// Result of [`scan_workspace_tree`]: every reachable regular file's
/// logical (workspace-relative, `/`-joined) path and content, plus every
/// reachable real directory's logical path (the scan's own starting point
/// included, so a caller scanning from the workspace root always gets `""`
/// as one entry, and a caller scanning a specific new subtree gets that
/// subtree's own path as one entry).
pub(crate) struct FsScanResult {
    pub files: HashMap<String, Vec<u8>>,
    pub dirs: Vec<String>,
}

/// Recursively scans the real, projected filesystem starting at
/// `workspace_dir.join(start_rel)` (or `workspace_dir` itself if
/// `start_rel` is empty), for C022 write-back's event handler
/// (`writeback.rs`) and its periodic reconciliation pass. This is the read
/// side's counterpart to `materialize_write` -- read here, not written --
/// and is deliberately narrower in its safety guarantee, for reasons
/// explained below rather than left implicit.
///
/// # Path construction (traversal risk)
///
/// Every path this function produces is built by joining an
/// already-walked `PathBuf` (descended, one hop at a time, from
/// `workspace_dir`, which is itself not attacker-controlled -- see
/// `ShadowStore::workspace_dir`'s doc comment) with a single filename taken
/// directly from `DirEntry::file_name()`. A kernel-reported directory-entry
/// name can never contain a `/` byte, and `std::fs::read_dir` never yields
/// the special `.`/`..` entries -- so, unlike `secure_relative_path`'s job
/// on the write side (parsing and validating a caller-supplied, arbitrary
/// *string*), there is no string-based `..`/absolute-path escape to check
/// for here, because nothing on this path is ever parsed from a raw string
/// handed to us by anything outside this process. This is the "smaller
/// risk than the write side" the task brief anticipated, and this is the
/// citable reason it is smaller, not an assumption.
///
/// # Symlink handling (deliberate, not an oversight)
///
/// Every entry's type is determined via `std::fs::symlink_metadata` --
/// never `std::fs::metadata`, and never by calling `std::fs::read` first
/// and hoping. Anything reporting `file_type().is_symlink()` is skipped
/// entirely (logged at `warn`, not read, not recursed into). This removes
/// "external tooling plants a symlink directly in the tmpfs projection to
/// pull arbitrary host content into a shadow commit" as a live attack
/// class, by construction, for every path this function walks.
///
/// # Residual TOCTOU (accepted, documented, *not* eliminated)
///
/// POC-DEBT R-3: read-side symlink check is not fd-anchored (unlike the
/// write side); see the reasoning below and `docs/DEBT-REGISTER.md` row R-3.
///
/// The `symlink_metadata` check and the subsequent `fs::read`/recursion are
/// two independent syscalls, not one fd-anchored operation the way the
/// write side's `openat(..., O_NOFOLLOW)` (`materialize_write_via_fd_walk`)
/// closes the equivalent gap in a single kernel call. A component could in
/// principle be swapped for a symlink in the narrow window between the two
/// calls. This gap is deliberately accepted here, not closed the way the
/// write side closes it, because the actor who could win that race already
/// has the local filesystem write access needed to plant it -- which is
/// exactly the premise of this entire task (raw mutations at the projected
/// path) -- and the worst case is that *same* actor's own workspace ending
/// up with a commit containing bytes read from outside its own workspace
/// root, not a cross-workspace or host-escape read, since write-back never
/// writes anything back to the real filesystem in this direction (see
/// `writeback.rs`'s module doc comment). This is a genuine judgment call,
/// not an unconsidered shortcut, and is flagged here explicitly for
/// Security Engineer review rather than silently narrowed to "safe" -- see
/// `docs/DEBT-REGISTER.md` for the tracked entry and a fd-anchored
/// hardening path if this residual risk is judged unacceptable.
///
/// # Non-UTF-8 filenames
///
/// POC-DEBT R-4: non-UTF-8 filenames are silently skipped (see
/// `docs/DEBT-REGISTER.md` row R-4).
///
/// Skipped with a `warn` log, not an error: this store's `files` map is
/// `HashMap<String, Oid>` and its IPC protocol carries paths as JSON
/// strings end-to-end, so a non-UTF-8 name could never be represented
/// here regardless of this function -- a pre-existing, system-wide
/// constraint this task does not introduce (see `docs/DEBT-REGISTER.md`).
pub(crate) fn scan_workspace_tree(workspace_dir: &Path, start_rel: &str) -> Result<FsScanResult> {
    let mut files = HashMap::new();
    let mut dirs = vec![start_rel.to_string()];
    let start_dir = if start_rel.is_empty() {
        workspace_dir.to_path_buf()
    } else {
        workspace_dir.join(start_rel)
    };
    scan_dir_into(&start_dir, start_rel, &mut files, &mut dirs)?;
    Ok(FsScanResult { files, dirs })
}

fn scan_dir_into(
    real_dir: &Path,
    rel_dir: &str,
    files: &mut HashMap<String, Vec<u8>>,
    dirs: &mut Vec<String>,
) -> Result<()> {
    let entries = match std::fs::read_dir(real_dir) {
        Ok(e) => e,
        // The directory may have been removed between the event that
        // triggered this scan and this call -- not an error, just nothing
        // left to find here.
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(()),
        Err(e) => return Err(e).with_context(|| format!("read_dir {real_dir:?}")),
    };
    for entry in entries {
        let entry = entry.with_context(|| format!("iterate {real_dir:?}"))?;
        let name = entry.file_name();
        let name_str = match name.to_str() {
            Some(s) => s,
            None => {
                tracing::warn!(
                    dir = ?real_dir,
                    name = ?name,
                    "write-back scan: skipping entry with non-UTF-8 filename"
                );
                continue;
            }
        };
        let rel_path = if rel_dir.is_empty() {
            name_str.to_string()
        } else {
            format!("{rel_dir}/{name_str}")
        };
        let real_path = entry.path();
        let meta = std::fs::symlink_metadata(&real_path)
            .with_context(|| format!("symlink_metadata {real_path:?}"))?;
        let ft = meta.file_type();
        if ft.is_symlink() {
            tracing::warn!(
                path = %rel_path,
                real_path = ?real_path,
                "write-back scan: skipping symlink planted directly in shadow workspace \
                 projection (never followed) -- see scan_workspace_tree's doc comment"
            );
            continue;
        }
        if ft.is_dir() {
            dirs.push(rel_path.clone());
            scan_dir_into(&real_path, &rel_path, files, dirs)?;
        } else if ft.is_file() {
            let content =
                std::fs::read(&real_path).with_context(|| format!("read {real_path:?}"))?;
            files.insert(rel_path, content);
        } else {
            tracing::warn!(
                path = %rel_path,
                "write-back scan: skipping non-regular, non-directory entry \
                 (device/FIFO/socket) in shadow workspace projection"
            );
        }
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

    // --- C022: filesystem write-back into the git ODB --------------------
    //
    // Unlike `store()` above, these tests need a live `WriteBackEngine`
    // attached (inotify watcher + reconciliation loop), since that is
    // exactly what is under test: an edit made *only* at the real,
    // projected path (never through `write_file`) must still show up in
    // `commit`'s tree. Write-back is asynchronous by design (an inotify
    // event is delivered to a background thread, or -- worst case --
    // picked up by the next reconciliation tick), so these tests poll
    // (`wait_until`) rather than asserting immediately after the raw
    // filesystem mutation.

    fn store_with_writeback() -> Arc<ShadowStore> {
        let dir = tempfile::tempdir().unwrap();
        let s = Arc::new(ShadowStore::new(dir.keep()).unwrap());
        let engine = crate::writeback::WriteBackEngine::spawn(Arc::clone(&s))
            .expect("spawn write-back engine");
        s.attach_writeback(engine);
        s
    }

    /// Polls `check` until it returns `true` or a bounded deadline passes.
    /// The common case (a healthy inotify watch) resolves in well under a
    /// second; the deadline is generous specifically to remain correct
    /// (not flaky) even if a given run has to fall back to a periodic
    /// reconciliation tick instead.
    fn wait_until(mut check: impl FnMut() -> bool, what: &str) {
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
        loop {
            if check() {
                return;
            }
            if std::time::Instant::now() >= deadline {
                panic!("write-back: timed out waiting for: {what}");
            }
            std::thread::sleep(std::time::Duration::from_millis(25));
        }
    }

    #[test]
    fn write_back_captures_file_created_directly_on_real_path() {
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();

        // Created via ordinary std::fs, never via the `write_file` IPC
        // call -- this is exactly the "raw filesystem mutation" scenario
        // this task closes.
        let real = s.workspace_dir(id).join("created_via_fs.txt");
        std::fs::write(&real, b"hello from raw fs").unwrap();

        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb create").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                match tree.get_path(Path::new("created_via_fs.txt")) {
                    Ok(entry) => {
                        let blob = repo.find_blob(entry.id()).unwrap();
                        blob.content() == b"hello from raw fs"
                    }
                    Err(_) => false,
                }
            },
            "commit tree to contain a file created directly on the real path",
        );
    }

    #[test]
    fn write_back_captures_file_modified_directly_on_real_path() {
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "existing.txt", b"v1").unwrap();

        let real = s.workspace_dir(id).join("existing.txt");
        std::fs::write(&real, b"v2-via-raw-fs").unwrap();

        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb modify").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                match tree.get_path(Path::new("existing.txt")) {
                    Ok(entry) => {
                        let blob = repo.find_blob(entry.id()).unwrap();
                        blob.content() == b"v2-via-raw-fs"
                    }
                    Err(_) => false,
                }
            },
            "commit tree to reflect a modification made directly on the real path",
        );
    }

    #[test]
    fn write_back_captures_file_deleted_directly_on_real_path() {
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "to_delete.txt", b"bye").unwrap();

        let real = s.workspace_dir(id).join("to_delete.txt");
        assert!(real.is_file());
        std::fs::remove_file(&real).unwrap();

        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb delete").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                tree.get_path(Path::new("to_delete.txt")).is_err()
            },
            "commit tree to no longer contain a file deleted directly on the real path",
        );
    }

    #[test]
    fn write_back_captures_file_renamed_directly_on_real_path() {
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "old_name.txt", b"payload").unwrap();

        let ws_dir = s.workspace_dir(id);
        std::fs::rename(ws_dir.join("old_name.txt"), ws_dir.join("new_name.txt")).unwrap();

        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb rename").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                let new_present = tree
                    .get_path(Path::new("new_name.txt"))
                    .map(|entry| {
                        let blob = repo.find_blob(entry.id()).unwrap();
                        blob.content() == b"payload"
                    })
                    .unwrap_or(false);
                let old_absent = tree.get_path(Path::new("old_name.txt")).is_err();
                new_present && old_absent
            },
            "commit tree to reflect a rename made directly on the real path (new name present, old name gone)",
        );
    }

    #[test]
    fn write_back_captures_new_subdirectory_with_nested_file_created_directly_on_real_path() {
        // A single bulk operation (e.g. `mkdir -p a/b && echo hi > a/b/c.txt`,
        // or a tar/cp -r extraction) can populate a brand-new subdirectory
        // faster than a per-CREATE-event watch can be registered for each
        // new level -- exactly the "inotify is not recursive" gap this
        // module's write-back engine documents. This test exercises that
        // path end-to-end via `sync_new_subtree`/reconciliation, not just
        // flat, single-file mutations.
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);
        std::fs::create_dir_all(ws_dir.join("a/b")).unwrap();
        std::fs::write(ws_dir.join("a/b/c.txt"), b"nested").unwrap();

        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb nested create").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                match tree.get_path(Path::new("a/b/c.txt")) {
                    Ok(entry) => {
                        let blob = repo.find_blob(entry.id()).unwrap();
                        blob.content() == b"nested"
                    }
                    Err(_) => false,
                }
            },
            "commit tree to contain a file nested in a brand-new subdirectory tree",
        );
    }

    #[test]
    fn write_back_never_follows_a_symlink_planted_directly_on_real_path() {
        // A hostile or merely careless external tool plants a real OS
        // symlink pointing outside the workspace directly in the tmpfs
        // projection. Write-back must never read through it into a commit.
        let s = store_with_writeback();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        let outside = tempfile::tempdir().unwrap();
        let secret = outside.path().join("secret.txt");
        std::fs::write(&secret, b"host-only content").unwrap();
        std::os::unix::fs::symlink(&secret, ws_dir.join("link.txt")).unwrap();

        // Give write-back a real chance to (wrongly) pick this up before
        // asserting it did not: a plain file written right after the
        // symlink, waited on the normal way, gives the engine's event loop
        // and/or reconciliation pass ample opportunity to also have
        // processed the symlink by the time this returns.
        std::fs::write(ws_dir.join("sentinel.txt"), b"go").unwrap();
        wait_until(
            || {
                let (tree_oid, _) = s.commit(id, "wb symlink sentinel").unwrap();
                let repo = s.repo.lock();
                let tree = repo.find_tree(tree_oid).unwrap();
                tree.get_path(Path::new("sentinel.txt")).is_ok()
            },
            "sentinel file to be captured (proves write-back had a chance to also see the symlink)",
        );

        let (tree_oid, _) = s.commit(id, "wb symlink final").unwrap();
        let repo = s.repo.lock();
        let tree = repo.find_tree(tree_oid).unwrap();
        assert!(
            tree.get_path(Path::new("link.txt")).is_err(),
            "a symlink planted directly at the projected path must never be materialized into a commit"
        );
    }
}

// Suppress unused warnings on _path holders.
#[allow(dead_code)]
fn _ensure_path_use(_p: &Path) {}
