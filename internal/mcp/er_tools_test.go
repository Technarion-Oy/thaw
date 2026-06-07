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
	"net/http"
	"net/http/httptest"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// newTestSessionWithEmit creates a test MCP server+client session with a
// non-nil emit function so emit-gated tools (like open_er_designer) are
// registered. The client is nil so handler calls that need Snowflake will error.
func newTestSessionWithEmit(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	handler := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	ctx := context.Background()
	c := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := c.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestMergeAITablesEmpty verifies that merging with no AI tables returns the
// live data unchanged.
func TestMergeAITablesEmpty(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "USERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
		},
		FKs: []snowflake.ERForeignKey{},
	}

	merged := mergeAITables(live, nil)
	if len(merged.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(merged.Tables))
	}
	if merged.Tables[0].Name != "USERS" {
		t.Errorf("expected table USERS, got %s", merged.Tables[0].Name)
	}
}

// TestMergeAITablesNewTable verifies that a new AI table is appended.
func TestMergeAITablesNewTable(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "USERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
		},
	}

	aiTables := []erDesignerTableIn{
		{
			Schema: "PUBLIC",
			Name:   "ORDERS",
			Columns: []erDesignerColumnIn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
				{Name: "USER_ID", DataType: "NUMBER(38,0)", FKRef: "PUBLIC.USERS.ID"},
			},
		},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(merged.Tables))
	}
	if merged.Tables[0].Name != "USERS" {
		t.Errorf("first table should be USERS, got %s", merged.Tables[0].Name)
	}
	if merged.Tables[1].Name != "ORDERS" {
		t.Errorf("second table should be ORDERS, got %s", merged.Tables[1].Name)
	}
	if len(merged.FKs) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(merged.FKs))
	}
	fk := merged.FKs[0]
	if fk.FromTable != "ORDERS" || fk.FromCol != "USER_ID" || fk.ToTable != "USERS" || fk.ToCol != "ID" {
		t.Errorf("unexpected FK: %+v", fk)
	}
}

// TestMergeAITablesReplacement verifies that an AI table with the same
// SCHEMA.NAME as a live table replaces it.
func TestMergeAITablesReplacement(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "USERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
				{Name: "NAME", DataType: "VARCHAR(100)", Nullable: "YES"},
			}},
		},
		FKs: []snowflake.ERForeignKey{
			{FromSchema: "PUBLIC", FromTable: "ORDERS", FromCol: "USER_ID",
				ToSchema: "PUBLIC", ToTable: "USERS", ToCol: "ID"},
		},
	}

	aiTables := []erDesignerTableIn{
		{
			Schema: "PUBLIC",
			Name:   "USERS",
			Columns: []erDesignerColumnIn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
				{Name: "EMAIL", DataType: "VARCHAR(256)", NotNull: true},
			},
		},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 1 {
		t.Fatalf("expected 1 table (replaced), got %d", len(merged.Tables))
	}
	if len(merged.Tables[0].Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(merged.Tables[0].Columns))
	}
	if merged.Tables[0].Columns[1].Name != "EMAIL" {
		t.Errorf("expected second column EMAIL, got %s", merged.Tables[0].Columns[1].Name)
	}
	// The live FK referencing the replaced table's from-side should be dropped.
	// (The FK's FromTable is ORDERS, not USERS, so it should be preserved.)
	if len(merged.FKs) != 1 {
		t.Fatalf("expected 1 FK (from ORDERS, untouched), got %d", len(merged.FKs))
	}
}

// TestMergeAITablesReplacementDropsFKs verifies that FKs originating from a
// replaced table are dropped.
func TestMergeAITablesReplacementDropsFKs(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "USERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
			{Schema: "PUBLIC", Name: "ORDERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
				{Name: "USER_ID", DataType: "NUMBER(38,0)", Nullable: "YES"},
			}},
		},
		FKs: []snowflake.ERForeignKey{
			{FromSchema: "PUBLIC", FromTable: "ORDERS", FromCol: "USER_ID",
				ToSchema: "PUBLIC", ToTable: "USERS", ToCol: "ID"},
		},
	}

	// Replace ORDERS with a new definition that has no FK.
	aiTables := []erDesignerTableIn{
		{
			Schema: "PUBLIC",
			Name:   "ORDERS",
			Columns: []erDesignerColumnIn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
				{Name: "TOTAL", DataType: "NUMBER(12,2)"},
			},
		},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(merged.Tables))
	}
	// The FK from the old ORDERS table should be dropped since ORDERS was replaced.
	if len(merged.FKs) != 0 {
		t.Errorf("expected 0 FKs (old FK from replaced ORDERS dropped), got %d: %+v", len(merged.FKs), merged.FKs)
	}
}

