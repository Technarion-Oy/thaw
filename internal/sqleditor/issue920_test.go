// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"reflect"
	"testing"

	sf "thaw/internal/snowflake"
)

// Issue #920: GetIdentifierAtColumn tokenizes the whole document with sqltok, so
// a cursor inside a comment or a literal is on no identifier, and each part
// keeps whether the source quoted it.
func TestGetIdentifierAtColumnTokenized(t *testing.T) {
	const doc = "-- SELECT * FROM ORDERS;\n" +
		"SELECT 'ORDERS' AS label FROM \"orders\";\n" +
		"/* block\n" +
		"   FROM ORDERS */\n" +
		"SELECT * FROM db.\"My Schema\".tbl;\n" +
		"$$ FROM ORDERS $$\n"

	tests := []struct {
		name      string
		line, col int
		want      []sf.IdentPart
	}{
		{"line comment", 1, 19, nil},      // ORDERS inside --
		{"string literal", 2, 10, nil},    // 'ORDERS'
		{"block comment body", 4, 9, nil}, // ORDERS inside /* */ opened a line earlier
		{"dollar-quoted body", 6, 9, nil}, // ORDERS inside $$ … $$
		{"quoted ident keeps its case", 2, 33, []sf.IdentPart{{Text: "orders", Quoted: true}}},
		{"bare parts fold, quoted part does not", 5, 22, []sf.IdentPart{
			{Text: "DB"}, {Text: "My Schema", Quoted: true}, {Text: "TBL"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetIdentifierAtColumn(doc, tt.line, tt.col); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIdentifierAtColumn(line %d, col %d) = %+v, want %+v", tt.line, tt.col, got, tt.want)
			}
		})
	}
}

// A dotted chain is byte-adjacent in the source. sqltok.Significant drops the
// whitespace and comments between tokens, so reading the chain off it alone
// would join `t.` with whatever follows the gap — and hand dot-autocomplete a
// qualifier (T.FROM) it then fails a SHOW/DESCRIBE on, per keystroke.
func TestGetIdentifierAtColumnStopsAtSourceGap(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		line, col int
		want      []sf.IdentPart
	}{
		// Adding a column to an existing query: cursor right after the typed dot.
		{"dot before a keyword on the same line", "SELECT t. FROM tbl t", 1, 10, []sf.IdentPart{{Text: "T"}}},
		{"dot at end of line", "SELECT t.\nFROM tbl t", 1, 10, []sf.IdentPart{{Text: "T"}}},
		{"comment between the dot and the next token", "SELECT a.\n-- c\nb", 1, 10, []sf.IdentPart{{Text: "A"}}},
		{"space between the dot and the next part", "db. schema", 1, 3, []sf.IdentPart{{Text: "DB"}}},
		{"space before the dot", "db .schema", 1, 1, []sf.IdentPart{{Text: "DB"}}},
		// The token after the gap is its own chain, not a continuation.
		{"next token starts a new chain", "SELECT t. FROM tbl t", 1, 11, []sf.IdentPart{{Text: "FROM"}}},
		// Adjacency is the rule, not line identity: a real dotted name still joins.
		{"adjacent parts still chain", "SELECT t.col FROM tbl t", 1, 10, []sf.IdentPart{{Text: "T"}, {Text: "COL"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetIdentifierAtColumn(tt.sql, tt.line, tt.col); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIdentifierAtColumn(%q, %d, %d) = %+v, want %+v", tt.sql, tt.line, tt.col, got, tt.want)
			}
		})
	}
}

// A quoted name is created case-sensitively, so "orders" and ORDERS are two
// objects — the hover path must not fold one onto the other.
func TestResolveStoreObjectQuotedIsCaseSensitive(t *testing.T) {
	objects := []StoreObject{
		{DB: "ANALYTICS", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
		{DB: "ANALYTICS", Schema: "PUBLIC", Name: "orders", Kind: "VIEW"},
	}
	sess := &SessionContext{Database: "ANALYTICS", Schema: "PUBLIC"}

	quoted := []sf.IdentPart{{Text: "orders", Quoted: true}}
	if got := ResolveStoreObject(quoted, objects, nil, sess); !got.Found || got.Name != "orders" {
		t.Errorf(`"orders" = %+v, want the lower-case VIEW`, got)
	}
	if got := ResolveStoreObject(bareParts("orders"), objects, nil, sess); !got.Found || got.Name != "ORDERS" {
		t.Errorf("bare orders = %+v, want the folded TABLE", got)
	}
	// A quoted name that exists in no case-exact form resolves to nothing, even
	// though a case-folded sibling is right there.
	missing := []sf.IdentPart{{Text: "Orders", Quoted: true}}
	if got := ResolveStoreObject(missing, objects, nil, sess); got.Found {
		t.Errorf(`"Orders" resolved to %+v, want no hit`, got)
	}
}
