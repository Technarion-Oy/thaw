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

func TestParseContinue(t *testing.T) {
	assertValid(t, (*Validator).ParseContinue,
		`CONTINUE`,
		`continue`, // case-insensitive
		`ITERATE`,  // synonym
		`iterate`,
		`CONTINUE my_label`,
		`ITERATE my_label`,
		`CONTINUE "My Label"`,
	)
	assertInvalid(t, (*Validator).ParseContinue,
		`CONTINUE a b`,     // two labels
		`CONTINUES`,        // wrong keyword
		`RETURN`,           // not a continue
		`CONTINUE a extra`, // trailing token after label
	)
}

func TestParseRaise(t *testing.T) {
	assertValid(t, (*Validator).ParseRaise,
		`RAISE my_exception`,
		`raise my_exception`, // case-insensitive
		`RAISE "My Exception"`,
		`RAISE`, // re-raise form: exception name omitted inside a handler
	)
	assertInvalid(t, (*Validator).ParseRaise,
		`RAISE a b`,           // two names
		`RAISE my_exc extra`,  // trailing token
		`RAISES my_exception`, // wrong keyword
	)
}

func TestParseReturn(t *testing.T) {
	assertValid(t, (*Validator).ParseReturn,
		`RETURN 1`,
		`return 1`, // case-insensitive
		`RETURN my_variable`,
		`RETURN a + b * 2`,
		`RETURN (SELECT count(*) FROM t)`,
		`RETURN 'done'`,
	)
	assertInvalid(t, (*Validator).ParseReturn,
		`RETURN`,          // missing expression
		`RETURNS 1`,       // wrong keyword
		`RETURN ;`,        // no expression before terminator
		`my_variable`,     // not a return
	)
}

func TestParseCancel(t *testing.T) {
	assertValid(t, (*Validator).ParseCancel,
		`CANCEL my_result_set`,
		`cancel my_result_set`, // case-insensitive
		`CANCEL "My Result Set"`,
	)
	assertInvalid(t, (*Validator).ParseCancel,
		`CANCEL`,                // missing target
		`CANCEL a b`,            // two names
		`CANCEL my_set extra`,   // trailing token
		`CANCELS my_result_set`, // wrong keyword
	)
}

func TestParseClose(t *testing.T) {
	assertValid(t, (*Validator).ParseClose,
		`CLOSE my_cursor`,
		`close my_cursor`, // case-insensitive
		`CLOSE "My Cursor"`,
	)
	assertInvalid(t, (*Validator).ParseClose,
		`CLOSE`,               // missing cursor name
		`CLOSE a b`,           // two names
		`CLOSE my_cursor xtr`, // trailing token
		`CLOSES my_cursor`,    // wrong keyword
	)
}

func TestParseFetch(t *testing.T) {
	assertValid(t, (*Validator).ParseFetch,
		`FETCH my_cursor INTO x`,
		`fetch my_cursor into x`, // case-insensitive
		`FETCH my_cursor INTO x, y, z`,
		`FETCH "My Cursor" INTO "Col A", "Col B"`,
	)
	assertInvalid(t, (*Validator).ParseFetch,
		`FETCH my_cursor`,          // missing INTO
		`FETCH INTO x`,             // missing cursor name
		`FETCH my_cursor INTO`,     // missing variable
		`FETCH my_cursor INTO x,`,  // trailing comma
		`FETCH my_cursor INTO x y`, // missing comma
		`FETCHES my_cursor INTO x`, // wrong keyword
	)
}

func TestParseOpen(t *testing.T) {
	assertValid(t, (*Validator).ParseOpen,
		`OPEN my_cursor`,
		`open my_cursor`, // case-insensitive
		`OPEN "My Cursor"`,
		`OPEN my_cursor USING (v1)`,
		`OPEN my_cursor USING (v1, v2, v3)`,
		`OPEN my_cursor using ("Bind A", "Bind B")`,
	)
	assertInvalid(t, (*Validator).ParseOpen,
		`OPEN`,                         // missing cursor name
		`OPEN a b`,                     // two names
		`OPEN my_cursor extra`,         // trailing token
		`OPENS my_cursor`,              // wrong keyword
		`OPEN my_cursor USING ()`,      // empty bind list
		`OPEN my_cursor USING v1`,      // missing parens
		`OPEN my_cursor USING (v1,)`,   // trailing comma
		`OPEN my_cursor USING (v1 v2)`, // missing comma
	)
}

