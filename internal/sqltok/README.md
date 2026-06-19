# internal/sqltok

Single-pass O(N) tokenizer for Snowflake SQL. Replaces regex-based parsing with a byte-level state machine that correctly handles all Snowflake quoting and comment styles.

## Domain

SQL Editor & Diagnostics

## What it does

- **Tokenizes** SQL into classified tokens (keywords, identifiers, literals, operators, punctuation, comments)
- **Tracks** 1-based line/column positions for each token
- **Splits** multi-statement SQL at top-level semicolons
- **Strips** comments or string literals while preserving byte offsets
- **Identifies** inert regions (comments, strings, dollar-quoted blocks) for safe regex filtering

## API

### Core tokenizer

```go
// Full tokenization — returns all tokens with EOF sentinel.
func Tokenize(sql string) []Token

// Iterator — yields one token per call, zero pre-allocation.
func TokenizeIter(sql string) func() (Token, bool)
```

### Token types

`TokenKind` enum: `Whitespace`, `Newline`, `LineComment`, `BlockComment`, `Keyword`, `Identifier`, `QuotedIdent`, `StringLit`, `DollarQuoted`, `NumberLit`, `Operator`, `Dot`, `Comma`, `Semicolon`, `LParen`, `RParen`, `LBracket`, `RBracket`, `Colon`, `At`, `Other`, `EOF`.

`Token.Unterminated` is `true` when a `StringLit`, `QuotedIdent`, `BlockComment`, or `DollarQuoted` token reached end-of-input without its closing delimiter (the token still spans to EOF). Always `false` for other kinds.

### Statement splitting

```go
func Split(sql string) []string              // trimmed statements
func SplitRanges(sql string) []StatementRange // per-statement byte ranges + lines
```

### Keyword classification

```go
func IsReserved(upper string) bool       // Snowflake reserved keywords
func IsKeyword(upper string) bool        // all SQL keywords
func IsBuiltinFunction(upper string) bool // built-in function names

// RegisterDataTypeKeywords injects the authoritative data-type names so the
// tokenizer treats them as keywords. The data-type list is NOT hardcoded here —
// the snowflake package (the single source of truth, internal/snowflake/
// datatypes.go) calls this from its init. sqltok is a leaf package, so it
// cannot import snowflake; this dependency-injection keeps the list in one place.
func RegisterDataTypeKeywords(names []string)
```

### Helpers

```go
func StripComments(sql string) string         // replace comments with spaces
func StripStrings(sql string) string          // replace string literals with space
func FirstToken(sql string) string            // first keyword/identifier, uppercased
func SkipTrivia(tokens []Token, i int) int    // index of next non-trivia token at/after i
func Significant(tokens []Token) []Token      // drop trivia + EOF → meaningful tokens
func SignificantTokens(sql string) []Token    // Significant(Tokenize(sql))
func ReadIdentPath(tokens []Token, src string, i, maxParts int) (string, int, bool)  // dot-joined name → raw substring
func ReadIdentParts(tokens []Token, src string, i, maxParts int) ([]string, int)     // dot-joined name → part texts
func StripQuotePair(s string) string          // "NAME" → NAME (no unescape)
func Unquote(s string) string                 // "my""id" → my"id (strip pair + unescape)
func InertRegions(sql string) [][2]int        // comment/string/dollar-quote byte ranges
func IsInert(regions [][2]int, offset int) bool // binary search offset check
```

### Token-kind predicates

```go
func (k TokenKind) IsTrivia() bool     // whitespace, newline, line/block comment (not EOF)
func (k TokenKind) IsIdentLike() bool  // identifier, quoted identifier, or keyword
```

`IsTrivia` + `SkipTrivia` are the shared way to scan from one significant token
to the next (used by the lineage parser and the SQL-editor validators).
`IsIdentLike` reports whether a token can sit in a (possibly qualified) name —
keywords included, since callers filter reserved words themselves.

`ReadIdentPath`/`ReadIdentParts` read a dot-joined identifier (`DB.SCHEMA."Tbl"`).
Parts join only across a `Dot` that is **immediately adjacent in the slice**: on
a raw token stream a space around the dot ends the path; on a significant-token
slice (trivia already removed) the parts join across the original whitespace.
`maxParts` caps the number of parts (`<= 0` = unbounded).

## Design decisions

- **Byte-level scanning**: No `[]rune` allocation. Uses `strings.Index`/`strings.IndexByte` for SIMD-accelerated jumps over comment and quoted bodies.
- **Newline as separate token**: Enables line tracking without scanning inside whitespace runs.
- **Pre-allocated slice**: `Tokenize` pre-allocates at `len(sql)/4` capacity.
- **Dollar sign in words**: `SYSTEM$TYPEOF` and similar Snowflake identifiers scan as a single token.

## Files

| File | Contents |
|------|----------|
| `doc.go` | Package doc, domain annotation |
| `token.go` | `TokenKind` enum, `Token` struct |
| `keywords.go` | Keyword, reserved, and builtin-function maps. Data-type names are injected at init via `RegisterDataTypeKeywords` from `internal/snowflake` rather than duplicated here. |
| `tokenizer.go` | Core `scan()` state machine, `Tokenize`, `TokenizeIter` |
| `split.go` | `Split`, `SplitRanges`, `StatementRange` |
| `helpers.go` | `StripComments`, `StripStrings`, `FirstToken`, `InertRegions`, `IsInert` |
