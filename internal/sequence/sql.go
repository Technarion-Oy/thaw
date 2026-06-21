// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sequence

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// SequenceConfig holds the parameters for creating a Snowflake SEQUENCE object.
// Start and Increment are emitted verbatim — the caller is expected to default
// them to 1. Ordered selects the optional ORDER / NOORDER ordering guarantee and
// is emitted only when set to one of those values.
type SequenceConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Start         int64  `json:"start"`     // START WITH
	Increment     int64  `json:"increment"` // INCREMENT BY
	Ordered       string `json:"ordered"`   // "" | "ORDER" | "NOORDER"
	Comment       string `json:"comment"`
}

// BuildCreateSequenceSql constructs a CREATE SEQUENCE statement from the given
// config. START WITH and INCREMENT BY are always emitted using the config values
// verbatim; ORDER / NOORDER is emitted only when Ordered is set to that value.
// Optional clauses follow in the order Snowflake documents them.
func BuildCreateSequenceSql(db, schema string, cfg SequenceConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SEQUENCE", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "sequence_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	fmt.Fprintf(&sb, "\n  START WITH %d", cfg.Start)
	fmt.Fprintf(&sb, "\n  INCREMENT BY %d", cfg.Increment)

	switch cfg.Ordered {
	case "ORDER":
		sb.WriteString("\n  ORDER")
	case "NOORDER":
		sb.WriteString("\n  NOORDER")
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
