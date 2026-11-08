// SPDX-License-Identifier: GPL-3.0-or-later

package column

import (
	"fmt"
	"regexp"
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
	ConstraintKind string `json:"constraintKind"` // "none" | "unique" | "primary_key" | "foreign_key" | "check"
	ConstraintName string `json:"constraintName"`
	FkDb           string `json:"fkDb"`
	FkSchema       string `json:"fkSchema"`
	FkTableName    string `json:"fkTableName"`
	FkColumn       string `json:"fkColumn"`
	CheckExpr      string `json:"checkExpr"` // CHECK (<expr>), emitted verbatim
	// Trailing constraint properties, e.g. "NOT ENFORCED RELY"; validated
	// against constraintPropsRe and emitted only when a constraint is emitted.
	ConstraintProps string `json:"constraintProps"`
	// Collation & comment
	Collation string `json:"collation"`
	Comment   string `json:"comment"`
	// Governance: [WITH] MASKING POLICY p [USING (col, cond…)], [WITH]
	// PROJECTION POLICY p, [WITH] TAG (…). Policy names are typed references
	// (bare or "quoted", up to db.schema.name); MaskingUsing is a comma list.
	MaskingPolicy    string              `json:"maskingPolicy"`
	MaskingUsing     string              `json:"maskingUsing"`
	ProjectionPolicy string              `json:"projectionPolicy"`
	Tags             []snowflake.TagPair `json:"tags"`
}

// constraintPropsRe mirrors CONSTRAINT_PROPS in
// frontend/src/components/common/tableClauses.ts; it is matched against the
// uppercased, whitespace-normalised value prefixed with a single space.
var constraintPropsRe = regexp.MustCompile(`^(?: (?:NOT )?(?:ENFORCED|DEFERRABLE)| INITIALLY (?:DEFERRED|IMMEDIATE)| (?:ENABLE|DISABLE|VALIDATE|NOVALIDATE|RELY|NORELY))*$`)

// tableRef returns a fully-quoted DB.SCHEMA.TABLE reference.
func tableRef(db, schema, table string) string {
	return fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(table))
}

// BuildAddColumnSql constructs an ALTER TABLE ... ADD COLUMN statement.
//
// It returns an error when a typed policy name or USING column is not a valid
// identifier, when a CHECK expression contains a semicolon, or when the
// constraint properties are not recognized keywords.
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
		// COLLATE belongs adjacent to the data type per Snowflake's
		// column-definition grammar (and GET_DDL output): it must precede
		// DEFAULT/AUTOINCREMENT and the inline constraints. Collation is
		// invalid on computed (virtual) columns, so it lives in this branch.
		if col := strings.TrimSpace(cfg.Collation); col != "" {
			parts = append(parts, "COLLATE "+snowflake.QuoteStringLit(col))
		}
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

	// Inline constraints are invalid for computed (virtual) columns, so they
	// are skipped in that mode.
	if !computed {
		// NOT NULL must precede a named CONSTRAINT clause, which applies to
		// UNIQUE / PRIMARY KEY / FOREIGN KEY.
		if cfg.NotNull {
			parts = append(parts, "NOT NULL")
		}
		if name := strings.TrimSpace(cfg.ConstraintName); name != "" {
			parts = append(parts, "CONSTRAINT "+snowflake.QuoteIdent(name))
		}
		con := ""
		switch cfg.ConstraintKind {
		case "unique":
			con = "UNIQUE"
		case "primary_key":
			con = "PRIMARY KEY"
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
				con = "REFERENCES " + tableRef(fkDb, fkSchema, cfg.FkTableName)
				if cfg.FkColumn != "" {
					con += fmt.Sprintf(" (%s)", snowflake.QuoteIdent(cfg.FkColumn))
				}
			}
		case "check":
			if expr := strings.TrimSpace(cfg.CheckExpr); expr != "" {
				if strings.Contains(expr, ";") {
					return "", fmt.Errorf("CHECK expression must not contain a semicolon")
				}
				con = "CHECK (" + expr + ")"
			}
		}
		if con != "" {
			parts = append(parts, con)
			// Constraint properties: validated as a trust boundary (the string
			// is emitted verbatim after normalisation).
			if props := strings.ToUpper(strings.Join(strings.Fields(cfg.ConstraintProps), " ")); props != "" {
				if !constraintPropsRe.MatchString(" " + props) {
					return "", fmt.Errorf("unrecognized constraint properties: %s", props)
				}
				parts = append(parts, props)
			}
		}
	}

	if p := strings.TrimSpace(cfg.MaskingPolicy); p != "" {
		ref, err := snowflake.RenderQualifiedName("masking policy", p, 3)
		if err != nil {
			return "", err
		}
		clause := "WITH MASKING POLICY " + ref
		if u := strings.TrimSpace(cfg.MaskingUsing); u != "" {
			var cols []string
			for _, c := range strings.Split(u, ",") {
				rc, err := snowflake.RenderQualifiedName("USING column", c, 1)
				if err != nil {
					return "", err
				}
				cols = append(cols, rc)
			}
			clause += " USING (" + strings.Join(cols, ", ") + ")"
		}
		parts = append(parts, clause)
	}
	if p := strings.TrimSpace(cfg.ProjectionPolicy); p != "" {
		ref, err := snowflake.RenderQualifiedName("projection policy", p, 3)
		if err != nil {
			return "", err
		}
		parts = append(parts, "WITH PROJECTION POLICY "+ref)
	}
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		parts = append(parts, "WITH "+tc)
	}

	if c := strings.TrimSpace(cfg.Comment); c != "" {
		// QuoteTextLit (not QuoteStringLit) so backslashes in a free-text comment
		// are doubled and stored literally rather than read as escape sequences.
		parts = append(parts, "COMMENT "+snowflake.QuoteTextLit(c))
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
		tableRef(db, schema, table), snowflake.QuoteIdent(column), snowflake.QuoteTextLit(c))
}

// BuildChangeDataTypeSql constructs an ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE statement.
func BuildChangeDataTypeSql(db, schema, table, column, dataType string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DATA TYPE %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column), strings.TrimSpace(dataType))
}

// BuildSetColumnDefaultSql constructs an ALTER TABLE ... ALTER COLUMN ... SET DEFAULT statement.
// The expression is emitted verbatim (Snowflake only permits a sequence reference
// here, e.g. seq.NEXTVAL); the caller is responsible for its correctness.
func BuildSetColumnDefaultSql(db, schema, table, column, expr string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column), strings.TrimSpace(expr))
}

// BuildDropColumnDefaultSql constructs an ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT statement.
func BuildDropColumnDefaultSql(db, schema, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column))
}

// BuildSetColumnMaskingPolicySql constructs an ALTER TABLE ... ALTER COLUMN ... SET MASKING POLICY statement.
func BuildSetColumnMaskingPolicySql(db, schema, table, column, policyDb, policySchema, policyName string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET MASKING POLICY %s;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column),
		tableRef(policyDb, policySchema, policyName))
}

// BuildUnsetColumnMaskingPolicySql constructs an ALTER TABLE ... ALTER COLUMN ... UNSET MASKING POLICY statement.
func BuildUnsetColumnMaskingPolicySql(db, schema, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s UNSET MASKING POLICY;",
		tableRef(db, schema, table), snowflake.QuoteIdent(column))
}
