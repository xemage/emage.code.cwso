//! Write-back path (C022, `docs/decisions/ADR-012-shadow-workspace-
//! filesystem-projection.md`): reflects filesystem mutations made *directly*
//! at a shadow workspace's real, projected path -- via `sed`, `tee`, `rm`,
//! `mv`, an editor, anything that bypasses this service's own IPC entirely
//! -- back into the in-memory git-shadow blob store (`Workspace.files`), so
//! `commit_shadow` captures them.
//!
//! # Mechanism (per ADR-012, exactly)
//!
//! `inotify`-driven write-back is the **primary** path: one dedicated thread
//! ([`WriteBackEngine::event_loop`]) blocks on `read_events_blocking` and
//! applies each event to the in-memory state as it arrives -- low latency,
//! no additional in-process buffering (see "Durability" below for why that
//! matters). A periodic hash-based reconciliation pass
//! ([`WriteBackEngine::reconcile_loop`]) is the **correctness backstop**,
//! not an equal-weight alternative: it exists specifically to recover from
//! `inotify`'s own documented failure modes, which are real and not merely
//! theoretical:
//!
//!  * `IN_Q_OVERFLOW` under a large write burst (the kernel drops events
//!    when its per-instance queue fills) -- handled explicitly in
//!    [`WriteBackEngine::handle_event`] by forcing an immediate, out-of-band
//!    reconciliation pass across *all* workspaces the moment an overflow is
//!    observed, rather than waiting for the next scheduled tick.
//!  * `inotify` is not recursive: a new subdirectory only starts being
//!    watched once this engine reacts to the `CREATE`/`MOVED_TO` event that
//!    announced it (see `sync_new_subtree`). A fast bulk operation (e.g.
//!    `cp -r` or `tar -x`) can populate that subdirectory's own children
//!    before the new watch is in place, a real, literature-documented
//!    inotify gap for exactly this class of tool -- the reconciliation pass
//!    hash-diffs the whole tree, including subdirectories, so it is not
//!    dependent on any watch having existed at all.
//!
//! # Why write-back never writes back to disk
//!
//! Both the event handler and the reconciliation pass only ever *read* the
//! real filesystem and write into the in-memory `files` map / git ODB;
//! neither ever writes to the real, projected path. This is deliberate, not
//! an oversight: `write_file`'s own disk write (`materialize_write`,
//! `repo.rs`) already keeps the real path in sync for edits made through
//! this service's own IPC, and a write-back path that *also* wrote to disk
//! after reading from it would immediately re-trigger the very `inotify`
//! event it just reacted to -- an unbounded feedback loop. Read-only
//! write-back closes that loop by construction, not by a debounce
//! heuristic. (A side effect: `write_file`'s own writes are *also* observed
//! by this engine, since inotify cannot distinguish "this process wrote
//! that" from "something else wrote that" -- harmless, since re-applying
//! the same content computes the same blob `Oid` and is a no-op update to
//! the `files` map.)
//!
//! # Rename handling: independent delete+create, not cookie-correlated
//!
//! `inotify` offers a "cookie" that, in principle, lets a `MOVED_FROM` event
//! be correlated with its matching `MOVED_TO` event as one logical rename.
//! `handle_event` deliberately does not do this correlation: `MOVED_TO` is
//! routed exactly like `CREATE` ([`WriteBackEngine::sync_file`]/
//! `sync_new_subtree` -- "a file/dir now exists here, sync its current
//! content") and `MOVED_FROM` exactly like `DELETE` (`wb_apply_delete`/
//! `wb_apply_delete_prefix` -- "this path is gone"). A rename is therefore
//! two independent operations, not one atomic move: cookie-correlation would
//! require buffering an incomplete rename indefinitely (the `MOVED_TO` half
//! may never arrive at all -- e.g. a move to an unwatched directory, or out
//! of the workspace entirely), and since identical bytes anywhere always
//! hash to the same blob `Oid` regardless of which path record points at
//! them, "delete old key, create new key" reaches the same correct end state
//! in `Workspace.files` (and hence the resulting git tree) that a "real"
//! move would -- git itself has no first-class rename record either
//! (renames are a diff-time content-similarity heuristic, never stored).
//!
//! POC-DEBT R-5: **accepted, documented race** (independent Tech Lead
//! review, MR !153):
//! the delete-half and create-half are applied as two separate, independent
//! critical sections (each is its own lock acquisition against
//! `ShadowStore`'s internal state), not one atomic operation. A `commit()`
//! that lands in the narrow window between the two -- after
//! `wb_apply_delete` has removed the old key but before the corresponding
//! `wb_apply_write` has inserted the new one -- would observe the workspace
//! as missing the file under BOTH its old and new path: a transient, fully-
//! disappeared state for that one file, resolving itself on the next event
//! or reconciliation pass. This is a narrow, real race (not merely a style
//! preference in choosing independent events over cookie-correlation), it is
//! not currently closed, and it is disclosed here rather than left implicit.
//! A future hardening (not required for v1.0): batch same-tick,
//! same-`inotify`-cookie `MOVED_FROM`/`MOVED_TO` pairs and apply them as one
//! atomic delete+create under a single lock acquisition, removing the gap
//! entirely rather than accepting it.
//!
//! # Durability (disclosed for C023)
//!
//! This engine keeps **no additional in-process queue or buffer** between
//! reading an inotify event and applying it: `event_loop` reads one batch of
//! events and calls [`WriteBackEngine::handle_event`] on each, synchronously,
//! in the same thread, before reading the next batch. So the set of writes
//! that could be lost if this process is killed (`kill -9`) mid-operation is
//! exactly: whatever is still sitting in the *kernel's own* inotify event
//! queue, not yet delivered to a `read()` call, at the instant of the kill --
//! a gap inherent to inotify itself (see `IN_Q_OVERFLOW` above), not a
//! buffering choice made in this module. This task does not introduce any
//! *additional* durability gap on top of that.
//!
//! A larger, pre-existing gap (present since C021, not introduced or
//! widened by this task): `ShadowStore::workspaces` -- and therefore every
//! workspace's entire `files` map, however it was populated -- lives only in
//! this process's memory, with no on-disk/ODB-ref persistence of *which*
//! blobs constitute a given open workspace. If `git-shadow` is killed and
//! restarted, all in-flight (uncommitted) workspace state is lost
//! regardless of whether C022 exists, because `ShadowStore::new` always
//! starts from an empty `workspaces` map (see `repo.rs`). C023 (lifecycle /
//! crash-safety) should treat "does any open workspace's state survive a
//! `git-shadow` process restart at all" as its own, prior question -- today
//! the answer is no, for every workspace, with or without this task's
//! write-back mechanism.
//!
//! # Read-back path safety (symlinks, path construction)
//!
//! See [`crate::repo::scan_workspace_tree`]'s doc comment for the full
//! reasoning (symlink handling, path construction, and the accepted
//! residual TOCTOU on the read side) -- this module's event handler applies
//! the identical policy for the single-file case (see `sync_file`): a
//! `symlink_metadata` check immediately before any read, refusing to follow
//! through anything reporting `is_symlink()`.

