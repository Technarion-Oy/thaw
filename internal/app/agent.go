// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterAgent runs an ALTER AGENT statement for the given Cortex agent. clause is
// everything that follows the agent name, e.g. "SET COMMENT = '...'",
// "SET PROFILE = '{...}'", or
// "MODIFY LIVE VERSION SET SPECIFICATION = $$ ... $$". ALTER AGENT has no RENAME,
// UNSET, or TAG clause. The caller is responsible for correct SQL quoting inside
// the clause; this method only double-quotes the agent identifier.
func (a *App) AlterAgent(database, schema, name, clause string) error {
	return a.alterObject("AGENT", database, schema, name, clause)
}

// DescribeAgent runs DESCRIBE AGENT and returns the raw QueryResult. SHOW AGENTS
// omits the full specification, so this is the source for the agent_spec column
// (the complete JSON spec) — plus profile, comment, and ownership metadata — read
// by the properties panel's specification editor. The single result row carries
// columns name / database_name / schema_name / owner / comment / profile /
// agent_spec / created_on.
func (a *App) DescribeAgent(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE AGENT %s",
		snowflake.Qualify(database, schema, name))
	return a.client.Execute(a.ctx, sql)
}
