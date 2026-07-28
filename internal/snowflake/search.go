// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"thaw/internal/objectkind"
)

// SearchAccountResult is the result of SearchAccountObjects: the matched objects
// plus the kinds whose SHOW … IN ACCOUNT hit the row cap (so their results may be
// incomplete and the UI can say so).
type SearchAccountResult struct {
	Objects     []SnowflakeObject `json:"objects"`
	CappedKinds []string          `json:"cappedKinds"`
}

// extendedShowKinds is every object kind NOT covered by SHOW OBJECTS, each
// carrying the plural noun of its dedicated SHOW command. It comes straight from
// the canonical registry (internal/objectkind), which both the per-schema
// listing (ListExtendedObjects) and the account-wide search (SearchAccountObjects)
// derive from, so a newly supported kind is one registry entry.
//
// Basic kinds (TABLE, VIEW, SEQUENCE) are intentionally absent — they come from
// SHOW OBJECTS, exactly as ListBasicObjects sources them per-schema.
var extendedShowKinds = objectkind.Extended()

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
			if !objectkind.IsExtended(k) {
				needBasic = true
				break
			}
		}
	}
	if needBasic {
		commands = append(commands, accountShowCmd{"SHOW OBJECTS" + suffix, ""})
	}

	for _, k := range extendedShowKinds {
		if excl[k.Name] {
			continue
		}
		if all || want[k.Name] {
			commands = append(commands, accountShowCmd{"SHOW " + k.Plural + suffix, k.Name})
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
//
// Because each SHOW is capped with LIMIT accountSearchRowLimit and, for regex
// (or wildcard-widened) searches, the name filter runs client-side *after* the
// fetch, a kind with more objects than the cap returns an arbitrary slice and
// real matches beyond it are never seen. SearchAccountResult.CappedKinds lists
// any kind whose SHOW hit the cap so the caller can warn that its results may be
// incomplete.
func (c *Client) SearchAccountObjects(ctx context.Context, namePattern string, kinds []string) (SearchAccountResult, error) {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	all := len(want) == 0
	commands := planAccountSearchCommands(namePattern, kinds, c.getExcludedExtendedKinds(), accountSearchRowLimit)

	type result struct {
		objs    []SnowflakeObject
		scanned int
		cmd     accountShowCmd
		err     error
	}
	results := make([]result, len(commands))
	var wg sync.WaitGroup
	for i, cmd := range commands {
		wg.Add(1)
		go func(i int, cmd accountShowCmd) {
			defer wg.Done()
			// schema="" → showInSchema reads database_name/schema_name per row.
			objs, scanned, err := c.showInSchema(ctx, cmd.query, cmd.fixedKind, "")
			results[i] = result{objs: objs, scanned: scanned, cmd: cmd, err: err}
		}(i, cmd)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var out []SnowflakeObject
	outcomes := make([]searchCommandOutcome, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			continue // skip kinds we can't access / that don't support IN ACCOUNT
		}
		keptKinds := make(map[string]bool)
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
			keptKinds[o.Kind] = true
		}
		kinds := make([]string, 0, len(keptKinds))
		for k := range keptKinds {
			kinds = append(kinds, k)
		}
		outcomes = append(outcomes, searchCommandOutcome{fixedKind: r.cmd.fixedKind, scanned: r.scanned, keptKinds: kinds})
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

	return SearchAccountResult{Objects: out, CappedKinds: collectCappedKinds(outcomes, accountSearchRowLimit)}, nil
}

// searchCommandOutcome is one SHOW … IN ACCOUNT command's contribution to cap
// detection: the raw rows it scanned, the kind it's fixed to ("" for SHOW
// OBJECTS), and the distinct kinds actually kept from it.
type searchCommandOutcome struct {
	fixedKind string
	scanned   int
	keptKinds []string
}

// collectCappedKinds returns, sorted, the object kinds whose command hit the row
// cap (scanned >= limit) — meaning the SHOW returned a truncated slice and the
// client-side name filter (regex / wildcard-widened) may have never seen real
// matches beyond it. A dedicated command maps to its one kind; SHOW OBJECTS maps
// to the kinds it actually returned. limit <= 0 means uncapped (nothing flagged).
func collectCappedKinds(outcomes []searchCommandOutcome, limit int) []string {
	if limit <= 0 {
		return nil
	}
	capped := make(map[string]bool)
	for _, o := range outcomes {
		if o.scanned < limit {
			continue
		}
		if o.fixedKind != "" {
			capped[o.fixedKind] = true
		} else {
			for _, k := range o.keptKinds {
				capped[k] = true
			}
		}
	}
	if len(capped) == 0 {
		return nil
	}
	kinds := make([]string, 0, len(capped))
	for k := range capped {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
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
