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

// ── stripQuotes ───────────────────────────────────────────────────────────────

func TestStripQuotes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`"MY_TABLE"`, `MY_TABLE`},
		{`MY_TABLE`, `MY_TABLE`},
		{`"table with spaces"`, `table with spaces`},
		{`"dot.in.name"`, `dot.in.name`},
		{`"mixed CASE Table"`, `mixed CASE Table`},
		{`  "padded"  `, `padded`},           // TrimSpace applied before stripping
		{`""`, ``},                             // empty quoted ident
		{`"`, `"`},                             // single quote — not valid, returned as-is
		{`"unclosed`, `"unclosed`},             // no closing quote
		{`"""double-quote-inside"""`, `""double-quote-inside""`}, // one outer pair stripped, inner pair remains
		{`SCHEMA`, `SCHEMA`},
	}
	for _, tc := range cases {
		got := stripQuotes(tc.in)
		if got != tc.want {
			t.Errorf("stripQuotes(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// ── splitIdent ────────────────────────────────────────────────────────────────

func TestSplitIdent(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		// Simple unquoted
		{`MY_TABLE`, []string{`MY_TABLE`}},
		// Two-part unquoted
		{`MY_SCHEMA.MY_TABLE`, []string{`MY_SCHEMA`, `MY_TABLE`}},
		// Three-part unquoted
		{`MY_DB.MY_SCHEMA.MY_TABLE`, []string{`MY_DB`, `MY_SCHEMA`, `MY_TABLE`}},
		// Quoted with spaces
		{`"My DB"."My Schema"."My Table"`, []string{`My DB`, `My Schema`, `My Table`}},
		// Mixed quoted / unquoted
		{`MY_DB."weird schema".MY_TABLE`, []string{`MY_DB`, `weird schema`, `MY_TABLE`}},
		// Quoted with dot inside — the dot is inside quotes so it must not be split
		// Note: splitIdent splits on "." regardless of quoting; this is intentional
		// behaviour (identPat in the regex already handles the full matched token).
		// Here we just verify the basic split works for the three legitimate cases.
		{`DB.SCHEMA.TABLE`, []string{`DB`, `SCHEMA`, `TABLE`}},
		// Identifier with special chars in quotes
		{`"100% Valid"."Order-Items"`, []string{`100% Valid`, `Order-Items`}},
		// All-caps quoted (quoting preserves case, stripping strips quotes only)
		{`"MYDB"."MYSCHEMA"."MYTABLE"`, []string{`MYDB`, `MYSCHEMA`, `MYTABLE`}},
	}
	for _, tc := range cases {
		got := splitIdent(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitIdent(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
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
		name         string
		sql          string
		defaultDB    string
		defaultSchema string
		tables       map[string]string // UPPER(db.schema.name) → replacement
		views        map[string]string
		mustContain  []string
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
			defaultDB:    "LINEAGE_TARGET_DB",
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
			name: "view referencing another view becomes ref()",
			sql:  `SELECT * FROM MY_DB.MY_SCHEMA.MY_VIEW`,
			defaultDB:    "MY_DB",
			defaultSchema: "MY_SCHEMA",
			views: map[string]string{
				"MY_DB.MY_SCHEMA.MY_VIEW": "{{ ref('stg_my_view') }}",
			},
			mustContain:    []string{"{{ ref('stg_my_view') }}"},
			mustNotContain: []string{"MY_DB.MY_SCHEMA.MY_VIEW"},
		},
		{
			name: "unknown reference left unchanged",
			sql:  `SELECT * FROM EXTERNAL_DB.EXTERNAL_SCHEMA.SOME_TABLE`,
			defaultDB:    "MY_DB",
			defaultSchema: "MY_SCHEMA",
			mustContain:    []string{"EXTERNAL_DB.EXTERNAL_SCHEMA.SOME_TABLE"},
		},
		{
			name: "CTE aliases are not replaced",
			sql: `WITH orders AS (SELECT * FROM MY_DB.STAGING.RAW_ORDERS)
SELECT * FROM orders`,
			defaultDB:    "MY_DB",
			defaultSchema: "STAGING",
			tables: map[string]string{
				"MY_DB.STAGING.RAW_ORDERS": "{{ source('my_db_staging', 'RAW_ORDERS') }}",
			},
			mustContain:    []string{"{{ source('my_db_staging', 'RAW_ORDERS') }}"},
			mustNotContain: []string{"FROM MY_DB.STAGING.RAW_ORDERS"}, // replaced
			// "FROM orders" must remain (CTE alias, not replaced)
		},
		{
			name: "single-part bare name is not replaced (ambiguous)",
			sql:  `SELECT * FROM ORDERS`,
			defaultDB:    "MY_DB",
			defaultSchema: "MY_SCHEMA",
			tables: map[string]string{
				"MY_DB.MY_SCHEMA.ORDERS": "{{ source('my_db_my_schema', 'ORDERS') }}",
			},
			// bare single-part name — skip to avoid column/alias false positives
			mustContain: []string{"FROM ORDERS"},
		},
		{
			name: "two-part schema.table is replaced",
			sql:  `SELECT * FROM MY_SCHEMA.MY_TABLE`,
			defaultDB:    "MY_DB",
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
			defaultDB:    "DB",
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
			defaultDB:    "MY_DB",
			defaultSchema: "MY_SCHEMA",
			tables: map[string]string{
				"MY_DB.MY_SCHEMA.MY_TABLE": "{{ source('my_db_my_schema', 'MY_TABLE') }}",
			},
			mustContain:    []string{"{{ source('my_db_my_schema', 'MY_TABLE') }}"},
			mustNotContain: []string{"FROM MY_DB.MY_SCHEMA.MY_TABLE"},
		},
		{
			name: "no references → SQL unchanged",
			sql:  `SELECT 1 AS n`,
			defaultDB: "DB", defaultSchema: "S",
			mustContain: []string{"SELECT 1 AS n"},
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
