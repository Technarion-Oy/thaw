// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"thaw/internal/apperrors"
	"thaw/internal/pipe"
	"thaw/internal/snowflake"
)

// AlterPipe runs an ALTER PIPE statement for the given pipe.
// clause is everything that follows the pipe name, e.g. "SET PIPE_EXECUTION_PAUSED = TRUE"
// or "SET COMMENT = 'hello'". The caller is responsible for correct SQL quoting
// inside the clause; this method only double-quotes the pipe identifier.
func (a *App) AlterPipe(database, schema, name, clause string) error {
	return a.alterObject("PIPE", database, schema, name, clause)
}

// GetPipeStatus returns the JSON string produced by SYSTEM$PIPE_STATUS for the given pipe.
// The JSON includes fields such as executionState, pendingFileCount, and
// notificationChannelName. executionState is "PAUSED" when the pipe has been
// paused via ALTER PIPE SET PIPE_EXECUTION_PAUSED = TRUE.
func (a *App) GetPipeStatus(database, schema, name string) (string, error) {
	client := a.currentClient()
	if client == nil {
		return "", apperrors.ErrNotConnected
	}
	// Build the FQN with double-quoted parts, then escape any embedded single
	// quotes so the whole string is safe inside the outer SQL string literal.
	pipeFqn := snowflake.QuoteIdent(database) + "." + snowflake.QuoteIdent(schema) + "." + snowflake.QuoteIdent(name)
	sql := fmt.Sprintf("SELECT SYSTEM$PIPE_STATUS('%s')", snowflake.EscapeStringLit(pipeFqn))
	result, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	if err != nil {
		return "", err
	}
	if result == nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 || result.Rows[0][0] == nil {
		return "", nil
	}
	return fmt.Sprint(result.Rows[0][0]), nil
}

// GetPipeCopyHistory returns copy history rows for the given pipe from INFORMATION_SCHEMA.
// startTime is an optional ISO-8601 timestamp; if empty, defaults to 24 hours ago.
// status is an optional status filter (LOADED, LOAD_FAILED, PARTIALLY_LOADED, etc.); if empty, all statuses are returned.
// fileName is an optional file name substring filter; if empty, all files are returned.
func (a *App) GetPipeCopyHistory(database, schema, name, startTime, status, fileName string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return pipe.GetCopyHistory(a.fctx(FeatureObjectEditor), client, database, schema, name, startTime, status, fileName)
}
