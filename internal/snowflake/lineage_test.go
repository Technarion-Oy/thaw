// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// refKey produces a stable string key for a sqlRef for set-based comparison.
func refKey(r sqlRef) string {
	isCall := "F"
	if r.isCall {
		isCall = "T"
	}
	return r.db + "." + r.schema + "." + r.name + ":" + isCall
}

// refKeys sorts and returns the keys of a []sqlRef so tests can use reflect.DeepEqual.
func refKeys(refs []sqlRef) []string {
	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = refKey(r)
	}
	sort.Strings(keys)
	return keys
}

// ── extractArgTypesFromShow ───────────────────────────────────────────────────

func TestExtractArgTypesFromShow(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`MY_PROC(VARCHAR, NUMBER) RETURN VARCHAR`, `VARCHAR, NUMBER`},
		{`MY_PROC() RETURN BOOLEAN`, ``},
		{`MY_PROC(TABLE(VARCHAR, NUMBER)) RETURN VARCHAR`, `TABLE(VARCHAR, NUMBER)`},
		{`MY_PROC(FLOAT) RETURN FLOAT`, `FLOAT`},
		// No parentheses at all
		{`MY_PROC RETURN VARCHAR`, ``},
		// Nested parens — last ) wins via LastIndex
		{`P(ARRAY, OBJECT) RETURN VARIANT`, `ARRAY, OBJECT`},
		// Leading/trailing whitespace inside parens
		{`P(  NUMBER  ) RETURN NUMBER`, `NUMBER`},
		// Empty arg list with spaces
		{`P(  ) RETURN VARCHAR`, ``},
	}
	for _, tc := range cases {
		got := extractArgTypesFromShow(tc.in)
		if got != tc.want {
			t.Errorf("extractArgTypesFromShow(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// ── depKey / depVisited ───────────────────────────────────────────────────────

func TestDepKey_CaseNormalization(t *testing.T) {
	if depKey("mydb", "myschema", "mytable") != depKey("MYDB", "MYSCHEMA", "MYTABLE") {
		t.Error("depKey should normalise to upper-case")
	}
	if depKey("DB", "SC", "A") == depKey("DB", "SC", "B") {
		t.Error("depKey should differ for different names")
	}
}

func TestDepVisited(t *testing.T) {
	v := make(depVisited)
	if v.has("DB", "SC", "T") {
		t.Error("should be absent initially")
	}
	v.add("DB", "SC", "T")
	if !v.has("DB", "SC", "T") {
		t.Error("should be present after add")
	}
	// Case-insensitive
	if !v.has("db", "sc", "t") {
		t.Error("should match case-insensitively")
	}
	// Different name must remain absent
	if v.has("DB", "SC", "OTHER") {
		t.Error("different name should be absent")
	}
}

// ── extractDDLBody ────────────────────────────────────────────────────────────

func TestExtractDDLBody_View_AS_SELECT(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "MY_DB"."MY_SCHEMA"."MY_VIEW" AS
SELECT a, b FROM base_table WHERE a > 0;`
	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	if !containsWord(body, "base_table") {
		t.Errorf("body should contain base_table; got:\n%s", body)
	}
}

func TestExtractDDLBody_View_AS_WITH(t *testing.T) {
	ddl := `CREATE VIEW "DB"."SC"."V" AS
WITH cte AS (SELECT id FROM raw)
SELECT * FROM cte;`
	body := ExtractDDLBody(ddl, "VIEW")
	if !containsWord(body, "raw") {
		t.Errorf("body should contain 'raw'; got:\n%s", body)
	}
}

func TestExtractDDLBody_View_ComplexMultiJoin(t *testing.T) {
	ddl := `CREATE OR REPLACE SECURE VIEW "ANALYTICS"."REPORTING"."REVENUE_SUMMARY" AS
SELECT
    o.order_id,
    c.customer_name,
    p.product_name,
    oi.quantity * p.unit_price AS revenue
FROM "SALES"."PUBLIC"."ORDERS" o
JOIN "SALES"."PUBLIC"."ORDER_ITEMS" oi ON o.order_id = oi.order_id
JOIN "SALES"."PUBLIC"."CUSTOMERS" c  ON o.customer_id = c.customer_id
JOIN "PRODUCT_DB"."CATALOG"."PRODUCTS" p ON oi.product_id = p.product_id
WHERE o.status = 'COMPLETE';`
	body := ExtractDDLBody(ddl, "VIEW")
	for _, tbl := range []string{"ORDERS", "ORDER_ITEMS", "CUSTOMERS", "PRODUCTS"} {
		if !containsWord(body, tbl) {
			t.Errorf("body should contain %q; got:\n%s", tbl, body)
		}
	}
}

func TestExtractDDLBody_Procedure_DoubleDollar(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "MY_DB"."MY_SCHEMA"."MY_PROC"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  SELECT * FROM audit_log;
  CALL helper_proc();
END;
$$;`
	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body == "" {
		t.Fatal("expected non-empty body for SQL procedure")
	}
	if !containsWord(body, "audit_log") {
		t.Errorf("body should contain audit_log; got:\n%s", body)
	}
}

func TestExtractDDLBody_Procedure_SingleDollar(t *testing.T) {
	ddl := "CREATE PROCEDURE \"DB\".\"SC\".\"P\"()\nRETURNS VARCHAR\nLANGUAGE SQL\nAS\n$\nBEGIN\n  SELECT * FROM single_dollar_table;\nEND;\n$"
	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body == "" {
		t.Fatal("expected non-empty body for single-$ procedure")
	}
	if !containsWord(body, "single_dollar_table") {
		t.Errorf("body should contain single_dollar_table; got:\n%s", body)
	}
}

func TestExtractDDLBody_Procedure_NonSQL_ReturnsEmpty(t *testing.T) {
	ddl := `CREATE PROCEDURE "DB"."SC"."JS_PROC"()
RETURNS VARIANT
LANGUAGE JAVASCRIPT
AS $$
  return snowflake.execute({sqlText: "SELECT 1"});
$$;`
	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body != "" {
		t.Errorf("non-SQL procedure should return empty body; got:\n%s", body)
	}
}

func TestExtractDDLBody_Function_SQL(t *testing.T) {
	ddl := `CREATE OR REPLACE FUNCTION "DB"."SC"."MY_FUNC"(val NUMBER)
RETURNS NUMBER
LANGUAGE SQL
AS $$
SELECT SUM(amount) FROM "FINANCE"."ACC"."TRANSACTIONS" WHERE id = val
$$;`
	body := ExtractDDLBody(ddl, "FUNCTION")
	if !containsWord(body, "TRANSACTIONS") {
		t.Errorf("body should contain TRANSACTIONS; got:\n%s", body)
	}
}

func TestExtractDDLBody_Function_Python_ReturnsEmpty(t *testing.T) {
	ddl := `CREATE FUNCTION "DB"."SC"."PY_FUNC"(x INT)
RETURNS INT
LANGUAGE PYTHON
RUNTIME_VERSION = '3.9'
HANDLER = 'compute'
AS $$
def compute(x):
    return x * 2
$$;`
	body := ExtractDDLBody(ddl, "FUNCTION")
	if body != "" {
		t.Errorf("Python function should return empty body; got:\n%s", body)
	}
}

func TestExtractDDLBody_UnknownKind_ReturnsEmpty(t *testing.T) {
	ddl := `CREATE TABLE foo (id INT)`
	if body := ExtractDDLBody(ddl, "TABLE"); body != "" {
		t.Errorf("TABLE kind should return empty body; got:\n%s", body)
	}
}

// ── parseSQLReferences ────────────────────────────────────────────────────────

func TestParseSQLReferences_SimpleFrom(t *testing.T) {
	refs := parseSQLReferences(`SELECT * FROM my_table`, "DB", "SC")
	assertRefs(t, refs, []sqlRef{
		{db: "DB", schema: "SC", name: "my_table", isCall: false},
	})
}

func TestParseSQLReferences_ThreePartName(t *testing.T) {
	refs := parseSQLReferences(`SELECT * FROM other_db.other_schema.fact_table`, "DB", "SC")
	assertRefs(t, refs, []sqlRef{
		{db: "other_db", schema: "other_schema", name: "fact_table"},
	})
}

func TestParseSQLReferences_TwoPartName(t *testing.T) {
	refs := parseSQLReferences(`SELECT * FROM analytics.events`, "MY_DB", "MY_SC")
	assertRefs(t, refs, []sqlRef{
		{db: "MY_DB", schema: "analytics", name: "events"},
	})
}

func TestParseSQLReferences_QuotedIdentifiers(t *testing.T) {
	sql := `SELECT * FROM "My DB"."My Schema"."My Table"`
	refs := parseSQLReferences(sql, "DEFAULT_DB", "DEFAULT_SC")
	assertRefs(t, refs, []sqlRef{
		{db: "My DB", schema: "My Schema", name: "My Table"},
	})
}

func TestParseSQLReferences_QuotedMixedCase(t *testing.T) {
	sql := `SELECT * FROM "Sales_DB"."public"."Order Items"`
	refs := parseSQLReferences(sql, "DB", "SC")
	assertRefs(t, refs, []sqlRef{
		{db: "Sales_DB", schema: "public", name: "Order Items"},
	})
}

func TestParseSQLReferences_JOIN(t *testing.T) {
	sql := `SELECT a.id, b.name
FROM orders a
JOIN customers b ON a.customer_id = b.id`
	refs := parseSQLReferences(sql, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "orders"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "customers"})
}

func TestParseSQLReferences_MultipleJoinTypes(t *testing.T) {
	sql := `SELECT *
FROM base_table
LEFT JOIN dim_date ON base_table.date_id = dim_date.id
INNER JOIN dim_customer ON base_table.cust_id = dim_customer.id
RIGHT JOIN dim_product ON base_table.prod_id = dim_product.id`
	refs := parseSQLReferences(sql, "DW", "CORE")
	for _, name := range []string{"base_table", "dim_date", "dim_customer", "dim_product"} {
		assertContainsRef(t, refs, sqlRef{db: "DW", schema: "CORE", name: name})
	}
}

func TestParseSQLReferences_MergeInto(t *testing.T) {
	sql := `MERGE INTO target_table AS t
USING source_view AS s ON t.id = s.id
WHEN MATCHED THEN UPDATE SET t.val = s.val
WHEN NOT MATCHED THEN INSERT (id, val) VALUES (s.id, s.val)`
	refs := parseSQLReferences(sql, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "target_table"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "source_view"})
}

func TestParseSQLReferences_Update(t *testing.T) {
	refs := parseSQLReferences(`UPDATE employee_records SET salary = 100`, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "employee_records"})
}

