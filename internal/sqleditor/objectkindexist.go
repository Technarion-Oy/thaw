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

// ── Object-kind reference existence validation (issue #719, phase 2) ──────────
//
// The mechanical sweep over statements that spell out their own schema-scoped
// object kind — ALTER/DROP/DESCRIBE/COMMENT ON <kind> <name>. The kind phrase is
// matched against snowflake.SchemaScopedObjectTypes() and <name> is resolved
// against the KnownObjects catalog filtered to that kind, mirroring the table and
// stage existence semantics.

// regByKind records an object (by bare name and full path) under a kind key.
func regByKind(m map[string]map[string]struct{}, kind string, parts []string) {
	if len(parts) == 0 {
		return
	}
	set := m[kind]
	if set == nil {
		set = make(map[string]struct{})
		m[kind] = set
	}
	set[parts[len(parts)-1]] = struct{}{}
	set[strings.Join(parts, ".")] = struct{}{}
}

// unregByKind removes an object (by bare name and full path) from a kind key.
func unregByKind(m map[string]map[string]struct{}, kind string, parts []string) {
	if len(parts) == 0 || m[kind] == nil {
		return
	}
	delete(m[kind], parts[len(parts)-1])
	delete(m[kind], strings.Join(parts, "."))
}

// validateObjectKindRefs flags an ALTER/DROP/DESCRIBE/COMMENT ON <kind> <name>
// whose named object does not exist, mirroring table/stage resolution.
func validateObjectKindRefs(
	raw string, sig []sqltok.Token, baseLine int, ic bool,
	checkEq func(string, string) bool,
	knownObjects []ObjectRef,
	fetchedSchemas []SchemaEntry,
	sessionDB, sessionSchema string,
	createdByKind, droppedByKind map[string]map[string]struct{},
) []DiagMarker {
	objType, rawPath, hasIfExists, ok := matchObjectKindRef(sig, raw)
	if !ok || hasIfExists {
		return nil
	}
	// Plain tables/views are existence-checked through the richer ResolvedRef
	// path (with db/schema resolution, quick-fix, in-script CREATE tracking); the
	// table-likes (DYNAMIC/EVENT/ICEBERG/… TABLE, SEMANTIC/MATERIALIZED VIEW) are
	// distinct kinds and flow through here (closes the #708 gaps).
	if objType == "table" || objType == "view" {
		return nil
	}

	// Only validate kinds we have actually seen in the catalog. A kind we never
	// list (SHOW failed on this edition, or it has no SHOW command — TYPE,
	// SEQUENCE, …) yields no KnownObjects, so treat it as "no data" and stay
	// silent rather than false-positive on a name we can't see.
	// ponytail: needs ≥1 object of the kind somewhere fetched; a schema whose
	// only object of that kind is the typo'd one won't flag. Upgrade path: pass an
	// explicit fetched-kinds set from the frontend if this proves too lax.
	kindObjs := objectsOfKind(knownObjects, objType)
	if len(kindObjs) == 0 {
		return nil
	}

	parts := extractIdentParts(rawPath, ic)
	if len(parts) == 0 {
		return nil
	}
	var db, schema, name string
	switch len(parts) {
	case 1:
		db, schema, name = normIdent(sessionDB, ic), normIdent(sessionSchema, ic), parts[0]
	case 2:
		db, schema, name = normIdent(sessionDB, ic), parts[0], parts[1]
	default:
		db, schema, name = parts[0], parts[1], parts[len(parts)-1]
	}
	if db == "" || schema == "" {
		return nil
	}
	path := name
	if len(parts) >= 2 {
		path = strings.Join(parts, ".")
	}
	if created := createdByKind[objType]; isIn(created, name) || isIn(created, path) {
		return nil
	}
	if !schemaFetched(fetchedSchemas, db, schema) {
		return nil
	}
	dropped := droppedByKind[objType]
	wasDropped := isIn(dropped, name) || isIn(dropped, path)
	if !wasDropped && objectExists(kindObjs, db, schema, name, checkEq) {
		return nil
	}

	var markers []DiagMarker
	for _, t := range findTokensLocally(raw, []string{name}, baseLine, ic) {
		m := diagMarkerAt(t, capitalizeKind(objType)+" '"+t.name+"' does not exist or is not authorized.", 8)
		m.Code = buildQualifyObjectCode(name, objType, kindObjs, checkEq)
		markers = append(markers, m)
	}
	return markers
}

// objectExists reports whether an object named name lives in db.schema.
func objectExists(objs []ObjectRef, db, schema, name string, eq func(string, string) bool) bool {
	for _, o := range objs {
		if eq(o.Name, name) && eq(o.Schema, schema) && eq(o.DB, db) {
			return true
		}
	}
	return false
}

// capitalizeKind upper-cases the first letter of an ObjectType.Name() for the
// diagnostic label, e.g. "file format" → "File format".
func capitalizeKind(kind string) string {
	if kind == "" {
		return kind
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}
