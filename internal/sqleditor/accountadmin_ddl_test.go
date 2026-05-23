// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package sqleditor

import (
	"fmt"
	"strings"
	"testing"
)

// accountAdminDDL is a real-world ACCOUNTADMIN role DDL export from a live
// Snowflake account, used to cross-validate the GRANT statement validator.
const accountAdminDDL = `CREATE ROLE IF NOT EXISTS "ACCOUNTADMIN"
  COMMENT = 'Account administrator can manage all aspects of the account.';
GRANT APPLY BACKUP RETENTION LOCK ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY CONTACT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY LEGAL HOLD ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY MASKING POLICY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY RESOURCE GROUP ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY STORAGE LIFECYCLE POLICY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT APPLY TAG ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT AUDIT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CANCEL QUERY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE ACCOUNT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE API INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE APPLICATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE APPLICATION PACKAGE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE EXTERNAL ACCESS INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE EXTERNAL VOLUME ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE FAILOVER GROUP ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE LISTING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE MIGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE OPENFLOW DATA PLANE INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE OPENFLOW RUNTIME INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE ORGANIZATION LISTING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE POSTGRES INSTANCE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE PREVIEW APPLICATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE PROVISIONED THROUGHPUT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE REPLICATION GROUP ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE SECURITY INTEGRATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE SHARE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE SNOWFLAKE INTELLIGENCE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT CREATE UPSTREAM REPOSITORY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT DELETE LINEAGE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE ALERT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE AUTO CLASSIFICATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE DATA METRIC FUNCTION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE MANAGED ALERT ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE MANAGED TASK ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE SPARK APPLICATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT EXECUTE TASK ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT IMPORT ORGANIZATION LISTING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT IMPORT ORGANIZATION USER GROUPS ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT IMPORT SHARE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT INGEST LINEAGE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE ACCOUNT SUPPORT CASES ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE BILLING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE DATA QUALITY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE EVENT SHARING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE FIREWALL_CONFIGURATION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE POSTGRES PRIVATE CONNECTIVITY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE SHARE TARGET ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE USER SUPPORT CASES ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MANAGE WAREHOUSES ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY LOG EVENT LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY LOG LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY METRIC LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY SESSION LOG EVENT LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY SESSION LOG LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY SESSION METRIC LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY SESSION TRACE LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MODIFY TRACE LEVEL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR EXECUTION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR ROLE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR SECURITY ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR USAGE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT MONITOR USER ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT OVERRIDE SHARE RESTRICTIONS ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT PURCHASE DATA EXCHANGE LISTING ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT READ SESSION ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT READ UNREDACTED AI OBSERVABILITY EVENTS TABLE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT READ UNREDACTED ERROR TABLE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT REPLICATE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT RESOLVE ALL ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT USE AI FUNCTIONS ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT VIEW LINEAGE ON ACCOUNT TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT USAGE ON APPLICATION_ROLE SNOWFLAKE.DATA_QUALITY_MONITORING_LOOKUP TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT USAGE ON APPLICATION_ROLE SNOWFLAKE.PERFORMANCE_EXPLORER_PUBLIC_USER TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT USAGE ON APPLICATION_ROLE SNOWFLAKE.PUBLIC TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT INSTANTIATE ON CLASS SNOWFLAKE.CORE.BUDGET TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.CORE.BUDGET TO ROLE "ACCOUNTADMIN";
GRANT INSTANTIATE ON CLASS SNOWFLAKE.CORE.QUOTA TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.CORE.QUOTA TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.MARKETPLACE_NOTIFICATION.MARKETPLACE_ANALYTICS_NOTIFICATION TO ROLE "ACCOUNTADMIN";
GRANT INSTANTIATE ON CLASS SNOWFLAKE.ML.DOCUMENT_INTELLIGENCE TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.ML.DOCUMENT_INTELLIGENCE TO ROLE "ACCOUNTADMIN";
GRANT INSTANTIATE ON CLASS SNOWFLAKE.WORKLOAD_INSIGHTS.ANOMALY_INSIGHTS TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.WORKLOAD_INSIGHTS.ANOMALY_INSIGHTS TO ROLE "ACCOUNTADMIN";
GRANT INSTANTIATE ON CLASS SNOWFLAKE.WORKLOAD_INSIGHTS.COST_INSIGHTS TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.WORKLOAD_INSIGHTS.COST_INSIGHTS TO ROLE "ACCOUNTADMIN";
GRANT INSTANTIATE ON CLASS SNOWFLAKE.WORKLOAD_OPTIMIZATION.PERFORMANCE_EXPLORER TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON CLASS SNOWFLAKE.WORKLOAD_OPTIMIZATION.PERFORMANCE_EXPLORER TO ROLE "ACCOUNTADMIN";
GRANT OWNERSHIP ON COMPUTE_POOL SYSTEM_COMPUTE_POOL_CPU TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT OWNERSHIP ON COMPUTE_POOL SYSTEM_COMPUTE_POOL_GPU TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT REFERENCE_USAGE ON DATABASE SNOWFLAKE TO ROLE "ACCOUNTADMIN";
GRANT OWNERSHIP ON DATABASE SNOWFLAKE_LEARNING_DB TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT OWNERSHIP ON DATABASE SNOWFLAKE_SAMPLE_DATA TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT USAGE ON DATABASE_ROLE SNOWFLAKE.AI_FUNCTIONS_USER TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON DATABASE_ROLE SNOWFLAKE.ALERT_VIEWER TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON FUNCTION SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES_WITH_LINEAGE(VARCHAR) TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON FUNCTION SNOWFLAKE.CORE.DATA_METRIC_SCHEDULED_TIME() TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON FUNCTION SNOWFLAKE.ML.TOP_INSIGHTS(OBJECT, OBJECT, FLOAT, BOOLEAN) TO ROLE "ACCOUNTADMIN";
GRANT READ ON IMAGE_REPOSITORY SNOWHOUSE_IMPORT.IMAGES.EXTERNAL_IMAGES TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON PROCEDURE SNOWFLAKE.ALERT.CREATE_RECOMMENDED_ALERTS(OBJECT) TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON PROCEDURE SNOWFLAKE.CORTEX.CREATE_AI_FUNCTION(VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARIANT, VARIANT, VARCHAR, VARCHAR, VARCHAR) TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON PROCEDURE SNOWFLAKE.DATA_PRIVACY.RESET_PRIVACY_BUDGET(VARCHAR, VARCHAR, VARCHAR, VARCHAR) TO ROLE "ACCOUNTADMIN";
GRANT OWNERSHIP ON ROLE "GITHUB_CI_TEST_ROLE" TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT ROLE "SECURITYADMIN" TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON SCHEMA SNOWFLAKE.ACCOUNT_USAGE TO ROLE "ACCOUNTADMIN";
GRANT USAGE ON SCHEMA SNOWFLAKE.ALERT TO ROLE "ACCOUNTADMIN";
GRANT SELECT ON TABLE SNOWFLAKE_SAMPLE_DATA.TPCH_SF001.CUSTOMER TO ROLE "ACCOUNTADMIN";
GRANT SELECT ON TABLE SNOWFLAKE_SAMPLE_DATA.TPCH_SF001.LINEITEM TO ROLE "ACCOUNTADMIN";
GRANT APPLY ON TAG SNOWFLAKE.CORE.CERTIFICATION_STATUS TO ROLE "ACCOUNTADMIN";
GRANT OWNERSHIP ON USER KALLESIUKOLA TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT SELECT ON VIEW SNOWFLAKE.ACCOUNT_USAGE.ACCESS_HISTORY TO ROLE "ACCOUNTADMIN";
GRANT SELECT ON VIEW SNOWFLAKE.ACCOUNT_USAGE.QUERY_HISTORY TO ROLE "ACCOUNTADMIN";
GRANT OWNERSHIP ON WAREHOUSE COMPUTE_WH TO ROLE "ACCOUNTADMIN" WITH GRANT OPTION;
GRANT ROLE "ACCOUNTADMIN" TO USER "KALLESIUKOLA";`