func TestParseSQLReferences_InsertInto(t *testing.T) {
	refs := parseSQLReferences(
		`INSERT INTO audit_log (event, ts) SELECT action, CURRENT_TIMESTAMP FROM staging_events`,
		"DB", "SC",
	)
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "audit_log"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "staging_events"})
}

func TestParseSQLReferences_CallProcedure(t *testing.T) {
	refs := parseSQLReferences(`CALL my_proc(1, 'hello')`, "DB", "SC")
	assertRefs(t, refs, []sqlRef{
		{db: "DB", schema: "SC", name: "my_proc", isCall: true},
	})
}

func TestParseSQLReferences_CallFullyQualified(t *testing.T) {
	refs := parseSQLReferences(`CALL "Ops DB"."ETL"."load_daily"()`, "DB", "SC")
	assertRefs(t, refs, []sqlRef{
		{db: "Ops DB", schema: "ETL", name: "load_daily", isCall: true},
	})
}

func TestParseSQLReferences_CTEExclusion(t *testing.T) {
	sql := `WITH cte_sales AS (
    SELECT id, amount FROM raw_sales
), cte_returns AS (
    SELECT id, amount FROM raw_returns
)
SELECT * FROM cte_sales
JOIN cte_returns ON cte_sales.id = cte_returns.id`
	refs := parseSQLReferences(sql, "DB", "SC")
	// cte_sales and cte_returns must NOT appear; raw_sales and raw_returns MUST
	for _, r := range refs {
		if r.name == "cte_sales" || r.name == "cte_returns" {
			t.Errorf("CTE name %q should be excluded", r.name)
		}
	}
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "raw_sales"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "raw_returns"})
}

func TestParseSQLReferences_InformationSchemaExclusion(t *testing.T) {
	sql := `SELECT * FROM my_db.INFORMATION_SCHEMA.TABLES`
	refs := parseSQLReferences(sql, "DB", "SC")
	for _, r := range refs {
		if r.schema == "INFORMATION_SCHEMA" || r.name == "TABLES" {
			t.Errorf("INFORMATION_SCHEMA reference should be excluded; got %+v", r)
		}
	}
	if len(refs) != 0 {
		t.Errorf("expected no refs; got %v", refs)
	}
}

func TestParseSQLReferences_SkipNames(t *testing.T) {
	// Keywords that follow FROM/INTO should be filtered by skipNames.
	sql := `SELECT * FROM TABLE(generator(rowcount => 10))`
	refs := parseSQLReferences(sql, "DB", "SC")
	for _, r := range refs {
		if r.name == "TABLE" || r.name == "GENERATOR" {
			t.Errorf("keyword %q must not appear as a reference", r.name)
		}
	}
}

func TestParseSQLReferences_LineCommentStripped(t *testing.T) {
	sql := `SELECT *
FROM real_table -- was: FROM old_table
JOIN another_table ON real_table.id = another_table.id`
	refs := parseSQLReferences(sql, "DB", "SC")
	for _, r := range refs {
		if r.name == "old_table" {
			t.Error("commented-out table must not appear as a reference")
		}
	}
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "real_table"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "another_table"})
}

func TestParseSQLReferences_BlockCommentStripped(t *testing.T) {
	sql := `SELECT *
FROM real_table
/* FROM commented_table */
WHERE real_table.active = TRUE`
	refs := parseSQLReferences(sql, "DB", "SC")
	for _, r := range refs {
		if r.name == "commented_table" {
			t.Error("block-commented table must not appear")
		}
	}
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "real_table"})
}

func TestParseSQLReferences_Deduplication(t *testing.T) {
	// The same table referenced multiple times must yield one sqlRef.
	sql := `SELECT a.id, b.id
FROM orders a
JOIN orders b ON a.parent_id = b.id
WHERE a.id IN (SELECT id FROM orders)`
	refs := parseSQLReferences(sql, "DB", "SC")
	count := 0
	for _, r := range refs {
		if r.name == "orders" {
			count++
		}
	}
	// parseSQLReferences itself does NOT deduplicate; deduplication is done
	// in buildChildren.  Assert at least one occurrence here.
	if count == 0 {
		t.Error("orders should appear at least once")
	}
}

// ── Full pipeline: extractDDLBody → parseSQLReferences ─────────────────────────

func TestPipeline_View_DeepCTE(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "ANALYTICS"."FINANCE"."MONTHLY_REVENUE" AS
WITH
  raw_orders AS (
    SELECT * FROM "SALES"."PUBLIC"."ORDERS"
  ),
  paid_orders AS (
    SELECT * FROM raw_orders WHERE status = 'PAID'
  ),
  enriched AS (
    SELECT po.*, c.segment
    FROM paid_orders po
    JOIN "SALES"."PUBLIC"."CUSTOMERS" c ON po.customer_id = c.id
  )
SELECT
  DATE_TRUNC('MONTH', order_date) AS month,
  SUM(total) AS revenue
FROM enriched
GROUP BY 1;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("body must not be empty")
	}
	refs := parseSQLReferences(body, "ANALYTICS", "FINANCE")

	// raw_orders, paid_orders, enriched are CTEs — must be excluded
	for _, r := range refs {
		switch r.name {
		case "raw_orders", "paid_orders", "enriched":
			t.Errorf("CTE name %q must not appear as a reference", r.name)
		}
	}
	// Real tables must appear
	assertContainsRef(t, refs, sqlRef{db: "SALES", schema: "PUBLIC", name: "ORDERS"})
	assertContainsRef(t, refs, sqlRef{db: "SALES", schema: "PUBLIC", name: "CUSTOMERS"})
}

