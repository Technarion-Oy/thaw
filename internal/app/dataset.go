// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterDataset runs an ALTER DATASET statement for the given dataset. clause is
// everything that follows the dataset name. ALTER DATASET only manages versions,
// so clause is one of:
//
//	ADD VERSION '<name>' FROM ( <query> ) [PARTITION BY <expr>] [COMMENT = '...'] [METADATA = '...']
//	DROP VERSION '<name>'
//
// The caller is responsible for correct SQL quoting inside the clause (version
// names are single-quoted string literals); this method only double-quotes the
// dataset identifier.
func (a *App) AlterDataset(database, schema, name, clause string) error {
	return a.alterObject("DATASET", database, schema, name, clause)
}

// ListDatasetVersions returns the versions of the given dataset via
// SHOW VERSIONS IN DATASET. The raw QueryResult is returned so the properties
// panel can render every column the Snowflake edition reports without the backend
// pinning a fixed shape.
func (a *App) ListDatasetVersions(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW VERSIONS IN DATASET %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}
