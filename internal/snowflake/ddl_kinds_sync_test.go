package snowflake

import (
	"os"
	"regexp"
	"testing"
)

// TestDDLUnsupportedKindsInSyncWithFrontend guards the hand-maintained mirror in
// frontend/src/utils/objectDdl.ts against drift from the Go source of truth
// (DDLUnsupportedKinds). A full generator is overkill for a 10-item list; this
// cheap read-and-compare closes the drift risk the "keep in sync" comment can't.
func TestDDLUnsupportedKindsInSyncWithFrontend(t *testing.T) {
	const tsPath = "../../frontend/src/utils/objectDdl.ts"
	src, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("read %s: %v", tsPath, err)
	}

	// Pull the Set body: DDL_UNSUPPORTED_KINDS = new Set<string>([ ... ]);
	body := regexp.MustCompile(`(?s)DDL_UNSUPPORTED_KINDS\s*=\s*new Set<string>\(\[(.*?)\]\)`).FindSubmatch(src)
	if body == nil {
		t.Fatal("could not locate DDL_UNSUPPORTED_KINDS Set literal in objectDdl.ts")
	}
	fe := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(string(body[1]), -1) {
		fe[m[1]] = true
	}

	for k := range DDLUnsupportedKinds {
		if !fe[k] {
			t.Errorf("kind %q is in Go DDLUnsupportedKinds but missing from objectDdl.ts", k)
		}
	}
	for k := range fe {
		if !DDLUnsupportedKinds[k] {
			t.Errorf("kind %q is in objectDdl.ts but missing from Go DDLUnsupportedKinds", k)
		}
	}
}