// TestAccountAdminDDL_CrossValidation feeds every statement from a real
// ACCOUNTADMIN role DDL export through ValidateSnowflakePatterns to identify
// which statements the validator considers invalid. This surfaces gaps in
// the grantObjectPrivileges map and the _grantObjType regex.
func TestAccountAdminDDL_CrossValidation(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	if len(stmts) == 0 {
		t.Fatal("no statements parsed from accountAdminDDL")
	}

	var failures []string
	for _, stmt := range stmts {
		ranges := GetStatementRanges(stmt)
		markers := ValidateSnowflakePatterns(stmt, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			var msgs []string
			for _, w := range warnings {
				msgs = append(msgs, w.Message)
			}
			failures = append(failures, stmt+" → "+strings.Join(msgs, "; "))
		}
	}

	if len(failures) > 0 {
		t.Errorf("Validator flagged %d/%d statements as invalid:\n\n%s",
			len(failures), len(stmts), strings.Join(failures, "\n\n"))
	}
}

// TestAccountAdminDDL_StatementCount verifies the DDL constant parses to the
// expected number of statements. This guards against accidental truncation or
// corruption of the test data.
func TestAccountAdminDDL_StatementCount(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	// 1 CREATE ROLE + 116 GRANT statements = 117 total
	const wantCount = 117
	if len(stmts) != wantCount {
		t.Errorf("expected %d statements, got %d", wantCount, len(stmts))
	}
}