func TestParseFor(t *testing.T) {
	assertValid(t, (*Validator).ParseFor,
		// Cursor-based.
		`FOR rec IN c1 DO SELECT rec.id; END FOR`,
		`for rec in c1 do select rec.id; end for`, // case-insensitive
		`FOR rec IN c1 DO SELECT 1; END FOR my_label`,
		`FOR rec IN c1 DO SELECT 1; SELECT 2; END FOR`, // multiple statements
		`FOR r IN "My Cursor" DO SELECT 1; END FOR`,
		// Counter-based.
		`FOR i IN 1 TO 10 DO SELECT i; END FOR`,
		`FOR i IN 1 TO 10 LOOP SELECT i; END LOOP`,
		`FOR i IN REVERSE 1 TO 10 DO SELECT i; END FOR`,
		`FOR i IN start_var TO end_var DO SELECT i; END FOR my_label`,
		`FOR i IN 1 TO n LOOP SELECT i; END LOOP counter_loop`,
		// Nested block in the body (inner `;` must not stop the body early).
		`FOR i IN 1 TO 10 DO BEGIN SELECT 1; SELECT 2; END; END FOR`,
	)
	assertInvalid(t, (*Validator).ParseFor,
		`FOR IN c1 DO SELECT 1; END FOR`,         // missing loop variable
		`FOR rec c1 DO SELECT 1; END FOR`,        // missing IN
		`FOR rec IN c1 SELECT 1; END FOR`,        // missing DO
		`FOR i IN 1 10 DO SELECT 1; END FOR`,     // missing TO
		`FOR rec IN c1 DO SELECT 1;`,             // missing END FOR
		`FOR rec IN c1 DO SELECT 1; END`,         // missing FOR/LOOP after END
		`FORS rec IN c1 DO SELECT 1; END FOR`,    // wrong keyword
		`FOR rec IN c1 DO SELECT 1; END FOR a b`, // two labels
	)
}

func TestParseIf(t *testing.T) {
	assertValid(t, (*Validator).ParseIf,
		`IF (a) THEN SELECT 1; END IF`,
		`if (a) then select 1; end if`, // case-insensitive
		`IF (a > 0) THEN SELECT 1; ELSE SELECT 2; END IF`,
		`IF (a) THEN SELECT 1; ELSEIF (b) THEN SELECT 2; END IF`,
		`IF (a) THEN SELECT 1; ELSEIF (b) THEN SELECT 2; ELSEIF (c) THEN SELECT 3; ELSE SELECT 4; END IF`,
		`IF (a) THEN SELECT 1; SELECT 2; END IF`, // multiple statements in a branch
		`IF cond THEN SELECT 1; END IF`,          // parens optional (permissive span)
		// Nested scripting block inside a branch (inner `;` must not stop the body early).
		`IF (a) THEN BEGIN SELECT 1; SELECT 2; END; END IF`,
		// Nested IF inside a branch.
		`IF (a) THEN IF (b) THEN SELECT 1; END IF; ELSE SELECT 2; END IF`,
	)
	assertInvalid(t, (*Validator).ParseIf,
		`IF THEN SELECT 1; END IF`,                           // empty condition
		`IF (a) SELECT 1; END IF`,                            // missing THEN
		`IF (a) THEN SELECT 1; END`,                          // missing IF after END
		`IF (a) THEN SELECT 1;`,                              // missing END IF
		`IFS (a) THEN SELECT 1; END IF`,                      // wrong keyword
		`IF (a) THEN SELECT 1; ELSEIF THEN SELECT 2; END IF`, // empty ELSEIF condition
	)
}

