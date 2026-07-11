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
		func() bool { return v.consumeStmtSpan("END", "EXCEPTION", "WHEN") },
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
