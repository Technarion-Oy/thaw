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
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterSequence runs an ALTER SEQUENCE statement for the given sequence. clause
// is everything that follows the sequence name, e.g. "SET INCREMENT = 2",
// "SET COMMENT = '...'", "UNSET COMMENT", or "RENAME TO ...". The caller is
// responsible for correct SQL quoting inside the clause; this method only
// double-quotes the sequence identifier.
func (a *App) AlterSequence(database, schema, name, clause string) error {
	return a.alterObject("SEQUENCE", database, schema, name, clause)
}

// ListAccountSequences returns every sequence in the account via SHOW SEQUENCES
// IN ACCOUNT (name, database_name, schema_name, next_value, interval, owner,
// comment, …). It backs the sequence-default picker in the column properties
// editor: an ALTER TABLE … ALTER COLUMN … SET DEFAULT on an existing column only
// accepts a sequence reference (<seq>.NEXTVAL), so this is the one default
// expression Snowflake permits there. Sequences require privileges, so accounts
// without access may see only a subset.
func (a *App) ListAccountSequences() (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return a.client.QuerySingle(a.ctx, "SHOW SEQUENCES IN ACCOUNT")
}
