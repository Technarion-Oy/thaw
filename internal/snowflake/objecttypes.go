// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
// Scope reflects Snowflake's namespace model. When a newer/rarely-used object's
// scope is genuinely ambiguous it is classified conservatively so the editor does
// not raise a false "missing database/schema" warning for it.
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
	{[]string{"VIEW"}, ScopeSchema},
	{[]string{"MATERIALIZED", "VIEW"}, ScopeSchema},
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
	{[]string{"INDEX"}, ScopeSchema},
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
	{[]string{"SNAPSHOT"}, ScopeSchema},
	{[]string{"BACKUP", "SET"}, ScopeSchema},
	{[]string{"SNAPSHOT", "SET"}, ScopeSchema},
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
