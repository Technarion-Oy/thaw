// SPDX-License-Identifier: GPL-3.0-or-later

package sqleditor

import (
	"context"
	"strings"
)

// ── Shared diagnostics orchestrator ────────────────────────────────────────────
//
// Diagnose is the single Go entry point that drives validation ordering and
// table-ref resolution for the schema-aware SQL diagnostics pipeline. Before it
// existed, the MCP validate_sql tool (internal/mcp/diag_tools.go) hand-mirrored
// the frontend editor pipeline (SqlEditor.tsx runDiagnostics), kept in sync only
// by NOTE comments — the two drifted (issue #354): the frontend dropped
// fully-qualified refs whose db/schema/table weren't in the fetched catalog so
// ValidateTablesExist would flag them, while the backend blindly trusted any
// fully-qualified ref and passed it on to the semantics / bare-column checks.
//
// The orchestrator removes that drift by owning both the phase ordering and the
// ref-resolution semantics. The MCP path calls Diagnose directly. The frontend
// editor still drives the individual validators through granular IPC methods
// because it layers an incremental, offline-first warm-up on top (fetch missing
// schemas/objects, then re-run) that can't live server-side — but the
// ref-resolution semantics now match: resolveRefsForDiagnostics is a faithful
// Go port of runDiagnostics' `resolved` mapping, so both editor and MCP drop the
// same refs. See internal/sqleditor/README.md and internal/mcp/README.md.

// SchemaProvider supplies the live Snowflake metadata that the schema-aware
// diagnostics phase (phase 2) needs. Production wires a *snowflake.Client
// adapter (internal/mcp); tests supply an in-memory fake. The interface is
// expressed entirely in sqleditor terms (no snowflake import) so the schema-aware
// path is testable with a fake client — the longer-term ask in issue #354.
type SchemaProvider interface {
	// SessionContext returns the connection's active database/schema.
	SessionContext(ctx context.Context) (SessionContext, error)
	// ListDatabases returns every database name visible to the session.
	ListDatabases(ctx context.Context) ([]string, error)
	// ListSchemas returns the schema names in the given database.
	ListSchemas(ctx context.Context, database string) ([]string, error)
	// ListObjects returns the objects (tables, views, stages, …) in the given
	// database.schema. Each StoreObject should have DB/Schema/Name/Kind set.
	ListObjects(ctx context.Context, database, schema string) ([]StoreObject, error)
	// TableColumns returns the ordered columns (name + type) for the table or
	// view database.schema.name.
	TableColumns(ctx context.Context, database, schema, name string) ([]ColInfo, error)
}

// Diagnose runs the full SQL diagnostics pipeline and returns the combined
// marker set. Phase 1 (pure validations — syntax, data types, grammar,
// anti-patterns) always runs and needs no network. Phase 2 (schema-aware — table
// existence, semantics, bare column refs) runs best-effort when provider is
// non-nil; if any phase-2 step fails, only the phase-1 markers are returned and
// the (non-fatal) error is reported so the caller can log it.
//
// The returned slice is never combined with the error: callers should use the
// markers regardless of err (err only signals "phase 2 was skipped").
func Diagnose(ctx context.Context, provider SchemaProvider, sql string) ([]DiagMarker, error) {
	stmtRanges := GetStatementRanges(sql)
	markers := runPureValidations(sql, stmtRanges)

	if provider == nil {
		return markers, nil
	}

	phase2, err := diagnoseSchemaAware(ctx, provider, sql, stmtRanges)
	if err != nil {
		return markers, err
	}
	return append(markers, phase2...), nil
}

// runPureValidations runs the phase-1 (no-network) validators in the same order
// as the frontend editor pipeline: syntax → data types → grammar → anti-patterns.
func runPureValidations(sql string, stmtRanges []StatementRange) []DiagMarker {
	syntaxMarkers := ValidateSyntax(sql)
	datatypeMarkers := ValidateDataTypes(sql, stmtRanges)
	grammarMarkers := ValidateGrammar(sql, stmtRanges)
	antiPatternMarkers := ValidateAntiPatterns(sql, stmtRanges)

	return append(append(append(syntaxMarkers, datatypeMarkers...), grammarMarkers...), antiPatternMarkers...)
}

