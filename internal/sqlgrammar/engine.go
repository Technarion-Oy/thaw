package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

// Validator is the recursive-descent / pushdown-automaton state for one statement.
//
// src is retained so token text can be recovered via sqltok.Token.Text(src)
// (sqltok.Token has no Value field). tokens holds significant tokens only
// (sqltok.Significant output — trivia and EOF already dropped). pos is the current
// cursor; furthest/expected track the furthest position reached and what the
// grammar expected there, which powers both diagnostics (error messages) and
// autocomplete (the "valid next" set).
//
// The terminals (Match/MatchKeyword/MatchWord/MatchOp) and combinators
// (Sequence/Choice/Optional/ZeroOrMore) are implemented per issue #556. The
// per-command Parse* rules are filled in incrementally: every CREATE rule
// (create.go) is implemented; the other statement families still stub to true.
type Validator struct {
	src      string
	tokens   []sqltok.Token // significant tokens only
	pos      int
	furthest int      // furthest pos reached — for error/expected reporting
	expected []string // what was expected at furthest (keywords/kinds)
}

// New builds a Validator over src, tokenizing into significant tokens.
func New(src string) *Validator {
	return &Validator{src: src, tokens: sqltok.SignificantTokens(src)}
}

// -- State management --

// Peek returns the token at the current cursor, or the EOF sentinel when the
// cursor has run past the end of the significant-token slice.
func (v *Validator) Peek() sqltok.Token {
	if v.pos >= len(v.tokens) {
		return sqltok.Token{Kind: sqltok.EOF}
	}
	return v.tokens[v.pos]
}

// AtEnd reports whether every significant token has been consumed. Top-level
// callers use it to reject trailing tokens after an otherwise valid statement.
func (v *Validator) AtEnd() bool { return v.pos >= len(v.tokens) }

func (v *Validator) advance() {
	if v.pos < len(v.tokens) {
		v.pos++
	}
	if v.pos > v.furthest {
		v.furthest, v.expected = v.pos, nil
	}
}

func (v *Validator) save() int     { return v.pos }
func (v *Validator) restore(p int) { v.pos = p }

// expect records what the grammar was looking for at the current position, so a
// failed parse can report (diagnostics) or complete (autocomplete) the expected
// set. Only labels recorded at the furthest position reached are retained.
func (v *Validator) expect(label string) {
	if v.pos == v.furthest {
		v.expected = append(v.expected, label)
	}
}

// Failure describes why a parse stopped: the furthest token reached and the set
// of things the grammar expected there. Tok is the EOF sentinel when the parser
// ran off the end of the statement (furthest >= len(tokens)); Found then is "".
type Failure struct {
	Tok      sqltok.Token // furthest token reached
	Found    string       // text of Tok ("" when the parser ran off the end)
	Expected []string     // distinct labels expected at Tok (keywords / kind names)
}

// Failure returns the furthest-position failure info. Call it after a top-level
// parse returns false. Expected is de-duplicated and order-preserving.
func (v *Validator) Failure() Failure {
	tok := sqltok.Token{Kind: sqltok.EOF}
	found := ""
	if v.furthest < len(v.tokens) {
		tok = v.tokens[v.furthest]
		found = tok.Text(v.src)
	}
	seen := make(map[string]struct{}, len(v.expected))
	uniq := v.expected[:0:0]
	for _, e := range v.expected {
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		uniq = append(uniq, e)
	}
	return Failure{Tok: tok, Found: found, Expected: uniq}
}

// Message renders a human-readable diagnostic naming both what was found and what
// the grammar expected there, e.g. `unexpected 'GROUP', expected FROM` or
// `unexpected end of statement, expected one of: FROM, (`. Naming the offending
// token (the furthest one reached, not token 0 — backtracking rewinds the cursor)
// is what makes the message actionable.
func (f Failure) Message() string {
	found := "end of statement"
	if f.Found != "" {
		found = "'" + f.Found + "'"
	}
	switch len(f.Expected) {
	case 0:
		return "unexpected " + found
	case 1:
		return "unexpected " + found + ", expected " + f.Expected[0]
	default:
		return "unexpected " + found + ", expected one of: " + strings.Join(f.Expected, ", ")
	}
}

