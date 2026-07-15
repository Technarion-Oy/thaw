// SPDX-License-Identifier: GPL-3.0-or-later

// Package agent builds SQL for Snowflake AGENT objects — CREATE AGENT statements
// and the structured config behind them. An agent is a schema-level Cortex AI
// object that combines LLM reasoning with tool use (Cortex Analyst, Cortex
// Search, custom SQL/procedures, …). Its behavior — orchestration model,
// instructions, tools, and tool resources — is supplied as a YAML or JSON
// specification (max 100,000 bytes) via FROM SPECIFICATION $THAW$ … $THAW$; an optional
// PROFILE JSON object carries display metadata (display_name, avatar, color).
//
// The mutable properties are issued as free-form ALTER AGENT statements from
// internal/app/agent.go (App.AlterAgent): SET COMMENT / SET PROFILE, and the
// live specification is replaced wholesale with
// MODIFY LIVE VERSION SET SPECIFICATION = $THAW$ … $THAW$. The full specification (which
// SHOW AGENTS omits) is read back with DESCRIBE AGENT. ALTER AGENT has no RENAME,
// UNSET, or TAG clause. GET_DDL supports agents via the CORTEX_AGENT object type.
//
// thaw:domain: Object Browser & Administration
package agent