use std::collections::HashSet;
use std::path::Path;
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result};
use inotify::{EventMask, EventOwned, Inotify, WatchDescriptor, WatchMask};
use parking_lot::Mutex;
use uuid::Uuid;

use crate::repo::{scan_workspace_tree, ShadowStore};

/// Events this engine asks `inotify` to report for every watched directory.
/// Notably absent: `ACCESS`, `ATTRIB`, `OPEN`, `CLOSE_NOWRITE` -- none of
/// those indicate a content change this store needs to know about.
/// `IN_ISDIR`/`IN_IGNORED`/`IN_Q_OVERFLOW`/`IN_UNMOUNT` are not requestable
/// `WatchMask` bits; the kernel reports them on relevant events regardless
/// of the requested mask (see `EventMask`, which is a strict superset of
/// `WatchMask` for exactly this reason).
fn watch_mask() -> WatchMask {
    WatchMask::CREATE
        | WatchMask::DELETE
        | WatchMask::CLOSE_WRITE
        | WatchMask::MOVED_FROM
        | WatchMask::MOVED_TO
        | WatchMask::MOVE_SELF
        | WatchMask::DELETE_SELF
}

/// Default interval between reconciliation passes (`docs/decisions/ADR-012`
/// requires this pass to exist; the exact cadence is a tuning knob, not a
/// correctness parameter -- `handle_event`'s overflow handling triggers an
/// out-of-band pass immediately when the primary mechanism reports it may
/// have missed something, so this interval only bounds the "quiet" gap
/// where nothing signaled a problem but one exists anyway).
const DEFAULT_RECONCILE_INTERVAL_MS: u64 = 2_000;