// diagnoseSchemaAware runs the schema-aware portion of the pipeline. It returns
// an error if any metadata step fails, allowing Diagnose to fall back to phase-1
// markers only.
func diagnoseSchemaAware(ctx context.Context, provider SchemaProvider, sql string, stmtRanges []StatementRange) ([]DiagMarker, error) {
	sc, err := provider.SessionContext(ctx)
	if err != nil {
		return nil, err
	}
	session := &SessionContext{Database: sc.Database, Schema: sc.Schema}

	refs := ParseJoinTables(sql)

	// Short-circuit: no table references AND no stage references → nothing
	// schema-aware to check. A stage-only statement (e.g. `SELECT $1 FROM
	// @stg/f.csv`) has no table refs but must still reach validateStageRefs via
	// ValidateTablesExist, so it isn't short-circuited away (issue #793 G). The
	// session db/schema is always warmed by gatherCatalog, giving the stage check
	// its catalog even with no table refs.
	if len(refs) == 0 && !hasStageRef(sql) {
		return nil, nil
	}

	cat, err := gatherCatalog(ctx, provider, refs, session)
	if err != nil {
		return nil, err
	}

	resolvedRefs := resolveRefsForDiagnostics(refs, cat)

	// Table-existence validation. KnownObjects/FetchedObjectSchemas bring the
	// stage / object-kind existence checks (stageexist.go, objectkindexist.go)
	// into parity with the frontend, which passes the same catalog data.
	tableExistMarkers := ValidateTablesExist(ValidateTablesExistRequest{
		SQL:                  sql,
		StmtRanges:           stmtRanges,
		ResolvedRefs:         resolvedRefs,
		KnownDatabases:       cat.databases,
		KnownSchemas:         cat.schemas,
		SessionDatabase:      session.Database,
		SessionSchema:        session.Schema,
		AllKnownTables:       tableViewRefs(cat.objects),
		KnownObjects:         storeObjectsToObjectRefs(cat.objects),
		FetchedObjectSchemas: cat.fetchedObjectSchemas(),
	})

	colEntries := gatherColumns(ctx, provider, resolvedRefs)

	semanticMarkers := ValidateSemantics(sql, resolvedRefs, colEntries)

	bareColMarkers := ValidateBareColumnRefs(ValidateBareColsRequest{
		SQL:          sql,
		StmtRanges:   stmtRanges,
		ResolvedRefs: resolvedRefs,
		ColEntries:   colEntries,
	})

	return append(append(tableExistMarkers, semanticMarkers...), bareColMarkers...), nil
}

// diagCatalog holds the Snowflake metadata gathered for a diagnostics run.
// The fetched* sets are the guards that keep resolution honest: a catalog level
// (schemas of a DB, objects of a schema) is only used to reject a reference when
// that level was actually fetched — mirroring the frontend's fetchedDatabaseSchemas
// / fetchedSchemaObjects guards, so shared/unlistable DBs never false-positive.
type diagCatalog struct {
	databases []string      // all DBs (from ListDatabases); empty ⇒ DB existence unknown
	schemas   []SchemaEntry // schemas fetched, per DB
	objects   []StoreObject // objects fetched, per DB.schema

	fetchedSchemas map[string]bool // UC(db) → ListSchemas succeeded
	fetchedObjects map[string]bool // UC(db)\0UC(schema) → ListObjects succeeded
}

// fetchedObjectSchemas returns the DB.schema pairs whose object lists were
// fetched, as SchemaEntry values for ValidateTablesExistRequest.FetchedObjectSchemas.
// It is derived from the fetched set (not from the returned objects) so a schema
// that legitimately has zero objects still counts as "fetched".
func (c diagCatalog) fetchedObjectSchemas() []SchemaEntry {
	out := make([]SchemaEntry, 0, len(c.fetchedObjects))
	for key := range c.fetchedObjects {
		db, schema, ok := strings.Cut(key, "\x00")
		if !ok {
			continue
		}
		out = append(out, SchemaEntry{DB: db, Name: schema})
	}
	return out
}

// schemaObjectKey is the map key for the fetchedObjects guard.
func schemaObjectKey(db, schema string) string {
	return strings.ToUpper(db) + "\x00" + strings.ToUpper(schema)
}

