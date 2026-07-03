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
	for i := 0; i < len(sig); i++ {
		t := sig[i]
		if t.Kind != sqltok.Keyword || !strings.EqualFold(t.Text(src), "WITH") {
			continue
		}
		j := i + 1
		// Skip a RECURSIVE modifier — but only when it is not itself the CTE
		// name (`WITH recursive AS (…)` is a valid alias).
		if _, _, _, ok := cteListMember(src, sig, j); !ok &&
			j < len(sig) && sig[j].Kind.IsIdentLike() && strings.EqualFold(sig[j].Text(src), "RECURSIVE") {
			j++
		}
		for {
			name, next, closed, ok := cteListMember(src, sig, j)
			if !ok {
				break
			}
			names = append(names, name)
			if !closed {
				break // unterminated body — nothing structural follows
			}
			if next >= len(sig) || sig[next].Kind != sqltok.Comma {
				break
			}
			j = next + 1
		}
	}
	return names
}

// cteListMember matches one CTE list member `name [ ( col [, col…] ) ] AS (…)`
// at sig[j]. It returns the raw name, the index just past the body's closing
// paren, whether the body's parens were balanced, and whether a member was
// recognized at all. An unbalanced body still yields the name (closed=false):
// `name AS (` is enough to identify a CTE while the statement is being typed.
func cteListMember(src string, sig []sqltok.Token, j int) (name string, next int, closed, ok bool) {
	if j >= len(sig) || !sig[j].Kind.IsIdentLike() {
		return
	}
	name = sig[j].Text(src)
	k := j + 1
	// Optional column list: contents must be only identifiers and commas, so a
	// `WITH TAG (key = 'value')` clause is not mistaken for a CTE member.
	if k < len(sig) && sig[k].Kind == sqltok.LParen {
		end := skipBalanced(sig, k)
		if end < 0 || !onlyIdentsAndCommas(sig, k+1, end-1) {
			return "", 0, false, false
		}
		k = end
	}
	if k >= len(sig) || !sig[k].Kind.IsIdentLike() || !strings.EqualFold(sig[k].Text(src), "AS") {
		return "", 0, false, false
	}
	k++
	if k >= len(sig) || sig[k].Kind != sqltok.LParen {
		return "", 0, false, false
	}
	if end := skipBalanced(sig, k); end >= 0 {
		return name, end, true, true
	}
	return name, len(sig), false, true
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
