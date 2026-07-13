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

// Negative control for the always-present-schema skip: an always-present schema
// (INFORMATION_SCHEMA, ACCOUNT_USAGE) must not let a bogus DB slip through
// unvalidated. The DB-existence check still runs, so BOGUS.INFORMATION_SCHEMA.T
// is flagged on the database, not silently accepted.
func TestValidateTablesExist_AlwaysPresentSchema_StillFlagsMissingDB(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
	}
	cases := []struct{ sql, want string }{
		{"SELECT * FROM TOTALLY_BOGUS_DB.INFORMATION_SCHEMA.TABLES;", "Database 'TOTALLY_BOGUS_DB'"},
		{"SELECT * FROM NOPEDB.ACCOUNT_USAGE.QUERY_HISTORY;", "Database 'NOPEDB'"},
	}
	for _, c := range cases {
		m := markersFor(req, c.sql)
		found := false
		for _, mk := range m {
			if strings.Contains(mk.Message, c.want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("for %q: expected a marker containing %q, got %+v", c.sql, c.want, m)
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

// ALTER TABLE/VIEW RENAME and ALTER TABLE … SWAP WITH: the DB is known but its
// schema list hasn't been fetched yet (empty for that DB), so a qualified
// db.schema.table target must not be flagged (#709). Mirrors the frontend
// state right after connecting.
func TestValidateTablesExist_AlterMissingSchemaData_NoFalsePositive(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases: []string{"MYDB"},
		// No schema entries for MYDB — schema list not yet fetched.
	}
	silent := []string{
		"ALTER TABLE MYDB.UNLISTEDSCHEMA.TBL RENAME TO MYDB.UNLISTEDSCHEMA.TBL2;",
		"ALTER VIEW MYDB.UNLISTEDSCHEMA.V RENAME TO MYDB.UNLISTEDSCHEMA.V2;",
		"ALTER TABLE MYDB.PUBLIC.A SWAP WITH MYDB.UNLISTEDSCHEMA.B;",
	}
	for _, sql := range silent {
		if m := markersFor(req, sql); len(m) != 0 {
			t.Errorf("for %q: expected no marker (DB known, schema list not fetched), got %d: %+v", sql, len(m), m)
		}
	}
}

// Negative control for the ALTER paths: with real schema data for the DB, a
// genuinely missing schema in RENAME / SWAP WITH is still flagged.
func TestValidateTablesExist_AlterKnownSchemaData_StillFlagsMissing(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases: []string{"MYDB"},
		KnownSchemas:   []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		// A is a real table so only the SWAP target's bad schema is under test.
		ResolvedRefs: []ResolvedRef{{DB: "MYDB", Schema: "PUBLIC", Name: "A"}},
	}
	cases := []struct{ sql, want string }{
		{"ALTER TABLE MYDB.NOPE.TBL RENAME TO MYDB.NOPE.TBL2;", "Schema 'NOPE'"},
		{"ALTER TABLE MYDB.PUBLIC.A SWAP WITH MYDB.NOPE.B;", "Schema 'NOPE'"},
	}
	for _, c := range cases {
		m := markersFor(req, c.sql)
		found := false
		for _, mk := range m {
			if strings.Contains(mk.Message, c.want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("for %q: expected a marker containing %q, got %+v", c.sql, c.want, m)
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
