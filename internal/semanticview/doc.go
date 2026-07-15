// SPDX-License-Identifier: GPL-3.0-or-later

// Package semanticview builds SQL for Snowflake SEMANTIC VIEW objects — CREATE
// SEMANTIC VIEW statements and the structured config behind them. A semantic
// view defines a semantic layer over physical tables for natural-language
// querying with Cortex Analyst: it names logical TABLES, the RELATIONSHIPS
// between them, and the FACTS / DIMENSIONS / METRICS that describe the business
// meaning of the data.
//
// CREATE SEMANTIC VIEW has a rich, order-sensitive body (TABLES → RELATIONSHIPS
// → FACTS → DIMENSIONS → METRICS) whose per-entity sub-grammar is far too large
// for a structured form, so the builder takes the body verbatim from a Monaco
// editor and frames it with the CREATE prefix, an optional COMMENT, and an
// optional COPY GRANTS. SHOW SEMANTIC VIEWS reports only metadata (owner,
// comment); the full structure comes from DESCRIBE SEMANTIC VIEW and the
// SHOW SEMANTIC DIMENSIONS / FACTS / METRICS commands, read by the properties
// panel. ALTER SEMANTIC VIEW only changes the comment, tags, or name; the body
// is changed via CREATE OR REPLACE. GET_DDL supports semantic views directly
// (object_type 'SEMANTIC VIEW').
//
// thaw:domain: Object Browser & Administration
package semanticview
