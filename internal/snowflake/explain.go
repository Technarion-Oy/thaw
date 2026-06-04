// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import (
	"context"
	"database/sql"
)

// ExplainFormat selects the output format for Snowflake's EXPLAIN command.
type ExplainFormat string

const (
	ExplainJSON    ExplainFormat = "JSON"
	ExplainTabular ExplainFormat = "TABULAR"
)

// Explain runs EXPLAIN USING <format> for the given query and returns the raw
// result. No business logic — callers are responsible for parsing.
func (c *Client) Explain(ctx context.Context, query string, format ExplainFormat) (*QueryResult, error) {
	return c.QuerySingle(ctx, "EXPLAIN USING "+string(format)+" "+query)
}

// ExplainOnConn runs EXPLAIN USING <format> on a pinned connection. No session
// sync is needed for EXPLAIN so this delegates directly to queryOnConn.
func (c *Client) ExplainOnConn(ctx context.Context, conn *sql.Conn, query string, format ExplainFormat) (*QueryResult, error) {
	return queryOnConn(ctx, conn, "EXPLAIN USING "+string(format)+" "+query)
}
