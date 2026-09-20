// SPDX-License-Identifier: GPL-3.0-or-later

package ai

import (
	"strings"
	"testing"
)

func tbl(name string, ncols int) SchemaTable {
	t := SchemaTable{Name: name, Kind: "TABLE"}
	for i := 0; i < ncols; i++ {
		t.Columns = append(t.Columns, SchemaColumn{Name: "C" + string(rune('A'+i%26)) + string(rune('0'+i/26)), DataType: "NUMBER"})
	}
	return t
}

func TestSchemaBlock(t *testing.T) {
	ctx := SchemaContext{Tables: []SchemaTable{
		{Name: "ORDERS", Columns: []SchemaColumn{{Name: "ID", DataType: "NUMBER"}, {Name: "CUSTOMER_ID", DataType: "NUMBER"}},
			FKs: []SchemaFK{{Column: "CUSTOMER_ID", RefTable: "CUSTOMERS", RefColumn: "ID"}}},
		{Name: "CUSTOMERS", Columns: []SchemaColumn{{Name: "ID", DataType: "NUMBER"}}},
	}}
	got := SchemaBlock(ctx, 4096)
	want := "-- Schema context\nORDERS(ID NUMBER, CUSTOMER_ID NUMBER)\n-- ORDERS.CUSTOMER_ID -> CUSTOMERS.ID\nCUSTOMERS(ID NUMBER)\n\n"
	if got != want {
		t.Fatalf("SchemaBlock =\n%q\nwant\n%q", got, want)
	}
}

func TestSchemaBlock_Empty(t *testing.T) {
	if got := SchemaBlock(SchemaContext{}, 4096); got != "" {
		t.Fatalf("empty context = %q, want \"\"", got)
	}
	// A table with no cached columns contributes nothing, so the block is dropped.
	if got := SchemaBlock(SchemaContext{Tables: []SchemaTable{{Name: "ORDERS"}}}, 4096); got != "" {
		t.Fatalf("column-less table = %q, want \"\"", got)
	}
	if got := SchemaBlock(SchemaContext{Tables: []SchemaTable{tbl("A", 1)}}, 5); got != "" {
		t.Fatalf("tiny budget = %q, want \"\"", got)
	}
}

func TestSchemaBlock_TruncatesColumns(t *testing.T) {
	got := SchemaBlock(SchemaContext{Tables: []SchemaTable{tbl("WIDE", maxColumnsPerTable+7)}}, 100000)
	if !strings.Contains(got, "…(+7 more)") {
		t.Fatalf("missing overflow marker in %q", got)
	}
	if n := strings.Count(got, "NUMBER"); n != maxColumnsPerTable {
		t.Fatalf("emitted %d columns, want %d", n, maxColumnsPerTable)
	}
}

func TestSchemaBlock_StopsAtBudget(t *testing.T) {
	// Two tables, budget big enough for the header and the first only.
	first := tableLine(tbl("A", 3))
	got := SchemaBlock(SchemaContext{Tables: []SchemaTable{tbl("A", 3), tbl("B", 3)}}, len("-- Schema context\n")+len(first))
	if strings.Contains(got, "B(") {
		t.Fatalf("second table should not fit: %q", got)
	}
	if !strings.Contains(got, "A(") {
		t.Fatalf("first table should fit: %q", got)
	}
	if len(got) > len("-- Schema context\n")+len(first)+1 {
		t.Fatalf("block %d chars exceeds budget: %q", len(got), got)
	}
}

func TestSchemaCharBudget(t *testing.T) {
	if got := SchemaCharBudget(0); got != 4096 {
		t.Fatalf("SchemaCharBudget(0) = %d, want 4096", got)
	}
	if got := SchemaCharBudget(32768); got != 32768 {
		t.Fatalf("SchemaCharBudget(32768) = %d, want 32768", got)
	}
}
