// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package erdesigner

import (
	"testing"

	"thaw/internal/snowflake"
)

func fk(fromSchema, fromTable, fromCol, toSchema, toTable, toCol string) snowflake.ERForeignKey {
	return snowflake.ERForeignKey{
		FromSchema: fromSchema, FromTable: fromTable, FromCol: fromCol,
		ToSchema: toSchema, ToTable: toTable, ToCol: toCol,
	}
}

func tbl(schema, name string) TableRef {
	return TableRef{Schema: schema, Name: name}
}

// ── FindJoinPaths ───────────────────────────────────────────────────────────

func TestFindJoinPaths(t *testing.T) {
	t.Run("fewer than 2 tables returns empty", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "A", "id", "S", "B", "a_id")}
		if paths := FindJoinPaths([]TableRef{tbl("S", "A")}, fks); len(paths) != 0 {
			t.Errorf("expected 0 paths, got %d", len(paths))
		}
		if paths := FindJoinPaths(nil, fks); len(paths) != 0 {
			t.Errorf("expected 0 paths for nil, got %d", len(paths))
		}
	})

	t.Run("direct path between 2 tables", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID")}
		paths := FindJoinPaths([]TableRef{tbl("S", "ORDERS"), tbl("S", "USERS")}, fks)
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		if len(paths[0].Tables) != 2 {
			t.Errorf("expected 2 tables, got %d", len(paths[0].Tables))
		}
		if len(paths[0].Edges) != 1 {
			t.Errorf("expected 1 edge, got %d", len(paths[0].Edges))
		}
		if paths[0].Edges[0].From.Table != "ORDERS" {
			t.Errorf("expected from table ORDERS, got %s", paths[0].Edges[0].From.Table)
		}
		if paths[0].Edges[0].To.Table != "USERS" {
			t.Errorf("expected to table USERS, got %s", paths[0].Edges[0].To.Table)
		}
	})

	t.Run("path through intermediate table", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{
			fk("S", "ORDER_ITEMS", "ORDER_ID", "S", "ORDERS", "ID"),
			fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID"),
		}
		paths := FindJoinPaths([]TableRef{tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")}, fks)
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		if len(paths[0].Tables) != 3 {
			t.Errorf("expected 3 tables, got %d", len(paths[0].Tables))
		}
		names := map[string]bool{}
		for _, tb := range paths[0].Tables {
			names[tb.Name] = true
		}
		for _, want := range []string{"ORDER_ITEMS", "ORDERS", "USERS"} {
			if !names[want] {
				t.Errorf("expected table %s in path", want)
			}
		}
	})

	t.Run("multiple paths for disambiguation", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{
			fk("S", "EMPLOYEES", "CREATED_BY", "S", "USERS", "ID"),
			fk("S", "EMPLOYEES", "UPDATED_BY", "S", "USERS", "ID"),
		}
		paths := FindJoinPaths([]TableRef{tbl("S", "EMPLOYEES"), tbl("S", "USERS")}, fks)
		if len(paths) < 2 {
			t.Errorf("expected at least 2 paths, got %d", len(paths))
		}
		for _, p := range paths {
			if len(p.Tables) != 2 {
				t.Errorf("expected 2 tables per path, got %d", len(p.Tables))
			}
		}
	})

	t.Run("disconnected tables returns empty", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "A", "id", "S", "B", "a_id")}
		paths := FindJoinPaths([]TableRef{tbl("S", "A"), tbl("S", "C")}, fks)
		if len(paths) != 0 {
			t.Errorf("expected 0 paths, got %d", len(paths))
		}
	})

	t.Run("3+ tables via Steiner tree", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{
			fk("S", "A", "B_ID", "S", "B", "ID"),
			fk("S", "B", "C_ID", "S", "C", "ID"),
		}
		paths := FindJoinPaths([]TableRef{tbl("S", "A"), tbl("S", "B"), tbl("S", "C")}, fks)
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		if len(paths[0].Tables) != 3 {
			t.Errorf("expected 3 tables, got %d", len(paths[0].Tables))
		}
		if len(paths[0].Edges) != 2 {
			t.Errorf("expected 2 edges, got %d", len(paths[0].Edges))
		}
	})

	t.Run("cross-schema FKs", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("SALES", "ORDERS", "PRODUCT_ID", "CATALOG", "PRODUCTS", "ID")}
		paths := FindJoinPaths([]TableRef{tbl("SALES", "ORDERS"), tbl("CATALOG", "PRODUCTS")}, fks)
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		if paths[0].Edges[0].From.Schema != "SALES" {
			t.Errorf("expected from schema SALES, got %s", paths[0].Edges[0].From.Schema)
		}
		if paths[0].Edges[0].To.Schema != "CATALOG" {
			t.Errorf("expected to schema CATALOG, got %s", paths[0].Edges[0].To.Schema)
		}
	})

	t.Run("non-existent table returns empty", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "A", "id", "S", "B", "a_id")}
		paths := FindJoinPaths([]TableRef{tbl("S", "A"), tbl("S", "NONEXISTENT")}, fks)
		if len(paths) != 0 {
			t.Errorf("expected 0 paths, got %d", len(paths))
		}
	})
}

