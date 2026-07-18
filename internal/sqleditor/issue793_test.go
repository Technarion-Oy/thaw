// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"context"
	"strings"
	"testing"
)

// issue793Provider is the warm catalog from issue #793 round 2: MYDB.PUBLIC with
// ORDERS(ID, CUSTOMER_ID, AMOUNT, CREATED_AT, V) and a CUSTOMERS view, plus a
// stage MYSTG. V is a VARIANT column (for the variant-path cases).
func issue793Provider() *fakeProvider {
	return &fakeProvider{
		session:   SessionContext{Database: "MYDB", Schema: "PUBLIC"},
		databases: []string{"MYDB"},
		schemas:   map[string][]string{"MYDB": {"PUBLIC"}},
		objects: map[string][]StoreObject{
			schemaObjectKey("MYDB", "PUBLIC"): {
				{DB: "MYDB", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
				{DB: "MYDB", Schema: "PUBLIC", Name: "CUSTOMERS", Kind: "VIEW"},
				{DB: "MYDB", Schema: "PUBLIC", Name: "MYSTG", Kind: "STAGE"},
			},
		},
		columns: map[string][]ColInfo{
			"MYDB\x00PUBLIC\x00ORDERS": {
				{Name: "ID", DataType: "NUMBER(38,0)"},
				{Name: "CUSTOMER_ID", DataType: "NUMBER(38,0)"},
				{Name: "AMOUNT", DataType: "NUMBER(38,2)"},
				{Name: "CREATED_AT", DataType: "TIMESTAMP_NTZ"},
				{Name: "V", DataType: "VARIANT"},
			},
			"MYDB\x00PUBLIC\x00CUSTOMERS": {
				{Name: "ID", DataType: "NUMBER(38,0)"},
				{Name: "NAME", DataType: "TEXT"},
			},
		},
	}
}

// diagnoseColMarkers returns the "column not found / does not exist" markers a
// full Diagnose run produces for sql against the issue #793 catalog.
func diagnoseColMarkers(t *testing.T, sql string) []DiagMarker {
	t.Helper()
	markers, err := Diagnose(context.Background(), issue793Provider(), sql)
	if err != nil {
		t.Fatalf("Diagnose(%q) error: %v", sql, err)
	}
	var out []DiagMarker
	for _, m := range markers {
		msg := strings.ToLower(m.Message)
		if strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "not authorized") {
			out = append(out, m)
		}
	}
	return out
}

// assertNoColMarkers fails if Diagnose flags any column/schema/db existence issue.
func assertNoColMarkers(t *testing.T, sql string) {
	t.Helper()
	if got := diagnoseColMarkers(t, sql); len(got) > 0 {
		t.Errorf("expected no column/existence markers for %q, got %d: %q", sql, len(got), got[0].Message)
	}
}

// A1: bare (non-$$) Snowflake Scripting anonymous blocks — pasted from Snowsight —
// must not be shredded on their inner `;` and flagged. These produced red errors
// and/or "unexpected end of statement" warnings before the fix.
func TestIssue793_A1_BareScriptingBlocks(t *testing.T) {
	blocks := []string{
		"BEGIN\n  LET x := 1;\n  RETURN x;\nEND;",
		"DECLARE\n  c INT DEFAULT 0;\nBEGIN\n  FOR i IN 1 TO 5 DO\n    c := c + i;\n  END FOR;\n  RETURN c;\nEND;",
		"DECLARE\n  cur CURSOR FOR SELECT a FROM t;\nBEGIN\n  FOR rec IN cur DO\n    INSERT INTO log VALUES (rec.a);\n  END FOR;\nEND;",
		"BEGIN\n  SELECT 1/0;\nEXCEPTION\n  WHEN STATEMENT_ERROR THEN\n    RETURN SQLERRM;\nEND;",
	}
	for _, sql := range blocks {
		markers, err := Diagnose(context.Background(), nil, sql)
		if err != nil {
			t.Fatalf("Diagnose(%q) error: %v", sql, err)
		}
		if len(markers) > 0 {
			t.Errorf("bare scripting block should produce no markers, got %d for %q:\n  first: [sev %d] %q",
				len(markers), sql, markers[0].Severity, markers[0].Message)
		}
	}
}

// A1 regression: transaction control (bare BEGIN … COMMIT) must NOT be glued into
// a scripting block — it stays multiple statements and keeps its existing
// transaction-tracking behavior (no false "unexpected end of statement").
func TestIssue793_A1_TransactionNotGlued(t *testing.T) {
	sql := "BEGIN;\nINSERT INTO t VALUES (1);\nCOMMIT;"
	ranges := GetStatementRanges(sql)
	if len(ranges) != 3 {
		t.Fatalf("transaction BEGIN/INSERT/COMMIT should split into 3 ranges, got %d", len(ranges))
	}
	markers, err := Diagnose(context.Background(), nil, sql)
	if err != nil {
		t.Fatalf("Diagnose error: %v", err)
	}
	for _, m := range markers {
		if strings.Contains(m.Message, "unexpected end of statement") ||
			strings.Contains(m.Message, "Unexpected token") {
			t.Errorf("valid transaction block wrongly flagged: %q", m.Message)
		}
	}
}

