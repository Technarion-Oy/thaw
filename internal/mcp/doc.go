// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:domain: MCP Server

// Package mcp hosts Model Context Protocol servers that expose a live
// Snowflake connection to external AI clients over a localhost SSE/HTTP
// transport. It is built on the official Go MCP SDK
// (github.com/modelcontextprotocol/go-sdk/mcp).
//
// The Manager owns multiple independent sessions; each session binds a
// dedicated *snowflake.Client to its own HTTP server on a localhost port
// (auto-assigned from 9100). Sessions are started and stopped only on
// explicit user action and are all torn down on application shutdown.
//
// This package must not import internal/app to avoid an import cycle: the
// App struct holds a *Manager and delegates to it via thin IPC methods.
package mcp
