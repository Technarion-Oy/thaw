// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"sort"
	"strings"
)

// ObjectScope classifies where a Snowflake object lives in the namespace
// hierarchy. It is the single fact that determines how an unqualified name in DDL
// is resolved — and, in particular, whether creating an object with an
// unqualified name requires an active database and schema.
type ObjectScope int

const (
	// ScopeAccount — the object lives directly under the account
	// (DATABASE, WAREHOUSE, ROLE, USER, SHARE, RESOURCE MONITOR, integrations, …).
	ScopeAccount ObjectScope = iota
	// ScopeDatabase — the object lives in a database (SCHEMA, DATABASE ROLE).
	ScopeDatabase
	// ScopeSchema — the object lives in a database.schema (TABLE, VIEW, SEQUENCE,
	// STAGE, STREAM, TASK, FUNCTION, the policy family, …). An unqualified name
	// therefore needs both an active database and an active schema.
	ScopeSchema
	// ScopeApplication — the object lives inside a Native App (APPLICATION ROLE).
	ScopeApplication
	// ScopeOrganization — the object lives at the organization level
	// (ORGANIZATION ACCOUNT / USER / PROFILE / LISTING).
	ScopeOrganization
	// ScopeUnknown — the object's name is not a `db.schema.name` path so the
	// "needs an active database/schema" question doesn't apply (INDEX, whose name
	// is relative to its already-qualified table), or its scope is genuinely
	// ambiguous (newer/preview objects). Because ScopeSchema is the ONLY scope that
	// raises the "no database/schema selected" diagnostic, anything uncertain must
	// land here rather than ScopeSchema — picking schema-when-unsure maximizes false
	// positives, the opposite of conservative.
	ScopeUnknown
)

