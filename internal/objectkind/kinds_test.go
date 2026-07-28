// SPDX-License-Identifier: GPL-3.0-or-later

package objectkind

import (
	"strings"
	"testing"
)

// TestKindInvariants checks the shape every registry entry must have, so a
// malformed new entry fails here rather than producing a broken SHOW command or
// an empty tree label at runtime.
func TestKindInvariants(t *testing.T) {
	seen := make(map[string]bool, len(Kinds))
	for i, k := range Kinds {
		if k.Name == "" {
			t.Fatalf("Kinds[%d] has an empty Name", i)
		}
		if seen[k.Name] {
			t.Errorf("duplicate kind %q", k.Name)
		}
		seen[k.Name] = true

		if k.Name != strings.ToUpper(strings.TrimSpace(k.Name)) {
			t.Errorf("kind %q: Name must be upper case and trimmed", k.Name)
		}
		if strings.Contains(k.Name, "_") {
			t.Errorf("kind %q: Name must use the space-separated SHOW form, not the underscore form", k.Name)
		}
		if k.Plural == "" || k.Plural != strings.ToUpper(k.Plural) {
			t.Errorf("kind %q: Plural must be non-empty and upper case, got %q", k.Name, k.Plural)
		}
		// "TABLE" → "TABLES", "MASKING POLICY" → "MASKING POLICIES": the plural
		// always extends the kind name (minus a trailing "y").
		if !strings.HasPrefix(k.Plural, strings.TrimSuffix(k.Name, "Y")) {
			t.Errorf("kind %q: Plural %q does not look like the plural of the kind", k.Name, k.Plural)
		}
		if k.Label == "" {
			t.Errorf("kind %q: Label must not be empty", k.Name)
		}
		if k.Label != strings.TrimSpace(k.Label) {
			t.Errorf("kind %q: Label %q has surrounding whitespace", k.Name, k.Label)
		}
		if k.GetDDLType != strings.ToUpper(k.GetDDLType) {
			t.Errorf("kind %q: GetDDLType %q must be upper case", k.Name, k.GetDDLType)
		}
		if k.Routine && k.GetDDLType == "" {
			t.Errorf("kind %q: a routine needs a GET_DDL type to append its signature to", k.Name)
		}
	}
	if len(Kinds) < 40 {
		t.Errorf("registry has only %d kinds — did entries get dropped?", len(Kinds))
	}
}

// TestBasicKinds pins the three kinds SHOW OBJECTS returns. Marking a kind basic
// by mistake silently removes its dedicated SHOW command (so it vanishes from
// the tree unless SHOW OBJECTS happens to return it), which is why this list is
// asserted exactly rather than by shape.
func TestBasicKinds(t *testing.T) {
	want := map[string]bool{"TABLE": true, "VIEW": true, "SEQUENCE": true}
	for _, k := range Kinds {
		if k.Basic != want[k.Name] {
			t.Errorf("kind %q: Basic = %v, want %v", k.Name, k.Basic, want[k.Name])
		}
	}
	if got := len(Kinds) - len(Extended()); got != len(want) {
		t.Errorf("got %d basic kinds, want %d", got, len(want))
	}
}

func TestByName(t *testing.T) {
	k, ok := ByName("materialized view")
	if !ok {
		t.Fatal("ByName should match case-insensitively")
	}
	if k.Plural != "MATERIALIZED VIEWS" {
		t.Errorf("Plural = %q", k.Plural)
	}
	if _, ok := ByName("  Dynamic Table  "); !ok {
		t.Error("ByName should tolerate surrounding whitespace")
	}
	// Account-scoped kinds are deliberately outside the registry.
	if _, ok := ByName("WAREHOUSE"); ok {
		t.Error("WAREHOUSE should not be in the schema-scoped registry")
	}
	if _, ok := ByName("NOT A KIND"); ok {
		t.Error("unknown kind should not resolve")
	}
}

func TestIsExtended(t *testing.T) {
	if IsExtended("TABLE") || IsExtended("VIEW") || IsExtended("SEQUENCE") {
		t.Error("basic kinds must not be extended")
	}
	if !IsExtended("STREAMLIT") {
		t.Error("STREAMLIT is sourced from its own SHOW command")
	}
	// Unknown kinds fall back to SHOW OBJECTS rather than getting a bogus command.
	if IsExtended("WAREHOUSE") {
		t.Error("unknown kinds must not be reported as extended")
	}
}

// TestExtendedReturnsCopy guards the callers that filter the extended list in
// place (the excluded-kinds filter in ListExtendedObjects) from corrupting the
// registry for everyone else.
func TestExtendedReturnsCopy(t *testing.T) {
	a := Extended()
	original := a[0]
	a[0] = Kind{Name: "MUTATED"}
	if Extended()[0].Name != original.Name {
		t.Error("Extended() handed out the shared backing array")
	}
}

func TestDDLUnsupported(t *testing.T) {
	unsupported := DDLUnsupported()
	for _, k := range Kinds {
		if (k.GetDDLType == "") != unsupported[k.Name] {
			t.Errorf("kind %q: GetDDLType %q vs DDLUnsupported %v", k.Name, k.GetDDLType, unsupported[k.Name])
		}
	}
	// A representative sample, so an accidental "fix" that hands one of these a
	// GET_DDL type has to be deliberate: each fails on the server today.
	for _, name := range []string{"PACKAGES POLICY", "SERVICE", "MODEL", "MCP SERVER"} {
		if !unsupported[name] {
			t.Errorf("%s should be GET_DDL-unsupported", name)
		}
	}
}
