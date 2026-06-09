// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package erdesigner

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// fqn returns a fully-qualified, properly-quoted table name:
// "DATABASE"."SCHEMA"."TABLE"
func fqn(database, schema, table string) string {
	return snowflake.QuoteIdent(database) + "." +
		snowflake.QuoteIdent(schema) + "." +
		snowflake.QuoteIdent(table)
}

// BuildJoinSQL generates a formatted SELECT ... JOIN ... SQL string from a
// JoinQueryState. Assigns short aliases (t1, t2, ...) per table, uses
// fully-qualified quoted DATABASE.SCHEMA.TABLE names, and respects
// selectedColumns (defaults to * when empty).
func BuildJoinSQL(state JoinQueryState) string {
	// Assign aliases: t1 for base, t2, t3, ... for joins.
	aliasMap := make(map[string]string)
	aliasMap[snowflake.TableKey(state.BaseTable.Schema, state.BaseTable.Name)] = "t1"
	for i, j := range state.Joins {
		aliasMap[snowflake.TableKey(j.Table.Schema, j.Table.Name)] = fmt.Sprintf("t%d", i+2)
	}

	// Build SELECT columns.
	var selectParts []string
	addColumnsForTable := func(schema, name string) {
		key := snowflake.TableKey(schema, name)
		alias := aliasMap[key]
		cols := state.SelectedColumns[key]
		if len(cols) == 0 {
			selectParts = append(selectParts, fmt.Sprintf("  %s.*", alias))
		} else {
			for _, col := range cols {
				selectParts = append(selectParts, fmt.Sprintf("  %s.%s", alias, snowflake.QuoteIdent(col)))
			}
		}
	}

	addColumnsForTable(state.BaseTable.Schema, state.BaseTable.Name)
	for _, j := range state.Joins {
		addColumnsForTable(j.Table.Schema, j.Table.Name)
	}

	// Build FROM clause.
	fromLine := fmt.Sprintf("FROM %s t1",
		fqn(state.Database, state.BaseTable.Schema, state.BaseTable.Name))

	// Build JOIN clauses with aliased ON conditions.
	var joinLines []string
	for _, j := range state.Joins {
		key := snowflake.TableKey(j.Table.Schema, j.Table.Name)
		alias := aliasMap[key]

		joinKw := j.JoinType + " JOIN"

		// Build aliased ON condition from structured FK pairs when available,
		// which also ensures column identifiers are properly quoted.
		var aliasedCondition string
		if len(j.FKPairs) > 0 {
			var parts []string
			for _, pair := range j.FKPairs {
				fromAlias := aliasMap[snowflake.TableKey(pair.From.Schema, pair.From.Table)]
				toAlias := aliasMap[snowflake.TableKey(pair.To.Schema, pair.To.Table)]
				parts = append(parts, fmt.Sprintf("%s.%s = %s.%s",
					fromAlias, snowflake.QuoteIdent(pair.From.Col),
					toAlias, snowflake.QuoteIdent(pair.To.Col)))
			}
			aliasedCondition = strings.Join(parts, " AND ")
		} else {
			// Fallback for states without structured FKPairs (e.g. manually
			// constructed test data). Column names are intentionally not quoted
			// here; BuildJoinState always populates FKPairs so production use
			// takes the quoting path above.
			aliasedCondition = j.OnCondition
			for tblKey, tblAlias := range aliasMap {
				schema, table, ok := strings.Cut(tblKey, ".")
				if !ok {
					continue
				}
				aliasedCondition = strings.ReplaceAll(aliasedCondition,
					schema+"."+table+".", tblAlias+".")
			}
		}

		joinLines = append(joinLines, fmt.Sprintf("%s %s %s ON %s",
			joinKw, fqn(state.Database, j.Table.Schema, j.Table.Name), alias, aliasedCondition))
	}

	// Assemble — include a default LIMIT to prevent accidental full-table
	// scans on large Snowflake tables.
	lines := []string{
		"SELECT",
		strings.Join(selectParts, ",\n"),
		fromLine,
	}
	lines = append(lines, joinLines...)
	lines = append(lines, "LIMIT 1000;")

	return strings.Join(lines, "\n")
}
