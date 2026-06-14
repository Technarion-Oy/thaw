// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package tag

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// AllowedValuesSequence is the ON_CONFLICT sentinel that resolves propagation
// conflicts using the order of the tag's ALLOWED_VALUES list (emitted as the
// bare keyword ALLOWED_VALUES_SEQUENCE rather than a quoted string literal).
const AllowedValuesSequence = "ALLOWED_VALUES_SEQUENCE"

// TagConfig holds the parameters for creating a Snowflake TAG object. The fields
// map to the CREATE TAG options in the order Snowflake documents them:
// ALLOWED_VALUES, PROPAGATE (with its nested ON_CONFLICT), then COMMENT. A tag
// with no allowed values accepts any string; supplying AllowedValues restricts
// the values that may be assigned when the tag is applied to an object or column.
type TagConfig struct {
	Name          string   `json:"name"`
	CaseSensitive bool     `json:"caseSensitive"`
	OrReplace     bool     `json:"orReplace"`
	IfNotExists   bool     `json:"ifNotExists"`
	AllowedValues []string `json:"allowedValues"` // optional whitelist of permitted tag values
	// Propagate enables tag lineage propagation from source to target objects.
	// Empty disables it; otherwise one of ON_DEPENDENCY_AND_DATA_MOVEMENT,
	// ON_DEPENDENCY, ON_DATA_MOVEMENT. ON_CONFLICT is only emitted alongside it.
	Propagate string `json:"propagate"`
	// OnConflict resolves conflicts between propagated tag values. Empty omits
	// the clause; the sentinel AllowedValuesSequence emits the bare keyword
	// ALLOWED_VALUES_SEQUENCE; any other value is emitted as a quoted string
	// literal. Ignored unless Propagate is set.
	OnConflict string `json:"onConflict"`
	Comment    string `json:"comment"`
}

// validPropagateModes is the set of accepted PROPAGATE values.
var validPropagateModes = map[string]bool{
	"ON_DEPENDENCY_AND_DATA_MOVEMENT": true,
	"ON_DEPENDENCY":                   true,
	"ON_DATA_MOVEMENT":                true,
}

// BuildCreateTagSql constructs a CREATE TAG statement from the given config.
// Only a name is required; ALLOWED_VALUES and COMMENT are emitted only when set,
// in the order Snowflake documents them. When the name is empty the builder
// emits a placeholder so the live preview reads as a completable template.
//
//	CREATE [OR REPLACE] TAG [IF NOT EXISTS] <fqn>
//	  [ALLOWED_VALUES 'v1', 'v2', …]
//	  [PROPAGATE = <mode> [ON_CONFLICT = {'…' | ALLOWED_VALUES_SEQUENCE}]]
//	  [COMMENT = '…'];
func BuildCreateTagSql(db, schema string, cfg TagConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " TAG"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "tag_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	// ALLOWED_VALUES takes a comma-separated list of string literals. Blank
	// entries (after trimming) are skipped so a stray empty input row does not
	// emit '' as a permitted value.
	vals := make([]string, 0, len(cfg.AllowedValues))
	for _, v := range cfg.AllowedValues {
		if strings.TrimSpace(v) == "" {
			continue
		}
		vals = append(vals, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(v)))
	}
	if len(vals) > 0 {
		fmt.Fprintf(&sb, "\n  ALLOWED_VALUES %s", strings.Join(vals, ", "))
	}

	// PROPAGATE (with its nested ON_CONFLICT) is only emitted when a valid
	// propagation mode is set; ON_CONFLICT has no meaning on its own.
	if mode := strings.ToUpper(strings.TrimSpace(cfg.Propagate)); validPropagateModes[mode] {
		fmt.Fprintf(&sb, "\n  PROPAGATE = %s", mode)
		if oc := strings.TrimSpace(cfg.OnConflict); oc != "" {
			if strings.EqualFold(oc, AllowedValuesSequence) {
				fmt.Fprintf(&sb, " ON_CONFLICT = %s", AllowedValuesSequence)
			} else {
				fmt.Fprintf(&sb, " ON_CONFLICT = '%s'", snowflake.EscapeStringLit(oc))
			}
		}
	}

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	return sb.String() + ";", nil
}
