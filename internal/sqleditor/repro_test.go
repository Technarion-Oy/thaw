package sqleditor

import (
	"strings"
	"testing"
)

func TestReproIssues(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases: []string{"GOVERNANCE"},
		KnownSchemas:   []SchemaEntry{{DB: "GOVERNANCE", Name: "PUBLIC"}},
		ResolvedRefs: []ResolvedRef{
			{Alias: "TEST_TABLE", DB: "GOVERNANCE", Schema: "PUBLIC", Name: "TEST_TABLE"},
		},
	}

	tests := []struct {
		name          string
		sql           string
		expectedMatch string
		shouldPass    bool
	}{
		{
			name:          "Issue 1: Quoted unknown DB should complain",
			sql:           `SELECT * FROM "fggdfgdf"."DEMO_SCHEMA"."CUSTOMER_SUMMARY_METRICS";`,
			expectedMatch: "fggdfgdf",
			shouldPass:    false,
		},
		{
			name:          "Issue 2: Unquoted unknown DB should complain",
			sql:           `SELECT * FROM fggdfgdf."DEMO_SCHEMA"."CUSTOMER_SUMMARY_METRICS";`,
			expectedMatch: "fggdfgdf",
			shouldPass:    false,
		},
		{
			name:          "Issue 3: Known DB, unknown schema should complain about schema",
			sql:           `SELECT * FROM GOVERNANCE."DEMO_SCHEMA"."CUSTOMER_SUMMARY_METRICS";`,
			expectedMatch: "DEMO_SCHEMA",
			shouldPass:    false,
		},
		{
			name:       "Issue 4: Known DB and schema should pass",
			sql:        `SELECT * FROM "GOVERNANCE"."PUBLIC"."TEST_TABLE";`,
			shouldPass: true,
		},
		{
			name:       "Quoted table only",
			sql:        `SELECT * FROM "TEST_TABLE";`,
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := req
			r.SQL = tt.sql
			r.StmtRanges = GetStatementRanges(tt.sql)
			markers := ValidateTablesExist(r)
			errs := getErrors(markers)

			if tt.shouldPass {
				if len(errs) > 0 {
					t.Errorf("Expected 0 errors, got %d: %v", len(errs), errs)
				}
			} else {
				if len(errs) == 0 {
					t.Fatalf("Expected errors, got 0")
				}
				found := false
				for _, e := range errs {
					if strings.Contains(strings.ToUpper(e.Message), strings.ToUpper(tt.expectedMatch)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error matching %q, got: %v", tt.expectedMatch, errs[0].Message)
				}
			}
		})
	}
}
