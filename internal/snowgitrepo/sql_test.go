// SPDX-License-Identifier: GPL-3.0-or-later

package snowgitrepo

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertNotContains(t *testing.T, sql, substr string) {
	t.Helper()
	if strings.Contains(sql, substr) {
		t.Errorf("expected SQL NOT to contain %q\nSQL:\n%s", substr, sql)
	}
}

// ── BuildCreateGitRepositorySql ───────────────────────────────────────────────

func TestBuildCreateGitRepositorySql_BasicRequired(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
	}
	sql, err := BuildCreateGitRepositorySql("MYDB", "MYSCHEMA", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE GIT REPOSITORY`)
	assertContains(t, sql, `"MYDB"."MYSCHEMA".MY_REPO`)
	assertContains(t, sql, `ORIGIN = 'https://github.com/org/repo.git'`)
	assertContains(t, sql, `API_INTEGRATION = "MY_API_INT"`)
	assertNotContains(t, sql, `GIT_CREDENTIALS`)
	assertNotContains(t, sql, `COMMENT`)
	assertNotContains(t, sql, `WITH TAG`)
	assertNotContains(t, sql, `OR REPLACE`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
}

func TestBuildCreateGitRepositorySql_OrReplace(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		OrReplace:      true,
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
	}
	sql, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE OR REPLACE GIT REPOSITORY`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
}

func TestBuildCreateGitRepositorySql_IfNotExists(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		IfNotExists:    true,
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
	}
	sql, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE GIT REPOSITORY IF NOT EXISTS`)
}

func TestBuildCreateGitRepositorySql_OrReplace_IfNotExists_Ignored(t *testing.T) {
	// OR REPLACE takes precedence; IF NOT EXISTS must not appear
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		OrReplace:      true,
		IfNotExists:    true,
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
	}
	sql, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE OR REPLACE GIT REPOSITORY`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
}

func TestBuildCreateGitRepositorySql_WithCredentialsCommentTags(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
		GitCredentials: `"DB"."SC"."MY_SECRET"`,
		Comment:        "repo comment",
		Tags: []snowflake.TagPair{
			{Name: "env", Value: "prod"},
			{Name: "team", Value: "data"},
		},
	}
	sql, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `GIT_CREDENTIALS = "DB"."SC"."MY_SECRET"`)
	assertContains(t, sql, `COMMENT = 'repo comment'`)
	assertContains(t, sql, `WITH TAG (`)
	assertContains(t, sql, `"env" = 'prod'`)
	assertContains(t, sql, `"team" = 'data'`)
}

func TestBuildCreateGitRepositorySql_EmptyTags_Skipped(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		OriginUrl:      "https://github.com/org/repo.git",
		ApiIntegration: "MY_API_INT",
		Tags:           []snowflake.TagPair{{Name: "", Value: "ignored"}},
	}
	sql, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNotContains(t, sql, `WITH TAG`)
}

func TestBuildCreateGitRepositorySql_MissingOriginUrl(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:           "MY_REPO",
		ApiIntegration: "MY_API_INT",
	}
	_, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err == nil {
		t.Fatal("expected error for missing OriginUrl")
	}
}

func TestBuildCreateGitRepositorySql_MissingApiIntegration(t *testing.T) {
	cfg := GitRepositoryConfig{
		Name:      "MY_REPO",
		OriginUrl: "https://github.com/org/repo.git",
	}
	_, err := BuildCreateGitRepositorySql("DB", "SC", cfg)
	if err == nil {
		t.Fatal("expected error for missing ApiIntegration")
	}
}

// ── BuildModifyGitRepositorySql ───────────────────────────────────────────────

func TestBuildModifyGitRepositorySql_ChangeIntegration(t *testing.T) {
	cfg := GitRepositoryConfig{
		ApiIntegration: "NEW_INT",
		GitCredentials: `"DB"."SC"."SECRET"`,
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "", "OLD_INT", `"DB"."SC"."SECRET"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `ALTER GIT REPOSITORY`)
	assertContains(t, stmts[0], `API_INTEGRATION = "NEW_INT"`)
	assertNotContains(t, stmts[0], `GIT_CREDENTIALS`) // unchanged
}

func TestBuildModifyGitRepositorySql_SetCredentials(t *testing.T) {
	cfg := GitRepositoryConfig{
		ApiIntegration: "INT",
		GitCredentials: `"DB"."SC"."NEW_SECRET"`,
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "", "INT", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	assertContains(t, stmts[0], `GIT_CREDENTIALS = "DB"."SC"."NEW_SECRET"`)
}

func TestBuildModifyGitRepositorySql_UnsetCredentials(t *testing.T) {
	cfg := GitRepositoryConfig{
		ApiIntegration: "INT",
		GitCredentials: "", // cleared
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "", "INT", `"DB"."SC"."OLD_SECRET"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 UNSET statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `UNSET GIT_CREDENTIALS`)
}

func TestBuildModifyGitRepositorySql_UnsetComment(t *testing.T) {
	cfg := GitRepositoryConfig{
		ApiIntegration: "INT",
		Comment:        "", // cleared
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "old comment", "INT", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 UNSET statement, got %d", len(stmts))
	}
	assertContains(t, stmts[0], `UNSET COMMENT`)
}

func TestBuildModifyGitRepositorySql_SetAndUnset(t *testing.T) {
	// Changing integration AND clearing credentials → SET + UNSET = two statements
	cfg := GitRepositoryConfig{
		ApiIntegration: "NEW_INT",
		GitCredentials: "", // cleared
		Comment:        "", // cleared
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "old comment", "OLD_INT", `"DB"."SC"."SECRET"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements (SET + UNSET), got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `SET`)
	assertContains(t, stmts[0], `API_INTEGRATION = "NEW_INT"`)
	assertContains(t, stmts[1], `UNSET`)
	assertContains(t, stmts[1], `GIT_CREDENTIALS`)
	assertContains(t, stmts[1], `COMMENT`)
}

func TestBuildModifyGitRepositorySql_NoChanges(t *testing.T) {
	cfg := GitRepositoryConfig{
		ApiIntegration: "INT",
		GitCredentials: `"DB"."SC"."SECRET"`,
		Comment:        "same comment",
	}
	stmts, err := BuildModifyGitRepositorySql("DB", "SC", "REPO", cfg, "same comment", "INT", `"DB"."SC"."SECRET"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Comment is set in SET because it's non-empty; integration and credentials unchanged
	// so only COMMENT SET is produced
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement (comment SET), got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `COMMENT = 'same comment'`)
}
