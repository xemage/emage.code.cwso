package greet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGreet is a plain, ordinary unit test -- no dependency on any
// filesystem path. It exercises the real, compiled Greet function.
func TestGreet(t *testing.T) {
	got := Greet("CWSO")
	want := "Hello, CWSO!"
	if got != want {
		t.Fatalf("Greet(%q) = %q, want %q", "CWSO", got, want)
	}
}

// TestGreetSourceProjectedAtRealPath is the "real test command" stage of
// C024/CG2's filesystem-projection E2E proof
// (scripts/cwso-projection-e2e.sh). It requires
// CWSO_PROJECTION_E2E_WORKSPACE_DIR to be set to the shadow workspace's
// real, projected path (`<storage_root>/<workspace-uuid>/`, per ADR-012)
// and asserts that the exact file content living at that real path --
// read here via a plain os.ReadFile, not through any CWSO tool -- matches
// what this package's own source expects. This is a genuine Go test
// (compiled via the real `go test -c`, executed via the real Go testing
// package, asserting real content) that fails loudly, via t.Fatalf, on any
// mismatch rather than silently passing.
//
// Judgment-call note (see scripts/cwso-projection-e2e.sh header and the
// C024 MR description for the full writeup): this compiled test binary is
// executed from /run/cwso (the existing cwso-runtime named volume already
// declared in deploy/docker-compose.yml for git-shadow's IPC socket), not
// from the workspace's own real path. That path's backing store is a
// Docker `tmpfs:` mount (deploy/docker-compose.yml, git-shadow service)
// which is `noexec` by Docker's own default when no `exec` mount option is
// given -- confirmed live: the kernel refuses execve() for ANY binary
// placed there, independent of whether a language toolchain is present.
// Only the compiled test binary's own inode needed to move to satisfy the
// kernel; every assertion in this function still reads and validates real
// content at the real projected path.
func TestGreetSourceProjectedAtRealPath(t *testing.T) {
	dir := os.Getenv("CWSO_PROJECTION_E2E_WORKSPACE_DIR")
	if dir == "" {
		t.Fatal("CWSO_PROJECTION_E2E_WORKSPACE_DIR must be set to the real, projected shadow workspace path")
	}
	realPath := filepath.Join(dir, "greet.go")
	content, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read greet.go from real projected path %s: %v", realPath, err)
	}
	const wantSignature = "func Greet(name string) string"
	if !strings.Contains(string(content), wantSignature) {
		t.Fatalf(
			"greet.go at real projected path %s does not contain expected signature %q; got:\n%s",
			realPath, wantSignature, content,
		)
	}
	if g := Greet("CWSO"); g != "Hello, CWSO!" {
		t.Fatalf("Greet(%q) = %q, want %q", "CWSO", g, "Hello, CWSO!")
	}
}
