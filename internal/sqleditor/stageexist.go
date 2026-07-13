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
	"encoding/json"
	"strings"

	"thaw/internal/sqltok"
)

// ── Stage-reference existence validation (issue #719, phase 1) ────────────────
//
// A `@` in significant SQL is unambiguously a stage reference (Snowflake has no
// `@` operator), and strings/comments/$$ are separate token kinds, so scanning
// for At tokens is both complete and free of false hits. This covers every stage
// context — FROM @stg, COPY INTO t FROM @stg, PUT … @stg, GET @stg, LIST/REMOVE,
// SELECT $1 FROM @stg/f.csv — without keyword anchoring.

// objectsOfKind filters KnownObjects to a single kind (case-insensitive).
func objectsOfKind(objs []ObjectRef, kind string) []ObjectRef {
	var out []ObjectRef
	for _, o := range objs {
		if strings.EqualFold(o.Kind, kind) {
			out = append(out, o)
		}
	}
	return out
}

// validateStageRefs flags @stage references whose stage does not exist, mirroring
// the table-existence semantics: unqualified names resolve against the session
// database/schema, qualified names against their own path, and validation only
// fires for schemas whose objects were actually fetched (fetchedSchemas).
func validateStageRefs(
	raw string, sig []sqltok.Token, baseLine, baseCol int, ic bool,
	checkEq func(string, string) bool,
	knownStages []ObjectRef,
	fetchedSchemas []SchemaEntry,
	sessionDB, sessionSchema string,
	created, dropped map[string]struct{},
) []DiagMarker {
	var markers []DiagMarker

	for _, ref := range scanStageRefs(sig, raw, ic) {
		var db, schema, name string
		switch len(ref.parts) {
		case 1:
			db, schema, name = normIdent(sessionDB, ic), normIdent(sessionSchema, ic), ref.parts[0]
		case 2:
			db, schema, name = normIdent(sessionDB, ic), ref.parts[0], ref.parts[1]
		default: // 3+
			db, schema, name = ref.parts[0], ref.parts[1], ref.parts[len(ref.parts)-1]
		}
		if db == "" || schema == "" {
			continue // no context to resolve an unqualified name against
		}
		path := name
		if len(ref.parts) >= 2 {
			path = strings.Join(ref.parts, ".")
		}
		if isIn(created, name) || isIn(created, path) {
			continue
		}
		// Only validate when we actually have this schema's object list.
		if !schemaFetched(fetchedSchemas, db, schema) {
			continue
		}
		wasDropped := isIn(dropped, name) || isIn(dropped, path)
		if !wasDropped && stageExists(knownStages, db, schema, name, checkEq) {
			continue
		}
		for _, t := range findTokensLocally(raw, []string{name}, baseLine, baseCol, ic) {
			m := diagMarkerAt(t, "Stage '"+t.name+"' does not exist or is not authorized.", 8)
			m.Code = buildQualifyObjectCode(name, "stage", knownStages, checkEq)
			markers = append(markers, m)
		}
	}
	return markers
}

// stageRefParts holds the normalised identifier parts of one @stage reference.
type stageRefParts struct {
	parts []string
}

// scanStageRefs finds every @stage reference in sig and returns its normalised
// name parts (@[db.][schema.]stage), stopping at the first '/'. Bare @~ (current
// user stage) and @%tbl (table stage) yield no ident parts and are skipped.
// ponytail: @%tbl table-stage validation deferred — conservative no-flag.
func scanStageRefs(sig []sqltok.Token, raw string, ic bool) []stageRefParts {
	var out []stageRefParts
	for i := 0; i < len(sig); i++ {
		if sig[i].Kind != sqltok.At {
			continue
		}
		var parts []string
		expectIdent := true
		lastEnd := sig[i].End
		for k := i + 1; k < len(sig); k++ {
			t := sig[k]
			if t.Start != lastEnd { // adjacency: no whitespace inside a stage path
				break
			}
			if expectIdent {
				if !t.Kind.IsIdentLike() {
					break
				}
				parts = append(parts, normIdent(t.Text(raw), ic))
				expectIdent = false
			} else {
				if t.Kind != sqltok.Dot {
					break // '/', operator, … → end of stage name
				}
				expectIdent = true
			}
			lastEnd = t.End
		}
		if len(parts) > 0 {
			out = append(out, stageRefParts{parts})
		}
	}
	return out
}

// schemaFetched reports whether (db, schema) is among the schemas whose object
// lists were fetched — the existence guard. Compared case-insensitively since
// fetched-ness is not identity-sensitive.
func schemaFetched(fetched []SchemaEntry, db, schema string) bool {
	for _, f := range fetched {
		if strings.EqualFold(f.DB, db) && strings.EqualFold(f.Name, schema) {
			return true
		}
	}
	return false
}

// stageExists reports whether a STAGE named name lives in db.schema.
func stageExists(stages []ObjectRef, db, schema, name string, eq func(string, string) bool) bool {
	for _, s := range stages {
		if eq(s.Name, name) && eq(s.Schema, schema) && eq(s.DB, db) {
			return true
		}
	}
	return false
}

// buildQualifyObjectCode returns quick-fix qualification suggestions (kind
// "qualify-<objKind>") when name exists as objKind in other schemas. Returns ""
// when there are no alternatives. The frontend CodeActionProvider consumes any
// "qualify-*" payload.
func buildQualifyObjectCode(name, objKind string, objs []ObjectRef, eq func(string, string) bool) string {
	var suggestions []string
	seen := make(map[string]struct{})
	for _, o := range objs {
		if !eq(o.Name, name) || o.DB == "" || o.Schema == "" {
			continue
		}
		qualified := o.DB + "." + o.Schema + "." + o.Name
		if _, ok := seen[strings.ToUpper(qualified)]; ok {
			continue
		}
		seen[strings.ToUpper(qualified)] = struct{}{}
		suggestions = append(suggestions, qualified)
	}
	if len(suggestions) == 0 {
		return ""
	}
	data, err := json.Marshal(qualifyTableCodePayload{
		Kind:        "qualify-" + objKind,
		Original:    name,
		Suggestions: suggestions,
	})
	if err != nil {
		return ""
	}
	return string(data)
}
