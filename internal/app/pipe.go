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
	"thaw/internal/pipe"
	"thaw/internal/snowflake"
)

// AlterPipe runs an ALTER PIPE statement for the given pipe.
// clause is everything that follows the pipe name, e.g. "SET PIPE_EXECUTION_PAUSED = TRUE"
// or "SET COMMENT = 'hello'". The caller is responsible for correct SQL quoting
// inside the clause; this method only double-quotes the pipe identifier.
func (a *App) AlterPipe(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER PIPE %s.%s.%s %s", snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// GetPipeStatus returns the JSON string produced by SYSTEM$PIPE_STATUS for the given pipe.
// The JSON includes fields such as executionState, pendingFileCount, and
// notificationChannelName. executionState is "PAUSED" when the pipe has been
// paused via ALTER PIPE SET PIPE_EXECUTION_PAUSED = TRUE.
func (a *App) GetPipeStatus(database, schema, name string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	// Build the FQN with double-quoted parts, then escape any embedded single
	// quotes so the whole string is safe inside the outer SQL string literal.
	pipeFqn := snowflake.QuoteIdent(database) + "." + snowflake.QuoteIdent(schema) + "." + snowflake.QuoteIdent(name)
	sql := fmt.Sprintf("SELECT SYSTEM$PIPE_STATUS('%s')", snowflake.EscapeStringLit(pipeFqn))
	result, err := a.client.Execute(a.ctx, sql)
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
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	// Fetch the pipe DDL to resolve the COPY INTO target table name, which is
	// required by the copy_history table function.
	ddl, err := a.client.GetObjectDDL(a.ctx, database, schema, "PIPE", name, "")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve pipe DDL to resolve target table: %w", err)
	}
	fqnParts, err := pipe.ParseCopyIntoTargetParts(ddl)
	if err != nil {
		return nil, fmt.Errorf("could not parse COPY INTO target table from pipe DDL: %w", err)
	}

	// Build the TABLE_NAME argument: double-quoted parts inside a single-quoted string literal.
	// Unquoted identifiers from GET_DDL may be in any case but Snowflake stores
	// them as uppercase, so uppercase them before quoting to ensure correct resolution.
	quotedParts := make([]string, len(fqnParts))
	for i, p := range fqnParts {
		val := p.Value
		if !p.Quoted {
			val = strings.ToUpper(val)
		}
		quotedParts[i] = snowflake.QuoteIdent(val)
	}
	tableNameArg := strings.Join(quotedParts, ".")

	startExpr := "dateadd(hours, -24, current_timestamp())"
	if startTime != "" {
		startExpr = fmt.Sprintf("'%s'::timestamp_ltz", snowflake.EscapeStringLit(startTime))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb,
		"SELECT * FROM TABLE(%s.information_schema.copy_history(TABLE_NAME => '%s', START_TIME => %s))",
		snowflake.QuoteIdent(database), snowflake.EscapeStringLit(tableNameArg), startExpr,
	)
	// Filter by pipe identity using exact case-sensitive match.
	// copy_history exposes pipe_catalog_name, pipe_schema_name, and pipe_name as
	// separate columns; PIPE_NAME alone does not contain a fully qualified name.
	fmt.Fprintf(&sb, " WHERE pipe_catalog_name = '%s' AND pipe_schema_name = '%s' AND pipe_name = '%s'",
		snowflake.EscapeStringLit(database), snowflake.EscapeStringLit(schema), snowflake.EscapeStringLit(name),
	)
	if status != "" {
		fmt.Fprintf(&sb, " AND STATUS ILIKE '%s'", snowflake.EscapeStringLit(status))
	}
	if fileName != "" {
		fmt.Fprintf(&sb, " AND FILE_NAME ILIKE '%%%s%%'", snowflake.EscapeStringLit(fileName))
	}
	fmt.Fprintf(&sb, " ORDER BY LAST_LOAD_TIME DESC NULLS LAST")

	return a.client.Execute(a.ctx, sb.String())
}
