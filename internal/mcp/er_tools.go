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
