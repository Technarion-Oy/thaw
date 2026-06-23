package sqlgrammar

import "thaw/internal/sqltok"

// Validator is the recursive-descent / pushdown-automaton state for one statement.
//
// src is retained so token text can be recovered via sqltok.Token.Text(src)
// (sqltok.Token has no Value field). tokens holds significant tokens only
// (sqltok.Significant output — trivia and EOF already dropped). pos is the current
// cursor; furthest/expected track the furthest position reached and what the
// grammar expected there, which powers both diagnostics (error messages) and
// autocomplete (the "valid next" set).
//
// The terminals (Match/MatchKeyword/MatchOp), combinators (Sequence/Choice/
// Optional/ZeroOrMore) and the dispatch (Recognized/ParseTopLevel) are implemented
// per issue #556; the per-command Parse* rules currently stub to return true.
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
