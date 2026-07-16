// SPDX-License-Identifier: GPL-3.0-or-later

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
	TimeTravelMode  string `json:"timeTravelMode"`  // "" | "AT" | "BEFORE"
	TimeTravelKind  string `json:"timeTravelKind"`  // "TIMESTAMP" | "OFFSET" | "STATEMENT" | "STREAM"
	TimeTravelValue string `json:"timeTravelValue"` // expression / offset / query id / stream name
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

	// The optional clauses are source-type-specific (see the per-source-type
	// grammar in Snowflake's CREATE STREAM reference):
	//   - AT | BEFORE Time Travel:      TABLE, EXTERNAL TABLE, VIEW
	//   - APPEND_ONLY / SHOW_INITIAL_ROWS: TABLE, VIEW
	//   - INSERT_ONLY:                  EXTERNAL TABLE (required, but optional here)
	//   - STAGE / DYNAMIC TABLE:        none of the above
	// The create modal already gates its inputs this way, but enforce it here too
	// so a clause can never leak into a CREATE STREAM for a source type that
	// rejects it (defense-in-depth for any caller that drives the builder directly).
	st := strings.ToUpper(sourceType)
	timeTravelSource := st == "TABLE" || st == "EXTERNAL TABLE" || st == "VIEW"
	rowChangeSource := st == "TABLE" || st == "VIEW"
	externalSource := st == "EXTERNAL TABLE"

	if timeTravelSource {
		if tt := timeTravelClause(cfg.TimeTravelMode, cfg.TimeTravelKind, cfg.TimeTravelValue); tt != "" {
			sb.WriteString(tt)
		}
	}
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

// timeTravelClause builds the optional `{ AT | BEFORE } ( <kind> => <value> )`
// Time Travel clause. It returns "" unless both mode and value are set. TIMESTAMP
// and OFFSET take a raw SQL expression / signed number and are emitted verbatim;
// STATEMENT (a query id) and STREAM (a stream name) are string literals and are
// quoted. An unrecognized kind falls back to STREAM's string-literal quoting,
// which is the safe default for a name-like value.
func timeTravelClause(mode, kind, value string) string {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	value = strings.TrimSpace(value)
	if (mode != "AT" && mode != "BEFORE") || value == "" {
		return ""
	}

	kind = strings.ToUpper(strings.TrimSpace(kind))
	if kind == "" {
		kind = "TIMESTAMP"
	}

	var arg string
	switch kind {
	case "TIMESTAMP", "OFFSET":
		arg = value
	default: // STATEMENT, STREAM
		arg = snowflake.QuoteStringLit(value)
	}

	return fmt.Sprintf("\n  %s ( %s => %s )", mode, kind, arg)
}
