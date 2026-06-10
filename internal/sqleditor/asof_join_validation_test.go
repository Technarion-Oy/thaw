package sqleditor

import (
	"strings"
	"testing"

	"thaw/internal/sqltok"
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

	t.Run("case insensitivity", func(t *testing.T) {
		validQueries := []string{
			// Lowercase keywords
			`select * from t1 asof join t2 match_condition (t1.ts >= t2.ts)`,
			// Mixed case
			`SELECT * FROM t1 Asof Join t2 Match_Condition (t1.ts >= t2.ts)`,
			// Lowercase USING FUNCTION form
			`select * from t1 asof join t2 using (my_func(t1.ts, t2.ts))`,
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
		// Lowercase invalid: ON clause
		sql := `select * from t1 asof join t2 on t1.ts >= t2.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected 1 warning for lowercase ON, got %d: %v", len(warns), warns)
		}
	})

	t.Run("ASOF JOIN inside string literal is ignored", func(t *testing.T) {
		// The string 'ASOF JOIN' inside a literal should not trigger validation.
		sql := `SELECT * FROM t1 WHERE comment = 'ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)'`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN inside string literal, got: %v", warns)
		}
	})

	t.Run("multiple ASOF JOINs with one invalid", func(t *testing.T) {
		// First ASOF JOIN is valid, second is missing MATCH_CONDITION.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) ASOF JOIN t3`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected warning for the second ASOF JOIN missing MATCH_CONDITION, got: %v", warns)
		}
	})

	t.Run("multiple ASOF JOINs with first invalid second valid", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 ASOF JOIN t3 MATCH_CONDITION (t1.ts >= t3.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected warning for the first ASOF JOIN missing MATCH_CONDITION, got: %v", warns)
		}
	})

	t.Run("ON after MATCH_CONDITION is not flagged", func(t *testing.T) {
		// ON keyword appearing after MATCH_CONDITION (e.g. in a CASE expression
		// or WHERE clause) should not be treated as an invalid ON clause.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) WHERE t1.flag = 1 AND t2.status ON`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("ON after MATCH_CONDITION should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("ON before MATCH_CONDITION is flagged", func(t *testing.T) {
		// ON appears before MATCH_CONDITION — should produce ON warning.
		sql := `SELECT * FROM t1 ASOF JOIN t2 ON t1.id = t2.id MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid with ASOF JOIN") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected ON warning when ON appears before MATCH_CONDITION, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with empty parens", func(t *testing.T) {
		// Empty MATCH_CONDITION body — no valid comparison operator.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION ()`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for empty MATCH_CONDITION, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with only bare = operators", func(t *testing.T) {
		// Multiple bare = signs, none of which are valid comparisons.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts = t2.ts AND t1.id = t2.id)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for bare = operators, got: %v", warns)
		}
	})

	t.Run("word boundary prevents false match on table/column names", func(t *testing.T) {
		validQueries := []string{
			// Table name ending in USING-like word (REUSING)
			`SELECT * FROM t1 ASOF JOIN reusing_data MATCH_CONDITION (t1.ts >= reusing_data.ts)`,
			// Column containing ON (e.g. "online")
			`SELECT online FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`,
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

	t.Run("USING without parentheses triggers missing MATCH_CONDITION", func(t *testing.T) {
		// Bare USING keyword without '(' is not a column-list form and not a
		// function form, so neither USING warning nor USING-function recognition
		// fires — the missing MATCH_CONDITION warning should appear instead.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning for bare USING without parens, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with unmatched paren skips operator check", func(t *testing.T) {
		// Unmatched opening paren — findMatchingParen returns -1, so the
		// comparison operator check is skipped (no false positive).
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts = t2.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				t.Errorf("Should not flag operator when paren is unmatched, got: %v", w.Message)
			}
		}
	})

	t.Run("ASOF JOIN inside comment is ignored", func(t *testing.T) {
		cases := []string{
			// Line comment
			"SELECT 1\n-- ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)",
			// Block comment
			`SELECT 1 /* ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) */`,
		}
		for _, sql := range cases {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for ASOF JOIN inside comment, got: %v", warns)
				}
			})
		}
	})

	t.Run("ASOF JOIN followed by regular JOIN with ON", func(t *testing.T) {
		// Regular JOIN's ON clause appears after MATCH_CONDITION in the ASOF
		// scope — it must not be flagged as an invalid ON for the ASOF JOIN.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) JOIN t3 ON t2.id = t3.id`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("Regular JOIN ON after MATCH_CONDITION should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("MATCH_CONDITION with nested parentheses", func(t *testing.T) {
		// Nested parens (e.g. CAST) — findMatchingParen must track depth.
		validQueries := []string{
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (CAST(t1.ts AS TIMESTAMP) >= t2.ts)`,
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= DATEADD(HOUR, -1, t2.ts))`,
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (COALESCE(t1.ts, CURRENT_TIMESTAMP()) >= t2.ts)`,
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

	t.Run("MATCH_CONDITION with string literal containing parens", func(t *testing.T) {
		// String literal inside MATCH_CONDITION with embedded parentheses —
		// findMatchingParen must handle quote-skipping so parens inside
		// strings don't confuse depth tracking.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= CAST('2024-01-01 (note)' AS TIMESTAMP))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION without parentheses", func(t *testing.T) {
		// MATCH_CONDITION not followed by '(' — parenStart < 0 branch;
		// no operator check fires, but missing-MATCH_CONDITION also doesn't
		// fire since the regex matched.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION t1.ts >= t2.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		// Should not produce an operator warning since parens are absent.
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				t.Errorf("Should not check operator when MATCH_CONDITION has no parens, got: %v", w.Message)
			}
		}
	})

	t.Run("multiple statements with separate ASOF JOINs", func(t *testing.T) {
		// Two statements separated by semicolon: first valid, second missing
		// MATCH_CONDITION — each is validated independently.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts); SELECT * FROM t3 ASOF JOIN t4`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning for second statement, got: %v", warns)
		}
	})

	t.Run("mixed valid and invalid operators in MATCH_CONDITION", func(t *testing.T) {
		// Invalid != followed by valid >= — the valid operator should be found.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.id != t2.id AND t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				t.Errorf("Should not flag operator when a valid comparison is present, got: %v", w.Message)
			}
		}
	})

	t.Run("word boundary prevents false match on USING-like names", func(t *testing.T) {
		// Table name "ABUSING" should not match the USING check.
		sql := `SELECT * FROM t1 ASOF JOIN abusing_data MATCH_CONDITION (t1.ts >= abusing_data.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for table name 'abusing_data', got: %v", warns)
		}
	})

	t.Run("ON inside parenthesized group not flagged", func(t *testing.T) {
		// ON keyword inside a subquery (depth > 0) should be ignored by
		// hasOnClause even when no MATCH_CONDITION precedes it.
		sql := `SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y ON x.id = y.id) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("ON inside parenthesized subquery should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("USING inside parenthesized group not flagged", func(t *testing.T) {
		// USING (col) inside a subquery (depth > 0) should be ignored by
		// hasUsingClause.
		sql := `SELECT * FROM t1 ASOF JOIN (SELECT * FROM x JOIN y USING (id)) AS t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "USING clause is not valid") {
				t.Errorf("USING inside parenthesized subquery should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("USING FUNCTION with 3-part qualified name", func(t *testing.T) {
		// Fully qualified function name: db.schema.func(...)
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING (mydb.myschema.my_func(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with 3-part name, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with double-quoted identifiers containing parens", func(t *testing.T) {
		// Double-quoted column name with parentheses inside — findMatchingParen
		// must skip quoted content.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1."col(1)" >= t2."col(2)")`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for double-quoted identifiers with parens, got: %v", warns)
		}
	})

	t.Run("ON at end of scope as last token", func(t *testing.T) {
		// ON as the very last token — tests word boundary check where
		// i+2 == len(upper) (no right boundary character).
		sql := `SELECT * FROM t1 ASOF JOIN t2 ON`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid with ASOF JOIN") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected ON warning when ON is last token, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with ORDER BY and GROUP BY after MATCH_CONDITION", func(t *testing.T) {
		// ORDER BY and GROUP BY after MATCH_CONDITION — should not produce
		// false positives.
		sql := `SELECT a.ts, COUNT(*) FROM t1 a ASOF JOIN t2 b MATCH_CONDITION (a.ts >= b.ts) GROUP BY a.ts ORDER BY a.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN with GROUP BY/ORDER BY, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with LIMIT and HAVING", func(t *testing.T) {
		sql := `SELECT a.sensor, COUNT(*) FROM measurements a ASOF JOIN prices b MATCH_CONDITION (a.ts >= b.ts) GROUP BY a.sensor HAVING COUNT(*) > 1 LIMIT 10`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings, got: %v", warns)
		}
	})

	t.Run("containsAsofValidComparison only invalid then valid", func(t *testing.T) {
		// Multiple invalid operators (<>, !=, =) followed by a valid one (<).
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.a <> t2.a AND t1.b != t2.b AND t1.c = t2.c AND t1.ts < t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				t.Errorf("Should not flag when a valid < is present among invalid operators, got: %v", w.Message)
			}
		}
	})

	t.Run("MATCH_CONDITION with only <> operators is flagged", func(t *testing.T) {
		// Only <> operators — containsAsofValidComparison must skip <> and
		// return false (no valid operator found).
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts <> t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for <> only, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN entirely inside subquery produces no warning", func(t *testing.T) {
		// ASOF JOIN only appears inside parenthesized subquery —
		// the top-level token scan finds nothing, so no validation runs.
		sql := `SELECT * FROM (SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)) AS sub`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN entirely inside subquery, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN inside subquery without MATCH_CONDITION no warning", func(t *testing.T) {
		// Invalid ASOF JOIN entirely nested — should not produce warning
		// because the top-level token scan skips it.
		sql := `SELECT * FROM (SELECT * FROM t1 ASOF JOIN t2) AS sub`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN") {
				t.Errorf("Expected no ASOF JOIN warnings for fully nested join, got: %v", w.Message)
			}
		}
	})

	t.Run("both ON and USING produce two warnings", func(t *testing.T) {
		// Both ON and USING (column-list) are present — both should be flagged,
		// but the redundant missing-MATCH_CONDITION warning should be suppressed.
		sql := `SELECT * FROM t1 ASOF JOIN t2 ON t1.id = t2.id USING (ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		onFound, usingFound, mcMissing := false, false, false
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				onFound = true
			}
			if strings.Contains(w.Message, "USING clause is not valid") {
				usingFound = true
			}
			if strings.Contains(w.Message, "requires a MATCH_CONDITION") {
				mcMissing = true
			}
		}
		if !onFound {
			t.Errorf("Expected ON warning, got: %v", warns)
		}
		if !usingFound {
			t.Errorf("Expected USING warning, got: %v", warns)
		}
		if mcMissing {
			t.Errorf("Missing MATCH_CONDITION warning should be suppressed when ON/USING flagged")
		}
	})

	t.Run("multiple statements first invalid second valid", func(t *testing.T) {
		// Reverse of existing test: first statement has missing MATCH_CONDITION,
		// second is valid — validates per-statement isolation.
		sql := `SELECT * FROM t1 ASOF JOIN t2; SELECT * FROM t3 ASOF JOIN t4 MATCH_CONDITION (t3.ts >= t4.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		mcMissing := 0
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				mcMissing++
			}
		}
		if mcMissing != 1 {
			t.Errorf("Expected exactly 1 missing MATCH_CONDITION warning (from first stmt), got %d: %v", mcMissing, warns)
		}
	})

	t.Run("USING FUNCTION with quoted identifier function name", func(t *testing.T) {
		// Quoted identifier in USING FUNCTION form — _ident regex must match
		// the "[^"]+" alternative.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING ("my_match_func"(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with quoted identifier, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with escaped single quotes in body", func(t *testing.T) {
		// Escaped single quotes ('') inside MATCH_CONDITION body — tests that
		// findMatchingParen correctly toggles quote state for consecutive quotes.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= CAST('' AS TIMESTAMP))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with NATURAL keyword after MATCH_CONDITION", func(t *testing.T) {
		// NATURAL JOIN after ASOF JOIN with MATCH_CONDITION — should not
		// produce false positives for the ASOF JOIN.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) NATURAL JOIN t3`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION inside string literal causes missing warning", func(t *testing.T) {
		// MATCH_CONDITION appears only inside a string literal — after
		// stripping, it's gone, so the missing-MATCH_CONDITION warning fires.
		sql := `SELECT * FROM t1 ASOF JOIN t2 WHERE comment = 'MATCH_CONDITION (t1.ts >= t2.ts)'`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning when it's only inside a string, got: %v", warns)
		}
	})

	t.Run("stray closing paren in scope does not crash", func(t *testing.T) {
		// Stray ')' before ON — depth is already 0, tests the `if depth > 0`
		// guard in hasOnClause.
		sql := `SELECT * FROM t1 ASOF JOIN t2) ON t1.ts >= t2.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid with ASOF JOIN") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected ON warning despite stray closing paren, got: %v", warns)
		}
	})

	t.Run("USING at end of scope without parens", func(t *testing.T) {
		// USING as the very last token — i+5 == len(upper), right boundary
		// passes, but `after` is empty so HasPrefix("(") is false.
		// Should produce missing-MATCH_CONDITION warning, not USING warning.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		mcFound := false
		usingFound := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				mcFound = true
			}
			if strings.Contains(w.Message, "USING clause is not valid") {
				usingFound = true
			}
		}
		if !mcFound {
			t.Errorf("Expected missing MATCH_CONDITION warning for bare USING at end, got: %v", warns)
		}
		if usingFound {
			t.Errorf("Should not flag USING without parentheses as invalid USING clause")
		}
	})

	t.Run("MATCH_CONDITION with BETWEEN operator no false positive", func(t *testing.T) {
		// BETWEEN uses implicit >= and <=; containsAsofValidComparison should
		// not find any valid operator in the raw text (no >=, >, <=, < tokens).
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts BETWEEN t2.ts AND t2.ts2)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for BETWEEN (no raw comparison operator), got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with UNION separating statements", func(t *testing.T) {
		// ASOF JOIN in first part of UNION — should validate independently.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) UNION ALL SELECT * FROM t3`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for valid ASOF JOIN before UNION, got: %v", warns)
		}
	})

	t.Run("USING FUNCTION with 2-part qualified name", func(t *testing.T) {
		// 2-part qualified function name: schema.func(...)
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING (myschema.my_func(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with 2-part name, got: %v", warns)
		}
	})

	t.Run("USING FUNCTION with quoted schema and function name", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING ("MY_SCHEMA"."my_func"(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with quoted names, got: %v", warns)
		}
	})

	t.Run("ON-like keyword at top level not flagged", func(t *testing.T) {
		// "ONLY" and "ONGOING" start with ON followed by a word char — the
		// right word boundary check in hasOnClause must skip them.
		validQueries := []string{
			`SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) WHERE ONLY t1.active`,
			`SELECT * FROM t1 ASOF JOIN ongoing_events MATCH_CONDITION (t1.ts >= ongoing_events.ts)`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				for _, w := range warns {
					if strings.Contains(w.Message, "ON clause is not valid") {
						t.Errorf("ON-prefix keyword should not be flagged, got: %v", w.Message)
					}
				}
			})
		}
	})

	t.Run("USING-like keyword at top level not flagged", func(t *testing.T) {
		// "USINGX" starts with USING followed by a word char — the right
		// word boundary check in hasUsingClause must skip it.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) WHERE t1.usingx = 1`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "USING clause is not valid") {
				t.Errorf("USING-prefix identifier should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("MATCH_CONDITION without parens produces missing warning", func(t *testing.T) {
		// MATCH_CONDITION not followed by '(' — the regex requires \(, so
		// hasMatchCondition is false and the missing-MATCH_CONDITION warning fires.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION t1.ts >= t2.ts`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning when parens are absent, got: %v", warns)
		}
	})

	t.Run("both MATCH_CONDITION and USING FUNCTION present", func(t *testing.T) {
		// Both forms appear in the same ASOF JOIN scope — MATCH_CONDITION
		// operator validation still runs.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) USING (my_func(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION") {
				t.Errorf("Should not flag missing MATCH_CONDITION when both forms present, got: %v", w.Message)
			}
		}
	})

	t.Run("USING column-list with multiple columns", func(t *testing.T) {
		// USING (ts, id) is a column-list form, not a USING FUNCTION form.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING (ts, id)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "USING clause is not valid") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected USING warning for multi-column USING, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN in CREATE VIEW context", func(t *testing.T) {
		sql := `CREATE VIEW v AS SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN in CREATE VIEW, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN in INSERT SELECT context", func(t *testing.T) {
		sql := `INSERT INTO target SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN in INSERT SELECT, got: %v", warns)
		}
	})
}

