// Package greet is a minimal, dependency-free Go fixture for C024's
// filesystem-projection E2E proof (scripts/cwso-projection-e2e.sh, CG2).
//
// This exact file is:
//  1. written into a real shadow workspace via the write_shadow_file MCP
//     tool, then inspected with `ls`/`cat` over `docker exec` at the real,
//     projected path (`<storage_root>/<workspace-uuid>/greet.go`);
//  2. exercised by a real `go test` binary (see greet_test.go and the
//     script's "BUILD TEST BINARY" stage for why that binary is compiled
//     ahead of time rather than via a toolchain inside the git-shadow
//     container);
//  3. mutated in place with `sed` (bypassing every CWSO tool entirely) to
//     prove the write-back path (services/cwso-git-shadow/src/writeback.rs,
//     C022) feeds `commit_shadow`.
package greet

import "fmt"

// Greet returns a friendly greeting for name.
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