// -- Terminals --

// Match consumes the current token if it is of kind, otherwise records kind as
// expected and leaves the cursor unmoved.
func (v *Validator) Match(kind sqltok.TokenKind) bool {
	if v.Peek().Kind == kind {
		v.advance()
		return true
	}
	v.expect(kind.String())
	return false
}

// MatchKeyword matches a token tagged sqltok.Keyword whose text equals word
// (case-insensitive). Keywords are classified by the lexer's keyword map; text
// is recovered via Token.Text(v.src) — there is no Token.Value.
func (v *Validator) MatchKeyword(word string) bool {
	t := v.Peek()
	if t.Kind == sqltok.Keyword && strings.EqualFold(t.Text(v.src), word) {
		v.advance()
		return true
	}
	v.expect(word)
	return false
}

// MatchWord matches word (case-insensitive) against any identifier-like token —
// a Keyword, bare Identifier, or QuotedIdent. Many Snowflake clause words and
// option names (IGNORE, LISTING, DATA_RETENTION_TIME_IN_DAYS, …) are not in the
// lexer's keyword map, so they arrive as Identifier; this matcher accepts a word
// regardless of that classification.
func (v *Validator) MatchWord(word string) bool {
	t := v.Peek()
	if t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), word) {
		v.advance()
		return true
	}
	v.expect(word)
	return false
}

// MatchOp matches an Operator token by text (e.g. "=", "=>", "::").
func (v *Validator) MatchOp(op string) bool {
	t := v.Peek()
	if t.Kind == sqltok.Operator && t.Text(v.src) == op {
		v.advance()
		return true
	}
	v.expect(op)
	return false
}

// -- Grammar combinators (the backtracking "machine") --

// Rule is a parse step that consumes zero or more tokens and reports success.
type Rule func() bool

// Sequence requires every rule to match in order; on any failure it rewinds the
// cursor to where the sequence began so no partial consumption leaks out.
func (v *Validator) Sequence(rules ...Rule) bool {
	saved := v.save()
	for _, r := range rules {
		if !r() {
			v.restore(saved)
			return false
		}
	}
	return true
}

// Optional always succeeds; it rewinds if the inner rule fails so an absent
// optional clause consumes nothing.
func (v *Validator) Optional(r Rule) bool {
	saved := v.save()
	if !r() {
		v.restore(saved)
	}
	return true
}

// Choice tries each alternative in order, rewinding between attempts, and
// returns true at the first that matches.
func (v *Validator) Choice(rules ...Rule) bool {
	for _, r := range rules {
		saved := v.save()
		if r() {
			return true
		}
		v.restore(saved)
	}
	return false
}

// ZeroOrMore applies r until it stops matching, rewinding the final failed
// attempt. It always succeeds. The zero-progress guard stops when an iteration
// succeeds without consuming a token: such a body (e.g. an all-Optional Sequence,
// or a Choice with an empty-matchable branch) would otherwise spin forever and
// hang the diagnostics goroutine. Breaking is always correct — a rule that
// relies on looping over a non-consuming body is already a bug.
func (v *Validator) ZeroOrMore(r Rule) bool {
	for {
		saved := v.save()
		if !r() {
			v.restore(saved)
			break
		}
		if v.save() == saved {
			break
		}
	}
	return true
}

