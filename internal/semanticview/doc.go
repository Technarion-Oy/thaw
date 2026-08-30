// SPDX-License-Identifier: GPL-3.0-or-later

// Package semanticview builds SQL for Snowflake SEMANTIC VIEW objects — CREATE
// SEMANTIC VIEW statements and the structured config behind them. A semantic
// view defines a semantic layer over physical tables for natural-language
// querying with Cortex Analyst: it names logical TABLES, the RELATIONSHIPS
// between them, and the FACTS / DIMENSIONS / METRICS that describe the business
// meaning of the data.
//
// CREATE SEMANTIC VIEW has a rich, order-sensitive body (TABLES → RELATIONSHIPS
// → FACTS → DIMENSIONS → METRICS). Each clause is modeled structurally
// (LogicalTable, Relationship, Expression) so the create modal can drive it from
// form controls and the builder — not the user — guarantees the clause order;
// SemanticViewConfig.Body remains as a raw-SQL escape hatch that replaces the
// whole structured definition when set. The view-level options (COMMENT,
// MAX_STALENESS, AI_SQL_GENERATION, AI_QUESTION_CATEGORIZATION,
// AI_VERIFIED_QUERIES, WITH TAG, COPY GRANTS) follow the body.
// SHOW SEMANTIC VIEWS reports only metadata (owner,
// comment); the full structure comes from DESCRIBE SEMANTIC VIEW and the
// SHOW SEMANTIC DIMENSIONS / FACTS / METRICS commands, read by the properties
// panel. ALTER SEMANTIC VIEW only changes the comment, tags, or name; the body
// is changed via CREATE OR REPLACE. GET_DDL supports semantic views directly
// (object_type 'SEMANTIC VIEW').
//
// thaw:domain: Object Browser & Administration
package semanticview
