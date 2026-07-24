// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"strings"
	"testing"
)

func queries(cmds []accountShowCmd) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.query
	}
	return out
}

func hasQuery(cmds []accountShowCmd, want string) bool {
	for _, c := range cmds {
		if c.query == want {
			return true
		}
	}
	return false
}

func TestShowLikeClause(t *testing.T) {
	if got := showLikeClause(""); got != "" {
		t.Errorf("empty pattern should yield no clause, got %q", got)
	}
	if got := showLikeClause("cust"); got != " LIKE '%cust%'" {
		t.Errorf("got %q", got)
	}
	// Single quotes in free text must be escaped so the literal stays intact.
	if got := showLikeClause("O'Brien"); !strings.Contains(got, "O''Brien") {
		t.Errorf("single quote not escaped: %q", got)
	}
}

func TestPlanAccountSearchCommands_AllKinds(t *testing.T) {
	cmds := planAccountSearchCommands("", nil, nil)
	// One SHOW OBJECTS + one per extended kind.
	if len(cmds) != 1+len(extendedShowKinds) {
		t.Fatalf("want %d commands, got %d", 1+len(extendedShowKinds), len(cmds))
	}
	if !hasQuery(cmds, "SHOW OBJECTS IN ACCOUNT") {
		t.Errorf("missing SHOW OBJECTS IN ACCOUNT; got %v", queries(cmds))
	}
	if !hasQuery(cmds, "SHOW STREAMLITS IN ACCOUNT") {
		t.Errorf("missing SHOW STREAMLITS IN ACCOUNT")
	}
	// Every command must be account-scoped.
	for _, c := range cmds {
		if !strings.HasSuffix(c.query, " IN ACCOUNT") {
			t.Errorf("command not account-scoped: %q", c.query)
		}
	}
}

func TestPlanAccountSearchCommands_SingleExtendedKind(t *testing.T) {
	// The Streamlit-only case the user reported: exactly one query, no per-schema walk.
	cmds := planAccountSearchCommands("", []string{"STREAMLIT"}, nil)
	if len(cmds) != 1 {
		t.Fatalf("want 1 command for STREAMLIT-only, got %d: %v", len(cmds), queries(cmds))
	}
	if cmds[0].query != "SHOW STREAMLITS IN ACCOUNT" {
		t.Errorf("got %q", cmds[0].query)
	}
	if cmds[0].fixedKind != "STREAMLIT" {
		t.Errorf("fixedKind = %q, want STREAMLIT", cmds[0].fixedKind)
	}
}

func TestPlanAccountSearchCommands_BasicKindUsesShowObjects(t *testing.T) {
	// TABLE/VIEW/SEQUENCE are sourced from SHOW OBJECTS, not a dedicated command.
	cmds := planAccountSearchCommands("", []string{"TABLE"}, nil)
	if len(cmds) != 1 || cmds[0].query != "SHOW OBJECTS IN ACCOUNT" || cmds[0].fixedKind != "" {
		t.Fatalf("TABLE should map to SHOW OBJECTS only, got %v", queries(cmds))
	}
}

func TestPlanAccountSearchCommands_MixedKinds(t *testing.T) {
	cmds := planAccountSearchCommands("", []string{"TABLE", "PROCEDURE"}, nil)
	if !hasQuery(cmds, "SHOW OBJECTS IN ACCOUNT") {
		t.Errorf("expected SHOW OBJECTS for TABLE")
	}
	if !hasQuery(cmds, "SHOW PROCEDURES IN ACCOUNT") {
		t.Errorf("expected SHOW PROCEDURES for PROCEDURE")
	}
	if len(cmds) != 2 {
		t.Errorf("want 2 commands, got %d: %v", len(cmds), queries(cmds))
	}
}

func TestPlanAccountSearchCommands_NamePatternPushdown(t *testing.T) {
	cmds := planAccountSearchCommands("cust", []string{"STREAMLIT"}, nil)
	if cmds[0].query != "SHOW STREAMLITS LIKE '%cust%' IN ACCOUNT" {
		t.Errorf("LIKE not pushed down: %q", cmds[0].query)
	}
}

func TestPlanAccountSearchCommands_ExcludedKindsSkipped(t *testing.T) {
	excl := map[string]bool{"STREAMLIT": true}
	cmds := planAccountSearchCommands("", nil, excl)
	if hasQuery(cmds, "SHOW STREAMLITS IN ACCOUNT") {
		t.Errorf("excluded STREAMLIT should be skipped")
	}
	if len(cmds) != len(extendedShowKinds) { // SHOW OBJECTS + (extended - 1)
		t.Errorf("want %d commands, got %d", len(extendedShowKinds), len(cmds))
	}
}
