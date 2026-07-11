package sqlgrammar

import "testing"

func TestParseAwait(t *testing.T) {
	assertValid(t, (*Validator).ParseAwait,
		`AWAIT ALL`,
		`await all`, // case-insensitive
		`AWAIT my_result_set`,
		`AWAIT "My Result Set"`,
	)
	assertInvalid(t, (*Validator).ParseAwait,
		`AWAIT`,                // missing target
		`AWAIT ALL extra`,      // trailing token
		`AWAIT my_set another`, // two names
		`WAIT ALL`,             // wrong keyword
	)
}

func TestParseBreak(t *testing.T) {
	assertValid(t, (*Validator).ParseBreak,
		`BREAK`,
		`break`, // case-insensitive
		`EXIT`,  // synonym
		`exit`,
		`BREAK my_label`,
		`EXIT my_label`,
		`BREAK "My Label"`,
	)
	assertInvalid(t, (*Validator).ParseBreak,
		`BREAK a b`,     // two labels
		`BREAKS`,        // wrong keyword
		`RETURN`,        // not a break
		`BREAK a extra`, // trailing token after label
	)
}

func TestParseScriptingBlock(t *testing.T) {
	assertValid(t, (*Validator).ParseScriptingBlock,
		`BEGIN SELECT 1; END`,
		`begin select 1; end`, // case-insensitive
		`BEGIN SELECT 1; SELECT 2; END`,
		// DECLARE section (declarations consumed as spans).
		`DECLARE x INTEGER DEFAULT 5; BEGIN RETURN x; END`,
		`DECLARE x INT; y INT; BEGIN SELECT x; END`,
		`DECLARE my_exc EXCEPTION (-20001, 'boom'); BEGIN SELECT 1; END`,
		// Nested block as a statement.
		`BEGIN BEGIN SELECT 1; END; SELECT 2; END`,
		// CASE … END inside a statement must not end the block early.
		`BEGIN SELECT CASE WHEN a THEN 1 ELSE 2 END FROM t; END`,
		// MERGE's bare `WHEN MATCHED …` clauses (depth 0, no CASE/parens) must not
		// be mistaken for a boundary — in the body and in a handler's THEN list.
		`BEGIN MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET x = 1 WHEN NOT MATCHED THEN INSERT (x) VALUES (1); END`,
		`BEGIN SELECT 1; EXCEPTION WHEN OTHER THEN MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET x = 1; END`,
		// A quoted identifier named after a stop word is not a boundary.
		`BEGIN SELECT 1 AS "END"; END`,
		// Exception handler.
		`BEGIN SELECT 1; EXCEPTION WHEN my_exc THEN SELECT 2; END`,
		`BEGIN SELECT 1; EXCEPTION WHEN OTHER THEN SELECT -1; END`,
		`BEGIN SELECT 1; EXCEPTION WHEN a OR b THEN SELECT 2; WHEN OTHER THEN SELECT 3; END`,
		// DECLARE + body + handler together.
		`DECLARE x INT; BEGIN SELECT x; EXCEPTION WHEN OTHER THEN SELECT 0; END`,
		// BREAK / EXIT wired into the block-body statement Choice.
		`BEGIN BREAK; END`,
		`BEGIN EXIT; END`,
		`BEGIN BREAK my_loop; END`,
		`BEGIN SELECT 1; BREAK; END`,
	)
	assertInvalid(t, (*Validator).ParseScriptingBlock,
		`BEGIN END`,                   // empty body — needs a statement
		`BEGIN SELECT 1 END`,          // missing statement terminator
		`DECLARE BEGIN SELECT 1; END`, // DECLARE with no declarations
		`BEGIN SELECT 1;`,             // missing END
		`SELECT 1; END`,               // missing BEGIN
		`BEGIN SELECT 1; EXCEPTION THEN SELECT 2; END`,        // handler missing WHEN <exc>
		`BEGIN SELECT 1; EXCEPTION WHEN my_exc SELECT 2; END`, // handler missing THEN
	)
}

func TestScriptingBlock_TopLevel(t *testing.T) {
	// Recognized + dispatched as a top-level unit under BEGIN / DECLARE.
	for _, sql := range []string{
		`BEGIN SELECT 1; END`,
		`BEGIN SELECT 1; END;`, // trailing semicolon tolerated
		`DECLARE x INT; BEGIN SELECT x; END`,
	} {
		if !topLevelOK(sql) {
			v := New(sql)
			v.ParseTopLevel()
			t.Errorf("expected top-level VALID for %q: %s", sql, v.Failure().Message())
		}
	}
	// The transaction BEGIN must still validate (dispatched on the same key).
	if !topLevelOK(`BEGIN`) {
		t.Errorf("transaction BEGIN regressed under the shared BEGIN dispatch key")
	}
}
