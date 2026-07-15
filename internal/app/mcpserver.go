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

// DescribeMCPServer runs DESCRIBE MCP SERVER and returns the raw QueryResult.
// SHOW MCP SERVERS omits the specification, so this is the source for the
// server_spec column (the complete tools spec serialized as JSON) read by the
// properties panel's read-only specification viewer. The single result row
// carries columns name / database_name / schema_name / owner / comment /
// server_spec / created_on. Snowflake has no ALTER MCP SERVER, so there is no
// corresponding mutation method — the object is changed via CREATE OR REPLACE.
func (a *App) DescribeMCPServer(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE MCP SERVER %s",
		snowflake.Qualify(database, schema, name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}
