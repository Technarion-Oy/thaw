// SPDX-License-Identifier: GPL-3.0-or-later

package dynamictable

import (
	"strings"
	"testing"
	"time"

	"thaw/internal/snowflake"
)

func TestBuildRefreshHistorySql(t *testing.T) {
	sql := BuildRefreshHistorySql("DB", "S", `my"dt`, 30)
	for _, want := range []string{
		`FROM TABLE("DB".INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY(`,
		`NAME => '"DB"."S"."my""dt"'`,
		`DATEADD('day', -7,`, // clamped to retention
		`STATISTICS:numInsertedRows::NUMBER AS NUMROWS_INSERTED`,
		`ORDER BY DATA_TIMESTAMP DESC`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q in:\n%s", want, sql)
		}
	}
	if !strings.Contains(BuildRefreshHistorySql("DB", "S", "T", 0), `DATEADD('day', -1,`) {
		t.Error("days <= 0 should clamp to 1")
	}
	if !strings.Contains(BuildRefreshHistorySql("DB", "S", "O'Brien", 1), `"O''Brien"'`) {
		t.Error("single quote in name must be escaped")
	}
}

func TestParseRefreshHistory(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	res := &snowflake.QueryResult{
		Columns: []string{"STATE", "STATE_MESSAGE", "REFRESH_START_TIME", "NUMROWS_INSERTED", "QUERY_ID"},
		Rows:    [][]any{{"FAILED", "boom", ts, nil, "01ab"}},
	}
	got := parseRefreshHistory(res)
	if len(got) != 1 {
		t.Fatalf("got %d rows", len(got))
	}
	r := got[0]
	if r.State != "FAILED" || r.StateMessage != "boom" || r.QueryID != "01ab" ||
		r.RefreshStartTime != "2026-01-02T03:04:05Z" || r.NumRowsInserted != "" || r.RefreshAction != "" {
		t.Errorf("unexpected row: %+v", r)
	}
}
