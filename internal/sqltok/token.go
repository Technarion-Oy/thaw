// SPDX-License-Identifier: GPL-3.0-or-later

package sqltok

// TokenKind classifies a token produced by the tokenizer.
type TokenKind uint8

const (
	Whitespace   TokenKind = iota // spaces, tabs, \r
	Newline                       // \n (separate for line tracking)
	LineComment                   // -- through end of line
	BlockComment                  // /* ... */
	Keyword                       // SQL keyword (classified via map lookup)
	Identifier                    // bare unquoted word
	QuotedIdent                   // "double-quoted"
	StringLit                     // 'single-quoted'
	DollarQuoted                  // $$...$$ or $tag$...$tag$
	NumberLit                     // 42, 3.14, 0xDEAD, 1e10
	Operator                      // ::, ||, =>, <>, !=, <=, >=, +, -, *, /, %, ^
	Dot                           // .
	Comma                         // ,
	Semicolon                     // ;
	LParen                        // (
	RParen                        // )
	LBracket                      // [
	RBracket                      // ]
	Colon                         // : (variant path)
	At                            // @  (stage references)
	Other                         // unrecognized byte
	EOF                           // end of input
)

var kindNames = [...]string{
	Whitespace:   "Whitespace",
	Newline:      "Newline",
	LineComment:  "LineComment",
	BlockComment: "BlockComment",
	Keyword:      "Keyword",
	Identifier:   "Identifier",
	QuotedIdent:  "QuotedIdent",
	StringLit:    "StringLit",
	DollarQuoted: "DollarQuoted",
	NumberLit:    "NumberLit",
	Operator:     "Operator",
	Dot:          "Dot",
	Comma:        "Comma",
	Semicolon:    "Semicolon",
	LParen:       "LParen",
	RParen:       "RParen",
	LBracket:     "LBracket",
	RBracket:     "RBracket",
	Colon:        "Colon",
	At:           "At",
	Other:        "Other",
	EOF:          "EOF",
}

// String returns the human-readable name of a TokenKind.
func (k TokenKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "Unknown"
}

// IsTrivia reports whether a token of this kind carries no syntactic meaning —
// whitespace, newlines, line comments, and block comments. EOF is not trivia.
// Use it (with SkipTrivia) to scan from one significant token to the next.
func (k TokenKind) IsTrivia() bool {
	switch k {
	case Whitespace, Newline, LineComment, BlockComment:
		return true
	default:
		return false
	}
}

// IsIdentLike reports whether a token of this kind can form part of a (possibly
// qualified) name: a bare identifier, a "quoted" identifier, or a keyword.
// Keywords are included because they are syntactically valid in identifier
// position (e.g. a column named VALUE, or FROM TABLE(...)); callers that must
// reject reserved words filter them separately.
func (k TokenKind) IsIdentLike() bool {
	switch k {
	case Identifier, QuotedIdent, Keyword:
		return true
	default:
		return false
	}
}

// Token represents a single lexical token in a SQL string.
type Token struct {
	Kind  TokenKind
	Start int    // byte offset in source
	End   int    // byte offset (exclusive); src[Start:End] = token text
	Line  int    // 1-based line number
	Col   int    // 1-based byte-column
	Tag   string // non-empty only for DollarQuoted
	// Unterminated is true when a StringLit, QuotedIdent, BlockComment, or
	// DollarQuoted token reached end-of-input without its closing delimiter
	// (the token still spans to EOF). It is always false for other kinds.
	Unterminated bool
}

// Text returns the source text of the token.
func (t Token) Text(src string) string { return src[t.Start:t.End] }
