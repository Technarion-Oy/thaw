// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/logger"
	"thaw/internal/snowflake"
)

// Tool input types for open_er_designer.

type openERDesignerInput struct {
	Database string              `json:"database" jsonschema:"the database name"`
	Tables   []erDesignerTableIn `json:"tables,omitempty" jsonschema:"optional AI-generated tables to pre-populate on the canvas; omit to open with live schema only"`
}

type erDesignerTableIn struct {
	Schema  string               `json:"schema" jsonschema:"the schema name for this table"`
	Name    string               `json:"name" jsonschema:"the table name"`
	Columns []erDesignerColumnIn `json:"columns" jsonschema:"columns for this table"`
}

type erDesignerColumnIn struct {
	Name     string `json:"name" jsonschema:"the column name"`
	DataType string `json:"dataType" jsonschema:"Snowflake data type, e.g. VARCHAR(256), NUMBER(38,0)"`
	IsPK     bool   `json:"isPK,omitempty" jsonschema:"true if this column is part of the primary key"`
	NotNull  bool   `json:"notNull,omitempty" jsonschema:"true if the column has a NOT NULL constraint"`
	FKRef    string `json:"fkRef,omitempty" jsonschema:"foreign key reference in SCHEMA.TABLE.COLUMN format"`
	Default  string `json:"defaultValue,omitempty" jsonschema:"existing column DEFAULT expression to preserve across edits (echo the value from get_er_designer_state, especially when renaming a column); leave empty for new columns"`
}

// OpenERDesignerPayload is the Wails event payload for "mcp:open-er-designer".
// The frontend listens for this event and opens the ER designer.
type OpenERDesignerPayload struct {
	Database string                `json:"database"`
	Merged   snowflake.ERDiagramData `json:"merged"`
	Baseline snowflake.ERDiagramData `json:"baseline"`
}

// mergeAITables merges AI-generated tables into a live ERDiagramData snapshot.
// AI tables that match an existing live table (by uppercase SCHEMA.NAME) replace
// it; unmatched AI tables are appended. Live tables not touched by AI are
// preserved. FKs from replaced tables are dropped; AI-provided fkRef fields are
// converted to ERForeignKey entries.
func mergeAITables(live snowflake.ERDiagramData, aiTables []erDesignerTableIn) snowflake.ERDiagramData {
	// Build a set of live tables keyed by "SCHEMA.NAME" (uppercase).
	liveMap := make(map[string]int, len(live.Tables))
	for i, t := range live.Tables {
		key := strings.ToUpper(t.Schema) + "." + strings.ToUpper(t.Name)
		liveMap[key] = i
	}

	// Track which live tables are replaced by AI tables.
	replaced := make(map[string]bool, len(aiTables))

	// Convert AI tables to ERTable entries.
	var aiERTables []snowflake.ERTable
	var aiFKs []snowflake.ERForeignKey
	for _, at := range aiTables {
		key := strings.ToUpper(at.Schema) + "." + strings.ToUpper(at.Name)
		if _, exists := liveMap[key]; exists {
			replaced[key] = true
		}
		var cols []snowflake.ERColumn
		for _, c := range at.Columns {
			nullable := "YES"
			if c.NotNull || c.IsPK {
				nullable = "NO"
			}
			cols = append(cols, snowflake.ERColumn{
				Name:     c.Name,
				DataType: c.DataType,
				IsPK:     c.IsPK,
				Nullable: nullable,
				Default:  c.Default,
			})
			// Convert fkRef to ERForeignKey.
			if c.FKRef != "" {
				parts := strings.Split(c.FKRef, ".")
				if len(parts) == 3 {
					aiFKs = append(aiFKs, snowflake.ERForeignKey{
						FromSchema: at.Schema,
						FromTable:  at.Name,
						FromCol:    c.Name,
						ToSchema:   parts[0],
						ToTable:    parts[1],
						ToCol:      parts[2],
					})
				}
			}
		}
		aiERTables = append(aiERTables, snowflake.ERTable{
			Schema:  at.Schema,
			Name:    at.Name,
			Columns: cols,
		})
	}

	// Build merged tables: keep untouched live tables, then append AI tables.
	var mergedTables []snowflake.ERTable
	for _, t := range live.Tables {
		key := strings.ToUpper(t.Schema) + "." + strings.ToUpper(t.Name)
		if !replaced[key] {
			mergedTables = append(mergedTables, t)
		}
	}
	mergedTables = append(mergedTables, aiERTables...)

	// Build merged FKs: keep FKs from non-replaced live tables, add AI FKs.
	// Note: FKs pointing TO replaced tables are intentionally preserved — if the
	// AI's replacement removes the referenced column, the FK becomes dangling but
	// the user can fix it in the designer (the "AI scaffolds, human refines" intent).
	var mergedFKs []snowflake.ERForeignKey
	for _, fk := range live.FKs {
		key := strings.ToUpper(fk.FromSchema) + "." + strings.ToUpper(fk.FromTable)
		if !replaced[key] {
			mergedFKs = append(mergedFKs, fk)
		}
	}
	mergedFKs = append(mergedFKs, aiFKs...)

	return snowflake.ERDiagramData{
		Database: live.Database,
		Tables:   mergedTables,
		FKs:      mergedFKs,
	}
}