// String renders the scope as a lowercase word for diagnostics and logging.
func (s ObjectScope) String() string {
	switch s {
	case ScopeAccount:
		return "account"
	case ScopeDatabase:
		return "database"
	case ScopeSchema:
		return "schema"
	case ScopeApplication:
		return "application"
	case ScopeOrganization:
		return "organization"
	case ScopeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// ObjectType is one Snowflake object kind: the CREATE keyword phrase that names
// it (split into upper-case words) and the namespace scope it lives in.
type ObjectType struct {
	// Keywords is the object keyword phrase as it appears after CREATE, with any
	// leading modifiers (OR REPLACE, TEMPORARY, SECURE, MATERIALIZED, …) removed —
	// e.g. {"ROW", "ACCESS", "POLICY"} or {"SEQUENCE"}.
	Keywords []string
	Scope    ObjectScope
}

// Name returns the human-readable object type, e.g. "row access policy".
func (o ObjectType) Name() string { return strings.ToLower(strings.Join(o.Keywords, " ")) }

// ObjectTypes is the authoritative catalog of Snowflake object kinds, derived
// from the CREATE grammar in internal/sqlgrammar (every ParseCreate* rule). It is
// the single source of truth for object-type scope; consumers (e.g. the SQL
// editor's "no database/schema selected" diagnostic) must read from here rather
// than maintaining their own lists.
//
// Scope reflects Snowflake's namespace model. Because ScopeSchema is the only
// scope that raises the "no database/schema selected" diagnostic, the conservative
// classification for a genuinely ambiguous object is ScopeUnknown — NOT
// ScopeSchema — so an uncertain guess errs toward a missed warning rather than a
// false positive on valid DDL.
var ObjectTypes = []ObjectType{
	// ── Schema-scoped: tables and table-likes ────────────────────────────────
	{[]string{"TABLE"}, ScopeSchema},
	{[]string{"DYNAMIC", "TABLE"}, ScopeSchema},
	{[]string{"EVENT", "TABLE"}, ScopeSchema},
	{[]string{"EVENT", "ROUTING", "TABLE"}, ScopeSchema},
	{[]string{"EXTERNAL", "TABLE"}, ScopeSchema},
	{[]string{"HYBRID", "TABLE"}, ScopeSchema},
	{[]string{"ICEBERG", "TABLE"}, ScopeSchema},
	{[]string{"ONLINE", "FEATURE", "TABLE"}, ScopeSchema},
	// ── Schema-scoped: views ─────────────────────────────────────────────────
	// MATERIALIZED / SECURE / RECURSIVE views fold into VIEW: those words are
	// stripped as CREATE modifiers before the object keyword is matched, so a
	// separate {"MATERIALIZED","VIEW"} entry would be dead. They are still flagged
	// (scope is identical) — just labeled "view".
	{[]string{"VIEW"}, ScopeSchema},
	{[]string{"SEMANTIC", "VIEW"}, ScopeSchema},
	// ── Schema-scoped: routines ──────────────────────────────────────────────
	{[]string{"FUNCTION"}, ScopeSchema},
	{[]string{"EXTERNAL", "FUNCTION"}, ScopeSchema},
	{[]string{"DATA", "METRIC", "FUNCTION"}, ScopeSchema},
	{[]string{"PROCEDURE"}, ScopeSchema},
	// ── Schema-scoped: data movement & storage ───────────────────────────────
	{[]string{"SEQUENCE"}, ScopeSchema},
	{[]string{"STAGE"}, ScopeSchema},
	{[]string{"STREAM"}, ScopeSchema},
	{[]string{"PIPE"}, ScopeSchema},
	{[]string{"TASK"}, ScopeSchema},
	{[]string{"FILE", "FORMAT"}, ScopeSchema},
	// INDEX names a (hybrid-table) index relative to its already-qualified table —
	// `CREATE INDEX idx ON db.sch.tbl(c)`, never `db.sch.idx` — so it must NOT
	// require an active database/schema.
	{[]string{"INDEX"}, ScopeUnknown},
	{[]string{"TYPE"}, ScopeSchema},
	{[]string{"SECRET"}, ScopeSchema},
	{[]string{"TAG"}, ScopeSchema},
	{[]string{"ALERT"}, ScopeSchema},
	// ── Schema-scoped: governance policies ───────────────────────────────────
	{[]string{"MASKING", "POLICY"}, ScopeSchema},
	{[]string{"ROW", "ACCESS", "POLICY"}, ScopeSchema},
	{[]string{"AGGREGATION", "POLICY"}, ScopeSchema},
	{[]string{"PROJECTION", "POLICY"}, ScopeSchema},
	{[]string{"JOIN", "POLICY"}, ScopeSchema},
	{[]string{"PASSWORD", "POLICY"}, ScopeSchema},
	{[]string{"SESSION", "POLICY"}, ScopeSchema},
	{[]string{"AUTHENTICATION", "POLICY"}, ScopeSchema},
	{[]string{"PRIVACY", "POLICY"}, ScopeSchema},
	{[]string{"PACKAGES", "POLICY"}, ScopeSchema},
	{[]string{"FEATURE", "POLICY"}, ScopeSchema},
	{[]string{"BACKUP", "POLICY"}, ScopeSchema},
	{[]string{"MAINTENANCE", "POLICY"}, ScopeSchema},
	{[]string{"SNAPSHOT", "POLICY"}, ScopeSchema},
	{[]string{"STORAGE", "LIFECYCLE", "POLICY"}, ScopeSchema},
	{[]string{"NETWORK", "RULE"}, ScopeSchema},
	// ── Schema-scoped: AI / ML / apps-in-schema ──────────────────────────────
	{[]string{"CORTEX", "SEARCH", "SERVICE"}, ScopeSchema},
	{[]string{"MODEL"}, ScopeSchema},
	{[]string{"MODEL", "MONITOR"}, ScopeSchema},
	{[]string{"AGENT"}, ScopeSchema},
	{[]string{"EXTERNAL", "AGENT"}, ScopeSchema},
	{[]string{"MCP", "SERVER"}, ScopeSchema},
	{[]string{"NOTEBOOK"}, ScopeSchema},
	{[]string{"STREAMLIT"}, ScopeSchema},
	{[]string{"SERVICE"}, ScopeSchema},
	{[]string{"IMAGE", "REPOSITORY"}, ScopeSchema},
	{[]string{"GIT", "REPOSITORY"}, ScopeSchema},
	{[]string{"ARTIFACT", "REPOSITORY"}, ScopeSchema},
	{[]string{"DATASET"}, ScopeSchema},
	{[]string{"EXPERIMENT"}, ScopeSchema},
	// Newer/preview backup & snapshot objects — scope (and whether the name is
	// path-qualified) is not firmly documented, so keep them out of the warning.
	{[]string{"SNAPSHOT"}, ScopeUnknown},
	{[]string{"BACKUP", "SET"}, ScopeUnknown},
	{[]string{"SNAPSHOT", "SET"}, ScopeUnknown},
	{[]string{"DBT", "PROJECT"}, ScopeSchema},
	{[]string{"DCM", "PROJECT"}, ScopeSchema},
	{[]string{"NOTEBOOK", "PROJECT"}, ScopeSchema},

	// ── Database-scoped ──────────────────────────────────────────────────────
	{[]string{"SCHEMA"}, ScopeDatabase},
	{[]string{"DATABASE", "ROLE"}, ScopeDatabase},

	// ── Account-scoped ───────────────────────────────────────────────────────
	{[]string{"DATABASE"}, ScopeAccount},
	{[]string{"WAREHOUSE"}, ScopeAccount},
	{[]string{"COMPUTE", "POOL"}, ScopeAccount},
	{[]string{"ROLE"}, ScopeAccount},
	{[]string{"USER"}, ScopeAccount},
	{[]string{"SHARE"}, ScopeAccount},
	{[]string{"ACCOUNT"}, ScopeAccount},
	{[]string{"MANAGED", "ACCOUNT"}, ScopeAccount},
	{[]string{"CONNECTION"}, ScopeAccount},
	{[]string{"CONTACT"}, ScopeAccount},
	{[]string{"RESOURCE", "MONITOR"}, ScopeAccount},
	{[]string{"NETWORK", "POLICY"}, ScopeAccount},
	{[]string{"FAILOVER", "GROUP"}, ScopeAccount},
	{[]string{"REPLICATION", "GROUP"}, ScopeAccount},
	{[]string{"EXTERNAL", "VOLUME"}, ScopeAccount},
	{[]string{"LISTING"}, ScopeAccount},
	{[]string{"GATEWAY"}, ScopeAccount},
	{[]string{"POSTGRES", "INSTANCE"}, ScopeAccount},
	{[]string{"PROVISIONED", "THROUGHPUT"}, ScopeAccount},
	{[]string{"APPLICATION"}, ScopeAccount},
	{[]string{"APPLICATION", "PACKAGE"}, ScopeAccount},
	// Integrations are all account-level.
	{[]string{"INTEGRATION"}, ScopeAccount},
	{[]string{"API", "INTEGRATION"}, ScopeAccount},
	{[]string{"SECURITY", "INTEGRATION"}, ScopeAccount},
	{[]string{"STORAGE", "INTEGRATION"}, ScopeAccount},
	{[]string{"NOTIFICATION", "INTEGRATION"}, ScopeAccount},
	{[]string{"CATALOG", "INTEGRATION"}, ScopeAccount},
	{[]string{"EXTERNAL", "ACCESS", "INTEGRATION"}, ScopeAccount},

	// ── Application-scoped ───────────────────────────────────────────────────
	{[]string{"APPLICATION", "ROLE"}, ScopeApplication},
	{[]string{"APPLICATION", "SERVICE"}, ScopeApplication},

	// ── Organization-scoped ──────────────────────────────────────────────────
	{[]string{"ORGANIZATION", "ACCOUNT"}, ScopeOrganization},
	{[]string{"ORGANIZATION", "USER"}, ScopeOrganization},
	{[]string{"ORGANIZATION", "USER", "GROUP"}, ScopeOrganization},
	{[]string{"ORGANIZATION", "PROFILE"}, ScopeOrganization},
	{[]string{"ORGANIZATION", "LISTING"}, ScopeOrganization},
}

// schemaScopedTypes caches every ScopeSchema object type, sorted
// longest-keyword-phrase first so a prefix match tries the most specific phrase
// first (e.g. EVENT ROUTING TABLE before TABLE).
var schemaScopedTypes = buildSchemaScopedTypes()

func buildSchemaScopedTypes() []ObjectType {
	var out []ObjectType
	for _, ot := range ObjectTypes {
		if ot.Scope == ScopeSchema {
			out = append(out, ot)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Keywords) > len(out[j].Keywords) })
	return out
}

// SchemaScopedObjectTypes returns every schema-scoped object type, longest keyword
// phrase first. Callers match a CREATE statement's object keyword against
// ObjectType.Keywords to decide whether an unqualified name needs an active
// database + schema, and use ObjectType.Name() for the diagnostic label. The
// returned slice and its Keywords must not be mutated.
func SchemaScopedObjectTypes() []ObjectType { return schemaScopedTypes }
