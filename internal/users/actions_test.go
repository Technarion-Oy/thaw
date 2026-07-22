// SPDX-License-Identifier: GPL-3.0-or-later

package users

import "testing"

func TestBuildResetPasswordSQL(t *testing.T) {
	got, err := BuildResetPasswordSQL("ALICE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `ALTER USER "ALICE" RESET PASSWORD`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if _, err := BuildResetPasswordSQL("  "); err == nil {
		t.Error("expected error for blank name")
	}
}

func TestBuildRenameUserSQL(t *testing.T) {
	tests := []struct {
		name, newName, want string
		wantErr             bool
	}{
		// bare target folds; a quoted name keeps its exact case. A name with a
		// space must be typed quoted (same SQL-syntax model as DEFAULT_NAMESPACE).
		{"ALICE", "bob", `ALTER USER "ALICE" RENAME TO bob`, false},
		{"ALICE", `"New Name"`, `ALTER USER "ALICE" RENAME TO "New Name"`, false},
		{"ALICE", `"Exact"`, `ALTER USER "ALICE" RENAME TO "Exact"`, false},
		{"ALICE", "New Name", "", true}, // unquoted name with a space is invalid
		{"ALICE", "", "", true},
		{"ALICE", "a.b", "", true}, // rename target is a single identifier
		{"", "bob", "", true},
	}
	for _, tt := range tests {
		got, err := BuildRenameUserSQL(tt.name, tt.newName)
		if tt.wantErr {
			if err == nil {
				t.Errorf("rename %q→%q: expected error, got %q", tt.name, tt.newName, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("rename %q→%q: unexpected error: %v", tt.name, tt.newName, err)
			continue
		}
		if got != tt.want {
			t.Errorf("rename %q→%q:\n got %q\nwant %q", tt.name, tt.newName, got, tt.want)
		}
	}
}

func TestBuildAbortAllQueriesSQL(t *testing.T) {
	got, err := BuildAbortAllQueriesSQL("ALICE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `ALTER USER "ALICE" ABORT ALL QUERIES`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if _, err := BuildAbortAllQueriesSQL(""); err == nil {
		t.Error("expected error for blank name")
	}
}

func TestBuildRemoveMfaMethodSQL(t *testing.T) {
	tests := []struct {
		method, want string
		wantErr      bool
	}{
		{"TOTP", `ALTER USER "ALICE" REMOVE MFA METHOD TOTP`, false},
		{"passkey", `ALTER USER "ALICE" REMOVE MFA METHOD PASSKEY`, false},
		{"DUO", `ALTER USER "ALICE" REMOVE MFA METHOD DUO`, false},
		{"SMS", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := BuildRemoveMfaMethodSQL("ALICE", tt.method)
		if tt.wantErr {
			if err == nil {
				t.Errorf("method %q: expected error, got %q", tt.method, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("method %q: unexpected error: %v", tt.method, err)
			continue
		}
		if got != tt.want {
			t.Errorf("method %q:\n got %q\nwant %q", tt.method, got, tt.want)
		}
	}
}

func TestBuildSetPolicySQL(t *testing.T) {
	tests := []struct {
		kind, policy string
		force        bool
		want         string
		wantErr      bool
	}{
		{"AUTHENTICATION", "corp_auth", false, `ALTER USER "ALICE" SET AUTHENTICATION POLICY corp_auth`, false},
		{"password", "MY_DB.SEC.pw_policy", true, `ALTER USER "ALICE" SET PASSWORD POLICY MY_DB.SEC.pw_policy FORCE`, false},
		{"SESSION", `"My Policy"`, false, `ALTER USER "ALICE" SET SESSION POLICY "My Policy"`, false},
		{"BOGUS", "p", false, "", true},
		{"SESSION", "", false, "", true},
		{"SESSION", "a.b.c.d", false, "", true}, // too many parts
	}
	for _, tt := range tests {
		got, err := BuildSetPolicySQL("ALICE", tt.kind, tt.policy, tt.force)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s/%s: expected error, got %q", tt.kind, tt.policy, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s/%s: unexpected error: %v", tt.kind, tt.policy, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s/%s:\n got %q\nwant %q", tt.kind, tt.policy, got, tt.want)
		}
	}
}

func TestBuildUnsetPolicySQL(t *testing.T) {
	got, err := BuildUnsetPolicySQL("ALICE", "SESSION")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `ALTER USER "ALICE" UNSET SESSION POLICY`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if _, err := BuildUnsetPolicySQL("ALICE", "NOPE"); err == nil {
		t.Error("expected error for bad kind")
	}
}

func TestBuildSetTagsSQL(t *testing.T) {
	got, err := BuildSetTagsSQL("ALICE", []TagPair{
		{Name: "cost_center", Value: "eng"},
		{Name: "DB.S.owner", Value: "O'Neil"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER USER "ALICE" SET TAG cost_center = 'eng', DB.S.owner = 'O''Neil'`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	if _, err := BuildSetTagsSQL("ALICE", nil); err == nil {
		t.Error("expected error for no tags")
	}
	if _, err := BuildSetTagsSQL("ALICE", []TagPair{{Name: "", Value: "x"}}); err == nil {
		t.Error("expected error for blank tag name")
	}
}

func TestBuildUnsetTagsSQL(t *testing.T) {
	got, err := BuildUnsetTagsSQL("ALICE", []string{"cost_center", `"My Tag"`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER USER "ALICE" UNSET TAG cost_center, "My Tag"`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	if _, err := BuildUnsetTagsSQL("ALICE", nil); err == nil {
		t.Error("expected error for no tags")
	}
}

func TestBuildAddDelegatedAuthSQL(t *testing.T) {
	got, err := BuildAddDelegatedAuthSQL("ALICE", "my_role", "oauth_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER USER "ALICE" ADD DELEGATED AUTHORIZATION OF ROLE my_role TO SECURITY INTEGRATION oauth_int`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	if _, err := BuildAddDelegatedAuthSQL("ALICE", "", "oauth_int"); err == nil {
		t.Error("expected error for blank role")
	}
	if _, err := BuildAddDelegatedAuthSQL("ALICE", "r", ""); err == nil {
		t.Error("expected error for blank integration")
	}
}

func TestBuildRemoveDelegatedAuthSQL(t *testing.T) {
	// with a role → single-authorization form
	got, err := BuildRemoveDelegatedAuthSQL("ALICE", "my_role", "oauth_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER USER "ALICE" REMOVE DELEGATED AUTHORIZATION OF ROLE my_role FROM SECURITY INTEGRATION oauth_int`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	// empty role → all-authorizations form
	got, err = BuildRemoveDelegatedAuthSQL("ALICE", "", "oauth_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = `ALTER USER "ALICE" REMOVE DELEGATED AUTHORIZATIONS FROM SECURITY INTEGRATION oauth_int`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	if _, err := BuildRemoveDelegatedAuthSQL("ALICE", "r", ""); err == nil {
		t.Error("expected error for blank integration")
	}
}
