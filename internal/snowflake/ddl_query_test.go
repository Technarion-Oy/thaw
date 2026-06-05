// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowflake

import "testing"

func TestBuildGetDDLQuery(t *testing.T) {
	tests := []struct {
		name      string
		database  string
		schema    string
		kind      string
		objName   string
		arguments string
		wantQuery string
		wantIdent string
	}{
		{
			name:      "account-level warehouse",
			kind:      "WAREHOUSE",
			objName:   "MY_WH",
			wantQuery: `SELECT GET_DDL('WAREHOUSE', 'MY_WH', true)`,
			wantIdent: "MY_WH",
		},
		{
			name:      "account-level database",
			kind:      "DATABASE",
			objName:   "MY_DB",
			wantQuery: `SELECT GET_DDL('DATABASE', 'MY_DB', true)`,
			wantIdent: "MY_DB",
		},
		{
			name:      "account-level name with single quote",
			kind:      "WAREHOUSE",
			objName:   "WH'NAME",
			wantQuery: `SELECT GET_DDL('WAREHOUSE', 'WH''NAME', true)`,
			wantIdent: "WH''NAME",
		},
		{
			name:      "schema-scoped table",
			database:  "MY_DB",
			schema:    "PUBLIC",
			kind:      "TABLE",
			objName:   "USERS",
			wantQuery: `SELECT GET_DDL('TABLE', '"MY_DB"."PUBLIC"."USERS"', true)`,
			wantIdent: `"MY_DB"."PUBLIC"."USERS"`,
		},
		{
			name:      "schema-scoped table with mixed case",
			database:  "myDb",
			schema:    "mySchema",
			kind:      "TABLE",
			objName:   "MyTable",
			wantQuery: `SELECT GET_DDL('TABLE', '"myDb"."mySchema"."MyTable"', true)`,
			wantIdent: `"myDb"."mySchema"."MyTable"`,
		},
		{
			name:      "procedure with arguments",
			database:  "DB",
			schema:    "SCH",
			kind:      "PROCEDURE",
			objName:   "MY_PROC",
			arguments: "NUMBER, VARCHAR",
			wantQuery: `SELECT GET_DDL('PROCEDURE', '"DB"."SCH"."MY_PROC"(NUMBER, VARCHAR)', true)`,
			wantIdent: `"DB"."SCH"."MY_PROC"(NUMBER, VARCHAR)`,
		},
		{
			name:      "function with no arguments",
			database:  "DB",
			schema:    "SCH",
			kind:      "FUNCTION",
			objName:   "MY_FUNC",
			arguments: "",
			wantQuery: `SELECT GET_DDL('FUNCTION', '"DB"."SCH"."MY_FUNC"()', true)`,
			wantIdent: `"DB"."SCH"."MY_FUNC"()`,
		},
		{
			name:      "schema-scoped view with quotes in identifier",
			database:  "DB",
			schema:    "SCH",
			kind:      "VIEW",
			objName:   "V'IEW",
			wantQuery: `SELECT GET_DDL('VIEW', '"DB"."SCH"."V''IEW"', true)`,
			wantIdent: `"DB"."SCH"."V''IEW"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotIdent := buildGetDDLQuery(tt.database, tt.schema, tt.kind, tt.objName, tt.arguments)
			if gotQuery != tt.wantQuery {
				t.Errorf("query:\n got  %s\n want %s", gotQuery, tt.wantQuery)
			}
			if gotIdent != tt.wantIdent {
				t.Errorf("identifier:\n got  %s\n want %s", gotIdent, tt.wantIdent)
			}
		})
	}
}
