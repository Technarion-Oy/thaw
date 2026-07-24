// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"fmt"
	"sync"
)

// kindShow pairs an object KIND with the plural noun used in its SHOW command.
type kindShow struct {
	kind   string
	plural string
}

// extendedShowKinds lists every object kind NOT covered by SHOW OBJECTS, paired
// with the plural noun in its dedicated SHOW command. It is the single source of
// truth for both the per-schema listing (ListExtendedObjects) and the
// account-wide search (SearchObjects), so a newly supported kind is added in one
// place. Order matches the historical ListExtendedObjects command list.
//
// Basic kinds (TABLE, VIEW, SEQUENCE) are intentionally absent — they come from
// SHOW OBJECTS, exactly as ListBasicObjects sources them per-schema.
var extendedShowKinds = []kindShow{
	{"DYNAMIC TABLE", "DYNAMIC TABLES"},
	{"EXTERNAL TABLE", "EXTERNAL TABLES"},
	{"ICEBERG TABLE", "ICEBERG TABLES"},
	{"HYBRID TABLE", "HYBRID TABLES"},
	{"EVENT TABLE", "EVENT TABLES"},
	{"MATERIALIZED VIEW", "MATERIALIZED VIEWS"},
	{"ALERT", "ALERTS"},
	{"TAG", "TAGS"},
	{"MASKING POLICY", "MASKING POLICIES"},
	{"ROW ACCESS POLICY", "ROW ACCESS POLICIES"},
	{"JOIN POLICY", "JOIN POLICIES"},
	{"PRIVACY POLICY", "PRIVACY POLICIES"},
	{"STORAGE LIFECYCLE POLICY", "STORAGE LIFECYCLE POLICIES"},
	{"PASSWORD POLICY", "PASSWORD POLICIES"},
	{"SESSION POLICY", "SESSION POLICIES"},
	{"AGGREGATION POLICY", "AGGREGATION POLICIES"},
	{"PROJECTION POLICY", "PROJECTION POLICIES"},
	{"AUTHENTICATION POLICY", "AUTHENTICATION POLICIES"},
	{"PACKAGES POLICY", "PACKAGES POLICIES"},
	{"NETWORK RULE", "NETWORK RULES"},
	{"IMAGE REPOSITORY", "IMAGE REPOSITORIES"},
	{"SERVICE", "SERVICES"},
	{"GATEWAY", "GATEWAYS"},
	{"CONTACT", "CONTACTS"},
	{"STREAMLIT", "STREAMLITS"},
	{"PROCEDURE", "PROCEDURES"},
	{"FUNCTION", "FUNCTIONS"},
	{"EXTERNAL FUNCTION", "EXTERNAL FUNCTIONS"},
	{"DATA METRIC FUNCTION", "DATA METRIC FUNCTIONS"},
	{"TASK", "TASKS"},
	{"STREAM", "STREAMS"},
	{"STAGE", "STAGES"},
	{"FILE FORMAT", "FILE FORMATS"},
	{"PIPE", "PIPES"},
	{"NOTEBOOK", "NOTEBOOKS"},
	{"SECRET", "SECRETS"},
	{"GIT REPOSITORY", "GIT REPOSITORIES"},
	{"DBT PROJECT", "DBT PROJECTS"},
	{"MODEL", "MODELS"},
	{"MODEL MONITOR", "MODEL MONITORS"},
	{"DATASET", "DATASETS"},
	{"CORTEX SEARCH SERVICE", "CORTEX SEARCH SERVICES"},
	{"AGENT", "AGENTS"},
	{"EXTERNAL AGENT", "EXTERNAL AGENTS"},
	{"MCP SERVER", "MCP SERVERS"},
	{"SEMANTIC VIEW", "SEMANTIC VIEWS"},
}

