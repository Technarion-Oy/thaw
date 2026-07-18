// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"strings"

	"thaw/internal/sqlgrammar"
	"thaw/internal/sqltok"
)

// Bare (non-$$) Snowflake Scripting anonymous blocks — `[DECLARE …] BEGIN …
// [EXCEPTION …] END` written WITHOUT $$ delimiters — are valid Snowsight syntax
// users paste from worksheets. sqltok.SplitRanges splits purely on top-level `;`,
// so it shreds such a block into fragments; each fragment then fails grammar /
// syntax validation in isolation (issue #793 A1 — an `EXCEPTION` fragment even
// trips ValidateSyntax's statement-start gate as a hard Error). The helpers here
// re-glue a block into one span so the grammar's ParseScriptingBlock — which
// already models the whole construct — validates it intact.

// byteSpan is a [start,end) byte range within a SQL string.
type byteSpan struct{ start, end int }

// startsBareScriptingBlock reports whether stmt (one SplitRanges fragment) opens a
// bare Snowflake Scripting anonymous block: a leading DECLARE, or a BEGIN that is
// NOT a transaction-control statement (BEGIN [WORK|TRANSACTION|NAME …]). The
// transaction whitelist mirrors the anti-pattern transaction tracker
// (antipatterns.go) so `BEGIN;` / `BEGIN WORK;` stay ordinary statements.
func startsBareScriptingBlock(stmt string) bool {
	stripped := stripCommentsSQL(stmt)
	sig := sigTokens(stripped)
	if len(sig) == 0 {
		return false
	}
	switch tokUpper(sig[0], stripped) {
	case "DECLARE":
		return true
	case "BEGIN":
		if len(sig) < 2 {
			return false // bare `BEGIN` — a transaction opener
		}
		u := tokUpper(sig[1], stripped)
		// A non-word second token (e.g. the trailing `;`) is a transaction; only a
		// real statement verb after BEGIN marks a scripting block.
		return u != "" && u != "WORK" && u != "TRANSACTION" && u != "NAME"
	}
	return false
}

// scriptingBlockComplete reports whether text parses as a complete Snowflake
// Scripting block ([DECLARE …] BEGIN … END) under the grammar engine. Using the
// grammar (rather than hand-counting BEGIN/END nesting) is what makes block-end
// detection robust against `CURSOR FOR`, expression `CASE … END`, `END IF`,
// `END FOR`, and the like.
func scriptingBlockComplete(text string) bool {
	v := sqlgrammar.New(text)
	return v.Recognized() && v.ParseTopLevel()
}

// bareScriptingBlockSpans returns the byte spans of top-level bare scripting
// blocks in sql. Consecutive SplitRanges fragments that together form one
// complete block (per the grammar) are coalesced into a single span; an
// unterminated block (one that never completes) coalesces every remaining
// fragment into one span so it reads as a single incomplete statement rather than
// a shower of per-fragment errors.
func bareScriptingBlockSpans(sql string, ranges []sqltok.StatementRange) []byteSpan {
	var spans []byteSpan
	for i := 0; i < len(ranges); i++ {
		start := ranges[i].StartOffset
		if !startsBareScriptingBlock(sql[start:ranges[i].EndOffset]) {
			continue
		}
		j := i
		for j < len(ranges) && !scriptingBlockComplete(sql[start:ranges[j].EndOffset]) {
			j++
		}
		if j >= len(ranges) {
			j = len(ranges) - 1 // unterminated block — absorb to the end
		}
		spans = append(spans, byteSpan{start, ranges[j].EndOffset})
		i = j
	}
	return spans
}

// hasBlockLeaderToken reports whether toks contains a BEGIN or DECLARE keyword —
// a cheap, allocation-free gate before the fuller bareScriptingBlockSpans scan,
// so SQL with neither keyword avoids the extra SplitRanges pass.
func hasBlockLeaderToken(toks []sqltok.Token, src string) bool {
	for _, t := range toks {
		if t.Kind != sqltok.Keyword {
			continue
		}
		if strings.EqualFold(t.Text(src), "BEGIN") || strings.EqualFold(t.Text(src), "DECLARE") {
			return true
		}
	}
	return false
}

// spanStartingAt returns the span whose start byte equals offset, if any.
func spanStartingAt(spans []byteSpan, offset int) (byteSpan, bool) {
	for _, s := range spans {
		if s.start == offset {
			return s, true
		}
	}
	return byteSpan{}, false
}
