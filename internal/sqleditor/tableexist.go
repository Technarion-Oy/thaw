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
	"regexp"
	"strings"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SchemaEntry pairs a database name with a schema name.
type SchemaEntry struct {
	DB   string `json:"db"`
	Name string `json:"name"`
}

// ValidateTablesExistRequest is the input to ValidateTablesExist.
type ValidateTablesExistRequest struct {
	SQL                         string         `json:"sql"`
	StmtRanges                  []StatementRange `json:"stmtRanges"`
	ResolvedRefs                []ResolvedRef  `json:"resolvedRefs"`
	KnownDatabases              []string       `json:"knownDatabases"`
	KnownSchemas                []SchemaEntry  `json:"knownSchemas"`
	QuotedIdentifiersIgnoreCase bool           `json:"quotedIdentifiersIgnoreCase"`
	DroppedDatabases            []string       `json:"droppedDatabases"`
	DroppedSchemas              []SchemaEntry  `json:"droppedSchemas"`
	// DroppedTables uses ResolvedRef (Alias field is ignored).
	DroppedTables []ResolvedRef `json:"droppedTables"`
}

// ── Precompiled regexes ───────────────────────────────────────────────────────

var (
	// CREATE TABLE/VIEW
	reCreateTVMatch = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:INTERACTIVE\s+)?` +
			`(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE|TRANSIENT)\s+)?` +
			`(?:RECURSIVE\s+)?(?:MATERIALIZED\s+)?(?:TABLE|VIEW)\s+` +
			`(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)

	// CREATE DATABASE/SCHEMA
	reCreateDbSchMatch = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?` +
			`(?:DATABASE|SCHEMA)\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)

	// DROP TABLE
	reDropTableMatch = regexp.MustCompile(
		`(?i)^\s*DROP\s+(?:TABLE)\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)

	// DROP DATABASE/SCHEMA
	reDropDbSchMatch = regexp.MustCompile(
		`(?i)^\s*DROP\s+(?:DATABASE|SCHEMA)\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)

	// UNDROP TABLE
	reUndropTableMatch = regexp.MustCompile(
		`(?i)^\s*UNDROP\s+TABLE\s+(` + _identPath + `)`)

	// UNDROP DATABASE/SCHEMA
	reUndropDbSchMatch = regexp.MustCompile(
		`(?i)^\s*UNDROP\s+(?:DATABASE|SCHEMA)\s+(` + _identPath + `)`)

	// USE DATABASE / USE SCHEMA / USE <db>.<schema>
	reUseDatabase      = regexp.MustCompile(`(?i)^\s*USE\s+DATABASE\s+`)
	reUseSchema        = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+`)
	reUseTwoPartSchema = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+` + _ident + `\.` + _ident)
	// Go regexp does not support negative lookaheads, so capture the first
	// token and reject DATABASE/SCHEMA/ROLE/WAREHOUSE in code.
	reUseTwoPart = regexp.MustCompile(`(?i)^\s*USE\s+(` + _ident + `)\.` + _ident)

	// Capturing variants used for existence validation.
	// reUseDatabaseIdent: captures DB name from USE DATABASE <name>
	reUseDatabaseIdent = regexp.MustCompile(`(?i)^\s*USE\s+DATABASE\s+(` + _ident + `)`)
	// reUseSchemaIdents: captures (db, schema) from USE SCHEMA <db>.<schema>
	reUseSchemaIdents = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+(` + _ident + `)\.(` + _ident + `)`)
	// reUseSchemaIdent: captures schema from USE SCHEMA <schema> (one-part, no dot)
	reUseSchemaIdent = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+(` + _ident + `)[;\s]*$`)
	// reUseTwoPartIdents: captures (db, schema) from bare USE <db>.<schema> (no keyword)
	reUseTwoPartIdents = regexp.MustCompile(`(?i)^\s*USE\s+(` + _ident + `)\.(` + _ident + `)`)
	// reUseOnePartIdent: captures name from bare USE <name> (no keyword, no dot)
	reUseOnePartIdent = regexp.MustCompile(`(?i)^\s*USE\s+(` + _ident + `)[;\s]*$`)
	reCreateAnyDb      = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?DATABASE\b`)
	reCreateAnySchema  = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\b`)

	// CREATE TABLE/VIEW (context – for qualification checks)
	reCreateTVCtxMatch = reCreateTVMatch // same regex

	// CREATE SCHEMA (with target path)
	reCreateSchemaMatch = regexp.MustCompile(
		`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\s+` +
			`(?:IF\s+NOT\s+EXISTS\s+)?(` + _identPath + `)`)

	// DROP DATABASE (with optional IF EXISTS)
	reDropDbMatch = regexp.MustCompile(
		`(?i)^\s*DROP\s+DATABASE\s+(?:IF\s+EXISTS\s+)?(` + _ident + `)`)
	reDropSchemaMatch = regexp.MustCompile(
		`(?i)^\s*DROP\s+SCHEMA\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)
	reIfExists = regexp.MustCompile(`(?i)\bIF\s+EXISTS\b`)

	// UNDROP DATABASE / UNDROP SCHEMA / UNDROP TABLE
	reUndropDbMatch  = regexp.MustCompile(`(?i)^\s*UNDROP\s+DATABASE\s+(` + _ident + `)`)
	reUndropSchMatch = regexp.MustCompile(`(?i)^\s*UNDROP\s+SCHEMA\s+(` + _identPath + `)`)
	reUndropTabMatch = regexp.MustCompile(`(?i)^\s*UNDROP\s+TABLE\s+(` + _identPath + `)`)

	// ALTER TABLE/VIEW
	reAlterTVMatch = regexp.MustCompile(
		`(?i)^\s*ALTER\s+(TABLE|VIEW)\s+(?:IF\s+EXISTS\s+)?(` + _identPath + `)`)

	// FROM/JOIN regex for fallback table extraction
	reFromJoinFallback = regexp.MustCompile(
		`(?i)(?:FROM|JOIN|MERGE\s+INTO|USING|INSERT\s+INTO|UPDATE|CLONE|LIKE)\s+(` + _identPath + `)`)

	// CREATE DYNAMIC TABLE → extract SELECT portion.
	// Go regexp has no lookahead; capture SELECT|WITH so the caller can slice
	// from asM[2] (start of the captured keyword) to keep it in the result.
	reDynAsMatch = regexp.MustCompile(`(?i)\bAS\s+(SELECT|WITH)\b`)
)

// ── ValidateTablesExist ───────────────────────────────────────────────────────

// ValidateTablesExist checks each statement for references to databases,
// schemas, or tables/views that are not in the resolved references or known
// catalogs.  It is a direct Go port of the validateTablesExist function from
// sqlDiagnostics.ts.
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
	scriptCreatedTables        := make(map[string]struct{})
	scriptCreatedDbsAndSchemas := make(map[string]struct{})
	scriptDroppedTables        := make(map[string]struct{})
	scriptDroppedDbsAndSchemas := make(map[string]struct{})
	// scriptEverCreatedSchemasByDB tracks every DB for which a 2-part
	// CREATE SCHEMA db.sch appeared in the script (append-only; not cleared by
	// DROP).  Used to decide whether to validate schema references in a
	// 3-part CREATE TABLE — if we've seen "CREATE SCHEMA DB.SCH" earlier in
	// the same script, we have enough context to validate that the schema still
	// exists (and catch "schema was dropped" errors).
	scriptEverCreatedSchemasByDB := make(map[string]struct{})

	scriptHasActiveDB     := false
	scriptHasActiveSchema := false

	for _, r := range req.StmtRanges {
		raw := sqlStmt(req.SQL, r)

		// ── (a) Apply CREATE effects before validation ─────────────
		if m := reCreateTVMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
				scriptCreatedTables[parts[len(parts)-1]] = struct{}{}
				scriptCreatedTables[strings.Join(parts, ".")] = struct{}{}
			}
		}
		if m := reCreateDbSchMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
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
		if reUseDatabase.MatchString(raw) || reCreateAnyDb.MatchString(raw) {
			scriptHasActiveDB = true
		}
		if reUseTwoPartSchema.MatchString(raw) {
			scriptHasActiveDB = true
		}
		if m2 := reUseTwoPart.FindStringSubmatch(raw); m2 != nil {
			first := strings.ToUpper(m2[1])
			if first != "DATABASE" && first != "SCHEMA" && first != "ROLE" && first != "WAREHOUSE" {
				scriptHasActiveDB = true
				scriptHasActiveSchema = true
			}
		}
		if reUseSchema.MatchString(raw) || reCreateAnySchema.MatchString(raw) {
			scriptHasActiveSchema = true
		}
		// Bare USE <name> (no keyword, no dot) is equivalent to USE DATABASE.
		if m := reUseOnePartIdent.FindStringSubmatch(raw); m != nil {
			first := strings.ToUpper(m[1])
			if first != "DATABASE" && first != "SCHEMA" && first != "ROLE" && first != "WAREHOUSE" {
				scriptHasActiveDB = true
			}
		}

		hasGlobalDB     := len(req.KnownDatabases) > 0 || anyHasDB(req.ResolvedRefs)
		hasGlobalSchema := len(req.KnownSchemas) > 0 || anyHasSchema(req.ResolvedRefs)

		// ── CREATE TABLE/VIEW ─────────────────────────────────────────
		if m := reCreateTVCtxMatch.FindStringSubmatch(raw); m != nil {
			parts := extractIdentParts(m[1], ic)
			rawParts := reIdentOrQuoted.FindAllString(m[1], -1)
			objType := "table"
			if regexp.MustCompile(`(?i)\bVIEW\b`).MatchString(m[0]) {
				objType = "view"
			}

			switch len(parts) {
			case 1:
				if !hasGlobalDB && !scriptHasActiveDB {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No database selected. Cannot create "+objType+" '"+t.name+"'.", 8))
					}
				} else if !hasGlobalSchema && !scriptHasActiveSchema {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No schema selected. Cannot create "+objType+" '"+t.name+"'.", 8))
					}
				}
			case 2:
				schemaNorm := parts[0]
				if !hasGlobalDB && !scriptHasActiveDB {
					for _, t := range findTokensLocally(raw, []string{normIdent(rawParts[0], ic)}, r.StartLine, ic) {
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
					// Only validate the schema if we have actual schema data for
					// this specific DB. Without data we cannot distinguish
					// "schema doesn't exist" from "schema list not yet loaded",
					// which would produce false alarms when no session context
					// is set.  Three sources count as "having schema data":
					//   1. KnownSchemas has at least one entry for this DB
					//   2. ResolvedRefs has at least one ref for this DB
					//   3. The script itself issued CREATE SCHEMA <db>.<sch> for
					//      this DB (tracked in scriptEverCreatedSchemasByDB)
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
		if m := reCreateSchemaMatch.FindStringSubmatch(raw); m != nil {
			parts := extractIdentParts(m[1], ic)
			rawParts := reIdentOrQuoted.FindAllString(m[1], -1)
			hasGlobalDBHere := len(req.KnownDatabases) > 0 || anyHasDB(req.ResolvedRefs)
			switch len(parts) {
			case 1:
				if !hasGlobalDBHere && !scriptHasActiveDB {
					for _, t := range findTokensLocally(raw, []string{parts[0]}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"No database selected. Cannot create schema '"+t.name+"'.", 8))
					}
				}
			case 2:
				dbNorm := parts[0]
				if len(req.KnownDatabases) > 0 {
					if !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
						for _, t := range findTokensLocally(raw, []string{normIdent(rawParts[0], ic)}, r.StartLine, ic) {
							markers = append(markers, diagMarkerAt(t,
								"Database '"+t.name+"' does not exist or is not authorized.", 8))
						}
					}
				}
			}
		}

		// ── DROP DATABASE ─────────────────────────────────────────────
		if m := reDropDbMatch.FindStringSubmatch(raw); m != nil {
			if !reIfExists.MatchString(raw) && len(req.KnownDatabases) > 0 {
				dbNorm := normIdent(m[1], ic)
				if !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Database '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── DROP SCHEMA ───────────────────────────────────────────────
		if m := reDropSchemaMatch.FindStringSubmatch(raw); m != nil {
			if !reIfExists.MatchString(raw) {
				parts := extractIdentParts(m[1], ic)
				rawParts := reIdentOrQuoted.FindAllString(m[1], -1)
				hasGlobalDBHere := len(req.KnownDatabases) > 0 || anyHasDB(req.ResolvedRefs)
				var targetDB, targetSch string
				if len(parts) >= 2 {
					targetDB = parts[0]
					targetSch = parts[1]
				} else {
					targetSch = parts[0]
				}
				if targetDB != "" {
					if !dbExists(targetDB, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
						for _, t := range findTokensLocally(raw, []string{normIdent(rawParts[0], ic)}, r.StartLine, ic) {
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
					if !hasGlobalDBHere && !scriptHasActiveDB {
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
		if m := reUndropDbMatch.FindStringSubmatch(raw); m != nil {
			dbNorm := normIdent(m[1], ic)
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
		if m := reUndropSchMatch.FindStringSubmatch(raw); m != nil {
			parts := extractIdentParts(m[1], ic)
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
		if m := reUndropTabMatch.FindStringSubmatch(raw); m != nil {
			parts := extractIdentParts(m[1], ic)
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
		if m := reAlterTVMatch.FindStringSubmatch(raw); m != nil {
			if !reIfExists.MatchString(raw) {
				parts := extractIdentParts(m[2], ic)
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
		}

		// ── Validate USE DATABASE <name> ──────────────────────────────────────
		if m := reUseDatabaseIdent.FindStringSubmatch(raw); m != nil {
			dbNorm := normIdent(m[1], ic)
			if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
				for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
					markers = append(markers, diagMarkerAt(t,
						"Database '"+t.name+"' does not exist or is not authorized.", 8))
				}
			}
		}

		// ── Validate USE SCHEMA <db>.<schema> ─────────────────────────────────
		if m := reUseSchemaIdents.FindStringSubmatch(raw); m != nil {
			dbNorm := normIdent(m[1], ic)
			schNorm := normIdent(m[2], ic)
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
		if m := reUseSchemaIdent.FindStringSubmatch(raw); m != nil {
			schNorm := normIdent(m[1], ic)
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
		if m := reUseTwoPartIdents.FindStringSubmatch(raw); m != nil {
			first := strings.ToUpper(m[1])
			if first != "DATABASE" && first != "SCHEMA" && first != "ROLE" && first != "WAREHOUSE" {
				dbNorm := normIdent(m[1], ic)
				schNorm := normIdent(m[2], ic)
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
		}

		// ── Validate bare USE <name> (no keyword, no dot) → USE DATABASE ──────
		if m := reUseOnePartIdent.FindStringSubmatch(raw); m != nil {
			first := strings.ToUpper(m[1])
			if first != "DATABASE" && first != "SCHEMA" && first != "ROLE" && first != "WAREHOUSE" {
				dbNorm := normIdent(m[1], ic)
				if len(req.KnownDatabases) > 0 && !dbExists(dbNorm, scriptCreatedDbsAndSchemas, req.KnownDatabases, req.ResolvedRefs, checkEq) {
					for _, t := range findTokensLocally(raw, []string{dbNorm}, r.StartLine, ic) {
						markers = append(markers, diagMarkerAt(t,
							"Database '"+t.name+"' does not exist or is not authorized.", 8))
					}
				}
			}
		}

		// ── (d) Apply DROP/UNDROP effects after validation ─────────
		// Runs before the SELECT/WITH continue so DROP TABLE etc. always
		// update state even though DROP is not in the SELECT/WITH list.
		if m := reDropTableMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
				name, path := parts[len(parts)-1], strings.Join(parts, ".")
				scriptDroppedTables[name] = struct{}{}
				scriptDroppedTables[path] = struct{}{}
				delete(scriptCreatedTables, name)
				delete(scriptCreatedTables, path)
			}
		}
		if m := reDropDbSchMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
				name, path := parts[len(parts)-1], strings.Join(parts, ".")
				scriptDroppedDbsAndSchemas[name] = struct{}{}
				scriptDroppedDbsAndSchemas[path] = struct{}{}
				delete(scriptCreatedDbsAndSchemas, name)
				delete(scriptCreatedDbsAndSchemas, path)
			}
		}
		if m := reUndropTableMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
				scriptCreatedTables[parts[len(parts)-1]] = struct{}{}
				scriptCreatedTables[strings.Join(parts, ".")] = struct{}{}
			}
		}
		if m := reUndropDbSchMatch.FindStringSubmatch(raw); m != nil {
			if parts := extractIdentParts(m[1], ic); len(parts) > 0 {
				scriptCreatedDbsAndSchemas[parts[len(parts)-1]] = struct{}{}
				scriptCreatedDbsAndSchemas[strings.Join(parts, ".")] = struct{}{}
			}
		}

		// ── SELECT / WITH / CREATE AS SELECT: table existence ─────────
		firstTok := getFirstSQLToken(raw)
		if firstTok != "SELECT" && firstTok != "WITH" && firstTok != "CREATE" && firstTok != "UNDROP" &&
			firstTok != "MERGE" && firstTok != "INSERT" && firstTok != "UPDATE" && firstTok != "DELETE" {
			continue
		}
		strippedCtx := strings.TrimSpace(stripCommentsSQL(raw))
		checkCtx := regexp.MustCompile(`(?i)\bCLUSTER\s+BY\s*\([^)]+\)`).ReplaceAllString(strippedCtx, "")
		if reSnowflakeFP.MatchString(checkCtx) {
			continue
		}

		parseText := strings.TrimRight(strings.TrimSpace(raw), "; \t\r\n")

		// For CREATE DYNAMIC TABLE, extract the SELECT part.
		// reDynAsMatch captures SELECT|WITH; asM[2] is the start of that keyword.
		if reIsCreateDynTable.MatchString(parseText) {
			if asM := reDynAsMatch.FindStringSubmatchIndex(parseText); asM != nil {
				parseText = parseText[asM[2]:]
			} else {
				continue
			}
		}

		// Extract FROM tables using regex (mirrors the TS fallback path)
		type fromTable struct {
			db, schema, name string
		}
		var fromTables []fromTable
		for _, fm := range reFromJoinFallback.FindAllStringSubmatch(strippedCtx, -1) {
			parts := extractIdentParts(fm[1], ic)
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
		if reIsCreateTable.MatchString(parseText) {
			reRefs := regexp.MustCompile(`(?i)\bREFERENCES\s+(` + _identPath + `)`)
			for _, rf := range reRefs.FindAllStringSubmatch(parseText, -1) {
				parts := extractIdentParts(rf[1], ic)
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

		// CTE names — every identifier that appears as `name AS (` is a CTE
		// definition.  Using a simple "ident AS (" pattern (without requiring
		// the preceding WITH keyword) captures all CTEs in a multi-CTE query
		// such as:
		//   WITH cte1 AS (...), cte2 AS (...) SELECT ...
		// where the old WITH-anchored regex only found the first CTE name.
		cteNames := make(map[string]struct{})
		reCTEName := regexp.MustCompile(`(?i)\b(` + _ident + `)\s+AS\s*\(`)
		for _, cm := range reCTEName.FindAllStringSubmatch(strippedCtx, -1) {
			cteNames[normIdent(cm[1], ic)] = struct{}{}
		}

		missingTokens := make(map[string]func(string) string)

		for _, ft := range fromTables {
			ftTable := ft.name
			compareTable := strings.ToUpper(ftTable)
			if (compareTable == "TABLE" || joinStopKW[compareTable]) && ft.db == "" && ft.schema == "" {
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
			markers = append(markers, diagMarkerAt(t, diagMsg, 8))
		}
	}

	return markers
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func isIn(m map[string]struct{}, key string) bool {
	_, ok := m[key]
	return ok
}

func anyHasDB(refs []ResolvedRef) bool {
	for _, r := range refs {
		if r.DB != "" {
			return true
		}
	}
	return false
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
