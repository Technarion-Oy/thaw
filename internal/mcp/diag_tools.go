// SPDX-License-Identifier: GPL-3.0-or-later

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
//
// cache is the session's shared metadataCache (built once in buildServer and
// passed to every diagnostics-running tool), so an AI client refining SQL in a
// tight loop — including a validate_sql → open_sql_tab handoff across tools —
// does not re-issue identical databases/schemas/objects/column/FK queries
// against the live account within the cache window (issue #355). It is nil when
// the session has no Snowflake connection.
func registerDiagTools(srv *mcpsdk.Server, cache *metadataCache) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "validate_sql",
		Description: "Run the SQL diagnostics pipeline and return structured markers (errors and warnings) plus a schemaAware flag. These approximate the Thaw editor's diagnostics and may differ from what the editor shows for the same SQL — the editor layers an incremental, offline-first catalog warm-up on top that this tool does not. Phase 1 (syntax, patterns, datatypes) always runs. Phase 2 (schema-aware table existence, semantics, bare column refs) runs best-effort when a Snowflake connection is available: schemaAware is true when it ran, and false (with schemaAwareSkippedReason set) when there was no connection or a metadata fetch failed — so an empty markers list means 'phase 2 ran and found nothing' only when schemaAware is true.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in validateSqlInput) (*mcpsdk.CallToolResult, any, error) {
		return jsonResult(validateSQL(ctx, cache, in.SQL)), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "suggest_join_conditions",
		Description: "Suggest JOIN ON/USING conditions for two tables based on foreign keys, primary keys, and type-compatible same-name columns.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in joinSuggestInput) (*mcpsdk.CallToolResult, any, error) {
		if cache == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		conditions, err := suggestJoinConditions(ctx, cache, in.TableA, in.TableB)
		if err != nil {
			return nil, nil, err
		}
		// Ensure non-nil so JSON serializes as [] not null.
		if conditions == nil {
			conditions = []sqleditor.JoinCondition{}
		}
		return jsonResult(conditions), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "format_sql",
		Description: "Apply keyword, identifier, and function casing to SQL text. Valid case values: UPPER, lower, Title. Omit or pass empty string to preserve original casing for that token type.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in formatSqlInput) (*mcpsdk.CallToolResult, any, error) {
		if err := validateCaseParam("keyword_case", in.KeywordCase); err != nil {
			return nil, nil, err
		}
		if err := validateCaseParam("identifier_case", in.IdentifierCase); err != nil {
			return nil, nil, err
		}
		if err := validateCaseParam("function_case", in.FunctionCase); err != nil {
			return nil, nil, err
		}
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

// validCaseValues is the set of accepted casing values for format_sql.
var validCaseValues = map[string]bool{
	"":      true, // preserve
	"UPPER": true,
	"lower": true,
	"Title": true,
}

// validateCaseParam rejects unknown case values with a descriptive error.
func validateCaseParam(param, value string) error {
	if !validCaseValues[value] {
		return fmt.Errorf("invalid %s value %q: must be UPPER, lower, Title, or empty", param, value)
	}
	return nil
}

// validateSqlResult is the structured payload the validate_sql tool returns. It
// wraps the marker list with a schemaAware flag so a client can distinguish
// "phase 2 ran and found nothing" (SchemaAware == true, empty Markers) from
// "phase 2 was skipped" (SchemaAware == false) — the latter carrying a reason
// (no connection, or a metadata fetch error). See issue #355: previously a
// single ListDatabases failure silently dropped all phase-2 markers with only a
// Debug log, giving the client no signal.
type validateSqlResult struct {
	// Markers is never nil, so it serializes as [] not null.
	Markers     []sqleditor.DiagMarker `json:"markers"`
	SchemaAware bool                   `json:"schemaAware"`
	// SchemaAwareSkippedReason explains why phase 2 did not run; empty when it did.
	SchemaAwareSkippedReason string `json:"schemaAwareSkippedReason,omitempty"`
}

