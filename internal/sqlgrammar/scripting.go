package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

// Snowflake Scripting — grammar rules for the procedural constructs that appear
// inside a scripting block (`DECLARE … BEGIN … EXCEPTION … END`). These form a
// separate grammar layer from the standalone-statement grammars and are wired
// into the block-body statement `Choice` (parseScriptingStatement) — the set of
// statements allowed inside `BEGIN … END` — rather than into top-level dispatch,
// except for the block itself which is dispatched under BEGIN and DECLARE.

// ParseScriptingBlock validates a Snowflake Scripting block — the structural core
// of the scripting grammar (issue #629).
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/begin
//
// Syntax:
//
//	[ DECLARE
//	    <declarations> ]
//	BEGIN
//	    <statement>;
//	    [ <statement>; ... ]
//	[ EXCEPTION <exception_handler> ]
//	END
//
// It recurses (a nested block is a statement — see parseScriptingStatement) and is
// dispatched under both BEGIN and DECLARE. It is DISTINCT from the transaction
// ParseBegin (`BEGIN [ { WORK | TRANSACTION } ]`, transactions.go); both register
// on the BEGIN key and ParseTopLevel accepts whichever fully consumes the input.
// (It cannot reuse the name `ParseBegin` — Go forbids two methods with one name.)
//
// ponytail: label support (`<<label>> … END label`) is skipped — the reference
// syntax above omits it; add it if labeled blocks turn up.
func (v *Validator) ParseScriptingBlock() bool {
	return v.Sequence(
		v.parseDeclareSection, // optional leading DECLARE
		func() bool { return v.MatchWord("BEGIN") },
		v.parseScriptingStmtList, // required <statement>; [ … ]
		v.parseExceptionHandler,  // optional trailing EXCEPTION
		func() bool { return v.MatchWord("END") },
	)
}

// parseScriptingStatement parses ONE statement permitted inside a scripting block
// body — the SHARED entry point the loop/branch/cursor issues (#556 family) extend
// by inserting their construct as a leading Choice branch ahead of the catch-all.
// Today it recognizes a nested scripting block and, as a catch-all, any single
// statement span (a plain SQL statement or an as-yet-unmodeled scripting statement).
func (v *Validator) parseScriptingStatement() bool {
	return v.Choice(
		v.ParseScriptingBlock,
		v.ParseBreak,
		v.ParseCancel,
		v.ParseCase,
		v.ParseClose,
		v.ParseContinue,
		// ELSE joins END/EXCEPTION/WHEN as a leading boundary so a CASE branch body
		// (THEN … / ELSE …) ends at the next branch. No plain statement legally starts
		// with any of these words, so the extra stop is harmless in a non-CASE body.
		func() bool { return v.consumeStmtSpan("END", "EXCEPTION", "WHEN", "ELSE") },
	)
}

// parseScriptingStmtList matches the required block body: at least one
// semicolon-terminated statement.
func (v *Validator) parseScriptingStmtList() bool {
	item := func() bool {
		return v.Sequence(
			v.parseScriptingStatement,
			func() bool { return v.Match(sqltok.Semicolon) },
		)
	}
	return v.Sequence(item, func() bool { return v.ZeroOrMore(item) })
}

// parseDeclareSection matches the optional leading `DECLARE <declarations>` — one
// or more semicolon-terminated declaration spans.
//
// ponytail: the individual declaration grammar (variable / cursor / exception /
// RESULTSET declarations) is a later issue, so each declaration is consumed as a
// permissive span up to its `;` (stopping only at the block's BEGIN). Replace the
// span with real declaration rules when those issues land.
func (v *Validator) parseDeclareSection() bool {
	return v.Optional(func() bool {
		decl := func() bool {
			return v.Sequence(
				func() bool { return v.consumeStmtSpan("BEGIN") },
				func() bool { return v.Match(sqltok.Semicolon) },
			)
		}
		return v.Sequence(
			func() bool { return v.MatchWord("DECLARE") },
			decl,
			func() bool { return v.ZeroOrMore(decl) },
		)
	})
}

