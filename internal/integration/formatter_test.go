// SPDX-License-Identifier: GPL-3.0-or-later

//go:build integration

// Package integration_test — SQL formatter dialect validation.
//
// These tests execute the representative formatted-SQL patterns that the
// frontend sql-formatter/applyCasing pipeline produces and verify that
// Snowflake accepts them without syntax errors.
//
// All queries use inline data (VALUES, PARSE_JSON, literals) so no tables,
// schemas, or elevated privileges are needed beyond basic query execution.
//
// Run with:
//
//	go test -v -tags integration -timeout 5m ./internal/integration/ -run TestFormatterSQL
package integration_test

import (
	"testing"
)

// TestFormatterSQL verifies that the SQL patterns produced by the Thaw
// formatter (keyword UPPER, 2-space indent, trailing comma, operator before)
// are syntactically valid on a real Snowflake account.
//
// Queries are grouped by formatter feature or Snowflake dialect construct.
// A failing case means the formatter emits SQL the Snowflake parser rejects.
func TestFormatterSQL(t *testing.T) {
	client := keyPairConnFromEnv(t)

	cases := []struct {
		name string
		sql  string
	}{
		// ── 1. Trivial sanity check ───────────────────────────────────────────
		{
			name: "select_literal",
			sql:  `SELECT 1 AS n, 'hello' AS s, TRUE AS b`,
		},

		// ── 2. :: cast operator ───────────────────────────────────────────────
		// Validates that no spurious whitespace around :: makes Snowflake reject
		// the query.  The formatter must produce col::TYPE, never col :: TYPE.
		{
			name: "cast_operator_basic",
			sql: `SELECT
  '42'::INTEGER AS int_val,
  '3.14'::FLOAT AS flt_val,
  '2024-06-01'::DATE AS date_val,
  123::VARCHAR AS str_val`,
		},
		{
			name: "cast_operator_chained",
			sql: `SELECT
  '2024-01-01'::DATE::TIMESTAMP_NTZ AS ts,
  42::VARCHAR::VARCHAR AS redundant_chain`,
		},
		{
			name: "cast_operator_arithmetic",
			sql: `SELECT
  (10 + 5 * 2)::NUMBER(18, 2) AS result,
  (100 - 1)::FLOAT AS diff`,
		},
		{
			name: "cast_operator_function_result",
			sql: `SELECT
  TO_DATE('2024-01-01')::TIMESTAMP_NTZ AS ts,
  CURRENT_TIMESTAMP()::DATE AS today,
  LENGTH('hello')::VARCHAR AS len_str`,
		},
		{
			name: "cast_operator_inside_function_args",
			sql: `SELECT
  COALESCE('42'::INTEGER, 0) AS safe_int,
  IFF('true'::BOOLEAN, 'yes', 'no') AS iff_result,
  NVL(NULL::VARCHAR, 'fallback') AS nvl_result`,
		},

		// ── 3. Snowflake VARIANT : path operator ─────────────────────────────
		// The formatter must preserve src:key syntax without inserting spaces.
		{
			name: "variant_single_level",
			sql: `SELECT
  v:user_id::STRING AS uid,
  v:score::INTEGER AS score
FROM
  (SELECT PARSE_JSON('{"user_id":"abc","score":99}') AS v) t`,
		},
		{
			name: "variant_nested_path",
			sql: `SELECT
  v:profile:name::STRING AS name,
  v:profile:address:city::STRING AS city,
  v:profile:age::INTEGER AS age
FROM
  (SELECT PARSE_JSON('{"profile":{"name":"Alice","address":{"city":"Berlin"},"age":30}}') AS v) t`,
		},
		{
			name: "variant_path_with_cast_combo",
			sql: `SELECT
  v:user_id::STRING AS uid,
  v:amount::NUMBER(18, 2) AS amount,
  v:meta:created_at::TIMESTAMP_NTZ AS created_at
FROM
  (SELECT PARSE_JSON('{"user_id":"u1","amount":"9.99","meta":{"created_at":"2024-01-01 12:00:00"}}') AS v) t`,
		},

		// ── 4. LATERAL FLATTEN ───────────────────────────────────────────────
		// Tests the named parameter => syntax and multi-level VARIANT paths.
		{
			name: "lateral_flatten_basic",
			sql: `SELECT
  f.index AS idx,
  f.value::STRING AS tag
FROM
  (SELECT PARSE_JSON('["alpha","beta","gamma"]') AS arr) t,
  LATERAL FLATTEN(INPUT => t.arr) f`,
		},
		{
			name: "lateral_flatten_nested_variant",
			sql: `SELECT
  doc:uid::STRING AS uid,
  f.value::STRING AS tag,
  doc:score::INTEGER AS score
FROM
  (
    SELECT PARSE_JSON('{"uid":"u1","tags":["sql","python"],"score":95}') AS doc
  ) t,
  LATERAL FLATTEN(INPUT => doc:tags) f`,
		},
		{
			name: "lateral_flatten_outer_param",
			sql: `SELECT
  f.index,
  f.value::STRING AS item
FROM
  (SELECT PARSE_JSON('[]') AS empty_arr) t,
  LATERAL FLATTEN(INPUT => t.empty_arr, OUTER => TRUE) f`,
		},

		// ── 5. CTE formatting ─────────────────────────────────────────────────
		{
			name: "cte_simple",
			sql: `WITH cte AS (
  SELECT 1 AS n
)
SELECT n
FROM cte`,
		},
		{
			name: "cte_multiple",
			sql: `WITH
  a AS (
    SELECT column1 AS n
    FROM VALUES (1), (2), (3) AS t (column1)
  ),
  b AS (
    SELECT n * 2 AS m
    FROM a
  )
SELECT
  a.n,
  b.m
FROM
  a
  JOIN b ON b.m = a.n * 2`,
		},
		{
			name: "cte_with_window_and_qualify",
			sql: `WITH
  base AS (
    SELECT
      column1 AS grp,
      column2 AS val
    FROM
      VALUES (1, 100), (1, 200), (1, 50), (2, 300), (2, 150) AS t (column1, column2)
  ),
  ranked AS (
    SELECT
      grp,
      val,
      ROW_NUMBER() OVER (
        PARTITION BY grp
        ORDER BY val DESC
      ) AS rn,
      SUM(val) OVER (PARTITION BY grp) AS grp_total
    FROM
      base
  )
SELECT
  grp,
  val,
  grp_total
FROM
  ranked
WHERE
  rn = 1`,
		},

		// ── 6. Window functions ───────────────────────────────────────────────
		{
			name: "window_frames_rows_between",
			sql: `WITH
  data AS (
    SELECT
      column1 AS grp,
      column2 AS amt
    FROM
      VALUES (1, 10), (1, 20), (1, 30), (2, 5), (2, 15) AS t (column1, column2)
  )
SELECT
  grp,
  amt,
  SUM(amt) OVER (
    PARTITION BY grp
    ORDER BY amt
    ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
  ) AS running_total,
  AVG(amt) OVER (
    PARTITION BY grp
    ORDER BY amt
    ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING
  ) AS moving_avg,
  FIRST_VALUE(amt) OVER (
    PARTITION BY grp
    ORDER BY amt
    ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
  ) AS grp_min
FROM
  data`,
		},
		{
			// Snowflake does not support the SQL standard WINDOW clause
			// (named window definitions).  Window specs must be inlined in each
			// OVER (...) expression.
			name: "window_multiple_functions_inline",
			sql: `SELECT
  column1 AS id,
  column2 AS grp,
  column3 AS val,
  ROW_NUMBER() OVER (PARTITION BY column2 ORDER BY column3 DESC) AS rn,
  RANK() OVER (PARTITION BY column2 ORDER BY column3 DESC) AS rnk,
  DENSE_RANK() OVER (PARTITION BY column2 ORDER BY column3 DESC) AS dense_rnk
FROM
  VALUES (1, 'a', 10), (2, 'a', 20), (3, 'b', 5) AS t (column1, column2, column3)`,
		},

		// ── 7. Aggregate functions ────────────────────────────────────────────
		{
			name: "listagg_within_group",
			sql: `WITH
  items AS (
    SELECT
      column1 AS grp,
      column2 AS name
    FROM
      VALUES (1, 'alpha'), (1, 'beta'), (1, 'gamma'), (2, 'delta') AS t (column1, column2)
  )
SELECT
  grp,
  LISTAGG(name, ', ') WITHIN GROUP (ORDER BY name) AS names,
  COUNT(*) AS cnt,
  MIN(name) AS first_name,
  MAX(name) AS last_name
FROM
  items
GROUP BY
  grp`,
		},
		{
			name: "group_by_rollup",
			sql: `WITH
  sales AS (
    SELECT
      column1 AS region,
      column2 AS product,
      column3 AS amount
    FROM
      VALUES
        ('NA', 'A', 100),
        ('NA', 'B', 200),
        ('EU', 'A', 150),
        ('EU', 'B', 250) AS t (column1, column2, column3)
  )
SELECT
  region,
  product,
  SUM(amount) AS total
FROM
  sales
GROUP BY
  ROLLUP (region, product)
ORDER BY
  region NULLS LAST,
  product NULLS LAST`,
		},
		{
			name: "approx_percentile",
			sql: `SELECT
  APPROX_PERCENTILE(column1, 0.5) AS median,
  APPROX_PERCENTILE(column1, 0.95) AS p95,
  APPROX_COUNT_DISTINCT(column2) AS approx_unique
FROM
  VALUES (1, 'a'), (2, 'b'), (3, 'a'), (4, 'c') AS t (column1, column2)`,
		},

		// ── 8. Semi-structured functions ──────────────────────────────────────
		{
			name: "array_construct_and_operations",
			sql: `SELECT
  ARRAY_CONSTRUCT(1, 2, 3) AS arr,
  ARRAY_CONSTRUCT(1, 2, 3)[1]::INTEGER AS second,
  ARRAY_SIZE(ARRAY_CONSTRUCT('a', 'b', 'c')) AS arr_len,
  ARRAY_CONTAINS(ARRAY_CONSTRUCT(1, 2, 3)::ARRAY, 2::VARIANT) AS has_two,
  ARRAY_AGG(column1)::ARRAY AS agg_arr
FROM
  VALUES (10), (20), (30) AS t (column1)`,
		},
		{
			name: "object_construct_and_access",
			sql: `SELECT
  OBJECT_CONSTRUCT('name', 'Alice', 'age', 30) AS obj,
  GET(OBJECT_CONSTRUCT('key', 'value'), 'key')::STRING AS obj_key,
  OBJECT_KEYS(PARSE_JSON('{"a":1,"b":2}')) AS keys`,
		},

		// ── 9. Conditional expressions ────────────────────────────────────────
		{
			name: "iff_nested_in_case_coalesce",
			sql: `SELECT
  COALESCE(
    CASE
      WHEN IFF(0.9 > 0.8, TRUE, FALSE) THEN 'high'
      WHEN IFF(0.6 > 0.5, TRUE, FALSE) THEN 'medium'
      ELSE 'low'
    END,
    'unknown'
  ) AS tier,
  IFF(
    COALESCE(NULL::INTEGER, 0) > 0,
    'positive',
    'non-positive'
  ) AS sign,
  NVL2(NULL, 'was_not_null', 'was_null') AS nvl2_result`,
		},
		{
			name: "decode_and_nullif",
			sql: `SELECT
  DECODE(column1, 1, 'one', 2, 'two', 'other') AS decoded,
  NULLIF(column1, 0) AS zero_to_null,
  ZEROIFNULL(NULL::INTEGER) AS null_to_zero,
  DIV0(column1, NULLIF(column2, 0)) AS safe_div
FROM
  VALUES (1, 0), (2, 5), (0, 3) AS t (column1, column2)`,
		},

		// ── 10. String functions ──────────────────────────────────────────────
		{
			name: "string_functions_complex",
			sql: `SELECT
  REGEXP_REPLACE(column1, '[aeiou]', '*') AS vowels_masked,
  REGEXP_COUNT(column1, '[0-9]') AS digit_count,
  INITCAP(LOWER(column1)) AS title_case,
  SPLIT_PART(column1, '.', 1) AS first_segment,
  STRTOK(column1, '.', 2) AS second_token,
  TRIM(LPAD(RPAD(column1, 10, '-'), 14, '-')) AS padded
FROM
  VALUES ('Hello.World.123'), ('abc.def') AS t (column1)`,
		},
		{
			name: "regexp_extract_and_like",
			sql: `SELECT
  column1,
  REGEXP_LIKE(column1, '^[A-Z].*') AS starts_upper,
  REGEXP_SUBSTR(column1, '[0-9]+') AS first_number,
  column1 ILIKE '%sql%' AS contains_sql
FROM
  VALUES ('SQL2024'), ('Python3'), ('hello sql world') AS t (column1)`,
		},

		// ── 11. Date/time functions ───────────────────────────────────────────
		{
			name: "date_functions",
			sql: `SELECT
  CURRENT_DATE() AS today,
  CURRENT_TIMESTAMP()::DATE AS today_via_cast,
  DATEADD('day', -7, CURRENT_DATE()) AS last_week,
  DATEDIFF('day', '2024-01-01'::DATE, CURRENT_DATE()) AS days_since_jan,
  DATE_TRUNC('month', CURRENT_DATE()) AS month_start,
  TO_TIMESTAMP_NTZ(DATEADD('second', -3600, CURRENT_TIMESTAMP())) AS one_hour_ago`,
		},
		{
			name: "timestamp_casts",
			sql: `SELECT
  CURRENT_TIMESTAMP()::TIMESTAMP_NTZ AS ts_ntz,
  CURRENT_TIMESTAMP()::TIMESTAMP_LTZ AS ts_ltz,
  CURRENT_TIMESTAMP()::TIMESTAMP_TZ AS ts_tz,
  DATEADD('minute', -5, CURRENT_TIMESTAMP())::TIMESTAMP_TZ AS five_min_ago`,
		},

		// ── 12. TRY_ functions (graceful failure) ─────────────────────────────
		{
			name: "try_functions",
			sql: `SELECT
  TRY_CAST('42' AS INTEGER) AS int_ok,
  TRY_CAST('not-a-number' AS INTEGER) AS int_null,
  TRY_TO_DATE('2024-01-15', 'YYYY-MM-DD') AS date_ok,
  TRY_TO_DATE('invalid', 'YYYY-MM-DD') AS date_null,
  TRY_TO_DECIMAL('3.14', 10, 2) AS dec_ok,
  TRY_TO_DECIMAL('abc', 10, 2) AS dec_null,
  IS_INTEGER(TRY_CAST('99' AS INTEGER)) AS is_int_check`,
		},

		// ── 13. Leading comma style ───────────────────────────────────────────
		// The formatter can produce commas at the start of lines. Verify Snowflake
		// accepts this style.
		{
			name: "leading_commas_select",
			sql: `SELECT
  column1 AS a
, column2 AS b
, column3 AS c
, column1 + column2 AS ab_sum
FROM
  VALUES (1, 2, 3) AS t (column1, column2, column3)`,
		},
		{
			name: "leading_commas_cte",
			sql: `WITH
  a AS (
    SELECT
      1 AS n
    , 2 AS m
    , 3 AS k
  )
SELECT
  n
, m
, k
, n + m + k AS total
FROM
  a`,
		},

		// ── 14. AND/OR operator placement ────────────────────────────────────
		// Both "before" (AND at start) and "after" (AND at end) must parse.
		{
			name: "operator_before_style",
			sql: `SELECT
  column1,
  column2
FROM
  VALUES (1, 'a'), (2, 'b'), (3, 'c'), (4, 'd') AS t (column1, column2)
WHERE
  column1 > 1
  AND column1 < 4
  AND column2 != 'c'`,
		},
		{
			name: "operator_after_style",
			sql: `SELECT
  column1,
  column2
FROM
  VALUES (1, 'a'), (2, 'b'), (3, 'c'), (4, 'd') AS t (column1, column2)
WHERE
  column1 > 1 AND
  column1 < 4 AND
  column2 != 'c'`,
		},

		// ── 15. PIVOT ────────────────────────────────────────────────────────
		{
			name: "pivot_basic",
			sql: `SELECT
  *
FROM
  (
    SELECT
      column1 AS region,
      column2 AS quarter,
      column3 AS amount
    FROM
      VALUES
        ('NA', 'Q1', 100),
        ('NA', 'Q2', 150),
        ('EU', 'Q1', 200),
        ('EU', 'Q2', 180) AS t (column1, column2, column3)
  )
PIVOT (SUM(amount) FOR quarter IN ('Q1', 'Q2'))`,
		},

		// ── 16. SAMPLE ───────────────────────────────────────────────────────
		// SAMPLE on a VALUES-based subquery.
		{
			name: "sample_clause",
			sql: `SELECT
  *
FROM
  (
    SELECT column1 AS n
    FROM VALUES (1), (2), (3), (4), (5) AS t (column1)
  ) sub
  SAMPLE (100)`,
		},

		// ── 17. Complex multi-CTE: FLATTEN + window + QUALIFY + :: ────────────
		// This is the "stress test" — the most likely query to cause a formatter
		// regression.  It combines every Snowflake-specific feature at once.
		{
			name: "complex_multi_cte_flatten_window_qualify",
			sql: `WITH
  raw AS (
    SELECT PARSE_JSON('{"uid":"u1","tags":["sql","snowflake"],"score":95}') AS doc
    UNION ALL
    SELECT PARSE_JSON('{"uid":"u2","tags":["python","sql"],"score":87}')
    UNION ALL
    SELECT PARSE_JSON('{"uid":"u3","tags":["sql"],"score":72}')
  ),
  exploded AS (
    SELECT
      doc:uid::STRING AS uid,
      f.value::STRING AS tag,
      doc:score::INTEGER AS score
    FROM
      raw,
      LATERAL FLATTEN(INPUT => doc:tags) f
  ),
  counted AS (
    SELECT
      uid,
      tag,
      score,
      COUNT(*) OVER (PARTITION BY tag) AS tag_popularity,
      ROW_NUMBER() OVER (
        PARTITION BY uid
        ORDER BY score DESC
      ) AS rn
    FROM
      exploded
  )
SELECT
  uid,
  tag,
  score,
  tag_popularity
FROM
  counted
WHERE
  rn = 1
ORDER BY
  score DESC,
  uid`,
		},

		// ── 18. Deeply nested VARIANT path with coalesce fallback ─────────────
		{
			name: "deeply_nested_variant_coalesce",
			sql: `SELECT
  COALESCE(v:a:b:c::STRING, v:a:b::STRING, v:a::STRING, 'default') AS best_match,
  NVL(v:missing:path::INTEGER, -1) AS with_fallback,
  IFF(
    v:flags:active::BOOLEAN IS NOT NULL,
    v:flags:active::BOOLEAN,
    FALSE
  ) AS active_flag
FROM
  (
    SELECT
      PARSE_JSON('{"a":{"b":{"c":"deep_value"},"flag":"yes"},"flags":{"active":true}}') AS v
  ) t`,
		},
	}

	for _, tc := range cases {
		tc := tc // capture for parallel sub-tests
		t.Run(tc.name, func(t *testing.T) {
			result := mustQuery(t, client, tc.sql)
			if result == nil {
				t.Fatal("query returned nil result")
			}
			t.Logf("rows=%d cols=%d", len(result.Rows), len(result.Columns))
		})
	}
}
