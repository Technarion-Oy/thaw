// SPDX-License-Identifier: GPL-3.0-or-later

package queryhistory

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// maxResultLimit is Snowflake's documented ceiling for the RESULT_LIMIT argument
// of the QUERY_HISTORY* table functions (1–10 000). When extra WHERE filters are
// active we fetch this full window from the function and re-apply the user's
// smaller limit afterwards (see buildQueryHistorySql).
const maxResultLimit = 10000

// QueryHistoryFilters holds the optional server-side WHERE-clause filters applied
// on top of the QUERY_HISTORY* table function's own (session/user/warehouse/time)
// named arguments. Every field is optional; a zero value adds no predicate.
//
// These fields are not accepted as named arguments by the table functions, so
// they are expressed as a WHERE clause wrapping the function call. Because the
// function applies RESULT_LIMIT itself (before our WHERE), buildQueryHistorySql
// fetches a full window when any filter here is set and re-applies the caller's
// limit as an outer LIMIT — so the limit means "N matching rows", not "matches
// within the first N".
type QueryHistoryFilters struct {
	// Statuses restricts to rows whose EXECUTION_STATUS is one of these values
	// (matched case-insensitively), e.g. "SUCCESS", "FAIL", "RUNNING".
	Statuses []string `json:"statuses"`
	// QueryTypes restricts to rows whose QUERY_TYPE is one of these values
	// (matched case-insensitively), e.g. "SELECT", "INSERT", "CREATE".
	QueryTypes []string `json:"queryTypes"`
	// MinDurationMs restricts to rows with TOTAL_ELAPSED_TIME >= this many
	// milliseconds. Zero or negative adds no duration predicate.
	MinDurationMs int64 `json:"minDurationMs"`
	// Database restricts to rows whose DATABASE_NAME matches this value
	// (case-insensitive equality). Empty adds no predicate.
	Database string `json:"database"`
	// Schema restricts to rows whose SCHEMA_NAME matches this value
	// (case-insensitive equality). Empty adds no predicate.
	Schema string `json:"schema"`
}

// buildQueryHistoryPredicates renders the QueryHistoryFilters into a slice of SQL
// boolean predicates (to be ANDed in a WHERE clause). All string literals are
// escaped via snowflake.QuoteStringLit; the numeric duration is emitted directly.
func buildQueryHistoryPredicates(f QueryHistoryFilters) []string {
	var predicates []string
	if clause := inClauseCI("EXECUTION_STATUS", f.Statuses); clause != "" {
		predicates = append(predicates, clause)
	}
	if clause := inClauseCI("QUERY_TYPE", f.QueryTypes); clause != "" {
		predicates = append(predicates, clause)
	}
	if f.MinDurationMs > 0 {
		predicates = append(predicates, fmt.Sprintf("TOTAL_ELAPSED_TIME >= %d", f.MinDurationMs))
	}
	if db := strings.TrimSpace(f.Database); db != "" {
		predicates = append(predicates, fmt.Sprintf("UPPER(DATABASE_NAME) = UPPER(%s)", snowflake.QuoteStringLit(db)))
	}
	if sch := strings.TrimSpace(f.Schema); sch != "" {
		predicates = append(predicates, fmt.Sprintf("UPPER(SCHEMA_NAME) = UPPER(%s)", snowflake.QuoteStringLit(sch)))
	}
	return predicates
}

// inClauseCI builds a case-insensitive `UPPER(col) IN (...)` predicate from vals,
// dropping blank entries and escaping each literal. Returns "" when no non-blank
// value remains (so the caller adds no predicate).
func inClauseCI(col string, vals []string) string {
	quoted := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		quoted = append(quoted, snowflake.QuoteStringLit(strings.ToUpper(v)))
	}
	if len(quoted) == 0 {
		return ""
	}
	return fmt.Sprintf("UPPER(%s) IN (%s)", col, strings.Join(quoted, ", "))
}

