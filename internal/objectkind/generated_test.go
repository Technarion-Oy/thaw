// SPDX-License-Identifier: GPL-3.0-or-later

package objectkind

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// reGenKind extracts one generated TS entry: name, label, basic, ddl.
var reGenKind = regexp.MustCompile(`\{ name: "([^"]*)", label: "([^"]*)", basic: (true|false), ddl: (true|false) \}`)

// TestGeneratedObjectKindsInSync verifies that the committed frontend artifact
// (frontend/src/generated/objectKinds.ts) carries exactly the kinds this registry
// defines, in the same order, with the same labels and flags. The sidebar's tree
// grouping, search type filter and DDL guard all read that file, so a stale copy
// means a kind is listed in the UI with the wrong label, in the wrong place, or
// offering a DDL action that cannot work.
//
// If this fails the artifact is stale — regenerate it with
// `go generate ./internal/objectkind/`.
func TestGeneratedObjectKindsInSync(t *testing.T) {
	path := filepath.Join("..", "..", "frontend", "src", "generated", "objectKinds.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading generated artifact: %v (run: go generate ./internal/objectkind/)", err)
	}

	matches := reGenKind.FindAllStringSubmatch(string(data), -1)
	if len(matches) != len(Kinds) {
		t.Fatalf("generated artifact has %d kinds, registry has %d — run: go generate ./internal/objectkind/",
			len(matches), len(Kinds))
	}
	for i, k := range Kinds {
		got := matches[i]
		want := []string{k.Name, k.Label, fmt.Sprintf("%t", k.Basic), fmt.Sprintf("%t", k.GetDDLType != "")}
		for j, field := range []string{"name", "label", "basic", "ddl"} {
			if got[j+1] != want[j] {
				t.Errorf("kind[%d] %s = %q in artifact, %q in registry — run: go generate ./internal/objectkind/",
					i, field, got[j+1], want[j])
			}
		}
	}
}
