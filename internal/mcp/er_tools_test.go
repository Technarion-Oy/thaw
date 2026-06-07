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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// newTestSessionWithEmit creates a test MCP server+client session with a
// non-nil emit function so emit-gated tools (like open_er_designer) are
// registered. The client is nil so handler calls that need Snowflake will error.
func newTestSessionWithEmit(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	return newTestSessionWithEmitAndState(t, NewERDesignerStateStore())
}

// newTestSessionWithEmitAndState creates a test MCP server+client session with
// a non-nil emit function and the provided ER designer state store. Pass nil
// erState to test the nil-store gating.
func newTestSessionWithEmitAndState(t *testing.T, erState *ERDesignerStateStore) *mcpsdk.ClientSession {
	t.Helper()
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, erState)
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

// ── ERDesignerStateStore unit tests ─────────────────────────────────────────

func TestERDesignerStateStoreSetGet(t *testing.T) {
	store := NewERDesignerStateStore()
	if store.IsOpen() {
		t.Error("new store should not be open")
	}
	if store.Get() != nil {
		t.Error("new store Get() should return nil")
	}

	state := &ERDesignerState{
		Database: "DB1",
		Tables: []ERDesignerTableOut{
			{Schema: "PUBLIC", Name: "USERS", Columns: []ERDesignerColumnOut{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, NotNull: true},
			}},
		},
	}
	store.Set(state)
	if !store.IsOpen() {
		t.Error("store should be open after Set")
	}
	got := store.Get()
	if got == nil {
		t.Fatal("Get() should return non-nil after Set")
	}
	if got.Database != "DB1" {
		t.Errorf("Database = %q, want DB1", got.Database)
	}
	if len(got.Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(got.Tables))
	}
}

func TestERDesignerStateStoreClear(t *testing.T) {
	store := NewERDesignerStateStore()
	store.Set(&ERDesignerState{Database: "DB"})
	if !store.IsOpen() {
		t.Fatal("store should be open after Set")
	}
	store.Clear()
	if store.IsOpen() {
		t.Error("store should not be open after Clear")
	}
	if store.Get() != nil {
		t.Error("Get() should return nil after Clear")
	}
}

func TestERDesignerStateStoreConcurrent(t *testing.T) {
	store := NewERDesignerStateStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Set(&ERDesignerState{Database: "DB"})
			_ = store.Get()
			_ = store.IsOpen()
			store.Clear()
		}()
	}
	wg.Wait()
}

// ── get_er_designer_state registration tests ─────────────────────────────────

func TestGetERDesignerStateRegisteredWithStore(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, NewERDesignerStateStore())
	names := toolNames(t, srv)
	if !hasToolName(names, "get_er_designer_state") {
		t.Errorf("get_er_designer_state should be registered with non-nil store (got: %v)", names)
	}
}

func TestGetERDesignerStateNotRegisteredWithNilStore(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, nil)
	names := toolNames(t, srv)
	if hasToolName(names, "get_er_designer_state") {
		t.Error("get_er_designer_state should not be registered with nil store")
	}
}

// ── modify_er_designer registration tests ────────────────────────────────────

func TestModifyERDesignerRegisteredWithEmitAndStore(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, NewERDesignerStateStore())
	names := toolNames(t, srv)
	if !hasToolName(names, "modify_er_designer") {
		t.Errorf("modify_er_designer should be registered with emit and store (got: %v)", names)
	}
}

func TestModifyERDesignerNotRegisteredWithNilEmit(t *testing.T) {
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil, nil)
	registerERDesignerStateTools(srv, nil, NewERDesignerStateStore())
	names := toolNames(t, srv)
	if hasToolName(names, "modify_er_designer") {
		t.Error("modify_er_designer should not be registered with nil emit")
	}
}

func TestModifyERDesignerNotRegisteredWithNilStore(t *testing.T) {
	emit := func(string, interface{}) {}
	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, nil)
	names := toolNames(t, srv)
	if hasToolName(names, "modify_er_designer") {
		t.Error("modify_er_designer should not be registered with nil store")
	}
}

// ── get_er_designer_state tool behavior ──────────────────────────────────────

func TestGetERDesignerStateDesignerClosed(t *testing.T) {
	cs := newTestSessionWithEmit(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_er_designer_state",
		Arguments: struct{}{},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if res.IsError {
		t.Error("expected no error for designer-closed case")
	}
	text := res.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "not currently open") {
		t.Errorf("expected 'not currently open' message, got: %s", text)
	}
}

