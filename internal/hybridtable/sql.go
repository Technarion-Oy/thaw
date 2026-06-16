// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package hybridtable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// HybridColumn is one column of a hybrid table: <name> <type> with the optional
// NOT NULL / DEFAULT modifiers and a PrimaryKey flag. Columns flagged PrimaryKey
// are collected into a single out-of-line PRIMARY KEY (...) clause (which also
// covers the composite-key case) rather than being emitted inline, so the
// builder produces one consistent PK form regardless of how many columns make
// up the key.
type HybridColumn struct {
	Name       string `json:"name"`       // column identifier
	Type       string `json:"type"`       // Snowflake data type, emitted verbatim
	NotNull    bool   `json:"notNull"`    // NOT NULL
	PrimaryKey bool   `json:"primaryKey"` // part of the table's PRIMARY KEY
	Default    string `json:"default"`    // DEFAULT <expr> (emitted verbatim) or ""
}

// HybridIndex is a secondary index defined inline in the CREATE statement
// (INDEX <name> (<cols>) [INCLUDE (<cols>)]) or created afterwards via
// BuildCreateIndexSql.
type HybridIndex struct {
	Name    string   `json:"name"`    // index identifier
	Columns []string `json:"columns"` // indexed columns (key columns)
	Include []string `json:"include"` // optional non-key INCLUDE columns
}

// HybridTableConfig holds the parameters for creating a Snowflake HYBRID TABLE.
// A hybrid table must declare a PRIMARY KEY, so the builder always emits a
// PRIMARY KEY clause derived from the columns flagged PrimaryKey (a placeholder
// is emitted when none are flagged). Hybrid tables do NOT support OR REPLACE
// (only IF NOT EXISTS), TRANSIENT, CLUSTER BY, DATA_RETENTION_TIME_IN_DAYS,
// CHANGE_TRACKING, or COPY GRANTS — none of those are modeled here.
type HybridTableConfig struct {
	Name          string         `json:"name"`
	CaseSensitive bool           `json:"caseSensitive"`
	IfNotExists   bool           `json:"ifNotExists"`
	Columns       []HybridColumn `json:"columns"`
	Indexes       []HybridIndex  `json:"indexes"`
	Comment       string         `json:"comment"`
}

// BuildCreateHybridTableSql constructs a CREATE HYBRID TABLE statement from the
// given config. Columns flagged PrimaryKey are gathered into a single out-of-line
// PRIMARY KEY (...) clause; when no column is flagged a "<column>" placeholder is
// emitted so the live preview reads as a template (a hybrid table cannot be
// created without a primary key). Secondary indexes are emitted inline as
// INDEX <name> (<cols>) [INCLUDE (<cols>)].
func BuildCreateHybridTableSql(db, schema string, cfg HybridTableConfig) (string, error) {
	var sb strings.Builder

	// Hybrid tables do not support OR REPLACE — only IF NOT EXISTS.
	createClause := "CREATE HYBRID TABLE"
	if cfg.IfNotExists {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "hybrid_table_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s (\n", createClause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	// Body lines: columns, then the PRIMARY KEY clause, then any inline indexes.
	var lines []string
	var pkCols []string
	for _, c := range cfg.Columns {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		typ := strings.TrimSpace(c.Type)
		if typ == "" {
			typ = "VARCHAR"
		}
		colTok := snowflake.QuoteOrBare(c.Name, cfg.CaseSensitive)
		line := fmt.Sprintf("  %s %s", colTok, typ)
		// Snowflake requires every primary-key column of a hybrid table to be
		// NOT NULL, so force it for PK columns regardless of the NotNull flag.
		if c.NotNull || c.PrimaryKey {
			line += " NOT NULL"
		}
		if d := strings.TrimSpace(c.Default); d != "" {
			line += " DEFAULT " + d
		}
		lines = append(lines, line)
		if c.PrimaryKey {
			pkCols = append(pkCols, colTok)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "  <column> <type>")
	}

	// A hybrid table requires a PRIMARY KEY; emit a placeholder when none of the
	// columns were flagged so the preview is a valid template.
	if len(pkCols) == 0 {
		lines = append(lines, "  PRIMARY KEY (<column>)")
	} else {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	for _, idx := range cfg.Indexes {
		if strings.TrimSpace(idx.Name) == "" {
			continue
		}
		cols := cleanIdentList(idx.Columns, cfg.CaseSensitive)
		if len(cols) == 0 {
			continue
		}
		line := fmt.Sprintf("  INDEX %s (%s)",
			snowflake.QuoteOrBare(idx.Name, cfg.CaseSensitive), strings.Join(cols, ", "))
		if inc := cleanIdentList(idx.Include, cfg.CaseSensitive); len(inc) > 0 {
			line += fmt.Sprintf(" INCLUDE (%s)", strings.Join(inc, ", "))
		}
		lines = append(lines, line)
	}

	sb.WriteString(strings.Join(lines, ",\n"))
	sb.WriteString("\n)")

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}

// BuildCreateIndexSql constructs a CREATE INDEX statement adding a secondary
// index to an existing hybrid table:
//
//	CREATE INDEX <name> ON <db>.<schema>.<table> (<cols>) [INCLUDE (<cols>)]
//
// The index name and every column are always double-quoted: this builder runs
// after the table exists, so the columns come from the catalog already in their
// canonical case (e.g. via GetTableColumnsWithTypes). Emitting a mixed-case name
// bare would let Snowflake upper-case it and fail with "invalid identifier", and
// double-quoting an all-upper name still resolves correctly.
func BuildCreateIndexSql(db, schema, table string, idx HybridIndex) (string, error) {
	if strings.TrimSpace(idx.Name) == "" {
		return "", fmt.Errorf("index name is required")
	}
	cols := cleanIdentList(idx.Columns, true)
	if len(cols) == 0 {
		return "", fmt.Errorf("at least one indexed column is required")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "CREATE INDEX %s ON %s.%s.%s (%s)",
		snowflake.QuoteIdent(idx.Name),
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(table),
		strings.Join(cols, ", "))
	if inc := cleanIdentList(idx.Include, true); len(inc) > 0 {
		fmt.Fprintf(&sb, " INCLUDE (%s)", strings.Join(inc, ", "))
	}
	return sb.String() + ";", nil
}

// BuildDropIndexSql constructs a DROP INDEX statement removing a secondary index
// from a hybrid table. Snowflake addresses the index by dot-qualified name,
// table first: DROP INDEX [IF EXISTS] <db>.<schema>.<table>.<index>.
func BuildDropIndexSql(db, schema, table, index string) (string, error) {
	if strings.TrimSpace(index) == "" {
		return "", fmt.Errorf("index name is required")
	}
	return fmt.Sprintf("DROP INDEX IF EXISTS %s.%s.%s.%s;",
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema),
		snowflake.QuoteIdent(table), snowflake.QuoteIdent(index)), nil
}

// cleanIdentList trims, drops empty entries, and quotes each identifier in the
// list (bare when simple, double-quoted otherwise per caseSensitive).
func cleanIdentList(names []string, caseSensitive bool) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		out = append(out, snowflake.QuoteOrBare(n, caseSensitive))
	}
	return out
}