// gatherCatalog collects the databases, schemas, and objects needed to resolve
// and validate the given table references. It deduplicates db/schema pairs to
// minimize Snowflake API calls and always includes the session's default
// db/schema (so unqualified refs resolve against the warm session schema, as the
// frontend object store does). Individual ListSchemas/ListObjects failures are
// tolerated (the corresponding fetched* guard simply stays unset); only the
// initial ListDatabases failure aborts phase 2.
func gatherCatalog(ctx context.Context, provider SchemaProvider, refs []JoinTableRef, session *SessionContext) (diagCatalog, error) {
	cat := diagCatalog{
		fetchedSchemas: make(map[string]bool),
		fetchedObjects: make(map[string]bool),
	}

	// Collect unique db/schema pairs to query (qualified with session defaults).
	type dbSchema struct{ db, schema string }
	seen := make(map[dbSchema]bool)
	addPair := func(db, schema string) {
		if db == "" {
			db = session.Database
		}
		if schema == "" {
			schema = session.Schema
		}
		if db != "" && schema != "" {
			seen[dbSchema{strings.ToUpper(db), strings.ToUpper(schema)}] = true
		}
	}
	for _, ref := range refs {
		addPair(ref.DB, ref.Schema)
	}
	// Always warm the session's default db/schema.
	addPair("", "")

	// List every database first (populates the DB-existence guard).
	dbs, err := provider.ListDatabases(ctx)
	if err != nil {
		return diagCatalog{}, err
	}
	cat.databases = dbs

	// schemasAttempted dedups the ListSchemas call per database regardless of
	// outcome, so an unlistable/shared DB referenced by several schemas in one
	// query fails ListSchemas once, not once per schema. It is deliberately
	// separate from cat.fetchedSchemas (set only on success): fetchedSchemas is
	// the resolution guard — marking a DB "fetched" after a failed list would
	// make resolveRefsForDiagnostics drop valid refs as typos (false positive).
	schemasAttempted := make(map[string]bool)

	for ds := range seen {
		// List schemas once per database.
		if !schemasAttempted[ds.db] {
			schemasAttempted[ds.db] = true
			schemas, err := provider.ListSchemas(ctx, ds.db)
			if err == nil {
				cat.fetchedSchemas[ds.db] = true
				for _, s := range schemas {
					cat.schemas = append(cat.schemas, SchemaEntry{DB: ds.db, Name: s})
				}
			}
		}

		// List objects in the specific schema.
		objs, err := provider.ListObjects(ctx, ds.db, ds.schema)
		if err != nil {
			continue
		}
		cat.fetchedObjects[schemaObjectKey(ds.db, ds.schema)] = true
		cat.objects = append(cat.objects, objs...)
	}

	return cat, nil
}

// resolveRefsForDiagnostics resolves parsed table references into fully-qualified
// refs, dropping any it cannot confirm against the fetched catalog. It is a
// faithful Go port of the `resolved` mapping in SqlEditor.tsx runDiagnostics,
// which is the parity target for the MCP path (issue #354):
//
//   - Fully-qualified ref (db + schema): trusted ONLY after catalog verification.
//     Dropped when its DB isn't in the (non-empty) database list, or its schema
//     isn't among the fetched schemas for that DB, or its table isn't among the
//     fetched objects for that DB.schema. Dropping it lets ValidateTablesExist
//     emit the "does not exist" marker instead of silently trusting a typo.
//   - Unqualified / partial ref: resolved only against fetched TABLE/VIEW objects
//     (never blindly qualified from session context) — an unresolvable name is
//     dropped, exactly as the editor does.
//
// USE refs (Name == "") are skipped. This is one deliberate tightening over the
// editor's `resolved` mapping, which has no such guard: a 2-part `USE SCHEMA
// db.schema` reaches the qualified branch there and can push a resolved ref with
// an empty Name into colEntries/semantic analysis. We drop it so no empty-named
// ref ever flows downstream. ParseJoinTables emits USE refs with Name == "".
func resolveRefsForDiagnostics(refs []JoinTableRef, cat diagCatalog) []ResolvedRef {
	var out []ResolvedRef
	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}

		if ref.DB != "" && ref.Schema != "" {
			// Verify a fully-qualified ref against the catalog before trusting it.
			if len(cat.databases) > 0 && !containsFold(cat.databases, ref.DB) {
				continue // DB is a typo
			}
			if cat.fetchedSchemas[strings.ToUpper(ref.DB)] && !schemaKnown(cat.schemas, ref.DB, ref.Schema) {
				continue // schema is a typo
			}
			if cat.fetchedObjects[schemaObjectKey(ref.DB, ref.Schema)] && !objectKnown(cat.objects, ref.DB, ref.Schema, ref.Name) {
				continue // table is a typo
			}
			out = append(out, ResolvedRef{Alias: ref.Alias, DB: ref.DB, Schema: ref.Schema, Name: ref.Name})
			continue
		}

		// Unqualified / partial: match a fetched TABLE/VIEW object only.
		if o, ok := findTableView(cat.objects, ref); ok {
			out = append(out, ResolvedRef{Alias: ref.Alias, DB: o.DB, Schema: o.Schema, Name: o.Name})
		}
	}
	return out
}

