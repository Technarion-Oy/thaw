// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowflake

import "testing"

func TestFormatRoleGrant(t *testing.T) {
	tests := []struct {
		name            string
		priv            string
		onType          string
		obj             string
		role            string
		withGrantOption bool
		want            string
	}{
		{
			name:   "account-level privilege omits account name",
			priv:   "MANAGE GRANTS",
			onType: "ACCOUNT",
			obj:    "WC16727",
			role:   "ORGADMIN",
			want:   `GRANT MANAGE GRANTS ON ACCOUNT TO ROLE "ORGADMIN";`,
		},
		{
			name:   "account-level with grant option",
			priv:   "CREATE DATABASE",
			onType: "ACCOUNT",
			obj:    "MY_ACCT",
			role:   "SYSADMIN",
			withGrantOption: true,
			want:   `GRANT CREATE DATABASE ON ACCOUNT TO ROLE "SYSADMIN" WITH GRANT OPTION;`,
		},
		{
			name:   "account-level case insensitive onType",
			priv:   "EXECUTE TASK",
			onType: "account",
			obj:    "ACCT123",
			role:   "TASKADMIN",
			want:   `GRANT EXECUTE TASK ON ACCOUNT TO ROLE "TASKADMIN";`,
		},
		{
			name:   "table-level includes object name",
			priv:   "SELECT",
			onType: "TABLE",
			obj:    `"MY_DB"."PUBLIC"."MY_TABLE"`,
			role:   "ANALYST",
			want:   `GRANT SELECT ON TABLE "MY_DB"."PUBLIC"."MY_TABLE" TO ROLE "ANALYST";`,
		},
		{
			name:   "warehouse-level includes object name",
			priv:   "USAGE",
			onType: "WAREHOUSE",
			obj:    `"COMPUTE_WH"`,
			role:   "DEV_ROLE",
			want:   `GRANT USAGE ON WAREHOUSE "COMPUTE_WH" TO ROLE "DEV_ROLE";`,
		},
		{
			name:            "table-level with grant option",
			priv:            "INSERT",
			onType:          "TABLE",
			obj:             `"MY_DB"."PUBLIC"."MY_TABLE"`,
			role:            "WRITER",
			withGrantOption: true,
			want:            `GRANT INSERT ON TABLE "MY_DB"."PUBLIC"."MY_TABLE" TO ROLE "WRITER" WITH GRANT OPTION;`,
		},
		{
			name:   "database-level includes object name",
			priv:   "USAGE",
			onType: "DATABASE",
			obj:    `"MY_DB"`,
			role:   "READER",
			want:   `GRANT USAGE ON DATABASE "MY_DB" TO ROLE "READER";`,
		},
		{
			name:   "schema-level includes object name",
			priv:   "CREATE TABLE",
			onType: "SCHEMA",
			obj:    `"MY_DB"."PUBLIC"`,
			role:   "DEV",
			want:   `GRANT CREATE TABLE ON SCHEMA "MY_DB"."PUBLIC" TO ROLE "DEV";`,
		},
		{
			name:   "role with double-quote in name",
			priv:   "SELECT",
			onType: "TABLE",
			obj:    `"MY_TABLE"`,
			role:   `MY""ROLE`,
			want:   `GRANT SELECT ON TABLE "MY_TABLE" TO ROLE "MY""ROLE";`,
		},
		{
			name:   "multi-word account privilege",
			priv:   "APPLY ROW ACCESS POLICY",
			onType: "ACCOUNT",
			obj:    "PROD_ACCT",
			role:   "SECURITYADMIN",
			want:   `GRANT APPLY ROW ACCESS POLICY ON ACCOUNT TO ROLE "SECURITYADMIN";`,
		},
		{
			name:   "integration-level includes object name",
			priv:   "USAGE",
			onType: "INTEGRATION",
			obj:    `"MY_INTEGRATION"`,
			role:   "ETL_ROLE",
			want:   `GRANT USAGE ON INTEGRATION "MY_INTEGRATION" TO ROLE "ETL_ROLE";`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRoleGrant(tt.priv, tt.onType, tt.obj, tt.role, tt.withGrantOption)
			if got != tt.want {
				t.Errorf("FormatRoleGrant() =\n  %s\nwant:\n  %s", got, tt.want)
			}
		})
	}
}