func TestContainsAsofValidComparison(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"bare > returns true", "t1.ts > t2.ts", true},
		{"bare < returns true", "t1.ts < t2.ts", true},
		{">= returns true", "t1.ts >= t2.ts", true},
		{"<= returns true", "t1.ts <= t2.ts", true},
		{"bare = returns false", "t1.ts = t2.ts", false},
		{"<> returns false", "t1.ts <> t2.ts", false},
		{"!= returns false", "t1.ts != t2.ts", false},
		{"empty body returns false", "", false},
		{"no operator returns false", "t1.ts t2.ts", false},
		{"bare ! not followed by = is skipped", "t1.ts ! t2.ts", false},
		{"bare ! at end of body is skipped", "t1.ts !", false},
		{"> at end of body returns true", "t1.ts >", true},
		{"< at end of body returns true", "t1.ts <", true},
		{"<> then >= returns true", "t1.a <> t2.a AND t1.ts >= t2.ts", true},
		{"!= then < returns true", "t1.a != t2.a AND t1.ts < t2.ts", true},
		{"only != and = returns false", "t1.a != t2.a AND t1.b = t2.b", false},
		{"sole > returns true", ">", true},
		{"sole < returns true", "<", true},
		{"sole >= returns true", ">=", true},
		{"sole <= returns true", "<=", true},
		{"sole = returns false", "=", false},
		{"sole ! returns false", "!", false},
		{"whitespace only returns false", "   ", false},
		{"multiple <> returns false", "<> <> <>", false},
		{"multiple != returns false", "!= != !=", false},
		{"= then <> both invalid", "t1.a = t2.a AND t1.b <> t2.b", false},
		{"= then > returns true", "t1.a = t2.a AND t1.ts > t2.ts", true},
		{"<> then < returns true", "t1.a <> t2.a AND t1.ts < t2.ts", true},
		{"!= at very end returns false", "t1.ts !=", false},
		{">= at end of body returns true", "t1.ts >=", true},
		{"<= at end of body returns true", "t1.ts <=", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsAsofValidComparison(tc.body); got != tc.want {
				t.Errorf("containsAsofValidComparison(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestHasOnClauseTok(t *testing.T) {
	toSig := func(s string) ([]sqltok.Token, string) {
		return sigToks(sqltok.Tokenize(s)), s
	}
	tests := []struct {
		name              string
		scope             string
		hasMatchCondition bool
		want              bool
	}{
		{"bare ON at top level", " t2 ON t1.id = t2.id", false, true},
		{"ON inside parens skipped", " (ON t1.id = t2.id)", false, false},
		{"ON after MATCH_CONDITION skipped", " t2 MATCH_CONDITION (t1.ts >= t2.ts) ON t1.id = t2.id", true, false},
		{"ON before MATCH_CONDITION flagged", " t2 ON t1.id = t2.id MATCH_CONDITION (t1.ts >= t2.ts)", true, true},
		{"ONLY not flagged (right word boundary)", " t2 ONLY x", false, false},
		{"ICON not flagged (left word boundary)", " ICON WHERE 1=1", false, false},
		{"ON at end of scope", " t2 ON", false, true},
		{"no ON present", " t2 WHERE 1=1", false, false},
		{"ON at position 0 (left boundary skipped)", "ON t1.id = t2.id", false, true},
		{"ON preceded by newline", "\nON t1.id = t2.id", false, true},
		{"ON preceded by tab", "\tON t1.id = t2.id", false, true},
		{"ON preceded by open paren at depth 0", " (ON x)", false, false},
		{"empty scope", "", false, false},
		{"ON at depth 2 (nested parens) skipped", " ((ON x))", false, false},
		{"multiple ON first at depth 0", " ON x (ON y)", false, true},
		{"ON followed by open paren", " ON(x)", false, true},
		{"scope with only whitespace", "   ", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sig, sql := toSig(tc.scope)
			if got := hasOnClauseTok(sig, sql, tc.hasMatchCondition); got != tc.want {
				t.Errorf("hasOnClauseTok(%q, %v) = %v, want %v", tc.scope, tc.hasMatchCondition, got, tc.want)
			}
		})
	}
}

func TestHasUsingClauseTok(t *testing.T) {
	toSig := func(s string) ([]sqltok.Token, string) {
		return sigToks(sqltok.Tokenize(s)), s
	}
	tests := []struct {
		name             string
		scope            string
		hasUsingFunction bool
		want             bool
	}{
		{"early exit when hasUsingFunction", " t2 USING (ts)", true, false},
		{"USING (col) flagged", " t2 USING (ts)", false, true},
		{"USING(col) no space flagged", " t2 USING(ts)", false, true},
		{"USING inside parens skipped", " (USING (ts))", false, false},
		{"ABUSING not flagged (left word boundary)", " ABUSING (ts)", false, false},
		{"USING without parens not flagged", " t2 USING ts", false, false},
		{"USING at end of scope not flagged", " t2 USING", false, false},
		{"no USING present", " t2 WHERE 1=1", false, false},
		{"USING at position 0 flagged", "USING (ts)", false, true},
		{"USING preceded by newline", "\nUSING (ts)", false, true},
		{"USING preceded by open paren at depth 0", "(USING (ts))", false, false},
		{"USING with newline before paren", " USING\n(ts)", false, true},
		{"empty scope", "", false, false},
		{"USING at depth 2 skipped", " ((USING (ts)))", false, false},
		{"USING at depth 1 skipped", " (USING (ts))", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sig, sql := toSig(tc.scope)
			if got := hasUsingClauseTok(sig, sql, tc.hasUsingFunction); got != tc.want {
				t.Errorf("hasUsingClauseTok(%q, %v) = %v, want %v", tc.scope, tc.hasUsingFunction, got, tc.want)
			}
		})
	}
}