// gatherColumns fetches column info for each unique resolved table. Per-table
// failures are skipped (best-effort), yielding an entry with no columns — matching
// the editor, whose cold column cache produces an empty entry for the same ref.
func gatherColumns(ctx context.Context, provider SchemaProvider, refs []ResolvedRef) []ColEntry {
	type tableKey struct{ db, schema, name string }
	seen := make(map[tableKey]bool)
	var entries []ColEntry

	for _, ref := range refs {
		key := tableKey{
			strings.ToUpper(ref.DB),
			strings.ToUpper(ref.Schema),
			strings.ToUpper(ref.Name),
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		cols, err := provider.TableColumns(ctx, ref.DB, ref.Schema, ref.Name)
		if err != nil {
			// A missing/undescribable table yields no columns; downstream
			// validators treat the entry as an empty column set, same as the
			// editor's cold cache.
			cols = nil
		}
		entries = append(entries, ColEntry{
			DB:     ref.DB,
			Schema: ref.Schema,
			Name:   ref.Name,
			Cols:   cols,
		})
	}
	return entries
}

// ── Catalog lookup helpers ──────────────────────────────────────────────────

// containsFold reports whether target is in names (case-insensitive).
func containsFold(names []string, target string) bool {
	for _, n := range names {
		if strings.EqualFold(n, target) {
			return true
		}
	}
	return false
}

// schemaKnown reports whether db.schema appears in the fetched schema list.
func schemaKnown(schemas []SchemaEntry, db, schema string) bool {
	for _, s := range schemas {
		if strings.EqualFold(s.DB, db) && strings.EqualFold(s.Name, schema) {
			return true
		}
	}
	return false
}

// objectKnown reports whether db.schema.name appears in the fetched object list
// (any kind — matching the editor's table-typo guard, which ignores kind).
func objectKnown(objects []StoreObject, db, schema, name string) bool {
	for _, o := range objects {
		if strings.EqualFold(o.DB, db) && strings.EqualFold(o.Schema, schema) && strings.EqualFold(o.Name, name) {
			return true
		}
	}
	return false
}

// findTableView resolves an unqualified/partial ref against the fetched objects,
// matching only TABLE/VIEW kinds by name (and by db/schema when the ref supplies
// them). Mirrors the editor's storeObjs.find in runDiagnostics.
func findTableView(objects []StoreObject, ref JoinTableRef) (StoreObject, bool) {
	for _, o := range objects {
		if !strings.EqualFold(o.Kind, "TABLE") && !strings.EqualFold(o.Kind, "VIEW") {
			continue
		}
		if !strings.EqualFold(o.Name, ref.Name) {
			continue
		}
		if ref.DB != "" && !strings.EqualFold(o.DB, ref.DB) {
			continue
		}
		if ref.Schema != "" && !strings.EqualFold(o.Schema, ref.Schema) {
			continue
		}
		return o, true
	}
	return StoreObject{}, false
}

// tableViewRefs converts fetched TABLE/VIEW objects to ResolvedRefs for
// ValidateTablesExistRequest.AllKnownTables (the quick-fix qualification source).
func tableViewRefs(objects []StoreObject) []ResolvedRef {
	var out []ResolvedRef
	for _, o := range objects {
		if !strings.EqualFold(o.Kind, "TABLE") && !strings.EqualFold(o.Kind, "VIEW") {
			continue
		}
		out = append(out, ResolvedRef{DB: o.DB, Schema: o.Schema, Name: o.Name})
	}
	return out
}

// storeObjectsToObjectRefs converts fetched objects to ObjectRefs for
// ValidateTablesExistRequest.KnownObjects (the stage / object-kind catalog).
func storeObjectsToObjectRefs(objects []StoreObject) []ObjectRef {
	out := make([]ObjectRef, len(objects))
	for i, o := range objects {
		// StoreObject and ObjectRef share identical fields, so a direct
		// conversion is exact (and avoids the staticcheck S1016 rewrite).
		out[i] = ObjectRef(o)
	}
	return out
}
