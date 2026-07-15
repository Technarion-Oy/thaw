// SPDX-License-Identifier: GPL-3.0-or-later

// Package mcpserver builds SQL for Snowflake MCP SERVER objects — CREATE MCP
// SERVER statements and the structured config behind them. An MCP server
// (Model Context Protocol) is a schema-level object that exposes Snowflake
// tools and resources — Cortex Search services, Cortex Analyst semantic views,
// SQL execution, Cortex agents, and generic UDFs / stored procedures — to MCP
// clients via a single YAML specification.
//
// CREATE MCP SERVER takes only a required FROM SPECIFICATION body (the tools
// YAML, emitted inside a tagged dollar-quote so multi-line YAML needs no
// escaping). There is no COMMENT clause, and Snowflake has no ALTER MCP SERVER
// statement: a server is mutated by re-issuing CREATE OR REPLACE with a new
// specification. SHOW MCP SERVERS reports only metadata (owner, comment); the
// full spec comes from DESCRIBE MCP SERVER (server_spec column), read by the
// properties panel via App.DescribeMCPServer. GET_DDL does not support MCP
// servers (handled by an exclusion in internal/snowflake).
//
// thaw:domain: Object Browser & Administration
package mcpserver
