// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import "testing"

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
		parts     []string
		session   *SessionContext
		wantFound bool
		wantDB    string
		wantSchem string
		wantFetch [2]string // FetchDB, FetchSchema on a miss
	}{
		{"1-part in session schema", []string{"ORDERS"}, sess, true, "ANALYTICS", "PUBLIC", [2]string{}},
		{"1-part case-folded", []string{"orders"}, sess, true, "ANALYTICS", "PUBLIC", [2]string{}},
		// The bug: ANALYTICS.RAW.EVENTS must not light up a bare EVENTS hovered
		// while the session schema is PUBLIC.
		{"1-part in another schema", []string{"EVENTS"}, sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},
		{"1-part in another database", []string{"CUSTOMERS"}, sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},
		// A temp table the script creates but has never run is in no schema listing.
		{"1-part not created yet", []string{"ORDERS_TMP"}, sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},

		{"2-part in session database", []string{"RAW", "EVENTS"}, sess, true, "ANALYTICS", "RAW", [2]string{}},
		{"2-part in another database", []string{"PUBLIC", "CUSTOMERS"}, sess, false, "", "", [2]string{"ANALYTICS", "PUBLIC"}},

		{"3-part exact", []string{"OTHER", "PUBLIC", "CUSTOMERS"}, sess, true, "OTHER", "PUBLIC", [2]string{}},
		{"3-part wrong database", []string{"NOPE", "PUBLIC", "CUSTOMERS"}, sess, false, "", "", [2]string{"NOPE", "PUBLIC"}},
		// A fully-qualified name needs no session at all.
		{"3-part without session", []string{"OTHER", "PUBLIC", "CUSTOMERS"}, nil, true, "OTHER", "PUBLIC", [2]string{}},

		// Nothing to resolve against → no hit and no namespace worth loading.
		{"1-part without session schema", []string{"ORDERS"}, &SessionContext{Database: "ANALYTICS"}, false, "", "", [2]string{}},
		{"2-part without session database", []string{"RAW", "EVENTS"}, &SessionContext{Schema: "PUBLIC"}, false, "", "", [2]string{}},
		{"no parts", nil, sess, false, "", "", [2]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStoreObject(tt.parts, objects, tt.session)
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
	if got := ResolveStoreObject([]string{"ORDERS"}, objects, sess); !got.Found || got.Kind != "TABLE" {
		t.Errorf("ORDERS = %+v, want the TABLE", got)
	}
	if got := ResolveStoreObject([]string{"EVENTS"}, objects, sess); !got.Found || got.Kind != "STREAM" {
		t.Errorf("EVENTS = %+v, want the STREAM (only kind present)", got)
	}
	for _, n := range []string{"MY_UDF", "MY_PROC"} {
		if got := ResolveStoreObject([]string{n}, objects, sess); got.Found {
			t.Errorf("%s resolved to %+v, want no hit (callable kinds are excluded)", n, got)
		}
	}
}
