// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// reGenName extracts the `name: "..."` field from each generated TS entry.
var reGenName = regexp.MustCompile(`name:\s*"([^"]*)"`)

// TestGeneratedDataTypesInSync verifies that the generated frontend artifact
// (frontend/src/generated/snowflakeDataTypes.ts) lists exactly the same type
// names, in the same order, as the authoritative registry.  If this fails the
// artifact is stale — regenerate it with `go generate ./internal/snowflake/`.
func TestGeneratedDataTypesInSync(t *testing.T) {
	path := filepath.Join("..", "..", "frontend", "src", "generated", "snowflakeDataTypes.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading generated artifact: %v (run: go generate ./internal/snowflake/)", err)
	}

	matches := reGenName.FindAllStringSubmatch(string(data), -1)
	got := make([]string, 0, len(matches))
	for _, m := range matches {
		got = append(got, m[1])
	}

	want := AllDataTypes()
	if len(got) != len(want) {
		t.Fatalf("generated artifact has %d types, registry has %d — run: go generate ./internal/snowflake/", len(got), len(want))
	}
	for i, dt := range want {
		if got[i] != dt.Name {
			t.Errorf("type[%d] = %q in artifact, %q in registry — run: go generate ./internal/snowflake/", i, got[i], dt.Name)
		}
	}
}
