package sqleditor

import (
	"strings"
	"testing"
)

// Regression tests for #709: ValidateTablesExist must not emit false positives
// when catalog data is legitimately missing/empty. SHOW SCHEMAS fails on shared
// DBs (SNOWFLAKE) so the frontend stores an empty schema list for them, and a
// disconnected / not-yet-loaded catalog sends empty KnownDatabases/KnownSchemas.
// Well-known always-present schemas (INFORMATION_SCHEMA, ACCOUNT_USAGE) must
// never be flagged either.

func markersFor(req ValidateTablesExistRequest, sql string) []DiagMarker {
	req.SQL = sql
	req.StmtRanges = GetStatementRanges(sql)
	return ValidateTablesExist(req)
}

// SNOWFLAKE shared DB: it's in KnownDatabases (always present) but SHOW SCHEMAS
// fails so its schema list is empty — ACCOUNT_USAGE etc. must not be flagged.
func TestValidateTablesExist_SharedDBEmptySchemas_NoFalsePositive(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases:  []string{"MYDB", "SNOWFLAKE"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
	}
	silent := []string{
		"SELECT * FROM SNOWFLAKE.ACCOUNT_USAGE.QUERY_HISTORY;",
		"SELECT * FROM SNOWFLAKE.ORGANIZATION_USAGE.WAREHOUSE_METERING_HISTORY;",
		"USE SCHEMA SNOWFLAKE.ACCOUNT_USAGE;",
		"USE SNOWFLAKE.ACCOUNT_USAGE;",
	}
	for _, sql := range silent {
		if m := markersFor(req, sql); len(m) != 0 {
			t.Errorf("for %q: expected no marker (shared DB, empty schema list), got %d: %+v", sql, len(m), m)
		}
	}
}

// INFORMATION_SCHEMA exists in every database and may be absent from the fetched
// schema list — it must never be flagged even when the DB was expanded.
func TestValidateTablesExist_InformationSchema_NoFalsePositive(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}}, // INFORMATION_SCHEMA not listed
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
	}
	silent := []string{
		"SELECT * FROM MYDB.INFORMATION_SCHEMA.TABLES;",
		"USE SCHEMA MYDB.INFORMATION_SCHEMA;",
	}
	for _, sql := range silent {
		if m := markersFor(req, sql); len(m) != 0 {
			t.Errorf("for %q: expected no marker (INFORMATION_SCHEMA always present), got %d: %+v", sql, len(m), m)
		}
	}
}

// Empty catalog (disconnected / not yet loaded): qualified db.schema references
// in DROP SCHEMA / USE SCHEMA / bare USE must stay silent, matching the existing
// DROP DATABASE guard rather than flagging the database or schema.
func TestValidateTablesExist_EmptyCatalog_NoFalsePositive(t *testing.T) {
	req := ValidateTablesExistRequest{
		// KnownDatabases / KnownSchemas deliberately empty (catalog not loaded).
	}
	silent := []string{
		"DROP SCHEMA d.s;",
		"USE SCHEMA d.s;",
		"USE d.s;",
	}
	for _, sql := range silent {
		if m := markersFor(req, sql); len(m) != 0 {
			t.Errorf("for %q: expected no marker (empty catalog), got %d: %+v", sql, len(m), m)
		}
	}
}

// Negative control: when we DO have schema data for the DB, a genuinely missing
// schema is still flagged (the guard must not suppress real errors).
func TestValidateTablesExist_KnownSchemaData_StillFlagsMissing(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
	}
	cases := []struct{ sql, want string }{
		{"SELECT * FROM MYDB.NOPE.T;", "Schema 'NOPE'"},
		{"USE SCHEMA MYDB.NOPE;", "Schema 'NOPE'"},
	}
	for _, c := range cases {
		m := markersFor(req, c.sql)
		if len(m) == 0 || !strings.Contains(m[0].Message, c.want) {
			t.Errorf("for %q: expected marker containing %q, got %+v", c.sql, c.want, m)
		}
	}
}
