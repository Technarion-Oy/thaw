// SPDX-License-Identifier: GPL-3.0-or-later

// Package externalagent builds SQL for Snowflake EXTERNAL AGENT objects — CREATE
// EXTERNAL AGENT statements and the structured config behind them. An external
// agent registers a third-party / generative-AI application in Snowflake (for use
// with AI Observability). Unlike a native AGENT it has no inline specification:
// it is version-based, where each version represents a different implementation
// (alternative retriever, prompt, LLM, or inference configuration).
//
// CREATE EXTERNAL AGENT takes an optional initial WITH VERSION name and an
// optional COMMENT. Mutations are issued as free-form ALTER EXTERNAL AGENT
// statements from internal/app/externalagent.go (App.AlterExternalAgent): SET
// COMMENT and ADD VERSION <name>. ALTER EXTERNAL AGENT has no RENAME, UNSET, or
// TAG clause. External agents share their namespace with model objects. GET_DDL
// does not support external agents.
//
// thaw:domain: Object Browser & Administration
package externalagent
