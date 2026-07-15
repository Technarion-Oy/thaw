// SPDX-License-Identifier: GPL-3.0-or-later

package pipe

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// GetCopyHistory returns copy history rows for the given pipe from
// INFORMATION_SCHEMA. It resolves the pipe's COPY INTO target table from the
// pipe DDL and queries the copy_history table function.
//
// startTime is an optional ISO-8601 timestamp; if empty, defaults to 24 hours ago.
// status is an optional status filter (LOADED, LOAD_FAILED, PARTIALLY_LOADED, etc.); if empty, all statuses are returned.
// fileName is an optional file name substring filter; if empty, all files are returned.
func GetCopyHistory(
	ctx context.Context,
	client *snowflake.Client,
	database, schema, name, startTime, status, fileName string,
) (*snowflake.QueryResult, error) {
	// Fetch the pipe DDL to resolve the COPY INTO target table name, which is
	// required by the copy_history table function.
	ddl, err := client.GetObjectDDL(ctx, database, schema, "PIPE", name, "")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve pipe DDL to resolve target table: %w", err)
	}
	fqnParts, err := ParseCopyIntoTargetParts(ddl)
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

	return client.Execute(ctx, sb.String())
}
