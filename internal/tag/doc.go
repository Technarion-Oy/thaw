// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package tag builds SQL for Snowflake tag objects — CREATE TAG statements and
// the structured config behind them, plus the ALTER <object> SET/UNSET TAG
// statements (BuildAlterObjectTagSql) that apply or remove a tag on another
// object. Tags are part of Snowflake's governance framework: named metadata that
// can be attached to other objects and columns for classification, lineage, and
// policy enforcement. The tag's own ALTER clauses (RENAME, SET/UNSET COMMENT,
// ADD/DROP/UNSET ALLOWED_VALUES, SET/UNSET PROPAGATE, UNSET ON_CONFLICT,
// SET/UNSET MASKING POLICY) are simple enough to be issued as free-form ALTER
// TAG statements from internal/app/tag.go (App.AlterTag).
//
// thaw:domain: Object Browser & Administration
package tag