func TestParseLet(t *testing.T) {
	assertValid(t, (*Validator).ParseLet,
		// Variable assignment.
		`LET x := 5`,
		`let x := 5`, // case-insensitive
		`LET x DEFAULT 5`,
		`LET profit NUMBER(38, 2) := 100`, // with type
		`LET name VARCHAR := 'a'`,
		`LET x := a + b * 2`,
		`LET "My Var" := 1`,
		// Cursor assignment.
		`LET c1 CURSOR FOR SELECT id FROM t`,
		`LET c1 CURSOR FOR res`, // for a resultset name
		// RESULTSET assignment.
		`LET rs RESULTSET := (SELECT 1)`,
		`LET rs RESULTSET DEFAULT (SELECT 1)`,
		`LET rs RESULTSET := ASYNC (SELECT 1)`,
	)
	assertInvalid(t, (*Validator).ParseLet,
		`LET x`,                       // missing assignment (required)
		`LET x NUMBER`,                // type but no assignment
		`LET := 5`,                    // missing name
		`LETS x := 5`,                 // wrong keyword
		`LET c1 CURSOR SELECT 1`,      // cursor missing FOR
		`LET rs RESULTSET (SELECT 1)`, // resultset missing assign op
	)
}

func TestParseAssignment(t *testing.T) {
	assertValid(t, (*Validator).ParseAssignment,
		`x := 5`,
		`x := 5`, // case-insensitive keyword-free
		`counter := counter + 1`,
		`profit := revenue - cost`,
		`x := (SELECT count(*) FROM t)`,
		`"My Var" := 'done'`,
		`x := :other`, // right-hand side references another bind variable
	)
	assertInvalid(t, (*Validator).ParseAssignment,
		`x`,           // no assignment
		`x := `,       // missing expression
		`x = 5`,       // `=` is not `:=`
		`x DEFAULT 5`, // DEFAULT is declaration-only, not an update
		`:= 5`,        // missing name
	)
}

// Bind references and IDENTIFIER(:name) as atoms (parseScalar) — issue #648.
func TestParseScalar_Bind(t *testing.T) {
	assertValid(t, (*Validator).parseScalar,
		`:x`,
		`:my_var`,
		`:"My Var"`,
		`IDENTIFIER(:tbl)`,
		`IDENTIFIER('my_table')`,
		`IDENTIFIER(my_table)`,
		// non-bind atoms still work
		`5`, `-5`, `'a'`, `TRUE`, `db.schema.obj`,
	)
	assertInvalid(t, (*Validator).parseScalar,
		`:`,             // colon with no name
		`:=`,            // assignment operator, not a bind
		`IDENTIFIER()`,  // empty wrapper
		`IDENTIFIER(:)`, // bind with no name
	)
}

func TestParseLoop(t *testing.T) {
	assertValid(t, (*Validator).ParseLoop,
		`LOOP SELECT 1; END LOOP`,
		`loop select 1; end loop`,           // case-insensitive
		`LOOP SELECT 1; SELECT 2; END LOOP`, // multiple statements
		`LOOP IF (done) THEN BREAK; END IF; END LOOP`,
		`LOOP SELECT 1; END LOOP my_label`, // trailing label
		// Nested block in the body (inner `;` must not stop the body early).
		`LOOP BEGIN SELECT 1; SELECT 2; END; END LOOP`,
	)
	assertInvalid(t, (*Validator).ParseLoop,
		`LOOP END LOOP`,               // empty body
		`LOOP SELECT 1;`,              // missing END LOOP
		`LOOP SELECT 1; END`,          // missing LOOP after END
		`LOOPS SELECT 1; END LOOP`,    // wrong keyword
		`LOOP SELECT 1; END LOOP a b`, // two labels
	)
}

