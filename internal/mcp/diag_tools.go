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
func registerDiagTools(srv *mcpsdk.Server, client *snowflake.Client) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "validate_sql",
		Description: "Run the SQL diagnostics pipeline and return structured markers (errors and warnings). These approximate the Thaw editor's diagnostics and may differ from what the editor shows for the same SQL — the editor layers an incremental, offline-first catalog warm-up on top that this tool does not. Phase 1 (syntax, patterns, datatypes) always runs. Phase 2 (schema-aware table existence, semantics, bare column refs) runs best-effort when a Snowflake connection is available.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in validateSqlInput) (*mcpsdk.CallToolResult, any, error) {
		markers := validateSQL(ctx, client, in.SQL)
		// Ensure non-nil so JSON serializes as [] not null.
		if markers == nil {
			markers = []sqleditor.DiagMarker{}
		}
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

// validateSQL runs the SQL diagnostics pipeline for the validate_sql tool. It
// delegates to the shared orchestrator sqleditor.Diagnose, which owns the phase
// ordering and table-ref resolution for both this MCP path and (in semantics)
// the frontend editor — see internal/sqleditor/orchestrator.go and #354. Phase 1
// (pure validations) always runs; phase 2 (schema-aware) runs best-effort when a
// Snowflake client is available. A non-fatal phase-2 error is logged and only the
// phase-1 markers are returned.
func validateSQL(ctx context.Context, client *snowflake.Client, sql string) []sqleditor.DiagMarker {
	var provider sqleditor.SchemaProvider
	if client != nil {
		provider = &clientSchemaProvider{client: client}
	}
	markers, err := sqleditor.Diagnose(ctx, provider, sql)
	if err != nil {
		logger.L.Debug("mcp validate_sql phase-2 skipped", "err", err)
	}
	return markers
}

// clientSchemaProvider adapts a *snowflake.Client to sqleditor.SchemaProvider so
// the orchestrator can gather catalog metadata without importing snowflake. Each
// method maps snowflake types to their sqleditor equivalents.
type clientSchemaProvider struct {
	client *snowflake.Client
}

func (p *clientSchemaProvider) SessionContext(ctx context.Context) (sqleditor.SessionContext, error) {
	sc, err := p.client.GetSessionContext(ctx)
	if err != nil {
		return sqleditor.SessionContext{}, err
	}
	return sqleditor.SessionContext{Database: sc.Database, Schema: sc.Schema}, nil
}

func (p *clientSchemaProvider) ListDatabases(ctx context.Context) ([]string, error) {
	return p.client.ListDatabases(ctx)
}

func (p *clientSchemaProvider) ListSchemas(ctx context.Context, database string) ([]string, error) {
	return p.client.ListSchemas(ctx, database)
}

func (p *clientSchemaProvider) ListObjects(ctx context.Context, database, schema string) ([]sqleditor.StoreObject, error) {
	objs, err := p.client.ListObjects(ctx, database, schema)
	if err != nil {
		return nil, err
	}
	return sfObjsToStoreObjects(database, schema, objs), nil
}

func (p *clientSchemaProvider) TableColumns(ctx context.Context, database, schema, name string) ([]sqleditor.ColInfo, error) {
	cols, err := p.client.GetTableColumnsWithTypes(ctx, database, schema, name)
	if err != nil {
		return nil, err
	}
	return sfColsToColInfo(cols), nil
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