// unorderedOnce parses a set of order-independent options, each at most once. It
// loops like ZeroOrMore but, on every pass, tries only the options not yet
// matched. Matching an option marks it used, which has two effects:
//
//   - A duplicate is rejected — the second occurrence matches no remaining option,
//     so the loop stops before it and the statement fails to fully parse
//     (diagnostics then flag the repeat).
//   - Because only un-matched options run their leading MatchWord at the cursor,
//     ExpectedAt stops offering an option already present — so autocomplete no
//     longer suggests, e.g., INCREMENT once `INCREMENT = 1` is typed.
//
// Each option's rule must consume at least its leading keyword on success (the
// ZeroOrMore zero-progress guard otherwise stops the loop). Mutually-exclusive
// keywords (ORDER | NOORDER) belong in one option so matching either retires both.
// It always succeeds — zero matched options is allowed.
func (v *Validator) unorderedOnce(opts ...Rule) bool {
	used := make([]bool, len(opts))
	return v.ZeroOrMore(func() bool {
		for i, opt := range opts {
			if used[i] {
				continue
			}
			saved := v.save()
			if opt() {
				used[i] = true
				return true
			}
			v.restore(saved)
		}
		return false
	})
}

// -- Shared name / value helpers --

// parseIdentPath consumes a (possibly dot-qualified) name such as DB.SCHEMA.OBJ
// using the existing sqltok helper, recording "identifier" as expected on miss.
func (v *Validator) parseIdentPath() bool {
	_, next := sqltok.ReadIdentParts(v.tokens, v.src, v.pos, 0 /* unbounded */)
	if next == v.pos {
		v.expect("identifier")
		return false
	}
	v.pos = next
	if v.pos > v.furthest {
		v.furthest, v.expected = v.pos, nil
	}
	return true
}

// parseBool matches a TRUE / FALSE literal.
func (v *Validator) parseBool() bool {
	return v.Choice(
		func() bool { return v.MatchWord("TRUE") },
		func() bool { return v.MatchWord("FALSE") },
	)
}

// parseScalar matches a single scalar value: a string/number literal (with an
// optional leading sign), a boolean, a Snowflake Scripting bind reference
// (`:<name>` / `IDENTIFIER(:<name>)`), or an identifier path. It is the catch-all
// right-hand side for `<option> = <value>` and `=> <value>` assignments.
func (v *Validator) parseScalar() bool {
	v.Optional(func() bool {
		return v.Choice(
			func() bool { return v.MatchOp("+") },
			func() bool { return v.MatchOp("-") },
		)
	})
	return v.Choice(
		func() bool { return v.Match(sqltok.StringLit) },
		func() bool { return v.Match(sqltok.NumberLit) },
		v.parseBool,
		v.parseBindVar,       // :<name>  (scripting variable reference; issue #648)
		v.parseIdentifierFn,  // IDENTIFIER( … ) wrapper — before parseIdentPath
		v.parseIdentPath,
	)
}

// parseBindVar matches a Snowflake Scripting bind reference `:<name>` — a Colon
// followed by an identifier — used to reference a scripting variable inside a SQL
// statement (VALUES lists, WHERE, function args, and array-spread bind lists). The
// `:=` assignment operator is a Colon followed by `=` (an Operator), so a Colon
// leading an identifier is unambiguously a bind, not an assignment.
// Reference: https://docs.snowflake.com/en/developer-guide/snowflake-scripting/variables
func (v *Validator) parseBindVar() bool {
	return v.Sequence(
		func() bool { return v.Match(sqltok.Colon) },
		v.parseIdentPath,
	)
}

// parseIdentifierFn matches Snowflake's `IDENTIFIER( <arg> )` wrapper, which
// resolves a bind reference, string literal, or name to an object name where a
// table/object name is expected. Accepted as an atom so `IDENTIFIER(:tbl)` parses
// anywhere a scalar/name does.
func (v *Validator) parseIdentifierFn() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("IDENTIFIER") },
		func() bool { return v.Match(sqltok.LParen) },
		func() bool { return v.Choice(v.parseBindVar, v.parseString, v.parseIdentPath) },
		func() bool { return v.Match(sqltok.RParen) },
	)
}

// option builds a rule matching `<key> = <value>` where value is produced by
// valueRule. The key is matched word-kind-agnostically (see MatchWord).
func (v *Validator) option(key string, valueRule Rule) Rule {
	return func() bool {
		return v.Sequence(
			func() bool { return v.MatchWord(key) },
			func() bool { return v.MatchOp("=") },
			valueRule,
		)
	}
}

