// SPDX-License-Identifier: GPL-3.0-or-later

//go:build production && mcp

package app

// Compile-time refusal: Wails v3's built-in app-control driver (enabled by the
// upstream `mcp` build tag) exposes arbitrary JavaScript evaluation, DOM
// inspection, and bound-method invocation over localhost HTTP. It must NEVER be
// compiled into a production/release build.
//
// This build tag is unrelated to Thaw's own Snowflake MCP server in internal/mcp.
var _ = productionBuildsMustNotEnableWailsAppControlDriver
