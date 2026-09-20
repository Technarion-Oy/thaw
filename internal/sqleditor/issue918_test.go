// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"strings"
	"testing"

	sf "thaw/internal/snowflake"
)

// Issue #918: the editor's cmd/ctrl-hover DDL link must only appear for objects
// that actually exist in the namespace the hovered name resolves against.
// ResolveStoreObject qualifies from session first, then matches strictly.
func TestResolveStoreObjectNamespaceScoping(t *testing.T) {
	objects := []StoreObject{
		{DB: "ANALYTICS", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
		{DB: "ANALYTICS", Schema: "RAW", Name: "EVENTS", Kind: "TABLE"},
		{DB: "OTHER", Schema: "PUBLIC", Name: "CUSTOMERS", Kind: "TABLE"},
	}
	sess := &SessionContext{Database: "ANALYTICS", Schema: "PUBLIC"}

	tests := []struct {
		name      string
		parts     []sf.IdentPart
		session   *SessionContext
		wantFound bool
		wantDB    string
		wantSchem string
		wantFetch [2]string // FetchDB, FetchSchema on a miss
	}{
		{"1-part in session schema", bareParts("ORDERS"), sess, true, "ANALYTICS", "PUBLIC", [2]string{}},
		{"1-part case-folded", bareParts("orders"), sess, true, "ANALYTICS", "PUBLIC", [2]string{}},
		// The bug: ANALYTICS.RAW.EVENTS must not light up a bare EVENTS hovered
		// while the session schema is PUBLIC.
		{"1-part in another schema", bareParts("EVENTS"), sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},
		{"1-part in another database", bareParts("CUSTOMERS"), sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},
		// A temp table the script creates but has never run is in no schema listing.
		{"1-part not created yet", bareParts("ORDERS_TMP"), sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},

		{"2-part in session database", bareParts("RAW", "EVENTS"), sess, true, "ANALYTICS", "RAW", [2]string{}},
		{"2-part in another database", bareParts("PUBLIC", "CUSTOMERS"), sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},

		{"3-part exact", bareParts("OTHER", "PUBLIC", "CUSTOMERS"), sess, true, "OTHER", "PUBLIC", [2]string{}},
		{"3-part wrong database", bareParts("NOPE", "PUBLIC", "CUSTOMERS"), sess, false, "", "", [2]string{"NOPE", "PUBLIC"}},
		// A fully-qualified name needs no session at all.
		{"3-part without session", bareParts("OTHER", "PUBLIC", "CUSTOMERS"), nil, true, "OTHER", "PUBLIC", [2]string{}},

		// Nothing to resolve against → no hit and no namespace worth loading.
		{"1-part without session schema", bareParts("ORDERS"), &SessionContext{Database: "ANALYTICS"}, false, "", "", [2]string{}},
		{"2-part without session database", bareParts("RAW", "EVENTS"), &SessionContext{Schema: "PUBLIC"}, false, "", "", [2]string{}},
		{"no parts", nil, sess, false, "", "", [2]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStoreObject(tt.parts, objects, nil, tt.session)
			if got.Found != tt.wantFound {
				t.Fatalf("Found = %v, want %v (%+v)", got.Found, tt.wantFound, got)
			}
			if tt.wantFound && (got.DB != tt.wantDB || got.Schema != tt.wantSchem) {
				t.Errorf("resolved %s.%s, want %s.%s", got.DB, got.Schema, tt.wantDB, tt.wantSchem)
			}
			if !tt.wantFound {
				if [2]string{got.FetchDB, got.FetchSchema} != tt.wantFetch {
					t.Errorf("fetch hint = %q.%q, want %q.%q",
						got.FetchDB, got.FetchSchema, tt.wantFetch[0], tt.wantFetch[1])
				}
			}
		})
	}
}

