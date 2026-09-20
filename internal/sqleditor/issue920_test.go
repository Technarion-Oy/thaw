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
