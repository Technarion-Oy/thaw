// SPDX-License-Identifier: GPL-3.0-or-later

package backup

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildListBackupSetsSql(t *testing.T) {
	plain := BuildListBackupSetsSql("")
	if plain != "SHOW BACKUP SETS IN ACCOUNT" {
		t.Errorf("unexpected unfiltered SQL: %q", plain)
	}
	filtered := BuildListBackupSetsSql("o'ne")
	if !strings.Contains(filtered, "LIKE '%o''ne%'") {
		t.Errorf("expected escaped LIKE filter, got: %q", filtered)
	}

	// LIKE wildcards in the typed name are escaped so they match literally.
	wild := BuildListBackupSetsSql("50%_off")
	if !strings.Contains(wild, `LIKE '%50\%\_off%'`) {
		t.Errorf("expected escaped LIKE wildcards, got: %q", wild)
	}
}

func TestBuildApplyBackupPolicySql(t *testing.T) {
	// The policy name is always double-quoted (stored form from SHOW), so a
	// case-sensitively created policy matches.
	got := BuildApplyBackupPolicySql(`"DB"."SC".BS`, "my_policy")
	want := `ALTER BACKUP SET "DB"."SC".BS APPLY BACKUP POLICY "my_policy"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCreateBackupSetNameFQN(t *testing.T) {
	// Case-insensitive name is emitted bare so CREATE and the follow-up APPLY
	// reference the same (uppercased) stored name.
	if got := createBackupSetNameFQN("bs", "DB", "SC", false); got != `"DB"."SC".bs` {
		t.Errorf("bare case: got %q", got)
	}
	if got := createBackupSetNameFQN("bs", "DB", "SC", true); got != `"DB"."SC"."bs"` {
		t.Errorf("case-sensitive: got %q", got)
	}
	if got := createBackupSetNameFQN("bs", "", "", false); got != "bs" {
		t.Errorf("unqualified: got %q", got)
	}
}

func TestBuildCreateBackupSetSql(t *testing.T) {
	sql := BuildCreateBackupSetSql("BS", "DB", "SC", "database", `"DB"`, false, true, false)
	want := `CREATE BACKUP SET IF NOT EXISTS "DB"."SC".BS FOR DATABASE "DB"`
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}

	// orReplace suppresses IF NOT EXISTS and case-sensitive quotes the name.
	sql2 := BuildCreateBackupSetSql("bs", "", "", "schema", `"DB"."SC"`, true, true, true)
	want2 := `CREATE OR REPLACE BACKUP SET "bs" FOR SCHEMA "DB"."SC"`
	if sql2 != want2 {
		t.Errorf("got %q, want %q", sql2, want2)
	}
}

func TestBuildCreateBackupPolicySql(t *testing.T) {
	sql := BuildCreateBackupPolicySql("P1", "USING CRON 0 0 * * * UTC", 30, true, "note's", "", false, false, false)
	for _, want := range []string{
		"CREATE BACKUP POLICY P1",
		"WITH RETENTION LOCK",
		"SCHEDULE = 'USING CRON 0 0 * * * UTC'",
		"EXPIRE_AFTER_DAYS = 30",
		"COMMENT = 'note''s'",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected %q in SQL:\n%s", want, sql)
		}
	}

	// Free-text comment: a trailing backslash must be doubled (EscapeTextLit) so
	// it cannot escape the closing quote of the literal.
	back := BuildCreateBackupPolicySql("P2", "", 0, false, `C:\temp\`, "", false, false, false)
	if !strings.Contains(back, `COMMENT = 'C:\\temp\\'`) {
		t.Errorf("expected backslash-escaped comment, got:\n%s", back)
	}
}

func TestBuildRestoreFromBackupSql(t *testing.T) {
	sql, err := BuildRestoreFromBackupSql("table", `"DB"."SC"."T"`, "BS", "DB", "SC", "ab'cd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `CREATE TABLE "DB"."SC"."T" FROM BACKUP SET "DB"."SC"."BS" IDENTIFIER 'ab''cd'`
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}

	if _, err := BuildRestoreFromBackupSql("", "T", "BS", "", "", "id"); err == nil {
		t.Error("expected error for empty object type")
	}
	if _, err := BuildRestoreFromBackupSql("table", "", "BS", "", "", "id"); err == nil {
		t.Error("expected error for empty target name")
	}
	if _, err := BuildRestoreFromBackupSql("table", "T", "", "", "", "id"); err == nil {
		t.Error("expected error for empty backup set name")
	}
}

func TestBuildDeleteOldestBackupSql(t *testing.T) {
	sql := BuildDeleteOldestBackupSql("BS", "DB", "SC", "x'y")
	want := `ALTER BACKUP SET "DB"."SC"."BS" DELETE BACKUP IDENTIFIER 'x''y'`
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestParseBackupSets(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{
			"name", "database_name", "schema_name", "created_on", "object_kind",
			"object_name", "object_database_name", "object_schema_name", "status", "comment",
		},
		Rows: [][]interface{}{
			{"BS1", "BDB", "BSC", "2026-01-01", "DATABASE", "MYDB", "", "", "ACTIVE", "c1"},
			{"BS2", "BDB", "BSC", "2026-01-02", "TABLE", "OTHER", "X", "Y", "ACTIVE", "c2"},
		},
	}
	rows, err := ParseBackupSets(res, "database", "MYDB", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 matching backup set, got %d", len(rows))
	}
	if rows[0].Name != "BS1" || rows[0].ObjectType != "DATABASE" || rows[0].ObjectName != "MYDB" {
		t.Errorf("unexpected projection: %+v", rows[0])
	}

	if _, err := ParseBackupSets(res, "bogus", "", "", ""); err == nil {
		t.Error("expected error for unsupported scope")
	}
}

func TestParseBackupSetsEmptyLocation(t *testing.T) {
	// When SHOW returns an empty database_name, the set's own location must stay
	// empty — NOT be fabricated from the backed-up object's database — so the
	// follow-up FQN is a bare quoted name resolved against the session context.
	res := &snowflake.QueryResult{
		Columns: []string{
			"name", "database_name", "schema_name", "created_on", "object_kind",
			"object_name", "object_database_name", "object_schema_name", "status", "comment",
		},
		Rows: [][]interface{}{
			{"BS1", "", "", "2026-01-01", "DATABASE", "MYDB", "", "", "ACTIVE", ""},
		},
	}
	rows, err := ParseBackupSets(res, "database", "MYDB", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].BackupSetDb != "" || rows[0].BackupSetSchema != "" {
		t.Errorf("expected empty backup set location, got db=%q schema=%q", rows[0].BackupSetDb, rows[0].BackupSetSchema)
	}
}

func TestParseBackupPolicies(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{"name", "created_on", "owner", "schedule", "expire_after_days", "retention_lock", "comment"},
		Rows: [][]interface{}{
			{"P1", "2026-01-01", "SYSADMIN", "daily", int64(30), true, "note"},
		},
	}
	rows := ParseBackupPolicies(res)
	if len(rows) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(rows))
	}
	got := rows[0]
	if got.Name != "P1" || got.ExpireAfterDays != 30 || !got.RetentionLock {
		t.Errorf("unexpected projection: %+v", got)
	}
}

func TestParseBackups(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{"backup_id", "name", "created_on", "status", "size_bytes", "comment"},
		Rows: [][]interface{}{
			{"uuid-1", "bk-1", "2026-01-01", "DONE", int64(2048), "c"},
			{"uuid-2", "", "2026-01-02", "DONE", int64(0), ""}, // name falls back to created_on
		},
	}
	rows := ParseBackups(res)
	if len(rows) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(rows))
	}
	if rows[0].ID != "uuid-1" || rows[0].Name != "bk-1" || rows[0].SizeBytes != 2048 {
		t.Errorf("unexpected first row: %+v", rows[0])
	}
	if rows[1].Name != "2026-01-02" {
		t.Errorf("expected name fallback to created_on, got %q", rows[1].Name)
	}
}

func TestFindOldestEligibleBackup(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{"backup_id", "created_on", "is_under_legal_hold"},
		Rows: [][]interface{}{
			{"new", "2026-03-01", false},
			{"old-held", "2026-01-01", true}, // skipped: legal hold
			{"old", "2026-02-01", false},
		},
	}
	id, ok := FindOldestEligibleBackup(res)
	if !ok || id != "old" {
		t.Errorf("expected oldest eligible 'old', got %q ok=%v", id, ok)
	}

	allHeld := &snowflake.QueryResult{
		Columns: []string{"backup_id", "created_on", "is_under_legal_hold"},
		Rows:    [][]interface{}{{"a", "2026-01-01", true}},
	}
	if _, ok := FindOldestEligibleBackup(allHeld); ok {
		t.Error("expected no eligible backup when all under legal hold")
	}

	// Timestamps with differing UTC offsets must order by the actual instant, not
	// lexicographically — a case where the two orderings disagree:
	//   "instant_older": 2026-01-01T20:00:00+10:00 = 10:00 UTC (the older instant)
	//   "instant_newer": 2026-01-01T09:00:00-05:00 = 14:00 UTC
	// A lexicographic compare ranks "09…" below "20…" and would wrongly pick
	// "instant_newer" as oldest; the chronological compare must pick "instant_older".
	offsets := &snowflake.QueryResult{
		Columns: []string{"backup_id", "created_on", "is_under_legal_hold"},
		Rows: [][]interface{}{
			{"instant_newer", "2026-01-01T09:00:00-05:00", false},
			{"instant_older", "2026-01-01T20:00:00+10:00", false},
		},
	}
	if id, ok := FindOldestEligibleBackup(offsets); !ok || id != "instant_older" {
		t.Errorf("expected chronologically oldest 'instant_older', got %q ok=%v", id, ok)
	}
}