func TestPipeline_Procedure_CallAndSelect(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "ETL"."PIPELINES"."LOAD_DAILY"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- Stage raw events
  INSERT INTO "ETL"."STAGING"."DAILY_EVENTS"
  SELECT event_id, ts, payload
  FROM "INGEST"."RAW"."KAFKA_EVENTS"
  WHERE DATE(ts) = CURRENT_DATE();

  -- Enrich
  UPDATE "ETL"."STAGING"."DAILY_EVENTS" t
  SET t.user_segment = u.segment
  FROM "CRM"."PUBLIC"."USERS" u
  WHERE t.user_id = u.id;

  -- Delegate to archiver
  CALL "ETL"."PIPELINES"."ARCHIVE_EVENTS"();

  RETURN 'OK';
END;
$$;`

	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	refs := parseSQLReferences(body, "ETL", "PIPELINES")

	assertContainsRef(t, refs, sqlRef{db: "ETL", schema: "STAGING", name: "DAILY_EVENTS"})
	assertContainsRef(t, refs, sqlRef{db: "INGEST", schema: "RAW", name: "KAFKA_EVENTS"})
	assertContainsRef(t, refs, sqlRef{db: "CRM", schema: "PUBLIC", name: "USERS"})
	assertContainsRef(t, refs, sqlRef{db: "ETL", schema: "PIPELINES", name: "ARCHIVE_EVENTS", isCall: true})
}

func TestPipeline_View_QuotedComplexNames(t *testing.T) {
	// All identifiers contain spaces, mixed case, or special characters.
	ddl := `CREATE VIEW "Analytics DB"."Reporting Schema"."Revenue By Region" AS
SELECT
    r.region_name,
    SUM(s.amount) AS total
FROM "Sales DB"."Order Mgmt"."Sales Fact" s
JOIN "Geo DB"."Reference"."Region Dim" r ON s.region_id = r.id
LEFT JOIN "Sales DB"."Order Mgmt"."Discount Table" d ON s.discount_id = d.id;`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "Analytics DB", "Reporting Schema")

	assertContainsRef(t, refs, sqlRef{db: "Sales DB", schema: "Order Mgmt", name: "Sales Fact"})
	assertContainsRef(t, refs, sqlRef{db: "Geo DB", schema: "Reference", name: "Region Dim"})
	assertContainsRef(t, refs, sqlRef{db: "Sales DB", schema: "Order Mgmt", name: "Discount Table"})
}

func TestPipeline_Procedure_ManyDependencyLevels(t *testing.T) {
	// A procedure that calls another procedure and selects from several tables
	// across multiple databases and schemas, with CTE aliases and comments.
	ddl := `CREATE OR REPLACE PROCEDURE "DW"."TRANSFORM"."BUILD_CUBE"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- Step 1: load dimension tables
  MERGE INTO "DW"."DIMS"."DIM_DATE" AS tgt
  USING (SELECT * FROM "STAGING"."RAW"."DATE_FEED") AS src
  ON tgt.date_key = src.date_key
  WHEN NOT MATCHED THEN INSERT VALUES (src.date_key, src.full_date);

  -- Step 2: aggregate facts
  INSERT INTO "DW"."FACTS"."SALES_CUBE"
  WITH daily AS (
    SELECT product_id, SUM(qty) AS total_qty
    FROM "DW"."STAGE"."DAILY_SALES"
    GROUP BY 1
  ),
  enriched AS (
    SELECT d.*, p.category
    FROM daily d
    JOIN "DW"."DIMS"."DIM_PRODUCT" p ON d.product_id = p.id
  )
  SELECT * FROM enriched;

  -- Step 3: delegate to archiver
  CALL "DW"."OPS"."ARCHIVE_CUBE"();

  -- Step 4: refresh another proc
  CALL "DW"."OPS"."REFRESH_STATS"();

  RETURN 'DONE';
END;
$$;`

	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	refs := parseSQLReferences(body, "DW", "TRANSFORM")

	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "DIMS", name: "DIM_DATE"})
	assertContainsRef(t, refs, sqlRef{db: "STAGING", schema: "RAW", name: "DATE_FEED"})
	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "FACTS", name: "SALES_CUBE"})
	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "STAGE", name: "DAILY_SALES"})
	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "DIMS", name: "DIM_PRODUCT"})
	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "OPS", name: "ARCHIVE_CUBE", isCall: true})
	assertContainsRef(t, refs, sqlRef{db: "DW", schema: "OPS", name: "REFRESH_STATS", isCall: true})

	// CTE aliases must not leak
	for _, r := range refs {
		if r.name == "daily" || r.name == "enriched" {
			t.Errorf("CTE alias %q must not appear as a reference", r.name)
		}
	}
}

func TestPipeline_View_CrossDatabaseMergeAndUnion(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "BI"."PUBLIC"."UNIFIED_EVENTS" AS
SELECT user_id, event_type, ts FROM "APP_DB"."EVENTS"."CLICK_EVENTS"
UNION ALL
SELECT user_id, event_type, ts FROM "APP_DB"."EVENTS"."PAGE_VIEWS"
UNION ALL
SELECT user_id, 'purchase' AS event_type, created_at AS ts
FROM "COMMERCE"."ORDERS"."PURCHASES";`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "BI", "PUBLIC")

	assertContainsRef(t, refs, sqlRef{db: "APP_DB", schema: "EVENTS", name: "CLICK_EVENTS"})
	assertContainsRef(t, refs, sqlRef{db: "APP_DB", schema: "EVENTS", name: "PAGE_VIEWS"})
	assertContainsRef(t, refs, sqlRef{db: "COMMERCE", schema: "ORDERS", name: "PURCHASES"})
}

func TestPipeline_View_SubqueryInWhereAndSelect(t *testing.T) {
	ddl := `CREATE VIEW "DB"."SC"."ACTIVE_USERS" AS
SELECT
    u.id,
    u.name,
    (SELECT MAX(ts) FROM "DB"."SC"."LOGIN_EVENTS" le WHERE le.user_id = u.id) AS last_login
FROM "DB"."SC"."USERS" u
WHERE u.id IN (SELECT DISTINCT user_id FROM "DB"."SC"."SESSIONS");`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "LOGIN_EVENTS"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "USERS"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "SESSIONS"})
}

func TestPipeline_Procedure_CommentedOutCALL(t *testing.T) {
	ddl := `CREATE PROCEDURE "DB"."SC"."REAL_PROC"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- CALL old_proc(); this was disabled
  /* CALL another_old_proc(); */
  SELECT * FROM active_table;
  CALL live_proc();
END;
$$;`
	body := ExtractDDLBody(ddl, "PROCEDURE")
	refs := parseSQLReferences(body, "DB", "SC")

	for _, r := range refs {
		if r.name == "old_proc" || r.name == "another_old_proc" {
			t.Errorf("commented-out CALL target %q must not appear", r.name)
		}
	}
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "active_table"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "live_proc", isCall: true})
}

func TestPipeline_View_MaxDepthBound(t *testing.T) {
	// Verifies the maxDependencyDepth constant is exposed and sane.
	if maxDependencyDepth < 4 || maxDependencyDepth > 20 {
		t.Errorf("maxDependencyDepth = %d, expected between 4 and 20", maxDependencyDepth)
	}
}

// ── assert helpers ────────────────────────────────────────────────────────────

// assertRefs checks that the refs slice contains exactly the expected references
// (order-independent, using key comparison).
func assertRefs(t *testing.T, got []sqlRef, want []sqlRef) {
	t.Helper()
	gotKeys := refKeys(got)
	wantKeys := refKeys(want)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("refs mismatch\n got:  %v\nwant: %v", gotKeys, wantKeys)
	}
}

// assertContainsRef checks that at least one element in refs matches the
// expected ref (db, schema, name, isCall all equal).
func assertContainsRef(t *testing.T, refs []sqlRef, want sqlRef) {
	t.Helper()
	for _, r := range refs {
		if strings.EqualFold(r.db, want.db) &&
			strings.EqualFold(r.schema, want.schema) &&
			strings.EqualFold(r.name, want.name) &&
			r.isCall == want.isCall {
			return
		}
	}
	t.Errorf("expected ref %+v not found in:\n%v", want, refs)
}

// containsWord returns true if word appears (case-insensitive) in s.
func containsWord(s, word string) bool {
	return strings.Contains(strings.ToUpper(s), strings.ToUpper(word))
}

// ── RewriteSQLReferences tests ────────────────────────────────────────────────

