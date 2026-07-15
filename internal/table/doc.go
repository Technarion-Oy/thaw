// SPDX-License-Identifier: GPL-3.0-or-later

// Package table implements Snowflake table administration: database-wide table
// summaries, modifiable table-setting retrieval (SHOW TABLES + SHOW PARAMETERS
// fallback), and ALTER TABLE property SQL builders, layered over the Snowflake
// client.
//
// thaw:domain: Object Browser & Administration
package table
