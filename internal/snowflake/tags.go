// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"fmt"
	"strings"
)

// TagPair is a single tag name/value pair used in object-level TAG clauses
// (CREATE … TAG (…) / WITH TAG (…)). It is the shared shape for the per-object
// CREATE config structs (dynamic tables, external tables, materialized views,
// alerts, git repositories, …) so the tag handling lives in one place.
type TagPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TagClause renders a `TAG (name = 'value', ...)` clause from the non-empty tag
// pairs, or "" when there are none. Tag names are quoted identifiers; values are
// single-quoted string literals. Pairs whose name is blank (after trimming) are
// skipped. Callers whose grammar uses the `WITH TAG (...)` form (e.g. CREATE
// ALERT / CREATE GIT REPOSITORY) prepend "WITH " to a non-empty result.
func TagClause(tags []TagPair) string {
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s = '%s'", QuoteIdent(name), EscapeStringLit(t.Value)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "TAG (" + strings.Join(parts, ", ") + ")"
}
