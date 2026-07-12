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

// ── Types ─────────────────────────────────────────────────────────────────────

// SchemaEntry pairs a database name with a schema name.
type SchemaEntry struct {
	DB   string `json:"db"`
	Name string `json:"name"`
}

// ObjectRef is a schema-scoped object beyond tables/views (stages, streams,
// tasks, pipes, file formats, …). Kind strings match ListObjects (e.g. "STAGE").
type ObjectRef struct {
	DB     string `json:"db"`
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

// ValidateTablesExistRequest is the input to ValidateTablesExist.
type ValidateTablesExistRequest struct {
	SQL            string           `json:"sql"`
	StmtRanges     []StatementRange `json:"stmtRanges"`
	ResolvedRefs   []ResolvedRef    `json:"resolvedRefs"`
	KnownDatabases []string         `json:"knownDatabases"`
	KnownSchemas   []SchemaEntry    `json:"knownSchemas"`
	// SessionDatabase/SessionSchema are the active session context (the
	// USE'd database/schema), NOT the catalog. They — plus in-script
	// USE/CREATE effects — decide whether an unqualified name has a
	// database/schema to resolve against ("No database selected" warnings).
	// KnownDatabases/KnownSchemas are the full catalog, used only for
	// existence checks on qualified names.
	SessionDatabase             string        `json:"sessionDatabase"`
	SessionSchema               string        `json:"sessionSchema"`
	QuotedIdentifiersIgnoreCase bool          `json:"quotedIdentifiersIgnoreCase"`
	DroppedDatabases            []string      `json:"droppedDatabases"`
	DroppedSchemas              []SchemaEntry `json:"droppedSchemas"`
	// DroppedTables uses ResolvedRef (Alias field is ignored).
	DroppedTables []ResolvedRef `json:"droppedTables"`
	// AllKnownTables is the full set of resolved table references available in
	// the session. When a "table not found" marker is emitted, this list is
	// searched for tables with the same name in other schemas, enabling
	// quick-fix qualification suggestions via the Code field.
	AllKnownTables []ResolvedRef `json:"allKnownTables"`
	// KnownObjects lists schema-scoped objects beyond tables/views (stages,
	// streams, tasks, …), used to existence-check non-table references such as
	// stage refs (@stg). FetchedObjectSchemas is the guard: only schemas whose
	// object lists were actually fetched are validated, so shared DBs where SHOW
	// can never succeed (e.g. SNOWFLAKE) stay silent instead of false-positiving.
	KnownObjects         []ObjectRef   `json:"knownObjects"`
	FetchedObjectSchemas []SchemaEntry `json:"fetchedObjectSchemas"`
}

// ── ValidateTablesExist ───────────────────────────────────────────────────────

// ValidateTablesExist checks each statement for references to databases,
// schemas, or tables/views that are not in the resolved references or known
// catalogs.  It is a Go port of the validateTablesExist function from
// sqlDiagnostics.ts, with one deliberate divergence (issue #689): the
// "No database/schema selected" warnings key off SessionDatabase/SessionSchema
// (the actual USE'd session context) plus in-script USE/CREATE effects, not
// off catalog size — a populated catalog does not mean a database is selected.
//
// Severity mapping: 8 = Monaco Error (red squiggles).
func ValidateTablesExist(req ValidateTablesExistRequest) []DiagMarker {
	ic := req.QuotedIdentifiersIgnoreCase
	checkEq := func(a, b string) bool {
		if ic {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	var markers []DiagMarker

	// ── Single sequential pass ─────────────────────────────────────────────
	// createdTables/createdDbsAndSchemas track what currently exists in the
	// script (updated at the START of each iteration for CREATE, at the END
	// for DROP/UNDROP so DROP validations see the pre-drop state).
	// droppedTables/droppedDbsAndSchemas are append-only ("ever dropped")
	// and are used only for UNDROP validation.
	scriptCreatedTables := make(map[string]struct{})
	scriptCreatedDbsAndSchemas := make(map[string]struct{})
	scriptDroppedTables := make(map[string]struct{})
	scriptDroppedDbsAndSchemas := make(map[string]struct{})
	// scriptEverCreatedSchemasByDB tracks every DB for which a 2-part
	// CREATE SCHEMA db.sch appeared in the script (append-only; not cleared by
	// DROP).  Used to decide whether to validate schema references in a
	// 3-part CREATE TABLE — if we've seen "CREATE SCHEMA DB.SCH" earlier in
	// the same script, we have enough context to validate that the schema still
	// exists (and catch "schema was dropped" errors).
	scriptEverCreatedSchemasByDB := make(map[string]struct{})

	scriptHasActiveDB := false
	scriptHasActiveSchema := false

	// Schema-scoped objects beyond tables/views (stages, streams, tasks, pipes,
	// file formats, table-likes, policies, …) — resolved against the KnownObjects
	// catalog rather than ResolvedRefs. In-script CREATE/DROP effects are tracked
	// per kind (keyed by ObjectType.Name(), e.g. "stage", "file format").
	scriptCreatedByKind := make(map[string]map[string]struct{})
	scriptDroppedByKind := make(map[string]map[string]struct{})

	for _, r := range req.StmtRanges {
		raw := sqlStmt(req.SQL, r)
		tokens := sqltok.Tokenize(raw)
		sig := sigToks(tokens)

		// ── (a) Apply CREATE effects before validation ─────────────
		if rawPath, _, ok := matchCreateTV(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				scriptCreatedTables[parts[len(parts)-1]] = struct{}{}
				scriptCreatedTables[strings.Join(parts, ".")] = struct{}{}
			}
		}
		if rawPath, objType, ok := matchCreateSchemaScoped(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				regByKind(scriptCreatedByKind, objType, parts)
				unregByKind(scriptDroppedByKind, objType, parts)
			}
		}
		if rawPath, ok := matchCreateDbSch(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				scriptCreatedDbsAndSchemas[parts[len(parts)-1]] = struct{}{}
				scriptCreatedDbsAndSchemas[strings.Join(parts, ".")] = struct{}{}
				// Track 2-part CREATE SCHEMA <db>.<sch> so we can validate
				// schema references in subsequent 3-part CREATE TABLE statements.
				if len(parts) == 2 {
					scriptEverCreatedSchemasByDB[parts[0]] = struct{}{}
				}
			}
		}

		// Track USE/CREATE statements that establish an active DB/schema
		firstKw := ""
		if len(sig) > 0 {
			firstKw = tokUpper(sig[0], raw)
		}

		if firstKw == "USE" {
			if u, ok := matchUse(sig, raw); ok {
				switch {
				case u.kind == "DATABASE":
					scriptHasActiveDB = true
				case u.kind == "SCHEMA" && u.parts == 2:
					scriptHasActiveDB = true
					scriptHasActiveSchema = true
				case u.kind == "SCHEMA":
					scriptHasActiveSchema = true
				case u.kind == "" && u.parts == 2:
					scriptHasActiveDB = true
					scriptHasActiveSchema = true
				case u.kind == "" && u.parts == 1:
					scriptHasActiveDB = true
				}
			}
		}
		if firstKw == "CREATE" {
			if matchCreateAnyDB(sig, raw) {
				scriptHasActiveDB = true
			}
			if matchCreateAnySchema(sig, raw) {
				scriptHasActiveSchema = true
			}
		}

		// "Is a database/schema selected for unqualified name resolution?" —
		// session context only, NOT catalog size. The catalog always has
		// databases once connected, which is unrelated to whether one is USE'd.
		hasSessionDB := req.SessionDatabase != ""
		hasSessionSchema := req.SessionSchema != ""

		// ── CREATE <schema-scoped object> ─────────────────────────────
		// TABLE, VIEW, SEQUENCE, STAGE, STREAM, TASK, FILE FORMAT, … all live in a
		// schema, so an unqualified name needs an active database + schema.
		if rawPath, objType, ok := matchCreateSchemaScoped(sig, raw); ok {
			parts := extractIdentParts(rawPath, ic)
			rawParts, _ := readIdentParts(sig, raw, findPathStartInSig(sig, raw, rawPath))

			switch len(parts) {
			case 1:
				if !hasSessionDB && !scriptHasActiveDB {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No database selected. Cannot create "+objType+" '"+t.name+"'.", 8))
					}
				} else if !hasSessionSchema && !scriptHasActiveSchema {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No schema selected. Cannot create "+objType+" '"+t.name+"'.", 8))
					}
				}
			case 2:
				schemaNorm := parts[0]
				if !hasSessionDB && !scriptHasActiveDB {
					searchToken := parts[0]
					if len(rawParts) > 0 {
						searchToken = normIdent(rawParts[0], ic)
					}
					for _, t := range findTokensLocally(raw, []string{searchToken}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No database selected. Cannot create "+objType+" using schema '"+t.name+"'.", 8))
					}
				} else {
					if !schemaExists(schemaNorm, "", scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
						for _, t := range findTokensLocally(raw, []string{schemaNorm}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"Schema '"+t.name+"' does not exist or is not authorized.", 8))
						}
					}
				}
			case 3:
				// A 3-part identifier (DB.SCH.TABLE) is fully self-contained.
				// Only validate if we actually have DB/schema catalog data —
				// otherwise we'd produce false alarms when no session context
				// is set (empty KnownDatabases / KnownSchemas).
				if len(req.KnownDatabases) == 0 {
					break
				}
				dbNorm := parts[0]
				if !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Database '"+t.name+"' does not exist or is not authorized.", 8))
					}
				} else {
					schemaNorm := parts[1]
					schemaPath := dbNorm + "." + schemaNorm
					hasSchemaDataForDB :=
						len(schemasForDB(req.KnownSchemas, dbNorm, checkEq)) > 0 ||
							isIn(scriptEverCreatedSchemasByDB, dbNorm)
					if !hasSchemaDataForDB {
						for _, ref := range req.ResolvedRefs {
							if checkEq(ref.DB, dbNorm) {
								hasSchemaDataForDB = true
								break
							}
						}
					}
					if hasSchemaDataForDB &&
						!schemaExistsForDB(dbNorm, schemaNorm, schemaPath, scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
						for _, t := range findTokensLocally(raw, []string{schemaNorm}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"Schema '"+dbNorm+"."+t.name+"' does not exist or is not authorized.", 8))
						}
					}
				}
			}
		}

		// ── CREATE SCHEMA ─────────────────────────────────────────────
		if rawPath, ok := matchCreateSchema(sig, raw); ok {
			parts := extractIdentParts(rawPath, ic)
			rawParts, _ := readIdentParts(sig, raw, findPathStartInSig(sig, raw, rawPath))
			switch len(parts) {
			case 1:
				if !hasSessionDB && !scriptHasActiveDB {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No database selected. Cannot create schema '"+t.name+"'.", 8))
					}
				}
			case 2:
				dbNorm := parts[0]
				if len(req.KnownDatabases) > 0 {
					if !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
						searchToken := parts[0]
						if len(rawParts) > 0 {
							searchToken = normIdent(rawParts[0], ic)
						}
						for _, t := range findTokensLocally(raw, []string{searchToken}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"Database '"+t.name+"' does not exist or is not authorized.", 8))
						}
					}
				}
			}
		}

		// ── DROP DATABASE ─────────────────────────────────────────────
		if rawPath, hasIfExists, ok := matchDropDB(sig, raw); ok {
			if !hasIfExists && len(req.KnownDatabases) > 0 {
				dbNorm := normIdent(rawPath, ic)
				if !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Database '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── DROP SCHEMA ───────────────────────────────────────────────
		if rawPath, hasIfExists, ok := matchDropSchema(sig, raw); ok {
			if !hasIfExists {
				parts := extractIdentParts(rawPath, ic)
				rawParts, _ := readIdentParts(sig, raw, findPathStartInSig(sig, raw, rawPath))
				var targetDB, targetSch string
				if len(parts) >= 2 {
					targetDB = parts[0]
					targetSch = parts[1]
				} else {
					targetSch = parts[0]
				}
				if targetDB != "" {
					if !dbExists(targetDB, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
						searchToken := targetDB
						if len(rawParts) > 0 {
							searchToken = normIdent(rawParts[0], ic)
						}
						for _, t := range findTokensLocally(raw, []string{searchToken}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"Database '"+t.name+"' does not exist or is not authorized.", 8))
						}
					} else {
						schPath := targetDB + "." + targetSch
						if !schemaExistsForDB(targetDB, targetSch, schPath, scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
							for _, t := range findTokensLocally(raw, []string{targetSch}, r.StartLine, ic) {
								markers = append(markers, diagMarkerAt(t,
									"Schema '"+t.name+"' does not exist or is not authorized.", 8))
							}
						}
					}
				} else {
					if !hasSessionDB && !scriptHasActiveDB {
						for _, t := range findTokensLocally(raw, []string{targetSch}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"No database selected. Cannot drop schema '"+t.name+"'.", 8))
						}
					} else {
						if !schemaExists(targetSch, "", scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
							for _, t := range findTokensLocally(raw, []string{targetSch}, r.StartLine, ic) {
								markers = append(markers, diagMarkerAt(t,
									"Schema '"+t.name+"' does not exist or is not authorized.", 8))
							}
						}
					}
				}
			}
		}

		// ── UNDROP DATABASE ───────────────────────────────────────────
		if rawPath, ok := matchUndropDB(sig, raw); ok {
			dbNorm := normIdent(rawPath, ic)
			isDropped := isIn(scriptDroppedDbsAndSchemas, dbNorm) ||
				anyEq(req.DroppedDatabases, dbNorm, checkEq)
			if !isDropped {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' is not available to undrop.", 8))
				}
			}
		}

		// ── UNDROP SCHEMA ─────────────────────────────────────────────
		if rawPath, ok := matchUndropSchema(sig, raw); ok {
			parts := extractIdentParts(rawPath, ic)
			targetSch := parts[len(parts)-1]
			path := strings.Join(parts, ".")
			isDropped := isIn(scriptDroppedDbsAndSchemas, targetSch) ||
				isIn(scriptDroppedDbsAndSchemas, path) ||
				anySchEq(req.DroppedSchemas, targetSch, checkEq)
			if !isDropped {
				for _, t := range findTokensLocally(raw, []string{targetSch}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Schema '"+t.name+"' is not available to undrop.", 8))
				}
			}
		}

		// ── UNDROP TABLE ──────────────────────────────────────────────
		if rawPath, ok := matchUndropTable(sig, raw); ok {
			parts := extractIdentParts(rawPath, ic)
			targetTab := parts[len(parts)-1]
			path := strings.Join(parts, ".")
			isDropped := isIn(scriptDroppedTables, targetTab) ||
				isIn(scriptDroppedTables, path) ||
				anyRefEq(req.DroppedTables, targetTab, checkEq)
			if !isDropped {
				for _, t := range findTokensLocally(raw, []string{targetTab}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Table '"+t.name+"' is not available to undrop.", 8))
				}
			}
		}

		// ── ALTER TABLE/VIEW ──────────────────────────────────────────
		if rawPath, _, hasIfExists, ok := matchAlterTV(sig, raw); ok {
			if !hasIfExists {
				parts := extractIdentParts(rawPath, ic)
				ftTable := parts[len(parts)-1]
				ftDB := ""
				ftSchema := ""
				if len(parts) == 3 {
					ftDB = parts[0]
					ftSchema = parts[1]
				} else if len(parts) == 2 {
					ftSchema = parts[0]
				}
				path := strings.Join(parts, ".")
				if !isIn(scriptCreatedTables, ftTable) && !isIn(scriptCreatedTables, path) {
					isLive := anyRefMatch(req.ResolvedRefs, ftTable, ftDB, ftSchema, checkEq)
					if !isLive {
						if ftDB != "" && len(req.KnownDatabases) == 0 {
							continue
						}
						badToken, msgFn := resolveErrorToken(ftTable, ftDB, ftSchema,
							scriptCreatedDbsAndSchemas, req.KnownDatabases, req.KnownSchemas, req.ResolvedRefs, checkEq)
						for _, t := range findTokensLocally(raw, []string{badToken}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t, msgFn(t.name), 8))
						}
					}
				}
			}

			// ── ALTER TABLE … SWAP WITH: validate the target table ──────
			if tgtPath, ok := findSwapWith(sig, raw); ok {
				tgtParts := extractIdentParts(tgtPath, ic)
				tgtTable := tgtParts[len(tgtParts)-1]
				tgtDB := ""
				tgtSchema := ""
				if len(tgtParts) == 3 {
					tgtDB = tgtParts[0]
					tgtSchema = tgtParts[1]
				} else if len(tgtParts) == 2 {
					tgtSchema = tgtParts[0]
				}
				tgtPathStr := strings.Join(tgtParts, ".")
				if !isIn(scriptCreatedTables, tgtTable) && !isIn(scriptCreatedTables, tgtPathStr) {
					isLive := anyRefMatch(req.ResolvedRefs, tgtTable, tgtDB, tgtSchema, checkEq)
					if !isLive {
						if tgtDB != "" && len(req.KnownDatabases) == 0 {
							continue
						}
						badToken, msgFn := resolveErrorToken(tgtTable, tgtDB, tgtSchema,
							scriptCreatedDbsAndSchemas, req.KnownDatabases, req.KnownSchemas, req.ResolvedRefs, checkEq)
						for _, t := range findTokensLocally(raw, []string{badToken}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t, msgFn(t.name), 8))
						}
					}
				}
			}
		}

		// ── Validate USE DATABASE <name> ──────────────────────────────────────
		if u, ok := matchUse(sig, raw); ok && u.kind == "DATABASE" {
			dbNorm := normIdent(u.ident1, ic)
			if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' does not exist or is not authorized.", 8))
				}
			}
		}

		// ── Validate USE SCHEMA <db>.<schema> ─────────────────────────────────
		if u, ok := matchUse(sig, raw); ok && u.kind == "SCHEMA" && u.parts == 2 {
			dbNorm := normIdent(u.ident1, ic)
			schNorm := normIdent(u.ident2, ic)
			if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' does not exist or is not authorized.", 8))
				}
			} else {
				schPath := dbNorm + "." + schNorm
				if !schemaExistsForDB(dbNorm, schNorm, schPath, scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{schNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Schema '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── Validate USE SCHEMA <schema> (one-part, no dot) ───────────────────
		if u, ok := matchUse(sig, raw); ok && u.kind == "SCHEMA" && u.parts == 1 {
			schNorm := normIdent(u.ident1, ic)
			if len(req.KnownSchemas) > 0 || anyHasSchema(req.ResolvedRefs) {
				if !schemaExists(schNorm, "", scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{schNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Schema '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── Validate bare USE <db>.<schema> (no keyword) ──────────────────────
		if u, ok := matchUse(sig, raw); ok && u.kind == "" && u.parts == 2 {
			dbNorm := normIdent(u.ident1, ic)
			schNorm := normIdent(u.ident2, ic)
			if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' does not exist or is not authorized.", 8))
				}
			} else {
				schPath := dbNorm + "." + schNorm
				if !schemaExistsForDB(dbNorm, schNorm, schPath, scriptCreatedDbsAndSchemas, req.KnownSchemas, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{schNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Schema '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── Validate bare USE <name> (no keyword, no dot) → USE DATABASE ──────
		if u, ok := matchUse(sig, raw); ok && u.kind == "" && u.parts == 1 {
			dbNorm := normIdent(u.ident1, ic)
			if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' does not exist or is not authorized.", 8))
				}
			}
		}

		// ── Object-kind references (phase 2): ALTER/DROP/DESCRIBE/COMMENT ON
		// <kind> <name>, where the statement names its own schema-scoped kind.
		// Runs BEFORE section (d) so a DROP statement is validated against the
		// pre-drop state (its own drop effect has not been applied yet).
		markers = append(markers, validateObjectKindRefs(
			raw, sig, r.StartLine, ic, checkEq, req.KnownObjects,
			req.FetchedObjectSchemas, req.SessionDatabase, req.SessionSchema,
			scriptCreatedByKind, scriptDroppedByKind)...)

		// ── (d) Apply DROP/UNDROP effects after validation ─────────
		// Runs before the SELECT/WITH continue so DROP TABLE etc. always
		// update state even though DROP is not in the SELECT/WITH list.
		if rawPath, ok := matchDropTable(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				name, path := parts[len(parts)-1], strings.Join(parts, ".")
				scriptDroppedTables[name] = struct{}{}
				scriptDroppedTables[path] = struct{}{}
				delete(scriptCreatedTables, name)
				delete(scriptCreatedTables, path)
			}
		}
		if rawPath, ok := matchDropDbSch(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				name, path := parts[len(parts)-1], strings.Join(parts, ".")
				scriptDroppedDbsAndSchemas[name] = struct{}{}
				scriptDroppedDbsAndSchemas[path] = struct{}{}
				delete(scriptCreatedDbsAndSchemas, name)
				delete(scriptCreatedDbsAndSchemas, path)
			}
		}
		if rawPath, ok := matchUndropTable(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				scriptCreatedTables[parts[len(parts)-1]] = struct{}{}
				scriptCreatedTables[strings.Join(parts, ".")] = struct{}{}
			}
		}
		if rawPath, ok := matchUndropDbSch(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				scriptCreatedDbsAndSchemas[parts[len(parts)-1]] = struct{}{}
				scriptCreatedDbsAndSchemas[strings.Join(parts, ".")] = struct{}{}
			}
		}
		if rawPath, objType, _, ok := matchDropSchemaScoped(sig, raw); ok {
			if parts := extractIdentParts(rawPath, ic); len(parts) > 0 {
				regByKind(scriptDroppedByKind, objType, parts)
				unregByKind(scriptCreatedByKind, objType, parts)
			}
		}

		// ── Stage references (@stg) — runs for every statement kind (PUT,
		// GET, LIST, REMOVE, COPY, SELECT … FROM @stg), so it must precede the
		// SELECT/WITH continue below. ─────────────────────────────────────────
		markers = append(markers, validateStageRefs(
			raw, sig, r.StartLine, ic, checkEq, objectsOfKind(req.KnownObjects, "STAGE"),
			req.FetchedObjectSchemas, req.SessionDatabase, req.SessionSchema,
			scriptCreatedByKind["stage"], scriptDroppedByKind["stage"])...)

		// ── SELECT / WITH / CREATE AS SELECT: table existence ─────────
		if firstKw != "SELECT" && firstKw != "WITH" && firstKw != "CREATE" && firstKw != "UNDROP" &&
			firstKw != "MERGE" && firstKw != "INSERT" && firstKw != "UPDATE" && firstKw != "DELETE" {
			continue
		}
		if matchesSnowflakeFP(sig, raw) {
			continue
		}
		strippedCtx := strings.TrimSpace(stripCommentsSQL(raw))

		parseText := strings.TrimRight(strings.TrimSpace(raw), "; \t\r\n")

		// For CREATE DYNAMIC TABLE, extract the SELECT part.
		if isCreateDynTable(parseText) {
			asOffset := findDynAsSelect(sig, raw)
			if asOffset >= 0 {
				parseText = parseText[asOffset:]
			} else {
				continue
			}
		}

		// Extract FROM tables using token matching.
		// Strip single-quoted string literals first so keywords inside
		// strings (e.g. 'USING CRON …' in CREATE TASK SCHEDULE clauses)
		// are not mistaken for SQL syntax.
		type fromTable struct {
			db, schema, name string
		}
		var fromTables []fromTable
		noStringsCtx := stripStringLiterals(strippedCtx)
		noStringsSig := sigTokens(noStringsCtx)
		for _, path := range findFromJoinTables(noStringsSig, noStringsCtx) {
			parts := extractIdentParts(path, ic)
			switch len(parts) {
			case 3:
				fromTables = append(fromTables, fromTable{parts[0], parts[1], parts[2]})
			case 2:
				fromTables = append(fromTables, fromTable{"", parts[0], parts[1]})
			case 1:
				fromTables = append(fromTables, fromTable{"", "", parts[0]})
			}
		}

		// Also handle CREATE TABLE ... REFERENCES
		if isCreateTable(parseText) {
			parseSig := sigTokens(parseText)
			for _, rm := range findReferences(parseSig, parseText) {
				parts := extractIdentParts(rm.tablePath, ic)
				switch len(parts) {
				case 3:
					fromTables = append(fromTables, fromTable{parts[0], parts[1], parts[2]})
				case 2:
					fromTables = append(fromTables, fromTable{"", parts[0], parts[1]})
				case 1:
					fromTables = append(fromTables, fromTable{"", "", parts[0]})
				}
			}
		}

		// CTE names — collected structurally from the statement's WITH clauses
		// by the shared sqlgrammar.CollectCTENames scanner.
		strippedSig := sigTokens(strippedCtx)
		cteNames := findCTENames(strippedSig, strippedCtx, ic)

		missingTokens := make(map[string]func(string) string)

		for _, ft := range fromTables {
			ftTable := ft.name
			compareTable := ftTable
			upperCompare := strings.ToUpper(compareTable)
			// VALUES is a table literal (`FROM VALUES (...), (...)`), not a table
			// name — skip it like the TABLE keyword.
			if (upperCompare == "TABLE" || upperCompare == "VALUES" || joinStopKW[upperCompare]) && ft.db == "" && ft.schema == "" {
				continue
			}
			// Skip SNOWFLAKE.CORTEX.* — built-in Cortex AI function namespace,
			// not a real database/schema/table path.
			if ft.db != "" && ft.schema != "" &&
				strings.EqualFold(ft.db, "SNOWFLAKE") && strings.EqualFold(ft.schema, "CORTEX") {
				continue
			}
			if _, isCTE := cteNames[compareTable]; isCTE {
				continue
			}
			if _, isSC := scriptCreatedTables[compareTable]; isSC {
				continue
			}

			isLive := anyRefMatch(req.ResolvedRefs, compareTable, ft.db, ft.schema, checkEq)
			if isLive {
				continue
			}

			// A 3-part identifier (DB.SCH.TABLE) is fully self-contained.
			// Only validate if we actually have database catalog data —
			// otherwise we'd produce false alarms when no session context
			// is set (empty KnownDatabases).
			if ft.db != "" && len(req.KnownDatabases) == 0 {
				continue
			}

			badToken, msgFn := resolveErrorToken(ftTable, ft.db, ft.schema,
				scriptCreatedDbsAndSchemas, req.KnownDatabases, req.KnownSchemas, req.ResolvedRefs, checkEq)
			missingTokens[badToken] = msgFn
		}

		if len(missingTokens) == 0 {
			continue
		}
		unknown := make([]string, 0, len(missingTokens))
		for k := range missingTokens {
			unknown = append(unknown, k)
		}
		for _, t := range findTokensLocally(raw, unknown, r.StartLine, ic) {
			name := t.name
			if !t.quoted {
				name = strings.ToUpper(name)
			}
			msgFn := missingTokens[name]
			var diagMsg string
			if msgFn != nil {
				diagMsg = msgFn(t.name)
			} else {
				diagMsg = "Object '" + t.name + "' does not exist or is not authorized."
			}
			marker := diagMarkerAt(t, diagMsg, 8)

			// Populate quick-fix Code when the unresolved token is a table name
			// and alternative fully-qualified paths exist in AllKnownTables.
			if len(req.AllKnownTables) > 0 {
				marker.Code = buildQualifyTableCode(name, req.AllKnownTables, checkEq)
			}

			markers = append(markers, marker)
		}
	}

	return markers
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// findPathStartInSig finds the position in sig where the given rawPath begins.
// This is used to call readIdentParts for getting raw token texts.
func findPathStartInSig(sig []sqltok.Token, sql, rawPath string) int {
	if rawPath == "" {
		return 0
	}
	for i, tok := range sig {
		if isIdent(tok) && tok.Start == strings.Index(sql, rawPath) {
			return i
		}
	}
	// Fallback: scan for the path start byte offset
	pathStart := strings.Index(sql, rawPath)
	if pathStart < 0 {
		return 0
	}
	for i, tok := range sig {
		if tok.Start == pathStart {
			return i
		}
	}
	return 0
}

func isIn(m map[string]struct{}, key string) bool {
	_, ok := m[key]
	return ok
}

func anyHasSchema(refs []ResolvedRef) bool {
	for _, r := range refs {
		if r.Schema != "" {
			return true
		}
	}
	return false
}

func anyEq(ss []string, target string, eq func(string, string) bool) bool {
	for _, s := range ss {
		if eq(s, target) {
			return true
		}
	}
	return false
}

func anySchEq(schemas []SchemaEntry, targetName string, eq func(string, string) bool) bool {
	for _, s := range schemas {
		if eq(s.Name, targetName) {
			return true
		}
	}
	return false
}

func anyRefEq(refs []ResolvedRef, targetName string, eq func(string, string) bool) bool {
	for _, r := range refs {
		if eq(r.Name, targetName) {
			return true
		}
	}
	return false
}

func anyRefMatch(refs []ResolvedRef, name, db, schema string, eq func(string, string) bool) bool {
	for _, r := range refs {
		if !eq(r.Name, name) {
			continue
		}
		if db != "" && !eq(r.DB, db) {
			continue
		}
		if schema != "" && !eq(r.Schema, schema) {
			continue
		}
		return true
	}
	return false
}

func dbExists(dbNorm string, created map[string]struct{}, knownDBs []string, refs []ResolvedRef, eq func(string, string) bool) bool {
	if isIn(created, dbNorm) {
		return true
	}
	if len(knownDBs) > 0 {
		return anyEq(knownDBs, dbNorm, eq)
	}
	return anyRefMatch(refs, "", dbNorm, "", eq) || anyRefMatchDB(refs, dbNorm, eq)
}

func anyRefMatchDB(refs []ResolvedRef, db string, eq func(string, string) bool) bool {
	for _, r := range refs {
		if eq(r.DB, db) {
			return true
		}
	}
	return false
}

func schemaExists(schemaNorm, _ string, created map[string]struct{}, knownSchemas []SchemaEntry, refs []ResolvedRef, eq func(string, string) bool) bool {
	if isIn(created, schemaNorm) {
		return true
	}
	if len(knownSchemas) > 0 {
		return anySchEq(knownSchemas, schemaNorm, eq)
	}
	for _, r := range refs {
		if eq(r.Schema, schemaNorm) {
			return true
		}
	}
	return false
}

func schemaExistsForDB(dbNorm, schemaNorm, schemaPath string, created map[string]struct{}, knownSchemas []SchemaEntry, refs []ResolvedRef, eq func(string, string) bool) bool {
	if isIn(created, schemaNorm) || isIn(created, schemaPath) {
		return true
	}
	dbSchemas := schemasForDB(knownSchemas, dbNorm, eq)
	if len(dbSchemas) > 0 {
		for _, s := range dbSchemas {
			if eq(s.Name, schemaNorm) {
				return true
			}
		}
		return false
	}
	for _, r := range refs {
		if eq(r.DB, dbNorm) && eq(r.Schema, schemaNorm) {
			return true
		}
	}
	return false
}

func schemasForDB(schemas []SchemaEntry, db string, eq func(string, string) bool) []SchemaEntry {
	var out []SchemaEntry
	for _, s := range schemas {
		if eq(s.DB, db) {
			out = append(out, s)
		}
	}
	return out
}

// resolveErrorToken determines which token to highlight (table, schema, or DB)
// and returns the appropriate error message function.
func resolveErrorToken(
	ftTable, ftDB, ftSchema string,
	created map[string]struct{},
	knownDBs []string,
	knownSchemas []SchemaEntry,
	refs []ResolvedRef,
	eq func(string, string) bool,
) (badToken string, msgFn func(string) string) {
	badToken = ftTable
	msgFn = func(n string) string { return "Table or View '" + n + "' does not exist or is not authorized." }

	if ftDB != "" {
		if !dbExists(ftDB, created, knownDBs, refs, eq) {
			badToken = ftDB
			msgFn = func(n string) string { return "Database '" + n + "' does not exist or is not authorized." }
			return
		}
		if ftSchema != "" {
			schPath := ftDB + "." + ftSchema
			if !schemaExistsForDB(ftDB, ftSchema, schPath, created, knownSchemas, refs, eq) {
				badToken = ftSchema
				msgFn = func(n string) string { return "Schema '" + n + "' does not exist or is not authorized." }
				return
			}
		}
	} else if ftSchema != "" {
		if !schemaExists(ftSchema, "", created, knownSchemas, refs, eq) {
			badToken = ftSchema
			msgFn = func(n string) string { return "Schema '" + n + "' does not exist or is not authorized." }
			return
		}
	}
	return
}

// qualifyTableCodePayload is the JSON structure embedded in DiagMarker.Code
// when an unresolved table name has alternative fully-qualified paths.
type qualifyTableCodePayload struct {
	Kind        string   `json:"kind"`
	Original    string   `json:"original"`
	Suggestions []string `json:"suggestions"`
}

// buildQualifyTableCode searches AllKnownTables for tables whose name matches
// the unresolved token and returns a JSON string with qualification suggestions.
// Returns "" if no matches are found.
func buildQualifyTableCode(tableName string, allKnown []ResolvedRef, eq func(string, string) bool) string {
	var suggestions []string
	seen := make(map[string]struct{})
	for _, ref := range allKnown {
		if !eq(ref.Name, tableName) {
			continue
		}
		var qualified string
		if ref.DB != "" && ref.Schema != "" {
			qualified = ref.DB + "." + ref.Schema + "." + ref.Name
		} else if ref.Schema != "" {
			qualified = ref.Schema + "." + ref.Name
		} else {
			continue
		}
		upper := strings.ToUpper(qualified)
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		suggestions = append(suggestions, qualified)
	}
	if len(suggestions) == 0 {
		return ""
	}
	payload := qualifyTableCodePayload{
		Kind:        "qualify-table",
		Original:    tableName,
		Suggestions: suggestions,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