func TestGetERDesignerStateDesignerOpen(t *testing.T) {
	erState := NewERDesignerStateStore()
	erState.Set(&ERDesignerState{
		Database: "TESTDB",
		Tables: []ERDesignerTableOut{
			{Schema: "PUBLIC", Name: "USERS", Columns: []ERDesignerColumnOut{
				{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true, NotNull: true},
				{Name: "EMAIL", DataType: "VARCHAR(256)", NotNull: true},
			}},
		},
	})

	cs := newTestSessionWithEmitAndState(t, erState)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_er_designer_state",
		Arguments: struct{}{},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if res.IsError {
		t.Error("expected no error for designer-open case")
	}
	text := res.Content[0].(*mcpsdk.TextContent).Text
	var state ERDesignerState
	if err := json.Unmarshal([]byte(text), &state); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if state.Database != "TESTDB" {
		t.Errorf("Database = %q, want TESTDB", state.Database)
	}
	if len(state.Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(state.Tables))
	}
	if state.Tables[0].Name != "USERS" {
		t.Errorf("Tables[0].Name = %q, want USERS", state.Tables[0].Name)
	}
	if len(state.Tables[0].Columns) != 2 {
		t.Errorf("Columns len = %d, want 2", len(state.Tables[0].Columns))
	}
}

// ── modify_er_designer tool behavior ─────────────────────────────────────────

func TestModifyERDesignerEmptyTables(t *testing.T) {
	erState := NewERDesignerStateStore()
	erState.Set(&ERDesignerState{Database: "DB"})
	cs := newTestSessionWithEmitAndState(t, erState)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "modify_er_designer",
		Arguments: modifyERDesignerInput{Tables: nil},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty tables")
	}
}

func TestModifyERDesignerDesignerClosed(t *testing.T) {
	cs := newTestSessionWithEmit(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "T", Columns: []erDesignerColumnIn{{Name: "ID", DataType: "INT"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if res.IsError {
		t.Error("expected no MCP error for designer-closed (graceful message)")
	}
	text := res.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "not currently open") {
		t.Errorf("expected 'not currently open' message, got: %s", text)
	}
}

func TestModifyERDesignerEmitsEvent(t *testing.T) {
	var emitted bool
	var emittedName string
	var emittedPayload interface{}
	emit := func(name string, data interface{}) {
		emitted = true
		emittedName = name
		emittedPayload = data
	}

	erState := NewERDesignerStateStore()
	erState.Set(&ERDesignerState{Database: "DB"})

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, emit, nil, nil)
	registerERDesignerStateTools(srv, emit, erState)
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

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "ORDERS", Columns: []erDesignerColumnIn{
					{Name: "ID", DataType: "NUMBER(38,0)", IsPK: true},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if res.IsError {
		t.Error("expected no error for valid modify call")
	}
	if !emitted {
		t.Fatal("expected emit to be called")
	}
	if emittedName != "mcp:modify-er-designer" {
		t.Errorf("emitted event name = %q, want mcp:modify-er-designer", emittedName)
	}
	payload, ok := emittedPayload.(ModifyERDesignerPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ModifyERDesignerPayload", emittedPayload)
	}
	if len(payload.Tables) != 1 {
		t.Fatalf("payload Tables len = %d, want 1", len(payload.Tables))
	}
	if payload.Tables[0].Name != "ORDERS" {
		t.Errorf("payload Tables[0].Name = %q, want ORDERS", payload.Tables[0].Name)
	}
}

func TestModifyERDesignerEmptySchemaName(t *testing.T) {
	erState := NewERDesignerStateStore()
	erState.Set(&ERDesignerState{Database: "DB"})
	cs := newTestSessionWithEmitAndState(t, erState)
	ctx := context.Background()

	// Empty schema.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
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
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
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

func TestModifyERDesignerColumnValidation(t *testing.T) {
	erState := NewERDesignerStateStore()
	erState.Set(&ERDesignerState{Database: "DB"})
	cs := newTestSessionWithEmitAndState(t, erState)
	ctx := context.Background()

	// Zero columns.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "T", Columns: nil},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for zero columns")
	}

	// Empty column name.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "T", Columns: []erDesignerColumnIn{{Name: "", DataType: "INT"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty column name")
	}

	// Empty column dataType.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "modify_er_designer",
		Arguments: modifyERDesignerInput{
			Tables: []erDesignerTableIn{
				{Schema: "PUBLIC", Name: "T", Columns: []erDesignerColumnIn{{Name: "ID", DataType: ""}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned Go error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty column dataType")
	}
}