// wordsValue builds a rule matching exactly one of the given keyword choices,
// e.g. `{ COMPATIBLE | OPTIMIZED }`.
func (v *Validator) wordsValue(words ...string) Rule {
	return func() bool {
		alts := make([]Rule, len(words))
		for i, w := range words {
			alts[i] = func() bool { return v.MatchWord(w) }
		}
		return v.Choice(alts...)
	}
}

// parseString matches a single-quoted string literal.
func (v *Validator) parseString() bool { return v.Match(sqltok.StringLit) }

// matchIdentLike consumes any single identifier-like token (Identifier, Keyword,
// or QuotedIdent). Used as the key of an open-ended `<option> = <value>` where the
// option name is not from a fixed set and some names are lexer keywords — e.g. the
// COPY INTO copyOptions PURGE / FORCE, which arrive as Keyword, not Identifier.
func (v *Validator) matchIdentLike() bool {
	if v.Peek().Kind.IsIdentLike() {
		v.advance()
		return true
	}
	v.expect("identifier")
	return false
}

// parseStageRef matches a stage reference `@[<namespace>.]<stage>[/<path>]`,
// consuming the `/path` suffix by only accepting tokens directly adjacent to the
// previous one (no intervening whitespace), so it stops before the next clause
// word (FROM, PATTERN, …). Shared by COPY INTO and the CREATE rules that take a
// LOCATION = @stage/path/ value.
func (v *Validator) parseStageRef() bool {
	at := v.Peek()
	if !v.Match(sqltok.At) {
		return false
	}
	lastEnd := at.End
	matched := false
	for !v.AtEnd() {
		t := v.Peek()
		if t.Start != lastEnd {
			break
		}
		ok := t.Kind.IsIdentLike() || t.Kind == sqltok.Dot || t.Kind == sqltok.NumberLit ||
			(t.Kind == sqltok.Operator && (t.Text(v.src) == "/" || t.Text(v.src) == "%")) ||
			(t.Kind == sqltok.Other && t.Text(v.src) == "~")
		if !ok {
			break
		}
		lastEnd = t.End
		v.advance()
		matched = true
	}
	return matched
}

// clusterByClause matches `CLUSTER BY [ LINEAR ] <parens>`. Snowflake accepts the
// optional LINEAR clustering-function keyword before the expression list, and
// SHOW TABLES metadata round-trips DDL as `CLUSTER BY LINEAR(...)`.
func (v *Validator) clusterByClause(parens Rule) Rule {
	return func() bool {
		return v.Sequence(
			func() bool { return v.phrase("CLUSTER", "BY") },
			func() bool { return v.Optional(func() bool { return v.MatchWord("LINEAR") }) },
			parens,
		)
	}
}

// parseNumber matches a numeric literal (optionally signed).
func (v *Validator) parseNumber() bool {
	v.Optional(func() bool {
		return v.Choice(
			func() bool { return v.MatchOp("+") },
			func() bool { return v.MatchOp("-") },
		)
	})
	return v.Match(sqltok.NumberLit)
}

// phrase matches the given words in order (case-insensitive, kind-agnostic),
// e.g. phrase("IF","NOT","EXISTS"). It backtracks fully on any miss.
func (v *Validator) phrase(words ...string) bool {
	rules := make([]Rule, len(words))
	for i, w := range words {
		rules[i] = func() bool { return v.MatchWord(w) }
	}
	return v.Sequence(rules...)
}

// orReplace matches the optional `OR { REPLACE | ALTER }` modifier (CREATE OR
// REPLACE and the CREATE OR ALTER convergence form).
func (v *Validator) orReplace() bool {
	return v.Optional(func() bool {
		return v.Sequence(
			func() bool { return v.MatchKeyword("OR") },
			v.wordsValue("REPLACE", "ALTER"),
		)
	})
}

// ifNotExists matches the optional `IF NOT EXISTS` clause.
func (v *Validator) ifNotExists() bool {
	return v.Optional(func() bool { return v.phrase("IF", "NOT", "EXISTS") })
}

