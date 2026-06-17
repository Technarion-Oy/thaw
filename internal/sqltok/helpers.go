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

import (
	"sort"
	"strings"
)

// StripComments replaces line comments and block comments with spaces,
// preserving byte offsets and newlines so that downstream regex or
// position-dependent code still works correctly.
func StripComments(sql string) string {
	tokens := Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))
	prev := 0
	for _, tok := range tokens {
		if tok.Kind == EOF {
			break
		}
		if tok.Kind == LineComment || tok.Kind == BlockComment {
			sb.WriteString(sql[prev:tok.Start])
			// Preserve newlines within the comment span.
			span := sql[tok.Start:tok.End]
			for i := 0; i < len(span); i++ {
				if span[i] == '\n' {
					sb.WriteByte('\n')
				} else {
					sb.WriteByte(' ')
				}
			}
			prev = tok.End
		}
	}
	sb.WriteString(sql[prev:])
	return sb.String()
}

// StripStrings replaces each single-quoted string literal with a single space.
//
// Unlike StripComments, this does NOT preserve byte offsets: a multi-byte literal
// collapses to one space, so the result is shorter than the input. Do not chain
// StripStrings before any offset-dependent operation on the original text (it is
// safe to re-tokenize the result, which is the only way it is used here).
func StripStrings(sql string) string {
	tokens := Tokenize(sql)
	var sb strings.Builder
	sb.Grow(len(sql))
	prev := 0
	for _, tok := range tokens {
		if tok.Kind == EOF {
			break
		}
		if tok.Kind == StringLit {
			sb.WriteString(sql[prev:tok.Start])
			sb.WriteByte(' ')
			prev = tok.End
		}
	}
	sb.WriteString(sql[prev:])
	return sb.String()
}

// SkipTrivia returns the index of the first non-trivia token at or after i,
// skipping whitespace, newlines, and comments (see TokenKind.IsTrivia). When
// only trivia remains it returns the index of the terminating EOF token (or
// len(tokens) if the slice has no EOF), so callers should bounds- or EOF-check
// the result before use.
func SkipTrivia(tokens []Token, i int) int {
	for i < len(tokens) && tokens[i].Kind.IsTrivia() {
		i++
	}
	return i
}

// FirstToken returns the first keyword or identifier token in sql,
// uppercased. Returns "" if none found. Comments and whitespace are skipped.
func FirstToken(sql string) string {
	next := TokenizeIter(sql)
	for {
		tok, ok := next()
		if !ok {
			return ""
		}
		if tok.Kind.IsTrivia() {
			continue
		}
		if tok.Kind == Keyword || tok.Kind == Identifier {
			return strings.ToUpper(tok.Text(sql))
		}
		return ""
	}
}

// identPathEnd returns the index of the last token of a dot-joined identifier
// path starting at tokens[i], and whether one was found (tokens[i] must be
// ident-like). Parts extend only across a Dot that is immediately adjacent in
// the slice followed by another ident-like token — so on a raw token stream a
// space around the dot ends the path, while on a pre-filtered significant-token
// slice (trivia already removed) the parts join across the original whitespace.
// maxParts caps the number of parts; maxParts <= 0 means unbounded.
func identPathEnd(tokens []Token, i, maxParts int) (last int, ok bool) {
	if i < 0 || i >= len(tokens) || !tokens[i].Kind.IsIdentLike() {
		return i, false
	}
	last = i
	for parts := 1; maxParts <= 0 || parts < maxParts; parts++ {
		if last+2 < len(tokens) && tokens[last+1].Kind == Dot && tokens[last+2].Kind.IsIdentLike() {
			last += 2
			continue
		}
		break
	}
	return last, true
}

// ReadIdentPath reads a dot-joined identifier path (e.g. DB.SCHEMA."My Table")
// starting at tokens[i]. It returns the raw source substring spanning the path,
// the index just past the last consumed token, and whether a path was found.
// When none is found it returns ("", i, false). See identPathEnd for the
// adjacency and maxParts (<= 0 = unbounded) semantics.
func ReadIdentPath(tokens []Token, src string, i, maxParts int) (string, int, bool) {
	last, ok := identPathEnd(tokens, i, maxParts)
	if !ok {
		return "", i, false
	}
	return src[tokens[i].Start:tokens[last].End], last + 1, true
}

// ReadIdentParts is like ReadIdentPath but returns the individual part texts
// (un-normalised, quotes preserved) rather than the joined raw substring. It
// returns (nil, i) when no path is found.
func ReadIdentParts(tokens []Token, src string, i, maxParts int) ([]string, int) {
	last, ok := identPathEnd(tokens, i, maxParts)
	if !ok {
		return nil, i
	}
	parts := make([]string, 0, (last-i)/2+1)
	for k := i; k <= last; k += 2 {
		parts = append(parts, tokens[k].Text(src))
	}
	return parts, last + 1
}

// InertRegions returns [start,end) byte ranges for tokens that are "inert" —
// inside comments, string literals, or dollar-quoted blocks. Regex matches
// falling inside these regions should be ignored.
// The returned ranges are sorted and non-overlapping.
func InertRegions(sql string) [][2]int {
	tokens := Tokenize(sql)
	var regions [][2]int
	for _, tok := range tokens {
		if tok.Kind == EOF {
			break
		}
		switch tok.Kind {
		case LineComment, BlockComment, StringLit, DollarQuoted:
			regions = append(regions, [2]int{tok.Start, tok.End})
		}
	}
	return regions
}

// IsInert checks if a byte offset falls inside any inert region.
// regions must be sorted (as returned by InertRegions).
func IsInert(regions [][2]int, offset int) bool {
	// Binary search for the first region whose start is > offset.
	idx := sort.Search(len(regions), func(i int) bool {
		return regions[i][0] > offset
	})
	// Check the region just before: if offset < its end, we're inside it.
	if idx > 0 && offset < regions[idx-1][1] {
		return true
	}
	return false
}
