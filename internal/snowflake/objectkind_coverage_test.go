// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"strings"
	"testing"

	"thaw/internal/objectkind"
)

// TestEveryKindIsSearchable asserts that every non-basic registry kind gets its
// own SHOW … IN ACCOUNT command in the account-wide search plan, and that the
// basic kinds get none (they ride along on SHOW OBJECTS). Without this a kind
// added to the registry could be listed in the sidebar's type filter while the
// search silently never looked for it.
func TestEveryKindIsSearchable(t *testing.T) {
	byKind := make(map[string]string)
	for _, cmd := range planAccountSearchCommands("", nil, nil, 0) {
		byKind[cmd.fixedKind] = cmd.query
	}
	if _, ok := byKind[""]; !ok {
		t.Error("the plan must always include SHOW OBJECTS for the basic kinds")
	}
	for _, k := range objectkind.Kinds {
		query, planned := byKind[k.Name]
		if k.Basic {
			if planned {
				t.Errorf("%s is basic but got its own command %q — it should come from SHOW OBJECTS", k.Name, query)
			}
			continue
		}
		if !planned {
			t.Errorf("%s has no SHOW … IN ACCOUNT command — the account-wide search can never find it", k.Name)
			continue
		}
		if want := "SHOW " + k.Plural + " IN ACCOUNT"; query != want {
			t.Errorf("%s: got %q, want %q", k.Name, query, want)
		}
	}
}

// TestEveryKindIsListable asserts the same for the per-schema listing: a kind not
// covered by SHOW OBJECTS must have a dedicated SHOW … IN SCHEMA command, or it
// never appears in the object tree.
func TestEveryKindIsListable(t *testing.T) {
	listed := make(map[string]bool, len(extendedShowKinds))
	for _, k := range extendedShowKinds {
		listed[k.Name] = true
	}
	for _, k := range objectkind.Kinds {
		if k.Basic == listed[k.Name] {
			t.Errorf("%s: Basic = %v but %v in the extended SHOW list — a kind must be sourced from exactly one of the two",
				k.Name, k.Basic, listed[k.Name])
		}
	}
}

// TestEveryKindHasDDLDisposition asserts that every registry kind either resolves
// to a GET_DDL object type or is rejected up front — never falls through to a
// GET_DDL('<SHOW kind>', …) the server will refuse. It also pins the
// normalizations that are not simply the SHOW kind (the policy family, the
// underscore forms, the table/function foldings), since getting one wrong yields
// a query that fails only against a live account.
func TestEveryKindHasDDLDisposition(t *testing.T) {
	for _, k := range objectkind.Kinds {
		if k.GetDDLType == "" {
			if !DDLUnsupportedKinds[k.Name] {
				t.Errorf("%s has no GET_DDL type but is not rejected by GetObjectDDL — it would emit an invalid GET_DDL", k.Name)
			}
			continue
		}
		if DDLUnsupportedKinds[k.Name] {
			t.Errorf("%s is rejected by GetObjectDDL yet declares GET_DDL type %q", k.Name, k.GetDDLType)
		}
		query, _ := buildGetDDLQuery("DB", "SCH", k.Name, "OBJ", "NUMBER")
		if want := "SELECT GET_DDL('" + k.GetDDLType + "', "; !strings.HasPrefix(query, want) {
			t.Errorf("%s: query %q does not use the registered object type %q", k.Name, query, k.GetDDLType)
		}
		// Only routines carry a signature; anything else must not grow parentheses.
		hasArgs := strings.Contains(query, `"OBJ"(NUMBER)`)
		if hasArgs != k.Routine {
			t.Errorf("%s: Routine = %v but the identifier %s the argument signature", k.Name, k.Routine, map[bool]string{true: "carries", false: "omits"}[hasArgs])
		}
	}
}

// TestDDLKindNormalizations pins the handful of kinds whose GET_DDL object type
// differs from their SHOW kind. These are the cases the old switch statement
// existed for; asserting them here means the registry can't quietly lose one.
func TestDDLKindNormalizations(t *testing.T) {
	want := map[string]string{
		"DYNAMIC TABLE":            "DYNAMIC_TABLE",
		"EXTERNAL TABLE":           "EXTERNAL_TABLE",
		"EVENT TABLE":              "EVENT_TABLE",
		"NETWORK RULE":             "NETWORK_RULE",
		"ICEBERG TABLE":            "TABLE",
		"HYBRID TABLE":             "TABLE",
		"MATERIALIZED VIEW":        "VIEW",
		"EXTERNAL FUNCTION":        "FUNCTION",
		"DATA METRIC FUNCTION":     "FUNCTION",
		"AGENT":                    "CORTEX_AGENT",
		"MASKING POLICY":           "POLICY",
		"ROW ACCESS POLICY":        "POLICY",
		"JOIN POLICY":              "POLICY",
		"PRIVACY POLICY":           "POLICY",
		"PASSWORD POLICY":          "POLICY",
		"SESSION POLICY":           "POLICY",
		"AGGREGATION POLICY":       "POLICY",
		"PROJECTION POLICY":        "POLICY",
		"AUTHENTICATION POLICY":    "POLICY",
		"STORAGE LIFECYCLE POLICY": "POLICY",
	}
	for kind, ddlType := range want {
		k, ok := objectkind.ByName(kind)
		if !ok {
			t.Errorf("%s dropped out of the registry", kind)
			continue
		}
		if k.GetDDLType != ddlType {
			t.Errorf("%s: GetDDLType = %q, want %q", kind, k.GetDDLType, ddlType)
		}
	}
	// Kinds outside the registry keep the caller's kind verbatim.
	if query, _ := buildGetDDLQuery("", "", "DATABASE", "DB", ""); !strings.HasPrefix(query, "SELECT GET_DDL('DATABASE', ") {
		t.Errorf("account-level DATABASE lost its object type: %q", query)
	}
}