// TestAccountAdminDDL_IndividualStatements runs each DDL statement as a
// separate subtest so failures pinpoint the exact statement instead of
// reporting a bulk count.
func TestAccountAdminDDL_IndividualStatements(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	for i, stmt := range stmts {
		// Use first 60 chars of the statement as the subtest name.
		name := stmt
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(fmt.Sprintf("%d/%s", i, name), func(t *testing.T) {
			ranges := GetStatementRanges(stmt)
			markers := ValidateSnowflakePatterns(stmt, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("statement flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					stmt, strings.Join(msgs, "; "))
			}
		})
	}
}

// TestAccountAdminDDL_RevokeEquivalents ensures that REVOKE counterparts of
// the account-level GRANT statements in the DDL also validate without
// warnings. This catches asymmetries between validateGrant and validateRevoke.
func TestAccountAdminDDL_RevokeEquivalents(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		// Only convert "GRANT <priv> ON <type> ... TO ROLE ..." statements.
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}

		// Build REVOKE: replace "GRANT" → "REVOKE", "TO ROLE" → "FROM ROLE",
		// strip "WITH GRANT OPTION".
		revoke := "REVOKE" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM ROLE ", 1)
		revoke = strings.Replace(revoke, " to role ", " from role ", 1)
		revoke = strings.Replace(revoke, " WITH GRANT OPTION", "", 1)

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE equivalent flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
}

// TestAccountAdminDDL_RevokeRoleEquivalents ensures that REVOKE ROLE
// counterparts of the GRANT ROLE statements in the DDL also validate without
// warnings. The main RevokeEquivalents test skips GRANT ROLE statements because
// they use a different syntax (no ON clause); this test covers them.
func TestAccountAdminDDL_RevokeRoleEquivalents(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		count++

		// GRANT ROLE X TO ROLE/USER Y → REVOKE ROLE X FROM ROLE/USER Y
		revoke := "REVOKE" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM ROLE ", 1)
		revoke = strings.Replace(revoke, " TO USER ", " FROM USER ", 1)
		revoke = strings.Replace(revoke, " to role ", " from role ", 1)
		revoke = strings.Replace(revoke, " to user ", " from user ", 1)

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE ROLE equivalent flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no GRANT ROLE statements found in accountAdminDDL")
	}
}

// TestAccountAdminDDL_RevokeGrantOptionFor ensures that REVOKE GRANT OPTION FOR
// counterparts of the GRANT ... WITH GRANT OPTION statements also validate.
// This exercises the reRevokeOnObject regex's optional GRANT OPTION FOR prefix.
func TestAccountAdminDDL_RevokeGrantOptionFor(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " WITH GRANT OPTION") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		// Build REVOKE GRANT OPTION FOR: insert "GRANT OPTION FOR" after REVOKE,
		// swap TO→FROM, strip WITH GRANT OPTION.
		revoke := "REVOKE GRANT OPTION FOR" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM ROLE ", 1)
		revoke = strings.Replace(revoke, " to role ", " from role ", 1)
		revoke = strings.Replace(revoke, " WITH GRANT OPTION", "", 1)

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE GRANT OPTION FOR equivalent flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no GRANT ... WITH GRANT OPTION statements found in accountAdminDDL")
	}
}