// parseExceptionHandler matches the optional trailing exception handler:
//
//	EXCEPTION
//	  WHEN <exception> [ OR <exception> ... ] THEN <statement>; [ <statement>; ... ]
//	  [ WHEN ... THEN ... ]
//
// The `WHEN OTHER THEN` catch-all is just a WHEN whose <exception> is the reserved
// name OTHER, so it needs no special case.
func (v *Validator) parseExceptionHandler() bool {
	when := func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("WHEN") },
			v.parseIdentPath, // <exception> (OTHER is a valid name here)
			func() bool {
				return v.ZeroOrMore(func() bool {
					return v.Sequence(func() bool { return v.MatchWord("OR") }, v.parseIdentPath)
				})
			},
			func() bool { return v.MatchWord("THEN") },
			v.parseScriptingStmtList,
		)
	}
	return v.Optional(func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord("EXCEPTION") },
			when,
			func() bool { return v.ZeroOrMore(when) },
		)
	})
}

// consumeStmtSpan consumes one statement's tokens up to — but not including — its
// terminating semicolon. A stop word (an enclosing block/handler keyword: END,
// EXCEPTION, the next WHEN) ends the span ONLY when it is the LEADING token — i.e.
// the statement list has run out of statements and reached the boundary. Mid-
// statement the same word belongs to the statement and is consumed, which is what
// lets MERGE's bare `WHEN MATCHED …` / `WHEN NOT MATCHED …` clauses — and a CASE
// expression's WHEN/END — sit inside a block body without cutting the span short.
// A `;` only terminates outside parentheses (a subquery cannot contain one, but
// the paren guard is kept defensively). It requires at least one token, so an
// empty statement fails.
//
// ponytail: a permissive span, NOT a real parse — precise per-construct rules
// (LET/IF/FOR/RETURN/…) supersede it as their issues land, inserted ahead of the
// catch-all branch in parseScriptingStatement.
func (v *Validator) consumeStmtSpan(stops ...string) bool {
	start := v.pos
	paren := 0
	for !v.AtEnd() {
		t := v.Peek()
		if v.pos == start && v.isWord(t, stops...) {
			break
		}
		if t.Kind == sqltok.Semicolon && paren == 0 {
			break
		}
		switch t.Kind {
		case sqltok.LParen:
			paren++
		case sqltok.RParen:
			if paren > 0 {
				paren--
			}
		}
		v.advance()
	}
	if v.pos == start {
		v.expect("statement")
		return false
	}
	return true
}

// isWord reports whether t is an UNQUOTED identifier-like token (Keyword or bare
// Identifier) whose text equals (case-insensitively) any of words. QuotedIdent is
// excluded: a structural keyword is never quoted, so a legal quoted alias like
// `"END"` or `"WHEN"` is not mistaken for a block/handler boundary.
func (v *Validator) isWord(t sqltok.Token, words ...string) bool {
	if t.Kind != sqltok.Keyword && t.Kind != sqltok.Identifier {
		return false
	}
	txt := t.Text(v.src)
	for _, w := range words {
		if strings.EqualFold(txt, w) {
			return true
		}
	}
	return false
}

// ParseAwait validates the Snowflake Scripting `AWAIT` construct.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/await
//
// Syntax:
//
//	AWAIT { ALL | <result_set_name> }
func (v *Validator) ParseAwait() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("AWAIT") },
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("ALL") },
				v.parseIdentPath,
			)
		},
	)
}

// ParseBreak validates the Snowflake Scripting `BREAK` construct (EXIT is a synonym)
// — terminates a loop, optionally the enclosing loop named by a label.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/break
//
// Syntax:
//
//	{ BREAK | EXIT } [ <label> ]
//
// (The terminating `;` belongs to the block-body statement list, not this rule.)
func (v *Validator) ParseBreak() bool {
	return v.Sequence(
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("BREAK") },
				func() bool { return v.MatchWord("EXIT") },
			)
		},
		func() bool { return v.Optional(v.parseIdentPath) }, // optional <label>
	)
}

