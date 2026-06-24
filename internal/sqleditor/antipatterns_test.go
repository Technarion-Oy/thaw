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
	}
	for _, sql := range clean {
		if m := antiPatternMarkers(sql); len(m) != 0 {
			t.Errorf("expected no anti-pattern markers for %q, got %d: %+v", sql, len(m), m)
		}
	}
}
