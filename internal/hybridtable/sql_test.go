// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package hybridtable

import (
	"strings"
	"testing"
)

func TestBuildCreateHybridTableSql_Basic(t *testing.T) {
	cfg := HybridTableConfig{
		Name: "USERS",
		Columns: []HybridColumn{
			{Name: "ID", Type: "NUMBER", NotNull: true, PrimaryKey: true},
			{Name: "EMAIL", Type: "VARCHAR", NotNull: true},
			{Name: "CREATED_AT", Type: "TIMESTAMP_NTZ", Default: "CURRENT_TIMESTAMP()"},
		},
		Comment: "app users",
	}
	sql, err := BuildCreateHybridTableSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		`CREATE HYBRID TABLE "DB"."SC".USERS (`,
		"ID NUMBER NOT NULL",
		"EMAIL VARCHAR NOT NULL",
		"CREATED_AT TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP()",
		"PRIMARY KEY (ID)",
		"COMMENT = 'app users'",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q\ngot:\n%s", want, sql)
		}
	}
	if !strings.HasSuffix(sql, ";") {
		t.Errorf("expected trailing semicolon, got:\n%s", sql)
	}
}

func TestBuildCreateHybridTableSql_CompositePKAndIndex(t *testing.T) {
	cfg := HybridTableConfig{
		Name:        "ORDERS",
		IfNotExists: true,
		Columns: []HybridColumn{
			{Name: "ORG_ID", Type: "NUMBER", PrimaryKey: true},
			{Name: "ORDER_ID", Type: "NUMBER", PrimaryKey: true},
			{Name: "STATUS", Type: "VARCHAR"},
			{Name: "CUSTOMER_ID", Type: "NUMBER"},
		},
		Indexes: []HybridIndex{
			{Name: "IDX_STATUS", Columns: []string{"STATUS"}, Include: []string{"CUSTOMER_ID"}},
			{Name: "", Columns: []string{"STATUS"}}, // skipped: no name
		},
	}
	sql, err := BuildCreateHybridTableSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sql, "OR REPLACE") {
		t.Errorf("hybrid tables do not support OR REPLACE; got:\n%s", sql)
	}
	for _, want := range []string{
		`CREATE HYBRID TABLE IF NOT EXISTS "DB"."SC".ORDERS (`,
		"PRIMARY KEY (ORG_ID, ORDER_ID)",
		"INDEX IDX_STATUS (STATUS) INCLUDE (CUSTOMER_ID)",
		// PK columns are forced NOT NULL even without the flag.
		"ORG_ID NUMBER NOT NULL",
		"ORDER_ID NUMBER NOT NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q\ngot:\n%s", want, sql)
		}
	}
}

func TestBuildCreateHybridTableSql_PlaceholderPK(t *testing.T) {
	// No column flagged PrimaryKey → the builder must still emit a PRIMARY KEY
	// placeholder, since a hybrid table cannot exist without one.
	cfg := HybridTableConfig{
		Name:    "T",
		Columns: []HybridColumn{{Name: "C", Type: "VARCHAR"}},
	}
	sql, err := BuildCreateHybridTableSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "PRIMARY KEY (<column>)") {
		t.Errorf("expected placeholder PRIMARY KEY, got:\n%s", sql)
	}
}

func TestBuildCreateHybridTableSql_CaseSensitive(t *testing.T) {
	cfg := HybridTableConfig{
		Name:          "MixedCase",
		CaseSensitive: true,
		Columns:       []HybridColumn{{Name: "Id", Type: "NUMBER", PrimaryKey: true}},
	}
	sql, err := BuildCreateHybridTableSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, `"MixedCase"`) || !strings.Contains(sql, `"Id"`) {
		t.Errorf("expected quoted case-sensitive identifiers, got:\n%s", sql)
	}
	if !strings.Contains(sql, `PRIMARY KEY ("Id")`) {
		t.Errorf("expected quoted PK column, got:\n%s", sql)
	}
}

func TestBuildCreateIndexSql(t *testing.T) {
	sql, err := BuildCreateIndexSql("DB", "SC", "ORDERS", HybridIndex{
		Name:    "IDX_CUST",
		Columns: []string{"CUSTOMER_ID", "STATUS"},
		Include: []string{"TOTAL"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `CREATE INDEX "IDX_CUST" ON "DB"."SC"."ORDERS" (CUSTOMER_ID, STATUS) INCLUDE (TOTAL);`
	if sql != want {
		t.Errorf("got:\n%s\nwant:\n%s", sql, want)
	}
}

func TestBuildCreateIndexSql_Errors(t *testing.T) {
	if _, err := BuildCreateIndexSql("DB", "SC", "T", HybridIndex{Name: "", Columns: []string{"C"}}); err == nil {
		t.Error("expected error for empty index name")
	}
	if _, err := BuildCreateIndexSql("DB", "SC", "T", HybridIndex{Name: "I", Columns: nil}); err == nil {
		t.Error("expected error for empty column list")
	}
}

func TestBuildDropIndexSql(t *testing.T) {
	sql, err := BuildDropIndexSql("DB", "SC", "ORDERS", "IDX_CUST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `DROP INDEX IF EXISTS "DB"."SC"."ORDERS"."IDX_CUST";`
	if sql != want {
		t.Errorf("got:\n%s\nwant:\n%s", sql, want)
	}
	if _, err := BuildDropIndexSql("DB", "SC", "ORDERS", ""); err == nil {
		t.Error("expected error for empty index name")
	}
}
