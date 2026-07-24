// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/snowflake"
)

// AlterCortexSearchService runs an ALTER CORTEX SEARCH SERVICE statement for the
// given service. clause is everything that follows the service name, e.g.
// "SUSPEND", "RESUME", "REFRESH", "SET TARGET_LAG = '1 hour'",
// "SET WAREHOUSE = \"WH\"", "SET ATTRIBUTES ( COL1, COL2 )", "UNSET ATTRIBUTES",
// "SET COMMENT = '...'", "UNSET COMMENT", "SET TAG ...", or "UNSET TAG ...". The
// caller is responsible for correct SQL quoting inside the clause; this method
// only double-quotes the service identifier. ALTER CORTEX SEARCH SERVICE has no
// RENAME clause.
func (a *App) AlterCortexSearchService(database, schema, name, clause string) error {
	return a.alterObject("CORTEX SEARCH SERVICE", database, schema, name, clause)
}

// FormatCortexSearchAttributes joins the given column names into a comma-separated
// list (without the surrounding parentheses) for the properties modal's
// "SET ATTRIBUTES ( … )" / "SET PRIMARY KEY ( … )" clauses, dropping blank
// entries. Exposed over IPC so the frontend doesn't duplicate the trim/skip-blank
// logic.
func (a *App) FormatCortexSearchAttributes(columns []string) string {
	return snowflake.JoinCleanList(columns, ", ")
}
