// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package contact

import (
	"strings"
	"testing"
)

func TestBuildCreateContactSql_Users(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:    "support_contact",
		Method:  MethodUsers,
		Users:   []string{"ALICE", "BOB"},
		Comment: "support team",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "CREATE CONTACT \"DB\".\"SC\".support_contact\n" +
		"  USERS = ('ALICE', 'BOB')\n" +
		"  COMMENT = 'support team';"
	if sql != want {
		t.Errorf("got:\n%s\nwant:\n%s", sql, want)
	}
}

func TestBuildCreateContactSql_Email(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:   "alerts",
		Method: MethodEmail,
		Email:  "ops@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "CREATE CONTACT \"DB\".\"SC\".alerts\n" +
		"  EMAIL_DISTRIBUTION_LIST = 'ops@example.com';"
	if sql != want {
		t.Errorf("got:\n%s\nwant:\n%s", sql, want)
	}
}

func TestBuildCreateContactSql_URL(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:   "oncall",
		Method: MethodURL,
		URL:    "https://example.com/oncall",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "URL = 'https://example.com/oncall'") {
		t.Errorf("URL clause missing: %s", sql)
	}
}

func TestBuildCreateContactSql_OrReplaceWinsOverIfNotExists(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:        "c",
		OrReplace:   true,
		IfNotExists: true,
		Method:      MethodEmail,
		Email:       "x@y.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "CREATE OR REPLACE CONTACT ") {
		t.Errorf("expected OR REPLACE prefix: %s", sql)
	}
	if strings.Contains(sql, "IF NOT EXISTS") {
		t.Errorf("IF NOT EXISTS should be dropped when OR REPLACE is set: %s", sql)
	}
}

func TestBuildCreateContactSql_CaseSensitiveName(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:          "MyContact",
		CaseSensitive: true,
		Method:        MethodEmail,
		Email:         "x@y.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "\"MyContact\"") {
		t.Errorf("expected quoted case-sensitive name: %s", sql)
	}
}

// A blank name yields a completable placeholder; an empty/blank method emits no
// method clause so the preview is still valid template SQL.
func TestBuildCreateContactSql_PlaceholdersAndNoMethod(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "contact_name") {
		t.Errorf("expected placeholder name: %s", sql)
	}
	for _, frag := range []string{"USERS", "EMAIL_DISTRIBUTION_LIST", "URL", "COMMENT"} {
		if strings.Contains(sql, frag) {
			t.Errorf("expected no %s clause for empty config: %s", frag, sql)
		}
	}
}

// A method selected but with no value yet must not emit an empty clause.
func TestBuildCreateContactSql_SelectedMethodNoValue(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{Name: "c", Method: MethodUsers})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sql, "USERS") {
		t.Errorf("expected no USERS clause when user list is empty: %s", sql)
	}
}

func TestFormatContactUsers(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"ALICE", "BOB"}, "('ALICE', 'BOB')"},
		{[]string{" ALICE ", "", "BOB"}, "('ALICE', 'BOB')"}, // trims + drops blanks
		{[]string{"o'brien"}, "('o''brien')"},                // escapes single quote
		{nil, "()"},
	}
	for _, c := range cases {
		if got := FormatContactUsers(c.in); got != c.want {
			t.Errorf("FormatContactUsers(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildCreateContactSql_CommentEscaped(t *testing.T) {
	sql, err := BuildCreateContactSql("DB", "SC", ContactConfig{
		Name:    "c",
		Method:  MethodEmail,
		Email:   "x@y.com",
		Comment: "it's fine",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "COMMENT = 'it''s fine'") {
		t.Errorf("expected escaped comment: %s", sql)
	}
}