// validateSQL runs the SQL diagnostics pipeline for the validate_sql tool. It
// delegates to the shared orchestrator sqleditor.Diagnose, which owns the phase
// ordering and table-ref resolution for both this MCP path and (in semantics)
// the frontend editor — see internal/sqleditor/orchestrator.go and #354. Phase 1
// (pure validations) always runs; phase 2 (schema-aware) runs best-effort when a
// metadata cache (backed by a Snowflake client) is available. A non-fatal
// phase-2 error is logged, only the phase-1 markers are returned, and the result
// reports schemaAware=false with the reason so the caller isn't left guessing.
func validateSQL(ctx context.Context, cache *metadataCache, sql string) validateSqlResult {
	var provider sqleditor.SchemaProvider
	if cache != nil {
		provider = &clientSchemaProvider{cache: cache}
	}
	markers, err := sqleditor.Diagnose(ctx, provider, sql)
	if markers == nil {
		markers = []sqleditor.DiagMarker{}
	}
	res := validateSqlResult{Markers: markers}
	switch {
	case provider == nil:
		res.SchemaAwareSkippedReason = "no Snowflake connection available"
	case err != nil:
		logger.L.Debug("mcp validate_sql phase-2 skipped", "err", err)
		res.SchemaAwareSkippedReason = err.Error()
	default:
		res.SchemaAware = true
	}
	return res
}

// clientSchemaProvider adapts a metadataCache to sqleditor.SchemaProvider so the
// orchestrator can gather catalog metadata without importing snowflake. Each
// method maps snowflake types to their sqleditor equivalents; reads route
// through the cache so repeated diagnostics runs reuse metadata (issue #355).
type clientSchemaProvider struct {
	cache *metadataCache
}

func (p *clientSchemaProvider) SessionContext(ctx context.Context) (sqleditor.SessionContext, error) {
	sc, err := p.cache.SessionContext(ctx)
	if err != nil {
		return sqleditor.SessionContext{}, err
	}
	return sqleditor.SessionContext{Database: sc.Database, Schema: sc.Schema}, nil
}

func (p *clientSchemaProvider) ListDatabases(ctx context.Context) ([]string, error) {
	return p.cache.ListDatabases(ctx)
}

func (p *clientSchemaProvider) ListSchemas(ctx context.Context, database string) ([]string, error) {
	return p.cache.ListSchemas(ctx, database)
}

func (p *clientSchemaProvider) ListObjects(ctx context.Context, database, schema string) ([]sqleditor.StoreObject, error) {
	objs, err := p.cache.ListObjects(ctx, database, schema)
	if err != nil {
		return nil, err
	}
	return sfObjsToStoreObjects(database, schema, objs), nil
}

func (p *clientSchemaProvider) TableColumns(ctx context.Context, database, schema, name string) ([]sqleditor.ColInfo, error) {
	cols, err := p.cache.GetTableColumnsWithTypes(ctx, database, schema, name)
	if err != nil {
		return nil, err
	}
	return sfColsToColInfo(cols), nil
}

// suggestJoinConditions computes JOIN ON suggestions for two tables.
// Self-joins (table_a == table_b) are supported — column entries are built
// per-ref without deduplication so ComputeJoinOnConditions sees both sides.
// Column and FK reads route through the shared metadataCache, so a suggestion
// following a validate_sql call for the same tables reuses their metadata
// (issue #355).
func suggestJoinConditions(ctx context.Context, cache *metadataCache, tableA, tableB string) ([]sqleditor.JoinCondition, error) {
	sc, err := cache.SessionContext(ctx)
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
		cols, err := cache.GetTableColumnsWithTypes(ctx, ref.DB, ref.Schema, ref.Name)
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
		fks, err := cache.GetTableForeignKeys(ctx, ref.DB, ref.Schema, ref.Name)
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