func TestFindMatchingParen(t *testing.T) {
	t.Run("empty string returns -1", func(t *testing.T) {
		if got := findMatchingParen(""); got != -1 {
			t.Errorf("findMatchingParen(\"\") = %d, want -1", got)
		}
	})

	t.Run("non-paren start returns -1", func(t *testing.T) {
		if got := findMatchingParen("abc"); got != -1 {
			t.Errorf("findMatchingParen(\"abc\") = %d, want -1", got)
		}
	})

	t.Run("simple matched parens", func(t *testing.T) {
		if got := findMatchingParen("(abc)"); got != 4 {
			t.Errorf("findMatchingParen(\"(abc)\") = %d, want 4", got)
		}
	})

	t.Run("nested parens", func(t *testing.T) {
		if got := findMatchingParen("(a(b)c)"); got != 6 {
			t.Errorf("findMatchingParen(\"(a(b)c)\") = %d, want 6", got)
		}
	})

	t.Run("unmatched paren returns -1", func(t *testing.T) {
		if got := findMatchingParen("(abc"); got != -1 {
			t.Errorf("findMatchingParen(\"(abc\") = %d, want -1", got)
		}
	})

	t.Run("parens inside single quotes skipped", func(t *testing.T) {
		// The ')' inside single quotes must not close the outer paren.
		if got := findMatchingParen("(')')"); got != 4 {
			t.Errorf("findMatchingParen(\"(')')\") = %d, want 4", got)
		}
	})

	t.Run("parens inside double quotes skipped", func(t *testing.T) {
		if got := findMatchingParen(`(")")`); got != 4 {
			t.Errorf(`findMatchingParen("(\")\")" = %d, want 4`, got)
		}
	})

	t.Run("mixed single and double quotes with parens", func(t *testing.T) {
		// Both quote types with embedded parens: ('(' ")" x)
		s := `('(' ")" x)`
		if got := findMatchingParen(s); got != len(s)-1 {
			t.Errorf("findMatchingParen(%q) = %d, want %d", s, got, len(s)-1)
		}
	})

	t.Run("deeply nested parens", func(t *testing.T) {
		if got := findMatchingParen("(((a)))"); got != 6 {
			t.Errorf("findMatchingParen(\"(((a)))\") = %d, want 6", got)
		}
	})

	t.Run("empty parens", func(t *testing.T) {
		if got := findMatchingParen("()"); got != 1 {
			t.Errorf("findMatchingParen(\"()\") = %d, want 1", got)
		}
	})

	t.Run("single opening paren returns -1", func(t *testing.T) {
		if got := findMatchingParen("("); got != -1 {
			t.Errorf("findMatchingParen(\"(\") = %d, want -1", got)
		}
	})

	t.Run("unclosed single quote returns -1", func(t *testing.T) {
		if got := findMatchingParen("('abc"); got != -1 {
			t.Errorf("findMatchingParen(\"('abc\") = %d, want -1", got)
		}
	})

	t.Run("unclosed double quote returns -1", func(t *testing.T) {
		if got := findMatchingParen(`("abc`); got != -1 {
			t.Errorf(`findMatchingParen("(\"abc") = %d, want -1`, got)
		}
	})

	t.Run("escaped single quotes inside parens", func(t *testing.T) {
		// '' toggles inSingle twice (back to false), so paren tracking resumes.
		if got := findMatchingParen("(''x)"); got != 4 {
			t.Errorf("findMatchingParen(\"(''x)\") = %d, want 4", got)
		}
	})

	t.Run("double-quoted paren content", func(t *testing.T) {
		// Double-quoted string containing '(' — must not increase depth.
		if got := findMatchingParen(`("(")`); got != 4 {
			t.Errorf(`findMatchingParen("(\"(\")" = %d, want 4`, got)
		}
	})

	t.Run("content after matching paren ignored", func(t *testing.T) {
		// Text after the closing paren should not affect the result.
		if got := findMatchingParen("(abc) extra stuff"); got != 4 {
			t.Errorf("findMatchingParen(\"(abc) extra stuff\") = %d, want 4", got)
		}
	})

	t.Run("multiple independent paren groups returns first close", func(t *testing.T) {
		// "(a)(b)" — should return position of first closing paren.
		if got := findMatchingParen("(a)(b)"); got != 2 {
			t.Errorf("findMatchingParen(\"(a)(b)\") = %d, want 2", got)
		}
	})

	t.Run("unclosed double quote with paren returns -1", func(t *testing.T) {
		// Paren inside an unterminated double-quoted string.
		if got := findMatchingParen(`("abc)`); got != -1 {
			t.Errorf(`findMatchingParen("(\"abc)") = %d, want -1`, got)
		}
	})

	t.Run("single quote inside double quotes preserved", func(t *testing.T) {
		// Single quote inside double-quoted string should not toggle inSingle.
		if got := findMatchingParen(`("it's" x)`); got != 9 {
			t.Errorf(`findMatchingParen("(\"it's\" x)") = %d, want 9`, got)
		}
	})
}