// TestAccountAdminDDL_CaseInsensitive verifies that lowercase and mixed-case
// variants of every DDL statement also validate without warnings. This catches
// case-sensitivity bugs in the regex patterns.
func TestAccountAdminDDL_CaseInsensitive(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)

	for i, stmt := range stmts {
		lower := strings.ToLower(stmt)
		name := lower
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(fmt.Sprintf("%d/lower/%s", i, name), func(t *testing.T) {
			ranges := GetStatementRanges(lower)
			markers := ValidateSnowflakePatterns(lower, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("lowercase variant flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					lower, strings.Join(msgs, "; "))
			}
		})
	}
}

// TestAccountAdminDDL_MixedCase verifies that mixed-case variants of every DDL
// statement also validate without warnings. This alternates upper/lower case on
// each word to catch regex patterns that only handle fully-upper or fully-lower.
func TestAccountAdminDDL_MixedCase(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)

	for i, stmt := range stmts {
		// Alternate case per word: word 0 lower, word 1 upper, etc.
		words := strings.Fields(stmt)
		for j, w := range words {
			if j%2 == 0 {
				words[j] = strings.ToLower(w)
			} else {
				words[j] = strings.ToUpper(w)
			}
		}
		mixed := strings.Join(words, " ")

		name := mixed
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(fmt.Sprintf("%d/%s", i, name), func(t *testing.T) {
			ranges := GetStatementRanges(mixed)
			markers := ValidateSnowflakePatterns(mixed, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("mixed-case variant flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					mixed, strings.Join(msgs, "; "))
			}
		})
	}
}

// TestAccountAdminDDL_RevokeWithCascade verifies that REVOKE equivalents with
// CASCADE suffix validate without warnings. The main RevokeEquivalents test
// produces bare REVOKE statements; this ensures the CASCADE keyword path works.
func TestAccountAdminDDL_RevokeWithCascade(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		revoke := "REVOKE" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM ROLE ", 1)
		revoke = strings.Replace(revoke, " to role ", " from role ", 1)
		revoke = strings.Replace(revoke, " WITH GRANT OPTION", "", 1)
		revoke += " CASCADE"

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE CASCADE flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no applicable GRANT statements found")
	}
}

// TestAccountAdminDDL_RevokeWithRestrict verifies that REVOKE equivalents with
// RESTRICT suffix validate without warnings.
func TestAccountAdminDDL_RevokeWithRestrict(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		revoke := "REVOKE" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM ROLE ", 1)
		revoke = strings.Replace(revoke, " to role ", " from role ", 1)
		revoke = strings.Replace(revoke, " WITH GRANT OPTION", "", 1)
		revoke += " RESTRICT"

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE RESTRICT flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no applicable GRANT statements found")
	}
}

// TestAccountAdminDDL_AccountPrivilegesNotInDDL verifies that account-level
// privileges present in the grantObjectPrivileges map but absent from the
// real-world DDL also validate without warnings. This ensures full privilege
// map coverage beyond what the ACCOUNTADMIN export happens to include.
func TestAccountAdminDDL_AccountPrivilegesNotInDDL(t *testing.T) {
	// Collect all account-level privileges present in the DDL.
	stmts := splitDDLStatements(accountAdminDDL)
	ddlPrivs := map[string]bool{}
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.Contains(upper, " ON ACCOUNT ") {
			continue
		}
		// Extract privilege: between "GRANT " and " ON ACCOUNT"
		idx := strings.Index(upper, " ON ACCOUNT")
		if idx < 6 {
			continue
		}
		priv := strings.TrimSpace(upper[6:idx]) // skip "GRANT "
		ddlPrivs[priv] = true
	}

	// Test every ACCOUNT privilege from the map that isn't already in the DDL.
	for _, priv := range grantObjectPrivileges["ACCOUNT"] {
		if ddlPrivs[priv] {
			continue
		}
		sql := fmt.Sprintf("GRANT %s ON ACCOUNT TO ROLE test_role", priv)
		t.Run(priv, func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("account privilege not in DDL flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					sql, strings.Join(msgs, "; "))
			}
		})
	}
}

// TestAccountAdminDDL_GranteeDatabaseRole verifies that swapping the grantee
// from TO ROLE to TO DATABASE ROLE in the DDL statements validates correctly.
// The real-world DDL only uses TO ROLE; this exercises the DATABASE ROLE grantee
// code path.
func TestAccountAdminDDL_GranteeDatabaseRole(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		// Only convert privilege grants (not GRANT ROLE or CREATE ROLE).
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		variant := strings.Replace(stmt, " TO ROLE ", " TO DATABASE ROLE ", 1)
		name := variant
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(variant)
			markers := ValidateSnowflakePatterns(variant, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("TO DATABASE ROLE grantee flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					variant, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no applicable GRANT statements found")
	}
}

