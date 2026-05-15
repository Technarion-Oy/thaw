// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// thaw:domain: Schema Migration

// Package migration implements the schema migration engine: scanning local SQL
// files, diffing against a live Snowflake database, deploying changes with
// retry, and generating human-readable migration scripts.
package migration
