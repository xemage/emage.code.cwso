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
use std::ffi::{CStr, CString, OsStr};
use std::io::{Read as _, Write as _};
use std::os::fd::{AsRawFd, FromRawFd, OwnedFd, RawFd};
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
    /// SEV-C041-001: per-workspace serialization for `commit()`. `commit()`
    /// reads a workspace's tracked `head`, builds a tree, calls
    /// `repo.commit`, and only then advances `head` -- a span of several
    /// operations, not one atomic step. The `workspaces` lock above only
    /// protects the workspace-map lookup itself (acquired and released
    /// twice, briefly, at the start and end of `commit()`), so two
    /// concurrent `commit()` calls against the *same* workspace could both
    /// read the same stale `head`, both commit against that same parent,
    /// and have the loser's commit silently dropped from the chain (still
    /// present in the ODB, but unreachable from `head` and thus invisible
    /// to any future ancestor-walk) once the winner overwrites `head` last.
    /// This side-table gives every workspace id its own `Arc<Mutex<()>>`,
    /// looked up (or inserted) under a brief lock on the table itself, then
    /// held by `commit()` for the *entire* read-head -> build-tree ->
    /// `repo.commit` -> advance-head span -- serializing same-workspace
    /// commits without serializing commits against different workspaces
    /// (each workspace's entry is an independent `Mutex`, so a lock held
    /// for workspace A never blocks a concurrent commit against workspace
    /// B). See `commit`'s doc comment for the full guarantee and
    /// `docs/DEBT-REGISTER.md` row R-7.
    ///
    /// Lifecycle: entries are never removed, including by `drop_workspace`.
    /// A dropped workspace's UUID is never reused (`Uuid::new_v4` per
    /// `create` call), so a stale entry can never be mistakenly acquired
    /// for a different, later workspace -- it just sits unused, one
    /// `Arc<Mutex<()>>` (a handful of words) per workspace ever created for
    /// the lifetime of the process. Actively cleaning up on
    /// `drop_workspace` would need to guard against a commit that is
    /// already mid-flight (holding the `Arc` it looked up) racing a
    /// concurrent drop-then-recreate-under-a-different-uuid -- impossible
    /// here since UUIDs aren't reused, but the cleanup code would still
    /// have to be written defensively against it, for a bound (this
    /// service's total distinct-workspaces-ever-created count) that is
    /// already small relative to process lifetime in every deployment this
    /// service targets. Deliberately left as an unbounded-but-negligible
    /// side-table rather than adding that complexity for no real payoff.
    commit_locks: Mutex<HashMap<Uuid, Arc<Mutex<()>>>>,
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
    /// C041: the commit `commit()` should use as this workspace's sole
    /// parent the next time it is called, and the running "current HEAD" of
    /// this workspace's own commit chain -- *not* necessarily the same thing
    /// as `base_tree`'s commit forever, since it advances every time
    /// `commit()` succeeds (see that method). `None` means this workspace
    /// has no commit yet to chain from: either it was created with no base
    /// (`create(None)`) and has never been committed, in which case its next
    /// commit is legitimately a root commit with no parent, or it *was*
    /// seeded from a base commit, in which case this is `Some(<that
    /// commit's oid>)` from the moment `create` returns -- so a workspace's
    /// very first `commit()` call continues that base commit's history
    /// rather than starting a disconnected one.
    head: Option<Oid>,
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
        // C023 startup reconciliation sweep (ADR-012 crash-safety): must run
        // after `bare.git` exists (so it is correctly recognized as the one
        // persistent entry to keep) and before `Self` is constructed, i.e.
        // before this instance's own (always-empty-at-construction, see the
        // `workspaces` field's doc comment) map could be mistaken for a
        // record of anything worth preserving. See `sweep_stale_workspace_dirs`.
        sweep_stale_workspace_dirs(&storage_root)
            .context("startup reconciliation sweep of shadow storage root")?;
        Ok(Self {
            repo: Mutex::new(repo),
            workspaces: Mutex::new(HashMap::new()),
            commit_locks: Mutex::new(HashMap::new()),
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
        // C041: capture the base *commit*'s own oid, not just its tree, so
        // it can seed this workspace's `head` below -- a workspace created
        // from an existing commit must chain its own first `commit()` off
        // that commit, not treat it as parentless.
        let (base_commit, base_tree) = if let Some(sha) = base {
            let oid = Oid::from_str(&sha).with_context(|| format!("bad sha {sha}"))?;
            let repo = self.repo.lock();
            let commit = repo.find_commit(oid)?;
            (Some(oid), Some(commit.tree_id()))
        } else {
            (None, None)
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
            head: base_commit,
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

    /// C041: builds a tree from the workspace's current `files` map and
    /// commits it, chaining onto the workspace's tracked `head` (this
    /// workspace's own previous commit, or the base commit it was created
    /// from) as the sole parent -- or with *no* parent, as a legitimate root
    /// commit, iff `head` is still `None` (a workspace created with no base
    /// that has never been committed before). `head` is read once up front
    /// and only advanced to the new commit's oid *after* `repo.commit`
    /// actually succeeds, so a failed commit attempt never leaves the
    /// workspace's chain pointing at a commit that doesn't exist.
    ///
    /// SEV-C041-001 (HIGH, fixed here, see `docs/DEBT-REGISTER.md` row R-7):
    /// the read-head -> build-tree -> `repo.commit` -> advance-head sequence
    /// above is several separate operations, not one atomic step, so it must
    /// never run concurrently against the *same* workspace -- two racing
    /// callers could otherwise both read the same stale `head`, both commit
    /// against that same parent, and have whichever one advances `head`
    /// last silently orphan the other's commit from the workspace's own
    /// chain (still present in the ODB, but unreachable by walking `head`
    /// backwards, and thus invisible to any future `git log`/ancestor
    /// walk -- exactly the kind of state C042's three-way merge depends on
    /// `head` correctly reflecting). This method now acquires this
    /// workspace's own entry in `commit_locks` (see that field's doc
    /// comment) and holds it for this call's *entire* body, so two
    /// `commit()` calls against the same `id` always serialize. Commits
    /// against *different* workspace ids use different `Arc<Mutex<()>>`
    /// entries and therefore never block each other -- this fix does not
    /// reintroduce a single global commit lock, which would defeat the
    /// point of C043's connection pooling. Regression coverage:
    /// `concurrent_commits_against_one_workspace_never_lose_a_commit` (an
    /// 8-thread adversarial probe) and
    /// `concurrent_commits_against_different_workspaces_are_not_serialized`.
    fn commit(&self, id: Uuid, message: &str) -> Result<(Oid, Oid)> {
        // Look up (or lazily create) this workspace's own commit lock under
        // a brief lock on the side-table itself, then drop that lock
        // immediately -- the side-table lock only ever guards the
        // map-lookup/insert, never the commit body below.
        let commit_lock = {
            self.commit_locks
                .lock()
                .entry(id)
                .or_insert_with(|| Arc::new(Mutex::new(())))
                .clone()
        };
        // Held for the full span of this method: every other `commit()`
        // call against this same workspace id blocks here until this one
        // returns (including on an early `?` error return, since the guard
        // is dropped by `Drop` regardless of exit path). A workspace id
        // that no longer exists in `self.workspaces` still safely acquires
        // and releases its own lock entry here; the "no such workspace"
        // check just below is what actually rejects it.
        let _commit_guard = commit_lock.lock();

        let wss = self.workspaces.lock();
        let ws = wss.get(&id).ok_or_else(|| anyhow!("no such workspace"))?;
        let entries: Vec<(String, Oid)> = ws.files.iter().map(|(k, v)| (k.clone(), *v)).collect();
        let parent_oid = ws.head;
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
        // Resolve the tracked HEAD (if any) to a real `git2::Commit` *before*
        // building the parent-refs slice below, so its lifetime outlives the
        // `repo.commit` call that borrows from it.
        let parent_commit = parent_oid.map(|oid| repo.find_commit(oid)).transpose()?;
        let parent_refs: Vec<&git2::Commit> = parent_commit.iter().collect();
        let commit_oid = repo.commit(None, &sig, &sig, message, &tree, &parent_refs)?;
        drop(parent_commit);
        drop(tree);
        drop(repo);

        // Advance this workspace's chain to the commit that was just made.
        // A missing workspace here (raced against a concurrent
        // `drop_workspace` between the lock above and this one) is a
        // harmless no-op, matching this module's existing pattern elsewhere
        // (e.g. `wb_apply_write`) for the same race -- the commit itself
        // already succeeded and its oid is still returned to the caller.
        // This write is itself still guarded by `_commit_guard` (held until
        // the end of this function), so it cannot race a concurrent
        // `commit()` against the same workspace id -- only against
        // `drop_workspace`, which is the documented, harmless race above.
        if let Some(ws) = self.workspaces.lock().get_mut(&id) {
            ws.head = Some(commit_oid);
        }
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

/// C023 crash-safety: removes every entry directly under `storage_root`
/// *except* the persistent `bare.git` ODB directory, and reports what it
/// removed. Called exactly once, synchronously, from `ShadowStore::new`,
/// immediately after `bare.git` is opened/created.
///
/// Why this is safe to do unconditionally, with no diff against "workspaces
/// this process still considers open": `ShadowStore::new` always constructs
/// `workspaces` as an empty map (see that field's doc comment) -- there is
/// no on-disk/ODB-ref record anywhere of *which* per-workspace directories
/// under `storage_root` belonged to a still-"open" workspace of whatever
/// process last held `storage_root` open. So by the time any caller can
/// observe a freshly-constructed `ShadowStore`, every subdirectory other
/// than `bare.git` is -- by construction, not by heuristic -- left over from
/// a previous instance and cannot be legitimately claimed by this one.
/// This holds identically whether the previous instance exited gracefully
/// (and simply didn't get to run its own `drop_workspace` calls for every
/// still-open workspace before exiting) or was killed outright (`kill -9`,
/// container OOM, etc.) -- this sweep does not need to distinguish those
/// cases, and doesn't try to.
///
/// This is expected, routine behavior on every restart -- not a warning-
/// worthy anomaly -- so a non-empty sweep is logged at `info`, not `warn`.
/// Per-entry removal failures (e.g. a permissions problem) are logged at
/// `warn` and skipped rather than aborting the whole sweep, so one stuck
/// entry cannot prevent the service from starting at all.
fn sweep_stale_workspace_dirs(storage_root: &Path) -> Result<()> {
    const PERSISTENT_ENTRY: &str = "bare.git";

    let read_dir = std::fs::read_dir(storage_root)
        .with_context(|| format!("read storage root {storage_root:?} for startup sweep"))?;

    let mut swept: Vec<PathBuf> = Vec::new();
    for entry in read_dir {
        let entry =
            entry.with_context(|| format!("read directory entry under {storage_root:?}"))?;
        if entry.file_name() == PERSISTENT_ENTRY {
            continue;
        }
        let path = entry.path();
        let is_dir = entry.file_type().map(|t| t.is_dir()).unwrap_or(false);
        let remove_result = if is_dir {
            std::fs::remove_dir_all(&path)
        } else {
            std::fs::remove_file(&path)
        };
        match remove_result {
            Ok(()) => swept.push(path),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => {
                tracing::warn!(
                    path = ?path,
                    error = %e,
                    "startup sweep: failed to remove stale entry under shadow storage root"
                );
            }
        }
    }

    if !swept.is_empty() {
        tracing::info!(
            count = swept.len(),
            paths = ?swept,
            "startup sweep: removed stale per-workspace projection directories left behind by a \
             previous cwso-git-shadow instance (graceful shutdown that didn't reach every \
             drop_workspace, or an unclean crash) -- this is expected, routine startup behavior"
        );
    }

    Ok(())
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

// --- C035 (closes R-3): fd-anchored recursive read-side walk ------------
//
// The write side (`materialize_write_via_fd_walk` above) anchors every hop
// of a path walk to the previous hop's already-open fd, so there is never a
// separate, independently-timed name-based lookup for a race to win
// against. Until this task, the read side (`scan_workspace_tree`, used by
// both C022's reconciliation pass and its inotify single-entry handlers)
// instead checked `symlink_metadata(path)` and then made a *second*,
// independent syscall (`fs::read`/recursion) against that same `path`
// string -- a real TOCTOU gap between the check and the use, tracked as
// `docs/DEBT-REGISTER.md` row R-3. The functions below generalize the
// write side's `open_root_dir`/`openat_dir_nofollow`/`openat_leaf_nofollow`
// pattern to a recursive **read** walk: every directory is opened once, by
// fd, from its parent's already-open fd; every directory's entries are
// enumerated via `fdopendir`/`readdir` against that same fd (never
// `std::fs::read_dir` on a path string); and every entry -- file or
// subdirectory -- is itself opened via `openat(..., O_NOFOLLOW)` relative
// to the containing directory's fd before its type is even inspected, so
// the *kernel* refuses to hand back an fd for a symlink at all, atomically,
// rather than this code checking a type first and trusting it a syscall
// later.

/// What a single `openat(..., O_NOFOLLOW)` attempt against one directory
/// entry, from its containing directory's already-open fd, turned out to
/// be. Never constructed from a `stat`/`symlink_metadata` call on a path
/// string -- always from `fstat`ing (or simply successfully obtaining) an
/// fd that a `O_NOFOLLOW` open already committed to.
enum EntryOpen {
    Dir(OwnedFd),
    File(OwnedFd),
    /// The kernel refused to open this entry with `O_NOFOLLOW` because it
    /// is currently a symlink -- whether pre-existing or swapped in by a
    /// concurrent operation between this walk discovering the entry's name
    /// and this open call. Never followed, per this store's read-side
    /// symlink policy.
    Symlink,
    /// Raced against a concurrent delete between this walk discovering the
    /// entry's name and this open call -- not an error, just nothing left
    /// to read here.
    Vanished,
    /// Opened, but neither a directory nor a regular file (device, FIFO,
    /// socket, ...) -- same "skip and log" policy the pre-fix code applied
    /// after a `symlink_metadata` check, now determined from the open fd's
    /// `fstat` result instead.
    Other,
}

/// Opens `name` as an entry of the directory referenced by `parent_fd`,
/// entirely via fd-anchored `openat` calls -- never a fresh, name-based
/// lookup from `workspace_dir` or the filesystem root, and never a
/// `symlink_metadata` call on a reconstructed path string.
///
/// Two attempts, in order, exactly mirroring how the write side's
/// `openat_dir_nofollow` is itself just one call of this same shape:
///
///  1. `openat(parent_fd, name, O_DIRECTORY | O_NOFOLLOW)` -- succeeds iff
///     `name` is, at the instant of *this* syscall, a real directory and
///     not a symlink. `ENOTDIR` means it exists but isn't a directory right
///     now (a file or a symlink); fall through to the second attempt.
///  2. `openat(parent_fd, name, O_NOFOLLOW | O_NONBLOCK)` -- a generic,
///     still-`O_NOFOLLOW`-guarded open. This is its *own*, independently
///     enforced refusal-to-follow: if `name` was swapped for a symlink in
///     the gap between attempt 1 and this attempt, this call fails closed
///     (`ELOOP`) exactly the same way attempt 1 would have. `O_NONBLOCK`
///     only matters if `name` turns out to be a FIFO with no writer (it has
///     no effect on reads of a regular file or on directory opens), so a
///     hostile or accidental FIFO planted in the projection can never hang
///     this walk.
///
/// Every branch here is safe independently of what any *other* branch or
/// any *earlier* check observed: there is no step in this function that
/// trusts a previous syscall's result when opening the real target, only
/// the kernel's own atomic enforcement of `O_NOFOLLOW` on the specific
/// `openat` call that actually obtains the fd used afterward.
fn open_entry_nofollow(parent_fd: RawFd, name: &OsStr) -> Result<EntryOpen> {
    let c = cstring_component(name)?;

    let dir_flags = libc::O_RDONLY | libc::O_DIRECTORY | libc::O_NOFOLLOW | libc::O_CLOEXEC;
    let dir_fd = unsafe { libc::openat(parent_fd, c.as_ptr(), dir_flags, 0) };
    if dir_fd >= 0 {
        return Ok(EntryOpen::Dir(unsafe { OwnedFd::from_raw_fd(dir_fd) }));
    }
    let dir_err = std::io::Error::last_os_error();
    match dir_err.raw_os_error() {
        Some(libc::ELOOP) => return Ok(EntryOpen::Symlink),
        Some(libc::ENOENT) => return Ok(EntryOpen::Vanished),
        Some(libc::ENOTDIR) => {} // exists, not (currently) a directory -- fall through
        _ => {
            return Err(dir_err)
                .with_context(|| format!("openat {name:?} (O_DIRECTORY|O_NOFOLLOW)"))
        }
    }

    let file_flags = libc::O_RDONLY | libc::O_NOFOLLOW | libc::O_NONBLOCK | libc::O_CLOEXEC;
    let fd = unsafe { libc::openat(parent_fd, c.as_ptr(), file_flags, 0) };
    if fd < 0 {
        let err = std::io::Error::last_os_error();
        return match err.raw_os_error() {
            Some(libc::ELOOP) => Ok(EntryOpen::Symlink),
            Some(libc::ENOENT) => Ok(EntryOpen::Vanished),
            _ => Err(err).with_context(|| format!("openat {name:?} (O_NOFOLLOW)")),
        };
    }
    let owned = unsafe { OwnedFd::from_raw_fd(fd) };
    let mut st: libc::stat = unsafe { std::mem::zeroed() };
    if unsafe { libc::fstat(owned.as_raw_fd(), &mut st) } != 0 {
        return Err(std::io::Error::last_os_error()).with_context(|| format!("fstat {name:?}"));
    }
    if st.st_mode & libc::S_IFMT == libc::S_IFREG {
        Ok(EntryOpen::File(owned))
    } else {
        Ok(EntryOpen::Other)
    }
}

/// Opens `dir` (a workspace projection root) as an `O_DIRECTORY` fd by
/// name -- the read side's counterpart to `open_root_dir`, tolerant of the
/// directory already having been removed (a workspace torn down mid-scan,
/// or a stale reconciliation tick racing `drop_workspace`) rather than
/// treating that as an error. `dir` is the same non-attacker-controlled
/// trust anchor `open_root_dir` documents, so `O_NOFOLLOW` is
/// correspondingly not required here either.
fn open_scan_root_dir(dir: &Path) -> Result<Option<OwnedFd>> {
    let c = CString::new(dir.as_os_str().as_bytes())
        .map_err(|_| anyhow!("workspace root path contains a NUL byte: {dir:?}"))?;
    let fd =
        unsafe { libc::open(c.as_ptr(), libc::O_RDONLY | libc::O_DIRECTORY | libc::O_CLOEXEC) };
    if fd < 0 {
        let err = std::io::Error::last_os_error();
        if err.kind() == std::io::ErrorKind::NotFound {
            return Ok(None);
        }
        return Err(err).with_context(|| format!("open workspace root {dir:?} for scan"));
    }
    Ok(Some(unsafe { OwnedFd::from_raw_fd(fd) }))
}

/// Fd-anchors from `workspace_dir` down to `rel_dir`, one component at a
/// time, entirely via `open_entry_nofollow` -- the read-side counterpart to
/// `materialize_write_via_fd_walk`'s directory descent. `Ok(None)` means
/// the path is not currently reachable as a real directory (removed, or a
/// component now names a symlink or non-directory) -- a graceful,
/// documented no-op matching this module's existing "directory removed
/// mid-scan" tolerance, not an error, since a live workspace's real
/// filesystem changing out from under a scan is an expected race, not a
/// bug.
fn open_scan_dir_anchored(workspace_dir: &Path, rel_dir: &str) -> Result<Option<OwnedFd>> {
    let mut current = match open_scan_root_dir(workspace_dir)? {
        Some(fd) => fd,
        None => return Ok(None),
    };
    if rel_dir.is_empty() {
        return Ok(Some(current));
    }
    for component in Path::new(rel_dir).iter() {
        match open_entry_nofollow(current.as_raw_fd(), component)? {
            EntryOpen::Dir(next) => current = next,
            EntryOpen::Symlink => {
                tracing::warn!(
                    dir = %rel_dir,
                    component = ?component,
                    "write-back scan: a component of the scan path is a symlink -- skipped, never followed"
                );
                return Ok(None);
            }
            EntryOpen::Vanished => return Ok(None),
            EntryOpen::File(_) | EntryOpen::Other => {
                tracing::warn!(
                    dir = %rel_dir,
                    component = ?component,
                    "write-back scan: a component of the scan path is no longer a directory -- skipped"
                );
                return Ok(None);
            }
        }
    }
    Ok(Some(current))
}

/// Duplicates `dir_fd` (via `F_DUPFD_CLOEXEC`) and hands the duplicate to
/// `fdopendir`, so the caller's own fd for the directory remains valid and
/// usable for further `openat` calls (each entry is opened relative to the
/// *original* fd) while a separate fd, owned by the returned `DIR*` stream,
/// is used purely for enumeration (`readdir`). This is what lets one
/// already-open directory fd both be listed *and* have its entries opened
/// by name, without a second, independently-timed name-based lookup for
/// either operation.
fn fdopendir_dup(dir_fd: RawFd) -> Result<*mut libc::DIR> {
    let dup_fd = unsafe { libc::fcntl(dir_fd, libc::F_DUPFD_CLOEXEC, 0) };
    if dup_fd < 0 {
        return Err(std::io::Error::last_os_error()).context("dup directory fd for fdopendir");
    }
    let dirp = unsafe { libc::fdopendir(dup_fd) };
    if dirp.is_null() {
        let err = std::io::Error::last_os_error();
        unsafe { libc::close(dup_fd) };
        return Err(err).context("fdopendir");
    }
    Ok(dirp)
}

/// RAII wrapper around a `DIR*` obtained from `fdopendir_dup`, so every
/// return path out of `scan_dir_into` (including via `?`) still calls
/// `closedir` exactly once.
struct OwnedDirStream(*mut libc::DIR);

impl Drop for OwnedDirStream {
    fn drop(&mut self) {
        unsafe {
            libc::closedir(self.0);
        }
    }
}

/// Reads the next directory entry's name from `dirp` (a still-open `DIR*`
/// from `fdopendir_dup`), skipping the synthetic `.`/`..` entries.
/// `Ok(None)` means end-of-stream, not an error -- distinguished from a
/// genuine `readdir` error by resetting `errno` to `0` immediately before
/// the call, per the POSIX-documented technique (a `NULL` return with
/// `errno` still `0` is end-of-stream; non-zero is a real error).
fn next_dir_entry_name(dirp: *mut libc::DIR) -> Result<Option<std::ffi::OsString>> {
    loop {
        unsafe {
            *libc::__errno_location() = 0;
        }
        let entry_ptr = unsafe { libc::readdir(dirp) };
        if entry_ptr.is_null() {
            let errno = unsafe { *libc::__errno_location() };
            if errno == 0 {
                return Ok(None);
            }
            return Err(std::io::Error::from_raw_os_error(errno)).context("readdir");
        }
        let entry = unsafe { &*entry_ptr };
        let bytes = unsafe { CStr::from_ptr(entry.d_name.as_ptr()) }.to_bytes();
        if bytes == b"." || bytes == b".." {
            continue;
        }
        return Ok(Some(OsStr::from_bytes(bytes).to_os_string()));
    }
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
/// side's counterpart to `materialize_write` -- read here, not written.
///
/// # Path construction (traversal risk)
///
/// Every logical path this function produces is built by joining an
/// already-walked relative-path prefix with a single directory-entry name
/// obtained from `readdir` (`next_dir_entry_name`). A kernel-reported
/// directory-entry name can never contain a `/` byte, and `.`/`..` are
/// filtered out before this function ever sees them -- so, unlike
/// `secure_relative_path`'s job on the write side (parsing and validating
/// a caller-supplied, arbitrary *string*), there is no string-based
/// `..`/absolute-path escape to check for here, because nothing on this
/// path is ever parsed from a raw string handed to us by anything outside
/// this process.
///
/// # Symlink handling and TOCTOU (fd-anchored -- closes R-3)
///
/// Every entry -- file or directory -- is opened via `openat(...,
/// O_NOFOLLOW)` relative to its containing directory's already-open fd
/// (`open_entry_nofollow`) *before* its type is inspected; the kernel
/// refuses the open atomically if the entry is currently a symlink,
/// exactly the same guarantee `materialize_write_via_fd_walk` gives the
/// write side. There is no separate `symlink_metadata`-then-`fs::read`/
/// recursion pair of syscalls left for a race to land in between: the
/// fd this function reads from, or recurses into, is the *same* fd the
/// `O_NOFOLLOW` open already committed to, not a fresh, name-based lookup
/// performed afterward. This closes `docs/DEBT-REGISTER.md` row R-3
/// (task C035) -- see `open_entry_nofollow`'s doc comment for the exact
/// mechanism, and
/// `scan_workspace_tree_race_against_symlink_swap_never_reads_outside_content`
/// (this module's tests) for the race-stress evidence that the pre-fix,
/// separate-syscall version of this function was not actually closed
/// against this class of race, and that this version is.
///
/// Anything the kernel reports as a symlink (`EntryOpen::Symlink`) is
/// skipped entirely (logged at `warn`, not read, not recursed into) --
/// the same skip-and-log *policy* the pre-fix code applied, just enforced
/// by construction now rather than by a separate, independently-timed
/// check.
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
    if let Some(dir_fd) = open_scan_dir_anchored(workspace_dir, start_rel)? {
        scan_dir_into(&dir_fd, start_rel, &mut files, &mut dirs)?;
    }
    Ok(FsScanResult { files, dirs })
}

/// Enumerates `dir_fd`'s entries (via `fdopendir_dup`/`next_dir_entry_name`)
/// and applies [`apply_scan_entry`] to each. `dir_fd` itself was obtained
/// via a fd-anchored `openat` hop (either `open_scan_dir_anchored`'s root
/// open, or a prior recursive call's `open_entry_nofollow` for a
/// subdirectory) -- never re-resolved by name here.
fn scan_dir_into(
    dir_fd: &OwnedFd,
    rel_dir: &str,
    files: &mut HashMap<String, Vec<u8>>,
    dirs: &mut Vec<String>,
) -> Result<()> {
    let dirp = fdopendir_dup(dir_fd.as_raw_fd())
        .with_context(|| format!("fdopendir rel {rel_dir:?}"))?;
    let stream = OwnedDirStream(dirp);
    while let Some(name) =
        next_dir_entry_name(stream.0).with_context(|| format!("readdir rel {rel_dir:?}"))?
    {
        apply_scan_entry(dir_fd, rel_dir, &name, files, dirs)?;
    }
    Ok(())
}

/// Opens one directory entry (`name`, a child of `dir_fd`) via
/// `open_entry_nofollow` and records it: recurses into a subdirectory,
/// reads a regular file's content via the same fd `open_entry_nofollow`
/// already obtained (never `std::fs::read` on a path string), or logs and
/// skips a symlink/vanished/other-typed entry per this store's read-side
/// policy.
fn apply_scan_entry(
    dir_fd: &OwnedFd,
    rel_dir: &str,
    name: &OsStr,
    files: &mut HashMap<String, Vec<u8>>,
    dirs: &mut Vec<String>,
) -> Result<()> {
    let name_str = match name.to_str() {
        Some(s) => s,
        None => {
            tracing::warn!(
                dir = %rel_dir,
                name = ?name,
                "write-back scan: skipping entry with non-UTF-8 filename"
            );
            return Ok(());
        }
    };
    let rel_path = if rel_dir.is_empty() {
        name_str.to_string()
    } else {
        format!("{rel_dir}/{name_str}")
    };

    match open_entry_nofollow(dir_fd.as_raw_fd(), name)? {
        EntryOpen::Dir(child_fd) => {
            dirs.push(rel_path.clone());
            scan_dir_into(&child_fd, &rel_path, files, dirs)?;
        }
        EntryOpen::File(file_fd) => {
            let mut f = std::fs::File::from(file_fd);
            let mut content = Vec::new();
            f.read_to_end(&mut content)
                .with_context(|| format!("read {rel_path:?}"))?;
            files.insert(rel_path, content);
        }
        EntryOpen::Symlink => {
            tracing::warn!(
                path = %rel_path,
                "write-back scan: skipping symlink planted directly in shadow workspace \
                 projection (never followed) -- see scan_workspace_tree's doc comment"
            );
        }
        EntryOpen::Vanished => {}
        EntryOpen::Other => {
            tracing::warn!(
                path = %rel_path,
                "write-back scan: skipping non-regular, non-directory entry \
                 (device/FIFO/socket) in shadow workspace projection"
            );
        }
    }
    Ok(())
}

/// Outcome of [`read_real_file`]: the fd-anchored, single-file counterpart
/// to [`scan_workspace_tree`], used by `writeback.rs`'s inotify event
/// handler (`sync_file`) instead of a separate `symlink_metadata`/
/// `fs::read` pair of syscalls against a path string.
pub(crate) enum SingleFileScan {
    Content(Vec<u8>),
    Symlink,
    Vanished,
    Other,
}

/// Fd-anchors from `workspace_dir`, through `rel_path`'s directory
/// components (via `open_scan_dir_anchored`), to its final component (via
/// `open_entry_nofollow`), and reads that file's content through the same
/// fd the `O_NOFOLLOW` open obtained -- the single-file counterpart to
/// `scan_workspace_tree`'s recursive walk, sharing the exact same
/// underlying primitives (`open_entry_nofollow`, `open_scan_dir_anchored`)
/// rather than a parallel, divergent implementation.
pub(crate) fn read_real_file(workspace_dir: &Path, rel_path: &str) -> Result<SingleFileScan> {
    let (dir_rel, leaf_str) = match rel_path.rsplit_once('/') {
        Some((d, l)) => (d, l),
        None => ("", rel_path),
    };
    let dir_fd = match open_scan_dir_anchored(workspace_dir, dir_rel)? {
        Some(fd) => fd,
        None => return Ok(SingleFileScan::Vanished),
    };
    let leaf = OsStr::new(leaf_str);
    match open_entry_nofollow(dir_fd.as_raw_fd(), leaf)? {
        EntryOpen::File(fd) => {
            let mut f = std::fs::File::from(fd);
            let mut content = Vec::new();
            f.read_to_end(&mut content)
                .with_context(|| format!("read {rel_path:?} under {workspace_dir:?}"))?;
            Ok(SingleFileScan::Content(content))
        }
        EntryOpen::Symlink => Ok(SingleFileScan::Symlink),
        EntryOpen::Vanished => Ok(SingleFileScan::Vanished),
        EntryOpen::Dir(_) | EntryOpen::Other => Ok(SingleFileScan::Other),
    }
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

    // --- C041: parent-commit tracking per workspace -----------------------

    /// A workspace created with no base (`create(None)`) has never had a
    /// commit, so its very first `commit()` call must produce a real root
    /// commit -- zero parents -- not an accidental one, and not fail. This
    /// is the "must not regress" half of C041's brief: the pre-fix code
    /// always built an empty `parents` vec, so this case already passed by
    /// construction, but the fix must preserve it exactly (rather than, say,
    /// only handling the "has a base" case and leaving a fresh workspace's
    /// first commit broken).
    #[test]
    fn first_commit_in_fresh_workspace_is_a_root_commit() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        s.write_file(id, "hello.txt", b"v1").unwrap();
        let (_tree, commit_oid) = s.commit(id, "root commit").unwrap();

        let repo = s.repo.lock();
        let commit = repo.find_commit(commit_oid).unwrap();
        assert_eq!(
            commit.parent_count(),
            0,
            "a workspace's first commit, with no base, must be a genuine root commit"
        );
    }

    /// Two sequential commits in the *same* workspace must form a real
    /// parent-child chain: the second commit's sole parent must be the
    /// first commit's oid, not orphaned. This is C041's core acceptance
    /// criterion -- `git log` (or, here, `git2::Commit::parent_id`) must
    /// show linkage, and this is exactly what C042's three-way merge will
    /// walk to find a common ancestor.
    #[test]
    fn sequential_commits_in_one_workspace_form_a_parent_chain() {
        let s = store();
        let (id, _) = s.create(None).unwrap();

        s.write_file(id, "hello.txt", b"v1").unwrap();
        let (_tree1, commit1) = s.commit(id, "first commit").unwrap();

        s.write_file(id, "hello.txt", b"v2").unwrap();
        let (_tree2, commit2) = s.commit(id, "second commit").unwrap();

        assert_ne!(commit1, commit2, "each commit must have a distinct oid");

        let repo = s.repo.lock();
        let c1 = repo.find_commit(commit1).unwrap();
        assert_eq!(
            c1.parent_count(),
            0,
            "the first commit is still a root commit"
        );

        let c2 = repo.find_commit(commit2).unwrap();
        assert_eq!(
            c2.parent_count(),
            1,
            "the second commit must have exactly one parent"
        );
        assert_eq!(
            c2.parent_id(0).unwrap(),
            commit1,
            "the second commit's parent must be the workspace's own first commit"
        );
    }

    /// A third commit continues the same chain (parent linkage isn't a
    /// one-shot fluke of exactly two commits): `head` must keep advancing
    /// across every successful `commit()` call.
    #[test]
    fn third_commit_continues_the_same_chain() {
        let s = store();
        let (id, _) = s.create(None).unwrap();

        s.write_file(id, "f.txt", b"v1").unwrap();
        let (_, c1) = s.commit(id, "c1").unwrap();
        s.write_file(id, "f.txt", b"v2").unwrap();
        let (_, c2) = s.commit(id, "c2").unwrap();
        s.write_file(id, "f.txt", b"v3").unwrap();
        let (_, c3) = s.commit(id, "c3").unwrap();

        let repo = s.repo.lock();
        assert_eq!(repo.find_commit(c2).unwrap().parent_id(0).unwrap(), c1);
        assert_eq!(repo.find_commit(c3).unwrap().parent_id(0).unwrap(), c2);
    }

    /// A workspace created *from* an existing commit (`create(Some(sha))`)
    /// must chain its own first commit onto that base commit, rather than
    /// treating "first commit in this workspace" and "root commit" as
    /// synonyms -- the base commit is real history, not something this
    /// workspace's chain should disconnect from.
    #[test]
    fn workspace_created_from_base_commit_chains_onto_it() {
        let s = store();
        let (seed_id, _) = s.create(None).unwrap();
        s.write_file(seed_id, "seed.txt", b"seed").unwrap();
        let (_, base_commit) = s.commit(seed_id, "base").unwrap();

        let (id, _) = s.create(Some(base_commit.to_string())).unwrap();
        s.write_file(id, "new.txt", b"new").unwrap();
        let (_, next_commit) = s.commit(id, "continues base").unwrap();

        let repo = s.repo.lock();
        let next = repo.find_commit(next_commit).unwrap();
        assert_eq!(
            next.parent_count(),
            1,
            "a workspace seeded from a base commit must chain onto it, not root itself"
        );
        assert_eq!(next.parent_id(0).unwrap(), base_commit);
    }

    // --- SEV-C041-001: per-workspace commit serialization -----------------
    //
    // Security Engineer finding (independent review of C041, reproduced
    // 5/5 runs with an adversarial 8-thread probe against the pre-fix
    // code): unsynchronized concurrent `commit()` calls against the SAME
    // workspace could silently orphan a commit from the parent chain --
    // both racers read the same stale `ws.head`, both commit successfully
    // against that same parent, and whichever advanced `ws.head` last
    // "won", leaving the other's commit unreachable by walking `head`
    // backwards (still present in the ODB, invisible to any ancestor
    // walk). See `commit`'s doc comment and `docs/DEBT-REGISTER.md` row
    // R-7. The two tests below are that probe, made a permanent regression
    // test, plus a companion test proving the fix does not regress
    // cross-workspace concurrency back to a single global commit lock.

    /// The adversarial probe itself: 8 threads, synchronized via a
    /// `Barrier` so they all call `commit()` against the SAME workspace as
    /// close to simultaneously as this platform allows, repeated over
    /// several independent iterations (fresh store/workspace each time) to
    /// build confidence this is not a one-shot fluke either way. After all
    /// 8 threads complete, walks `parent_id`/`parent_count` from the
    /// workspace's final `head` back to the root and asserts ALL 8
    /// commits are reachable -- not just "no panic", zero lost commits.
    ///
    /// Confirmed (see this task's execution notes in
    /// `docs/tasks/task-C041.md`) to reliably FAIL against the pre-fix
    /// `commit()` (no per-workspace serialization) and to reliably PASS
    /// against the fix below.
    #[test]
    fn concurrent_commits_against_one_workspace_never_lose_a_commit() {
        use std::collections::HashSet;
        use std::sync::Barrier;

        const THREADS: usize = 8;
        const ITERATIONS: usize = 20;

        for iteration in 0..ITERATIONS {
            let s = store();
            let (id, _) = s.create(None).unwrap();

            let barrier = Arc::new(Barrier::new(THREADS));
            let handles: Vec<_> = (0..THREADS)
                .map(|i| {
                    let s = s.clone();
                    let barrier = barrier.clone();
                    std::thread::spawn(move || {
                        // Distinct content per thread so each thread's write
                        // is unambiguous; the barrier below is what forces
                        // all 8 `commit()` calls to actually overlap rather
                        // than happening to run sequentially by scheduling
                        // luck.
                        s.write_file(id, "f.txt", format!("thread-{i}").as_bytes())
                            .unwrap();
                        barrier.wait();
                        s.commit(id, &format!("iteration {iteration} thread {i}"))
                            .unwrap()
                    })
                })
                .collect();

            let mut commit_oids: Vec<Oid> = Vec::with_capacity(THREADS);
            for h in handles {
                let (_, commit_oid) = h.join().unwrap();
                commit_oids.push(commit_oid);
            }
            assert_eq!(
                commit_oids.iter().collect::<HashSet<_>>().len(),
                THREADS,
                "iteration {iteration}: all 8 commit() calls must produce distinct oids"
            );

            let head = {
                let wss = s.workspaces.lock();
                wss.get(&id)
                    .unwrap()
                    .head
                    .expect("workspace must have a head after 8 successful commits")
            };
            let repo = s.repo.lock();
            let mut reachable: HashSet<Oid> = HashSet::new();
            let mut cursor = Some(head);
            while let Some(oid) = cursor {
                reachable.insert(oid);
                let commit = repo.find_commit(oid).unwrap();
                cursor = match commit.parent_count() {
                    0 => None,
                    1 => Some(commit.parent_id(0).unwrap()),
                    n => panic!(
                        "iteration {iteration}: commit {oid} has {n} parents; this chain must \
                         always be linear"
                    ),
                };
            }
            drop(repo);

            for (i, oid) in commit_oids.iter().enumerate() {
                assert!(
                    reachable.contains(oid),
                    "iteration {iteration}: thread {i}'s commit {oid} is NOT reachable by \
                     walking parent_id from the workspace's final head -- this is \
                     SEV-C041-001 (a concurrent commit silently orphaned from the chain)"
                );
            }
            assert_eq!(
                reachable.len(),
                THREADS,
                "iteration {iteration}: expected a linear chain of exactly {THREADS} commits \
                 reachable from head, got {}",
                reachable.len()
            );
        }
    }

    /// Companion to the probe above: proves the per-workspace lock does
    /// NOT regress into a single global commit lock. A commit against
    /// workspace `b` must complete promptly even while workspace `a`'s
    /// commit lock is being held (simulated here by acquiring `a`'s
    /// `commit_locks` entry directly, standing in for an in-flight
    /// `commit()` call against `a`) -- if the fix had accidentally
    /// serialized all workspaces behind one lock, this would hang until
    /// the held guard is dropped (never, within this test), and the
    /// `recv_timeout` below would fail. This is exactly the throttling
    /// C043's connection-pool change exists to remove, so this fix must
    /// not reintroduce it at this layer.
    #[test]
    fn concurrent_commits_against_different_workspaces_are_not_serialized() {
        let s = store();
        let (a, _) = s.create(None).unwrap();
        let (b, _) = s.create(None).unwrap();
        s.write_file(a, "f.txt", b"a").unwrap();
        s.write_file(b, "f.txt", b"b").unwrap();

        let a_lock = {
            s.commit_locks
                .lock()
                .entry(a)
                .or_insert_with(|| Arc::new(Mutex::new(())))
                .clone()
        };
        let _held = a_lock.lock();

        let (tx, rx) = std::sync::mpsc::channel();
        let s2 = s.clone();
        std::thread::spawn(move || {
            let result = s2.commit(b, "commit to workspace b");
            let _ = tx.send(result);
        });

        let result = rx.recv_timeout(std::time::Duration::from_secs(5)).expect(
            "commit() against a DIFFERENT workspace blocked while an unrelated \
                 workspace's commit lock was held -- the per-workspace lock must never \
                 regress into a single global commit lock",
        );
        result.expect("commit against workspace b must still succeed");
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

    // --- C023: crash-safe startup reconciliation sweep --------------------

    /// The deterministic, library-level crash test required by C023. A real
    /// `kill -9` gives zero opportunity for any in-process cleanup --
    /// including any `Drop` impl -- to run at all, so this test must not
    /// (and does not) rely on `ShadowStore`'s `Drop` behavior (it doesn't
    /// even implement one) to simulate that: it constructs a real
    /// `ShadowStore`, materializes a real workspace directory on a real temp
    /// `storage_root`, then simply lets that `ShadowStore` value go out of
    /// scope *without ever calling `drop_workspace`* -- exactly what happens
    /// to in-process state at the instant of an unclean crash, since nothing
    /// in this crate registers an `atexit`/signal-driven cleanup hook either
    /// (see `main.rs` and this module's shutdown-behavior tests). A second,
    /// independent `ShadowStore::new` is then constructed against the exact
    /// same `storage_root` -- standing in for a process restart against the
    /// same, still-mounted tmpfs directory -- and the assertion is that its
    /// own startup sweep, not any cleanup from the first instance, is what
    /// removes the stale directory.
    #[test]
    fn fresh_store_sweeps_directory_left_by_simulated_unclean_crash() {
        let storage_root = tempfile::tempdir().unwrap().keep();

        let crashed_workspace_dir = {
            let crashed = ShadowStore::new(storage_root.clone()).unwrap();
            let (id, _) = crashed.create(None).unwrap();
            crashed
                .write_file(id, "hello.txt", b"pre-crash content")
                .unwrap();
            let ws_dir = crashed.workspace_dir(id);
            assert!(
                ws_dir.is_dir(),
                "workspace directory must be materialized before the simulated crash"
            );
            // Simulate an unclean crash: `crashed` is discarded here, with no
            // call to `drop_workspace` and no reliance on any `Drop` impl
            // (there isn't one) -- this is the entire simulated "crash".
            drop(crashed);
            ws_dir
        };
        assert!(
            crashed_workspace_dir.is_dir(),
            "the stale directory must still be present immediately after the simulated crash -- \
             nothing in this test relied on any cleanup running for it yet"
        );

        // Simulate a process restart: a brand-new `ShadowStore` against the
        // same, still-populated `storage_root`.
        let restarted = ShadowStore::new(storage_root.clone()).unwrap();

        assert!(
            !crashed_workspace_dir.exists(),
            "the fresh instance's startup sweep must remove a per-workspace directory left \
             behind by an unclean crash of a previous instance"
        );
        // `bare.git` (the one persistent, non-workspace entry) must survive
        // the sweep.
        assert!(
            storage_root.join("bare.git").is_dir(),
            "the sweep must never remove the persistent bare.git ODB directory"
        );
        // The fresh instance must otherwise be fully usable (sweep does not
        // corrupt the ODB it just opened).
        let (id, _) = restarted.create(None).unwrap();
        assert!(restarted.workspace_dir(id).is_dir());
    }

    #[test]
    fn fresh_store_sweeps_multiple_stale_directories_and_leaves_bare_git() {
        let storage_root = tempfile::tempdir().unwrap().keep();

        let (dir_a, dir_b) = {
            let crashed = ShadowStore::new(storage_root.clone()).unwrap();
            let (a, _) = crashed.create(None).unwrap();
            let (b, _) = crashed.create(None).unwrap();
            let dirs = (crashed.workspace_dir(a), crashed.workspace_dir(b));
            drop(crashed); // simulated crash: no drop_workspace for either
            dirs
        };
        assert!(dir_a.is_dir());
        assert!(dir_b.is_dir());

        let _restarted = ShadowStore::new(storage_root.clone()).unwrap();

        assert!(!dir_a.exists(), "first stale workspace dir must be swept");
        assert!(!dir_b.exists(), "second stale workspace dir must be swept");
        assert!(storage_root.join("bare.git").is_dir());
    }

    #[test]
    fn new_is_a_no_op_sweep_on_an_already_clean_storage_root() {
        let storage_root = tempfile::tempdir().unwrap().keep();

        // First construction creates only `bare.git` -- nothing else exists
        // to sweep.
        let _first = ShadowStore::new(storage_root.clone()).unwrap();
        let entries_after_first: Vec<_> = std::fs::read_dir(&storage_root)
            .unwrap()
            .map(|e| e.unwrap().file_name())
            .collect();
        assert_eq!(
            entries_after_first,
            vec![std::ffi::OsString::from("bare.git")],
            "a clean storage root must contain only bare.git before any workspace is created"
        );

        // A second construction against the same, already-clean root must
        // not remove `bare.git` and must remain fully usable.
        let second = ShadowStore::new(storage_root.clone()).unwrap();
        assert!(storage_root.join("bare.git").is_dir());
        let (id, _) = second.create(None).unwrap();
        assert!(second.workspace_dir(id).is_dir());
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

    // --- C035 (closes R-3): fd-anchored read-side TOCTOU fix -------------

    #[test]
    fn scan_workspace_tree_skips_symlink_planted_at_intermediate_component() {
        // Deterministic regression coverage: a symlink at an intermediate
        // directory component, pre-planted (not concurrently) before the
        // scan runs. NOTE for reviewers, same caveat as the write side's
        // `write_file_refuses_to_follow_symlink_planted_at_intermediate_component`:
        // this static scenario was ALSO already caught by the pre-fix
        // `symlink_metadata`-then-`fs::read_dir` check (the check runs
        // before the racer has any chance to act), so this test alone does
        // not distinguish old from new. It is included as direct,
        // deterministic coverage of `scan_workspace_tree`/`scan_dir_into`
        // specifically. The race-stress test below is what actually
        // distinguishes pre-fix from post-fix behavior.
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        let outside = tempfile::tempdir().unwrap();
        std::fs::write(outside.path().join("secret.txt"), b"do-not-read").unwrap();
        std::os::unix::fs::symlink(outside.path(), ws_dir.join("sub")).unwrap();
        std::fs::write(ws_dir.join("sentinel.txt"), b"ok").unwrap();

        let scan = scan_workspace_tree(&ws_dir, "").unwrap();
        assert!(
            !scan.files.keys().any(|p| p.contains("secret.txt")),
            "a symlinked intermediate directory component must never be recursed into"
        );
        assert_eq!(
            scan.files.get("sentinel.txt").map(Vec::as_slice),
            Some(b"ok".as_slice())
        );
    }

    #[test]
    fn scan_workspace_tree_skips_symlink_planted_at_leaf() {
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        let outside = tempfile::tempdir().unwrap();
        let secret = outside.path().join("secret.txt");
        std::fs::write(&secret, b"do-not-read").unwrap();
        std::os::unix::fs::symlink(&secret, ws_dir.join("link.txt")).unwrap();

        let scan = scan_workspace_tree(&ws_dir, "").unwrap();
        assert!(
            !scan.files.contains_key("link.txt"),
            "a symlink planted directly at a scanned leaf must never be read"
        );
    }

    #[test]
    fn read_real_file_skips_symlink_planted_at_leaf() {
        // Direct unit coverage for `read_real_file` (the single-file
        // primitive `writeback.rs`'s `sync_file` uses), independent of the
        // full write-back engine.
        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);

        let outside = tempfile::tempdir().unwrap();
        let secret = outside.path().join("secret.txt");
        std::fs::write(&secret, b"do-not-read").unwrap();
        std::os::unix::fs::symlink(&secret, ws_dir.join("link.txt")).unwrap();

        assert!(
            matches!(
                read_real_file(&ws_dir, "link.txt").unwrap(),
                SingleFileScan::Symlink
            ),
            "read_real_file must report a leaf symlink as Symlink, never read through it"
        );
    }

    #[test]
    fn scan_workspace_tree_race_against_symlink_swap_never_reads_outside_content() {
        // The read-side counterpart to
        // `write_file_race_against_symlink_swap_never_escapes_workspace`,
        // proving R-3 is actually closed (not merely that the deterministic
        // tests above pass, which the pre-fix code already did too, since a
        // *static* symlink was always caught by the old `symlink_metadata`
        // check that ran before any race could act).
        //
        // Why this genuinely distinguishes pre-fix from post-fix: the
        // pre-fix code's `symlink_metadata` check on "race" and its
        // subsequent recursive `std::fs::read_dir(ws_dir.join("race"))`
        // call are two independent syscalls. `std::fs::read_dir` does not
        // use `O_NOFOLLOW` -- if "race" is swapped for a symlink to
        // `outside` in the narrow window between those two calls,
        // `read_dir` transparently follows it and lists `outside`'s
        // contents instead, and the subsequent per-entry
        // `symlink_metadata`/`fs::read` calls (now against `outside`'s own,
        // non-symlink file) happily read the secret's content into the
        // scan result. The post-fix code opens "race" via a single
        // `openat(..., O_DIRECTORY | O_NOFOLLOW)` call and only ever
        // lists/reads through that already-open fd afterward, so a later
        // swap of the "race" name cannot redirect anything this scan does.
        //
        // Getting this race to actually land reliably, in either direction,
        // needed more than a plain "remove, symlink, remove, mkdir" racer
        // (that version was tried first and did not reproduce the leak
        // against the pre-fix code even across 50,000 iterations / 5
        // wall-clock seconds with 8 racer + 4 reader threads -- the
        // pre-fix code's own check-to-use gap, a single `symlink_metadata`
        // call immediately followed by a `read_dir` call with essentially
        // no intervening work, is apparently narrower than that racer's
        // achievable swap frequency allowed it to reliably hit on this
        // hardware). This version instead pre-creates BOTH "race" (a real,
        // empty directory) and "race_link" (a symlink to `outside`)
        // up front, and has the racer threads repeatedly EXCHANGE the two
        // names via `renameat2(..., RENAME_EXCHANGE)` -- a single, atomic
        // syscall per swap, with no intermediate "name doesn't exist yet"
        // gap at all, so "race" spends very close to exactly half of all
        // wall-clock time as the real directory and half as the symlink,
        // at the racer's maximum achievable swap frequency. With this
        // change, this test was confirmed (see the MR description for the
        // actual before/after run output) to reliably fail in well under
        // 200ms when spliced onto the pre-fix commit, and to reliably pass
        // against the fd-anchored fix below.
        //
        // This does NOT assert the race window is hit on any particular
        // iteration/instant (that would be flaky by construction) -- only
        // that across the whole `deadline` window, content from outside
        // the workspace root is NEVER observed in a scan result.
        use std::sync::atomic::{AtomicBool, Ordering};
        use std::sync::Arc;

        let s = store();
        let (id, _) = s.create(None).unwrap();
        let ws_dir = s.workspace_dir(id);
        let race_dir = ws_dir.join("race");
        std::fs::create_dir(&race_dir).unwrap();

        let outside = tempfile::tempdir().unwrap();
        let outside_path = outside.path().to_path_buf();
        const SECRET_NAME: &str = "outside_secret.txt";
        std::fs::write(
            outside_path.join(SECRET_NAME),
            b"host-only content, must never be read into a shadow commit",
        )
        .unwrap();

        let race_link = ws_dir.join("race_link");
        std::os::unix::fs::symlink(&outside_path, &race_link).unwrap();

        let stop = Arc::new(AtomicBool::new(false));
        let leaked = Arc::new(AtomicBool::new(false));

        const RACER_THREADS: usize = 8;
        let racers: Vec<_> = (0..RACER_THREADS)
            .map(|_| {
                let racer_stop = stop.clone();
                let path_a = CString::new(race_dir.as_os_str().as_bytes()).unwrap();
                let path_b = CString::new(race_link.as_os_str().as_bytes()).unwrap();
                std::thread::spawn(move || {
                    while !racer_stop.load(Ordering::Relaxed) {
                        unsafe {
                            libc::renameat2(
                                libc::AT_FDCWD,
                                path_a.as_ptr(),
                                libc::AT_FDCWD,
                                path_b.as_ptr(),
                                libc::RENAME_EXCHANGE,
                            );
                        }
                    }
                })
            })
            .collect();

        const READER_THREADS: usize = 4;
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(3);
        let readers: Vec<_> = (0..READER_THREADS)
            .map(|_| {
                let ws_dir = ws_dir.clone();
                let leaked = leaked.clone();
                std::thread::spawn(move || {
                    while std::time::Instant::now() < deadline {
                        if leaked.load(Ordering::Relaxed) {
                            return;
                        }
                        if let Ok(scan) = scan_workspace_tree(&ws_dir, "") {
                            if scan.files.keys().any(|p| p.ends_with(SECRET_NAME)) {
                                leaked.store(true, Ordering::Relaxed);
                                return;
                            }
                        }
                    }
                })
            })
            .collect();
        for r in readers {
            r.join().unwrap();
        }

        stop.store(true, Ordering::Relaxed);
        for r in racers {
            r.join().unwrap();
        }

        assert!(
            !leaked.load(Ordering::Relaxed),
            "scan_workspace_tree read content from outside the workspace root during a \
             concurrent symlink swap of an intermediate directory component -- this is the R-3 \
             TOCTOU gap C035 closes"
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