// extendedKindSet is the set of kinds sourced from a dedicated SHOW command.
// Anything not in it (TABLE, VIEW, SEQUENCE, …) is sourced from SHOW OBJECTS.
var extendedKindSet = func() map[string]bool {
	m := make(map[string]bool, len(extendedShowKinds))
	for _, k := range extendedShowKinds {
		m[k.kind] = true
	}
	return m
}()

// showLikeClause builds a case-insensitive substring LIKE clause for a SHOW
// command, or "" when no pattern is given. The pattern is wrapped in %…% so it
// matches anywhere in the name; any % / _ the user typed act as wildcards, which
// only ever widens the match — the frontend re-filters for exact semantics, so a
// superset is safe. Free text is escaped to stay injection-safe.
func showLikeClause(pattern string) string {
	if pattern == "" {
		return ""
	}
	return " LIKE '%" + EscapeTextLit(pattern) + "%'"
}

// accountShowCmd is a single planned SHOW … IN ACCOUNT command.
type accountShowCmd struct {
	query     string
	fixedKind string // "" → read the kind column (SHOW OBJECTS)
}

// accountSearchRowLimit caps the rows each SHOW … IN ACCOUNT returns. Without it
// a broad search (e.g. no name filter, or a regex like `.*` where nothing is
// pushed to the server) would ship every object of a kind — thousands of tables
// — to the frontend to build and render, which locks up the UI. The frontend
// caps what it *displays* even tighter and prompts the user to refine.
const accountSearchRowLimit = 2000

// showLimitClause returns " LIMIT n" (or "" when limit <= 0). SHOW's LIMIT comes
// last, after the IN ACCOUNT scope.
func showLimitClause(limit int) string {
	if limit <= 0 {
		return ""
	}
	return fmt.Sprintf(" LIMIT %d", limit)
}

// planAccountSearchCommands decides which SHOW … IN ACCOUNT commands to run for a
// SearchAccountObjects call. Pure (no client), so it is unit-tested directly.
// `all` (no kind filter) runs SHOW OBJECTS plus every non-excluded extended
// command; otherwise SHOW OBJECTS is included only when a requested kind isn't
// sourced from a dedicated command (TABLE/VIEW/SEQUENCE), and each requested,
// non-excluded extended kind gets its own command. Each command is capped at
// `limit` rows (0 = uncapped).
func planAccountSearchCommands(namePattern string, kinds []string, excl map[string]bool, limit int) []accountShowCmd {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	all := len(want) == 0
	suffix := showLikeClause(namePattern) + " IN ACCOUNT" + showLimitClause(limit)

	var commands []accountShowCmd

	needBasic := all
	if !needBasic {
		for k := range want {
			if !extendedKindSet[k] {
				needBasic = true
				break
			}
		}
	}
	if needBasic {
		commands = append(commands, accountShowCmd{"SHOW OBJECTS" + suffix, ""})
	}

	for _, k := range extendedShowKinds {
		if excl[k.kind] {
			continue
		}
		if all || want[k.kind] {
			commands = append(commands, accountShowCmd{"SHOW " + k.plural + suffix, k.kind})
		}
	}
	return commands
}

