// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
	"thaw/internal/version"
)

// buildServer constructs an MCP server exposing the proof-of-life
// schema-browsing tools backed by the supplied Snowflake client. The mode
// parameter is reserved for future execution gating; the foundation milestone
// only registers read-only metadata tools.
func buildServer(client *snowflake.Client, mode string) *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "thaw",
		Version: version.Version,
	}, nil)

	registerTools(srv, client)
	return srv
}