// TestAccountAdminDDL_WhitespaceNormalization verifies that extra whitespace
// (double spaces, tabs) in privilege names doesn't cause false positives. This
// exercises the splitPrivileges whitespace-collapsing logic through the DDL
// cross-validation path.
func TestAccountAdminDDL_WhitespaceNormalization(t *testing.T) {
	// Pick multi-word account-level privilege statements from the DDL and
	// inject extra whitespace into the privilege name.
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.Contains(upper, " ON ACCOUNT ") {
			continue
		}
		// Only multi-word privileges benefit from this test.
		idx := strings.Index(upper, " ON ACCOUNT")
		priv := strings.TrimSpace(upper[6:idx])
		if !strings.Contains(priv, " ") {
			continue
		}
		count++

		// Double the first space in the privilege name.
		firstSpace := strings.Index(stmt[6:], " ") + 6
		variant := stmt[:firstSpace] + " " + stmt[firstSpace:]

		name := variant
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(variant)
			markers := ValidateSnowflakePatterns(variant, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("whitespace variant flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					variant, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no multi-word account privilege statements found")
	}
}

// TestAccountAdminDDL_GrantRoleToShare verifies that GRANT ROLE ... TO SHARE
// variants of the DDL's GRANT ROLE statements validate correctly. The real-world
// DDL only grants roles TO ROLE and TO USER.
func TestAccountAdminDDL_GrantRoleToShare(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		count++

		var variant string
		if strings.Contains(stmt, " TO ROLE ") {
			variant = strings.Replace(stmt, " TO ROLE ", " TO SHARE ", 1)
		} else if strings.Contains(stmt, " TO USER ") {
			variant = strings.Replace(stmt, " TO USER ", " TO SHARE ", 1)
		} else {
			continue
		}

		name := variant
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(variant)
			markers := ValidateSnowflakePatterns(variant, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("GRANT ROLE TO SHARE flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					variant, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no GRANT ROLE statements found")
	}
}

// TestAccountAdminDDL_GrantRoleToDatabaseRole verifies that GRANT ROLE ... TO
// DATABASE ROLE variants validate correctly. The DDL only uses TO ROLE/TO USER.
func TestAccountAdminDDL_GrantRoleToDatabaseRole(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ROLE ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		variant := strings.Replace(stmt, " TO ROLE ", " TO DATABASE ROLE ", 1)
		name := variant
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(variant)
			markers := ValidateSnowflakePatterns(variant, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("GRANT ROLE TO DATABASE ROLE flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					variant, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no GRANT ROLE ... TO ROLE statements found")
	}
}

// TestAccountAdminDDL_RevokeDatabaseRoleEquivalents verifies that REVOKE ...
// FROM DATABASE ROLE variants of the DDL's ON-clause GRANT statements validate
// correctly. The real-world DDL only uses TO ROLE as grantee.
func TestAccountAdminDDL_RevokeDatabaseRoleEquivalents(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)
	var count int
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if !strings.HasPrefix(upper, "GRANT ") || strings.HasPrefix(upper, "GRANT ROLE ") {
			continue
		}
		if !strings.Contains(upper, " ON ") || !strings.Contains(upper, " TO ROLE ") {
			continue
		}
		count++

		revoke := "REVOKE" + stmt[len("GRANT"):]
		revoke = strings.Replace(revoke, " TO ROLE ", " FROM DATABASE ROLE ", 1)
		revoke = strings.Replace(revoke, " to role ", " from database role ", 1)
		revoke = strings.Replace(revoke, " WITH GRANT OPTION", "", 1)

		name := revoke
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			ranges := GetStatementRanges(revoke)
			markers := ValidateSnowflakePatterns(revoke, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				var msgs []string
				for _, w := range warnings {
					msgs = append(msgs, w.Message)
				}
				t.Errorf("REVOKE FROM DATABASE ROLE flagged as invalid:\n  SQL: %s\n  Warnings: %s",
					revoke, strings.Join(msgs, "; "))
			}
		})
	}
	if count == 0 {
		t.Fatal("no applicable GRANT statements found")
	}
}

