// SPDX-License-Identifier: GPL-3.0-or-later

package objectkind

import "strings"

// Kind is everything the app needs to know about one Snowflake object kind.
//
// Adding support for a new kind is a single entry in [Kinds] plus an icon and
// color in frontend/src/components/sidebar/objectIcons.tsx — every other
// consumer derives from this struct, and the coverage tests fail if a consumer
// is left unwired.
type Kind struct {
	// Name is the canonical KIND string used everywhere in the app: the kind
	// column of SHOW OBJECTS, the sidebar node keys, the IPC arguments. Always
	// upper case and space-separated ("MATERIALIZED VIEW", never
	// "MATERIALIZED_VIEW").
	Name string

	// Plural is the noun in the kind's dedicated SHOW command — "SHOW <Plural> IN
	// SCHEMA …" for the object tree and Properties, "SHOW <Plural> IN ACCOUNT" for
	// the account-wide search. Set even for Basic kinds, which use it for
	// Properties although the tree sources them from SHOW OBJECTS.
	Plural string

	// Label is the pluralised display label for the tree group and the search
	// type filter ("Materialized Views").
	Label string

	// Basic marks the kinds SHOW OBJECTS already returns (TABLE, VIEW, SEQUENCE).
	// They are listed from that one command rather than a dedicated SHOW, so they
	// are excluded from the extended command list and from the per-kind
	// account-search commands.
	Basic bool

	// GetDDLType is the object_type GET_DDL expects for this kind, which is not
	// always the SHOW kind: some use the underscore form ("DYNAMIC_TABLE"), some
	// are folded into a broader type (the policy family → "POLICY", Iceberg and
	// hybrid tables → "TABLE", Cortex agents → "CORTEX_AGENT"). An empty string
	// means GET_DDL has no object type for the kind at all — DDL export, View
	// Definition and schema comparison are unavailable and Client.GetObjectDDL
	// rejects it up front rather than firing a doomed query.
	GetDDLType string

	// Routine marks kinds that overload by argument signature (functions and
	// procedures), so GET_DDL needs the parameter type list appended to the
	// identifier to resolve the right overload — omitting the parentheses makes
	// GET_DDL report "Object does not exist" even when it does.
	Routine bool

	// NoPropertiesQuery opts a kind out of the generic
	// objects.BuildObjectPropertiesQuery SHOW: its properties are read some other
	// way, so a generic query would be dead code. Only NOTEBOOK sets it (the
	// notebook UI reads the columns it needs through the dedicated notebook IPC
	// methods, not the Properties modal).
	NoPropertiesQuery bool
}

