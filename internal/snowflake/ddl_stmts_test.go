// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import "testing"

func TestNormalizeDropMode(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"CASCADE", "CASCADE"},
		{"RESTRICT", "RESTRICT"},
		{"", "CASCADE"},         // empty defaults to CASCADE
		{"cascade", "CASCADE"},  // not the exact keyword → default
		{"restrict", "CASCADE"}, // case-sensitive: only "RESTRICT" honored
		{"DROP", "CASCADE"},     // junk defaults to CASCADE
	}
	for _, tc := range tests {
		if got := normalizeDropMode(tc.in); got != tc.want {
			t.Errorf("normalizeDropMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestDropShowStmtQuoting locks the bug-class this PR fixed: the object name in
// each DROP/SHOW statement must be double-quoted (QuoteIdent/Qualify), never
// emitted bare. The case-sensitive, reserved-keyword, and embedded-quote inputs
// would each break or mis-resolve if a builder regressed to a bare identifier.
func TestDropShowStmtQuoting(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		// DROP
		{"drop integration plain", dropIntegrationStmt("MY_INT"), `DROP INTEGRATION "MY_INT"`},
		{"drop integration mixed case", dropIntegrationStmt("MyInt"), `DROP INTEGRATION "MyInt"`},
		{"drop integration embedded quote", dropIntegrationStmt(`we"ird`), `DROP INTEGRATION "we""ird"`},
		{"drop database cascade", dropDatabaseStmt("DB", "CASCADE"), `DROP DATABASE "DB" CASCADE`},
		{"drop database restrict", dropDatabaseStmt("DB", "RESTRICT"), `DROP DATABASE "DB" RESTRICT`},
		{"drop database reserved name", dropDatabaseStmt("select", ""), `DROP DATABASE "select" CASCADE`},
		{"drop schema two-part", dropSchemaStmt("DB", "S", "RESTRICT"), `DROP SCHEMA "DB"."S" RESTRICT`},
		{"drop schema mixed case default mode", dropSchemaStmt("Db", "My Schema", "x"), `DROP SCHEMA "Db"."My Schema" CASCADE`},
		// SHOW
		{"show grants to role", showGrantsToRoleStmt("ANALYST"), `SHOW GRANTS TO ROLE "ANALYST"`},
		{"show grants to role reserved", showGrantsToRoleStmt("order"), `SHOW GRANTS TO ROLE "order"`},
		{"show grants on role", showGrantsOnRoleStmt("ANALYST"), `SHOW GRANTS ON ROLE "ANALYST"`},
		{"show schemas history", showSchemasHistoryStmt("MY_DB"), `SHOW SCHEMAS HISTORY IN DATABASE "MY_DB"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
