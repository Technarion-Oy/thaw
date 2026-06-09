// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package queryhistory

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// QueryHistoryRow holds one row from INFORMATION_SCHEMA.QUERY_HISTORY*.
type QueryHistoryRow struct {
	QueryID       string `json:"queryId"`
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

// BuildQueryHistorySql builds the SELECT over the appropriate
// INFORMATION_SCHEMA.QUERY_HISTORY* table function for the given filter.
//
//   - filterType:             "session" | "user" | "warehouse" | "all"
//   - sessionID:              non-empty → SESSION_ID => <id> (filterType="session")
//   - userName:               non-empty → USER_NAME => '<name>'
//   - warehouseName:          non-empty → WAREHOUSE_NAME => '<name>'
//   - endTimeStart/End:       RFC3339 strings or "" for no filter
//   - resultLimit:            max rows returned (1–10 000)
//   - includeClientGenerated: include client-generated statements
func BuildQueryHistorySql(
	filterType string,
	sessionID string,
	userName string,
	warehouseName string,
	endTimeStart string,
	endTimeEnd string,
	resultLimit int,
	includeClientGenerated bool,
) string {
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
		if sessionID != "" {
			args = append(args, fmt.Sprintf("SESSION_ID => %s", sessionID))
		}
	case "user":
		if userName != "" {
			args = append(args, fmt.Sprintf("USER_NAME => '%s'", strings.ReplaceAll(userName, "'", "''")))
		}
	case "warehouse":
		if warehouseName != "" {
			args = append(args, fmt.Sprintf("WAREHOUSE_NAME => '%s'", strings.ReplaceAll(warehouseName, "'", "''")))
		}
	}
	if endTimeStart != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_START => '%s'::TIMESTAMP_LTZ", endTimeStart))
	}
	if endTimeEnd != "" {
		args = append(args, fmt.Sprintf("END_TIME_RANGE_END => '%s'::TIMESTAMP_LTZ", endTimeEnd))
	}
	if resultLimit > 0 {
		args = append(args, fmt.Sprintf("RESULT_LIMIT => %d", resultLimit))
	}
	if includeClientGenerated {
		args = append(args, "INCLUDE_CLIENT_GENERATED_STATEMENT => TRUE")
	}

	argClause := ""
	if len(args) > 0 {
		argClause = strings.Join(args, ", ")
	}

	return fmt.Sprintf(`
SELECT QUERY_ID, QUERY_TEXT, QUERY_TYPE, USER_NAME, WAREHOUSE_NAME,
       DATABASE_NAME, SCHEMA_NAME, START_TIME, END_TIME,
       TOTAL_ELAPSED_TIME, EXECUTION_STATUS, ERROR_MESSAGE,
       ROWS_PRODUCED, BYTES_SCANNED
FROM table(SNOWFLAKE.information_schema.%s(%s))
ORDER BY START_TIME DESC`, funcName, argClause)
}

// ParseQueryHistory projects a query-history result into QueryHistoryRow values.
func ParseQueryHistory(res *snowflake.QueryResult) []QueryHistoryRow {
	if res == nil {
		return []QueryHistoryRow{}
	}

	qidIdx := snowflake.ColIdx(res.Columns, "query_id")
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
) ([]QueryHistoryRow, error) {
	query := BuildQueryHistorySql(filterType, sessionID, userName, warehouseName, endTimeStart, endTimeEnd, resultLimit, includeClientGenerated)
	res, err := client.QuerySingle(ctx, query)
	if err != nil {
		return nil, err
	}
	return ParseQueryHistory(res), nil
}
