// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package column

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AddColumnConfig holds the parameters for an ALTER TABLE ... ADD COLUMN
// statement. Field names mirror the frontend ColumnConfig so the Wails-generated
// model maps cleanly.
type AddColumnConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	IfNotExists   bool   `json:"ifNotExists"`
	DataType      string `json:"dataType"`
	// Value mode: "none" | "default" | "autoincrement" | "computed"
	ValueMode    string `json:"valueMode"`
	DefaultValue string `json:"defaultValue"`
	ComputedExpr string `json:"computedExpr"`
	// Autoincrement / Identity
	IdentityStart int    `json:"identityStart"`
	IdentityStep  int    `json:"identityStep"`
	IdentityOrder string `json:"identityOrder"` // "ORDER" | "NOORDER" | ""
	// Inline constraint
	NotNull        bool   `json:"notNull"`
	ConstraintKind string `json:"constraintKind"` // "none" | "unique" | "primary_key" | "foreign_key"
	ConstraintName string `json:"constraintName"`
	FkDb           string `json:"fkDb"`
	FkSchema       string `json:"fkSchema"`
	FkTableName    string `json:"fkTableName"`
	FkColumn       string `json:"fkColumn"`
	// Collation & comment
	Collation string `json:"collation"`
	Comment   string `json:"comment"`
}

// tableRef returns a fully-quoted DB.SCHEMA.TABLE reference.
func tableRef(db, schema, table string) string {
	return fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(table))
}

// BuildAddColumnSql constructs an ALTER TABLE ... ADD COLUMN statement.
func BuildAddColumnSql(db, schema, table string, cfg AddColumnConfig) (string, error) {
	colName := cfg.Name
	if colName == "" {
		// Placeholder for live SQL preview before the user types a name.
		// The frontend gates submission with canSubmit (name must be non-empty).
		colName = "column_name"
	}
	colToken := snowflake.QuoteOrBare(colName, cfg.CaseSensitive)

	parts := []string{fmt.Sprintf("ALTER TABLE %s ADD COLUMN", tableRef(db, schema, table))}

	if cfg.IfNotExists {
		parts = append(parts, "IF NOT EXISTS")
	}
	parts = append(parts, colToken)

	computed := cfg.ValueMode == "computed"

	// Data type is omitted for computed columns, which derive their type from
	// the expression.
	if computed {
		if expr := strings.TrimSpace(cfg.ComputedExpr); expr != "" {
			parts = append(parts, fmt.Sprintf("AS (%s)", expr))
		}
	} else {
		parts = append(parts, cfg.DataType)
	}

	// Value: DEFAULT or AUTOINCREMENT (mutually exclusive with computed).
	switch cfg.ValueMode {
	case "default":
		if dv := strings.TrimSpace(cfg.DefaultValue); dv != "" {
			parts = append(parts, "DEFAULT "+dv)
		}
	case "autoincrement":
		parts = append(parts, fmt.Sprintf("AUTOINCREMENT (%d, %d)", cfg.IdentityStart, cfg.IdentityStep))
		if cfg.IdentityOrder != "" {
			parts = append(parts, cfg.IdentityOrder)
		}
	}

	// Inline constraints and collation are invalid for computed (virtual)
	// columns, so they are skipped in that mode.
	if !computed {
		// NOT NULL must precede a named CONSTRAINT clause, which applies to
		// UNIQUE / PRIMARY KEY / FOREIGN KEY.
		if cfg.NotNull {
			parts = append(parts, "NOT NULL")
		}
		if name := strings.TrimSpace(cfg.ConstraintName); name != "" {
			parts = append(parts, "CONSTRAINT "+snowflake.QuoteIdent(name))
		}
		switch cfg.ConstraintKind {
		case "unique":
			parts = append(parts, "UNIQUE")
		case "primary_key":
			parts = append(parts, "PRIMARY KEY")
		case "foreign_key":
			if cfg.FkTableName != "" {
				fkDb := cfg.FkDb
				if fkDb == "" {
					fkDb = db
				}
				fkSchema := cfg.FkSchema
				if fkSchema == "" {
					fkSchema = schema
				}
				ref := "REFERENCES " + tableRef(fkDb, fkSchema, cfg.FkTableName)
				if cfg.FkColumn != "" {
					ref += fmt.Sprintf(" (%s)", snowflake.QuoteIdent(cfg.FkColumn))
				}
				parts = append(parts, ref)
			}
		}
		if col := strings.TrimSpace(cfg.Collation); col != "" {
			parts = append(parts, "COLLATE "+snowflake.QuoteStringLit(col))
		}
	}

	if c := strings.TrimSpace(cfg.Comment); c != "" {
		parts = append(parts, "COMMENT "+snowflake.QuoteStringLit(c))
	}

	return strings.Join(parts, " ") + ";", nil
}

// BuildDropColumnSql constructs an ALTER TABLE ... DROP COLUMN statement.
func BuildDropColumnSql(db, schema, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column))
}

// BuildRenameColumnSql constructs an ALTER TABLE ... RENAME COLUMN statement.
func BuildRenameColumnSql(db, schema, table, oldName, newName string, caseSensitive bool) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(oldName),
		snowflake.QuoteOrBare(newName, caseSensitive))
}

// BuildSetNotNullSql constructs an ALTER TABLE ... ALTER COLUMN ... SET NOT NULL statement.
func BuildSetNotNullSql(db, schema, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column))
}

// BuildDropNotNullSql constructs an ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL statement.
func BuildDropNotNullSql(db, schema, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column))
}

// BuildSetColumnCommentSql constructs an ALTER TABLE ... ALTER COLUMN statement
// that sets the column comment, or UNSETs it when comment is empty.
func BuildSetColumnCommentSql(db, schema, table, column, comment string) string {
	c := strings.TrimSpace(comment)
	if c == "" {
		return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s UNSET COMMENT;",
			tableRef(db, schema, table), snowflake.QuoteIdent(column))
	}
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s COMMENT %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column), snowflake.QuoteStringLit(c))
}

// BuildChangeDataTypeSql constructs an ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE statement.
func BuildChangeDataTypeSql(db, schema, table, column, dataType string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DATA TYPE %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column), strings.TrimSpace(dataType))
}
