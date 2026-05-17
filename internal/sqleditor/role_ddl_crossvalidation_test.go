// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package sqleditor

import (
	"fmt"
	"testing"

	sf "thaw/internal/snowflake"
)

// TestFormatRoleGrant_CrossValidation generates GRANT statements using
// snowflake.FormatRoleGrant (the same function used by GetRoleDDL) and
// validates each one through ValidateSnowflakePatterns. This ensures the
// DDL generator and the SQL validator agree on what constitutes valid syntax.
func TestFormatRoleGrant_CrossValidation(t *testing.T) {
	tests := []struct {
		name            string
		priv            string
		onType          string
		obj             string
		role            string
		withGrantOption bool
	}{
		// ── Account-level privileges ─────────────────────────────────────
		{"account CREATE ROLE", "CREATE ROLE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE USER", "CREATE USER", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE WAREHOUSE", "CREATE WAREHOUSE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE DATABASE", "CREATE DATABASE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE INTEGRATION", "CREATE INTEGRATION", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE NETWORK POLICY", "CREATE NETWORK POLICY", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account MANAGE GRANTS", "MANAGE GRANTS", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account MONITOR USAGE", "MONITOR USAGE", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account EXECUTE TASK", "EXECUTE TASK", "ACCOUNT", "ACCT1", "TASKADMIN", false},
		{"account EXECUTE ALERT", "EXECUTE ALERT", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account EXECUTE MANAGED TASK", "EXECUTE MANAGED TASK", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account IMPORT SHARE", "IMPORT SHARE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account OVERRIDE SHARE RESTRICTIONS", "OVERRIDE SHARE RESTRICTIONS", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account ATTACH POLICY", "ATTACH POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY MASKING POLICY", "APPLY MASKING POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY ROW ACCESS POLICY", "APPLY ROW ACCESS POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY SESSION POLICY", "APPLY SESSION POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY TAG", "APPLY TAG", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY AGGREGATION POLICY", "APPLY AGGREGATION POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account MANAGE WAREHOUSES", "MANAGE WAREHOUSES", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE SHARE", "CREATE SHARE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account APPLYBUDGET", "APPLYBUDGET", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account BIND SERVICE ENDPOINT", "BIND SERVICE ENDPOINT", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE COMPUTE POOL", "CREATE COMPUTE POOL", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE EXTERNAL VOLUME", "CREATE EXTERNAL VOLUME", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account MANAGE ACCOUNT SUPPORT CASES", "MANAGE ACCOUNT SUPPORT CASES", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account RESOLVE ALL", "RESOLVE ALL", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account with GRANT OPTION", "MANAGE GRANTS", "ACCOUNT", "ACCT1", "SECURITYADMIN", true},

		// ── Additional account-level privileges from Snowflake docs ──────
		{"account APPLY AUTHENTICATION POLICY", "APPLY AUTHENTICATION POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY PACKAGES POLICY", "APPLY PACKAGES POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account APPLY PASSWORD POLICY", "APPLY PASSWORD POLICY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account CANCEL QUERY", "CANCEL QUERY", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account EXECUTE DATA METRIC FUNCTION", "EXECUTE DATA METRIC FUNCTION", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account EXECUTE MANAGED ALERT", "EXECUTE MANAGED ALERT", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE APPLICATION", "CREATE APPLICATION", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE APPLICATION PACKAGE", "CREATE APPLICATION PACKAGE", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE FAILOVER GROUP", "CREATE FAILOVER GROUP", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account CREATE REPLICATION GROUP", "CREATE REPLICATION GROUP", "ACCOUNT", "ACCT1", "SYSADMIN", false},
		{"account MANAGE EVENT SHARING", "MANAGE EVENT SHARING", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account MONITOR EXECUTION", "MONITOR EXECUTION", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},
		{"account MONITOR SECURITY", "MONITOR SECURITY", "ACCOUNT", "ACCT1", "SECURITYADMIN", false},
		{"account PURCHASE DATA EXCHANGE LISTING", "PURCHASE DATA EXCHANGE LISTING", "ACCOUNT", "ACCT1", "ACCOUNTADMIN", false},

		// ── Non-account object types ─────────────────────────────────────
		{"table SELECT", "SELECT", "TABLE", `"DB"."SCH"."TBL"`, "ANALYST", false},
		{"table INSERT with grant option", "INSERT", "TABLE", `"DB"."SCH"."TBL"`, "WRITER", true},
		{"view SELECT", "SELECT", "VIEW", `"DB"."SCH"."VW"`, "ANALYST", false},
		{"warehouse USAGE", "USAGE", "WAREHOUSE", `"COMPUTE_WH"`, "DEV", false},
		{"database USAGE", "USAGE", "DATABASE", `"MY_DB"`, "READER", false},
		{"schema CREATE TABLE", "CREATE TABLE", "SCHEMA", `"DB"."SCH"`, "DEV", false},
		{"integration USAGE", "USAGE", "INTEGRATION", `"MY_INT"`, "ETL", false},
		{"stage READ", "READ", "STAGE", `"DB"."SCH"."MY_STAGE"`, "LOADER", false},
		{"stream SELECT", "SELECT", "STREAM", `"DB"."SCH"."MY_STREAM"`, "READER", false},
		{"task MONITOR", "MONITOR", "TASK", `"DB"."SCH"."MY_TASK"`, "OPS", false},
		{"pipe OPERATE", "OPERATE", "PIPE", `"DB"."SCH"."MY_PIPE"`, "OPS", false},
		{"user MONITOR", "MONITOR", "USER", `"MY_USER"`, "ADMIN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := sf.FormatRoleGrant(tt.priv, tt.onType, tt.obj, tt.role, tt.withGrantOption)
			// Strip trailing semicolon for validator (ValidateSnowflakePatterns
			// works with the raw statement text without trailing semicolons,
			// but GetStatementRanges handles them).
			ranges := GetStatementRanges(stmt)
			markers := ValidateSnowflakePatterns(stmt, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("FormatRoleGrant() produced invalid SQL according to validator:\n  SQL:      %s\n  Warnings: %v",
					stmt, warnings)
			}
		})
	}
}

// TestFormatRoleGrant_AccountOmitsObjectName verifies that account-level
// grants produced by FormatRoleGrant never include the account identifier
// after ON ACCOUNT. This was the original bug from issue #265.
func TestFormatRoleGrant_AccountOmitsObjectName(t *testing.T) {
	accountNames := []string{"WC16727", "MY_ACCT", "PROD123", ""}
	privs := []string{"MANAGE GRANTS", "CREATE DATABASE", "EXECUTE TASK"}

	for _, acct := range accountNames {
		for _, priv := range privs {
			name := fmt.Sprintf("%s/acct=%s", priv, acct)
			t.Run(name, func(t *testing.T) {
				stmt := sf.FormatRoleGrant(priv, "ACCOUNT", acct, "TEST_ROLE", false)
				expected := fmt.Sprintf(`GRANT %s ON ACCOUNT TO ROLE "TEST_ROLE";`, priv)
				if stmt != expected {
					t.Errorf("Account name leaked into statement:\n  got:  %s\n  want: %s", stmt, expected)
				}

				// Cross-validate with the SQL pattern validator.
				ranges := GetStatementRanges(stmt)
				markers := ValidateSnowflakePatterns(stmt, ranges)
				warnings := getWarnings(markers)
				if len(warnings) > 0 {
					t.Errorf("Validator flagged account-level grant as invalid:\n  SQL:      %s\n  Warnings: %v",
						stmt, warnings)
				}
			})
		}
	}
}
