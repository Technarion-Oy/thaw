package sqlgrammar

import "testing"

// Tests for the query-construct (SELECT sub-clause) grammar rules implemented in
// query_constructs.go. Each rule must fully consume a realistic clause and reject
// structurally malformed or empty input.

func TestParseWith(t *testing.T) {
	assertValid(t, (*Validator).ParseWith,
		`WITH cte AS ( SELECT 1 ) SELECT * FROM cte`,
		`WITH a (x, y) AS ( SELECT 1, 2 ), b AS ( SELECT 3 ) SELECT * FROM a`,
		`WITH RECURSIVE r (n) AS ( SELECT 1 ) SELECT n FROM r`,
	)
	assertInvalid(t, (*Validator).ParseWith,
		``,
		`SELECT 1`,
		`WITH cte SELECT 1`, // missing AS ( ... )
	)
}

func TestParseTopN(t *testing.T) {
	assertValid(t, (*Validator).ParseTopN,
		`SELECT TOP 5 a, b FROM t`,
		`SELECT TOP 100 * FROM t ORDER BY a`,
		`SELECT a FROM t`,
	)
	assertInvalid(t, (*Validator).ParseTopN,
		``,
		`FROM t`,
		`UPDATE t SET a = 1`,
	)
}

func TestParseInto(t *testing.T) {
	assertValid(t, (*Validator).ParseInto,
		`SELECT 1 INTO :v`,
		`SELECT a, b INTO :x, :y FROM t`,
		`SELECT col INTO :var FROM t WHERE a = 1`,
	)
	assertInvalid(t, (*Validator).ParseInto,
		``,
		`SELECT 1 FROM t`, // no INTO clause
		`INTO :v`,         // no leading SELECT
	)
}

func TestParseFrom(t *testing.T) {
	assertValid(t, (*Validator).ParseFrom,
		`FROM mytable`,
		`FROM db.schema.tbl t`,
		`FROM ( SELECT 1 ) s`,
	)
	assertInvalid(t, (*Validator).ParseFrom,
		``,
		`SELECT 1`,
		`WHERE x = 1`,
	)
}

func TestParseAtBefore(t *testing.T) {
	assertValid(t, (*Validator).ParseAtBefore,
		`AT ( TIMESTAMP => '2024-01-01' )`,
		`BEFORE ( STATEMENT => '01a' )`,
		`AT ( OFFSET => -60 )`,
	)
	assertInvalid(t, (*Validator).ParseAtBefore,
		``,
		`SELECT 1`,
		`AT TIMESTAMP`, // missing parens
	)
}

func TestParseChanges(t *testing.T) {
	assertValid(t, (*Validator).ParseChanges,
		`CHANGES ( INFORMATION => DEFAULT ) AT ( OFFSET => -60 )`,
		`CHANGES ( INFORMATION => APPEND_ONLY ) BEFORE ( STATEMENT => '1' )`,
		`CHANGES ( INFORMATION => DEFAULT ) AT ( OFFSET => -60 ) END ( OFFSET => -1 )`,
	)
	assertInvalid(t, (*Validator).ParseChanges,
		``,
		`CHANGES ( INFORMATION => DEFAULT )`, // missing AT/BEFORE
		`SELECT 1`,
	)
}

func TestParseConnectBy(t *testing.T) {
	assertValid(t, (*Validator).ParseConnectBy,
		`SELECT a FROM t START WITH a = 1 CONNECT BY PRIOR a = b`,
		`SELECT id, parent FROM emp START WITH parent IS NULL CONNECT BY parent = PRIOR id`,
		`SELECT * FROM t START WITH x > 0 CONNECT BY PRIOR x = y`,
	)
	assertInvalid(t, (*Validator).ParseConnectBy,
		``,
		`SELECT a FROM t`, // no START WITH
		`FROM t START WITH a = 1`,
	)
}

func TestParseJoin(t *testing.T) {
	assertValid(t, (*Validator).ParseJoin,
		`FROM a JOIN b ON a.id = b.id`,
		`FROM a LEFT OUTER JOIN b ON a.id = b.id`,
		`FROM a INNER JOIN b USING ( id )`,
	)
	assertInvalid(t, (*Validator).ParseJoin,
		``,
		`SELECT 1`,
		`FROM a b c`, // no JOIN keyword
	)
}

func TestParseAsofJoin(t *testing.T) {
	assertValid(t, (*Validator).ParseAsofJoin,
		`FROM trades ASOF JOIN quotes MATCH_CONDITION ( trades.t >= quotes.t )`,
		`FROM a ASOF JOIN b MATCH_CONDITION ( a.ts <= b.ts ) ON a.k = b.k`,
		`FROM a ASOF JOIN b MATCH_CONDITION ( a.ts >= b.ts ) USING ( k )`,
	)
	assertInvalid(t, (*Validator).ParseAsofJoin,
		``,
		`FROM a JOIN b`, // missing ASOF and MATCH_CONDITION
		`FROM a ASOF JOIN b`,
	)
}