// ── BuildJoinState ──────────────────────────────────────────────────────────

func TestBuildJoinState(t *testing.T) {
	t.Run("2-table state", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID")}
		paths := FindJoinPaths([]TableRef{tbl("S", "ORDERS"), tbl("S", "USERS")}, fks)
		state := BuildJoinState(paths[0], []TableRef{tbl("S", "ORDERS"), tbl("S", "USERS")}, "MY_DB")

		if state.Database != "MY_DB" {
			t.Errorf("expected database MY_DB, got %s", state.Database)
		}
		if state.BaseTable.Name != "ORDERS" {
			t.Errorf("expected base table ORDERS, got %s", state.BaseTable.Name)
		}
		if len(state.Joins) != 1 {
			t.Fatalf("expected 1 join, got %d", len(state.Joins))
		}
		if state.Joins[0].Table.Name != "USERS" {
			t.Errorf("expected join table USERS, got %s", state.Joins[0].Table.Name)
		}
		if state.Joins[0].JoinType != "INNER" {
			t.Errorf("expected INNER join, got %s", state.Joins[0].JoinType)
		}
		if state.Joins[0].IsIntermediate {
			t.Error("expected IsIntermediate=false for selected table")
		}
	})

	t.Run("marks intermediate tables", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{
			fk("S", "ORDER_ITEMS", "ORDER_ID", "S", "ORDERS", "ID"),
			fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID"),
		}
		paths := FindJoinPaths([]TableRef{tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")}, fks)
		state := BuildJoinState(paths[0], []TableRef{tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")}, "MY_DB")

		var ordersJoin, usersJoin *JoinEntry
		for i := range state.Joins {
			switch state.Joins[i].Table.Name {
			case "ORDERS":
				ordersJoin = &state.Joins[i]
			case "USERS":
				usersJoin = &state.Joins[i]
			}
		}
		if ordersJoin == nil || !ordersJoin.IsIntermediate {
			t.Error("expected ORDERS to be intermediate")
		}
		if usersJoin == nil || usersJoin.IsIntermediate {
			t.Error("expected USERS to not be intermediate")
		}
	})

	t.Run("empty selectedColumns", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "A", "B_ID", "S", "B", "ID")}
		paths := FindJoinPaths([]TableRef{tbl("S", "A"), tbl("S", "B")}, fks)
		state := BuildJoinState(paths[0], []TableRef{tbl("S", "A"), tbl("S", "B")}, "MY_DB")
		if len(state.SelectedColumns) != 0 {
			t.Errorf("expected empty selectedColumns, got %d entries", len(state.SelectedColumns))
		}
	})

	t.Run("populates fkPairs", func(t *testing.T) {
		fks := []snowflake.ERForeignKey{fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID")}
		paths := FindJoinPaths([]TableRef{tbl("S", "ORDERS"), tbl("S", "USERS")}, fks)
		state := BuildJoinState(paths[0], []TableRef{tbl("S", "ORDERS"), tbl("S", "USERS")}, "MY_DB")
		if len(state.Joins[0].FKPairs) != 1 {
			t.Fatalf("expected 1 fkPair, got %d", len(state.Joins[0].FKPairs))
		}
		if state.Joins[0].FKPairs[0].From.Col != "USER_ID" {
			t.Errorf("expected from col USER_ID, got %s", state.Joins[0].FKPairs[0].From.Col)
		}
		if state.Joins[0].FKPairs[0].To.Col != "ID" {
			t.Errorf("expected to col ID, got %s", state.Joins[0].FKPairs[0].To.Col)
		}
	})

	t.Run("composite FK fkPairs", func(t *testing.T) {
		path := JoinPath{
			Tables: []TableRef{tbl("S", "DETAILS"), tbl("S", "ORDERS")},
			Edges: []JoinPathEdge{
				{From: FKColRef{Schema: "S", Table: "DETAILS", Col: "ORDER_ID"}, To: FKColRef{Schema: "S", Table: "ORDERS", Col: "ID"}},
				{From: FKColRef{Schema: "S", Table: "DETAILS", Col: "REGION"}, To: FKColRef{Schema: "S", Table: "ORDERS", Col: "REGION"}},
			},
		}
		state := BuildJoinState(path, []TableRef{tbl("S", "DETAILS"), tbl("S", "ORDERS")}, "MY_DB")
		if len(state.Joins) != 1 {
			t.Fatalf("expected 1 join, got %d", len(state.Joins))
		}
		if len(state.Joins[0].FKPairs) != 2 {
			t.Fatalf("expected 2 fkPairs, got %d", len(state.Joins[0].FKPairs))
		}
	})
}
