// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterSemanticView runs an ALTER SEMANTIC VIEW statement for the given view.
// clause is everything that follows the view name, e.g. "RENAME TO <new>",
// "SET COMMENT = '...'", "UNSET COMMENT", "SET TAG <tag> = '...'", or
// "UNSET TAG <tag>". Snowflake's ALTER SEMANTIC VIEW only supports renaming,
// the comment, and tags — the definition body (TABLES/RELATIONSHIPS/FACTS/
// DIMENSIONS/METRICS) is changed via CREATE OR REPLACE, not ALTER. The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the view identifier.
func (a *App) AlterSemanticView(database, schema, name, clause string) error {
	return a.alterObject("SEMANTIC VIEW", database, schema, name, clause)
}

// DescribeSemanticView runs DESCRIBE SEMANTIC VIEW and returns the raw
// QueryResult. SHOW SEMANTIC VIEWS reports only metadata (owner, comment), so
// this is the source for the view's full structure: one row per property of a
// logical table, relationship, dimension, fact, or metric. Columns are
// object_kind / object_name / parent_entity / property / property_value.
func (a *App) DescribeSemanticView(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE SEMANTIC VIEW %s",
		snowflake.Qualify(database, schema, name))
	return a.client.Execute(a.ctx, sql)
}

// ListSemanticDimensions runs SHOW SEMANTIC DIMENSIONS IN <fqn> and returns the
// raw QueryResult (columns database_name / schema_name / semantic_view_name /
// table_name / name / data_type / synonyms / comment), rendered by the
// properties panel's Dimensions table.
func (a *App) ListSemanticDimensions(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW SEMANTIC DIMENSIONS IN %s",
		snowflake.Qualify(database, schema, name))
	return a.client.Execute(a.ctx, sql)
}

// ListSemanticFacts runs SHOW SEMANTIC FACTS IN <fqn> and returns the raw
// QueryResult (same column shape as SHOW SEMANTIC DIMENSIONS), rendered by the
// properties panel's Facts table.
func (a *App) ListSemanticFacts(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW SEMANTIC FACTS IN %s",
		snowflake.Qualify(database, schema, name))
	return a.client.Execute(a.ctx, sql)
}

// ListSemanticMetrics runs SHOW SEMANTIC METRICS IN <fqn> and returns the raw
// QueryResult (same column shape as SHOW SEMANTIC DIMENSIONS), rendered by the
// properties panel's Metrics table.
func (a *App) ListSemanticMetrics(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW SEMANTIC METRICS IN %s",
		snowflake.Qualify(database, schema, name))
	return a.client.Execute(a.ctx, sql)
}

// ListSemanticDimensionsForMetric runs SHOW SEMANTIC DIMENSIONS IN <fqn> FOR
// METRIC <metric> and returns the raw QueryResult (columns table_name / name /
// data_type / required / synonyms / comment). It identifies which dimensions can
// be queried alongside a specific metric. The metric name is double-quoted as an
// identifier.
func (a *App) ListSemanticDimensionsForMetric(database, schema, name, metric string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW SEMANTIC DIMENSIONS IN %s FOR METRIC %s",
		snowflake.Qualify(database, schema, name), snowflake.QuoteIdent(metric))
	return a.client.Execute(a.ctx, sql)
}

// GetSemanticViewTags returns the tags currently applied to the given semantic
// view, via the INFORMATION_SCHEMA.TAG_REFERENCES table function (object domain
// SEMANTIC VIEW). Unlike the ACCOUNT_USAGE.TAG_REFERENCES view this reflects
// changes immediately (no propagation latency), which suits an interactive tag
// editor. The raw QueryResult is returned (tag_database / tag_schema / tag_name /
// tag_value columns) so the properties modal can render each tag as a removable
// chip. The caller treats an error as "no tags available" and still allows
// SET/UNSET TAG.
func (a *App) GetSemanticViewTags(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'SEMANTIC VIEW')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		// EscapeTextLit (not EscapeStringLit): QuoteIdent doubles " but not \, so a
		// backslash in an identifier must be doubled to survive the single-quoted
		// literal rather than being read as a Snowflake escape sequence.
		snowflake.QuoteIdent(database), snowflake.EscapeTextLit(fqn))
	return a.client.Execute(a.ctx, sql)
}