// Kinds is the canonical, ordered registry of every schema-scoped object kind
// the object browser supports. The order is the display order: it drives the
// sidebar tree grouping, the search type-filter options and the search result
// grouping (via the generated frontend artifact), and — order being irrelevant
// there, the commands run concurrently and the results are regrouped — the
// extended SHOW command list too.
//
// Account- and database-scoped kinds (DATABASE, SCHEMA, WAREHOUSE, ROLE, USER)
// are deliberately absent: they never appear as objects in the tree, are not
// listed by SHOW OBJECTS, and each needs its own scope clause, so their handful
// of consumers name them explicitly.
var Kinds = []Kind{
	{Name: "TABLE", Plural: "TABLES", Label: "Tables", Basic: true, GetDDLType: "TABLE"},
	{Name: "VIEW", Plural: "VIEWS", Label: "Views", Basic: true, GetDDLType: "VIEW"},
	// GET_DDL has no MATERIALIZED_VIEW object type — TABLE and VIEW are
	// interchangeable and materialized views are retrieved via 'VIEW'.
	{Name: "MATERIALIZED VIEW", Plural: "MATERIALIZED VIEWS", Label: "Materialized Views", GetDDLType: "VIEW"},
	{Name: "DYNAMIC TABLE", Plural: "DYNAMIC TABLES", Label: "Dynamic Tables", GetDDLType: "DYNAMIC_TABLE"},
	{Name: "EXTERNAL TABLE", Plural: "EXTERNAL TABLES", Label: "External Tables", GetDDLType: "EXTERNAL_TABLE"},
	// Iceberg and hybrid tables have no dedicated GET_DDL object type; both are
	// retrieved via 'TABLE'.
	{Name: "ICEBERG TABLE", Plural: "ICEBERG TABLES", Label: "Iceberg Tables", GetDDLType: "TABLE"},
	{Name: "HYBRID TABLE", Plural: "HYBRID TABLES", Label: "Hybrid Tables", GetDDLType: "TABLE"},
	// Event tables do have a dedicated GET_DDL type; only the underscore form differs.
	{Name: "EVENT TABLE", Plural: "EVENT TABLES", Label: "Event Tables", GetDDLType: "EVENT_TABLE"},
	{Name: "FUNCTION", Plural: "FUNCTIONS", Label: "Functions", GetDDLType: "FUNCTION", Routine: true},
	// GET_DDL has no EXTERNAL_FUNCTION / DATA_METRIC_FUNCTION object type — both
	// are retrieved via 'FUNCTION' with the argument signature appended. Both also
	// surface in SHOW FUNCTIONS (relabeled from the is_external_function /
	// is_data_metric discriminator column) alongside their dedicated SHOW, which
	// snowflake.dedupeFunctionVariant reconciles.
	{Name: "EXTERNAL FUNCTION", Plural: "EXTERNAL FUNCTIONS", Label: "External Functions", GetDDLType: "FUNCTION", Routine: true},
	{Name: "DATA METRIC FUNCTION", Plural: "DATA METRIC FUNCTIONS", Label: "Data Metric Functions", GetDDLType: "FUNCTION", Routine: true},
	{Name: "PROCEDURE", Plural: "PROCEDURES", Label: "Procedures", GetDDLType: "PROCEDURE", Routine: true},
	{Name: "SEQUENCE", Plural: "SEQUENCES", Label: "Sequences", Basic: true, GetDDLType: "SEQUENCE"},
	{Name: "STAGE", Plural: "STAGES", Label: "Stages", GetDDLType: "STAGE"},
	{Name: "STREAM", Plural: "STREAMS", Label: "Streams", GetDDLType: "STREAM"},
	{Name: "TASK", Plural: "TASKS", Label: "Tasks", GetDDLType: "TASK"},
	{Name: "ALERT", Plural: "ALERTS", Label: "Alerts", GetDDLType: "ALERT"},
	{Name: "TAG", Plural: "TAGS", Label: "Tags", GetDDLType: "TAG"},
	// GET_DDL exposes a single 'POLICY' object type covering the policy family
	// rather than a per-kind type. PACKAGES POLICY is deliberately NOT one of
	// them: GET_DDL supports neither 'POLICY' nor a 'PACKAGES POLICY' type for it
	// (the call fails with "Cannot initialize Snowflake Metadata. Dictionary
	// unavailable"), so it has no GET_DDL mapping at all.
	{Name: "MASKING POLICY", Plural: "MASKING POLICIES", Label: "Masking Policies", GetDDLType: "POLICY"},
	{Name: "ROW ACCESS POLICY", Plural: "ROW ACCESS POLICIES", Label: "Row Access Policies", GetDDLType: "POLICY"},
	{Name: "JOIN POLICY", Plural: "JOIN POLICIES", Label: "Join Policies", GetDDLType: "POLICY"},
	{Name: "PRIVACY POLICY", Plural: "PRIVACY POLICIES", Label: "Privacy Policies", GetDDLType: "POLICY"},
	{Name: "STORAGE LIFECYCLE POLICY", Plural: "STORAGE LIFECYCLE POLICIES", Label: "Storage Lifecycle Policies", GetDDLType: "POLICY"},
	{Name: "PASSWORD POLICY", Plural: "PASSWORD POLICIES", Label: "Password Policies", GetDDLType: "POLICY"},
	{Name: "SESSION POLICY", Plural: "SESSION POLICIES", Label: "Session Policies", GetDDLType: "POLICY"},
	{Name: "AGGREGATION POLICY", Plural: "AGGREGATION POLICIES", Label: "Aggregation Policies", GetDDLType: "POLICY"},
	{Name: "PROJECTION POLICY", Plural: "PROJECTION POLICIES", Label: "Projection Policies", GetDDLType: "POLICY"},
	{Name: "AUTHENTICATION POLICY", Plural: "AUTHENTICATION POLICIES", Label: "Authentication Policies", GetDDLType: "POLICY"},
	{Name: "PACKAGES POLICY", Plural: "PACKAGES POLICIES", Label: "Packages Policies"},
	{Name: "NETWORK RULE", Plural: "NETWORK RULES", Label: "Network Rules", GetDDLType: "NETWORK_RULE"},
	// Snowpark Container Services objects: GET_DDL has no 'IMAGE REPOSITORY',
	// 'SERVICE' or 'GATEWAY' object type.
	{Name: "IMAGE REPOSITORY", Plural: "IMAGE REPOSITORIES", Label: "Image Repositories"},
	{Name: "SERVICE", Plural: "SERVICES", Label: "Services"},
	{Name: "GATEWAY", Plural: "GATEWAYS", Label: "Gateways"},
	{Name: "CONTACT", Plural: "CONTACTS", Label: "Contacts", GetDDLType: "CONTACT"},
	{Name: "STREAMLIT", Plural: "STREAMLITS", Label: "Streamlits", GetDDLType: "STREAMLIT"},
	{Name: "FILE FORMAT", Plural: "FILE FORMATS", Label: "File Formats", GetDDLType: "FILE FORMAT"},
	{Name: "PIPE", Plural: "PIPES", Label: "Pipes", GetDDLType: "PIPE"},
	{Name: "NOTEBOOK", Plural: "NOTEBOOKS", Label: "Notebooks", GetDDLType: "NOTEBOOK", NoPropertiesQuery: true},
	{Name: "SECRET", Plural: "SECRETS", Label: "Secrets", GetDDLType: "SECRET"},
	{Name: "GIT REPOSITORY", Plural: "GIT REPOSITORIES", Label: "Git Repositories", GetDDLType: "GIT REPOSITORY"},
	{Name: "DBT PROJECT", Plural: "DBT PROJECTS", Label: "DBT Projects", GetDDLType: "DBT PROJECT"},
	// ML / Cortex objects GET_DDL has no object type for.
	{Name: "MODEL", Plural: "MODELS", Label: "Models"},
	{Name: "MODEL MONITOR", Plural: "MODEL MONITORS", Label: "Model Monitors"},
	{Name: "DATASET", Plural: "DATASETS", Label: "Datasets"},
	{Name: "CORTEX SEARCH SERVICE", Plural: "CORTEX SEARCH SERVICES", Label: "Cortex Search Services"},
	// Cortex agents are exposed to GET_DDL under 'CORTEX_AGENT'; external agents
	// and MCP servers have no object type at all.
	{Name: "AGENT", Plural: "AGENTS", Label: "Agents", GetDDLType: "CORTEX_AGENT"},
	{Name: "EXTERNAL AGENT", Plural: "EXTERNAL AGENTS", Label: "External Agents"},
	{Name: "MCP SERVER", Plural: "MCP SERVERS", Label: "MCP Servers"},
	{Name: "SEMANTIC VIEW", Plural: "SEMANTIC VIEWS", Label: "Semantic Views", GetDDLType: "SEMANTIC VIEW"},
}