// registerERDesignerTools wires the ER designer delivery tool onto srv. If emit
// is nil (e.g. in tests without a Wails runtime), no tools are registered.
func registerERDesignerTools(srv *mcpsdk.Server, client *snowflake.Client, emit func(string, interface{})) {
	if emit == nil {
		return
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "open_er_designer",
		Description: "Open the ER Designer in Thaw pre-populated with an AI-generated data model. " +
			"Fetches the live schema from Snowflake and merges any AI-provided tables onto the canvas. " +
			"The user can then visually refine the model and review the diff SQL before applying. " +
			"Omit the tables parameter to open the designer with the live schema only.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in openERDesignerInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no active Snowflake connection")
		}

		// Validate AI table inputs before fetching live data.
		for _, t := range in.Tables {
			if strings.TrimSpace(t.Schema) == "" || strings.TrimSpace(t.Name) == "" {
				return nil, nil, fmt.Errorf("each table must have a non-empty schema and name")
			}
		}

		// Fetch live ER data.
		live, err := client.GetERDiagramData(ctx, in.Database)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch ER data: %w", err)
		}

		// Merge AI tables if provided.
		merged := live
		if len(in.Tables) > 0 {
			merged = mergeAITables(live, in.Tables)
		}

		// Emit the event to the frontend with panic recovery.
		var emitFailed bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("mcp open_er_designer: emit panicked", "err", r)
					emitFailed = true
				}
			}()
			emit("mcp:open-er-designer", OpenERDesignerPayload{
				Database: in.Database,
				Merged:   merged,
				Baseline: live,
			})
		}()
		if emitFailed {
			return textResult("Failed to open ER designer: internal error"), nil, nil
		}

		mergedCount := len(merged.Tables)
		aiCount := len(in.Tables)
		if aiCount > 0 {
			return textResult(fmt.Sprintf(
				"ER designer opened with %d table(s) (%d from AI). "+
					"The user can now review and refine the model visually.",
				mergedCount, aiCount,
			)), nil, nil
		}
		return textResult(fmt.Sprintf(
			"ER designer opened with %d live table(s). The user can now edit the model visually.",
			mergedCount,
		)), nil, nil
	})
}

// modifyERDesignerInput is the tool input for modify_er_designer.
type modifyERDesignerInput struct {
	Tables []erDesignerTableIn `json:"tables" jsonschema:"the tables to merge into the open ER designer; matched by SCHEMA.NAME"`
}

// ModifyERDesignerPayload is the Wails event payload for "mcp:modify-er-designer".
// The frontend listens for this event and merges the AI tables into the canvas.
type ModifyERDesignerPayload struct {
	Tables []erDesignerTableIn `json:"tables"`
}

