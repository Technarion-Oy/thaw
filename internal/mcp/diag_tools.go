// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/logger"
	"thaw/internal/snowflake"
	"thaw/internal/sqleditor"
)

// Tool input types for the diagnostics tools.

type validateSqlInput struct {
	SQL string `json:"sql" jsonschema:"the SQL text to validate"`
}

type joinSuggestInput struct {
	TableA string `json:"table_a" jsonschema:"first table name (optionally qualified as db.schema.table)"`
	TableB string `json:"table_b" jsonschema:"second table name (optionally qualified as db.schema.table)"`
}

type formatSqlInput struct {
	SQL            string `json:"sql" jsonschema:"the SQL text to format"`
	KeywordCase    string `json:"keyword_case,omitempty" jsonschema:"case for keywords: UPPER, lower, Title, or empty to preserve"`
	IdentifierCase string `json:"identifier_case,omitempty" jsonschema:"case for identifiers: UPPER, lower, Title, or empty to preserve"`
	FunctionCase   string `json:"function_case,omitempty" jsonschema:"case for functions: UPPER, lower, Title, or empty to preserve"`
}

// registerDiagTools wires the SQL diagnostics and validation tools onto srv.
// These tools expose Thaw's sqleditor engine to external AI clients for
// iterative SQL refinement against real schema before delivering SQL to a
// tab or notebook.
func registerDiagTools(srv *mcpsdk.Server, client *snowflake.Client) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "validate_sql",
		Description: "Run the full SQL diagnostics pipeline and return structured markers (errors and warnings) matching the editor. Phase 1 (syntax, patterns, datatypes) always runs. Phase 2 (schema-aware table existence, semantics, bare column refs) runs best-effort when a Snowflake connection is available.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in validateSqlInput) (*mcpsdk.CallToolResult, any, error) {
		markers := validateSQL(ctx, client, in.SQL)
		return jsonResult(markers), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "suggest_join_conditions",
		Description: "Suggest JOIN ON/USING conditions for two tables based on foreign keys, primary keys, and type-compatible same-name columns.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in joinSuggestInput) (*mcpsdk.CallToolResult, any, error) {
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		conditions, err := suggestJoinConditions(ctx, client, in.TableA, in.TableB)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(conditions), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "format_sql",
		Description: "Apply keyword, identifier, and function casing to SQL text. Valid case values: UPPER, lower, Title. Omit or pass empty string to preserve original casing for that token type.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in formatSqlInput) (*mcpsdk.CallToolResult, any, error) {
		formatted := sqleditor.ApplyCasing(in.SQL, in.KeywordCase, in.IdentifierCase, in.FunctionCase)
		return textResult(formatted), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_snowflake_keywords",
		Description: "Return the list of Snowflake reserved keywords.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		return jsonResult(snowflake.ReservedKeywords()), nil, nil
	})
}

// validateSQL orchestrates the full diagnostics pipeline. Phase 1 (pure
// validations) always runs. Phase 2 (schema-aware) runs best-effort when a
// Snowflake client is available; if any phase-2 step fails, only phase-1
// markers are returned.
//
// NOTE: This pipeline mirrors the frontend orchestration in
// frontend/src/components/editor/SqlEditor.tsx (runDiagnostics, ~L601).
// Changes to the validation ordering or request assembly should be reflected
// in both locations until a shared orchestrator is extracted (see #336).
func validateSQL(ctx context.Context, client *snowflake.Client, sql string) []sqleditor.DiagMarker {
	// Phase 1 — pure validations (no network).
	syntaxMarkers := sqleditor.ValidateSyntax(sql)
	stmtRanges := sqleditor.GetStatementRanges(sql)
	patternMarkers := sqleditor.ValidateSnowflakePatterns(sql, stmtRanges)
	datatypeMarkers := sqleditor.ValidateDataTypes(sql, stmtRanges)

	phase1 := append(append(syntaxMarkers, patternMarkers...), datatypeMarkers...)

	// Phase 2 — schema-aware validations (needs Snowflake client).
	if client == nil {
		return phase1
	}

	phase2, err := validateSQLSchemaAware(ctx, client, sql, stmtRanges)
	if err != nil {
		logger.L.Debug("mcp validate_sql phase-2 skipped", "err", err)
		return phase1
	}

	return append(phase1, phase2...)
}

