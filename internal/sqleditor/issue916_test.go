// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"context"
	"strings"
	"testing"
)

// issue916Provider is a catalog holding SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.{CUSTOMER,
// ORDERS,NATION} and (optionally) the tables the demo script creates in COCO_DEMO.
func issue916Provider(withCoco bool) *fakeProvider {
	p := &fakeProvider{
		databases: []string{"SNOWFLAKE_SAMPLE_DATA"},
		schemas:   map[string][]string{"SNOWFLAKE_SAMPLE_DATA": {"TPCH_SF1"}},
		objects: map[string][]StoreObject{
			schemaObjectKey("SNOWFLAKE_SAMPLE_DATA", "TPCH_SF1"): {
				{DB: "SNOWFLAKE_SAMPLE_DATA", Schema: "TPCH_SF1", Name: "CUSTOMER", Kind: "TABLE"},
				{DB: "SNOWFLAKE_SAMPLE_DATA", Schema: "TPCH_SF1", Name: "ORDERS", Kind: "TABLE"},
				{DB: "SNOWFLAKE_SAMPLE_DATA", Schema: "TPCH_SF1", Name: "NATION", Kind: "TABLE"},
			},
		},
		columns: map[string][]ColInfo{
			"SNOWFLAKE_SAMPLE_DATA\x00TPCH_SF1\x00CUSTOMER": {{Name: "C_CUSTKEY"}, {Name: "C_NAME"}, {Name: "C_NATIONKEY"}, {Name: "C_ACCTBAL"}},
			"SNOWFLAKE_SAMPLE_DATA\x00TPCH_SF1\x00ORDERS":   {{Name: "O_ORDERKEY"}, {Name: "O_CUSTKEY"}, {Name: "O_ORDERDATE"}, {Name: "O_TOTALPRICE"}, {Name: "O_ORDERSTATUS"}},
			"SNOWFLAKE_SAMPLE_DATA\x00TPCH_SF1\x00NATION":   {{Name: "N_NATIONKEY"}, {Name: "N_NAME"}},
		},
	}
	if withCoco {
		p.databases = append(p.databases, "COCO_DEMO")
		p.schemas["COCO_DEMO"] = []string{"PUBLIC"}
		p.objects[schemaObjectKey("COCO_DEMO", "PUBLIC")] = []StoreObject{
			{DB: "COCO_DEMO", Schema: "PUBLIC", Name: "CUSTOMERS", Kind: "TABLE"},
			{DB: "COCO_DEMO", Schema: "PUBLIC", Name: "ORDERS", Kind: "TABLE"},
		}
		p.columns["COCO_DEMO\x00PUBLIC\x00CUSTOMERS"] = []ColInfo{{Name: "CUSTOMER_ID"}, {Name: "NAME"}, {Name: "NATION_ID"}, {Name: "BALANCE"}}
		p.columns["COCO_DEMO\x00PUBLIC\x00ORDERS"] = []ColInfo{{Name: "ORDER_ID"}, {Name: "CUSTOMER_ID"}, {Name: "ORDER_DATE"}, {Name: "TOTAL"}, {Name: "STATUS"}}
	}
	return p
}