func TestRewriteSQLReferences(t *testing.T) {
	// Lookup helper: maps (db, schema, name) to a fixed Jinja replacement.
	// Tables → {{ source(...) }}, views → {{ ref(...) }}, unknown → "".
	makeLookup := func(tables, views map[string]string) func(db, schema, name string) string {
		return func(db, schema, name string) string {
			key := strings.ToUpper(db + "." + schema + "." + name)
			if r, ok := tables[key]; ok {
				return r
			}
			if r, ok := views[key]; ok {
				return r
			}
			return ""
		}
	}

	tests := []struct {
		name           string
		sql            string
		defaultDB      string
		defaultSchema  string
		tables         map[string]string // UPPER(db.schema.name) → replacement
		views          map[string]string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "user example: three-part quoted view referencing source table",
			sql: `SELECT
    "Order ID" AS CLEAN_ORDER_ID,
    "SELECT" AS SELECTION_TYPE,
    "Date-Of-Purchase"
FROM LINEAGE_SOURCE_DB."Oddly Named Schema!"."Order Details #1"
WHERE "From" = 'Web'`,
			defaultDB:     "LINEAGE_TARGET_DB",
			defaultSchema: "MART",
			tables: map[string]string{
				`LINEAGE_SOURCE_DB.ODDLY NAMED SCHEMA!.ORDER DETAILS #1`: `{{ source('lineage_source_db_oddly named schema_', 'Order Details #1') }}`,
			},
			mustContain: []string{
				`{{ source('lineage_source_db_oddly named schema_', 'Order Details #1') }}`,
				`"Order ID" AS CLEAN_ORDER_ID`, // column aliases preserved
				`WHERE "From" = 'Web'`,
			},
			mustNotContain: []string{
				`LINEAGE_SOURCE_DB."Oddly Named Schema!"."Order Details #1"`,
			},
		},
		{
			name:          "view referencing another view becomes ref()",
			sql:           `SELECT * FROM MY_DB.MY_SCHEMA.MY_VIEW`,
			defaultDB:     "MY_DB",
			defaultSchema: "MY_SCHEMA",
			views: map[string]string{
				"MY_DB.MY_SCHEMA.MY_VIEW": "{{ ref('stg_my_view') }}",
			},
			mustContain:    []string{"{{ ref('stg_my_view') }}"},
			mustNotContain: []string{"MY_DB.MY_SCHEMA.MY_VIEW"},
		},
		{
			name:          "unknown reference left unchanged",
			sql:           `SELECT * FROM EXTERNAL_DB.EXTERNAL_SCHEMA.SOME_TABLE`,
			defaultDB:     "MY_DB",
			defaultSchema: "MY_SCHEMA",
			mustContain:   []string{"EXTERNAL_DB.EXTERNAL_SCHEMA.SOME_TABLE"},
		},
		{
			name: "CTE aliases are not replaced",
			sql: `WITH orders AS (SELECT * FROM MY_DB.STAGING.RAW_ORDERS)
SELECT * FROM orders`,
			defaultDB:     "MY_DB",
			defaultSchema: "STAGING",
			tables: map[string]string{
				"MY_DB.STAGING.RAW_ORDERS": "{{ source('my_db_staging', 'RAW_ORDERS') }}",
			},
			mustContain:    []string{"{{ source('my_db_staging', 'RAW_ORDERS') }}"},
			mustNotContain: []string{"FROM MY_DB.STAGING.RAW_ORDERS"}, // replaced
			// "FROM orders" must remain (CTE alias, not replaced)
		},
		{
			name:          "single-part bare name is not replaced (ambiguous)",
			sql:           `SELECT * FROM ORDERS`,
			defaultDB:     "MY_DB",
			defaultSchema: "MY_SCHEMA",
			tables: map[string]string{
				"MY_DB.MY_SCHEMA.ORDERS": "{{ source('my_db_my_schema', 'ORDERS') }}",
			},
			// bare single-part name — skip to avoid column/alias false positives
			mustContain: []string{"FROM ORDERS"},
		},
		{
			name:          "two-part schema.table is replaced",
			sql:           `SELECT * FROM MY_SCHEMA.MY_TABLE`,
			defaultDB:     "MY_DB",
			defaultSchema: "MY_SCHEMA",
			tables: map[string]string{
				"MY_DB.MY_SCHEMA.MY_TABLE": "{{ source('my_db_my_schema', 'MY_TABLE') }}",
			},
			mustContain:    []string{"{{ source('my_db_my_schema', 'MY_TABLE') }}"},
			mustNotContain: []string{"MY_SCHEMA.MY_TABLE"},
		},
		{
			name: "multiple references in one query",
			sql: `SELECT a.*, b.*
FROM DB.S1.TABLE_A a
JOIN DB.S1.TABLE_B b ON a.id = b.id`,
			defaultDB:     "DB",
			defaultSchema: "S1",
			tables: map[string]string{
				"DB.S1.TABLE_A": "{{ source('db_s1', 'TABLE_A') }}",
				"DB.S1.TABLE_B": "{{ source('db_s1', 'TABLE_B') }}",
			},
			mustContain: []string{
				"{{ source('db_s1', 'TABLE_A') }}",
				"{{ source('db_s1', 'TABLE_B') }}",
			},
			mustNotContain: []string{"DB.S1.TABLE_A", "DB.S1.TABLE_B"},
		},
		{
			name: "reference inside comment is also replaced (acceptable side effect)",
			sql: `-- reads from MY_DB.MY_SCHEMA.MY_TABLE
SELECT * FROM MY_DB.MY_SCHEMA.MY_TABLE`,
			defaultDB:     "MY_DB",
			defaultSchema: "MY_SCHEMA",
			tables: map[string]string{
				"MY_DB.MY_SCHEMA.MY_TABLE": "{{ source('my_db_my_schema', 'MY_TABLE') }}",
			},
			mustContain:    []string{"{{ source('my_db_my_schema', 'MY_TABLE') }}"},
			mustNotContain: []string{"FROM MY_DB.MY_SCHEMA.MY_TABLE"},
		},
		{
			name:      "no references → SQL unchanged",
			sql:       `SELECT 1 AS n`,
			defaultDB: "DB", defaultSchema: "S",
			mustContain: []string{"SELECT 1 AS n"},
		},

		// ── complex cases ─────────────────────────────────────────────────────

		{
			name: "mixed table→source() and view→ref() in same query",
			sql: `SELECT o.id, v.segment, o.amount
FROM PROD.ORDERS.FACT_ORDERS o
JOIN PROD.ORDERS.DIM_CUSTOMERS_VIEW v ON o.customer_id = v.id
WHERE o.status = 'COMPLETE'`,
			defaultDB: "PROD", defaultSchema: "ORDERS",
			tables: map[string]string{
				"PROD.ORDERS.FACT_ORDERS": "{{ source('prod_orders', 'FACT_ORDERS') }}",
			},
			views: map[string]string{
				"PROD.ORDERS.DIM_CUSTOMERS_VIEW": "{{ ref('stg_dim_customers_view') }}",
			},
			mustContain: []string{
				"{{ source('prod_orders', 'FACT_ORDERS') }}",
				"{{ ref('stg_dim_customers_view') }}",
			},
			mustNotContain: []string{
				"PROD.ORDERS.FACT_ORDERS",
				"PROD.ORDERS.DIM_CUSTOMERS_VIEW",
			},
		},
		{
			name: "same three-part reference appears three times — all occurrences replaced",
			sql: `SELECT a.*
FROM DB.STAGING.ORDERS a
WHERE a.id NOT IN (
    SELECT id FROM DB.STAGING.ORDERS WHERE status = 'CANCELLED'
)
AND a.customer_id IN (SELECT DISTINCT customer_id FROM DB.STAGING.ORDERS)`,
			defaultDB: "DB", defaultSchema: "STAGING",
			tables: map[string]string{
				"DB.STAGING.ORDERS": "{{ source('db_staging', 'ORDERS') }}",
			},
			mustContain:    []string{"{{ source('db_staging', 'ORDERS') }}"},
			mustNotContain: []string{"DB.STAGING.ORDERS"},
		},
		{
			name: "UNION ALL across four source databases",
			sql: `SELECT user_id, 'click'    AS kind FROM TRACK_DB.EVENTS.CLICKS
UNION ALL
SELECT user_id, 'view'     AS kind FROM TRACK_DB.EVENTS.PAGEVIEWS
UNION ALL
SELECT user_id, 'purchase' AS kind FROM COMMERCE_DB.ORDERS.PURCHASES
UNION ALL
SELECT user_id, 'signup'   AS kind FROM AUTH_DB.ACCOUNTS.SIGNUPS`,
			defaultDB: "BI", defaultSchema: "PUBLIC",
			tables: map[string]string{
				"TRACK_DB.EVENTS.CLICKS":       "{{ source('track_db_events', 'CLICKS') }}",
				"TRACK_DB.EVENTS.PAGEVIEWS":    "{{ source('track_db_events', 'PAGEVIEWS') }}",
				"COMMERCE_DB.ORDERS.PURCHASES": "{{ source('commerce_db_orders', 'PURCHASES') }}",
				"AUTH_DB.ACCOUNTS.SIGNUPS":     "{{ source('auth_db_accounts', 'SIGNUPS') }}",
			},
			mustContain: []string{
				"{{ source('track_db_events', 'CLICKS') }}",
				"{{ source('track_db_events', 'PAGEVIEWS') }}",
				"{{ source('commerce_db_orders', 'PURCHASES') }}",
				"{{ source('auth_db_accounts', 'SIGNUPS') }}",
			},
			mustNotContain: []string{
				"TRACK_DB.EVENTS.CLICKS",
				"TRACK_DB.EVENTS.PAGEVIEWS",
				"COMMERCE_DB.ORDERS.PURCHASES",
				"AUTH_DB.ACCOUNTS.SIGNUPS",
			},
		},
		{
			name: "all JOIN variants: LEFT, RIGHT, INNER, FULL OUTER",
			sql: `SELECT *
FROM DB.SCH.FACT_TABLE f
LEFT JOIN DB.SCH.DIM_A a ON f.a_id = a.id
RIGHT JOIN DB.SCH.DIM_B b ON f.b_id = b.id
INNER JOIN DB.SCH.DIM_C c ON f.c_id = c.id
FULL OUTER JOIN DB.SCH.DIM_D d ON f.d_id = d.id`,
			defaultDB: "DB", defaultSchema: "SCH",
			tables: map[string]string{
				"DB.SCH.FACT_TABLE": "{{ source('db_sch', 'FACT_TABLE') }}",
				"DB.SCH.DIM_A":      "{{ source('db_sch', 'DIM_A') }}",
				"DB.SCH.DIM_B":      "{{ source('db_sch', 'DIM_B') }}",
				"DB.SCH.DIM_C":      "{{ source('db_sch', 'DIM_C') }}",
				"DB.SCH.DIM_D":      "{{ source('db_sch', 'DIM_D') }}",
			},
			mustContain: []string{
				"{{ source('db_sch', 'FACT_TABLE') }}",
				"{{ source('db_sch', 'DIM_A') }}",
				"{{ source('db_sch', 'DIM_B') }}",
				"{{ source('db_sch', 'DIM_C') }}",
				"{{ source('db_sch', 'DIM_D') }}",
			},
			mustNotContain: []string{
				"DB.SCH.FACT_TABLE",
				"DB.SCH.DIM_A",
				"DB.SCH.DIM_B",
				"DB.SCH.DIM_C",
				"DB.SCH.DIM_D",
			},
		},
		{
			name: "deeply nested three-level CTEs: real tables replaced, all CTE aliases preserved",
			sql: `WITH
raw_data AS (
    SELECT * FROM PROD.SOURCES.EVENTS
),
filtered AS (
    SELECT * FROM raw_data WHERE status = 'ACTIVE'
),
enriched AS (
    SELECT f.*, u.name
    FROM filtered f
    JOIN PROD.SOURCES.USERS u ON f.user_id = u.id
)
SELECT * FROM enriched`,
			defaultDB: "PROD", defaultSchema: "SOURCES",
			tables: map[string]string{
				"PROD.SOURCES.EVENTS": "{{ source('prod_sources', 'EVENTS') }}",
				"PROD.SOURCES.USERS":  "{{ source('prod_sources', 'USERS') }}",
			},
			mustContain: []string{
				"{{ source('prod_sources', 'EVENTS') }}",
				"{{ source('prod_sources', 'USERS') }}",
				"FROM raw_data",   // CTE alias: single-part, not replaced
				"FROM filtered f", // CTE alias: single-part, not replaced
				"FROM enriched",   // CTE alias: single-part, not replaced
			},
			mustNotContain: []string{
				"PROD.SOURCES.EVENTS",
				"PROD.SOURCES.USERS",
			},
		},
		{
			name: "references in scalar subquery and WHERE IN clause",
			sql: `SELECT
    u.id,
    u.name,
    (SELECT MAX(ts) FROM ANALYTICS.EVENTS.LOGIN_LOG l WHERE l.user_id = u.id) AS last_login
FROM ANALYTICS.USERS.USERS u
WHERE u.id IN (
    SELECT DISTINCT user_id FROM ANALYTICS.EVENTS.ACTIVE_SESSIONS
)`,
			defaultDB: "ANALYTICS", defaultSchema: "USERS",
			tables: map[string]string{
				"ANALYTICS.EVENTS.LOGIN_LOG":       "{{ source('analytics_events', 'LOGIN_LOG') }}",
				"ANALYTICS.USERS.USERS":            "{{ source('analytics_users', 'USERS') }}",
				"ANALYTICS.EVENTS.ACTIVE_SESSIONS": "{{ source('analytics_events', 'ACTIVE_SESSIONS') }}",
			},
			mustContain: []string{
				"{{ source('analytics_events', 'LOGIN_LOG') }}",
				"{{ source('analytics_users', 'USERS') }}",
				"{{ source('analytics_events', 'ACTIVE_SESSIONS') }}",
			},
			mustNotContain: []string{
				"ANALYTICS.EVENTS.LOGIN_LOG",
				"ANALYTICS.USERS.USERS",
				"ANALYTICS.EVENTS.ACTIVE_SESSIONS",
			},
		},
		{
			name:      "case-insensitive lookup: lowercase SQL ref matches uppercase key",
			sql:       `SELECT * FROM mydb.myschema.mytable`,
			defaultDB: "MYDB", defaultSchema: "MYSCHEMA",
			tables: map[string]string{
				"MYDB.MYSCHEMA.MYTABLE": "{{ source('mydb_myschema', 'MYTABLE') }}",
			},
			mustContain:    []string{"{{ source('mydb_myschema', 'MYTABLE') }}"},
			mustNotContain: []string{"mydb.myschema.mytable"},
		},
		{
			name: "two-part and three-part of same table: both replaced independently",
			sql: `SELECT *
FROM DB.PUBLIC.ORDERS o1
JOIN PUBLIC.ORDERS o2 ON o1.id = o2.parent_id`,
			defaultDB: "DB", defaultSchema: "PUBLIC",
			tables: map[string]string{
				"DB.PUBLIC.ORDERS": "{{ source('db_public', 'ORDERS') }}",
			},
			mustContain:    []string{"{{ source('db_public', 'ORDERS') }}"},
			mustNotContain: []string{"DB.PUBLIC.ORDERS", "PUBLIC.ORDERS"},
		},
		{
			name: "three-part INFORMATION_SCHEMA reference is excluded from replacement",
			sql: `SELECT * FROM DB.INFORMATION_SCHEMA.TABLES
JOIN DB.PUBLIC.MY_TABLE t ON t.id = 1`,
			defaultDB: "DB", defaultSchema: "PUBLIC",
			tables: map[string]string{
				"DB.PUBLIC.MY_TABLE": "{{ source('db_public', 'MY_TABLE') }}",
			},
			mustContain: []string{
				"DB.INFORMATION_SCHEMA.TABLES",          // left unchanged
				"{{ source('db_public', 'MY_TABLE') }}", // known ref replaced
			},
			mustNotContain: []string{"DB.PUBLIC.MY_TABLE"},
		},
		{
			name: "LATERAL FLATTEN: TABLE and FLATTEN keywords produce no spurious refs",
			sql: `SELECT f.value
FROM DB.SCH.EVENTS e,
LATERAL FLATTEN(input => e.tags) f
WHERE e.active = TRUE`,
			defaultDB: "DB", defaultSchema: "SCH",
			tables: map[string]string{
				"DB.SCH.EVENTS": "{{ source('db_sch', 'EVENTS') }}",
			},
			mustContain:    []string{"{{ source('db_sch', 'EVENTS') }}"},
			mustNotContain: []string{"DB.SCH.EVENTS"},
		},
		{
			name: "MERGE INTO target and inner FROM source both replaced",
			sql: `MERGE INTO PROD.DWH.FACT_SALES tgt
USING (
    SELECT id, amount FROM STAGING.RAW.DAILY_SALES
) src
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET tgt.amount = src.amount
WHEN NOT MATCHED THEN INSERT (id, amount) VALUES (src.id, src.amount)`,
			defaultDB: "PROD", defaultSchema: "DWH",
			tables: map[string]string{
				"PROD.DWH.FACT_SALES":     "{{ source('prod_dwh', 'FACT_SALES') }}",
				"STAGING.RAW.DAILY_SALES": "{{ source('staging_raw', 'DAILY_SALES') }}",
			},
			mustContain: []string{
				"{{ source('prod_dwh', 'FACT_SALES') }}",
				"{{ source('staging_raw', 'DAILY_SALES') }}",
			},
			mustNotContain: []string{
				"PROD.DWH.FACT_SALES",
				"STAGING.RAW.DAILY_SALES",
			},
		},
		{
			name: "UPDATE target and FROM source both replaced",
			sql: `UPDATE PROD.DWH.USERS
SET status = src.status
FROM STAGING.RAW.USER_UPDATES src
WHERE id = src.id`,
			defaultDB: "PROD", defaultSchema: "DWH",
			tables: map[string]string{
				"PROD.DWH.USERS":           "{{ source('prod_dwh', 'USERS') }}",
				"STAGING.RAW.USER_UPDATES": "{{ source('staging_raw', 'USER_UPDATES') }}",
			},
			mustContain: []string{
				"{{ source('prod_dwh', 'USERS') }}",
				"{{ source('staging_raw', 'USER_UPDATES') }}",
			},
			mustNotContain: []string{
				"PROD.DWH.USERS",
				"STAGING.RAW.USER_UPDATES",
			},
		},
		{
			name: "external refs left unchanged while known refs are replaced",
			sql: `SELECT o.id, p.name
FROM PROD.ORDERS.FACT_ORDERS o
JOIN EXTERNAL_PARTNER.CATALOG.PRODUCTS p ON o.product_id = p.id
LEFT JOIN INTERNAL_LOOKUP.REF.REGIONS r ON o.region_id = r.id`,
			defaultDB: "PROD", defaultSchema: "ORDERS",
			tables: map[string]string{
				"PROD.ORDERS.FACT_ORDERS": "{{ source('prod_orders', 'FACT_ORDERS') }}",
				// EXTERNAL_PARTNER and INTERNAL_LOOKUP are not in the lookup
			},
			mustContain: []string{
				"{{ source('prod_orders', 'FACT_ORDERS') }}",
				"EXTERNAL_PARTNER.CATALOG.PRODUCTS",
				"INTERNAL_LOOKUP.REF.REGIONS",
			},
			mustNotContain: []string{"PROD.ORDERS.FACT_ORDERS"},
		},
		{
			name: "CTE alias name same as table's final component: three-part ref is excluded (known limitation)",
			// When a CTE is named 'orders', any identifier ending in 'orders'
			// (including DB.SCHEMA.ORDERS) is excluded from replacement because
			// RewriteSQLReferences checks only the final name component against
			// cteNames.  This is a known conservative behavior.
			sql: `WITH orders AS (SELECT * FROM PROD.SOURCES.RAW_ORDERS)
SELECT * FROM PROD.SOURCES.ORDERS`,
			defaultDB: "PROD", defaultSchema: "SOURCES",
			tables: map[string]string{
				"PROD.SOURCES.RAW_ORDERS": "{{ source('prod_sources', 'RAW_ORDERS') }}",
				"PROD.SOURCES.ORDERS":     "{{ source('prod_sources', 'ORDERS') }}",
			},
			// RAW_ORDERS is replaced (its name differs from the CTE alias)
			mustContain: []string{
				"{{ source('prod_sources', 'RAW_ORDERS') }}",
				// PROD.SOURCES.ORDERS is NOT replaced: "ORDERS" matches the CTE alias
				"PROD.SOURCES.ORDERS",
			},
			mustNotContain: []string{"PROD.SOURCES.RAW_ORDERS"},
		},
		{
			name:      "mixed two-part schema.table replaced via defaultDB resolution",
			sql:       `SELECT * FROM SALES.ORDERS o JOIN SALES.CUSTOMERS c ON o.cust_id = c.id`,
			defaultDB: "MYDB", defaultSchema: "SALES",
			tables: map[string]string{
				"MYDB.SALES.ORDERS":    "{{ source('mydb_sales', 'ORDERS') }}",
				"MYDB.SALES.CUSTOMERS": "{{ source('mydb_sales', 'CUSTOMERS') }}",
			},
			mustContain: []string{
				"{{ source('mydb_sales', 'ORDERS') }}",
				"{{ source('mydb_sales', 'CUSTOMERS') }}",
			},
			mustNotContain: []string{"SALES.ORDERS", "SALES.CUSTOMERS"},
		},
		{
			name: "INSERT INTO target and SELECT FROM source both replaced",
			sql: `INSERT INTO PROD.AUDIT.EVENT_LOG (event, ts)
SELECT action, CURRENT_TIMESTAMP
FROM STAGING.EVENTS.RAW_ACTIONS`,
			defaultDB: "PROD", defaultSchema: "AUDIT",
			tables: map[string]string{
				"PROD.AUDIT.EVENT_LOG":       "{{ source('prod_audit', 'EVENT_LOG') }}",
				"STAGING.EVENTS.RAW_ACTIONS": "{{ source('staging_events', 'RAW_ACTIONS') }}",
			},
			mustContain: []string{
				"{{ source('prod_audit', 'EVENT_LOG') }}",
				"{{ source('staging_events', 'RAW_ACTIONS') }}",
			},
			mustNotContain: []string{
				"PROD.AUDIT.EVENT_LOG",
				"STAGING.EVENTS.RAW_ACTIONS",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables := tt.tables
			if tables == nil {
				tables = map[string]string{}
			}
			views := tt.views
			if views == nil {
				views = map[string]string{}
			}
			got := RewriteSQLReferences(tt.sql, tt.defaultDB, tt.defaultSchema, makeLookup(tables, views))
			for _, want := range tt.mustContain {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q\ngot:\n%s", want, got)
				}
			}
			for _, notWant := range tt.mustNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("expected output NOT to contain %q\ngot:\n%s", notWant, got)
				}
			}
		})
	}
}

