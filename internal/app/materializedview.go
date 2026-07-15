// SPDX-License-Identifier: GPL-3.0-or-later

package app

// AlterMaterializedView runs an ALTER MATERIALIZED VIEW statement for the given
// view. clause is everything that follows the view name, e.g. "SUSPEND",
// "RESUME", "SUSPEND RECLUSTER", "RESUME RECLUSTER", "CLUSTER BY (c1)",
// "DROP CLUSTERING KEY", "SET COMMENT = '...'", "SET SECURE", or
// "UNSET COMMENT". The caller is responsible for correct SQL quoting inside the
// clause; this method only double-quotes the view identifier. Materialized views
// have no manual REFRESH command — Snowflake maintains them automatically.
func (a *App) AlterMaterializedView(database, schema, name, clause string) error {
	return a.alterObject("MATERIALIZED VIEW", database, schema, name, clause)
}