func TestParseRepeat(t *testing.T) {
	assertValid(t, (*Validator).ParseRepeat,
		`REPEAT SELECT 1; UNTIL (done) END REPEAT`,
		`repeat select 1; until (done) end repeat`,            // case-insensitive
		`REPEAT SELECT 1; SELECT 2; UNTIL (c > 0) END REPEAT`, // multiple statements
		`REPEAT counter := counter + 1; UNTIL (counter >= 10) END REPEAT`,
		`REPEAT SELECT 1; UNTIL (done) END REPEAT my_label`, // trailing label
		`REPEAT IF (x) THEN BREAK; END IF; UNTIL (done) END REPEAT`,
		// Nested block in the body (inner `;` must not stop the body early).
		`REPEAT BEGIN SELECT 1; SELECT 2; END; UNTIL (done) END REPEAT`,
	)
	assertInvalid(t, (*Validator).ParseRepeat,
		`REPEAT UNTIL (done) END REPEAT`,               // empty body
		`REPEAT SELECT 1; END REPEAT`,                  // missing UNTIL
		`REPEAT SELECT 1; UNTIL done END REPEAT`,       // condition not parenthesized
		`REPEAT SELECT 1; UNTIL (done) END`,            // missing REPEAT after END
		`REPEAT SELECT 1; UNTIL (done) END LOOP`,       // wrong END keyword
		`REPEAT SELECT 1; UNTIL (done) END REPEAT a b`, // two labels
	)
	// A REPEAT embedded in a block with trailing statements: the body list must stop
	// at UNTIL, not scan past `END REPEAT` to the next `;`. Exercised via
	// ParseScriptingBlock since the bug only surfaces when REPEAT is not the last
	// statement (a standalone ParseRepeat runs off the end of input regardless).
	assertValid(t, (*Validator).ParseScriptingBlock,
		`BEGIN REPEAT SELECT 1; UNTIL (done) END REPEAT; SELECT 2; END`,
	)
	assertInvalid(t, (*Validator).ParseScriptingBlock,
		`BEGIN REPEAT SELECT 1; UNTIL done END REPEAT; SELECT 2; END`, // condition not parenthesized
		`BEGIN REPEAT SELECT 1; UNTIL (done) END LOOP; SELECT 2; END`, // wrong END keyword
	)
}

func TestParseNull(t *testing.T) {
	assertValid(t, (*Validator).ParseNull,
		`NULL`,
		`null`, // case-insensitive
	)
	assertInvalid(t, (*Validator).ParseNull,
		`NULL extra`, // trailing token
		`NULLS`,      // wrong keyword
		``,           // empty
	)
}

func TestParseCase(t *testing.T) {
	assertValid(t, (*Validator).ParseCase,
		// Searched form.
		`CASE WHEN a THEN SELECT 1; END`,
		`case when a then select 1; end`, // case-insensitive
		`CASE WHEN a THEN SELECT 1; END CASE`,
		`CASE WHEN a > 0 THEN SELECT 1; WHEN a < 0 THEN SELECT 2; END`,
		`CASE WHEN a THEN SELECT 1; ELSE SELECT 2; END`,
		`CASE WHEN a THEN SELECT 1; SELECT 2; END`, // multiple statements in a branch
		// Simple form (operand).
		`CASE (x) WHEN 1 THEN SELECT 1; END`,
		`CASE x WHEN 1 THEN SELECT 1; WHEN 2 THEN SELECT 2; ELSE SELECT 3; END CASE`,
		// Nested scripting block inside a branch (must not stop at its inner `;`).
		`CASE WHEN a THEN BEGIN SELECT 1; SELECT 2; END; END`,
		// A scalar CASE expression embedded in a WHEN condition — inner WHEN/THEN/END
		// must not be mistaken for this statement's boundaries.
		`CASE WHEN CASE WHEN x THEN 1 ELSE 2 END > 0 THEN SELECT 1; END`,
	)
	assertInvalid(t, (*Validator).ParseCase,
		`CASE END`,                             // no WHEN branch
		`CASE WHEN THEN SELECT 1; END`,         // empty condition
		`CASE WHEN a SELECT 1; END`,            // missing THEN
		`CASE WHEN a THEN SELECT 1;`,           // missing END
		`CASES WHEN a THEN SELECT 1; END`,      // wrong keyword
		`CASE WHEN a THEN SELECT 1; END extra`, // trailing token
	)
}

func TestParseException(t *testing.T) {
	assertValid(t, (*Validator).ParseException,
		`EXCEPTION WHEN my_exc THEN SELECT 1;`,
		`exception when my_exc then select 1;`, // case-insensitive
		`EXCEPTION WHEN OTHER THEN SELECT -1;`,
		`EXCEPTION WHEN a OR b OR c THEN SELECT 1;`,
		`EXCEPTION WHEN a THEN SELECT 1; WHEN OTHER THEN SELECT 2;`,
		`EXCEPTION WHEN my_exc THEN SELECT 1; SELECT 2;`, // multiple statements
		// Optional { EXIT | CONTINUE } before THEN.
		`EXCEPTION WHEN my_exc EXIT THEN SELECT 1;`,
		`EXCEPTION WHEN my_exc CONTINUE THEN SELECT 1;`,
		`EXCEPTION WHEN a OR b CONTINUE THEN SELECT 1;`,
		`EXCEPTION WHEN OTHER EXIT THEN SELECT 0;`,
	)
	assertInvalid(t, (*Validator).ParseException,
		`EXCEPTION THEN SELECT 1;`,           // missing WHEN <exception_name>
		`EXCEPTION WHEN my_exc SELECT 1;`,    // missing THEN
		`EXCEPTION WHEN my_exc THEN`,         // missing handler statement
		`WHEN my_exc THEN SELECT 1;`,         // missing EXCEPTION keyword
		`EXCEPTION WHEN a OR THEN SELECT 1;`, // dangling OR
	)
}

