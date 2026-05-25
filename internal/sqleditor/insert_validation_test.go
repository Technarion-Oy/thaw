package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_InsertAllFirstOverwrite(t *testing.T) {
	t.Run("valid INSERT ALL queries", func(t *testing.T) {
		validQueries := []string{
			// Unconditional INSERT ALL (no WHEN)
			`INSERT ALL
			   INTO t1 (id, amount) VALUES (id, amount)
			   INTO t2 (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM source`,
			// Unconditional INSERT ALL without VALUES
			`INSERT ALL
			   INTO t1 (id, amount)
			   INTO t2 (id, amount)
			 SELECT id, amount FROM source`,
			// Unconditional INSERT ALL without column list or VALUES
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id, amount FROM source`,
			// Conditional INSERT ALL with WHEN/THEN
			`INSERT ALL
			   WHEN amount > 1000 THEN INTO large_orders (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount)
			   ELSE INTO small_orders (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// Conditional INSERT ALL with VALUES
			`INSERT ALL
			   WHEN amount > 1000 THEN INTO large_orders (id, amount) VALUES (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount) VALUES (id, amount)
			   ELSE INTO small_orders (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT ALL with multiple INTO per WHEN
			`INSERT ALL
			   WHEN status = 'A' THEN INTO t1 INTO t2
			   ELSE INTO t3
			 SELECT id, status FROM source`,
			// INSERT ALL without ELSE
			`INSERT ALL
			   WHEN amount > 0 THEN INTO positive_amounts (id, amount)
			 SELECT id, amount FROM source`,
			// INSERT ALL with fully qualified table names
			`INSERT ALL
			   WHEN x > 0 THEN INTO db.sch.t1 (id) VALUES (id)
			   ELSE INTO db.sch.t2 (id) VALUES (id)
			 SELECT id, x FROM source`,
			// INSERT ALL with quoted identifiers
			`INSERT ALL
			   WHEN x > 0 THEN INTO "MY_TABLE" (id)
			 SELECT id, x FROM source`,
			// INSERT OVERWRITE ALL (unconditional)
			`INSERT OVERWRITE ALL
			   INTO t1
			   INTO t2
			 SELECT id FROM source`,
			// INSERT OVERWRITE ALL (conditional)
			`INSERT OVERWRITE ALL
			   WHEN x > 0 THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM source`,
			// CASE WHEN/ELSE in trailing SELECT must not trigger false positive
			`INSERT ALL
			   INTO t1 (id, label)
			   INTO t2 (id, label)
			 SELECT id, CASE WHEN status = 1 THEN 'active' ELSE 'inactive' END AS label FROM source`,
			// Subquery with WHEN/ELSE in trailing SELECT
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id, (SELECT CASE WHEN x > 0 THEN 1 ELSE 0 END FROM y) AS flag FROM source`,
			// String literal containing WHEN/ELSE
			`INSERT ALL
			   INTO t1 (id, val)
			 SELECT id, 'WHEN ELSE SELECT INTO' AS val FROM source`,
			// CTE with INSERT ALL (WITH ... SELECT)
			`INSERT ALL
			   INTO t1
			   INTO t2
			 WITH cte AS (SELECT id, amount FROM raw_data) SELECT * FROM cte`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("valid INSERT FIRST queries", func(t *testing.T) {
		validQueries := []string{
			// Basic INSERT FIRST
			`INSERT FIRST
			   WHEN amount > 1000 THEN INTO large_orders (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount)
			   ELSE INTO small_orders (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT FIRST with VALUES
			`INSERT FIRST
			   WHEN amount > 1000 THEN INTO large_orders (id, amount) VALUES (id, amount)
			   WHEN amount > 100 THEN INTO medium_orders (id, amount) VALUES (id, amount)
			   ELSE INTO small_orders (id, amount) VALUES (id, amount)
			 SELECT id, amount FROM raw_orders`,
			// INSERT FIRST without ELSE
			`INSERT FIRST
			   WHEN amount > 0 THEN INTO positive_amounts (id)
			 SELECT id, amount FROM source`,
			// INSERT FIRST with multiple WHEN branches
			`INSERT FIRST
			   WHEN status = 'A' THEN INTO t1
			   WHEN status = 'B' THEN INTO t2
			   WHEN status = 'C' THEN INTO t3
			   ELSE INTO t_other
			 SELECT id, status FROM source`,
			// INSERT OVERWRITE FIRST
			`INSERT OVERWRITE FIRST
			   WHEN x > 0 THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("valid INSERT OVERWRITE queries", func(t *testing.T) {
		validQueries := []string{
			// Basic INSERT OVERWRITE INTO
			`INSERT OVERWRITE INTO t1 SELECT * FROM source`,
			// INSERT OVERWRITE INTO with column list
			`INSERT OVERWRITE INTO t1 (id, name) SELECT id, name FROM source`,
			// INSERT OVERWRITE INTO with VALUES
			`INSERT OVERWRITE INTO t1 (id) VALUES (1)`,
			// INSERT OVERWRITE INTO with fully qualified table
			`INSERT OVERWRITE INTO db.sch.t1 SELECT * FROM source`,
			// INSERT OVERWRITE INTO with multiple VALUES rows
			`INSERT OVERWRITE INTO t1 (id, name) VALUES (1, 'a'), (2, 'b')`,
			// INSERT OVERWRITE INTO with subquery in SELECT
			`INSERT OVERWRITE INTO t1 SELECT id, (SELECT MAX(val) FROM t2) FROM source`,
			// INSERT OVERWRITE INTO with CTE source
			`INSERT OVERWRITE INTO t1 WITH cte AS (SELECT id FROM source) SELECT * FROM cte`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT ALL queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT ALL without any INTO clause
			{
				sql:     `INSERT ALL SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// INSERT ALL without trailing SELECT
			{
				sql:     `INSERT ALL INTO t1 (id) VALUES (1)`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
			// Conditional INSERT ALL without WHEN branches
			{
				sql: `INSERT ALL
				   ELSE INTO t1
				 SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one WHEN branch",
			},
			// INSERT ALL with WHEN but no THEN INTO
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN
				 SELECT id, x FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			// INSERT OVERWRITE ALL without any INTO
			{
				sql:     `INSERT OVERWRITE ALL SELECT * FROM source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// INSERT OVERWRITE ALL without trailing SELECT
			{
				sql:     `INSERT OVERWRITE ALL INTO t1 (id) VALUES (1)`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
			// Conditional INSERT ALL without trailing SELECT
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN INTO t1`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
			// Bare INSERT ALL with nothing else
			{
				sql:     `INSERT ALL`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// Multiple WHEN branches where one is missing THEN INTO
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN INTO t1
				   WHEN y > 0 THEN
				 SELECT id, x, y FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			// INTO keyword only inside a string literal (should not count as real INTO)
			{
				sql:     `INSERT ALL 'INTO t1' SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// Trailing SELECT only inside parentheses (not top-level)
			{
				sql:     `INSERT ALL INTO t1 INTO t2 (SELECT id FROM source)`,
				wantMsg: "INSERT ALL requires a source SELECT",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT FIRST queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT FIRST without WHEN branches
			{
				sql:     `INSERT FIRST INTO t1 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// INSERT FIRST with ELSE only (no WHEN)
			{
				sql: `INSERT FIRST
				   ELSE INTO t1
				 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// INSERT FIRST without trailing SELECT
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN INTO t1`,
				wantMsg: "INSERT FIRST requires a source SELECT",
			},
			// INSERT OVERWRITE FIRST without WHEN branches
			{
				sql:     `INSERT OVERWRITE FIRST INTO t1 SELECT id FROM source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// INSERT FIRST with WHEN but no THEN INTO
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN
				 SELECT id, x FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			// INSERT OVERWRITE FIRST without trailing SELECT
			{
				sql: `INSERT OVERWRITE FIRST
				   WHEN x > 0 THEN INTO t1`,
				wantMsg: "INSERT FIRST requires a source SELECT",
			},
			// Bare INSERT FIRST with nothing else
			{
				sql:     `INSERT FIRST`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			// Multiple WHEN branches where second is missing THEN INTO
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN INTO t1
				   WHEN y > 0 THEN
				 SELECT id, x, y FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("case insensitivity", func(t *testing.T) {
		validQueries := []string{
			// Lowercase INSERT ALL
			`insert all
			   into t1 (id) values (id)
			   into t2 (id) values (id)
			 select id from source`,
			// Mixed case INSERT FIRST
			`Insert First
			   When amount > 100 Then Into t1 (id)
			 Select id, amount From source`,
			// Lowercase INSERT OVERWRITE INTO
			`insert overwrite into t1 select * from source`,
			// Mixed case INSERT OVERWRITE ALL
			`Insert Overwrite All
			   Into t1
			   Into t2
			 Select id From source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}

		// Invalid lowercase should still produce warnings
		invalidCases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     `insert all select id from source`,
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			{
				sql:     `insert first into t1 select id from source`,
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			{
				sql:     `insert overwrite t1 select * from source`,
				wantMsg: "INSERT OVERWRITE requires INTO",
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("comments inside INSERT statements", func(t *testing.T) {
		validQueries := []string{
			// Line comment between INSERT ALL and INTO
			"INSERT ALL -- distribute rows\n   INTO t1\n   INTO t2\n SELECT id FROM source",
			// Block comment before INTO
			"INSERT ALL /* target tables */ INTO t1 INTO t2 SELECT id FROM source",
			// Line comment inside conditional form
			"INSERT ALL\n   WHEN x > 0 THEN INTO t1 -- positive\n   ELSE INTO t2 -- negative\n SELECT id, x FROM source",
			// Block comment inside INSERT OVERWRITE
			"INSERT OVERWRITE /* full replace */ INTO t1 SELECT * FROM source",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("multi-statement context", func(t *testing.T) {
		// Valid INSERT ALL as second statement
		sql := "SELECT 1;\nINSERT ALL\n   INTO t1\n   INTO t2\n SELECT id FROM source"
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for valid INSERT ALL in multi-statement:\n%s\ngot: %v", sql, warns)
		}

		// Invalid INSERT ALL (no INTO) as second statement should still warn
		sql2 := "SELECT 1;\nINSERT ALL SELECT id FROM source"
		stmtRanges2 := GetStatementRanges(sql2)
		markers2 := ValidateSnowflakePatterns(sql2, stmtRanges2)
		warns2 := getWarnings(markers2)
		found := false
		for _, w := range warns2 {
			if strings.Contains(w.Message, "INSERT ALL requires at least one INTO clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected warning for invalid INSERT ALL in multi-statement:\n%s\ngot: %v", sql2, warns2)
		}
	})

	t.Run("nested parentheses and subqueries in WHEN conditions", func(t *testing.T) {
		validQueries := []string{
			// Nested parentheses in WHEN condition
			`INSERT ALL
			   WHEN (x > 0 AND (y < 10)) THEN INTO t1
			 SELECT id, x, y FROM source`,
			// Subquery in WHEN condition (SELECT inside parens should not be trailing SELECT)
			`INSERT ALL
			   WHEN x > (SELECT MIN(val) FROM thresholds) THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM source`,
			// INTO inside a subquery in WHEN condition should not count as multi-table INTO
			`INSERT FIRST
			   WHEN x > (SELECT val FROM t WHERE INTO_COL = 1) THEN INTO t1
			 SELECT id, x FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("non-regression: plain INSERT INTO is not flagged", func(t *testing.T) {
		validQueries := []string{
			`INSERT INTO t1 SELECT * FROM source`,
			`INSERT INTO t1 (id, name) VALUES (1, 'a')`,
			`INSERT INTO t1 (id) VALUES (1), (2), (3)`,
			`INSERT INTO db.sch.t1 SELECT id FROM source`,
			`insert into t1 select * from source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Plain INSERT INTO should not produce warnings:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("word boundary: identifiers containing keyword substrings", func(t *testing.T) {
		// Identifiers like WHENEVER, INTOXICATED, ELSEWHERE, SELECTIVITY
		// contain WHEN, INTO, ELSE, SELECT as substrings but must NOT be
		// treated as those keywords.
		validQueries := []string{
			// Column named WHENEVER should not be confused with WHEN
			`INSERT ALL
			   INTO t1 (id, whenever)
			   INTO t2 (id, whenever)
			 SELECT id, whenever FROM source`,
			// Table named SELECTIVITY should not confuse trailing SELECT detection
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id FROM selectivity_stats`,
			// Column named INTOLERANCE should not be confused with INTO
			`INSERT ALL
			   INTO t1 (id, intolerance)
			   INTO t2 (id, intolerance)
			 SELECT id, intolerance FROM source`,
			// Table named ELSEWHERE should not be confused with ELSE
			`INSERT ALL
			   WHEN x > 0 THEN INTO t1
			 SELECT id, x FROM elsewhere_data`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Keyword-like identifiers should not cause false positives:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("compound trailing SELECT (UNION, INTERSECT)", func(t *testing.T) {
		validQueries := []string{
			// INSERT ALL with UNION ALL source
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id FROM a UNION ALL SELECT id FROM b`,
			// INSERT ALL with INTERSECT source
			`INSERT ALL
			   INTO t1
			   INTO t2
			 SELECT id FROM a INTERSECT SELECT id FROM b`,
			// INSERT FIRST with UNION source
			`INSERT FIRST
			   WHEN x > 0 THEN INTO t1
			   ELSE INTO t2
			 SELECT id, x FROM a UNION SELECT id, x FROM b`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Compound trailing SELECT should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("first WHEN branch missing THEN INTO", func(t *testing.T) {
		// Existing tests cover the last branch missing INTO; verify the
		// first branch is also caught when subsequent branches are valid.
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN
				   WHEN y > 0 THEN INTO t2
				 SELECT id, x, y FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN
				   WHEN y > 0 THEN INTO t2
				 SELECT id, x, y FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("INSERT OVERWRITE INTO with quoted and qualified identifiers", func(t *testing.T) {
		validQueries := []string{
			`INSERT OVERWRITE INTO "my_table" SELECT * FROM source`,
			`INSERT OVERWRITE INTO db."my_schema".t1 SELECT * FROM source`,
			`INSERT OVERWRITE INTO "DB"."SCHEMA"."TABLE" (id) VALUES (1)`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid INSERT OVERWRITE queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INSERT OVERWRITE without INTO
			{
				sql:     `INSERT OVERWRITE t1 SELECT * FROM source`,
				wantMsg: "INSERT OVERWRITE requires INTO",
			},
			// INSERT OVERWRITE INTO without source
			{
				sql:     `INSERT OVERWRITE INTO t1`,
				wantMsg: "INSERT OVERWRITE INTO requires a source SELECT or VALUES",
			},
			// INSERT OVERWRITE INTO with column list but no source
			{
				sql:     `INSERT OVERWRITE INTO t1 (id, name)`,
				wantMsg: "INSERT OVERWRITE INTO requires a source SELECT or VALUES",
			},
			// Bare INSERT OVERWRITE with no table or INTO
			{
				sql:     `INSERT OVERWRITE`,
				wantMsg: "INSERT OVERWRITE requires INTO",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("multiple diagnostics for one statement", func(t *testing.T) {
		// A conditional INSERT ALL with a WHEN branch missing THEN INTO
		// and also missing a trailing SELECT should produce two warnings.
		cases := []struct {
			sql      string
			wantMsgs []string
		}{
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN`,
				wantMsgs: []string{
					"WHEN branch must contain INTO clause",
					"INSERT ALL requires a source SELECT",
				},
			},
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN`,
				wantMsgs: []string{
					"WHEN branch must contain INTO clause",
					"INSERT FIRST requires a source SELECT",
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				for _, wantMsg := range tc.wantMsgs {
					found := false
					for _, w := range warns {
						if strings.Contains(w.Message, wantMsg) {
							found = true
						}
					}
					if !found {
						t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", wantMsg, tc.sql, warns)
					}
				}
			})
		}
	})

	t.Run("tab and mixed whitespace between keywords", func(t *testing.T) {
		validQueries := []string{
			// Tabs between INSERT and ALL
			"INSERT\tALL\n\tINTO t1\n\tINTO t2\n SELECT id FROM source",
			// Multiple spaces and tabs
			"INSERT  \t ALL\n   INTO t1\n   INTO t2\n SELECT id FROM source",
			// Tabs in INSERT FIRST
			"INSERT\tFIRST\n\tWHEN x > 0 THEN INTO t1\n SELECT id, x FROM source",
			// Tabs in INSERT OVERWRITE INTO
			"INSERT\tOVERWRITE\tINTO t1 SELECT * FROM source",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Tab/mixed whitespace should not cause warnings:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("newlines between keywords", func(t *testing.T) {
		validQueries := []string{
			// Newline between INSERT and ALL
			"INSERT\nALL\n   INTO t1\n   INTO t2\n SELECT id FROM source",
			// Newline between INSERT and FIRST with WHEN
			"INSERT\nFIRST\n   WHEN x > 0 THEN INTO t1\n SELECT id, x FROM source",
			// Newline between INSERT, OVERWRITE, and INTO
			"INSERT\nOVERWRITE\nINTO t1 SELECT * FROM source",
			// THEN and INTO on separate lines in WHEN branch
			"INSERT ALL\n   WHEN x > 0 THEN\n   INTO t1\n SELECT id, x FROM source",
			// Multiple newlines between keywords
			"INSERT\n\nALL\n   INTO t1\n   INTO t2\n SELECT id FROM source",
			// INSERT OVERWRITE ALL with newlines between all keywords
			"INSERT\nOVERWRITE\nALL\n   INTO t1\n   INTO t2\n SELECT id FROM source",
			// INSERT OVERWRITE FIRST with newlines
			"INSERT\nOVERWRITE\nFIRST\n   WHEN x > 0 THEN\n   INTO t1\n SELECT id, x FROM source",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Newlines between keywords should not cause warnings:\n%s\ngot: %v", sql, warns)
				}
			})
		}

		// Invalid cases with newlines should still produce warnings
		invalidCases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "INSERT\nALL\nSELECT id FROM source",
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			{
				sql:     "INSERT\nFIRST\nINTO t1\nSELECT id FROM source",
				wantMsg: "INSERT FIRST requires at least one WHEN branch",
			},
			{
				sql:     "INSERT\nOVERWRITE\nt1 SELECT * FROM source",
				wantMsg: "INSERT OVERWRITE requires INTO",
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("CTE without trailing top-level SELECT", func(t *testing.T) {
		// WITH ... AS (SELECT ...) has SELECT only inside parens (depth > 0),
		// so findLastTopLevelSelectPos returns -1 and the validator should
		// flag the missing source SELECT.
		sql := `INSERT ALL INTO t1 WITH cte AS (SELECT id FROM source)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "INSERT ALL requires a source SELECT") {
				found = true
			}
		}
		if !found {
			t.Errorf("CTE with SELECT only inside parens should flag missing source SELECT:\n%s\ngot: %v", sql, warns)
		}
	})

	t.Run("WHEN/ELSE inside VALUES parens not treated as conditional", func(t *testing.T) {
		// WHEN and ELSE inside parenthesized VALUES are at depth > 0 and
		// must not trigger the conditional INSERT ALL form.
		validQueries := []string{
			// CASE WHEN/ELSE inside VALUES
			`INSERT ALL
			   INTO t1 (id) VALUES (CASE WHEN x > 0 THEN 1 ELSE 0 END)
			   INTO t2 (id) VALUES (id)
			 SELECT id, x FROM source`,
			// Nested function call with ELSE-like content in VALUES
			`INSERT ALL
			   INTO t1 (val) VALUES (IFF(x > 0, 1, 0))
			   INTO t2 (val) VALUES (val)
			 SELECT val, x FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("WHEN/ELSE inside VALUES parens should not cause warnings:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("keywords only inside comments are stripped before validation", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// INTO only in a line comment — should warn about missing INTO
			{
				sql:     "INSERT ALL -- INTO t1\n SELECT id FROM source",
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// INTO only in a block comment — should warn about missing INTO
			{
				sql:     "INSERT ALL /* INTO t1 INTO t2 */ SELECT id FROM source",
				wantMsg: "INSERT ALL requires at least one INTO clause",
			},
			// SELECT only in a block comment — should warn about missing source SELECT
			{
				sql:     "INSERT ALL INTO t1 INTO t2 /* SELECT id FROM source */",
				wantMsg: "INSERT ALL requires a source SELECT",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("SELECT inside string literal is not trailing SELECT", func(t *testing.T) {
		// After string literal stripping, the SELECT disappears, so the
		// validator should flag missing source SELECT.
		sql := `INSERT ALL INTO t1 INTO t2 'SELECT id FROM source'`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "INSERT ALL requires a source SELECT") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected warning about missing source SELECT when SELECT is only inside string literal:\n%s\ngot: %v", sql, warns)
		}
	})

	t.Run("all WHEN branches missing THEN INTO produces one marker per branch", func(t *testing.T) {
		cases := []struct {
			sql       string
			wantCount int
		}{
			// Two WHEN branches, both missing THEN INTO → 2 markers
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN
				   WHEN y > 0 THEN
				 SELECT id, x, y FROM source`,
				wantCount: 2,
			},
			// Three WHEN branches, all missing THEN INTO → 3 markers
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN
				   WHEN y > 0 THEN
				   WHEN z > 0 THEN
				 SELECT id, x, y, z FROM source`,
				wantCount: 3,
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				count := 0
				for _, w := range warns {
					if strings.Contains(w.Message, "WHEN branch must contain INTO clause") {
						count++
					}
				}
				if count != tc.wantCount {
					t.Errorf("Expected %d 'WHEN branch must contain INTO clause' markers, got %d for:\n%s\nmarkers: %v",
						tc.wantCount, count, tc.sql, warns)
				}
			})
		}
	})

	t.Run("INTO only inside parentheses is not counted as top-level INTO", func(t *testing.T) {
		// INTO at depth > 0 should be ignored by the scanner; the statement
		// should produce a "requires at least one INTO clause" warning.
		sql := `INSERT ALL (INTO t1) SELECT id FROM source`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "INSERT ALL requires at least one INTO clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("INTO inside parentheses should not count; expected missing INTO warning:\n%s\ngot: %v", sql, warns)
		}
	})

	t.Run("WHEN without space before parenthesis is recognized", func(t *testing.T) {
		// WHEN( without a space — '(' is not a word char, so the word
		// boundary check should still recognize the WHEN keyword.
		validQueries := []string{
			`INSERT ALL
			   WHEN(x > 0) THEN INTO t1
			 SELECT id, x FROM source`,
			`INSERT FIRST
			   WHEN(amount > 100) THEN INTO t1
			   ELSE INTO t2
			 SELECT id, amount FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("WHEN( without space should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("INSERT OVERWRITE INTO with nested parentheses in VALUES", func(t *testing.T) {
		validQueries := []string{
			// Nested expression parens in VALUES
			`INSERT OVERWRITE INTO t1 (id) VALUES ((1 + 2))`,
			// Subquery in VALUES
			`INSERT OVERWRITE INTO t1 (id) VALUES ((SELECT MAX(id) FROM t2))`,
			// Multiple nested parens
			`INSERT OVERWRITE INTO t1 (a, b) VALUES ((1+2), (3*(4+5)))`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Nested parens in VALUES should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("unconditional INSERT ALL with single INTO", func(t *testing.T) {
		// A single INTO in unconditional INSERT ALL is valid Snowflake syntax.
		validQueries := []string{
			`INSERT ALL
			   INTO t1
			 SELECT id FROM source`,
			`INSERT ALL
			   INTO t1 (id, name) VALUES (id, name)
			 SELECT id, name FROM source`,
			`INSERT OVERWRITE ALL
			   INTO t1
			 SELECT id FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Single INTO in unconditional INSERT ALL should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("middle WHEN branch missing THEN INTO", func(t *testing.T) {
		// Three WHEN branches where only the middle one is missing THEN INTO.
		// Verifies that the clause slicing (upper[whenPos:whenPositions[i+1]])
		// correctly isolates middle branches.
		cases := []struct {
			sql       string
			wantMsg   string
			wantCount int
		}{
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN INTO t1
				   WHEN y > 0 THEN
				   WHEN z > 0 THEN INTO t3
				 SELECT id, x, y, z FROM source`,
				wantMsg:   "WHEN branch must contain INTO clause",
				wantCount: 1,
			},
			{
				sql: `INSERT FIRST
				   WHEN x > 0 THEN INTO t1
				   WHEN y > 0 THEN
				   WHEN z > 0 THEN INTO t3
				 SELECT id, x, y, z FROM source`,
				wantMsg:   "WHEN branch must contain INTO clause",
				wantCount: 1,
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				count := 0
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						count++
					}
				}
				if count != tc.wantCount {
					t.Errorf("Expected exactly %d '%s' marker for:\n%s\ngot %d, markers: %v",
						tc.wantCount, tc.wantMsg, tc.sql, count, warns)
				}
			})
		}
	})

	t.Run("ELSE with plain INTO but no WHEN triggers conditional path", func(t *testing.T) {
		// INSERT ALL with INTO and ELSE but no WHEN: the presence of ELSE
		// forces the conditional path, which requires at least one WHEN branch.
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql: `INSERT ALL
				   INTO t1
				   ELSE INTO t2
				 SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one WHEN branch",
			},
			{
				sql: `INSERT ALL
				   INTO t1 (id)
				   INTO t2 (id)
				   ELSE INTO t3 (id)
				 SELECT id FROM source`,
				wantMsg: "INSERT ALL requires at least one WHEN branch",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("INSERT OVERWRITE INTO with empty column list", func(t *testing.T) {
		// Empty column list () followed by VALUES — tests findMatchingParen
		// returning index 1 for "()" which passes the endIdx > 0 check.
		validQueries := []string{
			`INSERT OVERWRITE INTO t1 () VALUES (1)`,
			`INSERT OVERWRITE INTO t1 () SELECT * FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Empty column list should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("trailing whitespace after final SELECT", func(t *testing.T) {
		// Trailing whitespace/newlines after the SELECT clause should not
		// affect detection of the trailing top-level SELECT.
		validQueries := []string{
			"INSERT ALL\n   INTO t1\n   INTO t2\n SELECT id FROM source   ",
			"INSERT ALL\n   INTO t1\n   INTO t2\n SELECT id FROM source\n\n",
			"INSERT FIRST\n   WHEN x > 0 THEN INTO t1\n SELECT id, x FROM source\t\t",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Trailing whitespace should not cause warnings:\n%q\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("INSERT OVERWRITE INTO with unmatched paren in column list", func(t *testing.T) {
		// When column list paren is never closed, findMatchingParen returns -1
		// and the code skips stripping. The leftover text starting with "("
		// doesn't match SELECT/VALUES so we get the missing-source warning.
		sql := `INSERT OVERWRITE INTO t1 (id, name`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "INSERT OVERWRITE INTO requires a source SELECT or VALUES") {
				found = true
			}
		}
		if !found {
			t.Errorf("Unmatched paren should trigger missing source warning:\n%s\ngot: %v", sql, warns)
		}
	})

	t.Run("WHEN branch with INTO but missing THEN keyword", func(t *testing.T) {
		// reInsertMultiThenInto requires \bTHEN\s+INTO\b — omitting THEN
		// should still flag the branch as invalid even though INTO is present.
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql: `INSERT ALL
				   WHEN x > 0 INTO t1
				 SELECT id, x FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			{
				sql: `INSERT FIRST
				   WHEN x > 0 INTO t1
				 SELECT id, x FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
			// Multiple branches, second has INTO without THEN
			{
				sql: `INSERT ALL
				   WHEN x > 0 THEN INTO t1
				   WHEN y > 0 INTO t2
				 SELECT id, x, y FROM source`,
				wantMsg: "WHEN branch must contain INTO clause",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for:\n%s\ngot: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})

	t.Run("comment between THEN and INTO in WHEN branch", func(t *testing.T) {
		// After comment stripping, "THEN -- comment\nINTO" becomes "THEN \nINTO"
		// which matches \bTHEN\s+INTO\b. This should be valid.
		validQueries := []string{
			// Line comment between THEN and INTO
			"INSERT ALL\n   WHEN x > 0 THEN -- target table\n   INTO t1\n SELECT id, x FROM source",
			// Block comment between THEN and INTO
			"INSERT ALL\n   WHEN x > 0 THEN /* route here */ INTO t1\n SELECT id, x FROM source",
			// INSERT FIRST with comment between THEN and INTO
			"INSERT FIRST\n   WHEN x > 0 THEN -- positive\n   INTO t1\n   ELSE INTO t2\n SELECT id, x FROM source",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Comment between THEN and INTO should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("escaped single quotes in string literals containing keywords", func(t *testing.T) {
		// reStripStringLiterals matches '(?:''|[^'])*' — doubled single quotes
		// must not break the stripping, leaving keywords exposed.
		validQueries := []string{
			// Escaped quote with keyword-like content
			`INSERT ALL
			   INTO t1 (id, val)
			   INTO t2 (id, val)
			 SELECT id, 'it''s INTO the SELECT' AS val FROM source`,
			// Multiple escaped quotes surrounding keywords
			`INSERT ALL
			   INTO t1 (name)
			 SELECT 'WHEN ''status'' THEN ELSE' AS name FROM source`,
			// INSERT OVERWRITE with escaped quote in VALUES
			`INSERT OVERWRITE INTO t1 (name) VALUES ('can''t SELECT this')`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Escaped quotes in strings should not cause false positives:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})

	t.Run("multiple INSERT multi-table statements in same SQL", func(t *testing.T) {
		// Two valid INSERT ALL statements separated by semicolon should
		// both pass without warnings (no state leakage between statements).
		sql := "INSERT ALL\n   INTO t1\n   INTO t2\n SELECT id FROM a;\nINSERT FIRST\n   WHEN x > 0 THEN INTO t3\n   ELSE INTO t4\n SELECT id, x FROM b"
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Two valid INSERT multi-table statements should produce no warnings:\n%s\ngot: %v", sql, warns)
		}

		// One valid, one invalid — only the invalid one should produce a warning.
		sql2 := "INSERT ALL\n   INTO t1\n   INTO t2\n SELECT id FROM a;\nINSERT ALL SELECT id FROM b"
		stmtRanges2 := GetStatementRanges(sql2)
		markers2 := ValidateSnowflakePatterns(sql2, stmtRanges2)
		warns2 := getWarnings(markers2)
		if len(warns2) != 1 {
			t.Errorf("Expected exactly 1 warning (for the invalid statement), got %d:\n%s\nmarkers: %v", len(warns2), sql2, warns2)
		} else if !strings.Contains(warns2[0].Message, "INSERT ALL requires at least one INTO clause") {
			t.Errorf("Wrong warning message for:\n%s\ngot: %v", sql2, warns2)
		}
	})

	t.Run("INSERT OVERWRITE INTO with keyword-like table name", func(t *testing.T) {
		// Table names that look like or contain SQL keywords should be handled
		// by the identifier-path regex in reInsertOverwritePrefix.
		validQueries := []string{
			`INSERT OVERWRITE INTO select_results SELECT * FROM source`,
			`INSERT OVERWRITE INTO values_archive (id) VALUES (1)`,
			`INSERT OVERWRITE INTO "SELECT" SELECT * FROM source`,
			`INSERT OVERWRITE INTO "VALUES" (id) VALUES (1)`,
			`INSERT OVERWRITE INTO db.schema.into_table SELECT * FROM source`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Keyword-like table name should be valid:\n%s\ngot: %v", sql, warns)
				}
			})
		}
	})
}