// TestMergeAITablesMixed verifies a mix of new and replacement tables.
func TestMergeAITablesMixed(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "A", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
			{Schema: "PUBLIC", Name: "B", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
		},
	}

	aiTables := []erDesignerTableIn{
		{Schema: "PUBLIC", Name: "B", Columns: []erDesignerColumnIn{
			{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
			{Name: "NEW_COL", DataType: "VARCHAR(100)"},
		}},
		{Schema: "PUBLIC", Name: "C", Columns: []erDesignerColumnIn{
			{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
		}},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 3 {
		t.Fatalf("expected 3 tables (A kept, B replaced, C new), got %d", len(merged.Tables))
	}
	// Order: A (untouched live), then B (AI replacement), then C (AI new).
	names := make([]string, len(merged.Tables))
	for i, tbl := range merged.Tables {
		names[i] = tbl.Name
	}
	expected := []string{"A", "B", "C"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("table[%d] = %q, want %q (all: %v)", i, names[i], want, names)
		}
	}
}

// TestMergeAITablesCaseInsensitive verifies that matching is case-insensitive.
func TestMergeAITablesCaseInsensitive(t *testing.T) {
	live := snowflake.ERDiagramData{
		Database: "DB",
		Tables: []snowflake.ERTable{
			{Schema: "PUBLIC", Name: "USERS", Columns: []snowflake.ERColumn{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, Nullable: "NO"},
			}},
		},
	}

	aiTables := []erDesignerTableIn{
		{Schema: "public", Name: "users", Columns: []erDesignerColumnIn{
			{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
			{Name: "EMAIL", DataType: "VARCHAR(256)"},
		}},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 1 {
		t.Fatalf("expected 1 table (replaced case-insensitively), got %d", len(merged.Tables))
	}
	if len(merged.Tables[0].Columns) != 2 {
		t.Errorf("expected 2 columns in replaced table, got %d", len(merged.Tables[0].Columns))
	}
}

// TestMergeAITablesNullability verifies that NotNull and IsPK correctly set
// the Nullable field on merged columns.
func TestMergeAITablesNullability(t *testing.T) {
	live := snowflake.ERDiagramData{Database: "DB"}

	aiTables := []erDesignerTableIn{
		{Schema: "PUBLIC", Name: "T", Columns: []erDesignerColumnIn{
			{Name: "PK_COL", DataType: "NUMBER(38,0)", IsPK: true},
			{Name: "NN_COL", DataType: "VARCHAR(100)", NotNull: true},
			{Name: "NULL_COL", DataType: "VARCHAR(100)"},
		}},
	}

	merged := mergeAITables(live, aiTables)
	if len(merged.Tables) != 1 || len(merged.Tables[0].Columns) != 3 {
		t.Fatalf("unexpected table/column count")
	}
	cols := merged.Tables[0].Columns
	if cols[0].Nullable != "NO" {
		t.Errorf("PK column should be NOT NULL, got Nullable=%q", cols[0].Nullable)
	}
	if cols[1].Nullable != "NO" {
		t.Errorf("NotNull column should be NOT NULL, got Nullable=%q", cols[1].Nullable)
	}
	if cols[2].Nullable != "YES" {
		t.Errorf("nullable column should be YES, got Nullable=%q", cols[2].Nullable)
	}
}

// TestOpenERDesignerNilClient verifies the tool returns an error when no
// Snowflake client is available.
func TestOpenERDesignerNilClient(t *testing.T) {
	cs := newTestSessionWithEmit(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "open_er_designer",
		Arguments: openERDesignerInput{Database: "DB"},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
}

// TestOpenERDesignerEmptyDatabase verifies the tool returns an error when
// database is empty.
func TestOpenERDesignerEmptyDatabase(t *testing.T) {
	cs := newTestSessionWithEmit(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "open_er_designer",
		Arguments: openERDesignerInput{Database: ""},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
}

// TestOpenERDesignerEmptyTableSchemaName verifies validation rejects tables
// with empty schema or name.
func TestOpenERDesignerEmptyTableSchemaName(t *testing.T) {
	cs := newTestSessionWithEmit(t)
	ctx := context.Background()

	// Empty schema.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_er_designer",
		Arguments: openERDesignerInput{
			Database: "DB",
			Tables: []erDesignerTableIn{
				{Schema: "", Name: "T", Columns: []erDesignerColumnIn{{Name: "ID", DataType: "INT"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty schema")
	}

	// Empty name.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "open_er_designer",
		Arguments: openERDesignerInput{
			Database: "DB",
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "", Columns: []erDesignerColumnIn{{Name: "ID", DataType: "INT"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty name")
	}
}

// TestOpenERDesignerNotRegisteredWithNilEmit verifies that open_er_designer is
// not registered when emit is nil.
func TestOpenERDesignerNotRegisteredWithNilEmit(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	names := toolNames(t, srv)
	if hasToolName(names, "open_er_designer") {
		t.Error("open_er_designer should not be registered when emit is nil")
	}
}

// TestOpenERDesignerRegisteredWithEmit verifies that open_er_designer is
// registered when a non-nil emit function is provided.
func TestOpenERDesignerRegisteredWithEmit(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	names := toolNames(t, srv)
	if !hasToolName(names, "open_er_designer") {
		t.Errorf("open_er_designer should be registered when emit is non-nil (got: %v)", names)
	}
}
