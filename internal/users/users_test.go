// SPDX-License-Identifier: GPL-3.0-or-later

package users

import "testing"

func TestBuildAlterUserPropertySQL(t *testing.T) {
	tests := []struct {
		property string
		value    string
		want     string
		wantErr  bool
	}{
		// strings — quoted + escaped (text-literal rules: backslash doubled), empty → UNSET
		{"comment", "O'Neil", `ALTER USER "ALICE" SET COMMENT = 'O''Neil'`, false},
		{"comment", `ends in \`, `ALTER USER "ALICE" SET COMMENT = 'ends in \\'`, false},
		{"password", `p\w`, `ALTER USER "ALICE" SET PASSWORD = 'p\\w'`, false},
		{"comment", "", `ALTER USER "ALICE" UNSET COMMENT`, false},
		{"email", "a@x.io", `ALTER USER "ALICE" SET EMAIL = 'a@x.io'`, false},
		{"middleName", "Q", `ALTER USER "ALICE" SET MIDDLE_NAME = 'Q'`, false},
		// identifiers — Select-sourced values quoted exactly; empty → UNSET
		{"defaultWarehouse", "WH", `ALTER USER "ALICE" SET DEFAULT_WAREHOUSE = "WH"`, false},
		// networkPolicy is typed free-hand: bare names stay bare so Snowflake
		// folds them; names needing quoting are quoted
		{"networkPolicy", "", `ALTER USER "ALICE" UNSET NETWORK_POLICY`, false},
		{"networkPolicy", "corp_policy", `ALTER USER "ALICE" SET NETWORK_POLICY = corp_policy`, false},
		{"networkPolicy", "My Policy", `ALTER USER "ALICE" SET NETWORK_POLICY = "My Policy"`, false},
		// namespace — bare parts stay bare (identifier folding); quoted parts
		// keep exact case; empty segments rejected
		{"defaultNamespace", "analytics.public", `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = analytics.public`, false},
		{"defaultNamespace", "DB", `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = DB`, false},
		{"defaultNamespace", "DB.", "", true},
		{"defaultNamespace", ".SCHEMA", "", true},
		{"defaultNamespace", "A.B.C", "", true},
		// quote-aware: a quoted identifier containing a dot stays one part
		{"defaultNamespace", `"MY.DB".PUB`, `ALTER USER "ALICE" SET DEFAULT_NAMESPACE = "MY.DB".PUB`, false},
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
		// RSA public keys — stripped base64 SET, empty → UNSET, both slots
		{"rsaPublicKey", "MIIBIjANBgkq", `ALTER USER "ALICE" SET RSA_PUBLIC_KEY = 'MIIBIjANBgkq'`, false},
		{"rsaPublicKey", "", `ALTER USER "ALICE" UNSET RSA_PUBLIC_KEY`, false},
		{"rsaPublicKey2", "MIIBIjANBgkq", `ALTER USER "ALICE" SET RSA_PUBLIC_KEY_2 = 'MIIBIjANBgkq'`, false},
		{"rsaPublicKey2", "", `ALTER USER "ALICE" UNSET RSA_PUBLIC_KEY_2`, false},
		// whitespace/newlines inside a pasted key are stripped
		{"rsaPublicKey", "MIIBIjAN\n  BgkqhkiG\n", `ALTER USER "ALICE" SET RSA_PUBLIC_KEY = 'MIIBIjANBgkqhkiG'`, false},
		// trailing base64 padding is allowed
		{"rsaPublicKey", "MIIBIjAN==", `ALTER USER "ALICE" SET RSA_PUBLIC_KEY = 'MIIBIjAN=='`, false},
		// non-base64 characters are rejected — the value is fed by a free-form
		// paste UI, so a quote/backslash must never reach the SQL literal
		{"rsaPublicKey", "MIIB'key", "", true},
		{"rsaPublicKey", `abc\',DISABLED=FALSE,COMMENT='pwned`, "", true},
		// full-PEM input is rejected — the -----BEGIN/-----END lines would break the literal
		{"rsaPublicKey", "-----BEGIN PUBLIC KEY-----\nMIIBIjAN\n-----END PUBLIC KEY-----", "", true},
		{"rsaPublicKey2", "-----BEGIN PUBLIC KEY-----\nMIIBIjAN\n-----END PUBLIC KEY-----", "", true},
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