// SearchAccountObjects finds objects across the whole account with one SHOW … IN
// ACCOUNT query per object kind, instead of walking every schema of every
// database (which is O(databases × schemas) round-trips). This makes a search
// for, say, a single Streamlit one query rather than thousands. It powers the
// sidebar object-browser search and — unlike the INFORMATION_SCHEMA-based
// SearchObjects (MCP search_objects tool, tables/columns in one database) — it
// covers every object kind account-wide via SHOW.
//
//   - namePattern: optional case-insensitive substring pushed to the server as a
//     LIKE filter to bound large kinds (TABLE/VIEW). Pass "" to fetch all of the
//     requested kinds (e.g. for regex search, where the caller filters names
//     client-side).
//   - kinds: object KINDs to search (e.g. "STREAMLIT", "PROCEDURE"). Empty means
//     all kinds. SHOW OBJECTS is issued when any requested kind is not sourced
//     from a dedicated SHOW command (TABLE/VIEW/SEQUENCE), and each requested
//     extended kind gets its own SHOW … IN ACCOUNT.
//
// Per-kind failures (missing privileges, kinds an edition doesn't support IN
// ACCOUNT) are skipped, mirroring ListExtendedObjects. Results carry their
// Database/Schema from the SHOW row and are deduped by (db, schema, kind, name,
// args); a plain FUNCTION that is really an EXTERNAL / DATA METRIC FUNCTION (on
// editions without the discriminator column) is reconciled per database, the
// same way ListExtendedObjects does within a schema.
func (c *Client) SearchAccountObjects(ctx context.Context, namePattern string, kinds []string) ([]SnowflakeObject, error) {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	all := len(want) == 0
	commands := planAccountSearchCommands(namePattern, kinds, c.getExcludedExtendedKinds(), accountSearchRowLimit)

	type result struct {
		objs []SnowflakeObject
		err  error
	}
	results := make([]result, len(commands))
	var wg sync.WaitGroup
	for i, cmd := range commands {
		wg.Add(1)
		go func(i int, cmd accountShowCmd) {
			defer wg.Done()
			// schema="" → showInSchema reads database_name/schema_name per row.
			results[i].objs, results[i].err = c.showInSchema(ctx, cmd.query, cmd.fixedKind, "")
		}(i, cmd)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var out []SnowflakeObject
	for _, r := range results {
		if r.err != nil {
			continue // skip kinds we can't access / that don't support IN ACCOUNT
		}
		for _, o := range r.objs {
			// SHOW OBJECTS returns every basic kind; keep only requested ones.
			if !all && !want[o.Kind] {
				continue
			}
			key := o.Database + "\x00" + o.Schema + "\x00" + o.Kind + "\x00" + o.Name + "\x00" + o.Arguments
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, o)
		}
	}

	// The exact-key dedup above collapses the duplicate EXTERNAL / DATA METRIC
	// FUNCTION rows (SHOW FUNCTIONS relabel + the dedicated SHOW command), but not
	// the cross-kind collision: on editions lacking the is_external_function /
	// is_data_metric column a plain FUNCTION row is really a variant. Reconcile it
	// the way ListExtendedObjects does — but only when both a plain FUNCTION and a
	// variant kind are present (the user selected both, or searched all kinds).
	hasPlainFn, hasVariant := false, false
	for _, o := range out {
		switch o.Kind {
		case "FUNCTION":
			hasPlainFn = true
		case "EXTERNAL FUNCTION", "DATA METRIC FUNCTION":
			hasVariant = true
		}
	}
	if hasPlainFn && hasVariant {
		out = reconcileFunctionVariants(out)
	}
	return out, nil
}

// reconcileFunctionVariants applies the FUNCTION vs EXTERNAL / DATA METRIC
// FUNCTION reconciliation (dedupeFunctionVariant) to account-wide results.
// dedupeFunctionVariant keys by (schema, name, args) without a database, so we
// group by database first: without that, two distinct functions that share a
// schema/name/signature across different databases would wrongly collapse. Group
// order is preserved (the frontend re-groups and sorts anyway).
func reconcileFunctionVariants(objs []SnowflakeObject) []SnowflakeObject {
	byDB := make(map[string][]SnowflakeObject)
	order := make([]string, 0)
	for _, o := range objs {
		if _, ok := byDB[o.Database]; !ok {
			order = append(order, o.Database)
		}
		byDB[o.Database] = append(byDB[o.Database], o)
	}
	out := make([]SnowflakeObject, 0, len(objs))
	for _, db := range order {
		g := dedupeFunctionVariant(byDB[db], "EXTERNAL FUNCTION")
		g = dedupeFunctionVariant(g, "DATA METRIC FUNCTION")
		out = append(out, g...)
	}
	return out
}