func TestParseLateral(t *testing.T) {
	assertValid(t, (*Validator).ParseLateral,
		`FROM t, LATERAL ( SELECT 1 )`,
		`FROM db.t, LATERAL ( SELECT a FROM other WHERE other.id = t.id )`,
		`FROM emp e, LATERAL ( SELECT x )`,
	)
	assertInvalid(t, (*Validator).ParseLateral,
		``,
		`FROM t LATERAL ( SELECT 1 )`, // missing comma
		`SELECT 1`,
	)
}

func TestParseMatchRecognize(t *testing.T) {
	assertValid(t, (*Validator).ParseMatchRecognize,
		`MATCH_RECOGNIZE ( PATTERN ( a b ) DEFINE a AS true )`,
		`MATCH_RECOGNIZE ( PARTITION BY x ORDER BY t PATTERN ( a ) DEFINE a AS x > 0 )`,
		`MATCH_RECOGNIZE ( MEASURES x AS y PATTERN ( a ) DEFINE a AS true )`,
	)
	assertInvalid(t, (*Validator).ParseMatchRecognize,
		``,
		`MATCH_RECOGNIZE`, // no parens
		`SELECT 1`,
	)
}

func TestParsePivot(t *testing.T) {
	assertValid(t, (*Validator).ParsePivot,
		`PIVOT ( sum(amount) FOR month IN ( 'jan', 'feb' ) )`,
		`PIVOT ( count(*) FOR k IN ( ANY ) )`,
		`PIVOT ( avg(v) FOR c IN ( 1, 2 ) DEFAULT ON NULL ( 0 ) )`,
	)
	assertInvalid(t, (*Validator).ParsePivot,
		``,
		`PIVOT`, // no parens
		`UNPIVOT ( a FOR b IN ( c ) )`,
	)
}

func TestParseUnpivot(t *testing.T) {
	assertValid(t, (*Validator).ParseUnpivot,
		`UNPIVOT ( val FOR name IN ( a, b, c ) )`,
		`UNPIVOT INCLUDE NULLS ( val FOR name IN ( a, b ) )`,
		`UNPIVOT EXCLUDE NULLS ( v FOR n IN ( x ) )`,
	)
	assertInvalid(t, (*Validator).ParseUnpivot,
		``,
		`UNPIVOT`, // no parens
		`PIVOT ( a FOR b IN ( c ) )`,
	)
}

func TestParseValues(t *testing.T) {
	assertValid(t, (*Validator).ParseValues,
		`FROM ( VALUES ( 1, 2 ) )`,
		`FROM ( VALUES ( 1, 'a' ), ( 2, 'b' ) ) AS t ( id, name )`,
		`FROM ( VALUES ( 1 ), ( 2 ), ( 3 ) ) v`,
	)
	assertInvalid(t, (*Validator).ParseValues,
		``,
		`VALUES ( 1, 2 )`, // missing FROM (
		`FROM ( SELECT 1 )`,
	)
}

func TestParseSample(t *testing.T) {
	assertValid(t, (*Validator).ParseSample,
		`SAMPLE ( 10 )`,
		`TABLESAMPLE BERNOULLI ( 20 )`,
		`SAMPLE SYSTEM ( 5 ) SEED ( 99 )`,
	)
	assertInvalid(t, (*Validator).ParseSample,
		``,
		`SAMPLE`, // no parens
		`SELECT 1`,
	)
}

func TestParseResample(t *testing.T) {
	assertValid(t, (*Validator).ParseResample,
		`FROM t RESAMPLE ( USING ts INCREMENT BY 1 )`,
		`FROM db.t AS x RESAMPLE ( USING ts INCREMENT BY 5 )`,
		`FROM t alias RESAMPLE ( USING c INCREMENT BY 1 PARTITION BY p )`,
	)
	assertInvalid(t, (*Validator).ParseResample,
		``,
		`FROM t`, // no RESAMPLE
		`RESAMPLE ( USING ts INCREMENT BY 1 )`,
	)
}

func TestParseSemanticView(t *testing.T) {
	assertValid(t, (*Validator).ParseSemanticView,
		`SEMANTIC_VIEW ( my_view )`,
		`SEMANTIC_VIEW ( db.sv METRICS revenue )`,
		`SEMANTIC_VIEW ( sv DIMENSIONS region WHERE region = 'EU' )`,
	)
	assertInvalid(t, (*Validator).ParseSemanticView,
		``,
		`SEMANTIC_VIEW`, // no parens
		`SELECT 1`,
	)
}

