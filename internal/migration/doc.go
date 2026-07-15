// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:domain: Schema Migration

// Package migration implements the schema migration engine: scanning local SQL
// files, diffing against a live Snowflake database, deploying changes with
// retry, and generating human-readable migration scripts.
package migration