// ── Full pipeline: ExtractDDLBody → RewriteSQLReferences ─────────────────────

// TestPipeline_ViewInline_RewrittenReferences simulates the full dbt inline-view
// flow: GET_DDL → ExtractDDLBody → RewriteSQLReferences.
func TestPipeline_ViewInline_RewrittenReferences(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "ANALYTICS"."MART"."REVENUE_BY_SEGMENT" AS
WITH base AS (
    SELECT
        o.id,
        o.amount,
        c.segment
    FROM "ANALYTICS"."SOURCES"."ORDERS" o
    JOIN "CRM"."PUBLIC"."CUSTOMERS" c ON o.customer_id = c.id
),
enriched AS (
    SELECT b.*, p.region
    FROM base b
    JOIN "GEO"."REF"."POSTAL_CODES" p ON b.postal = p.code
)
SELECT segment, region, SUM(amount) AS revenue
FROM enriched
GROUP BY 1, 2;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("body must not be empty")
	}

	lookup := func(db, schema, name string) string {
		switch strings.ToUpper(db + "." + schema + "." + name) {
		case "ANALYTICS.SOURCES.ORDERS":
			return "{{ source('analytics_sources', 'ORDERS') }}"
		case "CRM.PUBLIC.CUSTOMERS":
			return "{{ ref('stg_customers') }}"
		default:
			return "" // GEO.REF.POSTAL_CODES is external — left unchanged
		}
	}

	got := RewriteSQLReferences(body, "ANALYTICS", "MART", lookup)

	if !strings.Contains(got, "{{ source('analytics_sources', 'ORDERS') }}") {
		t.Errorf("ORDERS not replaced with source():\n%s", got)
	}
	if !strings.Contains(got, "{{ ref('stg_customers') }}") {
		t.Errorf("CUSTOMERS not replaced with ref():\n%s", got)
	}
	// External reference must survive unchanged.
	if !strings.Contains(got, `"GEO"."REF"."POSTAL_CODES"`) {
		t.Errorf("external ref POSTAL_CODES must remain unchanged:\n%s", got)
	}
	// CTE aliases must survive (single-part names, not replaced).
	if !strings.Contains(got, "FROM base b") {
		t.Errorf("CTE alias 'base' must not be replaced:\n%s", got)
	}
	if !strings.Contains(got, "FROM enriched") {
		t.Errorf("CTE alias 'enriched' must not be replaced:\n%s", got)
	}
	// Aggregate result must still be structurally valid SQL.
	if !strings.Contains(got, "GROUP BY 1, 2") {
		t.Errorf("GROUP BY clause must survive rewrite:\n%s", got)
	}
}

