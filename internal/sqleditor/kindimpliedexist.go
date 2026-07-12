// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import (
	"strings"

	"thaw/internal/sqltok"
)

// ── Kind-implied reference existence validation (issue #719, phase 3) ─────────
//
// Positions where the object kind is implied by the clause rather than spelled
// out, so the phase-2 `<verb> <kind> <name>` matcher can't see them:
//
//	CALL <proc>(…)                          → procedure
//	EXECUTE TASK <task>                      → task
//	SET|ADD MASKING POLICY <name>            → masking policy
//	SET|ADD ROW ACCESS POLICY <name>         → row access policy
//	FILE_FORMAT = ( FORMAT_NAME = '<ff>' )   → file format
//
// Each is a small keyword-anchored extractor on the shared resolution backbone
// (resolveObjectMissing / flagMissingObject), guarded exactly like phases 1–2.

// validateKindImpliedRefs scans one statement for kind-implied object references
// and flags the missing ones.
func validateKindImpliedRefs(
	raw string, sig []sqltok.Token, baseLine, baseCol int, ic bool,
	checkEq func(string, string) bool,
	knownObjects []ObjectRef, fetchedSchemas []SchemaEntry,
	sessionDB, sessionSchema string,
	createdByKind, droppedByKind map[string]map[string]struct{},
) []DiagMarker {
	var markers []DiagMarker

	// flagBare resolves a bare-identifier reference and appends any marker.
	flagBare := func(objType, rawPath string) {
		if rawPath == "" {
			return
		}
		markers = append(markers, flagMissingObject(raw, baseLine, baseCol, ic, checkEq, objType,
			extractIdentParts(rawPath, ic), knownObjects, fetchedSchemas,
			sessionDB, sessionSchema, createdByKind, droppedByKind)...)
	}

	for i := 0; i < len(sig); i++ {
		u := tokUpper(sig[i], raw)
		switch u {
		case "CALL":
			// CALL as a statement head: CALL [db.][schema.]proc(...).
			if i != 0 {
				continue
			}
			p, end := readIdentPath(sig, raw, i+1)
			if p == "" {
				continue
			}
			// Skip built-in system functions: SYSTEM$… (the '$' may tokenize as a
			// trailing Other token or fold into one identifier) and the SNOWFLAKE.*
			// namespace — neither is a user procedure in the catalog.
			if strings.Contains(p, "$") || (end < len(sig) && sig[end].Kind == sqltok.Other && sig[end].Text(raw) == "$") {
				continue
			}
			if pp := extractIdentParts(p, ic); len(pp) > 0 && strings.EqualFold(pp[0], "SNOWFLAKE") {
				continue
			}
			flagBare("procedure", p)

		case "EXECUTE":
			// EXECUTE TASK <task> (not EXECUTE IMMEDIATE …).
			if kwAt(sig, raw, i+1, "TASK") {
				if p, _ := readIdentPath(sig, raw, i+2); p != "" {
					flagBare("task", p)
				}
			}

		case "MASKING":
			// SET|ADD MASKING POLICY <name> — an apply, not a CREATE/ALTER/DROP of
			// the policy itself (those are the statement head → phase 2).
			if precededBySetOrAdd(sig, raw, i) && kwAt(sig, raw, i+1, "POLICY") {
				if p, _ := readIdentPath(sig, raw, i+2); p != "" {
					flagBare("masking policy", p)
				}
			}

		case "ROW":
			// SET|ADD ROW ACCESS POLICY <name>.
			if precededBySetOrAdd(sig, raw, i) && kwAt(sig, raw, i+1, "ACCESS") && kwAt(sig, raw, i+2, "POLICY") {
				if p, _ := readIdentPath(sig, raw, i+3); p != "" {
					flagBare("row access policy", p)
				}
			}

		case "FORMAT_NAME":
			// FORMAT_NAME = '<ff>' | FORMAT_NAME => '<ff>'. The value is a string
			// literal, so it places its own marker (findTokensLocally only sees
			// identifiers). A bare-identifier form is handled by flagBare.
			j := i + 1
			for j < len(sig) && sig[j].Kind == sqltok.Operator { // skip '=' / '=>'
				j++
			}
			if j >= len(sig) {
				continue
			}
			if sig[j].Kind == sqltok.StringLit {
				inner := unquoteSingle(sig[j].Text(raw))
				if inner == "" {
					continue
				}
				missing, normName, kindObjs := resolveObjectMissing("file format",
					extractIdentParts(inner, ic), ic, checkEq, knownObjects, fetchedSchemas,
					sessionDB, sessionSchema, createdByKind, droppedByKind)
				if missing {
					// Display the last segment as written (original case); the value
					// lives inside a string literal, so mark that token span directly.
					disp := inner
					if k := strings.LastIndexByte(inner, '.'); k >= 0 {
						disp = inner[k+1:]
					}
					t := tokenPosOf(sig[j], baseLine, baseCol, disp)
					m := diagMarkerAt(t, "File format '"+disp+"' does not exist or is not authorized.", 8)
					m.Code = buildQualifyObjectCode(normName, "file format", kindObjs, checkEq)
					markers = append(markers, m)
				}
			} else if isIdent(sig[j]) {
				if p, _ := readIdentPath(sig, raw, j); p != "" {
					flagBare("file format", p)
				}
			}
		}
	}
	return markers
}

// unquoteSingle strips the surrounding single quotes from a SQL string literal
// and collapses doubled '' escapes, returning the raw content ('a''b' → a'b).
func unquoteSingle(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}
	return strings.ReplaceAll(s, "''", "'")
}

// precededBySetOrAdd reports whether the significant token before sig[i] is SET
// or ADD — the apply verbs that introduce a policy reference.
func precededBySetOrAdd(sig []sqltok.Token, raw string, i int) bool {
	if i == 0 {
		return false
	}
	u := tokUpper(sig[i-1], raw)
	return u == "SET" || u == "ADD"
}

// tokenPosOf builds a tokenPos spanning a single token (used to mark a value that
// lives inside a string literal, which findTokensLocally cannot locate).
func tokenPosOf(tok sqltok.Token, baseLine, baseCol int, name string) tokenPos {
	col := tok.Col
	if tok.Line == 1 {
		col += baseCol - 1 // rebase first-line columns to document coords
	}
	return tokenPos{
		name:   name,
		line:   baseLine + tok.Line - 1,
		col:    col,
		endCol: col + (tok.End - tok.Start),
	}
}
