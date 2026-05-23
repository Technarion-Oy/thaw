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
}