#[derive(Clone)]
struct WatchEntry {
    workspace_id: Uuid,
    /// Logical, "/"-joined, workspace-root-relative path of the watched
    /// directory (`""` for the workspace root itself) -- matches the key
    /// shape used by `Workspace.files`.
    rel_dir: String,
}

pub struct WriteBackEngine {
    store: Arc<ShadowStore>,
    /// Handle for adding/removing watches (`inotify_add_watch`/
    /// `inotify_rm_watch`) -- deliberately a *separate* lock from the raw
    /// `Inotify` value used for reading events (see `spawn`'s doc comment).
    /// Both share the same underlying kernel fd (`Watches` is a thin,
    /// `Clone`-able wrapper around an `Arc` of it), but adding/removing a
    /// watch is a fast, non-blocking syscall, categorically different from
    /// `read_events_blocking`, which blocks for however long it takes for
    /// the next event to arrive -- potentially forever if nothing is
    /// watched yet. Guarding both operations with the *same* mutex would
    /// mean the event-reading thread holds that mutex for the entire time
    /// it is blocked waiting for an event, starving every other thread
    /// that ever needs to add or remove a watch (including the very first
    /// `create_workspace` call) -- a real deadlock this engine had in an
    /// earlier draft, not a hypothetical one.
    watch_handle: Mutex<inotify::Watches>,
    watches: Mutex<std::collections::HashMap<WatchDescriptor, WatchEntry>>,
    reconcile_interval: Duration,
}

impl WriteBackEngine {
    /// Starts the write-back engine: opens one `inotify` instance shared by
    /// every workspace, and spawns the two background threads described in
    /// this module's doc comment (event loop + reconciliation loop).
    ///
    /// Note on the `Arc<ShadowStore>` <-> `Arc<WriteBackEngine>` reference
    /// shape: `ShadowStore` holds this engine back via
    /// `ShadowStore::attach_writeback` (a `OnceLock`), forming a genuine
    /// reference cycle with the `Arc<ShadowStore>` held here. This is
    /// deliberate and harmless: both are process-lifetime singletons,
    /// constructed once in `main` and never dropped before process exit, so
    /// there is no leak in any practical sense -- the cycle is reclaimed by
    /// the OS at process exit regardless of Rust's reference counting.
    pub fn spawn(store: Arc<ShadowStore>) -> Result<Arc<Self>> {
        let inotify = Inotify::init().context("initialize inotify instance")?;
        // Obtained once, up front, then cloned into the engine's own field;
        // the raw `inotify` value itself is moved, exclusively, into the
        // event-loop thread below (see the `watch_handle` field's doc
        // comment for why these must not share a lock).
        let watch_handle = inotify.watches();
        let reconcile_interval = std::env::var("CWSO_GIT_SHADOW_RECONCILE_INTERVAL_MS")
            .ok()
            .and_then(|v| v.parse::<u64>().ok())
            .map(Duration::from_millis)
            .unwrap_or(Duration::from_millis(DEFAULT_RECONCILE_INTERVAL_MS));

        let engine = Arc::new(Self {
            store,
            watch_handle: Mutex::new(watch_handle),
            watches: Mutex::new(std::collections::HashMap::new()),
            reconcile_interval,
        });

        {
            let engine = Arc::clone(&engine);
            std::thread::Builder::new()
                .name("gitshadow-inotify".into())
                .spawn(move || engine.event_loop(inotify))
                .context("spawn inotify event-loop thread")?;
        }
        {
            let engine = Arc::clone(&engine);
            std::thread::Builder::new()
                .name("gitshadow-reconcile".into())
                .spawn(move || engine.reconcile_loop())
                .context("spawn reconciliation-loop thread")?;
        }

        Ok(engine)
    }

