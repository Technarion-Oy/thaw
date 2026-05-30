// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package table implements Snowflake table administration: database-wide table
// summaries, modifiable table-setting retrieval (SHOW TABLES + SHOW PARAMETERS
// fallback), and ALTER TABLE property SQL builders, layered over the Snowflake
// client.
//
// thaw:domain: Object Browser & Administration
package table
