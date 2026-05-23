package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_AsofJoin(t *testing.T) {
	t.Run("valid ASOF JOIN queries", func(t *testing.T) {
		validQueries := []string{
			// Basic ASOF JOIN with MATCH_CONDITION >=
			`SELECT a.ts, a.val, b.price FROM measurements a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with WHERE clause
			`SELECT a.ts, a.val, b.price FROM measurements a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts) WHERE a.sensor = 'X'`,
			// ASOF JOIN with MATCH_CONDITION using >
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts > t2.ts)`,
			// ASOF JOIN with MATCH_CONDITION using <=
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts <= t2.ts)`,
			// ASOF JOIN with MATCH_CONDITION using <
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts < t2.ts)`,
			// Multiline ASOF JOIN
			`SELECT a.ts, b.price
			 FROM measurements a
			 ASOF JOIN prices b
			   MATCH_CONDITION (a.ts >= b.ts)
			 WHERE a.sensor = 'X'`,
			// ASOF JOIN with fully qualified table names
			`SELECT * FROM db.schema.measurements a ASOF JOIN db.schema.prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with quoted identifiers
			`SELECT * FROM "DB"."SCH"."MEASUREMENTS" a ASOF JOIN "DB"."SCH"."PRICES" b MATCH_CONDITION (a."TS" >= b."TS")`,
			// Multiple columns in MATCH_CONDITION expression
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.event_time >= t2.event_time)`,
			// ASOF JOIN with subquery context
			`SELECT * FROM (SELECT ts, val FROM raw_data) a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// ASOF JOIN with CTE
			`WITH cte AS (SELECT ts, val FROM measurements) SELECT * FROM cte a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts)`,
			// USING FUNCTION form (custom matching logic)
			`SELECT * FROM t1 ASOF JOIN t2 USING (my_match_func(t1.ts, t2.ts))`,
			// USING FUNCTION form with qualified function name
			`SELECT * FROM t1 ASOF JOIN t2 USING (db.schema.my_func(t1.ts, t2.ts))`,
			// Multiple ASOF JOINs in one statement
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) ASOF JOIN t3 MATCH_CONDITION (t1.ts >= t3.ts)`,
			// ASOF JOIN with subquery containing a regular JOIN with ON
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y ON x.id = y.id) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
			// ASOF JOIN with subquery containing a regular JOIN with USING
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y USING (id)) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
			// Table name containing "ON" (e.g. options)
			`SELECT * FROM t1 ASOF JOIN options MATCH_CONDITION (t1.ts >= options.ts)`,
			// Nested ASOF JOIN inside subquery — outer scope must not be truncated
			`SELECT * FROM t1 ASOF JOIN (SELECT * FROM x ASOF JOIN y MATCH_CONDITION (x.ts >= y.ts)) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid ASOF JOIN queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			// Missing MATCH_CONDITION
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 WHERE t1.ts >= t2.ts`,
				wantMsg: "ASOF JOIN requires a MATCH_CONDITION clause",
			},
			// Bare ASOF JOIN without any condition
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2`,
				wantMsg: "ASOF JOIN requires a MATCH_CONDITION clause",
			},
			// ON clause used instead of MATCH_CONDITION
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 ON t1.ts >= t2.ts`,
				wantMsg: "ON clause is not valid with ASOF JOIN",
			},
			// USING column-list clause (not USING FUNCTION)
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 USING (ts)`,
				wantMsg: "USING clause is not valid with ASOF JOIN",
			},
			// Invalid comparison operator: =
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts = t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
			},
			// Invalid comparison operator: <>
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts <> t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
			},
			// Invalid comparison operator: !=
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts != t2.ts)`,
				wantMsg: "MATCH_CONDITION comparison must use one of: >=, >, <=, <",
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
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})

	t.Run("ON/USING without MATCH_CONDITION produces single warning", func(t *testing.T) {
		// When ON or USING is used instead of MATCH_CONDITION, only the
		// ON/USING warning should appear — not a redundant "requires
		// MATCH_CONDITION" warning on top of it.
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 ON t1.ts >= t2.ts`,
				wantMsg: "ON clause is not valid with ASOF JOIN",
			},
			{
				sql:     `SELECT * FROM t1 ASOF JOIN t2 USING (ts)`,
				wantMsg: "USING clause is not valid with ASOF JOIN",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) != 1 {
					t.Errorf("Expected exactly 1 warning, got %d: %v", len(warns), warns)
				}
				if len(warns) > 0 && !strings.Contains(warns[0].Message, tc.wantMsg) {
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})
}

