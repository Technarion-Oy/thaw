// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package objects

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ColumnComment holds a column name and its optional comment.
type ColumnComment struct {
	Column  string `json:"column"`
	Comment string `json:"comment"`
}

// BuildObjectPropertiesQuery returns the SHOW/DESCRIBE query that fetches the
// metadata for a single Snowflake object. kind is one of: DATABASE, SCHEMA,
// TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE, MATERIALIZED VIEW, ALERT, TAG,
// MASKING POLICY, ROW ACCESS POLICY, NETWORK RULE, IMAGE REPOSITORY, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM,
// TASK, FILE FORMAT, PIPE, SECRET, GIT REPOSITORY, DBT PROJECT, WAREHOUSE, ROLE,
// USER.
func BuildObjectPropertiesQuery(database, schema, kind, name string) (string, error) {
	like := strings.ReplaceAll(name, `\`, `\\`)
	like = strings.ReplaceAll(like, "'", "''")

	switch strings.ToUpper(kind) {
	case "DATABASE":
		return fmt.Sprintf("SHOW DATABASES LIKE '%s'", like), nil
	case "DYNAMIC TABLE":
		return fmt.Sprintf("SHOW DYNAMIC TABLES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "EXTERNAL TABLE":
		return fmt.Sprintf("SHOW EXTERNAL TABLES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "MATERIALIZED VIEW":
		return fmt.Sprintf("SHOW MATERIALIZED VIEWS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "ALERT":
		return fmt.Sprintf("SHOW ALERTS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "TAG":
		return fmt.Sprintf("SHOW TAGS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "MASKING POLICY":
		return fmt.Sprintf("SHOW MASKING POLICIES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "ROW ACCESS POLICY":
		return fmt.Sprintf("SHOW ROW ACCESS POLICIES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "NETWORK RULE":
		return fmt.Sprintf("SHOW NETWORK RULES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "IMAGE REPOSITORY":
		return fmt.Sprintf("SHOW IMAGE REPOSITORIES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "SCHEMA":
		return fmt.Sprintf("SHOW SCHEMAS LIKE '%s' IN DATABASE %s", like, snowflake.QuoteIdent(database)), nil
	case "TABLE":
		return fmt.Sprintf("SHOW TABLES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "VIEW":
		return fmt.Sprintf("SHOW VIEWS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "FUNCTION":
		return fmt.Sprintf("SHOW FUNCTIONS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "PROCEDURE":
		return fmt.Sprintf("SHOW PROCEDURES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "SEQUENCE":
		return fmt.Sprintf("SHOW SEQUENCES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "STAGE":
		return fmt.Sprintf("SHOW STAGES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "STREAM":
		return fmt.Sprintf("SHOW STREAMS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "TASK":
		return fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "FILE FORMAT":
		return fmt.Sprintf("SHOW FILE FORMATS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "PIPE":
		return fmt.Sprintf("SHOW PIPES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "SECRET":
		return fmt.Sprintf("SHOW SECRETS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "GIT REPOSITORY":
		return fmt.Sprintf("SHOW GIT REPOSITORIES LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "DBT PROJECT":
		return fmt.Sprintf("SHOW DBT PROJECTS LIKE '%s' IN SCHEMA %s.%s", like, snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema)), nil
	case "WAREHOUSE":
		return fmt.Sprintf("SHOW WAREHOUSES LIKE '%s'", like), nil
	case "ROLE":
		return fmt.Sprintf("SHOW ROLES LIKE '%s'", like), nil
	case "USER":
		return fmt.Sprintf("SHOW USERS LIKE '%s'", like), nil
	default:
		return "", fmt.Errorf("unsupported object kind: %s", kind)
	}
}

// BuildDescribeStageQuery returns the DESCRIBE STAGE query used to enrich the
// SHOW STAGES result with stage-specific properties.
func BuildDescribeStageQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE STAGE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
}

// BuildDescribeMaskingPolicyQuery returns the DESCRIBE MASKING POLICY query used
// to enrich the SHOW MASKING POLICIES result with the policy's signature, return
// type, and body — none of which SHOW MASKING POLICIES reports.
func BuildDescribeMaskingPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE MASKING POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
}

// BuildDescribeRowAccessPolicyQuery returns the DESCRIBE ROW ACCESS POLICY query
// used to enrich the SHOW ROW ACCESS POLICIES result with the policy's
// signature, return type, and body — none of which SHOW ROW ACCESS POLICIES
// reports.
func BuildDescribeRowAccessPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE ROW ACCESS POLICY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
}

// BuildDescribeNetworkRuleQuery returns the DESCRIBE NETWORK RULE query used to
// enrich the SHOW NETWORK RULES result with the rule's value_list — which SHOW
// NETWORK RULES reports only as a count (entries_in_valuelist), not the actual
// identifiers.
func BuildDescribeNetworkRuleQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE NETWORK RULE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
}

// GetObjectProperties returns structured metadata for any Snowflake object by
// running the appropriate SHOW or DESCRIBE command and returning the result as
// key/value pairs. For STAGE objects it also appends DESCRIBE STAGE properties;
// for MASKING POLICY and ROW ACCESS POLICY objects it appends the DESCRIBE
// signature, return type, and body; for NETWORK RULE objects it appends the
// DESCRIBE NETWORK RULE value_list.
func GetObjectProperties(ctx context.Context, client *snowflake.Client, database, schema, kind, name string) ([]snowflake.PropertyPair, error) {
	query, err := BuildObjectPropertiesQuery(database, schema, kind, name)
	if err != nil {
		return nil, err
	}

	res, err := client.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	pairs := snowflake.ResultToPairs(res)

	if strings.ToUpper(kind) == "MASKING POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeMaskingPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE MASKING POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "ROW ACCESS POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeRowAccessPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE ROW ACCESS POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "NETWORK RULE" {
		descRes, err := client.Execute(ctx, BuildDescribeNetworkRuleQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE NETWORK RULE returns one row whose columns include
			// value_list (the actual identifiers). Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the value.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				if strings.ToLower(col) == "value_list" {
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "STAGE" {
		descRes, err := client.Execute(ctx, BuildDescribeStageQuery(database, schema, name))
		if err == nil {
			for _, row := range descRes.Rows {
				if len(row) >= 4 {
					parent := fmt.Sprintf("%v", row[0]) // parent_property
					prop := fmt.Sprintf("%v", row[1])   // property
					val := fmt.Sprintf("%v", row[3])    // property_value
					key := prop
					if parent != "" && parent != "STAGE_PROPERTIES" && parent != "null" {
						key = parent + "." + prop
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: key, Value: val})
				}
			}
		}
	}

	return pairs, nil
}

// BuildGetColumnCommentsQuery returns the INFORMATION_SCHEMA query that selects
// each column name and its comment, ordered by ordinal position.
func BuildGetColumnCommentsQuery(database, schema, table string) string {
	return fmt.Sprintf(
		`SELECT COLUMN_NAME, COALESCE(COMMENT, '') AS COMMENT`+
			` FROM %s.INFORMATION_SCHEMA.COLUMNS`+
			` WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'`+
			` ORDER BY ORDINAL_POSITION`,
		snowflake.QuoteIdent(database), snowflake.EscapeStringLit(strings.ToUpper(schema)), snowflake.EscapeStringLit(strings.ToUpper(table)),
	)
}