func TestParseWhere(t *testing.T) {
	assertValid(t, (*Validator).ParseWhere,
		`WHERE a = 1`,
		`WHERE a > 1 AND b < 2`,
		`WHERE x IN ( 1, 2, 3 )`,
	)
	assertInvalid(t, (*Validator).ParseWhere,
		``,
		`WHERE`, // empty predicate
		`SELECT 1`,
	)
}

func TestParseGroupBy(t *testing.T) {
	assertValid(t, (*Validator).ParseGroupBy,
		`GROUP BY a, b`,
		`GROUP BY ALL`,
		`GROUP BY 1, 2, 3`,
	)
	assertInvalid(t, (*Validator).ParseGroupBy,
		``,
		`GROUP a`, // missing BY
		`ORDER BY a`,
	)
}

func TestParseGroupByCube(t *testing.T) {
	assertValid(t, (*Validator).ParseGroupByCube,
		`GROUP BY CUBE ( a, b )`,
		`GROUP BY x, CUBE ( a, b )`,
		`GROUP BY CUBE ( a )`,
	)
	assertInvalid(t, (*Validator).ParseGroupByCube,
		``,
		`GROUP BY a, b`, // no CUBE
		`GROUP BY CUBE`, // no parens
	)
}

func TestParseGroupByGroupingSets(t *testing.T) {
	assertValid(t, (*Validator).ParseGroupByGroupingSets,
		`GROUP BY GROUPING SETS ( ( a ), ( b ) )`,
		`GROUP BY x, GROUPING SETS ( ( a, b ) )`,
		`GROUP BY GROUPING SETS ( a, b )`,
	)
	assertInvalid(t, (*Validator).ParseGroupByGroupingSets,
		``,
		`GROUP BY a`,             // no GROUPING SETS
		`GROUP BY GROUPING SETS`, // no parens
	)
}

func TestParseGroupByRollup(t *testing.T) {
	assertValid(t, (*Validator).ParseGroupByRollup,
		`GROUP BY ROLLUP ( a, b )`,
		`GROUP BY x, ROLLUP ( a, b )`,
		`GROUP BY ROLLUP ( a )`,
	)
	assertInvalid(t, (*Validator).ParseGroupByRollup,
		``,
		`GROUP BY a, b`,   // no ROLLUP
		`GROUP BY ROLLUP`, // no parens
	)
}

func TestParseHaving(t *testing.T) {
	assertValid(t, (*Validator).ParseHaving,
		`HAVING count(*) > 1`,
		`HAVING sum(x) > 10 AND avg(y) < 5`,
		`HAVING max(a) = 1`,
	)
	assertInvalid(t, (*Validator).ParseHaving,
		``,
		`HAVING`, // empty predicate
		`WHERE a = 1`,
	)
}

func TestParseQualify(t *testing.T) {
	assertValid(t, (*Validator).ParseQualify,
		`QUALIFY row_number() OVER ( ORDER BY a ) = 1`,
		`QUALIFY rank() OVER ( PARTITION BY x ORDER BY y ) <= 3`,
		`QUALIFY count(*) > 1`,
	)
	assertInvalid(t, (*Validator).ParseQualify,
		``,
		`QUALIFY`, // empty predicate
		`HAVING x > 1`,
	)
}

func TestParseOrderBy(t *testing.T) {
	assertValid(t, (*Validator).ParseOrderBy,
		`ORDER BY a, b DESC`,
		`ORDER BY 1 ASC NULLS LAST`,
		`ORDER BY ALL`,
	)
	assertInvalid(t, (*Validator).ParseOrderBy,
		``,
		`ORDER a`, // missing BY
		`GROUP BY a`,
	)
}

func TestParseLimitFetch(t *testing.T) {
	assertValid(t, (*Validator).ParseLimitFetch,
		`LIMIT 10`,
		`LIMIT 10 OFFSET 5`,
		`OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY`,
	)
	assertInvalid(t, (*Validator).ParseLimitFetch,
		``,
		`LIMIT`, // no count
		`ORDER BY a`,
	)
}

func TestParseForUpdate(t *testing.T) {
	assertValid(t, (*Validator).ParseForUpdate,
		`FOR UPDATE`,
		`FOR UPDATE NOWAIT`,
		`FOR UPDATE WAIT 30`,
	)
	assertInvalid(t, (*Validator).ParseForUpdate,
		``,
		`FOR`, // missing UPDATE
		`UPDATE t SET a = 1`,
	)
}