    /// Registers a freshly created workspace: recursively discovers every
    /// real subdirectory under `ws_dir` (adding an `inotify` watch to each)
    /// and every regular file (syncing it into the blob store immediately).
    ///
    /// The file-sync half is intentionally redundant with what `create()`
    /// already inserted into `Workspace.files` while seeding from a base
    /// tree (same content on disk -> same computed `Oid` -> idempotent
    /// overwrite) -- this redundancy is a deliberate design choice, not an
    /// oversight: it means that immediately after `create_workspace`
    /// returns, this engine's view of the workspace already matches disk
    /// exactly, closing the "brand-new workspace, watches not yet in place"
    /// bootstrap gap before any external edit could even occur.
    pub fn register_workspace(&self, id: Uuid, ws_dir: &Path) {
        match scan_workspace_tree(ws_dir, "") {
            Ok(scan) => {
                for dir in &scan.dirs {
                    self.add_single_watch(id, ws_dir, dir);
                }
                for (path, content) in &scan.files {
                    if let Err(e) = self.store.wb_apply_write(id, path, content) {
                        tracing::warn!(
                            workspace = %id,
                            path = %path,
                            error = %e,
                            "write-back: failed to seed state for pre-existing file at workspace creation"
                        );
                    }
                }
            }
            Err(e) => tracing::warn!(
                workspace = %id,
                error = %e,
                "write-back: failed to register workspace for filesystem write-back; \
                 external edits to this workspace will only be captured by the next \
                 periodic reconciliation pass"
            ),
        }
    }

    /// Tears down every watch this engine holds for `id`. Called by
    /// `drop_workspace` *before* it removes the real directory, so that a
    /// stream of `IN_DELETE`/`IN_IGNORED` events from the teardown itself
    /// doesn't need to do anything (see `handle_event`'s
    /// `workspace_exists` guard for the backstop if a stray event still
    /// arrives after this returns).
    pub fn unregister_workspace(&self, id: Uuid) {
        let stale: Vec<WatchDescriptor> = {
            let watches = self.watches.lock();
            watches
                .iter()
                .filter(|(_, e)| e.workspace_id == id)
                .map(|(wd, _)| wd.clone())
                .collect()
        };
        if stale.is_empty() {
            return;
        }
        {
            let mut w = self.watch_handle.lock();
            for wd in &stale {
                // Best-effort: the directory may already be gone by the
                // time this runs, in which case the kernel has already
                // dropped the watch itself; `remove` erroring here is
                // harmless.
                let _ = w.remove(wd.clone());
            }
        }
        let mut watches = self.watches.lock();
        for wd in &stale {
            watches.remove(wd);
        }
    }

    fn add_single_watch(&self, id: Uuid, ws_dir: &Path, rel_dir: &str) {
        let real_dir = if rel_dir.is_empty() {
            ws_dir.to_path_buf()
        } else {
            ws_dir.join(rel_dir)
        };
        let add_result = self.watch_handle.lock().add(&real_dir, watch_mask());
        match add_result {
            Ok(wd) => {
                self.watches.lock().insert(
                    wd,
                    WatchEntry {
                        workspace_id: id,
                        rel_dir: rel_dir.to_string(),
                    },
                );
            }
            Err(e) => tracing::warn!(
                workspace = %id,
                dir = %rel_dir,
                error = %e,
                "write-back: failed to add inotify watch; relying on periodic reconciliation for this subtree"
            ),
        }
    }

    fn remove_watches_under(&self, id: Uuid, rel_dir: &str) {
        // `rel_dir == ""` means the workspace root itself vanished/moved
        // (DELETE_SELF/MOVE_SELF on the root watch) -- in that case every
        // watch this engine holds for the workspace is stale, not just an
        // exact-match on the empty string, since "" is logically a prefix
        // of every path under it.
        let prefix = format!("{rel_dir}/");
        let stale: Vec<WatchDescriptor> = {
            let watches = self.watches.lock();
            watches
                .iter()
                .filter(|(_, e)| {
                    e.workspace_id == id
                        && (rel_dir.is_empty()
                            || e.rel_dir == rel_dir
                            || e.rel_dir.starts_with(&prefix))
                })
                .map(|(wd, _)| wd.clone())
                .collect()
        };
        if stale.is_empty() {
            return;
        }
        {
            let mut w = self.watch_handle.lock();
            for wd in &stale {
                let _ = w.remove(wd.clone());
            }
        }
        let mut watches = self.watches.lock();
        for wd in &stale {
            watches.remove(wd);
        }
    }

