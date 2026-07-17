package sqlgrammar

import "testing"

func TestRepro776(t *testing.T) {
	cases := []string{
		// GET_DDL style: cluster by BEFORE the column list
		"create or replace TABLE LINEAGE_SOURCE_DB.RAW_DATA.BIG_SALES_DATA cluster by (sale_date)(\n\tSALE_ID NUMBER(38,0),\n\tCUSTOMER_ID NUMBER(38,0),\n\tSALE_DATE DATE,\n\tAMOUNT NUMBER(10,2),\n\tNOTES VARCHAR(16777216)\n);",
		// documented style: cluster by AFTER the column list
		"create or replace TABLE t (SALE_DATE DATE) cluster by (sale_date);",
		// GET_DDL with LINEAR
		"create or replace TABLE t cluster by LINEAR(sale_date)(SALE_DATE DATE);",
	}
	for _, sql := range cases {
		v := New(sql)
		if !v.Recognized() {
			t.Errorf("not recognized: %q", sql)
			continue
		}
		if !v.ParseTopLevel() {
			f := v.Failure()
			t.Errorf("FAIL: %s\n  sql: %.60s", f.Message(), sql)
		}
	}
}