const issue916Script = `CREATE OR REPLACE DATABASE COCO_DEMO;
CREATE OR REPLACE WAREHOUSE DEMO_WH WAREHOUSE_SIZE = XSMALL AUTO_SUSPEND = 60;
USE SCHEMA COCO_DEMO.PUBLIC;

-- Round 1: schema with planted data quality bugs
CREATE OR REPLACE TABLE CUSTOMERS AS
  SELECT C_CUSTKEY AS CUSTOMER_ID, C_NAME AS NAME, C_NATIONKEY AS NATION_ID, C_ACCTBAL AS BALANCE
  FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.CUSTOMER;
CREATE OR REPLACE TABLE ORDERS AS
  SELECT O_ORDERKEY AS ORDER_ID, O_CUSTKEY AS CUSTOMER_ID, O_ORDERDATE AS ORDER_DATE,
         O_TOTALPRICE AS TOTAL, O_ORDERSTATUS AS STATUS
  FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.ORDERS;
INSERT INTO ORDERS SELECT ORDER_ID + 90000000, 999999999, ORDER_DATE, TOTAL, STATUS FROM ORDERS LIMIT 500;
INSERT INTO ORDERS SELECT * FROM ORDERS LIMIT 200;
INSERT INTO ORDERS VALUES (1, 1, '2031-01-01', 10, 'O');

-- Round 2: deliberately slow view
CREATE OR REPLACE VIEW SLOW_REVENUE AS
  SELECT n.N_NAME AS NATION, TO_CHAR(o.ORDER_DATE, 'YYYY-MM') AS MONTH, SUM(o.TOTAL) AS REVENUE
  FROM ORDERS o, CUSTOMERS c, SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATION n
  WHERE TO_VARCHAR(o.CUSTOMER_ID) = TO_VARCHAR(c.CUSTOMER_ID) AND c.NATION_ID = n.N_NATIONKEY
  GROUP BY 1, 2;

-- Round 4: task that fails on purpose (references a column that no longer exists)
CREATE OR REPLACE TABLE DAILY_TOTALS (DAY DATE, TOTAL NUMBER);
CREATE OR REPLACE TASK LOAD_DAILY_TOTALS WAREHOUSE = DEMO_WH SCHEDULE = '5 MINUTE' AS
  INSERT INTO DAILY_TOTALS SELECT ORDER_DATE, SUM(O_TOTALPRICE) FROM ORDERS GROUP BY 1;
ALTER TASK LOAD_DAILY_TOTALS RESUME;

-- Round 5: oversized warehouse with no auto-suspend
CREATE OR REPLACE WAREHOUSE LEGACY_WH WAREHOUSE_SIZE = LARGE AUTO_SUSPEND = 0;
USE WAREHOUSE LEGACY_WH;
SELECT COUNT(*) FROM ORDERS;
SELECT NATION, SUM(REVENUE) FROM SLOW_REVENUE GROUP BY 1;
ALTER WAREHOUSE LEGACY_WH SUSPEND;  -- suspend manually after filming, not before
`

// TestIssue916_DemoScriptNoFalsePositives: CTAS tables shadow same-named catalog
// tables, comma-joined FROM sources are all recognized, and CREATE TASK header
// identifiers are not scanned as columns. The only marker left is the bug the
// script plants on purpose: O_TOTALPRICE no longer exists in the CTAS'd ORDERS.
func TestIssue916_DemoScriptNoFalsePositives(t *testing.T) {
	for _, withCoco := range []bool{false, true} {
		markers, err := Diagnose(context.Background(), issue916Provider(withCoco), issue916Script)
		if err != nil {
			t.Fatalf("Diagnose: %v", err)
		}
		if len(markers) != 1 || markers[0].StartLineNumber != 27 || !strings.Contains(markers[0].Message, "'O_TOTALPRICE'") {
			for _, m := range markers {
				t.Logf("line %d: %s", m.StartLineNumber, m.Message)
			}
			t.Errorf("withCoco=%v: want exactly the O_TOTALPRICE marker on line 27, got %d markers", withCoco, len(markers))
		}
	}
}

// TestIssue916_CTASUnknownColumnsShadowCatalog: a CTAS whose columns can't be
// derived (SELECT *) still shadows the same-named catalog table, disabling
// validation instead of checking against the wrong table's columns.
// An in-script ALTER … ADD COLUMN must not turn the "columns unknown" entry into
// a known set holding only the added column (PR #917 review).
func TestIssue916_CTASUnknownColumnsShadowCatalog(t *testing.T) {
	for name, alter := range map[string]string{
		"plain":      "",
		"alter-add":  "ALTER TABLE ORDERS ADD COLUMN NOTES VARCHAR;\n",
		"alter-qual": "ALTER TABLE COCO_DEMO.PUBLIC.ORDERS ADD COLUMN NOTES VARCHAR;\n",
	} {
		sql := "CREATE DATABASE COCO_DEMO;\nUSE SCHEMA COCO_DEMO.PUBLIC;\n" +
			"CREATE OR REPLACE TABLE ORDERS AS SELECT *, 1 AS X FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.ORDERS;\n" +
			alter + "SELECT o.ANYTHING, ANYTHING FROM ORDERS o;"
		markers, err := Diagnose(context.Background(), issue916Provider(false), sql)
		if err != nil {
			t.Fatalf("Diagnose: %v", err)
		}
		for _, m := range markers {
			t.Errorf("%s: unexpected marker line %d: %s", name, m.StartLineNumber, m.Message)
		}
	}
}

