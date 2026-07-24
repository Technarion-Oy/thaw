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
