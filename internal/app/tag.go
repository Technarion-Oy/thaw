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
	"strings"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"thaw/internal/tag"
)

// maxTagReferenceRows caps the account-wide TAG_REFERENCES scan used by the
// centralized Tag Management view. ACCOUNT_USAGE.TAG_REFERENCES can hold a very
// large number of rows; the cap keeps the read bounded and the result is
// filtered client-side.
const maxTagReferenceRows = 10000

// AlterTag runs an ALTER TAG statement for the given tag. clause is everything
// that follows the tag name, e.g. "RENAME TO <new>", "SET COMMENT = '...'",
// "UNSET COMMENT", "ADD ALLOWED_VALUES 'a', 'b'", "DROP ALLOWED_VALUES 'a'",
// "UNSET ALLOWED_VALUES", "SET MASKING POLICY <policy>", or "UNSET MASKING
// POLICY <policy>". The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the tag identifier.
func (a *App) AlterTag(database, schema, name, clause string) error {
	return a.alterObject("TAG", database, schema, name, clause)
}

// GetTagReferences returns the objects and columns to which the given tag is
// currently applied, by querying SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES. The
// view requires governance privileges (e.g. the ACCOUNTADMIN role or a grant on
// the SNOWFLAKE database) and has propagation latency, so newly-applied tags may
// not appear immediately. Rows with a non-null OBJECT_DELETED are excluded so
// only live references are returned.
func (a *App) GetTagReferences(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME, DOMAIN, TAG_VALUE "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES "+
			"WHERE TAG_DATABASE = '%s' AND TAG_SCHEMA = '%s' AND TAG_NAME = '%s' AND OBJECT_DELETED IS NULL "+
			"ORDER BY OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name))
	return a.client.QuerySingle(a.ctx, query)
}

// ListAccountTags returns the tag catalog for the whole account via SHOW TAGS IN
// ACCOUNT — every tag with its database, schema, owner, comment, and allowed
// values. It backs the catalog tab of the centralized Tag Management view.
// SHOW TAGS requires privileges on the tags (or a role that owns/has access to
// them); accounts without governance access may see only a subset.
func (a *App) ListAccountTags() (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.QuerySingle(a.ctx, "SHOW TAGS IN ACCOUNT")
}

// GetAllTagReferences returns every live tag application across the account by
// scanning SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES (tag name/value alongside the
// tagged object's database/schema/name, column, and domain). It backs the
// references browser of the centralized Tag Management view, which applies tag /
// value / database / schema / domain filters client-side. The view requires
// governance privileges and has propagation latency, so very recent changes may
// be missing. The scan is capped at maxTagReferenceRows; QueryResult.Truncated
// reflects whether more references exist than were returned.
func (a *App) GetAllTagReferences() (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	query := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE, "+
			"OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME, DOMAIN "+
			"FROM SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES "+
			"WHERE OBJECT_DELETED IS NULL "+
			"ORDER BY TAG_NAME, OBJECT_DATABASE, OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME "+
			"LIMIT %d",
		maxTagReferenceRows)
	return a.client.QuerySingle(a.ctx, query)
}

// SetObjectTag applies (or retags, with a new value) the tag identified by
// tagDatabase/tagSchema/tagName on the object described by ref, issuing
// `ALTER <object> SET TAG <tag> = '<value>'`. It backs the apply / change-value
// edit actions of the centralized Tag Management view. ref.Domain selects the
// ALTER object keyword and how the object name is qualified; see
// tag.BuildAlterObjectTagSql.
func (a *App) SetObjectTag(ref tag.ObjectTagRef, tagDatabase, tagSchema, tagName, value string) error {
	return a.alterObjectTag(ref, tagDatabase, tagSchema, tagName, value, false)
}

