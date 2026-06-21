// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package stream

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// StreamConfig holds the parameters for creating a Snowflake STREAM object. A
// stream tracks change data (CDC) on a source object identified by SourceType
// (TABLE / VIEW / EXTERNAL TABLE / STAGE / DYNAMIC TABLE) and Source. The
// change-tracking flags map to the CREATE STREAM options in the order Snowflake
// documents them.
type StreamConfig struct {
	Name            string `json:"name"`
	CaseSensitive   bool   `json:"caseSensitive"`
	OrReplace       bool   `json:"orReplace"`
	IfNotExists     bool   `json:"ifNotExists"`
	CopyGrants      bool   `json:"copyGrants"`
	SourceType      string `json:"sourceType"`      // "TABLE" | "VIEW" | "EXTERNAL TABLE" | "STAGE" | "DYNAMIC TABLE"
	Source          string `json:"source"`          // source object name (db.schema.obj or bare)
	AppendOnly      bool   `json:"appendOnly"`      // APPEND_ONLY = TRUE
	ShowInitialRows bool   `json:"showInitialRows"` // SHOW_INITIAL_ROWS = TRUE
	InsertOnly      bool   `json:"insertOnly"`      // INSERT_ONLY = TRUE
	Comment         string `json:"comment"`
}

const sourcePlaceholder = "<source_object>"

// BuildCreateStreamSql constructs a CREATE STREAM statement from the given
// config. The source object is required by Snowflake; when it is empty the
// builder emits a placeholder so the preview reads as a completable template. A
// source name that already contains a "." is treated as fully qualified and
// emitted verbatim; a bare name is qualified with the active db/schema. Optional
// clauses are emitted only when set, in the order Snowflake documents them.
func BuildCreateStreamSql(db, schema string, cfg StreamConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("STREAM", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "stream_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	if cfg.CopyGrants {
		fmt.Fprintf(&sb, "\n  COPY GRANTS")
	}

	sourceType := strings.TrimSpace(cfg.SourceType)
	if sourceType == "" {
		sourceType = "TABLE"
	}

	source := strings.TrimSpace(cfg.Source)
	var sourceIdent string
	switch {
	case source == "":
		sourceIdent = sourcePlaceholder
	case strings.Contains(source, "."):
		sourceIdent = source
	default:
		sourceIdent = snowflake.Qualify(db, schema, source)
	}

	fmt.Fprintf(&sb, "\n  ON %s %s", sourceType, sourceIdent)

	// The CDC flags are source-type-specific: APPEND_ONLY / SHOW_INITIAL_ROWS
	// apply to streams on tables, views and dynamic tables; INSERT_ONLY applies
	// only to streams on external tables. The create modal already gates the
	// checkboxes this way, but enforce it here too so a flag can never leak into a
	// CREATE STREAM for a source type that rejects it (defense-in-depth for any
	// caller that drives the builder directly).
	st := strings.ToUpper(sourceType)
	rowChangeSource := st == "TABLE" || st == "VIEW" || st == "DYNAMIC TABLE"
	externalSource := st == "EXTERNAL TABLE"

	if cfg.AppendOnly && rowChangeSource {
		fmt.Fprintf(&sb, "\n  APPEND_ONLY = TRUE")
	}
	if cfg.ShowInitialRows && rowChangeSource {
		fmt.Fprintf(&sb, "\n  SHOW_INITIAL_ROWS = TRUE")
	}
	if cfg.InsertOnly && externalSource {
		fmt.Fprintf(&sb, "\n  INSERT_ONLY = TRUE")
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
