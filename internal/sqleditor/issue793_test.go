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

// hasMarkerContaining reports whether any phase-1 marker for sql contains sub.
func hasMarkerContaining(t *testing.T, sql, sub string) bool {
	t.Helper()
	markers, err := Diagnose(context.Background(), nil, sql)
	if err != nil {
		t.Fatalf("Diagnose(%q) error: %v", sql, err)
	}
	for _, m := range markers {
		if strings.Contains(m.Message, sub) {
			return true
		}
	}
	return false
}

// B15: a MERGE with no WHEN clause is invalid.
func TestIssue793_B15_MergeRequiresWhen(t *testing.T) {
	if !hasMarkerContaining(t, "MERGE INTO t USING u ON t.id = u.id", "at least one WHEN") {
		t.Error("expected a 'requires at least one WHEN' marker for a MERGE with no WHEN clause")
	}
	// A complete MERGE is not flagged.
	if hasMarkerContaining(t, "MERGE INTO t USING u ON t.id = u.id WHEN MATCHED THEN DELETE", "at least one WHEN") {
		t.Error("a MERGE with a WHEN clause must not be flagged")
	}
}

// B16: ALTER TABLE ADD COLUMN with no data type is invalid.
func TestIssue793_B16_AddColumnNoType(t *testing.T) {
	if !hasMarkerContaining(t, "ALTER TABLE t ADD COLUMN newcol", "missing a data type") {
		t.Error("expected a 'missing a data type' marker for ADD COLUMN with no type")
	}
	if hasMarkerContaining(t, "ALTER TABLE t ADD COLUMN newcol INT", "missing a data type") {
		t.Error("ADD COLUMN with a type must not be flagged")
	}
}

// B18: the second word of a multi-word data type is validated (DOUBLE PRECISIONX).
func TestIssue793_B18_MultiWordDataType(t *testing.T) {
	if !hasMarkerContaining(t, "CREATE TABLE t (a DOUBLE PRECISIONX)", "Unknown data type") {
		t.Error("expected an unknown-data-type marker for 'DOUBLE PRECISIONX'")
	}
	// Legitimate forms are not flagged.
	for _, sql := range []string{
		"CREATE TABLE t (a DOUBLE PRECISION)",
		"CREATE TABLE t (a DOUBLE)",
		"CREATE TABLE t (a DOUBLE NOT NULL)",
	} {
		if hasMarkerContaining(t, sql, "Unknown data type") {
			t.Errorf("valid type wrongly flagged: %q", sql)
		}
	}
}

// B19 / C: Snowflake-unsupported commands (SAVEPOINT, ANALYZE) are flagged.
func TestIssue793_B19_UnsupportedCommands(t *testing.T) {
	if !hasMarkerContaining(t, "SAVEPOINT sp1", "does not support SAVEPOINT") {
		t.Error("expected SAVEPOINT to be flagged as unsupported")
	}
	if !hasMarkerContaining(t, "ANALYZE TABLE t", "does not support the ANALYZE") {
		t.Error("expected ANALYZE to be flagged as unsupported")
	}
}

// C: the ASOF MATCH_CONDITION comparison check is depth-aware — a `>` inside a
// function argument no longer satisfies a top-level `=`-only (invalid) condition.
func TestIssue793_C_AsofMatchConditionDepth(t *testing.T) {
	if !hasMarkerContaining(t,
		"SELECT * FROM a ASOF JOIN b MATCH_CONDITION (foo(a.x > 1) = b.y)",
		"MATCH_CONDITION comparison") {
		t.Error("expected a MATCH_CONDITION marker: the top-level operator is '=' (invalid)")
	}
	// A genuine top-level inequality is still accepted.
	if hasMarkerContaining(t,
		"SELECT * FROM a ASOF JOIN b MATCH_CONDITION (a.ts >= b.ts)",
		"MATCH_CONDITION comparison") {
		t.Error("a valid >= MATCH_CONDITION must not be flagged")
	}
}

// C: checkVariantPathColon now matches keyword path segments (payload.user.name,
// where USER is a keyword) — previously a keyword segment silently defeated it.
func TestIssue793_C_VariantPathKeywordSegment(t *testing.T) {
	if !hasMarkerContaining(t, "SELECT payload.user.name FROM events", "Missing colon for variant path") {
		t.Error("expected a variant-path suggestion for payload.user.name")
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

// D3: inline derived tables and scalar subqueries must not have their alias /
// inner tables misread as missing columns.
func TestIssue793_D3_DerivedTablesAndSubqueries(t *testing.T) {
	assertNoColMarkers(t, "SELECT x.id FROM (SELECT id FROM orders) x")
	assertNoColMarkers(t, "SELECT (SELECT MAX(amount) FROM orders) AS max_amt")
	assertNoColMarkers(t, "SELECT id, (SELECT COUNT(*) FROM orders) AS n FROM customers")
	// Regression: a CTE + alias already worked and must keep working.
	assertNoColMarkers(t, "WITH recent AS (SELECT id FROM orders) SELECT r.id FROM recent r")
}

// D3 regression: a genuinely unknown bare column against a plain table is still
// flagged (the derived-table guard must not over-suppress real errors).
func TestIssue793_D3_RealErrorStillFlagged(t *testing.T) {
	got := diagnoseColMarkers(t, "SELECT nosuchcol FROM orders")
	if len(got) == 0 {
		t.Error("expected a missing-column marker for 'nosuchcol' against a plain table")
	}
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

// G: a statement whose only reference is a stage must still reach the stage
// existence check instead of being short-circuited away for having no table refs.
func TestIssue793_G_StageOnlyStatement(t *testing.T) {
	// Nonexistent stage → flagged.
	markers, err := Diagnose(context.Background(), issue793Provider(), "SELECT $1 FROM @nostg/f.csv")
	if err != nil {
		t.Fatalf("Diagnose error: %v", err)
	}
	found := false
	for _, m := range markers {
		if strings.Contains(strings.ToUpper(m.Message), "STAGE 'NOSTG'") && strings.Contains(m.Message, "does not exist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a missing-stage marker for @nostg, got %+v", markers)
	}
	// Existing stage → not flagged.
	markers, _ = Diagnose(context.Background(), issue793Provider(), "SELECT $1 FROM @mystg/f.csv")
	for _, m := range markers {
		if strings.Contains(m.Message, "Stage") && strings.Contains(m.Message, "does not exist") {
			t.Errorf("existing stage @mystg wrongly flagged: %q", m.Message)
		}
	}
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