// TestPipeline_ViewInline_AllUnknown checks that a view whose references are
// all outside the selected schemas returns the SQL completely unchanged.
func TestPipeline_ViewInline_AllUnknown(t *testing.T) {
	ddl := `CREATE VIEW "DB"."SC"."BRIDGE" AS
SELECT a.id, b.code
FROM EXTERNAL_A.SCH.TABLE_A a
JOIN EXTERNAL_B.SCH.TABLE_B b ON a.id = b.id`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("body must not be empty")
	}
	// Lookup always returns "" — nothing is in the selected schemas.
	got := RewriteSQLReferences(body, "DB", "SC", func(_, _, _ string) string { return "" })
	if got != body {
		t.Errorf("SQL with no known refs must be returned unchanged\ngot:\n%s\nwant:\n%s", got, body)
	}
}

// TestPipeline_ViewInline_TenSourcesComplex exercises a realistic wide view
// that joins ten tables across three schemas, with CTEs, subqueries, and
// multiple alias layers.
func TestPipeline_ViewInline_TenSourcesComplex(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "DW"."REPORTING"."EXEC_DASHBOARD" AS
WITH
orders AS (
    SELECT o.id, o.amount, o.customer_id, o.product_id, o.region_id
    FROM "DW"."CORE"."FACT_ORDERS" o
    WHERE o.status = 'COMPLETE'
),
returns AS (
    SELECT r.order_id, r.refund_amount
    FROM "DW"."CORE"."FACT_RETURNS" r
),
net AS (
    SELECT o.id, o.amount - COALESCE(r.refund_amount, 0) AS net_amount, o.customer_id, o.product_id, o.region_id
    FROM orders o
    LEFT JOIN returns r ON o.id = r.order_id
),
customers AS (
    SELECT c.id, c.name, c.segment, c.country_id
    FROM "DW"."DIMS"."DIM_CUSTOMER" c
),
products AS (
    SELECT p.id, p.name AS product_name, p.category, p.brand_id
    FROM "DW"."DIMS"."DIM_PRODUCT" p
),
brands AS (
    SELECT b.id, b.brand_name
    FROM "DW"."DIMS"."DIM_BRAND" b
),
regions AS (
    SELECT rg.id, rg.region_name, rg.country
    FROM "DW"."DIMS"."DIM_REGION" rg
),
enriched AS (
    SELECT
        n.id,
        n.net_amount,
        cust.name AS customer_name,
        cust.segment,
        prod.product_name,
        prod.category,
        br.brand_name,
        reg.region_name
    FROM net n
    JOIN customers cust ON n.customer_id = cust.id
    JOIN products prod ON n.product_id = prod.id
    JOIN brands br ON prod.brand_id = br.id
    JOIN regions reg ON n.region_id = reg.id
)
SELECT
    segment,
    category,
    brand_name,
    region_name,
    SUM(net_amount) AS total_revenue,
    COUNT(*) AS order_count
FROM enriched
GROUP BY 1, 2, 3, 4;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("body must not be empty")
	}

	// Build a lookup covering the six real tables; enriched/orders/returns/net/
	// customers/products/brands/regions are CTEs and must be excluded.
	knownTables := map[string]string{
		"DW.CORE.FACT_ORDERS":  "{{ source('dw_core', 'FACT_ORDERS') }}",
		"DW.CORE.FACT_RETURNS": "{{ source('dw_core', 'FACT_RETURNS') }}",
		"DW.DIMS.DIM_CUSTOMER": "{{ source('dw_dims', 'DIM_CUSTOMER') }}",
		"DW.DIMS.DIM_PRODUCT":  "{{ source('dw_dims', 'DIM_PRODUCT') }}",
		"DW.DIMS.DIM_BRAND":    "{{ source('dw_dims', 'DIM_BRAND') }}",
		"DW.DIMS.DIM_REGION":   "{{ source('dw_dims', 'DIM_REGION') }}",
	}
	lookup := func(db, schema, name string) string {
		return knownTables[strings.ToUpper(db+"."+schema+"."+name)]
	}

	got := RewriteSQLReferences(body, "DW", "REPORTING", lookup)

	// All six real tables must be replaced.
	for orig, repl := range knownTables {
		if !strings.Contains(got, repl) {
			t.Errorf("expected %q to be replaced with %q:\n%s", orig, repl, got)
		}
	}

	// CTE aliases used as single-part table references must survive unchanged
	// (single-part names are never replaced by RewriteSQLReferences).
	for _, alias := range []string{
		"FROM orders o",       // net CTE's FROM
		"LEFT JOIN returns r", // net CTE's JOIN
		"FROM net n",          // enriched CTE's FROM
		"JOIN customers cust", // enriched CTE's JOINs
		"JOIN products prod",
		"JOIN brands br",
		"JOIN regions reg",
		"FROM enriched", // final SELECT
	} {
		if !strings.Contains(got, alias) {
			t.Errorf("CTE alias reference %q must remain: got:\n%s", alias, got)
		}
	}
}