// validateSQLSchemaAware runs the schema-aware portion of the diagnostics
// pipeline. It returns an error if any step fails, allowing the caller to
// fall back to phase-1 markers only.
func validateSQLSchemaAware(ctx context.Context, client *snowflake.Client, sql string, stmtRanges []sqleditor.StatementRange) ([]sqleditor.DiagMarker, error) {
	sc, err := client.GetSessionContext(ctx)
	if err != nil {
		return nil, err
	}
	sessionCtx := &sqleditor.SessionContext{
		Database: sc.Database,
		Schema:   sc.Schema,
	}

	refs := sqleditor.ParseJoinTables(sql)

	// Short-circuit: if the SQL references no tables, skip metadata gathering.
	if len(refs) == 0 {
		return nil, nil
	}

	// Gather metadata for referenced databases/schemas.
	storeObjects, knownDatabases, knownSchemas, err := gatherMetadata(ctx, client, refs, sessionCtx)
	if err != nil {
		return nil, err
	}

	resolvedRefs := sqleditor.ResolveTableRefs(refs, storeObjects, nil, sessionCtx)

	// Validate table existence.
	tableExistMarkers := sqleditor.ValidateTablesExist(sqleditor.ValidateTablesExistRequest{
		SQL:            sql,
		StmtRanges:     stmtRanges,
		ResolvedRefs:   resolvedRefs,
		KnownDatabases: knownDatabases,
		KnownSchemas:   knownSchemas,
		AllKnownTables: storeObjsToResolvedRefs(storeObjects),
	})

	// Gather column info for resolved tables.
	colEntries := fetchColumnEntries(ctx, client, resolvedRefs)

	semanticMarkers := sqleditor.ValidateSemantics(sql, resolvedRefs, colEntries)

	bareColMarkers := sqleditor.ValidateBareColumnRefs(sqleditor.ValidateBareColsRequest{
		SQL:          sql,
		StmtRanges:   stmtRanges,
		ResolvedRefs: resolvedRefs,
		ColEntries:   colEntries,
	})

	return append(append(tableExistMarkers, semanticMarkers...), bareColMarkers...), nil
}

