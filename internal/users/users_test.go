// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package users

import "testing"

func TestBuildAlterUserPropertySQL(t *testing.T) {
	tests := []struct {
		property string
		value    string
		want     string
		wantErr  bool
	}{
		// strings — quoted + escaped, empty → UNSET
		{"comment", "O'Neil", `ALTER USER "ALICE" SET COMMENT = 'O''Neil'`, false},
		{"comment", "", `ALTER USER "ALICE" UNSET COMMENT`, false},
		{"email", "a@x.io", `ALTER USER "ALICE" SET EMAIL = 'a@x.io'`, false},
		{"middleName", "Q", `ALTER USER "ALICE" SET MIDDLE_NAME = 'Q'`, false},
		// identifiers — quoted, empty → UNSET
		{"defaultWarehouse", "WH", `ALTER USER "ALICE" SET DEFAULT_WAREHOUSE = "WH"`, false},
		{"networkPolicy", "", `ALTER USER "ALICE" UNSET NETWORK_POLICY`, false},
		// namespace — each dotted part quoted separately; empty segments rejected
		{"defaultNamespace", "DB.PUB", `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = "DB"."PUB"`, false},
		{"defaultNamespace", "DB", `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = "DB"`, false},
		{"defaultNamespace", "DB.", "", true},
		{"defaultNamespace", ".SCHEMA", "", true},
		{"defaultNamespace", "A.B.C", "", true},
		// quote-aware: a quoted identifier containing a dot stays one part
		{"defaultNamespace", `"MY.DB".PUB`, `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = "MY.DB"."PUB"`, false},
		{"defaultNamespace", `"MY""DB"`, `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = "MY""DB"`, false},
		{"defaultNamespace", `"UNBALANCED`, "", true},
		// integers — validated, empty → UNSET
		{"minsToBypassMfa", "30", `ALTER USER "ALICE" SET MINS_TO_BYPASS_MFA = 30`, false},
		{"minsToBypassMfa", "", `ALTER USER "ALICE" UNSET MINS_TO_BYPASS_MFA`, false},
		{"minsToUnlock", "abc", "", true},
		{"daysToExpiry", "-1", "", true},
		// booleans — TRUE/FALSE only, no UNSET
		{"disabled", "TRUE", `ALTER USER "ALICE" SET DISABLED = TRUE`, false},
		{"mustChangePassword", "true", `ALTER USER "ALICE" SET MUST_CHANGE_PASSWORD = TRUE`, false},
		{"mustChangePassword", "", "", true},
		// enums
		{"type", "SERVICE", `ALTER USER "ALICE" SET TYPE = SERVICE`, false},
		{"type", "", `ALTER USER "ALICE" UNSET TYPE`, false},
		{"type", "ROBOT", "", true},
		{"defaultSecondaryRoles", "ALL", `ALTER USER "ALICE" SET DEFAULT_SECONDARY_ROLES = ('ALL')`, false},
		{"defaultSecondaryRoles", "NONE", `ALTER USER "ALICE" SET DEFAULT_SECONDARY_ROLES = ()`, false},
		{"defaultSecondaryRoles", "", `ALTER USER "ALICE" UNSET DEFAULT_SECONDARY_ROLES`, false},
		// password — set-only, never trimmed, never UNSET
		{"password", " p'w ", `ALTER USER "ALICE" SET PASSWORD = ' p''w '`, false},
		{"password", "", "", true},
		// unknown
		{"nope", "x", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.property+"="+tt.value, func(t *testing.T) {
			got, err := BuildAlterUserPropertySQL("ALICE", tt.property, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}