// ── Additional ExtractDDLBody tests ───────────────────────────────────────────

func TestExtractDDLBody_View_SecureWithCopyGrants(t *testing.T) {
	// SECURE + COPY GRANTS + COMMENT appear between the view name and AS SELECT;
	// ExtractDDLBody must skip them and return only the SELECT body.
	ddl := `CREATE OR REPLACE SECURE VIEW "PROD"."REPORTING"."CUSTOMER_SUMMARY"
COPY GRANTS
COMMENT = 'Customer-level summary for analysts'
AS
SELECT c.id, c.name, COUNT(o.id) AS order_count
FROM "PROD"."CORE"."CUSTOMERS" c
LEFT JOIN "PROD"."CORE"."ORDERS" o ON c.id = o.customer_id
GROUP BY 1, 2;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("expected non-empty body for SECURE view with COPY GRANTS")
	}
	for _, want := range []string{"CUSTOMERS", "ORDERS", "GROUP BY"} {
		if !containsWord(body, want) {
			t.Errorf("body should contain %q; got:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"COPY GRANTS", "SECURE", "COMMENT ="} {
		if strings.Contains(body, notWant) {
			t.Errorf("body must not contain DDL header token %q; got:\n%s", notWant, body)
		}
	}
}

func TestExtractDDLBody_View_ManyColumnAliases(t *testing.T) {
	// Multiple AS keywords inside the SELECT clause and FROM aliases must not
	// confuse the body extractor — only the outermost AS SELECT matters.
	ddl := `CREATE OR REPLACE VIEW "DB"."SC"."ALIASES_GALORE" AS
SELECT
    id AS user_id,
    name AS full_name,
    email AS contact_email,
    created_at AS signup_date
FROM "DB"."SC"."USERS" AS u
WHERE u.id IN (SELECT user_id AS uid FROM "DB"."SC"."SESSIONS" AS s);`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	for _, want := range []string{"user_id", "USERS", "SESSIONS"} {
		if !containsWord(body, want) {
			t.Errorf("body should contain %q; got:\n%s", want, body)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(body), "CREATE") {
		t.Errorf("body must not start with CREATE; got:\n%s", body)
	}
}

func TestExtractDDLBody_View_WithRecursiveKeyword(t *testing.T) {
	// Some dialects emit CREATE RECURSIVE VIEW; the AS SELECT body should still
	// be extracted correctly.
	ddl := `CREATE OR REPLACE RECURSIVE VIEW "DB"."SC"."HIERARCHY" AS
WITH RECURSIVE org (id, parent_id, depth) AS (
    SELECT id, parent_id, 0 FROM "DB"."SC"."DEPARTMENTS" WHERE parent_id IS NULL
    UNION ALL
    SELECT d.id, d.parent_id, r.depth + 1
    FROM "DB"."SC"."DEPARTMENTS" d
    JOIN org r ON d.parent_id = r.id
)
SELECT * FROM org;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("expected non-empty body for RECURSIVE view")
	}
	if !containsWord(body, "DEPARTMENTS") {
		t.Errorf("body should contain DEPARTMENTS; got:\n%s", body)
	}
	if !containsWord(body, "RECURSIVE") {
		// RECURSIVE appears inside the CTE declaration, should survive in body.
		// (It's part of the WITH RECURSIVE clause, not the CREATE header.)
		t.Errorf("body should contain RECURSIVE CTE keyword; got:\n%s", body)
	}
}

func TestExtractDDLBody_Procedure_ScalarSingleQuote(t *testing.T) {
	// Some Snowflake tools export procedures with single-quoted bodies.
	ddl := "CREATE OR REPLACE PROCEDURE \"DB\".\"SC\".\"QUOTED_PROC\"()\nRETURNS VARCHAR\nLANGUAGE SQL\nAS 'BEGIN\n  SELECT * FROM \"DB\".\"SC\".\"AUDIT_TABLE\";\n  RETURN ''OK'';\nEND'"

	body := ExtractDDLBody(ddl, "PROCEDURE")
	if body == "" {
		t.Fatal("expected non-empty body for single-quoted procedure")
	}
	if !containsWord(body, "AUDIT_TABLE") {
		t.Errorf("body should contain AUDIT_TABLE; got:\n%s", body)
	}
}

func TestExtractDDLBody_Function_InlineSQL(t *testing.T) {
	// SQL UDF with an inline expression (no dollar-quoting).
	ddl := `CREATE OR REPLACE FUNCTION "DB"."SC"."TAX_RATE"(region VARCHAR)
RETURNS FLOAT
LANGUAGE SQL
AS $$
SELECT rate FROM "DB"."FINANCE"."TAX_TABLE" WHERE region_code = region LIMIT 1
$$;`

	body := ExtractDDLBody(ddl, "FUNCTION")
	if !containsWord(body, "TAX_TABLE") {
		t.Errorf("body should contain TAX_TABLE; got:\n%s", body)
	}
}

// TestExtractDDLBody_View_SemicolonInsideStringLiteral ensures that a
// semicolon embedded in a string value does not truncate the extracted body.
func TestExtractDDLBody_View_SemicolonInsideStringLiteral(t *testing.T) {
	ddl := `CREATE VIEW "DB"."SC"."V" AS
SELECT id,
    CASE WHEN type = 'A;B' THEN 'match' ELSE 'none' END AS label
FROM "DB"."SC"."SOURCE_TABLE";`

	body := ExtractDDLBody(ddl, "VIEW")
	if !containsWord(body, "SOURCE_TABLE") {
		t.Errorf("body should contain SOURCE_TABLE; got:\n%s", body)
	}
	if !strings.Contains(body, "A;B") {
		t.Errorf("body should preserve semicolon inside string literal; got:\n%s", body)
	}
}