// gatherMetadata collects store objects, known databases, and known schemas
// for the set of table references. It deduplicates db/schema pairs to
// minimize Snowflake API calls.
func gatherMetadata(ctx context.Context, client *snowflake.Client, refs []sqleditor.JoinTableRef, session *sqleditor.SessionContext) ([]sqleditor.StoreObject, []string, []sqleditor.SchemaEntry, error) {
	// Collect unique db/schema pairs to query.
	type dbSchema struct{ db, schema string }
	seen := make(map[dbSchema]bool)

	for _, ref := range refs {
		db := ref.DB
		schema := ref.Schema
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

	// Always include the session's default db/schema.
	if session.Database != "" && session.Schema != "" {
		seen[dbSchema{strings.ToUpper(session.Database), strings.ToUpper(session.Schema)}] = true
	}

	var storeObjects []sqleditor.StoreObject
	dbSet := make(map[string]bool)
	var knownSchemas []sqleditor.SchemaEntry

	// List databases first to populate knownDatabases.
	dbs, err := client.ListDatabases(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, db := range dbs {
		dbSet[strings.ToUpper(db)] = true
	}

	// Dedupe ListSchemas calls by database (multiple schemas in the same db
	// should not re-list the db's schemas).
	schemasListed := make(map[string]bool)

	for ds := range seen {
		// List schemas once per database.
		if !schemasListed[ds.db] {
			schemasListed[ds.db] = true
			schemas, err := client.ListSchemas(ctx, ds.db)
			if err != nil {
				logger.L.Debug("mcp validate_sql: ListSchemas skipped", "db", ds.db, "err", err)
			} else {
				for _, s := range schemas {
					knownSchemas = append(knownSchemas, sqleditor.SchemaEntry{
						DB:   ds.db,
						Name: s,
					})
				}
			}
		}

		// List objects in the specific schema.
		objs, err := client.ListObjects(ctx, ds.db, ds.schema)
		if err != nil {
			logger.L.Debug("mcp validate_sql: ListObjects skipped", "db", ds.db, "schema", ds.schema, "err", err)
			continue
		}
		storeObjects = append(storeObjects, sfObjsToStoreObjects(ds.db, ds.schema, objs)...)
	}

	knownDatabases := make([]string, 0, len(dbSet))
	for db := range dbSet {
		knownDatabases = append(knownDatabases, db)
	}

	return storeObjects, knownDatabases, knownSchemas, nil
}

// fetchColumnEntries fetches column info for each unique resolved table.
// Errors for individual tables are logged and skipped (best-effort).
func fetchColumnEntries(ctx context.Context, client *snowflake.Client, refs []sqleditor.ResolvedRef) []sqleditor.ColEntry {
	type tableKey struct{ db, schema, name string }
	seen := make(map[tableKey]bool)
	var entries []sqleditor.ColEntry

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

		cols, err := client.GetTableColumnsWithTypes(ctx, ref.DB, ref.Schema, ref.Name)
		if err != nil {
			logger.L.Debug("mcp: GetTableColumnsWithTypes skipped", "table", ref.DB+"."+ref.Schema+"."+ref.Name, "err", err)
			continue
		}
		entries = append(entries, sqleditor.ColEntry{
			DB:     ref.DB,
			Schema: ref.Schema,
			Name:   ref.Name,
			Cols:   sfColsToColInfo(cols),
		})
	}

	return entries
}

// suggestJoinConditions computes JOIN ON suggestions for two tables.
// Self-joins (table_a == table_b) are supported — column entries are built
// per-ref without deduplication so ComputeJoinOnConditions sees both sides.
func suggestJoinConditions(ctx context.Context, client *snowflake.Client, tableA, tableB string) ([]sqleditor.JoinCondition, error) {
	sc, err := client.GetSessionContext(ctx)
	if err != nil {
		return nil, err
	}

	refA := qualifyTableRef(tableA, sc.Database, sc.Schema)
	refB := qualifyTableRef(tableB, sc.Database, sc.Schema)

	resolvedRefs := []sqleditor.ResolvedRef{
		{DB: refA.db, Schema: refA.schema, Name: refA.table},
		{DB: refB.db, Schema: refB.schema, Name: refB.table},
	}

	// Build column entries per-ref (no deduplication — self-joins need both).
	var colEntries []sqleditor.ColEntry
	for _, ref := range resolvedRefs {
		cols, err := client.GetTableColumnsWithTypes(ctx, ref.DB, ref.Schema, ref.Name)
		if err != nil {
			return nil, fmt.Errorf("describe %s.%s.%s: %w", ref.DB, ref.Schema, ref.Name, err)
		}
		colEntries = append(colEntries, sqleditor.ColEntry{
			DB:     ref.DB,
			Schema: ref.Schema,
			Name:   ref.Name,
			Cols:   sfColsToColInfo(cols),
		})
	}

	// Gather foreign key info.
	var fkEntries []sqleditor.TableFKEntry
	for _, ref := range resolvedRefs {
		fks, err := client.GetTableForeignKeys(ctx, ref.DB, ref.Schema, ref.Name)
		if err != nil {
			// FKs are best-effort — the suggestion engine can still use
			// type-compatible same-name columns without FK data.
			logger.L.Debug("mcp suggest_join_conditions: GetTableForeignKeys skipped", "table", ref.DB+"."+ref.Schema+"."+ref.Name, "err", err)
			fkEntries = append(fkEntries, sqleditor.TableFKEntry{
				DB: ref.DB, Schema: ref.Schema, Name: ref.Name,
			})
			continue
		}
		fkEntries = append(fkEntries, sqleditor.TableFKEntry{
			DB:     ref.DB,
			Schema: ref.Schema,
			Name:   ref.Name,
			FKs:    sfFKsToFKEntries(fks),
		})
	}

	conditions := sqleditor.ComputeJoinOnConditions(sqleditor.JoinOnSuggestionsReq{
		ResolvedRefs: resolvedRefs,
		FKEntries:    fkEntries,
		ColEntries:   colEntries,
	})
	return conditions, nil
}

// --- Type conversion helpers ---

// sfObjsToStoreObjects maps snowflake.SnowflakeObject to sqleditor.StoreObject.
func sfObjsToStoreObjects(db, schema string, objs []snowflake.SnowflakeObject) []sqleditor.StoreObject {
	out := make([]sqleditor.StoreObject, len(objs))
	for i, o := range objs {
		out[i] = sqleditor.StoreObject{
			DB:     db,
			Schema: schema,
			Name:   o.Name,
			Kind:   o.Kind,
		}
	}
	return out
}

// sfColsToColInfo maps snowflake.ColumnInfo to sqleditor.ColInfo.
func sfColsToColInfo(cols []snowflake.ColumnInfo) []sqleditor.ColInfo {
	out := make([]sqleditor.ColInfo, len(cols))
	for i, c := range cols {
		out[i] = sqleditor.ColInfo{
			Name:     c.Name,
			DataType: c.DataType,
		}
	}
	return out
}

// sfFKsToFKEntries maps snowflake.TableForeignKey to sqleditor.FKEntry.
func sfFKsToFKEntries(fks []snowflake.TableForeignKey) []sqleditor.FKEntry {
	out := make([]sqleditor.FKEntry, len(fks))
	for i, fk := range fks {
		out[i] = sqleditor.FKEntry{
			PKDatabase:     fk.PKDatabase,
			PKSchema:       fk.PKSchema,
			PKTable:        fk.PKTable,
			PKColumn:       fk.PKColumn,
			FKColumn:       fk.FKColumn,
			ConstraintName: fk.ConstraintName,
			KeySequence:    fk.KeySequence,
		}
	}
	return out
}

// storeObjsToResolvedRefs converts store objects to resolved refs for the
// AllKnownTables field in ValidateTablesExistRequest.
func storeObjsToResolvedRefs(objs []sqleditor.StoreObject) []sqleditor.ResolvedRef {
	out := make([]sqleditor.ResolvedRef, len(objs))
	for i, o := range objs {
		out[i] = sqleditor.ResolvedRef{
			DB:     o.DB,
			Schema: o.Schema,
			Name:   o.Name,
		}
	}
	return out
}

// qualifiedParts holds parsed dot-separated table name parts.
type qualifiedParts struct {
	db, schema, table string
}

// parseTableParts splits a qualified name into db, schema, table, respecting
// double-quoted identifiers that may contain dots (e.g. `db."my.schema".table`).
// Unquoted identifiers are uppercased to match Snowflake's canonical casing;
// quoted identifiers preserve their original case.
func parseTableParts(name string) qualifiedParts {
	parts, quoted := splitQualifiedNameWithQuoteInfo(name)
	// Snowflake folds unquoted identifiers to uppercase.
	for i, p := range parts {
		if !quoted[i] {
			parts[i] = strings.ToUpper(p)
		}
	}
	switch len(parts) {
	case 3:
		return qualifiedParts{db: parts[0], schema: parts[1], table: parts[2]}
	case 2:
		return qualifiedParts{schema: parts[0], table: parts[1]}
	default:
		return qualifiedParts{table: parts[0]}
	}
}

// splitQualifiedNameWithQuoteInfo splits a potentially quoted identifier chain
// on dots, respecting double-quoted segments. Quotes are stripped from the
// returned parts. The second return value indicates whether each part was quoted.
func splitQualifiedNameWithQuoteInfo(name string) ([]string, []bool) {
	var parts []string
	var quoted []bool
	var current strings.Builder
	inQuotes := false
	partWasQuoted := false

	for i := 0; i < len(name); i++ {
		ch := name[i]
		switch {
		case ch == '"':
			if inQuotes && i+1 < len(name) && name[i+1] == '"' {
				// Escaped quote inside quoted identifier ("").
				current.WriteByte('"')
				i++
			} else {
				inQuotes = !inQuotes
				if inQuotes {
					partWasQuoted = true
				}
			}
		case ch == '.' && !inQuotes:
			parts = append(parts, current.String())
			quoted = append(quoted, partWasQuoted)
			current.Reset()
			partWasQuoted = false
		default:
			current.WriteByte(ch)
		}
	}
	parts = append(parts, current.String())
	quoted = append(quoted, partWasQuoted)
	return parts, quoted
}

// qualifyTableRef parses a table name and fills in missing db/schema from
// the session defaults.
func qualifyTableRef(name, defaultDB, defaultSchema string) qualifiedParts {
	p := parseTableParts(name)
	if p.db == "" {
		p.db = defaultDB
	}
	if p.schema == "" {
		p.schema = defaultSchema
	}
	return p
}