// A1: a bare block followed by a normal statement keeps them as separate ranges
// (the block is one range, the SELECT another).
func TestIssue793_A1_BlockThenStatement(t *testing.T) {
	sql := "BEGIN\n  LET x := 1;\n  RETURN x;\nEND;\nSELECT 1;"
	ranges := GetStatementRanges(sql)
	if len(ranges) != 2 {
		t.Fatalf("block + SELECT should be 2 ranges, got %d", len(ranges))
	}
}

// D1: a single-segment variant path (v:field) must not be flagged as a missing
// column, matching the already-passing deeper-path form.
func TestIssue793_D1_VariantPath(t *testing.T) {
	assertNoColMarkers(t, "SELECT v:field FROM orders")
	assertNoColMarkers(t, "SELECT o.v:field::STRING FROM orders o")
	assertNoColMarkers(t, "SELECT v:field.sub[0]::STRING FROM orders") // regression guard (already passed)
}

// D2: LATERAL FLATTEN output columns and its alias must not be flagged.
func TestIssue793_D2_LateralFlatten(t *testing.T) {
	assertNoColMarkers(t, "SELECT f.value FROM orders o, LATERAL FLATTEN(input => o.v) f")
	assertNoColMarkers(t, "SELECT value FROM orders, LATERAL FLATTEN(input => v)")
}

// D4: PIVOT/UNPIVOT clause tokens (FOR, the value column) must not leak into
// column validation.
func TestIssue793_D4_PivotUnpivot(t *testing.T) {
	assertNoColMarkers(t, "SELECT * FROM orders PIVOT (SUM(amount) FOR customer_id IN (1, 2))")
	assertNoColMarkers(t, "SELECT o.amount FROM orders o UNPIVOT (val FOR key IN (id, customer_id))")
}

// D5: CONNECT BY pseudo-columns (LEVEL) and the PRIOR keyword must not be flagged
// as unknown columns.
func TestIssue793_D5_ConnectBy(t *testing.T) {
	assertNoColMarkers(t, "SELECT LEVEL, id FROM orders START WITH id = 1 CONNECT BY PRIOR id = customer_id")
}

// D6: INFORMATION_SCHEMA (2-part, session-DB-relative) and the shared SNOWFLAKE
// database must not raise a schema/database existence error.
func TestIssue793_D6_AlwaysPresentSchemas(t *testing.T) {
	assertNoColMarkers(t, "SELECT table_name FROM information_schema.tables")
	assertNoColMarkers(t, "SELECT column_name FROM mydb.information_schema.columns") // 3-part regression guard
	assertNoColMarkers(t, "SELECT query_text FROM snowflake.account_usage.query_history")
}

// A2: the AISQL AI_* family must not be flagged as an unknown Cortex function.
func TestIssue793_A2_CortexAISQL(t *testing.T) {
	for _, sql := range []string{
		"SELECT SNOWFLAKE.CORTEX.AI_COMPLETE('claude-3-5-sonnet', 'hi')",
		"SELECT SNOWFLAKE.CORTEX.AI_CLASSIFY('x', ['a', 'b'])",
		"SELECT SNOWFLAKE.CORTEX.AI_FILTER('is this positive?')",
	} {
		markers, _ := Diagnose(context.Background(), nil, sql)
		for _, m := range markers {
			if strings.Contains(m.Message, "Unknown Cortex function") {
				t.Errorf("AISQL function flagged for %q: %q", sql, m.Message)
			}
		}
	}
	// A genuine typo is still flagged.
	markers, _ := Diagnose(context.Background(), nil, "SELECT SNOWFLAKE.CORTEX.COMPLET('m', 'hi')")
	found := false
	for _, m := range markers {
		if strings.Contains(m.Message, "Unknown Cortex function") {
			found = true
		}
	}
	if !found {
		t.Error("expected a typo'd Cortex function (COMPLET) to still be flagged")
	}
}

// A3: broadened PIVOT aggregate allowlist accepts MODE / PERCENTILE_CONT.
func TestIssue793_A3_PivotAggregates(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM t PIVOT (MODE(v) FOR m IN ('a', 'b'))",
		"SELECT * FROM t PIVOT (PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY v) FOR m IN ('a'))",
	} {
		markers, _ := Diagnose(context.Background(), nil, sql)
		for _, m := range markers {
			if strings.Contains(m.Message, "not a valid aggregate function for PIVOT") {
				t.Errorf("valid PIVOT aggregate flagged for %q: %q", sql, m.Message)
			}
		}
	}
}

// E1: built-in exception variables (SQLERRM/SQLCODE/SQLSTATE) inside a scripting
// EXCEPTION handler must not be flagged as undeclared.
func TestIssue793_E1_ExceptionVariables(t *testing.T) {
	sqls := []string{
		`EXECUTE IMMEDIATE $$
BEGIN
  SELECT 1/0;
  RETURN 'ok';
EXCEPTION
  WHEN STATEMENT_ERROR THEN
    RETURN SQLERRM;
END;
$$`,
		`EXECUTE IMMEDIATE $$
BEGIN
  RETURN 1;
EXCEPTION
  WHEN OTHER THEN
    RETURN SQLCODE || SQLSTATE;
END;
$$`,
	}
	for _, sql := range sqls {
		markers, _ := Diagnose(context.Background(), nil, sql)
		for _, m := range markers {
			if strings.Contains(m.Message, "is not declared") {
				t.Errorf("built-in exception variable flagged for %q: %q", sql, m.Message)
			}
		}
	}
}