    /// Owns `inotify` exclusively for the lifetime of this thread -- no
    /// other thread ever touches it (see the `watch_handle` field's doc
    /// comment for why watch add/remove is routed through a separate
    /// handle instead), so the blocking `read_events_blocking` call below
    /// never contends with, or blocks, anything else in this process.
    fn event_loop(self: Arc<Self>, mut inotify: Inotify) {
        let mut buffer = vec![0u8; 8192];
        loop {
            let events: Vec<EventOwned> = match inotify.read_events_blocking(&mut buffer) {
                Ok(evs) => evs.map(|e| e.to_owned()).collect(),
                Err(e) => {
                    tracing::error!(
                        error = %e,
                        "write-back: inotify read_events_blocking failed; stopping the \
                         event loop for this process -- periodic reconciliation is now \
                         the only write-back mechanism until restart"
                    );
                    return;
                }
            };
            for event in events {
                self.handle_event(event);
            }
        }
    }

    fn handle_event(&self, event: EventOwned) {
        if event.mask.contains(EventMask::Q_OVERFLOW) {
            tracing::warn!(
                "write-back: inotify IN_Q_OVERFLOW -- the kernel dropped events under a \
                 write burst; forcing an immediate, out-of-band reconciliation pass across \
                 all workspaces rather than waiting for the next scheduled tick"
            );
            self.reconcile_all();
            return;
        }
        if event.mask.contains(EventMask::IGNORED) {
            // Watch removed (explicitly via unregister_workspace, or
            // automatically because the watched directory was deleted or
            // its filesystem unmounted) -- just clean up bookkeeping.
            self.watches.lock().remove(&event.wd);
            return;
        }

        let entry = match self.watches.lock().get(&event.wd).cloned() {
            Some(e) => e,
            None => return, // unknown/stale watch descriptor; nothing to do
        };
        if !self.store.workspace_exists(entry.workspace_id) {
            // Raced against drop_workspace; harmless no-op.
            return;
        }

        if event.mask.intersects(EventMask::DELETE_SELF | EventMask::MOVE_SELF) {
            // The watched directory itself vanished or moved away from
            // under us. Prune anything still recorded under it. The
            // parent directory's own DELETE/MOVED_FROM event (if the
            // parent is also watched) independently does the same thing --
            // handling both is deliberately redundant-safe, not a bug (see
            // wb_apply_delete_prefix, which is idempotent).
            self.store
                .wb_apply_delete_prefix(entry.workspace_id, &entry.rel_dir);
            self.remove_watches_under(entry.workspace_id, &entry.rel_dir);
            return;
        }

        // Every other event type this engine watches for carries a `name`
        // (see `WatchMask::CREATE`/`DELETE`/`CLOSE_WRITE`/`MOVED_FROM`/
        // `MOVED_TO`'s own doc comments upstream: each "is only triggered
        // for objects inside the directory, not the directory itself").
        let name = match event.name.as_deref() {
            Some(n) => n,
            None => return,
        };
        let name = match name.to_str() {
            Some(s) => s,
            None => {
                tracing::warn!(
                    workspace = %entry.workspace_id,
                    dir = %entry.rel_dir,
                    "write-back: skipping event for a non-UTF-8 filename -- this path can \
                     never be represented in this store's String-keyed files map or sent \
                     back over the JSON IPC protocol, a pre-existing, system-wide constraint \
                     (see docs/DEBT-REGISTER.md)"
                );
                return;
            }
        };
        let rel_path = if entry.rel_dir.is_empty() {
            name.to_string()
        } else {
            format!("{}/{}", entry.rel_dir, name)
        };
        let is_dir = event.mask.contains(EventMask::ISDIR);

        if event.mask.intersects(EventMask::CREATE | EventMask::MOVED_TO) {
            if is_dir {
                self.sync_new_subtree(entry.workspace_id, &rel_path);
            } else {
                self.sync_file(entry.workspace_id, &rel_path);
            }
        } else if event.mask.contains(EventMask::CLOSE_WRITE) {
            // CLOSE_WRITE is only ever delivered for non-directory objects.
            self.sync_file(entry.workspace_id, &rel_path);
        } else if event.mask.intersects(EventMask::DELETE | EventMask::MOVED_FROM) {
            if is_dir {
                self.store.wb_apply_delete_prefix(entry.workspace_id, &rel_path);
                self.remove_watches_under(entry.workspace_id, &rel_path);
            } else {
                self.store.wb_apply_delete(entry.workspace_id, &rel_path);
            }
        }
    }

