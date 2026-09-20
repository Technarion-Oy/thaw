// SPDX-License-Identifier: GPL-3.0-or-later

package ai

import (
	"fmt"
	"regexp"
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
// There is no Kind: views, dynamic and iceberg tables all render the same way, so
// nothing downstream has a use for it yet.
type SchemaTable struct {
	DB      string         `json:"db"`
	Schema  string         `json:"schema"`
	Name    string         `json:"name"`
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

// SchemaBlock renders ctx as DDL-like lines to prepend to a completion prompt,
// each table's foreign keys as comments right below it:
//
//	-- Schema context
//	ORDERS(ID NUMBER, CUSTOMER_ID NUMBER)
//	-- ORDERS.CUSTOMER_ID -> CUSTOMERS.ID
//	CUSTOMERS(ID NUMBER, EMAIL VARCHAR)
//
// charBudget caps the whole block; tables are emitted in the order given (the
// caller passes them nearest-to-the-cursor first) and the first one that would
// overflow ends the block. Returns "" when nothing fits or nothing is known.
func SchemaBlock(ctx SchemaContext, charBudget int) string {
	const header = "-- Schema context\n"
	if len(ctx.Tables) == 0 || charBudget <= len(header) {
		return ""
	}

	// Two tables of the same name from different schemas are indistinguishable
	// when rendered bare, so those — and only those — carry their schema.
	byName := map[string]int{}
	for _, t := range ctx.Tables {
		byName[t.Name]++
	}

	var b strings.Builder
	b.WriteString(header)
	for _, t := range ctx.Tables {
		line := tableLine(t, byName[t.Name] > 1)
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
	// The blank separator line is deliberately added after the cap: one byte
	// over budget is cheaper than reserving for it on every call.
	return b.String() + "\n"
}

// tableLine renders one table's signature plus its FK comment lines, or "" when
// the table has no columns cached (a bare name tells the model nothing it can't
// already read in the prefix). qualify prefixes the name with its schema.
func tableLine(t SchemaTable, qualify bool) string {
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
		parts = append(parts, strings.TrimSpace(quoteIdent(c.Name)+" "+c.DataType))
	}
	if extra > 0 {
		parts = append(parts, fmt.Sprintf("…(+%d more)", extra))
	}

	name := quoteIdent(t.Name)
	if qualify && t.Schema != "" {
		name = quoteIdent(t.Schema) + "." + name
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s(%s)\n", name, strings.Join(parts, ", "))
	for _, fk := range t.FKs {
		if fk.Column == "" || fk.RefTable == "" || fk.RefColumn == "" {
			continue
		}
		fmt.Fprintf(&b, "-- %s.%s -> %s.%s\n",
			name, quoteIdent(fk.Column), quoteIdent(fk.RefTable), quoteIdent(fk.RefColumn))
	}
	return b.String()
}

// bareIdent matches an identifier Snowflake resolves unquoted. The frontend
// resolves refs through the same normalisation as the diagnostics pass (#922),
// so anything else here is a name that was quoted in the source and stays
// case-sensitive — it has to reach the model with its quotes.
var bareIdent = regexp.MustCompile(`^[A-Z_][A-Z0-9_$]*$`)

// quoteIdent double-quotes an identifier that Snowflake would not fold to itself.
func quoteIdent(id string) string {
	if id == "" || bareIdent.MatchString(id) {
		return id
	}
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
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

// completionInstruction is the standing instruction for inline completions; it
// follows the schema block so the model reads the catalog before the task.
const completionInstruction = "Complete this Snowflake SQL query. Return ONLY the completion text to insert at the cursor — no explanation, no markdown, no repetition of existing text. Keep it to 1–2 lines.\n\n"

// BuildPrompt assembles the inline-completion prompt: the schema block (when
// includeSchema is set), the instruction, then the text before the cursor.
// numCtx is the configured Ollama context window (0 for hosted providers).
//
// Whether schema context may be sent is a privacy decision, so it is a single
// explicit parameter rather than something inferred from the context being empty.
func BuildPrompt(prefix string, ctx SchemaContext, numCtx int, includeSchema bool) string {
	block := ""
	if includeSchema {
		block = SchemaBlock(ctx, SchemaCharBudget(numCtx))
	}
	return block + completionInstruction + prefix
}
