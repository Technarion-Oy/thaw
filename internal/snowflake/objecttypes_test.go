// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"strings"
	"testing"
)

func phraseKey(words []string) string { return strings.Join(words, " ") }

// TestObjectTypesNoDuplicates ensures each object keyword phrase appears once.
func TestObjectTypesNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, ot := range ObjectTypes {
		k := phraseKey(ot.Keywords)
		if seen[k] {
			t.Errorf("duplicate object type %q", k)
		}
		seen[k] = true
		if len(ot.Keywords) == 0 {
			t.Errorf("object type with empty keyword phrase: %+v", ot)
		}
	}
}

// TestObjectScopeClassification locks in the scope of representative objects —
// especially the pairs that are easy to confuse (NETWORK POLICY vs NETWORK RULE,
// DATABASE vs DATABASE ROLE, VIEW vs the table-likes).
func TestObjectScopeClassification(t *testing.T) {
	want := map[string]ObjectScope{
		"TABLE":             ScopeSchema,
		"VIEW":              ScopeSchema,
		"SEQUENCE":          ScopeSchema,
		"STAGE":             ScopeSchema,
		"STREAM":            ScopeSchema,
		"TASK":              ScopeSchema,
		"FILE FORMAT":       ScopeSchema,
		"MASKING POLICY":    ScopeSchema,
		"ROW ACCESS POLICY": ScopeSchema,
		"NETWORK RULE":      ScopeSchema,
		"DYNAMIC TABLE":     ScopeSchema,
		"DATABASE":          ScopeAccount,
		"WAREHOUSE":         ScopeAccount,
		"ROLE":              ScopeAccount,
		"NETWORK POLICY":    ScopeAccount,
		"RESOURCE MONITOR":  ScopeAccount,
		"API INTEGRATION":   ScopeAccount,
		"SCHEMA":            ScopeDatabase,
		"DATABASE ROLE":     ScopeDatabase,
		"APPLICATION ROLE":  ScopeApplication,
		"ORGANIZATION USER": ScopeOrganization,
	}
	got := map[string]ObjectScope{}
	for _, ot := range ObjectTypes {
		got[phraseKey(ot.Keywords)] = ot.Scope
	}
	for phrase, scope := range want {
		g, ok := got[phrase]
		if !ok {
			t.Errorf("object type %q missing from ObjectTypes", phrase)
			continue
		}
		if g != scope {
			t.Errorf("scope of %q = %v; want %v", phrase, g, scope)
		}
	}
}

// TestSchemaScopedObjectTypes verifies the helper returns only schema-scoped
// types, longest keyword phrase first (so prefix matching picks the most specific
// phrase), and that Name() renders the lowercased phrase.
func TestSchemaScopedObjectTypes(t *testing.T) {
	types := SchemaScopedObjectTypes()
	if len(types) == 0 {
		t.Fatal("expected schema-scoped object types")
	}
	schemaSet := map[string]bool{}
	for _, ot := range ObjectTypes {
		if ot.Scope == ScopeSchema {
			schemaSet[phraseKey(ot.Keywords)] = true
		}
	}
	for i, ot := range types {
		if ot.Scope != ScopeSchema {
			t.Errorf("%q is not schema-scoped", ot.Name())
		}
		if !schemaSet[phraseKey(ot.Keywords)] {
			t.Errorf("phrase %q is not a schema-scoped object", phraseKey(ot.Keywords))
		}
		if i > 0 && len(types[i-1].Keywords) < len(ot.Keywords) {
			t.Errorf("not sorted longest-first at %d: %v before %v", i, types[i-1].Keywords, ot.Keywords)
		}
	}
	if got := SchemaScopedObjectTypes()[0]; got.Name() != strings.ToLower(strings.Join(got.Keywords, " ")) {
		t.Errorf("Name() = %q, want lowercased %v", got.Name(), got.Keywords)
	}
	if len(types) != len(schemaSet) {
		t.Errorf("got %d schema types, want %d", len(types), len(schemaSet))
	}
}
