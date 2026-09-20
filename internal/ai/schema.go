// SPDX-License-Identifier: GPL-3.0-or-later

package ai

import (
	"fmt"
	"strings"
)

// SchemaContext is the in-memory schema knowledge the editor already has for the
// statement under the cursor: the resolved table references plus their cached
// columns and foreign keys. The frontend fills it from its own caches (a cache
// miss simply omits the table), so building it never costs a Snowflake round-trip.
type SchemaContext struct {
	Tables []SchemaTable `json:"tables"`
}

// SchemaTable is one resolved object reference with whatever the editor had cached.
type SchemaTable struct {
	DB      string         `json:"db"`
	Schema  string         `json:"schema"`
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	Columns []SchemaColumn `json:"columns"`
	FKs     []SchemaFK     `json:"fks"`
}

// SchemaColumn is a column name with its Snowflake data type.
type SchemaColumn struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// SchemaFK is one foreign-key edge: Column in the owning table references
// RefTable.RefColumn.
type SchemaFK struct {
	Column    string `json:"column"`
	RefTable  string `json:"refTable"`
	RefColumn string `json:"refColumn"`
}

// maxColumnsPerTable caps how many columns one table contributes before the rest
// are summarized as "…(+k more)". Wide tables (Snowflake allows thousands of
// columns) would otherwise consume the whole budget on the first entry.
const maxColumnsPerTable = 40

// SchemaBlock renders ctx as DDL-like lines to prepend to a completion prompt:
//
//	-- Schema context
//	CUSTOMERS(ID NUMBER, EMAIL VARCHAR)
//	ORDERS(ID NUMBER, CUSTOMER_ID NUMBER)
//	-- ORDERS.CUSTOMER_ID -> CUSTOMERS.ID
//
// charBudget caps the whole block; tables are emitted in the order given (the
// caller passes them nearest-to-the-cursor first) and the first one that would
// overflow ends the block. Returns "" when nothing fits or nothing is known.
func SchemaBlock(ctx SchemaContext, charBudget int) string {
	const header = "-- Schema context\n"
	if len(ctx.Tables) == 0 || charBudget <= len(header) {
		return ""
	}

	var b strings.Builder
	b.WriteString(header)
	for _, t := range ctx.Tables {
		line := tableLine(t)
		if line == "" {
			continue
		}
		if b.Len()+len(line) > charBudget {
			break
		}
		b.WriteString(line)
	}
	if b.Len() == len(header) {
		return ""
	}
	return b.String() + "\n"
}

// tableLine renders one table's signature plus its FK comment lines, or "" when
// the table has no columns cached (a bare name tells the model nothing it can't
// already read in the prefix).
func tableLine(t SchemaTable) string {
	if len(t.Columns) == 0 {
		return ""
	}
	cols := t.Columns
	extra := 0
	if len(cols) > maxColumnsPerTable {
		extra = len(cols) - maxColumnsPerTable
		cols = cols[:maxColumnsPerTable]
	}
	parts := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		parts = append(parts, strings.TrimSpace(c.Name+" "+c.DataType))
	}
	if extra > 0 {
		parts = append(parts, fmt.Sprintf("…(+%d more)", extra))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s(%s)\n", t.Name, strings.Join(parts, ", "))
	for _, fk := range t.FKs {
		if fk.Column == "" || fk.RefTable == "" || fk.RefColumn == "" {
			continue
		}
		fmt.Fprintf(&b, "-- %s.%s -> %s.%s\n", t.Name, fk.Column, fk.RefTable, fk.RefColumn)
	}
	return b.String()
}

// SchemaCharBudget returns the character budget for the schema block.
// Ollama users commonly run a 4 096-token window, so scale with the configured
// one: ~4 chars per token means numCtx chars is roughly a quarter of the window,
// leaving the rest for the instruction, the prefix and the completion.
// numCtx <= 0 (hosted providers, or "let Ollama decide") uses the 4 096 default.
func SchemaCharBudget(numCtx int) int {
	if numCtx <= 0 {
		return 4096
	}
	return numCtx
}