    /// Re-reads a single real file at `rel_path` and records its content in
    /// the blob store. Applies the same symlink policy documented on
    /// `scan_workspace_tree`: `symlink_metadata` is checked first, and a
    /// symlink is skipped (logged), never followed.
    fn sync_file(&self, id: Uuid, rel_path: &str) {
        let real_path = self.store.workspace_dir(id).join(rel_path);
        match std::fs::symlink_metadata(&real_path) {
            Ok(meta) if meta.file_type().is_symlink() => {
                tracing::warn!(
                    workspace = %id,
                    path = %rel_path,
                    "write-back: skipping symlink planted directly at the projected path \
                     (never followed) -- see scan_workspace_tree's doc comment for the \
                     reasoning shared with the reconciliation pass"
                );
            }
            Ok(meta) if meta.file_type().is_file() => match std::fs::read(&real_path) {
                Ok(content) => {
                    if let Err(e) = self.store.wb_apply_write(id, rel_path, &content) {
                        tracing::warn!(workspace = %id, path = %rel_path, error = %e, "write-back: failed to record filesystem edit");
                    }
                }
                Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                    // Raced against a fast delete/rename-away between the
                    // event and this read; the corresponding
                    // DELETE/MOVED_FROM event (already delivered, or still
                    // pending in this same batch) resolves this, with
                    // reconciliation as the ultimate backstop.
                }
                Err(e) => tracing::warn!(workspace = %id, path = %rel_path, error = %e, "write-back: failed to read edited file"),
            },
            Ok(_) => tracing::warn!(
                workspace = %id,
                path = %rel_path,
                "write-back: skipping non-regular filesystem entry (device/FIFO/socket) at projected path"
            ),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                // Gone again already; nothing to do.
            }
            Err(e) => tracing::warn!(workspace = %id, path = %rel_path, error = %e, "write-back: failed to stat edited path"),
        }
    }

    /// A new directory appeared at `rel_dir` (via `CREATE` or `MOVED_TO`
    /// with `IN_ISDIR`). Recursively discovers its contents right away
    /// (rather than waiting for further per-entry events, which may have
    /// already been missed for anything the directory brought in with it --
    /// see this module's doc comment on `inotify` not being recursive) and
    /// adds a watch to every subdirectory found.
    fn sync_new_subtree(&self, id: Uuid, rel_dir: &str) {
        let ws_dir = self.store.workspace_dir(id);
        match scan_workspace_tree(&ws_dir, rel_dir) {
            Ok(scan) => {
                for dir in &scan.dirs {
                    self.add_single_watch(id, &ws_dir, dir);
                }
                for (path, content) in &scan.files {
                    if let Err(e) = self.store.wb_apply_write(id, path, content) {
                        tracing::warn!(workspace = %id, path = %path, error = %e, "write-back: failed to record file in new subtree");
                    }
                }
            }
            Err(e) => tracing::warn!(workspace = %id, dir = %rel_dir, error = %e, "write-back: failed to scan new subtree"),
        }
    }

    /// Runs one reconciliation pass for every currently known workspace.
    /// See this module's doc comment for why this is a required
    /// correctness backstop, not an optional optimization.
    fn reconcile_all(&self) {
        for id in self.store.workspace_ids_snapshot() {
            self.reconcile_workspace(id);
        }
    }

    fn reconcile_loop(self: Arc<Self>) {
        loop {
            std::thread::sleep(self.reconcile_interval);
            self.reconcile_all();
        }
    }

    /// Hash-diffs the real, projected directory tree for workspace `id`
    /// against its current in-memory `files` map, and repairs any drift:
    /// new/changed files are (re-)recorded, and files present in the map
    /// but missing on disk are removed. This is what recovers from *every*
    /// mutation type -- including ones `inotify` never reported at all
    /// (overflowed, or raced against a not-yet-registered watch) -- because
    /// it does not depend on any event having been observed in the first
    /// place; it only compares "what's on disk right now" against "what we
    /// last recorded."
    fn reconcile_workspace(&self, id: Uuid) {
        let ws_dir = self.store.workspace_dir(id);
        let scan = match scan_workspace_tree(&ws_dir, "") {
            Ok(s) => s,
            Err(e) => {
                tracing::warn!(workspace = %id, error = %e, "write-back: reconciliation scan failed");
                return;
            }
        };
        let known = match self.store.workspace_files_snapshot(id) {
            Some(k) => k,
            None => return, // torn down concurrently
        };

        let existing_dirs: HashSet<String> = {
            self.watches
                .lock()
                .values()
                .filter(|e| e.workspace_id == id)
                .map(|e| e.rel_dir.clone())
                .collect()
        };
        for dir in &scan.dirs {
            if !existing_dirs.contains(dir) {
                self.add_single_watch(id, &ws_dir, dir);
            }
        }

        for (path, content) in &scan.files {
            let candidate = match git2::Oid::hash_object(git2::ObjectType::Blob, content) {
                Ok(o) => o,
                Err(e) => {
                    tracing::warn!(workspace = %id, path = %path, error = %e, "write-back: failed to hash file during reconciliation");
                    continue;
                }
            };
            let changed = known.get(path).copied().map(|o| o != candidate).unwrap_or(true);
            if changed {
                if let Err(e) = self.store.wb_apply_write(id, path, content) {
                    tracing::warn!(workspace = %id, path = %path, error = %e, "write-back: reconciliation failed to record change");
                }
            }
        }

        for path in known.keys() {
            if !scan.files.contains_key(path) {
                self.store.wb_apply_delete(id, path);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::repo::ShadowStore;

    fn store_with_engine() -> (Arc<ShadowStore>, Arc<WriteBackEngine>) {
        let dir = tempfile::tempdir().unwrap();
        let store = Arc::new(ShadowStore::new(dir.keep()).unwrap());
        let engine = WriteBackEngine::spawn(Arc::clone(&store)).expect("spawn write-back engine");
        store.attach_writeback(Arc::clone(&engine));
        (store, engine)
    }

    /// Exercises the `IN_Q_OVERFLOW` recovery path (ADR-012's explicit
    /// requirement: "a periodic hash-based reconciliation pass as a
    /// fallback for missed or overflowed inotify events") deterministically.
    ///
    /// A literal, kernel-forced overflow (writing enough files fast enough
    /// to exceed `/proc/sys/fs/inotify/max_queued_events`) was deliberately
    /// **not** used here: it depends on a sysctl this test does not control
    /// inside a container, and on scheduling behavior that would make the
    /// test flaky across CI environments -- exactly the kind of test this
    /// project's own conventions (see `systematic-debugging`/verification
    /// skills) discourage. Instead, this test writes a file directly to the
    /// real path and deliberately never routes any event for it through
    /// `handle_event` -- standing in for "the kernel dropped this event" --
    /// then hand-delivers a single synthetic `Q_OVERFLOW` event (built from
    /// a `WatchDescriptor` obtained from a *real* registered watch, so it
    /// is not a fabricated/invalid descriptor) and asserts the resulting
    /// forced reconciliation pass recovers the write anyway. This is a
    /// strictly harder case than a real overflow (a real overflow still
    /// lets *some* events through before dropping the rest): if
    /// reconciliation recovers a write for which literally zero events were
    /// ever observed, it recovers from any overflow pattern.
    #[test]
    fn q_overflow_event_forces_reconciliation_that_recovers_a_completely_unobserved_write() {
        let (store, engine) = store_with_engine();
        let (id, _) = store.create(None).unwrap();

        let real = store.workspace_dir(id).join("missed_by_overflow.txt");
        std::fs::write(&real, b"dropped by a simulated overflow").unwrap();
        // No CREATE/CLOSE_WRITE event for the write above is ever passed to
        // `handle_event` -- deliberately, standing in for a kernel-dropped
        // burst under IN_Q_OVERFLOW.

        let wd = {
            let watches = engine.watches.lock();
            watches
                .iter()
                .find(|(_, e)| e.workspace_id == id && e.rel_dir.is_empty())
                .map(|(wd, _)| wd.clone())
                .expect("create() must have registered a root watch for this workspace")
        };

        engine.handle_event(EventOwned {
            wd,
            mask: EventMask::Q_OVERFLOW,
            cookie: 0,
            name: None,
        });

        let files = store
            .workspace_files_snapshot(id)
            .expect("workspace must still exist");
        assert!(
            files.contains_key("missed_by_overflow.txt"),
            "the reconciliation pass forced by IN_Q_OVERFLOW must recover a write for which no \
             inotify event was ever observed at all"
        );
    }
}