// ParseColumnComments projects a column-comment query result into ColumnComment
// rows (column name + comment), preserving ordinal order.
func ParseColumnComments(res *snowflake.QueryResult) []ColumnComment {
	if res == nil {
		return []ColumnComment{}
	}
	out := make([]ColumnComment, 0, len(res.Rows))
	for _, row := range res.Rows {
		col, cmt := "", ""
		if len(row) > 0 && row[0] != nil {
			col = fmt.Sprint(row[0])
		}
		if len(row) > 1 && row[1] != nil {
			cmt = fmt.Sprint(row[1])
		}
		out = append(out, ColumnComment{Column: col, Comment: cmt})
	}
	return out
}

// GetColumnComments returns the comment for every column in a table, ordered
// by ordinal position.
func GetColumnComments(ctx context.Context, client *snowflake.Client, database, schema, table string) ([]ColumnComment, error) {
	res, err := client.Execute(ctx, BuildGetColumnCommentsQuery(database, schema, table))
	if err != nil {
		return nil, err
	}
	return ParseColumnComments(res), nil
}

// BuildSetColumnCommentSql returns the ALTER TABLE ... MODIFY COLUMN ... COMMENT
// statement that sets (or clears) the comment on a single table column.
func BuildSetColumnCommentSql(database, schema, table, column, comment string) string {
	return fmt.Sprintf("ALTER TABLE %s.%s.%s MODIFY COLUMN %s COMMENT '%s'",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(table),
		snowflake.QuoteIdent(column), snowflake.EscapeStringLit(comment),
	)
}

// SetColumnComment sets (or clears) the COMMENT on a single table column.
func SetColumnComment(ctx context.Context, client *snowflake.Client, database, schema, table, column, comment string) error {
	_, err := client.Execute(ctx, BuildSetColumnCommentSql(database, schema, table, column, comment))
	return err
}
