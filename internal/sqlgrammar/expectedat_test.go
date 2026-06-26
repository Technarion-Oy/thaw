package sqlgrammar

import (
	"slices"
	"strings"
	"testing"
)

// TestExpectedAt verifies that the grammar's "valid next" set at the cursor is
// the context-aware completion set autocomplete needs: each input is a statement
// prefix (the cursor sits at its end) and we assert the labels that must / must
// not appear.
func TestExpectedAt(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		want    []string // labels that must be present
		notWant []string // labels that must be absent
	}{
		// The issue's worked example: after COPY INTO <table>, FROM is next.
		{name: "COPY INTO table → FROM", prefix: "COPY INTO mytable ", want: []string{"FROM"}},
		// CREATE <leader> offers the object-type keywords + IF, plus an identifier kind.
		{name: "CREATE → object types", prefix: "CREATE ", want: []string{"TABLE", "VIEW", "DATABASE", "OR"}},
		// CREATE TABLE expects an optional IF NOT EXISTS, then the table name.
		{name: "CREATE TABLE → name/IF", prefix: "CREATE TABLE ", want: []string{"IF", "identifier"}},
		// ALTER TABLE <name> offers the alter-action verbs.
		{name: "ALTER TABLE → actions", prefix: "ALTER TABLE foo ", want: []string{"RENAME", "SET", "ADD", "DROP"}},
		// DROP offers droppable object types.
		{name: "DROP → object types", prefix: "DROP ", want: []string{"TABLE", "VIEW", "SCHEMA"}},
		// A trailing comma in the projection means another item is required: expect
		// an expression, NOT the next clause's keyword. This is what lets
		// autocomplete offer columns on a blank line mid-SELECT (e.g. cursor on the
		// empty line in `SELECT\n  AMOUNT,\n  <cursor>\nFROM t`) instead of FROM.
		{
			name:    "SELECT projection trailing comma → expression",
			prefix:  "SELECT \nAMOUNT,\n",
			want:    []string{"expression"},
			notWant: []string{"FROM", "WHERE", "GROUP"},
		},
		// Same rule for a comma-separated FROM list: another table is expected.
		{
			name:    "FROM list trailing comma → identifier",
			prefix:  "SELECT a FROM t1, ",
			want:    []string{"identifier"},
			notWant: []string{"WHERE", "GROUP"},
		},
		// An order-independent parameter already set (unorderedOnce) is not
		// re-offered: after START and INCREMENT, only the remaining parameters
		// (ORDER/NOORDER, COMMENT) are expected — never START or INCREMENT again.
		{
			name:    "CREATE SEQUENCE drops already-set params",
			prefix:  "CREATE SEQUENCE s START = 1 INCREMENT = 1 ",
			want:    []string{"ORDER", "NOORDER", "COMMENT"},
			notWant: []string{"START", "INCREMENT"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.prefix)
			got := v.ExpectedAt(len(tt.prefix))
			for _, w := range tt.want {
				if !slices.Contains(got, w) {
					t.Errorf("ExpectedAt(%q) = %v; missing %q", tt.prefix, got, w)
				}
			}
			for _, w := range tt.notWant {
				if slices.Contains(got, w) {
					t.Errorf("ExpectedAt(%q) = %v; unexpected %q", tt.prefix, got, w)
				}
			}
		})
	}
}

// TestExpectedAtPartialWord verifies the half-typed word abutting the cursor is
// dropped before parsing: typing the first letters of FROM must still yield the
// same expectation as the clean clause boundary.
func TestExpectedAtPartialWord(t *testing.T) {
	clean := "COPY INTO mytable "
	typing := "COPY INTO mytable FRO"
	gotClean := New(clean).ExpectedAt(len(clean))
	gotTyping := New(typing).ExpectedAt(len(typing))
	if !slices.Contains(gotTyping, "FROM") {
		t.Errorf("ExpectedAt(%q) = %v; want FROM (partial word should be dropped)", typing, gotTyping)
	}
	if strings.Join(gotClean, ",") != strings.Join(gotTyping, ",") {
		t.Errorf("partial-word prefix should match clean prefix:\n clean  = %v\n typing = %v", gotClean, gotTyping)
	}
}

// TestExpectedAtMidToken verifies a cursor inside a token excludes that token —
// completing the table name mid-word reports the expectations before the name.
func TestExpectedAtMidToken(t *testing.T) {
	src := "CREATE TABLE myt"
	// Cursor after "my" (offset 14), inside the "myt" identifier.
	got := New(src).ExpectedAt(14)
	if !slices.Contains(got, "identifier") {
		t.Errorf("ExpectedAt(%q, 14) = %v; want identifier", src, got)
	}
}

// TestExpectedAtUnrecognized verifies an unrecognized or empty leading keyword
// yields no expectations, so callers can keep completion leading-keyword-gated.
func TestExpectedAtUnrecognized(t *testing.T) {
	for _, s := range []string{"", "   ", "FLOOBAR x y"} {
		if got := New(s).ExpectedAt(len(s)); len(got) != 0 {
			t.Errorf("ExpectedAt(%q) = %v; want empty", s, got)
		}
	}
}