// ifExists matches the optional `IF EXISTS` clause.
func (v *Validator) ifExists() bool {
	return v.Optional(func() bool { return v.phrase("IF", "EXISTS") })
}

// commentOption returns a rule matching `COMMENT = '<string>'`.
func (v *Validator) commentOption() Rule {
	return v.option("COMMENT", v.parseString)
}

// tagClause matches the standard `[ WITH ] TAG ( <name> = '<value>' [ , ... ] )`
// clause shared by many CREATE commands.
func (v *Validator) tagClause() bool {
	return v.Sequence(
		func() bool { return v.Optional(func() bool { return v.MatchWord("WITH") }) },
		func() bool { return v.MatchWord("TAG") },
		func() bool {
			return v.parseParenList(v.option2(v.parseIdentPath, v.parseString))
		},
	)
}

// -- Permissive consumers (for spans too detailed to model fully) --

// consumeBalancedParens matches a `( … )` group with arbitrary balanced content,
// including nested parens. Use for option/column/constraint lists that are not
// worth modeling token-for-token. Fails if there is no opening paren or the
// parens are unbalanced before end-of-statement.
func (v *Validator) consumeBalancedParens() bool {
	if !v.Match(sqltok.LParen) {
		return false
	}
	depth := 1
	for depth > 0 {
		if v.AtEnd() {
			return false
		}
		switch v.Peek().Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		}
		v.advance()
	}
	return true
}

// consumeRest consumes every remaining token and always succeeds (even on an
// empty tail). Use for free-form trailing spans such as `AS <query>`, a stored
// procedure body, or a policy expression.
func (v *Validator) consumeRest() bool {
	for !v.AtEnd() {
		v.advance()
	}
	return true
}

// -- SHOW / DESCRIBE trailing-clause helpers (shared by ~180 SHOW commands) --

// likeClause matches `LIKE '<pattern>'`.
func (v *Validator) likeClause() bool {
	return v.Sequence(func() bool { return v.MatchWord("LIKE") }, v.parseString)
}

// inScopeClause matches the common object-scope clause, leniently:
//
//	IN { ACCOUNT | ORGANIZATION | APPLICATION [ PACKAGE ] [ <name> ]
//	     | DATABASE [ <name> ] | SCHEMA [ <name> ] | TABLE [ <name> ]
//	     | VIEW [ <name> ] | CLASS [ <name> ] | <name> }
func (v *Validator) inScopeClause() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("IN") },
		func() bool {
			return v.Choice(
				func() bool { return v.MatchWord("ACCOUNT") },
				func() bool { return v.MatchWord("ORGANIZATION") },
				func() bool {
					return v.Sequence(
						v.wordsValue("APPLICATION", "DATABASE", "SCHEMA", "TABLE", "VIEW", "CLASS", "MODEL"),
						func() bool { return v.Optional(func() bool { return v.MatchWord("PACKAGE") }) },
						func() bool { return v.Optional(v.parseIdentPath) },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}

// startsWithClause matches `STARTS WITH '<string>'`.
func (v *Validator) startsWithClause() bool {
	return v.Sequence(func() bool { return v.phrase("STARTS", "WITH") }, v.parseString)
}

// limitFromClause matches `LIMIT <rows> [ FROM '<string>' ]`.
func (v *Validator) limitFromClause() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("LIMIT") },
		func() bool { return v.Match(sqltok.NumberLit) },
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(func() bool { return v.MatchWord("FROM") }, v.parseString)
			})
		},
	)
}

// showTrailers consumes any number of the common trailing SHOW clauses in any
// order: LIKE, IN <scope>, STARTS WITH, LIMIT … FROM, and the bare HISTORY flag.
func (v *Validator) showTrailers() bool {
	return v.ZeroOrMore(func() bool {
		return v.Choice(
			v.likeClause,
			v.inScopeClause,
			v.startsWithClause,
			v.limitFromClause,
			func() bool { return v.MatchWord("HISTORY") },
		)
	})
}