// ParseCase validates the Snowflake Scripting `CASE` construct — both the simple
// form (matches an operand against each WHEN expression) and the searched form
// (evaluates each WHEN boolean). It is a block-body statement, not top-level.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/case
//
// Syntax:
//
//	-- Simple
//	CASE ( <expression_to_match> )
//	    WHEN <expression> THEN <statement>; [ <statement>; ... ]
//	    [ WHEN ... ]
//	    [ ELSE <statement>; [ <statement>; ... ] ]
//	END [ CASE ]
//
//	-- Searched
//	CASE
//	    WHEN <boolean_expression> THEN <statement>; [ <statement>; ... ]
//	    [ WHEN ... ]
//	    [ ELSE <statement>; [ <statement>; ... ] ]
//	END [ CASE ]
//
// The optional operand distinguishes the two forms: present → simple, absent →
// searched. Operand and WHEN conditions are consumed as expression spans (there is
// no full expression grammar yet). The terminating `;` belongs to the block-body
// statement list, not this rule.
func (v *Validator) ParseCase() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("CASE") },
		// Simple-form operand `( <expr> )` (or a bare expr) up to the first WHEN;
		// searched form has none, so the span is empty and Optional rewinds.
		func() bool { return v.Optional(func() bool { return v.consumeExprSpan("WHEN") }) },
		v.parseCaseWhen, // required first WHEN … THEN …
		func() bool { return v.ZeroOrMore(v.parseCaseWhen) }, // additional WHEN branches
		func() bool { return v.Optional(v.parseCaseElse) },   // optional ELSE branch
		func() bool { return v.MatchWord("END") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("CASE") }) }, // END [ CASE ]
	)
}

// parseCaseWhen matches one `WHEN <expression> THEN <statement>; [ … ]` branch.
func (v *Validator) parseCaseWhen() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("WHEN") },
		func() bool { return v.consumeExprSpan("THEN") },
		func() bool { return v.MatchWord("THEN") },
		v.parseScriptingStmtList,
	)
}

// parseCaseElse matches the trailing `ELSE <statement>; [ … ]` branch.
func (v *Validator) parseCaseElse() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("ELSE") },
		v.parseScriptingStmtList,
	)
}

// consumeExprSpan consumes an expression's tokens up to — but not including — the
// first `stop` keyword that sits at paren depth 0 and outside any nested CASE … END.
// Tracking CASE nesting lets a WHEN condition or operand embedding a scalar
// `CASE … WHEN … THEN … END` expression not be cut short by the inner WHEN/THEN. A
// top-level `;` also ends the span (an expression never spans a statement terminator),
// which keeps a missing THEN failing at the right spot. Requires at least one token,
// so an immediate stop word yields an empty span and fails.
//
// ponytail: a permissive span, NOT a real expression parse — replace with the
// expression grammar once it lands.
func (v *Validator) consumeExprSpan(stop string) bool {
	start := v.pos
	paren, caseDepth := 0, 0
	for !v.AtEnd() {
		t := v.Peek()
		if paren == 0 && caseDepth == 0 && v.isWord(t, stop) {
			break
		}
		if t.Kind == sqltok.Semicolon && paren == 0 {
			break
		}
		switch {
		case t.Kind == sqltok.LParen:
			paren++
		case t.Kind == sqltok.RParen:
			if paren > 0 {
				paren--
			}
		case paren == 0 && v.isWord(t, "CASE"):
			caseDepth++
		case paren == 0 && caseDepth > 0 && v.isWord(t, "END"):
			caseDepth--
		}
		v.advance()
	}
	if v.pos == start {
		v.expect("expression")
		return false
	}
	return true
}

// ParseContinue validates the Snowflake Scripting `CONTINUE` construct (ITERATE is a
// synonym) — skips the rest of the current loop iteration and proceeds to the next,
// optionally the enclosing loop named by a label.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/continue
//
// Syntax:
//
//	{ CONTINUE | ITERATE } [ <label> ]
//
// (The terminating `;` belongs to the block-body statement list, not this rule.)
func (v *Validator) ParseContinue() bool {
	return v.Sequence(
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("CONTINUE") },
				func() bool { return v.MatchWord("ITERATE") },
			)
		},
		func() bool { return v.Optional(v.parseIdentPath) }, // optional <label>
	)
}

// ParseCancel validates the Snowflake Scripting `CANCEL` construct — terminates the
// asynchronous child job running for a RESULTSET.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/cancel
//
// Syntax:
//
//	CANCEL <result_set_name>
//
// (The terminating `;` belongs to the block-body statement list, not this rule.)
func (v *Validator) ParseCancel() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("CANCEL") },
		v.parseIdentPath, // required <result_set_name>
	)
}

// ParseClose validates the Snowflake Scripting `CLOSE` construct — closes a cursor,
// ending access and invalidating its row pointer.
// Reference: https://docs.snowflake.com/en/sql-reference/snowflake-scripting/close
//
// Syntax:
//
//	CLOSE <cursor_name>
//
// (The terminating `;` belongs to the block-body statement list, not this rule.)
func (v *Validator) ParseClose() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("CLOSE") },
		v.parseIdentPath, // required <cursor_name>
	)
}