// byName indexes Kinds for O(1) lookup. Built once at init; Kinds is never
// mutated at runtime.
var byName = func() map[string]Kind {
	m := make(map[string]Kind, len(Kinds))
	for _, k := range Kinds {
		m[k.Name] = k
	}
	return m
}()

// extended is the non-basic subset of Kinds — the kinds needing a dedicated SHOW
// command because SHOW OBJECTS does not return them — in registry order.
var extended = func() []Kind {
	out := make([]Kind, 0, len(Kinds))
	for _, k := range Kinds {
		if !k.Basic {
			out = append(out, k)
		}
	}
	return out
}()

// normalize canonicalizes a caller-supplied kind string ("  dynamic table " →
// "DYNAMIC TABLE") so lookups tolerate the casing/padding variations that reach
// the IPC boundary.
func normalize(kind string) string {
	return strings.ToUpper(strings.TrimSpace(kind))
}

// ByName returns the registry entry for a kind, matched case-insensitively and
// ignoring surrounding whitespace. The second return is false for anything not
// in the registry — including the account- and database-scoped kinds (DATABASE,
// SCHEMA, WAREHOUSE, ROLE, USER), which callers handle explicitly.
func ByName(kind string) (Kind, bool) {
	k, ok := byName[normalize(kind)]
	return k, ok
}

// Extended returns the kinds that need a dedicated SHOW command (everything
// except TABLE / VIEW / SEQUENCE, which SHOW OBJECTS covers), in registry order.
// The result is a fresh slice the caller may keep or reorder.
func Extended() []Kind {
	out := make([]Kind, len(extended))
	copy(out, extended)
	return out
}

// IsExtended reports whether a kind is sourced from its own SHOW command rather
// than from SHOW OBJECTS. Unknown kinds are not extended, so a caller asking for
// one falls back to SHOW OBJECTS exactly as an unrecognized basic kind would.
func IsExtended(kind string) bool {
	k, ok := ByName(kind)
	return ok && !k.Basic
}

// DDLUnsupported returns the set of kinds GET_DDL has no object type for, keyed
// by canonical name. Callers must reject these before building a GET_DDL query:
// the call fails on the server and the driver logs it as error noise.
func DDLUnsupported() map[string]bool {
	m := make(map[string]bool)
	for _, k := range Kinds {
		if k.GetDDLType == "" {
			m[k.Name] = true
		}
	}
	return m
}
