// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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

// Token represents a single lexical token in a SQL string.
type Token struct {
	Kind  TokenKind
	Start int    // byte offset in source
	End   int    // byte offset (exclusive); src[Start:End] = token text
	Line  int    // 1-based line number
	Col   int    // 1-based byte-column
	Tag   string // non-empty only for DollarQuoted
}

// Text returns the source text of the token.
func (t Token) Text(src string) string { return src[t.Start:t.End] }