func TestValidateSnowflakePatterns_AsofJoinAdditional(t *testing.T) {
	t.Run("three ASOF JOINs middle one invalid", func(t *testing.T) {
		// Three ASOF JOINs: first and third valid, middle missing MATCH_CONDITION.
		// Tests that the scope for the middle join is bounded on both sides.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) ASOF JOIN t3 ASOF JOIN t4 MATCH_CONDITION (t1.ts >= t4.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning for middle ASOF JOIN, got: %v", warns)
		}
	})

	t.Run("three ASOF JOINs all valid", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) ASOF JOIN t3 MATCH_CONDITION (t1.ts >= t3.ts) ASOF JOIN t4 MATCH_CONDITION (t1.ts >= t4.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for three valid ASOF JOINs, got: %v", warns)
		}
	})

	t.Run("USING without space before paren flagged", func(t *testing.T) {
		// USING(ts) with no space — hasUsingClause uses TrimSpace so this
		// should still be flagged as invalid column-list form.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING(ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "USING clause is not valid") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected USING warning for USING(ts) without space, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN followed by LEFT JOIN with ON", func(t *testing.T) {
		// LEFT JOIN ON after ASOF JOIN with MATCH_CONDITION — the LEFT JOIN's
		// ON must not be flagged.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) LEFT JOIN t3 ON t2.id = t3.id`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("LEFT JOIN ON after MATCH_CONDITION should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("ASOF JOIN followed by CROSS JOIN", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) CROSS JOIN t3`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN followed by CROSS JOIN, got: %v", warns)
		}
	})

	t.Run("both statements invalid", func(t *testing.T) {
		// Two semicolon-separated statements, both with missing MATCH_CONDITION.
		sql := `SELECT * FROM t1 ASOF JOIN t2; SELECT * FROM t3 ASOF JOIN t4`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		mcMissing := 0
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				mcMissing++
			}
		}
		if mcMissing != 2 {
			t.Errorf("Expected 2 missing MATCH_CONDITION warnings (one per statement), got %d: %v", mcMissing, warns)
		}
	})

	t.Run("ASOF JOIN in CTAS", func(t *testing.T) {
		sql := `CREATE TABLE result AS SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN in CTAS, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with MATCH_CONDITION containing block comment", func(t *testing.T) {
		// Block comment inside MATCH_CONDITION parens — comments are stripped
		// by the caller before validateAsofJoinClauses runs.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (/* timestamp */ t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for MATCH_CONDITION with block comment, got: %v", warns)
		}
	})

	t.Run("USING FUNCTION with no arguments", func(t *testing.T) {
		// USING (func()) — function call with no arguments.
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING (my_func())`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with no arguments, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with line comment inside parens", func(t *testing.T) {
		sql := "SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (\n-- compare timestamps\nt1.ts >= t2.ts)"
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for MATCH_CONDITION with line comment, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with dollar-quoted string containing ASOF JOIN", func(t *testing.T) {
		// ASOF JOIN text inside a dollar-quoted block should not trigger validation.
		sql := `CREATE PROCEDURE foo() RETURNS STRING LANGUAGE SQL AS $$ SELECT * FROM t1 ASOF JOIN t2 $$`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN") {
				t.Errorf("ASOF JOIN inside dollar-quoted block should not be validated, got: %v", w.Message)
			}
		}
	})

	t.Run("ASOF JOIN in MERGE statement", func(t *testing.T) {
		sql := `MERGE INTO target t USING (SELECT * FROM src a ASOF JOIN ref b MATCH_CONDITION (a.ts >= b.ts)) s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.val = s.val`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN in MERGE, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with window function OVER", func(t *testing.T) {
		// Window function with OVER() after MATCH_CONDITION — ORDER BY inside
		// OVER() must not interfere with validation.
		sql := `SELECT ROW_NUMBER() OVER (ORDER BY a.ts) FROM t1 a ASOF JOIN t2 b MATCH_CONDITION (a.ts >= b.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN with window function, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with multi-whitespace between keywords", func(t *testing.T) {
		// Tab and multiple spaces between ASOF and JOIN.
		sql := "SELECT * FROM t1 ASOF\t  JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)"
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for multi-whitespace ASOF JOIN, got: %v", warns)
		}
	})

	t.Run("empty statement between ASOF JOIN statements", func(t *testing.T) {
		// Empty statement (just semicolons) between two ASOF JOIN queries.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts);; SELECT * FROM t3 ASOF JOIN t4`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected missing MATCH_CONDITION warning for third statement, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with QUALIFY clause", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) QUALIFY ROW_NUMBER() OVER (PARTITION BY t1.id ORDER BY t1.ts) = 1`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN with QUALIFY, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with LATERAL subquery", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts), LATERAL (SELECT * FROM t3 WHERE t3.id = t1.id)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for ASOF JOIN with LATERAL, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN ON where ON is part of function name", func(t *testing.T) {
		// CONCAT_ON is not a real function but tests that ON-as-suffix in a
		// function call after ASOF JOIN is not flagged.
		sql := `SELECT CONCAT_ON(a.x, b.x) FROM t1 a ASOF JOIN t2 b MATCH_CONDITION (a.ts >= b.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("ON suffix in function name should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("ON keyword inside string literal not flagged", func(t *testing.T) {
		// ON appears only inside a string literal — after stripping, it's
		// gone, so hasOnClause should not fire.
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) WHERE x = 'ON'`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("ON inside string literal should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("USING keyword inside string literal not flagged", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts) WHERE x = 'USING (ts)'`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "USING clause is not valid") {
				t.Errorf("USING inside string literal should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("multiple ASOF JOINs first ON second missing MATCH_CONDITION", func(t *testing.T) {
		// First ASOF JOIN uses invalid ON, second is missing MATCH_CONDITION.
		// Both should produce warnings.
		sql := `SELECT * FROM t1 ASOF JOIN t2 ON t1.id = t2.id ASOF JOIN t3`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		onFound, mcMissing := false, false
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				onFound = true
			}
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION clause") {
				mcMissing = true
			}
		}
		if !onFound {
			t.Errorf("Expected ON warning for first ASOF JOIN, got: %v", warns)
		}
		if !mcMissing {
			t.Errorf("Expected missing MATCH_CONDITION warning for second ASOF JOIN, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN with column alias containing ON", func(t *testing.T) {
		// Column alias "on_flag" should not trigger ON warning.
		sql := `SELECT t1.on_flag FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ON clause is not valid") {
				t.Errorf("Column containing ON should not be flagged, got: %v", w.Message)
			}
		}
	})

	t.Run("exact warning count for single missing MATCH_CONDITION", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning, got %d: %v", len(warns), warns)
		}
	})

	t.Run("exact warning count for ON clause", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 ON t1.id = t2.id`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning for ON clause, got %d: %v", len(warns), warns)
		}
	})

	t.Run("exact warning count for invalid operator", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.ts = t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) != 1 {
			t.Errorf("Expected exactly 1 warning for invalid operator, got %d: %v", len(warns), warns)
		}
	})

	t.Run("no warnings when statement has no ASOF JOIN", func(t *testing.T) {
		sql := `SELECT * FROM t1 JOIN t2 ON t1.id = t2.id`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		// Filter to ASOF-related warnings only.
		asofWarns := 0
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF") || strings.Contains(w.Message, "MATCH_CONDITION") {
				asofWarns++
			}
		}
		if asofWarns > 0 {
			t.Errorf("Expected no ASOF-related warnings for regular JOIN, got %d", asofWarns)
		}
	})

	t.Run("USING FUNCTION with dollar sign in function name", func(t *testing.T) {
		// _ident allows $ in non-first position: [a-zA-Z_][a-zA-Z0-9_$]*
		sql := `SELECT * FROM t1 ASOF JOIN t2 USING (my_func$1(t1.ts, t2.ts))`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		if len(warns) > 0 {
			t.Errorf("Expected no warnings for USING FUNCTION with $ in name, got: %v", warns)
		}
	})

	t.Run("ASOF JOIN at very start of statement", func(t *testing.T) {
		// ASOF JOIN at the very start of the token stream (index 0).
		sql := `ASOF JOIN t2 MATCH_CONDITION (t1.ts >= t2.ts)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		for _, w := range warns {
			if strings.Contains(w.Message, "ASOF JOIN requires a MATCH_CONDITION") {
				t.Errorf("Should not flag missing MATCH_CONDITION when it is present, got: %v", w.Message)
			}
		}
	})

	t.Run("MATCH_CONDITION with only != operators flagged", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.a != t2.a AND t1.b != t2.b)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for != only, got: %v", warns)
		}
	})

	t.Run("MATCH_CONDITION with = and <> mixed both invalid", func(t *testing.T) {
		sql := `SELECT * FROM t1 ASOF JOIN t2 MATCH_CONDITION (t1.a = t2.a AND t1.b <> t2.b)`
		stmtRanges := GetStatementRanges(sql)
		markers := ValidateSnowflakePatterns(sql, stmtRanges)
		warns := getWarnings(markers)
		found := false
		for _, w := range warns {
			if strings.Contains(w.Message, "MATCH_CONDITION comparison must use one of") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected invalid operator warning for = and <> mix, got: %v", warns)
		}
	})
}