func TestParseDeclare(t *testing.T) {
	assertValid(t, (*Validator).ParseDeclare,
		// Variable declarations.
		`DECLARE x INTEGER;`,
		`declare x integer;`, // case-insensitive
		`DECLARE x INT DEFAULT 5;`,
		`DECLARE x NUMBER(38,2) := 0;`,
		`DECLARE profit NUMBER(38, 2) := 0;`,
		`DECLARE x DEFAULT 5;`,      // no type, DEFAULT not swallowed as type
		`DECLARE x := SELECT 1;`,    // := form
		`DECLARE x INT; y VARCHAR;`, // multiple
		`DECLARE flag DEFAULT TRUE;`,
		// Cursor declaration.
		`DECLARE c1 CURSOR FOR SELECT id, price FROM invoices;`,
		// RESULTSET declaration.
		`DECLARE res RESULTSET;`,
		`DECLARE res RESULTSET DEFAULT (SELECT col1 FROM mytable ORDER BY col1);`,
		`DECLARE res RESULTSET := (SELECT 1);`,
		`DECLARE res RESULTSET DEFAULT ASYNC (SELECT 1);`,
		// Exception declaration.
		`DECLARE my_exc EXCEPTION;`,
		`DECLARE my_exc EXCEPTION (-20001, 'boom');`,
		// Nested stored procedure declaration.
		`DECLARE p PROCEDURE (x INT) RETURNS INT AS BEGIN RETURN x; END;`,
		`DECLARE p PROCEDURE () RETURNS TABLE (a INT) AS BEGIN SELECT 1; END;`,
		// Mixed forms in one DECLARE.
		`DECLARE x INT; c CURSOR FOR SELECT 1; e EXCEPTION;`,
	)
	assertInvalid(t, (*Validator).ParseDeclare,
		`DECLARE`,                            // no declarations
		`DECLARE ;`,                          // empty declaration
		`DECLARE x INT`,                      // missing terminating ;
		`DECLARE x INT GARBAGE;`,             // junk after type
		`DECLARE c CURSOR SELECT 1;`,         // cursor missing FOR
		`DECLARE my_exc EXCEPTION (-20001);`, // exception missing message
		`DECLAER x INT;`,                     // misspelled keyword
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
		`BEGIN SELECT 1; EXCEPTION WHEN my_exc CONTINUE THEN SELECT 2; WHEN OTHER EXIT THEN SELECT 3; END`,
		// DECLARE + body + handler together.
		`DECLARE x INT; BEGIN SELECT x; EXCEPTION WHEN OTHER THEN SELECT 0; END`,
		// BREAK / EXIT wired into the block-body statement Choice.
		`BEGIN BREAK; END`,
		`BEGIN EXIT; END`,
		`BEGIN BREAK my_loop; END`,
		`BEGIN SELECT 1; BREAK; END`,
		// CANCEL wired into the block-body statement Choice.
		`BEGIN CANCEL my_rs; END`,
		`BEGIN SELECT 1; CANCEL my_rs; END`,
		// Standalone assignment wired into the block-body statement Choice (issue #648).
		`BEGIN counter := counter + 1; END`,
		`BEGIN SELECT 1; x := 5; SELECT x; END`,
		// Bind reference used inside a body SQL statement (tolerated by the statement span).
		`BEGIN INSERT INTO t (a) VALUES (:x); END`,
		`BEGIN SELECT * FROM IDENTIFIER(:tbl) WHERE c = :x; END`,
		// CASE wired into the block-body statement Choice.
		`BEGIN CASE WHEN a THEN SELECT 1; END CASE; END`,
		`BEGIN CASE x WHEN 1 THEN SELECT 1; ELSE SELECT 2; END; SELECT 3; END`,
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
