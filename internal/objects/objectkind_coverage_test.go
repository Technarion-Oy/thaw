// SPDX-License-Identifier: GPL-3.0-or-later

package objects

import (
	"fmt"
	"testing"

	"thaw/internal/objectkind"
)

// TestEveryKindHasPropertiesQuery asserts that every registry kind either builds
// a Properties query or is explicitly opted out. Without it a kind could be
// listed in the object tree with a Properties context-menu item that always
// errors with "unsupported object kind".
func TestEveryKindHasPropertiesQuery(t *testing.T) {
	for _, k := range objectkind.Kinds {
		query, err := BuildObjectPropertiesQuery("DB", "SCH", k.Name, "OBJ")
		if k.NoPropertiesQuery {
			if err == nil {
				t.Errorf("%s opts out of the generic Properties query but built one: %q", k.Name, query)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s has no Properties query: %v", k.Name, err)
			continue
		}
		want := fmt.Sprintf("SHOW %s LIKE 'OBJ' IN SCHEMA \"DB\".\"SCH\"", k.Plural)
		if query != want {
			t.Errorf("%s:\n got  %s\n want %s", k.Name, query, want)
		}
	}
}

// TestAccountScopedPropertiesQueries pins the kinds that live outside the
// schema-scoped registry: each needs its own scope clause, so they stay explicit
// in the builder and must keep working.
func TestAccountScopedPropertiesQueries(t *testing.T) {
	tests := []struct{ kind, want string }{
		{"DATABASE", "SHOW DATABASES LIKE 'OBJ'"},
		{"SCHEMA", `SHOW SCHEMAS LIKE 'OBJ' IN DATABASE "DB"`},
		{"WAREHOUSE", "SHOW WAREHOUSES LIKE 'OBJ'"},
		{"ROLE", "SHOW ROLES LIKE 'OBJ'"},
		{"USER", "SHOW USERS LIKE 'OBJ'"},
	}
	for _, tt := range tests {
		got, err := BuildObjectPropertiesQuery("DB", "SCH", tt.kind, "OBJ")
		if err != nil {
			t.Errorf("%s: %v", tt.kind, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s:\n got  %s\n want %s", tt.kind, got, tt.want)
		}
	}
	if _, err := BuildObjectPropertiesQuery("DB", "SCH", "NOT A KIND", "OBJ"); err == nil {
		t.Error("an unknown kind must be rejected, not turned into a SHOW command")
	}
}
