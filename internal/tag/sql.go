// SPDX-License-Identifier: GPL-3.0-or-later

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

	createClause := snowflake.CreateClause("TAG", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "tag_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

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

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}

// ObjectTagRef identifies an object (or a column on an object) to which a tag is
// applied, mirroring a row of SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES. Domain is
// the object kind (e.g. TABLE, VIEW, SCHEMA, DATABASE, WAREHOUSE, COLUMN, …) as
// reported in the TAG_REFERENCES DOMAIN column. Database/Schema/Name are the
// object's name parts (some are empty for account-level objects); Column is set
// only when Domain is COLUMN.
//
// ParentKind is the kind of the object that owns a tagged column (TABLE or VIEW)
// and is consulted only when Domain is COLUMN, to pick the correct ALTER keyword
// — a column tag on a view needs `ALTER VIEW … ALTER COLUMN`, not `ALTER TABLE`.
// An empty ParentKind defaults to TABLE; callers that can't determine it (e.g.
// the ACCOUNT_USAGE references browser, where the parent kind isn't reported)
// resolve it before building the statement.
type ObjectTagRef struct {
	Domain     string `json:"domain"`
	Database   string `json:"database"`
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	Column     string `json:"column"`
	ParentKind string `json:"parentKind"`
}

// columnParentAlterType returns the ALTER object-type keyword for setting or
// unsetting a tag on a column, based on the kind of the object that owns the
// column. Snowflake exposes column tags via `ALTER TABLE … ALTER COLUMN` and
// `ALTER VIEW … ALTER COLUMN`; a VIEW parent yields VIEW, and any other (or an
// empty/unknown) kind defaults to TABLE.
func columnParentAlterType(parentKind string) string {
	if strings.EqualFold(strings.TrimSpace(parentKind), "VIEW") {
		return "VIEW"
	}
	return "TABLE"
}

// qualifyNonEmpty joins the non-empty parts into a dotted, double-quoted
// reference, skipping blanks. qualifyNonEmpty("DB", "", "T") yields `"DB"."T"`.
func qualifyNonEmpty(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		quoted = append(quoted, snowflake.QuoteIdent(p))
	}
	return strings.Join(quoted, ".")
}

// BuildAlterObjectTagSql constructs an `ALTER <object> SET/UNSET TAG` statement
// that applies (SET, with value), retags (SET, new value), or removes (UNSET) a
// tag on the object described by ref. tagFQN is the fully-qualified, quoted tag
// name (e.g. `"DB"."S"."COST_CENTER"`); value is the tag value for SET and is
// ignored when unset is true.
//
// The object reference is derived from ref.Domain:
//
//   - ACCOUNT             → `ALTER ACCOUNT …` (no name)
//
//   - DATABASE            → `ALTER DATABASE "<name>" …`
//
//   - SCHEMA              → `ALTER SCHEMA "<db>"."<name>" …`
//
//   - COLUMN              → `ALTER <TABLE|VIEW> "<db>"."<sc>"."<obj>" ALTER COLUMN "<col>" …`
//     (VIEW when ref.ParentKind is VIEW, else TABLE)
//
//   - everything else     → `ALTER <DOMAIN> <qualified-name> …` where the name is
//     built from whichever of database/schema/name are present (so schema-level
//     objects get a three-part name and account-level objects a bare name).
//
//     ALTER TABLE "DB"."SC"."T"   SET TAG "DB"."SC"."PII" = 'true'
//     ALTER WAREHOUSE "WH"        UNSET TAG "DB"."SC"."COST_CENTER"
//     ALTER TABLE "DB"."SC"."T" ALTER COLUMN "EMAIL" SET TAG "DB"."SC"."PII" = 'true'
func BuildAlterObjectTagSql(ref ObjectTagRef, tagFQN, value string, unset bool) (string, error) {
	domain := strings.ToUpper(strings.TrimSpace(ref.Domain))
	if domain == "" {
		return "", fmt.Errorf("tag reference is missing an object domain")
	}
	if strings.TrimSpace(tagFQN) == "" {
		return "", fmt.Errorf("tag reference is missing a tag name")
	}

	var action string
	if unset {
		action = fmt.Sprintf("UNSET TAG %s", tagFQN)
	} else {
		action = fmt.Sprintf("SET TAG %s = '%s'", tagFQN, snowflake.EscapeStringLit(value))
	}

	var objectType, refClause string
	switch domain {
	case "ACCOUNT":
		objectType = "ACCOUNT"
	case "COLUMN":
		if strings.TrimSpace(ref.Column) == "" {
			return "", fmt.Errorf("column tag reference is missing a column name")
		}
		objectType = columnParentAlterType(ref.ParentKind)
		refClause = qualifyNonEmpty(ref.Database, ref.Schema, ref.Name) +
			" ALTER COLUMN " + snowflake.QuoteIdent(ref.Column)
	case "DATABASE":
		objectType = "DATABASE"
		// TAG_REFERENCES reports the database in OBJECT_NAME (and may repeat it in
		// OBJECT_DATABASE); the reference is a single bare identifier.
		refClause = snowflake.QuoteIdent(snowflake.FirstNonEmpty(ref.Name, ref.Database))
	case "SCHEMA":
		objectType = "SCHEMA"
		refClause = qualifyNonEmpty(ref.Database, snowflake.FirstNonEmpty(ref.Name, ref.Schema))
	default:
		objectType = domain
		refClause = qualifyNonEmpty(ref.Database, ref.Schema, ref.Name)
	}

	if domain != "ACCOUNT" && refClause == "" {
		return "", fmt.Errorf("tag reference for domain %s is missing an object name", domain)
	}

	if refClause == "" {
		return fmt.Sprintf("ALTER %s %s;", objectType, action), nil
	}
	return fmt.Sprintf("ALTER %s %s %s;", objectType, refClause, action), nil
}