// TestAccountAdminDDL_StatementCategories verifies the expected breakdown of
// statement types in the DDL. This catches silent data corruption at a finer
// granularity than the total statement count.
func TestAccountAdminDDL_StatementCategories(t *testing.T) {
	stmts := splitDDLStatements(accountAdminDDL)

	counts := map[string]int{}
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		switch {
		case strings.HasPrefix(upper, "CREATE ROLE"):
			counts["CREATE ROLE"]++
		case strings.HasPrefix(upper, "GRANT ROLE"):
			counts["GRANT ROLE"]++
		case strings.Contains(upper, " ON ACCOUNT "):
			counts["ON ACCOUNT"]++
		case strings.Contains(upper, " ON APPLICATION_ROLE "):
			counts["ON APPLICATION_ROLE"]++
		case strings.Contains(upper, " ON CLASS "):
			counts["ON CLASS"]++
		case strings.Contains(upper, " ON COMPUTE_POOL "):
			counts["ON COMPUTE_POOL"]++
		case strings.Contains(upper, " ON DATABASE_ROLE "):
			counts["ON DATABASE_ROLE"]++
		case strings.Contains(upper, " ON DATABASE "):
			counts["ON DATABASE"]++
		case strings.Contains(upper, " ON FUNCTION "):
			counts["ON FUNCTION"]++
		case strings.Contains(upper, " ON IMAGE_REPOSITORY "):
			counts["ON IMAGE_REPOSITORY"]++
		case strings.Contains(upper, " ON PROCEDURE "):
			counts["ON PROCEDURE"]++
		case strings.Contains(upper, " ON ROLE "):
			counts["ON ROLE"]++
		case strings.Contains(upper, " ON SCHEMA "):
			counts["ON SCHEMA"]++
		case strings.Contains(upper, " ON TABLE "):
			counts["ON TABLE"]++
		case strings.Contains(upper, " ON TAG "):
			counts["ON TAG"]++
		case strings.Contains(upper, " ON USER "):
			counts["ON USER"]++
		case strings.Contains(upper, " ON VIEW "):
			counts["ON VIEW"]++
		case strings.Contains(upper, " ON WAREHOUSE "):
			counts["ON WAREHOUSE"]++
		default:
			t.Errorf("uncategorized statement: %s", stmt)
		}
	}

	expected := map[string]int{
		"CREATE ROLE":         1,
		"ON ACCOUNT":          74,
		"ON APPLICATION_ROLE": 3,
		"ON CLASS":            13,
		"ON COMPUTE_POOL":     2,
		"ON DATABASE":         3,
		"ON DATABASE_ROLE":    2,
		"ON FUNCTION":         3,
		"ON IMAGE_REPOSITORY": 1,
		"ON PROCEDURE":        3,
		"ON ROLE":             1,
		"GRANT ROLE":          2,
		"ON SCHEMA":           2,
		"ON TABLE":            2,
		"ON TAG":              1,
		"ON USER":             1,
		"ON VIEW":             2,
		"ON WAREHOUSE":        1,
	}

	for cat, want := range expected {
		got := counts[cat]
		if got != want {
			t.Errorf("category %q: got %d, want %d", cat, got, want)
		}
	}
	for cat, got := range counts {
		if _, ok := expected[cat]; !ok {
			t.Errorf("unexpected category %q with count %d", cat, got)
		}
	}
}

// TestSplitDDLStatements tests the splitDDLStatements helper for edge cases.
func TestSplitDDLStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"whitespace only", "   \n\t  ", nil},
		{"single statement no semicolon", "SELECT 1", []string{"SELECT 1"}},
		{"single statement with semicolon", "SELECT 1;", []string{"SELECT 1"}},
		{"two statements", "SELECT 1; SELECT 2", []string{"SELECT 1", "SELECT 2"}},
		{"trailing semicolons", "SELECT 1;;;\n;", []string{"SELECT 1"}},
		{"whitespace between semicolons", "SELECT 1;  ;  ; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"multiline statement", "CREATE TABLE t\n  (id INT);\nSELECT 1",
			[]string{"CREATE TABLE t\n  (id INT)", "SELECT 1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitDDLStatements(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d statements, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("statement[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// splitDDLStatements splits a multi-statement DDL string on semicolons,
// trimming whitespace and skipping empty entries. Note: this splits naively
// without handling semicolons inside string literals, which is fine for the
// controlled test data in this file.
func splitDDLStatements(ddl string) []string {
	raw := strings.Split(ddl, ";")
	var result []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}