// UnsetObjectTag removes the tag identified by tagDatabase/tagSchema/tagName
// from the object described by ref, issuing `ALTER <object> UNSET TAG <tag>`. It
// backs the remove edit action of the centralized Tag Management view.
func (a *App) UnsetObjectTag(ref tag.ObjectTagRef, tagDatabase, tagSchema, tagName string) error {
	return a.alterObjectTag(ref, tagDatabase, tagSchema, tagName, "", true)
}

// alterObjectTag is the shared body behind SetObjectTag / UnsetObjectTag: it
// builds the fully-qualified tag name, delegates the ALTER statement to
// tag.BuildAlterObjectTagSql, and executes it.
func (a *App) alterObjectTag(ref tag.ObjectTagRef, tagDatabase, tagSchema, tagName, value string, unset bool) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	tagFQN := snowflake.Qualify(tagDatabase, tagSchema, tagName)
	sql, err := tag.BuildAlterObjectTagSql(ref, tagFQN, value, unset)
	if err != nil {
		return err
	}
	_, err = a.client.Execute(a.ctx, sql)
	return err
}

// GetObjectTagReferences returns the tags currently applied to a single object,
// using the INFORMATION_SCHEMA.TAG_REFERENCES table function. Unlike the
// account-wide ACCOUNT_USAGE.TAG_REFERENCES view (GetTagReferences /
// GetAllTagReferences), this function is per-object and has no propagation
// latency, so it reflects tag changes immediately — it backs the per-object
// "Tag References" view in the object browser. domain is the object's Snowflake
// domain (its SHOW kind, e.g. TABLE, VIEW, STAGE, …). LEVEL distinguishes a tag
// set directly on the object from one inherited from a higher level (the schema,
// database, or account).
//
// args is the comma-separated parameter type list for callable objects
// (procedures and functions, e.g. "NUMBER, VARCHAR"); it is required there
// because the object name must carry the argument signature to resolve the
// overload, and is ignored for every other domain. The EXTERNAL FUNCTION and
// DATA METRIC FUNCTION kinds are normalized to the FUNCTION domain the table
// function expects.
//
// The table function lives in the object database's INFORMATION_SCHEMA and takes
// the object name as a string literal; the fully-qualified, double-quoted name is
// embedded so mixed-case / special-character identifiers resolve correctly.
func (a *App) GetObjectTagReferences(domain, database, schema, name, args string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := snowflake.Qualify(database, schema, name)
	d := strings.ToUpper(strings.TrimSpace(domain))
	switch d {
	case "FUNCTION", "EXTERNAL FUNCTION", "DATA METRIC FUNCTION":
		// All function variants share the FUNCTION domain and need a signature.
		fqn += "(" + args + ")"
		d = "FUNCTION"
	case "PROCEDURE":
		fqn += "(" + args + ")"
	}
	query := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE, LEVEL "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', '%s')) "+
			"ORDER BY TAG_NAME",
		snowflake.QuoteIdent(database),
		snowflake.EscapeStringLit(fqn),
		snowflake.EscapeStringLit(d))
	return a.client.QuerySingle(a.ctx, query)
}

// GetColumnTagReferences returns the tags applied to every column of a table or
// view via the INFORMATION_SCHEMA.TAG_REFERENCES_ALL_COLUMNS table function — the
// no-latency, per-table companion to GetObjectTagReferences for column-level
// tags. domain is the parent object's domain (TABLE or VIEW, and the other
// column-bearing kinds Snowflake accepts). One row is returned per (column, tag).
func (a *App) GetColumnTagReferences(domain, database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := snowflake.Qualify(database, schema, name)
	query := fmt.Sprintf(
		"SELECT COLUMN_NAME, TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES_ALL_COLUMNS('%s', '%s')) "+
			"ORDER BY COLUMN_NAME, TAG_NAME",
		snowflake.QuoteIdent(database),
		snowflake.EscapeStringLit(fqn),
		snowflake.EscapeStringLit(strings.ToUpper(domain)))
	return a.client.QuerySingle(a.ctx, query)
}