// TestPipeline_View_MergeAndWindowFunctions verifies that window functions
// and complex aggregations are parsed correctly without false positives.
func TestPipeline_View_MergeAndWindowFunctions(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "ANALYTICS"."MART"."USER_COHORTS" AS
SELECT
    u.id,
    u.signup_date,
    e.event_count,
    ROW_NUMBER() OVER (PARTITION BY u.segment ORDER BY e.event_count DESC) AS rank_in_segment,
    LAG(e.event_count) OVER (ORDER BY u.signup_date) AS prev_count,
    SUM(e.event_count) OVER (PARTITION BY u.country) AS country_total
FROM "ANALYTICS"."USERS"."USER_BASE" u
JOIN (
    SELECT user_id, COUNT(*) AS event_count
    FROM "ANALYTICS"."EVENTS"."RAW_EVENTS"
    GROUP BY 1
) e ON u.id = e.user_id
WHERE u.active = TRUE;`

	body := ExtractDDLBody(ddl, "VIEW")
	if body == "" {
		t.Fatal("body must not be empty")
	}
	refs := parseSQLReferences(body, "ANALYTICS", "MART")

	assertContainsRef(t, refs, sqlRef{db: "ANALYTICS", schema: "USERS", name: "USER_BASE"})
	assertContainsRef(t, refs, sqlRef{db: "ANALYTICS", schema: "EVENTS", name: "RAW_EVENTS"})

	// Window function keywords must not appear as spurious references.
	for _, r := range refs {
		switch strings.ToUpper(r.name) {
		case "OVER", "PARTITION", "ORDER", "ROW_NUMBER", "LAG", "SUM":
			t.Errorf("window function keyword %q must not appear as a reference", r.name)
		}
	}
}

func TestPipeline_ImplicitJoins(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "DB"."SC"."OLD_SCHOOL_JOIN" AS
SELECT a.id, b.name, c.status
FROM "DB"."SC"."TABLE_A" a, "DB"."SC"."TABLE_B" b, "DB"."SC"."TABLE_C" c
WHERE a.id = b.id AND b.id = c.id;`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "DB", "SC")

	// The parser easily finds TABLE_A because it follows the "FROM" keyword
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "TABLE_A"})

	// Now it also finds TABLE_B and TABLE_C by parsing the comma-separated list
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "TABLE_B"})
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "TABLE_C"})
}

func TestPipeline_CloneDependency(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "DB"."SC"."BACKUP_PROC"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- The procedure creates a backup from production
  CREATE TRANSIENT TABLE "DB"."SC"."USERS_BACKUP" 
  CLONE "PROD"."CORE"."USERS";
END;
$$;`

	body := ExtractDDLBody(ddl, "PROCEDURE")
	refs := parseSQLReferences(body, "DB", "SC")

	// The regex parser now looks for the "CLONE" keyword, picking up USERS
	assertContainsRef(t, refs, sqlRef{db: "PROD", schema: "CORE", name: "USERS"})
}
func TestPipeline_StageFalsePositive(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "DB"."SC"."EXPORT_DATA"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- Exporting data to an internal Snowflake stage
  COPY INTO @my_export_stage/daily_run/
  FROM "DB"."SC"."EVENTS";
END;
$$;`

	body := ExtractDDLBody(ddl, "PROCEDURE")
	refs := parseSQLReferences(body, "DB", "SC")

	// The real table dependency is found successfully
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "EVENTS"})

	// Snowflake stages (@stage_name) are correctly ignored by the parser
	for _, r := range refs {
		if strings.HasPrefix(r.name, "@") {
			t.Errorf("Lineage failure: falsely identified Snowflake stage %q as a table", r.name)
		}
	}
}

func TestPipeline_KnownLimitation_IdentifierFunction(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "DB"."SC"."DYNAMIC_REF" AS
SELECT a.id, a.status
FROM IDENTIFIER('"PROD_DB"."CORE"."USERS"');`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "DB", "SC")

	// ACTUAL BEHAVIOR (Flawed): The parser stops at the word IDENTIFIER.
	// It never looks inside the parentheses to find the real table.
	for _, r := range refs {
		if r.name == "USERS" {
			t.Errorf("Parser surprisingly resolved the IDENTIFIER() function. Has it been upgraded?")
		}
	}
}

func TestPipeline_KnownLimitation_SpacedIdentifiers(t *testing.T) {
	ddl := `CREATE OR REPLACE VIEW "DB"."SC"."SPACED_OUT" AS
SELECT * FROM "PROD_DB" . "CORE" . "USERS";`

	body := ExtractDDLBody(ddl, "VIEW")
	refs := parseSQLReferences(body, "DB", "SC")

	// ACTUAL BEHAVIOR (Flawed): The regex `identPat` requires strict dots.
	// When it sees `FROM "PROD_DB" .`, it stops at "PROD_DB" and treats it as a single-part
	// table name in the default DB/Schema, completely missing "CORE" and "USERS".

	// Ensure the real 3-part dependency is NOT found, proving the bug exists.
	assertNotContainsRef(t, refs, sqlRef{db: "PROD_DB", schema: "CORE", name: "USERS"})
}

// Helper for the tests to assert absence
func assertNotContainsRef(t *testing.T, refs []sqlRef, want sqlRef) {
	t.Helper()
	for _, r := range refs {
		if strings.EqualFold(r.db, want.db) &&
			strings.EqualFold(r.schema, want.schema) &&
			strings.EqualFold(r.name, want.name) &&
			r.isCall == want.isCall {
			t.Errorf("expected ref %+v to NOT be found, but it was", want)
		}
	}
}

// Mocking Client methods required by GetObjectDependencies
// Since Go doesn't allow overriding methods easily without an interface,
// we'll adapt the test by running the `buildChildren` logic with an interface or just verifying `depVisited` behavior directly.
// The issue was in how `depVisited` was passed. Let's test `buildChildren` directly if possible, or verify `clone` behavior.
// Because we can't easily mock `c.GetObjectDDL`, we can construct a test case that ensures `depVisited.clone()` isolates state.

func TestDepVisited_CloneIsolatesState(t *testing.T) {
	v1 := make(depVisited)
	v1.add("DB", "SC", "NODE_A")

	v2 := v1.clone()
	v2.add("DB", "SC", "NODE_B")

	if v1.has("DB", "SC", "NODE_B") {
		t.Error("v1 should not have NODE_B after cloning")
	}

	if !v2.has("DB", "SC", "NODE_A") {
		t.Error("v2 should still have NODE_A from the clone")
	}

	if !v2.has("DB", "SC", "NODE_B") {
		t.Error("v2 should have NODE_B after adding to it")
	}
}

func TestPipeline_KnownLimitation_TableFunctions(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "DB"."SC"."PARSE_DATA"()
RETURNS VARCHAR
LANGUAGE SQL
AS $$
BEGIN
  -- Calling a User-Defined Table Function (UDTF)
  INSERT INTO "DB"."SC"."LOGS"
  SELECT * FROM TABLE("PROD"."UTILS"."PARSE_JSON_FUNC"(payload));
END;
$$;`

	body := ExtractDDLBody(ddl, "PROCEDURE")
	refs := parseSQLReferences(body, "DB", "SC")

	// It successfully finds the INSERT target
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "LOGS"})

	// ACTUAL BEHAVIOR (Flawed): Because "PARSE_JSON_FUNC" is wrapped in "TABLE()",
	// it doesn't directly follow FROM/JOIN, so the regex engine misses it entirely.
	assertNotContainsRef(t, refs, sqlRef{db: "PROD", schema: "UTILS", name: "PARSE_JSON_FUNC"})
}

// ── tokenizer-backed correctness (regressions the old regexes failed) ─────────

// TestParseSQLReferences_NestedBlockComment verifies that Snowflake's nested
// block comments are stripped as a whole. The previous non-greedy /\*.*?\*/
// regex stopped at the first "*/", leaking the tail of a nested comment back
// into the scanned SQL and producing a phantom FROM reference.
func TestParseSQLReferences_NestedBlockComment(t *testing.T) {
	sql := `SELECT * FROM real_table
/* comment /* nested */ FROM fake_table still_comment */
WHERE 1 = 1`
	refs := parseSQLReferences(sql, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "real_table"})
	for _, r := range refs {
		if r.name == "fake_table" {
			t.Errorf("table inside a nested block comment must not appear as a reference")
		}
	}
}

// TestParseSQLReferences_KeywordInStringLiteral verifies that a SQL keyword
// embedded in a string literal does not yield a spurious reference. The old
// parser stripped comments but not strings, so 'FROM secret_table' matched.
func TestParseSQLReferences_KeywordInStringLiteral(t *testing.T) {
	sql := `SELECT 'FROM secret_table' AS note FROM real_table`
	refs := parseSQLReferences(sql, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "real_table"})
	for _, r := range refs {
		if r.name == "secret_table" {
			t.Errorf("identifier inside a string literal must not appear as a reference")
		}
	}
}

// TestExtractDDLBody_Procedure_TaggedDollarQuote verifies that a $tag$-delimited
// body is extracted. The old reProcBodyDouble only matched $$ and returned an
// empty body (no references) for tagged dollar quotes.
func TestExtractDDLBody_Procedure_TaggedDollarQuote(t *testing.T) {
	ddl := `CREATE OR REPLACE PROCEDURE "DB"."SC"."TAGGED"()
RETURNS VARCHAR
LANGUAGE SQL
AS $proc$
BEGIN
  SELECT * FROM "DB"."SC"."TAGGED_SOURCE";
END;
$proc$;`
	body := ExtractDDLBody(ddl, "PROCEDURE")
	if !containsWord(body, "TAGGED_SOURCE") {
		t.Errorf("body should contain TAGGED_SOURCE; got:\n%s", body)
	}
	refs := parseSQLReferences(body, "DB", "SC")
	assertContainsRef(t, refs, sqlRef{db: "DB", schema: "SC", name: "TAGGED_SOURCE"})
}
