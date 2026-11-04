// SPDX-License-Identifier: GPL-3.0-or-later

package dynamictable

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// RefreshHistoryRow is one row of INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY().
// Timestamps are RFC3339 strings; numeric columns are their text form ("" when NULL).
type RefreshHistoryRow struct {
	State            string `json:"state"`
	StateCode        string `json:"stateCode"`
	StateMessage     string `json:"stateMessage"`
	RefreshAction    string `json:"refreshAction"`
	RefreshTrigger   string `json:"refreshTrigger"`
	DataTimestamp    string `json:"dataTimestamp"`
	RefreshStartTime string `json:"refreshStartTime"`
	RefreshEndTime   string `json:"refreshEndTime"`
	NumRowsInserted  string `json:"numRowsInserted"`
	NumRowsDeleted   string `json:"numRowsDeleted"`
	TargetLagSec     string `json:"targetLagSec"`
	QueryID          string `json:"queryId"`
}

// refreshHistoryColumns is the SELECT list; the row counts live inside the
// STATISTICS object, so they are flattened here.
var refreshHistoryColumns = []string{
	"STATE", "STATE_CODE", "STATE_MESSAGE", "REFRESH_ACTION", "REFRESH_TRIGGER",
	"DATA_TIMESTAMP", "REFRESH_START_TIME", "REFRESH_END_TIME",
	"STATISTICS:numInsertedRows::NUMBER AS NUMROWS_INSERTED",
	"STATISTICS:numDeletedRows::NUMBER AS NUMROWS_DELETED",
	"TARGET_LAG_SEC", "QUERY_ID",
}

// BuildRefreshHistorySql returns the DYNAMIC_TABLE_REFRESH_HISTORY query for one
// dynamic table over the last days days (clamped to 1..7 — the function only
// retains 7 days), newest first.
func BuildRefreshHistorySql(database, schema, name string, days int) string {
	days = max(1, min(days, 7))
	return fmt.Sprintf(
		`SELECT %s FROM TABLE(%s.INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY(`+
			`NAME => '%s', `+
			`DATA_TIMESTAMP_START => DATEADD('day', -%d, CURRENT_TIMESTAMP()), `+
			`RESULT_LIMIT => 10000))`+
			` ORDER BY DATA_TIMESTAMP DESC, REFRESH_START_TIME DESC NULLS FIRST`,
		strings.Join(refreshHistoryColumns, ", "), snowflake.QuoteIdent(database),
		snowflake.EscapeStringLit(snowflake.Qualify(database, schema, name)), days)
}

// parseRefreshHistory maps a query result to rows by column name.
func parseRefreshHistory(res *snowflake.QueryResult) []RefreshHistoryRow {
	i := snowflake.ColumnIndexes(res, "state", "state_code", "state_message", "refresh_action",
		"refresh_trigger", "data_timestamp", "refresh_start_time", "refresh_end_time",
		"numrows_inserted", "numrows_deleted", "target_lag_sec", "query_id")
	rows := make([]RefreshHistoryRow, 0, len(res.Rows))
	for _, r := range res.Rows {
		rows = append(rows, RefreshHistoryRow{
			State:            snowflake.Cell(r, i["state"]),
			StateCode:        snowflake.Cell(r, i["state_code"]),
			StateMessage:     snowflake.Cell(r, i["state_message"]),
			RefreshAction:    snowflake.Cell(r, i["refresh_action"]),
			RefreshTrigger:   snowflake.Cell(r, i["refresh_trigger"]),
			DataTimestamp:    snowflake.Cell(r, i["data_timestamp"]),
			RefreshStartTime: snowflake.Cell(r, i["refresh_start_time"]),
			RefreshEndTime:   snowflake.Cell(r, i["refresh_end_time"]),
			NumRowsInserted:  snowflake.Cell(r, i["numrows_inserted"]),
			NumRowsDeleted:   snowflake.Cell(r, i["numrows_deleted"]),
			TargetLagSec:     snowflake.Cell(r, i["target_lag_sec"]),
			QueryID:          snowflake.Cell(r, i["query_id"]),
		})
	}
	return rows
}

// GetDynamicTableRefreshHistory returns recent refreshes of one dynamic table.
// Requires MONITOR (or OPERATE/ownership) on the table; Snowflake's error is
// returned unchanged so the UI can surface it.
func GetDynamicTableRefreshHistory(ctx context.Context, client *snowflake.Client, database, schema, name string, days int) ([]RefreshHistoryRow, error) {
	res, err := client.Execute(ctx, BuildRefreshHistorySql(database, schema, name, days))
	if err != nil {
		return nil, fmt.Errorf("failed to query dynamic table refresh history: %w", err)
	}
	return parseRefreshHistory(res), nil
}
