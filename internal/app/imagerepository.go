// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterImageRepository runs an ALTER IMAGE REPOSITORY statement for the given
// repository. clause is everything that follows the repository name, e.g.
// "SET COMMENT = '...'" or "UNSET COMMENT". Image repositories cannot be renamed
// and COMMENT is the only mutable property, so those are the only clauses the
// properties panel issues. The caller is responsible for correct SQL quoting
// inside the clause; this method only double-quotes the repository identifier.
func (a *App) AlterImageRepository(database, schema, name, clause string) error {
	return a.alterObject("IMAGE REPOSITORY", database, schema, name, clause)
}

// ListImagesInRepository returns the images stored in the given image
// repository via SHOW IMAGES IN IMAGE REPOSITORY. The raw QueryResult is
// returned so the properties panel can render every column the Snowflake edition
// reports (typically created_on, image_name, tags, digest, image_path) without
// the backend pinning a fixed shape.
func (a *App) ListImagesInRepository(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW IMAGES IN IMAGE REPOSITORY %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}
