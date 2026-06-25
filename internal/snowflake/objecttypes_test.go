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

// TestSchemaScopedCreateKeywords verifies the helper returns only schema-scoped
// phrases, longest first (so prefix matching picks the most specific phrase).
func TestSchemaScopedCreateKeywords(t *testing.T) {
	kws := SchemaScopedCreateKeywords()
	if len(kws) == 0 {
		t.Fatal("expected schema-scoped keywords")
	}
	schemaSet := map[string]bool{}
	for _, ot := range ObjectTypes {
		if ot.Scope == ScopeSchema {
			schemaSet[phraseKey(ot.Keywords)] = true
		}
	}
	for i, words := range kws {
		if !schemaSet[phraseKey(words)] {
			t.Errorf("phrase %q is not a schema-scoped object", phraseKey(words))
		}
		if i > 0 && len(kws[i-1]) < len(words) {
			t.Errorf("not sorted longest-first at %d: %v before %v", i, kws[i-1], words)
		}
	}
	if len(kws) != len(schemaSet) {
		t.Errorf("got %d schema phrases, want %d", len(kws), len(schemaSet))
	}
}
