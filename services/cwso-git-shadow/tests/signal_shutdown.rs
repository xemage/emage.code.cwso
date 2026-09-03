//! C023: live-process confirmation that `SIGTERM`/`SIGINT` shut the real
//! `cwso-git-shadow` binary down promptly, and do not hang or panic the
//! `WriteBackEngine`'s two background threads (`event_loop`, `reconcile_loop`
//! -- see `src/writeback.rs`).
//!
//! # Conclusion (investigated for C023, documented here rather than in a
//! code comment, since the evidence for it is this very test)
//!
//! `main.rs` installs **no** custom `SIGTERM`/`SIGINT` handler at all (no
//! `signal-hook`/`ctrlc`-style crate is even a dependency of this service).
//! That is a deliberate conclusion, not an oversight: both signals' default
//! disposition (`SIG_DFL`) is `Term` -- the kernel terminates every thread in
//! the process's thread group immediately and unconditionally the instant
//! the signal is delivered, without waiting for any blocking syscall (e.g.
//! `event_loop`'s `inotify.read_events_blocking`, `reconcile_loop`'s
//! `thread::sleep`) to return, and without running any of this crate's code
//! at all. Because no code runs in response, there is no code path left that
//! could hang or panic: this is not "shutdown code that happens to be
//! correct," it is the *absence* of shutdown code, which is exactly what
//! `writeback.rs`'s own "Durability" doc comment (`src/writeback.rs`) already
//! establishes is sufficient here -- the engine keeps no additional
//! in-process queue/buffer that would need an orderly flush before exit, and
//! `parking_lot::Mutex` (used throughout this crate) never poisons on a
//! panicking thread, so even a hypothetical in-flight panic in one thread at
//! the instant of the signal could not propagate into a poisoned-lock panic
//! on another. Installing an explicit handler here would add a new shutdown
//! code path (with its own possibility of hanging or panicking) to solve a
//! problem that does not exist; it was deliberately not added.
//!
//! This test exists to make that conclusion falsifiable rather than merely
//! asserted: it spawns the real, compiled binary, confirms it has finished
//! starting (both background threads spawned, IPC socket bound), sends the
//! real signal via `kill(2)`, and asserts the process exits promptly and via
//! that same signal -- not via a timeout (hang) and not via a panic-shaped
//! exit path.

use std::os::unix::process::ExitStatusExt;
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

/// Generous but bounded: long enough that a slow, loaded CI box won't flake,
/// short enough that a genuine hang fails the test instead of the test
/// itself hanging forever.
const STARTUP_TIMEOUT: Duration = Duration::from_secs(10);
const SHUTDOWN_TIMEOUT: Duration = Duration::from_secs(5);

fn wait_for_socket(path: &std::path::Path, timeout: Duration) -> bool {
    let start = Instant::now();
    while start.elapsed() < timeout {
        if path.exists() {
            return true;
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    false
}

fn assert_signal_shuts_down_promptly(signal: libc::c_int) {
    let storage = tempfile::tempdir().expect("create temp storage root");
    let sock_dir = tempfile::tempdir().expect("create temp socket dir");
    let socket_path = sock_dir.path().join("git-shadow.sock");

    let mut child = Command::new(env!("CARGO_BIN_EXE_cwso-git-shadow"))
        .env("CWSO_GIT_SHADOW_STORAGE", storage.path())
        .env("CWSO_GIT_SHADOW_SOCKET", &socket_path)
        // Quiet the JSON logger for this test; irrelevant to what's being
        // asserted and keeps test output readable.
        .env("RUST_LOG", "error")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("spawn the real cwso-git-shadow binary");

    assert!(
        wait_for_socket(&socket_path, STARTUP_TIMEOUT),
        "service must finish starting up (including spawning the write-back engine's two \
         background threads, and binding its IPC socket) before this test signals it -- a \
         missing socket after {STARTUP_TIMEOUT:?} means startup itself is broken, not the \
         thing this test is trying to exercise"
    );

    let pid = child.id() as libc::pid_t;
    let rc = unsafe { libc::kill(pid, signal) };
    assert_eq!(rc, 0, "kill({pid}, {signal}) must succeed");

    let deadline = Instant::now() + SHUTDOWN_TIMEOUT;
    loop {
        match child.try_wait().expect("try_wait on signaled child") {
            Some(status) => {
                assert_eq!(
                    status.signal(),
                    Some(signal),
                    "process must terminate via the exact signal it was sent (the kernel's \
                     default disposition), not via some other, unexpected exit path -- a \
                     different signal or a plain exit code here would mean something in this \
                     service is intercepting or reacting to the signal in an unexpected way"
                );
                assert!(
                    !status.core_dumped(),
                    "signaled shutdown must not core-dump (would indicate a crash-worthy \
                     signal/state, not a clean default-disposition termination)"
                );
                return;
            }
            None => {
                assert!(
                    Instant::now() < deadline,
                    "process did not exit within {SHUTDOWN_TIMEOUT:?} of receiving signal \
                     {signal} -- shutdown hung"
                );
                std::thread::sleep(Duration::from_millis(20));
            }
        }
    }
}

#[test]
fn sigterm_shuts_down_promptly_without_hanging_or_panicking() {
    assert_signal_shuts_down_promptly(libc::SIGTERM);
}

#[test]
fn sigint_shuts_down_promptly_without_hanging_or_panicking() {
    assert_signal_shuts_down_promptly(libc::SIGINT);
}