// registerERDesignerStateTools wires the ER designer state inspection and
// modification tools onto srv. Called from session.start() after buildServer()
// to avoid changing the buildServer signature (and touching ~60 test call sites).
//
// erState must be non-nil for get_er_designer_state to be registered.
// Both emit and erState must be non-nil for modify_er_designer to be registered.
func registerERDesignerStateTools(srv *mcpsdk.Server, emit func(string, interface{}), erState *ERDesignerStateStore) {
	if erState != nil {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "get_er_designer_state",
			Description: "Get the current state of the ER Designer if it is open. " +
				"Returns the database name and all tables with their columns, primary keys, nullability, and FK references. " +
				"Returns a message if the designer is not currently open.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct{}) (*mcpsdk.CallToolResult, any, error) {
			state := erState.Get()
			if state == nil {
				return textResult("The ER designer is not currently open."), nil, nil
			}
			return jsonResult(state), nil, nil
		})
	}

	if emit != nil && erState != nil {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "modify_er_designer",
			Description: "Push table modifications into the currently open ER Designer. " +
				"Tables are matched by uppercase SCHEMA.NAME: matching tables are replaced, new tables are appended. " +
				"The designer must be open (use open_er_designer first). " +
				"Columns need a name and dataType; isPK, notNull, and fkRef are optional. " +
				"Returns a summary of exactly what changed (tables/columns added, removed, or modified) so you can " +
				"self-correct without re-reading the whole model. The user can undo any change in the designer.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in modifyERDesignerInput) (*mcpsdk.CallToolResult, any, error) {
			if len(in.Tables) == 0 {
				return nil, nil, fmt.Errorf("tables must contain at least one table")
			}
			for _, t := range in.Tables {
				if strings.TrimSpace(t.Schema) == "" || strings.TrimSpace(t.Name) == "" {
					return nil, nil, fmt.Errorf("each table must have a non-empty schema and name")
				}
				if len(t.Columns) == 0 {
					return nil, nil, fmt.Errorf("table %s.%s must have at least one column", t.Schema, t.Name)
				}
				for _, c := range t.Columns {
					if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.DataType) == "" {
						return nil, nil, fmt.Errorf("each column in %s.%s must have a non-empty name and dataType", t.Schema, t.Name)
					}
					// Simple 3-part split check. Quoted identifiers containing
					// dots (e.g. PUBLIC."MY.TABLE".COL) are not supported —
					// the designer uses unquoted uppercase identifiers.
					if c.FKRef != "" && len(strings.Split(c.FKRef, ".")) != 3 {
						return nil, nil, fmt.Errorf("fkRef %q in %s.%s.%s must be in SCHEMA.TABLE.COLUMN format", c.FKRef, t.Schema, t.Name, c.Name)
					}
				}
			}
			// Note: a TOCTOU window exists between IsOpen() and emit() — the
			// designer could close in between. This is accepted: the frontend
			// listener is torn down on unmount so a stale event is harmless.
			// Snapshot the pre-change state up front so the delta is computed from
			// the merge we perform here — not from a follow-up get_er_designer_state
			// read, which would race the frontend's 300ms debounced push-back.
			before := erState.Get()
			if before == nil {
				return textResult("The ER designer is not currently open. Use open_er_designer to open it first."), nil, nil
			}

			var emitFailed bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.L.Error("mcp modify_er_designer: emit panicked", "err", r)
						emitFailed = true
					}
				}()
				// Type conversion — works because ModifyERDesignerPayload and
				// modifyERDesignerInput have identical fields. If either type
				// gains a field, the compiler will catch the mismatch.
				emit("mcp:modify-er-designer", ModifyERDesignerPayload(in))
			}()
			if emitFailed {
				return textResult("Failed to modify ER designer: internal error"), nil, nil
			}

			// Keep the state store immediately consistent: apply the same merge
			// the frontend will apply and cache it now, so a get_er_designer_state
			// call inside the frontend's 300ms debounce window reflects the change
			// instead of returning pre-modification data. The frontend's later
			// (authoritative) push overwrites this with an equivalent result.
			merged := mergeAITablesIntoState(before, in.Tables)
			erState.Set(merged)

			delta := describeERDelta(before, in.Tables)
			return textResult(fmt.Sprintf(
				"Applied %d table change(s) to the ER designer. The user can review, undo, or ask for further edits.\n\n%s",
				len(in.Tables), delta,
			)), nil, nil
		})
	}
}

// erStateKey is the case-insensitive "SCHEMA.NAME" match key used by the ER
// designer merge, mirroring the frontend's baselineTableKey.
func erStateKey(schema, name string) string {
	return strings.ToUpper(strings.TrimSpace(schema)) + "." + strings.ToUpper(strings.TrimSpace(name))
}

// aiColsToStateCols converts AI column inputs to the MCP-facing designer column
// view, deriving NotNull from IsPK and falling back to a same-named column's
// existing DEFAULT when the AI omits one (mirrors mergeAITablesIntoDesigner).
func aiColsToStateCols(at erDesignerTableIn, before *ERDesignerTableOut) []ERDesignerColumnOut {
	prevDefault := map[string]string{}
	if before != nil {
		for _, c := range before.Columns {
			prevDefault[strings.ToUpper(strings.TrimSpace(c.Name))] = c.Default
		}
	}
	cols := make([]ERDesignerColumnOut, 0, len(at.Columns))
	for _, c := range at.Columns {
		def := c.Default
		if def == "" {
			def = prevDefault[strings.ToUpper(strings.TrimSpace(c.Name))]
		}
		cols = append(cols, ERDesignerColumnOut{
			Name:     c.Name,
			DataType: c.DataType,
			IsPK:     c.IsPK,
			NotNull:  c.NotNull || c.IsPK,
			FKRef:    c.FKRef,
			Default:  def,
		})
	}
	return cols
}

