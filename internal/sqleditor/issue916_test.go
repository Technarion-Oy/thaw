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
// tables, comma-joined FROM sources are all recognised, and CREATE TASK header
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
func TestIssue916_CTASUnknownColumnsShadowCatalog(t *testing.T) {
	sql := `CREATE DATABASE COCO_DEMO;
USE SCHEMA COCO_DEMO.PUBLIC;
CREATE OR REPLACE TABLE ORDERS AS SELECT *, 1 AS X FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF1.ORDERS;
SELECT o.ANYTHING, ANYTHING FROM ORDERS o;`
	markers, err := Diagnose(context.Background(), issue916Provider(false), sql)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	for _, m := range markers {
		t.Errorf("unexpected marker line %d: %s", m.StartLineNumber, m.Message)
	}
}
