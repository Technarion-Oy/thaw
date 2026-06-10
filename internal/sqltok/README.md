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
```

### Helpers

```go
func StripComments(sql string) string         // replace comments with spaces
func StripStrings(sql string) string          // replace string literals with space
func FirstToken(sql string) string            // first keyword/identifier, uppercased
func InertRegions(sql string) [][2]int        // comment/string/dollar-quote byte ranges
func IsInert(regions [][2]int, offset int) bool // binary search offset check
```

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
| `keywords.go` | Keyword, reserved, and builtin-function maps |
| `tokenizer.go` | Core `scan()` state machine, `Tokenize`, `TokenizeIter` |
| `split.go` | `Split`, `SplitRanges`, `StatementRange` |
| `helpers.go` | `StripComments`, `StripStrings`, `FirstToken`, `InertRegions`, `IsInert` |