// QueryHistoryRow holds one row from INFORMATION_SCHEMA.QUERY_HISTORY*.
type QueryHistoryRow struct {
	QueryID       string `json:"queryId"`
	SessionID     string `json:"sessionId"`
	QueryText     string `json:"queryText"`
	QueryType     string `json:"queryType"`
	UserName      string `json:"userName"`
	WarehouseName string `json:"warehouseName"`
	DatabaseName  string `json:"databaseName"`
	SchemaName    string `json:"schemaName"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	ElapsedMs     int64  `json:"elapsedMs"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"errorMessage"`
	RowsProduced  int64  `json:"rowsProduced"`
	BytesScanned  int64  `json:"bytesScanned"`
}

// buildQueryHistorySql builds the SELECT over the appropriate
// INFORMATION_SCHEMA.QUERY_HISTORY* table function for the given filter.
//
//   - filterType:             "session" | "user" | "warehouse" | "all"
//   - sessionID:              valid int64 → SESSION_ID => <id> (filterType="session")
//   - userName:               non-empty → USER_NAME => '<name>'
//   - warehouseName:          non-empty → WAREHOUSE_NAME => '<name>'
//   - endTimeStart/End:       RFC3339 strings or "" for no filter
//   - resultLimit:            max rows returned (1–10 000)
//   - includeClientGenerated: include client-generated statements
//   - filters:                extra WHERE-clause filters (status/type/duration/db/schema)
//
// Precondition: for filterType "session", sessionID must be a valid bare int64.
// GetQueryHistory (the primary gate, and the only production caller) enforces
// this; passing an invalid id here is a programmer error and panics rather than
// silently emitting an argument-less QUERY_HISTORY_BY_SESSION() that resolves to
// the wrong (pooled) session. Always go through GetQueryHistory.
//
// When filters add a WHERE clause, the table function is asked for a full window
// (RESULT_LIMIT => maxResultLimit) and the caller's resultLimit is re-applied as
// an outer LIMIT — so the limit counts matching rows, not rows scanned before the
// filter. Without filters the resultLimit is passed straight to the function as
// before.
func buildQueryHistorySql(
	filterType string,
	sessionID string,
	userName string,
	warehouseName string,
	endTimeStart string,
	endTimeEnd string,
	resultLimit int,
	includeClientGenerated bool,
	filters QueryHistoryFilters,
) string {
	predicates := buildQueryHistoryPredicates(filters)
	hasFilters := len(predicates) > 0

	// The table function applies RESULT_LIMIT before our WHERE runs, so with a
	// filter active a small user limit would filter an already-truncated page.
	// Fetch the full window and re-apply the user's limit outside the function.
	innerLimit := resultLimit
	if hasFilters && resultLimit > 0 && resultLimit < maxResultLimit {
		innerLimit = maxResultLimit
	}
	// Choose the table function name.
	var funcName string
	switch filterType {
	case "session":
		funcName = "QUERY_HISTORY_BY_SESSION"
	case "user":
		funcName = "QUERY_HISTORY_BY_USER"
	case "warehouse":
		funcName = "QUERY_HISTORY_BY_WAREHOUSE"
	default:
		funcName = "QUERY_HISTORY"
	}

	// Build the named-argument list.
	var args []string
	switch filterType {
	case "session":
		// SESSION_ID is a bare numeric argument (not quoted), so it must never be
		// embedded verbatim — a value like "1234, RESULT_LIMIT => 10000" would
		// inject extra named arguments. The precondition (a valid bare int64) is
		// enforced by GetQueryHistory; an invalid id reaching here is a programmer
		// error, so fail loud rather than emit a semantically wrong query.
		if !snowflake.IsNumericID(sessionID) {
			panic(fmt.Sprintf("buildQueryHistorySql: invalid session id %q (callers must validate via GetQueryHistory)", sessionID))
		}
		args = append(args, fmt.Sprintf("SESSION_ID => %s", sessionID))
	case "user":
		if userName != "" {
			args = append(args, fmt.Sprintf("USER_NAME => %s", snowflake.QuoteStringLit(userName)))
		}
	case "warehouse":
		if warehouseName != "" {
			args = append(args, fmt.Sprintf("WAREHOUSE_NAME => %s", snowflake.QuoteStringLit(warehouseName)))
		}
	}
	// QuoteStringLit escapes any embedded single-quote so the timestamp literal
	// cannot break out of its delimiter (consistent with USER_NAME/WAREHOUSE_NAME).
	if endTimeStart != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_START => %s::TIMESTAMP_LTZ", snowflake.QuoteStringLit(endTimeStart)))
	}
	if endTimeEnd != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_END => %s::TIMESTAMP_LTZ", snowflake.QuoteStringLit(endTimeEnd)))
	}
	if innerLimit > 0 {
		args = append(args, fmt.Sprintf("RESULT_LIMIT => %d", innerLimit))
	}
	if includeClientGenerated {
		args = append(args, "INCLUDE_CLIENT_GENERATED_STATEMENT => TRUE")
	}

	argClause := ""
	if len(args) > 0 {
		argClause = strings.Join(args, ", ")
	}

	whereClause := ""
	if hasFilters {
		whereClause = "\nWHERE " + strings.Join(predicates, "\n  AND ")
	}
	// Re-apply the user's limit after filtering (see the innerLimit bump above).
	outerLimit := ""
	if hasFilters && resultLimit > 0 {
		outerLimit = fmt.Sprintf("\nLIMIT %d", resultLimit)
	}

	return fmt.Sprintf(`
SELECT QUERY_ID, SESSION_ID, QUERY_TEXT, QUERY_TYPE, USER_NAME, WAREHOUSE_NAME,
       DATABASE_NAME, SCHEMA_NAME, START_TIME, END_TIME,
       TOTAL_ELAPSED_TIME, EXECUTION_STATUS, ERROR_MESSAGE,
       ROWS_PRODUCED, BYTES_SCANNED
FROM table(SNOWFLAKE.information_schema.%s(%s))%s
ORDER BY START_TIME DESC%s`, funcName, argClause, whereClause, outerLimit)
}

// ParseQueryHistory projects a query-history result into QueryHistoryRow values.
func ParseQueryHistory(res *snowflake.QueryResult) []QueryHistoryRow {
	if res == nil {
		return []QueryHistoryRow{}
	}

	qidIdx := snowflake.ColIdx(res.Columns, "query_id")
	sidIdx := snowflake.ColIdx(res.Columns, "session_id")
	qtxtIdx := snowflake.ColIdx(res.Columns, "query_text")
	qtypIdx := snowflake.ColIdx(res.Columns, "query_type")
	userIdx := snowflake.ColIdx(res.Columns, "user_name")
	whIdx := snowflake.ColIdx(res.Columns, "warehouse_name")
	dbIdx := snowflake.ColIdx(res.Columns, "database_name")
	schIdx := snowflake.ColIdx(res.Columns, "schema_name")
	stIdx := snowflake.ColIdx(res.Columns, "start_time")
	etIdx := snowflake.ColIdx(res.Columns, "end_time")
	elIdx := snowflake.ColIdx(res.Columns, "total_elapsed_time")
	statIdx := snowflake.ColIdx(res.Columns, "execution_status")
	errIdx := snowflake.ColIdx(res.Columns, "error_message")
	rpIdx := snowflake.ColIdx(res.Columns, "rows_produced")
	bsIdx := snowflake.ColIdx(res.Columns, "bytes_scanned")

	get := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	rows := make([]QueryHistoryRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, QueryHistoryRow{
			QueryID:       snowflake.CellString(get(row, qidIdx)),
			SessionID:     snowflake.CellString(get(row, sidIdx)),
			QueryText:     snowflake.CellString(get(row, qtxtIdx)),
			QueryType:     snowflake.CellString(get(row, qtypIdx)),
			UserName:      snowflake.CellString(get(row, userIdx)),
			WarehouseName: snowflake.CellString(get(row, whIdx)),
			DatabaseName:  snowflake.CellString(get(row, dbIdx)),
			SchemaName:    snowflake.CellString(get(row, schIdx)),
			StartTime:     snowflake.CellString(get(row, stIdx)),
			EndTime:       snowflake.CellString(get(row, etIdx)),
			ElapsedMs:     snowflake.CellInt64(get(row, elIdx)),
			Status:        snowflake.CellString(get(row, statIdx)),
			ErrorMessage:  snowflake.CellString(get(row, errIdx)),
			RowsProduced:  snowflake.CellInt64(get(row, rpIdx)),
			BytesScanned:  snowflake.CellInt64(get(row, bsIdx)),
		})
	}
	return rows
}

// GetQueryHistory runs the query-history query for the given filter and returns
// the parsed rows ordered by start time descending.
func GetQueryHistory(
	ctx context.Context,
	client *snowflake.Client,
	filterType string,
	sessionID string,
	userName string,
	warehouseName string,
	endTimeStart string,
	endTimeEnd string,
	resultLimit int,
	includeClientGenerated bool,
	filters QueryHistoryFilters,
) ([]QueryHistoryRow, error) {
	// Trim the name filters up front so the empty-checks and the value embedded
	// by buildQueryHistorySql agree — otherwise "ALICE " would pass the guard and
	// be matched verbatim (zero rows, no error).
	userName = strings.TrimSpace(userName)
	warehouseName = strings.TrimSpace(warehouseName)

	// Reject a non-numeric session id at the boundary with a clear error rather
	// than silently producing an argument-less QUERY_HISTORY_BY_SESSION() that
	// resolves to the wrong (pooled metadata) session.
	if filterType == "session" && !snowflake.IsNumericID(sessionID) {
		return nil, fmt.Errorf("invalid session id %q: must be a numeric Snowflake session id", sessionID)
	}
	// Likewise require an explicit user for user scope — an empty USER_NAME would
	// drop the filter and widen the query beyond the intended user. Use the "all"
	// scope to query history across users.
	if filterType == "user" && userName == "" {
		return nil, fmt.Errorf("a user name is required for user-scoped query history")
	}
	// And an explicit warehouse for warehouse scope — an empty WAREHOUSE_NAME
	// would drop the filter, leaving QUERY_HISTORY_BY_WAREHOUSE() to resolve to
	// the pooled metadata connection's warehouse (silently wrong, not an error).
	if filterType == "warehouse" && warehouseName == "" {
		return nil, fmt.Errorf("a warehouse name is required for warehouse-scoped query history")
	}
	query := buildQueryHistorySql(filterType, sessionID, userName, warehouseName, endTimeStart, endTimeEnd, resultLimit, includeClientGenerated, filters)
	res, err := client.QuerySingle(ctx, query)
	if err != nil {
		return nil, err
	}
	return ParseQueryHistory(res), nil
}
