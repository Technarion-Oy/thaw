// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package alert

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AlertConfig holds the parameters for creating a Snowflake ALERT object. The
// condition (Condition) is wrapped in the mandatory IF (EXISTS (...)) clause and
// the action (Action) follows the THEN keyword; the remaining fields map to the
// alert-level CREATE ALERT options in the order Snowflake documents them.
//
// Warehouse is optional: an empty value produces a serverless alert (Snowflake
// supplies the compute). When set, the alert runs on the named user-managed
// warehouse. CONFIG, RUNBOOK, and SUSPEND_ALERT_AFTER_NUM_FAILURES are
// intentionally out of scope for the visual builder and are left to raw SQL.
type AlertConfig struct {
	Name          string              `json:"name"`
	CaseSensitive bool                `json:"caseSensitive"`
	OrReplace     bool                `json:"orReplace"`
	IfNotExists   bool                `json:"ifNotExists"`
	Warehouse     string              `json:"warehouse"` // user-managed warehouse, or "" for a serverless alert
	Schedule      string              `json:"schedule"`  // e.g. "60 MINUTE" or "USING CRON 0 9 * * * UTC"
	Comment       string              `json:"comment"`
	Tags          []snowflake.TagPair `json:"tags"`      // alert-level WITH TAG (name = 'value', ...)
	Condition     string              `json:"condition"` // the query inside IF (EXISTS (...))
	Action        string              `json:"action"`    // the statement after THEN
}

// trimStmt strips surrounding whitespace and any trailing semicolons from a
// user-supplied SQL fragment so the builder controls statement termination.
func trimStmt(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";")
	return strings.TrimSpace(s)
}

// BuildCreateAlertSql constructs a CREATE ALERT statement from the given config.
// A schedule, a condition, and an action are required by Snowflake; when any is
// empty the builder emits a placeholder so the preview remains a syntactically
// obvious template the user can complete. Optional clauses are emitted only when
// set, in the order Snowflake documents them: TAG, SCHEDULE, WAREHOUSE, COMMENT,
// then the mandatory IF (EXISTS (<condition>)) THEN <action>.
func BuildCreateAlertSql(db, schema string, cfg AlertConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " ALERT"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "alert_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  WITH %s", tc)
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "60 MINUTE"
	}
	fmt.Fprintf(&sb, "\n  SCHEDULE = '%s'", snowflake.EscapeStringLit(schedule))

	if wh := strings.TrimSpace(cfg.Warehouse); wh != "" {
		fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", snowflake.QuoteIdent(wh))
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	condition := trimStmt(cfg.Condition)
	if condition == "" {
		condition = "SELECT 1 FROM my_table WHERE <condition>"
	}
	action := trimStmt(cfg.Action)
	if action == "" {
		action = "INSERT INTO my_alert_log SELECT CURRENT_TIMESTAMP()"
	}

	// The IF (EXISTS (<condition>)) wrapper is the only documented CREATE ALERT
	// form and is mandatory for every permitted condition command — SELECT, SHOW,
	// and CALL all go inside EXISTS (per the CREATE ALERT grammar). The condition
	// editor's "Insert CALL…" helper therefore correctly yields
	// IF (EXISTS (CALL my_proc(...))); the wrapper is intentionally unconditional
	// and is not special-cased per condition kind.
	fmt.Fprintf(&sb, "\nIF (EXISTS (\n%s\n))\nTHEN\n%s", condition, action)

	return sb.String() + ";", nil
}