// mergeAITablesIntoState merges AI tables into the cached designer state,
// matching by SCHEMA.NAME: matching tables are replaced in place (preserving
// their position), new tables are appended. This mirrors the frontend's
// mergeAITablesIntoDesigner so the backend cache can be kept consistent
// immediately after modify_er_designer.
func mergeAITablesIntoState(before *ERDesignerState, aiTables []erDesignerTableIn) *ERDesignerState {
	out := &ERDesignerState{Database: "", Tables: nil}
	if before != nil {
		out.Database = before.Database
		out.Tables = make([]ERDesignerTableOut, len(before.Tables))
		copy(out.Tables, before.Tables)
	}

	idx := make(map[string]int, len(out.Tables))
	for i, t := range out.Tables {
		idx[erStateKey(t.Schema, t.Name)] = i
	}

	for _, at := range aiTables {
		key := erStateKey(at.Schema, at.Name)
		if i, ok := idx[key]; ok {
			out.Tables[i] = ERDesignerTableOut{
				Schema:  at.Schema,
				Name:    at.Name,
				Columns: aiColsToStateCols(at, &before.Tables[i]),
			}
		} else {
			out.Tables = append(out.Tables, ERDesignerTableOut{
				Schema:  at.Schema,
				Name:    at.Name,
				Columns: aiColsToStateCols(at, nil),
			})
			idx[key] = len(out.Tables) - 1
		}
	}
	return out
}

// describeColShort renders a one-line description of an AI column for the delta
// summary, e.g. `EMAIL VARCHAR(256) [NOT NULL]` or `USER_ID NUMBER [FK→PUBLIC.USERS.ID]`.
func describeColShort(c erDesignerColumnIn) string {
	s := c.Name + " " + c.DataType
	var tags []string
	if c.IsPK {
		tags = append(tags, "PK")
	}
	if c.NotNull && !c.IsPK {
		tags = append(tags, "NOT NULL")
	}
	if c.FKRef != "" {
		tags = append(tags, "FK→"+c.FKRef)
	}
	if len(tags) > 0 {
		s += " [" + strings.Join(tags, ", ") + "]"
	}
	return s
}

// describeERDelta compares the incoming AI tables against the cached (pre-change)
// state and produces a concise, LLM-legible summary of what changed. Since
// modify_er_designer only replaces or appends tables (never removes them), the
// delta covers added tables and per-table column additions/removals/modifications.
func describeERDelta(before *ERDesignerState, aiTables []erDesignerTableIn) string {
	beforeByKey := map[string]*ERDesignerTableOut{}
	if before != nil {
		for i := range before.Tables {
			t := &before.Tables[i]
			beforeByKey[erStateKey(t.Schema, t.Name)] = t
		}
	}

	up := func(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

	var lines []string
	for _, at := range aiTables {
		label := at.Schema + "." + at.Name
		bt := beforeByKey[erStateKey(at.Schema, at.Name)]
		if bt == nil {
			descs := make([]string, 0, len(at.Columns))
			for _, c := range at.Columns {
				descs = append(descs, describeColShort(c))
			}
			lines = append(lines, fmt.Sprintf("Added table %s (%d column(s)): %s", label, len(at.Columns), strings.Join(descs, ", ")))
			continue
		}

		beforeCols := map[string]ERDesignerColumnOut{}
		for _, c := range bt.Columns {
			beforeCols[up(c.Name)] = c
		}
		incoming := map[string]bool{}
		var added, removed, changed []string
		for _, c := range at.Columns {
			incoming[up(c.Name)] = true
			bc, ok := beforeCols[up(c.Name)]
			if !ok {
				added = append(added, describeColShort(c))
				continue
			}
			var diffs []string
			if up(bc.DataType) != up(c.DataType) {
				diffs = append(diffs, fmt.Sprintf("type %s→%s", bc.DataType, c.DataType))
			}
			if bc.IsPK != c.IsPK {
				diffs = append(diffs, fmt.Sprintf("PK %t→%t", bc.IsPK, c.IsPK))
			}
			bNN := bc.NotNull || bc.IsPK
			cNN := c.NotNull || c.IsPK
			if bNN != cNN {
				diffs = append(diffs, fmt.Sprintf("NOT NULL %t→%t", bNN, cNN))
			}
			if up(bc.FKRef) != up(c.FKRef) {
				diffs = append(diffs, fmt.Sprintf("FK %q→%q", bc.FKRef, c.FKRef))
			}
			if len(diffs) > 0 {
				changed = append(changed, fmt.Sprintf("%s (%s)", c.Name, strings.Join(diffs, ", ")))
			}
		}
		for _, c := range bt.Columns {
			if !incoming[up(c.Name)] {
				removed = append(removed, c.Name)
			}
		}

		var parts []string
		if len(added) > 0 {
			parts = append(parts, "added "+strings.Join(added, ", "))
		}
		if len(removed) > 0 {
			parts = append(parts, "removed "+strings.Join(removed, ", "))
		}
		if len(changed) > 0 {
			parts = append(parts, "changed "+strings.Join(changed, ", "))
		}
		if len(parts) == 0 {
			lines = append(lines, fmt.Sprintf("Replaced table %s (no column changes)", label))
		} else {
			lines = append(lines, fmt.Sprintf("Updated table %s: %s", label, strings.Join(parts, "; ")))
		}
	}

	if len(lines) == 0 {
		return "No changes."
	}
	return strings.Join(lines, "\n")
}
