// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "testing"

func TestTagReferenceDomain(t *testing.T) {
	tests := []struct {
		kind         string
		wantDomain   string
		wantCallable bool
	}{
		// Table family folds onto TABLE.
		{"TABLE", "TABLE", false},
		{"DYNAMIC TABLE", "TABLE", false},
		{"EXTERNAL TABLE", "TABLE", false},
		{"ICEBERG TABLE", "TABLE", false},
		{"HYBRID TABLE", "TABLE", false},
		{"EVENT TABLE", "TABLE", false},
		// View family folds onto VIEW.
		{"VIEW", "VIEW", false},
		{"MATERIALIZED VIEW", "VIEW", false},
		// Callables need an argument signature.
		{"FUNCTION", "FUNCTION", true},
		{"EXTERNAL FUNCTION", "FUNCTION", true},
		{"DATA METRIC FUNCTION", "FUNCTION", true},
		{"PROCEDURE", "PROCEDURE", true},
		// Case / whitespace are normalized.
		{"  iceberg table ", "TABLE", false},
		// Unmapped kinds pass through uppercased.
		{"STAGE", "STAGE", false},
		{"warehouse", "WAREHOUSE", false},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			gotDomain, gotCallable := tagReferenceDomain(tt.kind)
			if gotDomain != tt.wantDomain || gotCallable != tt.wantCallable {
				t.Errorf("tagReferenceDomain(%q) = (%q, %v), want (%q, %v)",
					tt.kind, gotDomain, gotCallable, tt.wantDomain, tt.wantCallable)
			}
		})
	}
}

func TestAccountLevelTagDomain(t *testing.T) {
	// Account-level domains resolve their INFORMATION_SCHEMA host database via
	// accountLevelFuncDatabase and use a bare object name; everything else is
	// scoped to its own database.
	accountLevel := []string{
		"WAREHOUSE", "ROLE", "USER", "INTEGRATION", "ACCOUNT",
		"COMPUTE POOL", "NETWORK POLICY",
		// Case / whitespace are normalized.
		"  warehouse ", "role",
	}
	for _, d := range accountLevel {
		if !accountLevelTagDomain(d) {
			t.Errorf("accountLevelTagDomain(%q) = false, want true", d)
		}
	}
	schemaLevel := []string{
		"TABLE", "VIEW", "SCHEMA", "DATABASE", "STAGE", "STREAM", "TASK",
		"STREAMLIT", "NETWORK RULE", "FUNCTION", "", "  ",
	}
	for _, d := range schemaLevel {
		if accountLevelTagDomain(d) {
			t.Errorf("accountLevelTagDomain(%q) = true, want false", d)
		}
	}
}

func TestTagReferenceObjectName(t *testing.T) {
	tests := []struct {
		name                  string
		kind, db, schema, obj string
		args                  string
		want                  string
	}{
		{"table", "TABLE", "DB", "SC", "T", "", `"DB"."SC"."T"`},
		{"view variant", "MATERIALIZED VIEW", "DB", "SC", "MV", "", `"DB"."SC"."MV"`},
		{"database is bare", "DATABASE", "DB", "", "DB", "", `"DB"`},
		{"schema is two parts", "SCHEMA", "DB", "SC", "SC", "", `"DB"."SC"`},
		{"procedure carries signature", "PROCEDURE", "DB", "SC", "P", "NUMBER, VARCHAR", `"DB"."SC"."P"(NUMBER, VARCHAR)`},
		{"function variant carries signature", "DATA METRIC FUNCTION", "DB", "SC", "F", "TABLE(NUMBER)", `"DB"."SC"."F"(TABLE(NUMBER))`},
		{"no-arg procedure still has parens", "PROCEDURE", "DB", "SC", "P", "", `"DB"."SC"."P"()`},
		// Account-level objects carry only a bare name — the db/schema the caller
		// passes are ignored (GetObjectTagReferences leaves them empty anyway).
		{"warehouse is bare", "WAREHOUSE", "", "", "WH", "", `"WH"`},
		{"role is bare", "ROLE", "", "", "MY_ROLE", "", `"MY_ROLE"`},
		{"integration is bare", "INTEGRATION", "", "", "MY_INT", "", `"MY_INT"`},
		{"user is bare", "USER", "", "", "ALICE", "", `"ALICE"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compose with tagReferenceDomain exactly as GetObjectTagReferences does,
			// so the test exercises the real fold-then-build path.
			d, callable := tagReferenceDomain(tt.kind)
			if got := tagReferenceObjectName(d, callable, tt.db, tt.schema, tt.obj, tt.args); got != tt.want {
				t.Errorf("tagReferenceObjectName(%q→%q, callable=%v, %q, %q, %q, %q) = %q, want %q",
					tt.kind, d, callable, tt.db, tt.schema, tt.obj, tt.args, got, tt.want)
			}
		})
	}
}
