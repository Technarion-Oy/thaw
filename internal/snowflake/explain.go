// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ExplainFormat selects the output format for Snowflake's EXPLAIN command.
type ExplainFormat string

const (
	ExplainJSON    ExplainFormat = "JSON"
	ExplainTabular ExplainFormat = "TABULAR"
)

// validateExplainFormat returns an error if format is not a known constant.
func validateExplainFormat(format ExplainFormat) error {
	if format != ExplainJSON && format != ExplainTabular {
		return fmt.Errorf("snowflake: unsupported explain format %q", format)
	}
	return nil
}

// Explain runs EXPLAIN USING <format> for the given query and returns the raw
// result. No business logic — callers are responsible for parsing.
func (c *Client) Explain(ctx context.Context, query string, format ExplainFormat) (*QueryResult, error) {
	if err := validateExplainFormat(format); err != nil {
		return nil, err
	}
	return c.QuerySingle(ctx, "EXPLAIN USING "+string(format)+" "+query)
}

// ExplainOnConn runs EXPLAIN USING <format> on a pinned connection. No session
// sync is needed for EXPLAIN so this delegates directly to queryOnConn.
func (c *Client) ExplainOnConn(ctx context.Context, conn *sql.Conn, query string, format ExplainFormat) (*QueryResult, error) {
	if err := validateExplainFormat(format); err != nil {
		return nil, err
	}
	stmt := "EXPLAIN USING " + string(format) + " " + query
	start := time.Now()
	result, err := queryOnConn(ctx, conn, stmt)
	if c.OnQuery != nil {
		c.OnQuery(ctx, stmt, "", err, time.Since(start))
	}
	return result, err
}
