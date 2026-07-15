// SPDX-License-Identifier: GPL-3.0-or-later

package storagelifecyclepolicy

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// StorageLifecycleArg is a single entry in a storage lifecycle policy's
// signature. Each argument names a column the policy body may reference to
// decide whether a row is eligible for the lifecycle action (archival, then
// expiration). Name is the parameter name used inside the body; Type is its SQL
// data type (e.g. TIMESTAMP_NTZ, DATE). When the policy is attached to a table,
// each argument is mapped to one of that table's columns. Snowflake requires at
// least one argument.
type StorageLifecycleArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// StorageLifecyclePolicyConfig holds the parameters for creating a Snowflake
// STORAGE LIFECYCLE POLICY object. The fields map to the CREATE STORAGE LIFECYCLE
// POLICY options in the order Snowflake documents them: the AS (...) signature,
// the (always BOOLEAN) RETURNS, the body expression, then the archival options
// and COMMENT. ArchiveTier is the empty string (rows expire without archiving),
// "COOL", or "COLD". ArchiveForDays is the number of days rows remain in the
// archive tier. The two are a coupled pair — Snowflake rejects a half-set
// combination — so the builder emits both or neither: the archive clauses appear
// only when ArchiveTier is set AND ArchiveForDays > 0 (the documented minimums
// are 90 days for COOL and 180 for COLD).
type StorageLifecyclePolicyConfig struct {
	Name           string                `json:"name"`
	CaseSensitive  bool                  `json:"caseSensitive"`
	OrReplace      bool                  `json:"orReplace"`
	IfNotExists    bool                  `json:"ifNotExists"`
	Args           []StorageLifecycleArg `json:"args"` // signature; columns the body evaluates
	Body           string                `json:"body"` // boolean expression deciding row eligibility
	ArchiveTier    string                `json:"archiveTier"`
	ArchiveForDays int                   `json:"archiveForDays"`
	Comment        string                `json:"comment"`
}

// BuildCreateStorageLifecyclePolicySql constructs a CREATE STORAGE LIFECYCLE
// POLICY statement from the given config. When required parts are blank the
// builder substitutes placeholders so the live preview reads as a completable
// template rather than invalid SQL.
//
//	CREATE [OR REPLACE] STORAGE LIFECYCLE POLICY [IF NOT EXISTS] <fqn> AS
//	  (<arg> <type> [, …]) RETURNS BOOLEAN -> <body>
//	  [ARCHIVE_TIER = {COOL | COLD}]
//	  [ARCHIVE_FOR_DAYS = <n>]
//	  [COMMENT = '…'];
func BuildCreateStorageLifecyclePolicySql(db, schema string, cfg StorageLifecyclePolicyConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("STORAGE LIFECYCLE POLICY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "storage_lifecycle_policy_name"
	}

	fmt.Fprintf(&sb, "%s %s AS", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// Signature: drop entries missing a name or type so a stray empty input row
	// does not emit "( TIMESTAMP_NTZ)". Snowflake requires at least one argument,
	// so if nothing valid remains emit a placeholder to keep the statement
	// well-formed.
	args := make([]string, 0, len(cfg.Args))
	for _, a := range cfg.Args {
		argName := strings.TrimSpace(a.Name)
		typ := strings.TrimSpace(a.Type)
		if argName == "" || typ == "" {
			continue
		}
		args = append(args, fmt.Sprintf("%s %s", argName, typ))
	}
	if len(args) == 0 {
		args = append(args, "val TIMESTAMP_NTZ")
	}
	fmt.Fprintf(&sb, " (%s)", strings.Join(args, ", "))

	// Storage lifecycle policies always return BOOLEAN — TRUE marks the row as
	// eligible for the lifecycle action.
	sb.WriteString(" RETURNS BOOLEAN ->")

	body := strings.TrimSpace(cfg.Body)
	if body == "" {
		body = "TRUE"
	}
	fmt.Fprintf(&sb, "\n  %s", body)

	// ARCHIVE_TIER (an unquoted COOL | COLD keyword) and ARCHIVE_FOR_DAYS are a
	// coupled pair: Snowflake validates their combined state and rejects a
	// half-set pair ("invalid property combination"). So emit both or neither —
	// only when a tier is set AND a positive day count is given. When unset the
	// rows simply expire without being archived.
	if tier := strings.ToUpper(strings.TrimSpace(cfg.ArchiveTier)); tier != "" && cfg.ArchiveForDays > 0 {
		fmt.Fprintf(&sb, "\n  ARCHIVE_TIER = %s", tier)
		fmt.Fprintf(&sb, "\n  ARCHIVE_FOR_DAYS = %d", cfg.ArchiveForDays)
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