// Within one namespace a name can be shared (a stream named after its source
// table); the TABLE/VIEW wins. Callable kinds never resolve — GET_DDL needs an
// overload argument list a bare hover cannot supply.
func TestResolveStoreObjectKindPreference(t *testing.T) {
	sess := &SessionContext{Database: "D", Schema: "S"}
	objects := []StoreObject{
		{DB: "D", Schema: "S", Name: "ORDERS", Kind: "STREAM"},
		{DB: "D", Schema: "S", Name: "ORDERS", Kind: "TABLE"},
		{DB: "D", Schema: "S", Name: "EVENTS", Kind: "STREAM"},
		{DB: "D", Schema: "S", Name: "MY_UDF", Kind: "FUNCTION"},
		{DB: "D", Schema: "S", Name: "MY_PROC", Kind: "PROCEDURE"},
	}
	if got := ResolveStoreObject(bareParts("ORDERS"), objects, nil, sess); !got.Found || got.Kind != "TABLE" {
		t.Errorf("ORDERS = %+v, want the TABLE", got)
	}
	if got := ResolveStoreObject(bareParts("EVENTS"), objects, nil, sess); !got.Found || got.Kind != "STREAM" {
		t.Errorf("EVENTS = %+v, want the STREAM (only kind present)", got)
	}
	for _, n := range []string{"MY_UDF", "MY_PROC"} {
		if got := ResolveStoreObject(bareParts(n), objects, nil, sess); got.Found {
			t.Errorf("%s resolved to %+v, want no hit (callable kinds are excluded)", n, got)
		}
	}
}

// An in-script USE DATABASE/SCHEMA outranks the tab's session for the qualifier a
// name omits — the precedence ResolveTableRefs already applies for diagnostics.
// Before this, a worksheet opening with USE SCHEMA X lost its links for bare
// names until it was run and the session caught up.
func TestResolveStoreObjectUseContext(t *testing.T) {
	objects := []StoreObject{
		{DB: "ANALYTICS", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
		{DB: "ANALYTICS", Schema: "STAGING", Name: "ORDERS", Kind: "TABLE"},
		{DB: "OTHER", Schema: "STAGING", Name: "ORDERS", Kind: "TABLE"},
	}
	sess := &SessionContext{Database: "ANALYTICS", Schema: "PUBLIC"}

	if got := ResolveStoreObject(bareParts("ORDERS"), objects, &UseContext{Schema: "STAGING"}, sess); !got.Found || got.Schema != "STAGING" {
		t.Errorf("USE SCHEMA STAGING: got %+v, want ANALYTICS.STAGING", got)
	}
	if got := ResolveStoreObject(bareParts("ORDERS"), objects, &UseContext{Database: "OTHER", Schema: "STAGING"}, sess); !got.Found || got.DB != "OTHER" {
		t.Errorf("USE DATABASE OTHER: got %+v, want OTHER.STAGING", got)
	}
	// A partial USE leaves the other half to the session.
	if got := ResolveStoreObject(bareParts("STAGING", "ORDERS"), objects, &UseContext{Schema: "IGNORED"}, sess); !got.Found || got.DB != "ANALYTICS" {
		t.Errorf("2-part with USE SCHEMA: got %+v, want the session database", got)
	}
	// A fully-qualified name ignores both contexts.
	if got := ResolveStoreObject(bareParts("OTHER", "STAGING", "ORDERS"), objects, &UseContext{Database: "ANALYTICS"}, sess); !got.Found || got.DB != "OTHER" {
		t.Errorf("3-part: got %+v, want the written database", got)
	}
}

// UseContextAt reports the USE context in effect at a cursor offset: statements
// before the cursor count, later ones do not.
func TestUseContextAt(t *testing.T) {
	sql := "USE SCHEMA STAGING;\nSELECT * FROM ORDERS;\nUSE SCHEMA LATER;\nSELECT 1;"
	at := func(needle string) *UseContext {
		return UseContextAt(sql, len([]rune(sql[:strings.Index(sql, needle)])))
	}
	if got := at("ORDERS"); got == nil || got.Schema != "STAGING" {
		t.Errorf("at ORDERS: got %+v, want STAGING", got)
	}
	if got := at("SELECT 1"); got == nil || got.Schema != "LATER" {
		t.Errorf("at SELECT 1: got %+v, want LATER", got)
	}
	if got := UseContextAt("SELECT * FROM ORDERS;", 5); got != nil {
		t.Errorf("no USE statement: got %+v, want nil", got)
	}
}

// bareParts builds the unquoted identifier path GetIdentifierAtColumn returns
// for a name written without double quotes.
func bareParts(names ...string) []sf.IdentPart {
	if len(names) == 0 {
		return nil
	}
	parts := make([]sf.IdentPart, len(names))
	for i, n := range names {
		parts[i] = sf.IdentPart{Text: n}
	}
	return parts
}