// The CREATE header is skipped by position, not by name: header identifiers
// (option values, a view's output-name list) aren't flagged, but a same-named
// ref in the body still is (PR #917 review).
func TestIssue916_CreateHeaderSkippedByPosition(t *testing.T) {
	sql := `CREATE OR REPLACE TASK T1 WAREHOUSE = N_TYPO SCHEDULE = '5 MINUTE' AS
  INSERT INTO SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATION SELECT N_TYPO FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATION;
CREATE OR REPLACE VIEW SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.V (RENAMED, N_TYPO2) AS
  SELECT N_NAME, N_TYPO2 FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATION;`
	markers := ValidateSemantics(sql, nil, []ColEntry{{
		DB: "SNOWFLAKE_SAMPLE_DATA", Schema: "TPCH_SF1", Name: "NATION",
		Cols: []ColInfo{{Name: "N_NATIONKEY"}, {Name: "N_NAME"}},
	}})
	want := map[int]string{2: "'N_TYPO'", 4: "'N_TYPO2'"}
	if len(markers) != len(want) {
		t.Errorf("want %d markers, got %d", len(want), len(markers))
	}
	for _, m := range markers {
		if !strings.Contains(m.Message, want[m.StartLineNumber]) || want[m.StartLineNumber] == "" {
			t.Errorf("unexpected marker line %d: %s", m.StartLineNumber, m.Message)
		}
	}
}

// A CTAS over an earlier in-script table derives its columns in both
// validators, so a typo against it is flagged by each (PR #917 review).
func TestIssue916_ChainedCTASColumns(t *testing.T) {
	sql := "CREATE TABLE A (ID INT);\nCREATE TABLE B AS SELECT * FROM A;\nSELECT ID, TYPO FROM B;"
	ranges := GetStatementRanges(sql)
	sem := ValidateSemantics(sql, nil, nil)
	bare := ValidateBareColumnRefs(ValidateBareColsRequest{SQL: sql, StmtRanges: ranges})
	for name, markers := range map[string][]DiagMarker{"ValidateSemantics": sem, "ValidateBareColumnRefs": bare} {
		if len(markers) != 1 || !strings.Contains(markers[0].Message, "'TYPO'") {
			t.Errorf("%s: want one TYPO marker, got %+v", name, markers)
		}
	}
}

// diag916 runs both column validators over sql.
func diag916(sql string, refs []ResolvedRef, cols []ColEntry) []DiagMarker {
	markers := ValidateSemantics(sql, refs, cols)
	return append(markers, ValidateBareColumnRefs(ValidateBareColsRequest{
		SQL: sql, StmtRanges: GetStatementRanges(sql), ResolvedRefs: refs, ColEntries: cols,
	})...)
}

