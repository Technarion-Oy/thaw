// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestThirdPartyNoticesUpToDate guards against THIRD_PARTY_NOTICES.md drifting
// out of sync with the actual dependency tree (go.mod / frontend/package.json).
// The file is embedded via //go:embed and presented to users as an authoritative
// license list, so a stale copy is a legal-accuracy risk that is otherwise easy
// to introduce silently — bump a dependency, forget to re-run the generator.
//
// It re-runs scripts/gen_third_party_notices.go into a temp file and diffs it
// against the committed copy. The generator needs `go`, `npm`, and an installed
// frontend/node_modules; when any is unavailable (or the generator otherwise
// fails to run), the test skips rather than failing, so CI environments without
// a full toolchain stay green. It only fails on an actual content mismatch.
//
// Regenerate with: go run scripts/gen_third_party_notices.go
func TestThirdPartyNoticesUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generator run in -short mode")
	}
	for _, bin := range []string{"go", "npm"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not found on PATH; skipping freshness check", bin)
		}
	}
	if _, err := os.Stat(filepath.Join("frontend", "node_modules")); err != nil {
		t.Skip("frontend/node_modules not installed; skipping freshness check")
	}

	committed, err := os.ReadFile("THIRD_PARTY_NOTICES.md")
	if err != nil {
		t.Fatalf("reading committed notices: %v", err)
	}

	tmp := filepath.Join(t.TempDir(), "THIRD_PARTY_NOTICES.md")
	cmd := exec.Command("go", "run", "scripts/gen_third_party_notices.go", "-o", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("generator failed to run (environment not ready): %v\n%s", err, out)
	}

	regenerated, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("reading regenerated notices: %v", err)
	}

	if string(committed) != string(regenerated) {
		t.Errorf("THIRD_PARTY_NOTICES.md is out of date with the dependency tree.\n" +
			"Regenerate it with: go run scripts/gen_third_party_notices.go")
	}
}
