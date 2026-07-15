// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

// This file holds pure (no I/O) builders for the DROP and SHOW statements whose
// object names must be quoted. They exist as a unit-testable seam: the executing
// *Client methods (DropDatabase, GetRoleDDL, …) delegate here,
// so the quoting that guards against bare/reserved/case-sensitive identifiers is
// asserted by tests rather than relying on a live connection. Every name is run
// through QuoteIdent/Qualify, which both wrap and escape — never emit an
// identifier into one of these statements unquoted.

// normalizeDropMode coerces a DROP … drop mode to a valid keyword. RESTRICT is
// honored; any other value (including the empty string) defaults to CASCADE, the
// safer choice for the UI's "drop everything" intent.
func normalizeDropMode(mode string) string {
	if mode == "RESTRICT" {
		return "RESTRICT"
	}
	return "CASCADE"
}

// dropIntegrationStmt builds `DROP INTEGRATION "<name>"`.
func dropIntegrationStmt(name string) string {
	return "DROP INTEGRATION " + QuoteIdent(name)
}

// dropDatabaseStmt builds `DROP DATABASE "<name>" <CASCADE|RESTRICT>`.
func dropDatabaseStmt(name, mode string) string {
	return "DROP DATABASE " + QuoteIdent(name) + " " + normalizeDropMode(mode)
}

// dropSchemaStmt builds `DROP SCHEMA "<db>"."<schema>" <CASCADE|RESTRICT>`.
func dropSchemaStmt(database, schema, mode string) string {
	return "DROP SCHEMA " + Qualify(database, schema) + " " + normalizeDropMode(mode)
}

// showGrantsToRoleStmt builds `SHOW GRANTS TO ROLE "<role>"` — the privileges
// granted to the role. Used by GetRoleDDL.
func showGrantsToRoleStmt(role string) string {
	return "SHOW GRANTS TO ROLE " + QuoteIdent(role)
}

// showGrantsOnRoleStmt builds `SHOW GRANTS ON ROLE "<role>"` — who the role is
// granted to.
func showGrantsOnRoleStmt(role string) string {
	return "SHOW GRANTS ON ROLE " + QuoteIdent(role)
}

// showSchemasHistoryStmt builds `SHOW SCHEMAS HISTORY IN DATABASE "<db>"` — used
// to list dropped (but still within Time Travel) schemas.
func showSchemasHistoryStmt(database string) string {
	return "SHOW SCHEMAS HISTORY IN DATABASE " + QuoteIdent(database)
}