// TestIssue916_ReviewFollowUps covers the PR #917 review findings: shadowing is
// USE-qualified (no cross-schema collisions), unknown-column CTAS chains stay
// unknown, a partially-unknown FROM skips bare validation, a comma after a
// JOIN condition starts another source, a derived table doesn't desync the
// comma tracker, an ALTER after a USE switch reaches its table, a
// script-global alias doesn't suppress the local fallback, and a CTE wildcard
// over an unknown source stays unknown. want lists the column names that must
// be flagged (every marker must name one of them).
func TestIssue916_ReviewFollowUps(t *testing.T) {
	prod := func(name string, cols ...string) ([]ResolvedRef, []ColEntry) {
		ci := make([]ColInfo, len(cols))
		for i, c := range cols {
			ci[i] = ColInfo{Name: c}
		}
		return []ResolvedRef{{Alias: name, DB: "DB1", Schema: "PROD", Name: name}},
			[]ColEntry{{DB: "DB1", Schema: "PROD", Name: name, Cols: ci}}
	}
	custRefs, custCols := prod("CUSTOMERS", "ID", "NAME")
	logRefs, logCols := prod("LOGS", "MSG")
	tRefs, tCols := prod("T", "Y")
	tRefs[0].Alias = "t"
	xRefs, xCols := prod("CAT", "A")
	xRefs[0].Alias = "x"
	otherRefs, otherCols := prod("OTHER_TBL", "X")
	otherRefs[0].Alias = "t"

	cases := []struct {
		name string
		sql  string
		refs []ResolvedRef
		cols []ColEntry
		want []string
	}{
		{"other-schema table doesn't shadow (false positive)",
			"CREATE TABLE STAGING.CUSTOMERS (ID INT);\nUSE SCHEMA PROD;\nSELECT NAME FROM CUSTOMERS;",
			custRefs, custCols, nil},
		{"other-schema unknown CTAS doesn't shadow (false negative)",
			"CREATE TABLE STAGING.LOGS AS SELECT * FROM SRC;\nUSE SCHEMA PROD;\nSELECT TYPO_COL FROM LOGS;",
			logRefs, logCols, []string{"TYPO_COL"}},
		{"other-schema table doesn't take over a resolved alias",
			"CREATE TABLE DEV.T (X INT);\nSELECT t.Y FROM DB1.PROD.T t;",
			tRefs, tCols, nil},
		{"CTAS chained over an unknown-columns CTAS stays unknown",
			"CREATE TABLE A AS SELECT * FROM SRC;\nCREATE TABLE B AS SELECT *, 1 AS X FROM A;\nSELECT INHERITED, b.INHERITED FROM B b;",
			nil, nil, nil},
		{"unknown CTAS among known sources skips bare validation",
			"CREATE TABLE ORDERS AS SELECT * FROM SRC;\nCREATE TABLE CUSTOMERS (ID INT, NAME VARCHAR);\nSELECT TOTAL, NAME FROM ORDERS o, CUSTOMERS c;",
			nil, nil, nil},
		{"CTAS wildcard over a qualified source ignores a same-named table elsewhere",
			"CREATE TABLE STAGING.ORDERS (X INT);\nCREATE TABLE ANALYTICS.SUMMARY AS SELECT * FROM PROD.S.ORDERS;\nSELECT A FROM ANALYTICS.SUMMARY;",
			nil, nil, nil},
		{"CTAS over a UNION takes columns from the first branch only",
			"CREATE TABLE A (X INT);\nCREATE TABLE B (Y INT);\nCREATE TABLE C AS SELECT * FROM A UNION SELECT * FROM B;\nSELECT Y FROM C;",
			nil, nil, []string{"Y"}},
		{"unknown in-script table under another qualification owns its alias",
			"USE SCHEMA S1;\nCREATE TABLE T AS SELECT * FROM SRC;\nUSE SCHEMA S2;\nSELECT x.COL FROM T x;\nSELECT x.A FROM DB1.PROD.CAT x;",
			xRefs, xCols, nil},
		{"comma after JOIN condition starts another source",
			"CREATE TABLE A (X INT);\nCREATE TABLE B (X INT);\nCREATE TABLE C (Y INT);\nSELECT c.Y, c.TYPO FROM A JOIN B ON A.X = B.X, C c;",
			nil, nil, []string{"TYPO"}},
		// Second-round review findings.
		{"derived-table source doesn't desync the comma tracker",
			"CREATE TABLE INNER_TBL (P INT, Q INT);\nCREATE TABLE OTHER_TABLE (M INT, N INT);\n" +
				"CREATE TABLE T2 AS SELECT * FROM (SELECT P FROM INNER_TBL) x, OTHER_TABLE;\nSELECT N FROM T2;",
			nil, nil, nil},
		{"comma source after a derived table is read",
			"CREATE TABLE INNER_TBL (P INT);\nCREATE TABLE OTHER_TABLE (M INT, N INT);\n" +
				"SELECT o.N, o.TYPO FROM (SELECT P FROM INNER_TBL) x, OTHER_TABLE o;",
			nil, nil, []string{"TYPO"}},
		{"ALTER after a USE switch still reaches the table created earlier",
			"USE SCHEMA A;\nCREATE TABLE FOO (A INT);\nUSE SCHEMA B;\nALTER TABLE FOO ADD COLUMN C INT;\nUSE SCHEMA A;\nSELECT C, TYPO FROM FOO;",
			nil, nil, []string{"TYPO"}},
		{"an earlier statement's alias doesn't suppress the local fallback",
			"SELECT X FROM DB1.PROD.OTHER_TBL AS t;\nCREATE TABLE DEV.T (A INT);\nSELECT A, TYPO FROM T;",
			otherRefs, otherCols, []string{"TYPO"}},
		{"CTE wildcard over an unknown source stays unknown",
			"CREATE TABLE KNOWN_TBL (K INT);\nWITH c AS (SELECT * FROM KNOWN_TBL, UNRESOLVED_TBL) SELECT c.ONLY_ON_UNRESOLVED FROM c;",
			nil, nil, nil},
	}
	for _, tc := range cases {
		markers := diag916(tc.sql, tc.refs, tc.cols)
		flagged := map[string]bool{}
		for _, m := range markers {
			ok := false
			for _, w := range tc.want {
				if strings.Contains(m.Message, "'"+w+"'") {
					ok, flagged[w] = true, true
				}
			}
			if !ok {
				t.Errorf("%s: unexpected marker line %d: %s", tc.name, m.StartLineNumber, m.Message)
			}
		}
		for _, w := range tc.want {
			if !flagged[w] {
				t.Errorf("%s: %s not flagged", tc.name, w)
			}
		}
	}
}

// A comma-joined source is existence-checked too, not only the first one after
// FROM (PR #917 review).
func TestIssue916_CommaSourceExistence(t *testing.T) {
	sql := "SELECT * FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATION n, SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.NATIONZ z;"
	markers, err := Diagnose(context.Background(), issue916Provider(false), sql)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if len(markers) != 1 || !strings.Contains(markers[0].Message, "NATIONZ") {
		t.Errorf("want one marker for NATIONZ, got %+v", markers)
	}
}
