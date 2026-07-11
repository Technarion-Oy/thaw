package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

// CollectCTENames scans a significant-token slice for WITH clauses and returns
// the raw source text of every CTE alias name, in source order. It is the single
// CTE-name scanner shared by lineage extraction (internal/snowflake) and the
// editor diagnostics (internal/sqleditor); callers apply their own name
// normalisation (issue #559).
//
// It walks each CTE list structurally rather than pattern-matching `name AS (`
// anywhere, so it handles:
//
//   - WITH RECURSIVE r AS (…) — and a CTE literally named RECURSIVE,
//   - column-list members: WITH c (a, b) AS (…),
//   - comma-separated lists, skipping each body's balanced parens so commas
//     inside a body are not mistaken for list separators,
//   - WITH clauses nested inside CTE bodies or subqueries (every WITH keyword
//     anchors its own walk),
//   - an unterminated final body (statement still being typed): the member's
//     name is already recorded once `name AS (` has been seen.
//
// Non-CTE WITH forms (SWAP WITH, STARTS WITH, WITH TAG (t='v'), WITH ROW ACCESS
// POLICY …) never produce a name: a member requires `AS (`, and an optional
// column list must contain only identifiers and commas — which rejects the
// `key = 'value'` shape of a tag list.
func CollectCTENames(src string, sig []sqltok.Token) []string {
	var names []string
	for _, d := range CollectCTEDefs(src, sig) {
		names = append(names, d.Name)
	}
	return names
}

// CTEDef describes one CTE definition located by CollectCTEDefs, as byte-offset
// spans into src. It carries the structural positions a consumer needs to slice
// out the name, the optional explicit column list, and the body — without
// re-scanning with its own regex (issue #673).
type CTEDef struct {
	Name      string // raw CTE alias name text, as it appears in src
	ColsStart int    // byte offset of the explicit column-list '(', or -1 if none
	ColsEnd   int    // byte offset just past the column-list ')'
	BodyStart int    // byte offset of the body's opening '('
	BodyEnd   int    // byte offset just past the body's ')'; len(src) if unterminated
	Closed    bool   // whether the body's parens were balanced
}

// CollectCTEDefs is the span-returning sibling of CollectCTENames: it walks the
// same CTE-list positions but returns each member's structural spans (see
// CTEDef) instead of just its name. Nested WITH clauses each anchor their own
// walk, so nested CTE members are returned too, in source order.
func CollectCTEDefs(src string, sig []sqltok.Token) []CTEDef {
	var defs []CTEDef
	for i := 0; i < len(sig); i++ {
		t := sig[i]
		if t.Kind != sqltok.Keyword || !strings.EqualFold(t.Text(src), "WITH") {
			continue
		}
		j := i + 1
		// Skip a RECURSIVE modifier — but only when it is not itself the CTE
		// name (`WITH recursive AS (…)` is a valid alias).
		if _, _, ok := cteListMember(src, sig, j); !ok &&
			j < len(sig) && sig[j].Kind.IsIdentLike() && strings.EqualFold(sig[j].Text(src), "RECURSIVE") {
			j++
		}
		for {
			def, next, ok := cteListMember(src, sig, j)
			if !ok {
				break
			}
			defs = append(defs, def)
			if !def.Closed {
				break // unterminated body — nothing structural follows
			}
			if next >= len(sig) || sig[next].Kind != sqltok.Comma {
				break
			}
			j = next + 1
		}
	}
	return defs
}

// cteListMember matches one CTE list member `name [ ( col [, col…] ) ] AS (…)`
// at sig[j]. It returns the member's spans (see CTEDef), the index just past the
// body's closing paren, and whether a member was recognized at all. An
// unbalanced body still yields the member (def.Closed=false): `name AS (` is
// enough to identify a CTE while the statement is being typed.
func cteListMember(src string, sig []sqltok.Token, j int) (def CTEDef, next int, ok bool) {
	if j >= len(sig) || !sig[j].Kind.IsIdentLike() {
		return CTEDef{}, 0, false
	}
	def = CTEDef{Name: sig[j].Text(src), ColsStart: -1}
	k := j + 1
	// Optional column list: contents must be only identifiers and commas, so a
	// `WITH TAG (key = 'value')` clause is not mistaken for a CTE member.
	if k < len(sig) && sig[k].Kind == sqltok.LParen {
		end := skipBalanced(sig, k)
		if end < 0 || !onlyIdentsAndCommas(sig, k+1, end-1) {
			return CTEDef{}, 0, false
		}
		def.ColsStart = sig[k].Start
		def.ColsEnd = sig[end-1].End
		k = end
	}
	if k >= len(sig) || !sig[k].Kind.IsIdentLike() || !strings.EqualFold(sig[k].Text(src), "AS") {
		return CTEDef{}, 0, false
	}
	k++
	if k >= len(sig) || sig[k].Kind != sqltok.LParen {
		return CTEDef{}, 0, false
	}
	def.BodyStart = sig[k].Start
	if end := skipBalanced(sig, k); end >= 0 {
		def.BodyEnd = sig[end-1].End
		def.Closed = true
		return def, end, true
	}
	def.BodyEnd = len(src)
	return def, len(sig), true
}

// skipBalanced returns the index just past the paren group opening at
// sig[open], or -1 when the group is unterminated.
func skipBalanced(sig []sqltok.Token, open int) int {
	depth := 0
	for m := open; m < len(sig); m++ {
		switch sig[m].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return m + 1
			}
		}
	}
	return -1
}

// onlyIdentsAndCommas reports whether every token in sig[from:to) is
// identifier-like or a comma.
func onlyIdentsAndCommas(sig []sqltok.Token, from, to int) bool {
	for m := from; m < to; m++ {
		if !sig[m].Kind.IsIdentLike() && sig[m].Kind != sqltok.Comma {
			return false
		}
	}
	return true
}
