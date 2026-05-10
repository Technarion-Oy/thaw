// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import "testing"

func TestValidateSnowflakePatterns_UseRole(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "Simple role name",
			sql:  "USE ROLE my_role",
		},
		{
			name: "Quoted role name",
			sql:  `USE ROLE "My Role"`,
		},
		{
			name: "Trailing semicolon",
			sql:  "USE ROLE my_role;",
		},
		{
			name: "Lowercase — case insensitive",
			sql:  "use role my_role",
		},
		{
			name: "Mixed case",
			sql:  "Use Role admin",
		},
		{
			name: "Role with dollar sign",
			sql:  "USE ROLE my$role",
		},
		{
			name: "Role with digits",
			sql:  "USE ROLE role_123",
		},
		{
			name: "Role with extra whitespace",
			sql:  "USE   ROLE   my_role",
		},
		{
			name: "Role with comment before name",
			sql:  "USE ROLE /* admin */ sysadmin",
		},
		{
			name: "Role NONE is a valid Snowflake role name",
			sql:  "USE ROLE NONE",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "Bare USE ROLE — no role name",
			sql:           "USE ROLE",
			expectWarning: true,
			expectedMatch: "requires a role name",
		},
		{
			name:          "USE ROLE with only semicolon",
			sql:           "USE ROLE;",
			expectWarning: true,
			expectedMatch: "requires a role name",
		},
		{
			name:          "USE ROLE with space then semicolon",
			sql:           "USE ROLE ;",
			expectWarning: true,
			expectedMatch: "requires a role name",
		},
		{
			name:          "USE ROLE with only line comment — no role name",
			sql:           "USE ROLE -- my_role",
			expectWarning: true,
			expectedMatch: "requires a role name",
		},
	})
}

func TestValidateSnowflakePatterns_UseWarehouse(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "Simple warehouse name",
			sql:  "USE WAREHOUSE my_wh",
		},
		{
			name: "Quoted warehouse name",
			sql:  `USE WAREHOUSE "My Warehouse"`,
		},
		{
			name: "Trailing semicolon",
			sql:  "USE WAREHOUSE my_wh;",
		},
		{
			name: "Lowercase — case insensitive",
			sql:  "use warehouse my_wh",
		},
		{
			name: "Mixed case",
			sql:  "Use Warehouse compute_wh",
		},
		{
			name: "Warehouse with digits",
			sql:  "USE WAREHOUSE wh_01",
		},
		{
			name: "Warehouse with extra whitespace",
			sql:  "USE   WAREHOUSE   my_wh",
		},
		{
			name: "Warehouse with comment before name",
			sql:  "USE WAREHOUSE /* old */ new_wh",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "Bare USE WAREHOUSE — no warehouse name",
			sql:           "USE WAREHOUSE",
			expectWarning: true,
			expectedMatch: "requires a warehouse name",
		},
		{
			name:          "USE WAREHOUSE with only semicolon",
			sql:           "USE WAREHOUSE;",
			expectWarning: true,
			expectedMatch: "requires a warehouse name",
		},
		{
			name:          "USE WAREHOUSE with space then semicolon",
			sql:           "USE WAREHOUSE ;",
			expectWarning: true,
			expectedMatch: "requires a warehouse name",
		},
		{
			name:          "USE WAREHOUSE with only line comment — no warehouse name",
			sql:           "USE WAREHOUSE -- my_wh",
			expectWarning: true,
			expectedMatch: "requires a warehouse name",
		},
	})
}

func TestValidateSnowflakePatterns_UseSecondaryRoles(t *testing.T) {
	runPatternTests(t, []patternTestCase{
		// ── PASS ─────────────────────────────────────────────────────────────
		{
			name: "USE SECONDARY ROLES ALL",
			sql:  "USE SECONDARY ROLES ALL",
		},
		{
			name: "USE SECONDARY ROLES NONE",
			sql:  "USE SECONDARY ROLES NONE",
		},
		{
			name: "Lowercase — case insensitive",
			sql:  "use secondary roles all",
		},
		{
			name: "Mixed case",
			sql:  "Use Secondary Roles None",
		},
		{
			name: "ALL with trailing semicolon",
			sql:  "USE SECONDARY ROLES ALL;",
		},
		{
			name: "NONE with trailing semicolon",
			sql:  "USE SECONDARY ROLES NONE;",
		},
		{
			name: "Extra whitespace",
			sql:  "USE   SECONDARY   ROLES   ALL",
		},
		{
			name: "Comment before value",
			sql:  "USE SECONDARY ROLES /* toggle */ ALL",
		},

		// ── FAIL ─────────────────────────────────────────────────────────────
		{
			name:          "Bare USE SECONDARY ROLES — no value",
			sql:           "USE SECONDARY ROLES",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
		{
			name:          "USE SECONDARY ROLES with only semicolon",
			sql:           "USE SECONDARY ROLES;",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
		{
			name:          "USE SECONDARY ROLES with space then semicolon",
			sql:           "USE SECONDARY ROLES ;",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
		{
			name:          "Invalid value — SOME",
			sql:           "USE SECONDARY ROLES SOME",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
		{
			name:          "Invalid value — arbitrary identifier",
			sql:           "USE SECONDARY ROLES my_roles",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
		{
			name:          "Invalid value — number",
			sql:           "USE SECONDARY ROLES 123",
			expectWarning: true,
			expectedMatch: "requires ALL or NONE",
		},
	})
}
