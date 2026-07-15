// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterModel runs an ALTER MODEL statement for the given model. clause is
// everything that follows the model name, e.g. "SET COMMENT = '...'",
// "SET DEFAULT_VERSION = 'V2'", "VERSION \"V1\" SET ALIAS = PROD",
// "VERSION \"V1\" UNSET ALIAS", "SET TAG ...", "UNSET TAG ...", or
// "RENAME TO ...". The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the model identifier.
func (a *App) AlterModel(database, schema, name, clause string) error {
	return a.alterObject("MODEL", database, schema, name, clause)
}

// ListModels returns every model the current role can see, as fully-qualified
// quoted identifiers (`"DB"."SCHEMA"."NAME"`), via SHOW MODELS IN ACCOUNT. The
// create-model / add-version source pickers use it to offer existing models as a
// copy source instead of a free-text field.
func (a *App) ListModels() ([]string, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListModels(a.fctx(FeatureObjectEditor))
}

// GetModelTags returns the tags currently applied to the given model, via the
// INFORMATION_SCHEMA.TAG_REFERENCES table function (object domain MODEL). Unlike
// the ACCOUNT_USAGE.TAG_REFERENCES view this reflects changes immediately (no
// propagation latency), which suits an interactive tag editor. The raw
// QueryResult is returned (tag_database / tag_schema / tag_name / tag_value
// columns) so the properties modal can render each tag as a removable chip. The
// caller treats an error as "no tags available" and still allows SET/UNSET TAG.
func (a *App) GetModelTags(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	sql := fmt.Sprintf(
		"SELECT TAG_DATABASE, TAG_SCHEMA, TAG_NAME, TAG_VALUE "+
			"FROM TABLE(%s.INFORMATION_SCHEMA.TAG_REFERENCES('%s', 'MODEL')) "+
			"ORDER BY TAG_DATABASE, TAG_SCHEMA, TAG_NAME",
		// EscapeTextLit (not EscapeStringLit): QuoteIdent doubles " but not \, so a
		// backslash in an identifier must be doubled to survive the single-quoted
		// literal rather than being read as a Snowflake escape sequence.
		snowflake.QuoteIdent(database), snowflake.EscapeTextLit(fqn))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// ListModelVersions returns the versions of the given model via
// SHOW VERSIONS IN MODEL. The raw QueryResult is returned so the properties panel
// can render every column the Snowflake edition reports (typically created_on,
// name, database_name, schema_name, model_name, is_default_version,
// is_last_version, aliases, comment, …) without the backend pinning a fixed
// shape.
func (a *App) ListModelVersions(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW VERSIONS IN MODEL %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}
