package sqleditor

import (
	"strings"
	"testing"
)

func antiPatternMarkers(sql string) []DiagMarker {
	return ValidateAntiPatterns(sql, GetStatementRanges(sql))
}

func TestValidateAntiPatterns_Flags(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		{`MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN INSERT (a) VALUES (1)`, "INSERT action is not allowed in WHEN MATCHED"},
		{`MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED THEN UPDATE SET t.a = 1`, "WHEN NOT MATCHED clause"},
		{`MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED BY SOURCE THEN DELETE`, "BY SOURCE is not supported"},
		{`SELECT a FROM t ORDER BY a QUALIFY ROW_NUMBER() OVER (ORDER BY a) = 1`, "QUALIFY"},
		{`SELECT * FROM t, FLATTEN(t.col)`, "FLATTEN used as a table function requires LATERAL"},
		{`SELECT LATERALFLATTEN(x)`, "Did you mean 'LATERAL FLATTEN'"},
		{`SELECT payload.customer.name FROM t`, "Missing colon for variant path"},
		{`SELECT SNOWFLAKE.CORTEX.NOTAFUNC('x')`, "Unknown Cortex function"},
		// Recovered clause-level anti-pattern validators.
		{`SELECT * FROM t PIVOT (SUM(amount))`, "PIVOT requires FOR"},
		{`SELECT * FROM t UNPIVOT (val FOR name IN ())`, "UNPIVOT IN list must not be empty"},
		{`SELECT * FROM t MATCH_RECOGNIZE (PARTITION BY id ORDER BY ts PATTERN (a b))`, "requires a DEFINE clause"},
		{`SELECT * FROM a ASOF JOIN t`, "ASOF JOIN requires a MATCH_CONDITION"},
		{`SELECT * FROM a ASOF JOIN t ON a = b`, "ON clause is not valid with ASOF JOIN"},
		{`INSERT OVERWRITE t SELECT 1`, "INSERT OVERWRITE requires INTO"},
		{`SELECT * FROM t AT TIMESTAMP => '2020-01-01'`, "Time Travel clause requires parentheses"},
		// Stray token / dangling AS after a FROM/JOIN table reference (ported from
		// the removed checkStrayAfterTableRef — not caught by the grammar).
		{`SELECT * FROM t 1000`, "Unexpected token '1000' after table reference"},
		{`SELECT * FROM t AS`, "Expected an alias after AS"},
		{`SELECT * FROM t myalias AS`, "Unexpected 'AS' after the table alias"},
		// Cross-statement transaction tracking.
		{"BEGIN;\nUPDATE t SET a = 1;", "not committed or rolled back"},
		{`COMMIT`, "no open transaction"},
		{"BEGIN;\nUPDATE t SET a = 1;\nBEGIN;\nUPDATE t SET b = 2;\nCOMMIT;", "does not support nested BEGIN"},
	}
	for _, c := range cases {
		m := antiPatternMarkers(c.sql)
		if len(m) == 0 {
			t.Errorf("expected a marker for %q, got none", c.sql)
			continue
		}
		found := false
		for _, k := range m {
			if strings.Contains(k.Message, c.want) {
				found = true
				if k.Severity != SeverityWarning {
					t.Errorf("for %q: expected Warning severity, got %d", c.sql, k.Severity)
				}
			}
		}
		if !found {
			t.Errorf("for %q: no marker contained %q; got %+v", c.sql, c.want, m)
		}
	}
}

func TestValidateAntiPatterns_Clean(t *testing.T) {
	// Valid statements must not be flagged — notably MERGE with correct actions
	// (this is what the PR live-test exercised) and proper QUALIFY/LATERAL usage.
	clean := []string{
		`MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.a = s.a`,
		`MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED THEN INSERT (a) VALUES (s.a)`,
		`SELECT a FROM t QUALIFY ROW_NUMBER() OVER (ORDER BY a) = 1 ORDER BY a`,
		`SELECT * FROM t, LATERAL FLATTEN(input => t.col)`,
		`SELECT * FROM TABLE(FLATTEN(input => parse_json('[]')))`,
		`SELECT SNOWFLAKE.CORTEX.SUMMARIZE('text')`,
		`SELECT a, b FROM t WHERE a = 1 ORDER BY b`,
		`CREATE TABLE t (id INT)`,
		// Well-formed FROM/JOIN table references must NOT trip checkStrayAfterTableRef.
		`SELECT * FROM t`,
		`SELECT * FROM t a`,
		`SELECT * FROM t AS a`,
		`SELECT * FROM t, s`,
		`SELECT * FROM t JOIN s ON t.id = s.id`,
		`SELECT * FROM t WHERE a = 1`,
		`SELECT * FROM t GROUP BY a`,
		`SELECT * FROM db.sch.t alias WHERE alias.x = 1`,
		// Recovered clause-level validators — valid forms must stay clean.
		`SELECT * FROM t PIVOT (SUM(amount) FOR month IN ('jan','feb'))`,
		`SELECT * FROM t UNPIVOT (val FOR name IN (c1, c2))`,
		`SELECT * FROM a ASOF JOIN t MATCH_CONDITION (a.ts >= t.ts)`,
		`INSERT OVERWRITE INTO t SELECT 1`,
		`SELECT * FROM t AT (OFFSET => -60)`,
		"BEGIN;\nUPDATE t SET a = 1;\nCOMMIT;",
	}
	for _, sql := range clean {
		if m := antiPatternMarkers(sql); len(m) != 0 {
			t.Errorf("expected no anti-pattern markers for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

// TestValidateAntiPatterns_Issue710FalsePositives pins the seven confirmed false
// positives from issue #710: valid SQL that must NOT be flagged. Each case failed
// before the fix and is clean after it.
func TestValidateAntiPatterns_Issue710FalsePositives(t *testing.T) {
	clean := []string{
		// 1. Scripting BEGIN…END block with DML must not count as an open (or
		//    nested) transaction.
		"BEGIN\nINSERT INTO t VALUES (1);\nEND;",
		// 2. Canonical QUALIFY: an ORDER BY inside a window OVER (…) does not
		//    out-order a trailing QUALIFY.
		"SELECT a, ROW_NUMBER() OVER (ORDER BY b) AS rn FROM t QUALIFY rn = 1",
		// 3. A table/db literally named `payload` in a FROM position is a qualified
		//    name, not a variant path.
		"SELECT * FROM payload.raw.events",
		// 4. Current Cortex functions, and a quoted function name.
		`SELECT SNOWFLAKE.CORTEX.PARSE_DOCUMENT('x')`,
		`SELECT SNOWFLAKE.CORTEX.ENTITY_SENTIMENT('x')`,
		`SELECT SNOWFLAKE.CORTEX."COMPLETE"('x')`,
		// 5. ASOF JOIN with a valid USING after MATCH_CONDITION.
		"SELECT * FROM a ASOF JOIN b MATCH_CONDITION (a.ts >= b.ts) USING (id)",
		// 6. Non-reserved keyword aliases after AS.
		"SELECT * FROM t AS key",
		"SELECT * FROM t AS first",
		// 7. A column named `flatten` after a comma is not a table function.
		"SELECT a, flatten FROM t",
	}
	for _, sql := range clean {
		if m := antiPatternMarkers(sql); len(m) != 0 {
			t.Errorf("expected no anti-pattern markers for %q, got %d: %+v", sql, len(m), m)
		}
	}
}
