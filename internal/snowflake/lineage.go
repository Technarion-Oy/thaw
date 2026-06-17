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
	"context"
	"fmt"
	"sort"
	"strings"

	"thaw/internal/sqltok"
)

// maxDependencyDepth caps recursive DDL-parsing depth to prevent runaway
// recursion on deeply nested or cyclic object graphs.
const maxDependencyDepth = 8

// SchemaRef identifies a (database, schema) pair.  It is returned by
// GetSchemaCrossDeps to surface cross-schema lineage hints in the dbt project
// wizard.
type SchemaRef struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

// DependencyNode is one node in the recursive dependency tree returned by
// GetObjectDependencies.
type DependencyNode struct {
	Name     string `json:"name"`
	Schema   string `json:"schema"`
	Database string `json:"database"`
	// Kind is one of TABLE, VIEW, PROCEDURE, FUNCTION, UNKNOWN.
	Kind     string           `json:"kind"`
	Children []DependencyNode `json:"children"`
	// Circular is true when this node was already encountered in the current
	// tree (prevents infinite expansion of cyclic or shared dependencies).
	Circular bool `json:"circular,omitempty"`
	// Error contains a short description when the object could not be resolved.
	Error string `json:"error,omitempty"`
}

// sqlRef is an unresolved reference extracted from a SQL body.
type sqlRef struct {
	db     string
	schema string
	name   string
	// isCall is true when the reference was found in a CALL statement,
	// meaning the target is definitely a stored procedure.
	isCall bool
}

// depVisited is a simple string-set used to track which objects have already
// been expanded in the current tree traversal.
type depVisited map[string]bool

func (v depVisited) has(db, schema, name string) bool {
	return v[depKey(db, schema, name)]
}

func (v depVisited) add(db, schema, name string) {
	v[depKey(db, schema, name)] = true
}

func (v depVisited) clone() depVisited {
	c := make(depVisited, len(v))
	for k, val := range v {
		c[k] = val
	}
	return c
}

func depKey(db, schema, name string) string {
	return strings.ToUpper(db + "." + schema + "." + name)
}

// ── token-based SQL scanning ────────────────────────────────────────────────────
//
// Object references used to be extracted with a bank of hand-tuned regexes.
// They are now derived from the shared lexer in internal/sqltok, which is both
// more robust (it understands nested block comments, the "" / '' escapes inside
// quoted identifiers and strings, and $tag$ dollar-quoting) and easier to reason
// about than the previous (?i)\bFROM\s+… patterns.
//
// Identifier semantics are deliberately preserved from the old identPat regex:
//   - an identifier is up to three word/quoted parts joined by dots,
//   - the dots must be immediately adjacent (no whitespace) — so a spaced
//     "DB" . "SC" . "T" still collapses to a single-part reference, and
//   - bare keywords (TABLE, SET, GENERATOR, …) are captured as parts and then
//     dropped by skipNames, exactly as \w+ + skipNames did before.

// refIntroKeywords are the keywords that introduce a table/view reference as
// the identifier immediately following them. MERGE INTO is covered by INTO.
var refIntroKeywords = map[string]bool{
	"FROM": true, "JOIN": true, "INTO": true,
	"UPDATE": true, "USING": true, "CLONE": true,
}

// rawRef is an identifier reference extracted from the token stream, in the
// original source text (e.g. `MY_DB.PUBLIC."My Table"`), before qualification.
type rawRef struct {
	raw    string
	isCall bool
}

// tokKeywordIs reports whether token t is a keyword equal to kw (case-insensitive).
func tokKeywordIs(t sqltok.Token, src, kw string) bool {
	return t.Kind == sqltok.Keyword && strings.EqualFold(t.Text(src), kw)
}

// scanIdentRun reads an identifier starting at tokens[i] and returns its raw
// source text, the index just past it, and whether one was found. It consumes
// up to three dot-joined word parts; dots must be immediately adjacent (a
// whitespace or comment token before the dot ends the identifier).
func scanIdentRun(tokens []sqltok.Token, src string, i int) (string, int, bool) {
	if i >= len(tokens) || !tokens[i].Kind.IsIdentLike() {
		return "", i, false
	}
	start := tokens[i].Start
	last := i
	for parts := 1; parts < 3; parts++ {
		if last+2 < len(tokens) && tokens[last+1].Kind == sqltok.Dot && tokens[last+2].Kind.IsIdentLike() {
			last += 2
			continue
		}
		break
	}
	return src[start:tokens[last].End], last + 1, true
}

// extractObjectRefs walks the token stream and returns every table/view/proc
// reference in source order. FROM additionally walks old-style comma joins
// (FROM a, b, c); CALL references are flagged isCall and require a following
// "(" (mirroring the previous reCall pattern).
func extractObjectRefs(src string, tokens []sqltok.Token) []rawRef {
	var refs []rawRef
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if t.Kind != sqltok.Keyword {
			i++
			continue
		}
		kw := strings.ToUpper(t.Text(src))
		switch {
		case kw == "FROM":
			j := sqltok.SkipTrivia(tokens, i+1)
			raw, next, ok := scanIdentRun(tokens, src, j)
			if !ok {
				i++
				continue
			}
			refs = append(refs, rawRef{raw: raw})
			i = commaWalk(tokens, src, next, &refs)
		case refIntroKeywords[kw]:
			j := sqltok.SkipTrivia(tokens, i+1)
			if raw, next, ok := scanIdentRun(tokens, src, j); ok {
				refs = append(refs, rawRef{raw: raw})
				i = next
				continue
			}
			i++
		case kw == "CALL":
			j := sqltok.SkipTrivia(tokens, i+1)
			if raw, next, ok := scanIdentRun(tokens, src, j); ok {
				if k := sqltok.SkipTrivia(tokens, next); k < len(tokens) && tokens[k].Kind == sqltok.LParen {
					refs = append(refs, rawRef{raw: raw, isCall: true})
					i = next
					continue
				}
			}
			i++
		default:
			i++
		}
	}
	return refs
}

// commaWalk consumes an old-style comma-separated table list that follows a
// FROM table. start is the index just past the previous table; for each
// "[alias] , <table>" it appends the table and continues. It returns the index
// at which scanning should resume (the previous table's end when the list
// stops), so the main loop can re-examine any JOIN/WHERE that ended the list.
func commaWalk(tokens []sqltok.Token, src string, start int, refs *[]rawRef) int {
	cur := start
	for {
		// Optional single-word alias for the previous table, e.g. "o" or "AS o".
		k := sqltok.SkipTrivia(tokens, cur)
		aliasEnd := k
		if k < len(tokens) && tokens[k].Kind.IsIdentLike() {
			if tokKeywordIs(tokens[k], src, "AS") {
				if m := sqltok.SkipTrivia(tokens, k+1); m < len(tokens) && tokens[m].Kind.IsIdentLike() {
					aliasEnd = m + 1
				} else {
					aliasEnd = k + 1
				}
			} else {
				aliasEnd = k + 1
			}
		}
		c := sqltok.SkipTrivia(tokens, aliasEnd)
		if c >= len(tokens) || tokens[c].Kind != sqltok.Comma {
			return cur
		}
		raw, next, ok := scanIdentRun(tokens, src, sqltok.SkipTrivia(tokens, c+1))
		if !ok {
			return cur
		}
		*refs = append(*refs, rawRef{raw: raw})
		cur = next
	}
}

// collectCTENames returns the set of CTE alias names (upper-cased, unquoted)
// declared by "WITH name AS (" or ", name AS (". These are local aliases and
// must never be treated as object references.
func collectCTENames(src string, tokens []sqltok.Token) map[string]bool {
	names := map[string]bool{}
	for i, t := range tokens {
		if !(tokKeywordIs(t, src, "WITH") || t.Kind == sqltok.Comma) {
			continue
		}
		j := sqltok.SkipTrivia(tokens, i+1)
		if j >= len(tokens) || !tokens[j].Kind.IsIdentLike() {
			continue
		}
		name := tokens[j].Text(src)
		k := sqltok.SkipTrivia(tokens, j+1)
		if k >= len(tokens) || !tokKeywordIs(tokens[k], src, "AS") {
			continue
		}
		if l := sqltok.SkipTrivia(tokens, k+1); l >= len(tokens) || tokens[l].Kind != sqltok.LParen {
			continue
		}
		names[strings.ToUpper(stripQuotes(name))] = true
	}
	return names
}

// skipNames is a set of Snowflake / SQL keywords that must never be treated as
// object references even if they follow FROM / INTO / etc.
var skipNames = map[string]bool{
	"SELECT": true, "WHERE": true, "SET": true, "VALUES": true,
	"LATERAL": true, "FLATTEN": true, "UNNEST": true, "GENERATOR": true,
	"TABLE": true, "RESULT_SCAN": true, "SNOWFLAKE": true,
	"DUAL": true, "NULL": true, "TRUE": true, "FALSE": true,
}

// ── public entry-point ────────────────────────────────────────────────────────

// GetObjectDependencies returns the full recursive dependency tree for the
// given VIEW, PROCEDURE, or FUNCTION by parsing its DDL.  Tables are treated
// as leaf nodes.  Non-SQL-language procedures and functions are not parsed
// (they have no reachable body).
func (c *Client) GetObjectDependencies(ctx context.Context, database, schema, kind, name, arguments string) (DependencyNode, error) {
	ddlText, err := c.GetObjectDDL(ctx, database, schema, kind, name, arguments)
	if err != nil {
		return DependencyNode{}, fmt.Errorf("GetObjectDDL: %w", err)
	}

	vis := make(depVisited)
	vis.add(database, schema, name)

	root := DependencyNode{
		Name:     strings.ToUpper(name),
		Schema:   strings.ToUpper(schema),
		Database: strings.ToUpper(database),
		Kind:     strings.ToUpper(kind),
	}
	root.Children = c.buildChildren(ctx, ddlText, database, schema, strings.ToUpper(kind), vis, 0)
	return root, nil
}

// GetSchemaCrossDeps returns the unique (database, schema) pairs referenced
// by views in db.schema that fall outside that schema.
//
// It issues a single SHOW VIEWS IN SCHEMA query — one round-trip, no
// goroutines.  The "text" column returned by SHOW VIEWS contains the full
// CREATE VIEW DDL, so we can extract the SELECT body with ExtractDDLBody and
// scan it with parseSQLReferences without any additional Snowflake calls.
// An empty schema returns immediately with zero rows.
func (c *Client) GetSchemaCrossDeps(ctx context.Context, db, schema string) ([]SchemaRef, error) {
	q := fmt.Sprintf(`SHOW VIEWS IN SCHEMA %s.%s`, QuoteIdent(db), QuoteIdent(schema))

	rows, err := c.queryCtx(ctx, q)
	if err != nil {
		return nil, nil //nolint:nilerr // inaccessible schema is non-fatal
	}
	defer rows.Close() //nolint:errcheck

	// Locate the "text" column that contains the full CREATE VIEW DDL.
	cols, _ := rows.Columns()
	textIdx := -1
	for i, col := range cols {
		if strings.EqualFold(col, "text") {
			textIdx = i
			break
		}
	}
	if textIdx < 0 {
		return nil, nil
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	seen := map[string]bool{}
	var refs []SchemaRef

	for rows.Next() {
		if ctx.Err() != nil {
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		viewDDL, _ := vals[textIdx].(string)
		if viewDDL == "" {
			continue
		}
		body := ExtractDDLBody(viewDDL, "VIEW")
		if body == "" {
			continue
		}
		for _, ref := range parseSQLReferences(body, db, schema) {
			if strings.EqualFold(ref.db, db) && strings.EqualFold(ref.schema, schema) {
				continue
			}
			key := strings.ToUpper(ref.db + "." + ref.schema)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, SchemaRef{
					Database: strings.ToUpper(ref.db),
					Schema:   strings.ToUpper(ref.schema),
				})
			}
		}
	}
	return refs, nil
}

// GetDatabaseCrossDeps calls GetSchemaCrossDeps for each schema sequentially
// and returns the combined unique (database, schema) pairs.  Callers should
// prefer this over firing N concurrent GetSchemaCrossDeps IPC calls to avoid
// exhausting Snowflake connection pool when a database has many schemas.
func (c *Client) GetDatabaseCrossDeps(ctx context.Context, db string, schemas []string) ([]SchemaRef, error) {
	seen := map[string]bool{}
	var result []SchemaRef
	for _, schema := range schemas {
		if ctx.Err() != nil {
			break
		}
		refs, _ := c.GetSchemaCrossDeps(ctx, db, schema)
		for _, r := range refs {
			key := r.Database + "." + r.Schema
			if !seen[key] {
				seen[key] = true
				result = append(result, r)
			}
		}
	}
	return result, nil
}

// ── recursive builder ─────────────────────────────────────────────────────────

func (c *Client) buildChildren(ctx context.Context, ddlText, defaultDB, defaultSchema, kind string, vis depVisited, depth int) []DependencyNode {
	if depth >= maxDependencyDepth {
		return nil
	}

	body := ExtractDDLBody(ddlText, kind)
	if body == "" {
		return nil
	}

	refs := parseSQLReferences(body, defaultDB, defaultSchema)
	if len(refs) == 0 {
		return nil
	}

	// Deduplicate references by (db, schema, name) before resolving.
	seen := map[string]bool{}
	var unique []sqlRef
	for _, r := range refs {
		k := depKey(r.db, r.schema, r.name)
		if !seen[k] {
			seen[k] = true
			unique = append(unique, r)
		}
	}

	var nodes []DependencyNode
	for _, ref := range unique {
		nodes = append(nodes, c.resolveRef(ctx, ref, vis, depth))
	}
	return nodes
}

func (c *Client) resolveRef(ctx context.Context, ref sqlRef, vis depVisited, depth int) DependencyNode {
	node := DependencyNode{
		Name:     strings.ToUpper(ref.name),
		Schema:   strings.ToUpper(ref.schema),
		Database: strings.ToUpper(ref.db),
	}

	if vis.has(ref.db, ref.schema, ref.name) {
		node.Kind = "UNKNOWN"
		node.Circular = true
		return node
	}

	if ref.isCall {
		return c.resolveProcedureRef(ctx, ref, vis, depth)
	}

	// Use INFORMATION_SCHEMA.TABLES to determine whether the reference is a
	// VIEW or a TABLE.  Doing a speculative GET_DDL('VIEW',...) call would
	// trigger error log entries inside the gosnowflake driver for every
	// non-view reference, producing confusing noise in application logs.
	if c.isViewInSchema(ctx, ref.db, ref.schema, ref.name) {
		node.Kind = "VIEW"
		branchVis := vis.clone()
		branchVis.add(ref.db, ref.schema, ref.name)
		ddlText, err := c.GetObjectDDL(ctx, ref.db, ref.schema, "VIEW", ref.name, "")
		if err == nil {
			node.Children = c.buildChildren(ctx, ddlText, ref.db, ref.schema, "VIEW", branchVis, depth+1)
		} else {
			node.Error = err.Error()
		}
		return node
	}

	node.Kind = "TABLE"
	return node
}

// isViewInSchema queries INFORMATION_SCHEMA.TABLES to check whether the given
// object is a VIEW.  Returns false for tables and for objects not found in
// INFORMATION_SCHEMA (e.g. procedures, functions, or objects the current role
// cannot access).
func (c *Client) isViewInSchema(ctx context.Context, db, schema, name string) bool {
	escVal := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	q := fmt.Sprintf(
		`SELECT TABLE_TYPE FROM %s.INFORMATION_SCHEMA.TABLES`+
			` WHERE TABLE_CATALOG = '%s' AND TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'`,
		strings.ReplaceAll(db, `"`, `""`),
		escVal(strings.ToUpper(db)),
		escVal(strings.ToUpper(schema)),
		escVal(strings.ToUpper(name)),
	)
	row := c.queryRowCtx(ctx, q)
	var typ string
	if err := row.Scan(&typ); err != nil {
		return false
	}
	return strings.ToUpper(typ) == "VIEW"
}

func (c *Client) resolveProcedureRef(ctx context.Context, ref sqlRef, vis depVisited, depth int) DependencyNode {
	node := DependencyNode{
		Name:     strings.ToUpper(ref.name),
		Schema:   strings.ToUpper(ref.schema),
		Database: strings.ToUpper(ref.db),
		Kind:     "PROCEDURE",
	}

	// SHOW PROCEDURES returns all overloads; we need the argument signature to
	// call GET_DDL for each one.
	query := fmt.Sprintf(
		`SHOW PROCEDURES LIKE '%s' IN SCHEMA %s.%s`,
		strings.ReplaceAll(ref.name, "'", "''"),
		strings.ReplaceAll(ref.db, `"`, `""`),
		strings.ReplaceAll(ref.schema, `"`, `""`),
	)
	rows, err := c.queryCtx(ctx, query)
	if err != nil {
		node.Error = err.Error()
		return node
	}
	defer rows.Close() //nolint:errcheck

	cols, _ := rows.Columns()
	argIdx, nameIdx := -1, -1
	for i, col := range cols {
		switch strings.ToLower(col) {
		case "arguments":
			argIdx = i
		case "name":
			nameIdx = i
		}
	}

	type overload struct{ args string }
	var overloads []overload

	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		var procName, argsRaw string
		if nameIdx >= 0 {
			if v, ok := vals[nameIdx].(string); ok {
				procName = v
			}
		}
		if argIdx >= 0 {
			if v, ok := vals[argIdx].(string); ok {
				argsRaw = v
			}
		}
		if strings.EqualFold(procName, ref.name) {
			overloads = append(overloads, overload{args: extractArgTypesFromShow(argsRaw)})
		}
	}

	if len(overloads) == 0 {
		return node
	}

	branchVis := vis.clone()
	branchVis.add(ref.db, ref.schema, ref.name)
	for _, ol := range overloads {
		ddlText, err := c.GetObjectDDL(ctx, ref.db, ref.schema, "PROCEDURE", ref.name, ol.args)
		if err != nil {
			continue
		}
		children := c.buildChildren(ctx, ddlText, ref.db, ref.schema, "PROCEDURE", branchVis, depth+1)
		node.Children = append(node.Children, children...)
	}
	return node
}

// extractArgTypesFromShow parses the argument type list from the string
// returned by SHOW PROCEDURES / SHOW FUNCTIONS in the "arguments" column.
// That column has the form "PROC_NAME(TYPE1, TYPE2) RETURN RETURN_TYPE".
func extractArgTypesFromShow(s string) string {
	i := strings.Index(s, "(")
	j := strings.LastIndex(s, ")")
	if i < 0 || j <= i {
		return ""
	}
	return strings.TrimSpace(s[i+1 : j])
}

// ── DDL body extraction ───────────────────────────────────────────────────────

// ExtractDDLBody returns the SQL body of a DDL statement that should be
// scanned for object references.  Returns an empty string when the body cannot
// be extracted (e.g. non-SQL-language procedures).
//
// For VIEWs this is the SELECT/WITH clause that follows the final AS keyword.
// Exported so callers outside the package (e.g. the dbt generator) can inline
// view definitions without duplicating the extraction logic.
func ExtractDDLBody(ddl, kind string) string {
	switch strings.ToUpper(kind) {
	case "VIEW":
		// Everything after the final AS that precedes SELECT or WITH.
		if body := extractViewBody(ddl); body != "" {
			return body
		}
		// Fallback: parse the entire DDL (may produce false positives from
		// the CREATE header, but deduplication and the visited set handle it).
		return ddl

	case "PROCEDURE", "FUNCTION":
		tokens := sqltok.Tokenize(ddl)
		// Only parse SQL-language bodies; JavaScript / Python / Java bodies
		// contain no Snowflake object references we can statically extract.
		if !hasLanguageSQL(ddl, tokens) {
			return ""
		}
		// Standard Snowflake dollar-quoting: $$ ... $$ (or $tag$ ... $tag$).
		if body := dollarQuotedBody(ddl, tokens); body != "" {
			return body
		}
		// Single-$ delimiter: some clients / older DDL exports use a bare $
		// on its own line (e.g.  AS\n$\nBEGIN...\nEND;\n$).
		if body := singleDollarBody(ddl, tokens); body != "" {
			return body
		}
		// Single-quoted body: AS '...'
		if i := strings.Index(ddl, "AS '"); i >= 0 {
			inner := ddl[i+4:]
			if j := strings.LastIndex(inner, "'"); j > 0 {
				return inner[:j]
			}
		}
		return ""
	}
	return ""
}

// extractViewBody returns the SELECT/WITH clause that follows the first "AS"
// keyword preceding a SELECT or WITH, or "" when none is found. The trailing
// semicolon and whitespace are trimmed before scanning so the body runs to the
// end of the statement.
func extractViewBody(ddl string) string {
	trimmed := strings.TrimRight(ddl, "; \t\n\r")
	tokens := sqltok.Tokenize(trimmed)
	for i, t := range tokens {
		if !tokKeywordIs(t, trimmed, "AS") {
			continue
		}
		j := sqltok.SkipTrivia(tokens, i+1)
		if j < len(tokens) && (tokKeywordIs(tokens[j], trimmed, "WITH") || tokKeywordIs(tokens[j], trimmed, "SELECT")) {
			return trimmed[tokens[j].Start:]
		}
	}
	return ""
}

// hasLanguageSQL reports whether the DDL declares LANGUAGE SQL (two adjacent
// keyword tokens), identifying a SQL-language procedure or function body.
func hasLanguageSQL(src string, tokens []sqltok.Token) bool {
	for i, t := range tokens {
		if tokKeywordIs(t, src, "LANGUAGE") {
			if j := sqltok.SkipTrivia(tokens, i+1); j < len(tokens) && tokKeywordIs(tokens[j], src, "SQL") {
				return true
			}
		}
	}
	return false
}

// dollarQuotedBody returns the inner text of the first dollar-quoted block
// ($$ ... $$ or $tag$ ... $tag$), or "" if there is no properly terminated one.
func dollarQuotedBody(src string, tokens []sqltok.Token) string {
	for _, t := range tokens {
		if t.Kind != sqltok.DollarQuoted {
			continue
		}
		text := t.Text(src)
		tag := t.Tag
		// Require an opening and a distinct closing delimiter with ≥1 char between.
		if strings.HasPrefix(text, tag) && strings.HasSuffix(text, tag) && len(text) > 2*len(tag) {
			return text[len(tag) : len(text)-len(tag)]
		}
	}
	return ""
}

// singleDollarBody handles the bare single-$ delimiter that some clients emit:
//
//	AS
//	$
//	BEGIN ... END;
//	$
//
// The tokenizer does not treat a lone "$" on its own line as a dollar quote
// (it has no tag and no second "$"), so each delimiter surfaces as an Other
// token. We anchor on AS, require "$" on its own line as the opener, and return
// the text up to the next "$" that begins a line.
func singleDollarBody(src string, tokens []sqltok.Token) string {
	isDollar := func(t sqltok.Token) bool {
		return t.Kind == sqltok.Other && t.Text(src) == "$"
	}
	for i, t := range tokens {
		if !tokKeywordIs(t, src, "AS") {
			continue
		}
		// AS [ws] \n $ [ws] \n
		j := i + 1
		if j < len(tokens) && tokens[j].Kind == sqltok.Whitespace {
			j++
		}
		if j >= len(tokens) || tokens[j].Kind != sqltok.Newline {
			continue
		}
		j++
		if j >= len(tokens) || !isDollar(tokens[j]) {
			continue
		}
		j++
		if j < len(tokens) && tokens[j].Kind == sqltok.Whitespace {
			j++
		}
		if j >= len(tokens) || tokens[j].Kind != sqltok.Newline {
			continue
		}
		bodyStart := tokens[j].End
		// Closing delimiter: the next "$" immediately preceded by a newline.
		for m := j + 1; m < len(tokens); m++ {
			if isDollar(tokens[m]) && tokens[m-1].Kind == sqltok.Newline {
				if bodyEnd := tokens[m-1].Start; bodyEnd > bodyStart {
					return src[bodyStart:bodyEnd]
				}
			}
		}
	}
	return ""
}

// ── SQL reference parser ──────────────────────────────────────────────────────

// parseSQLReferences scans a SQL text for table/view/procedure references and
// returns a list of sqlRef values.  defaultDB and defaultSchema are used to
// qualify unqualified names.
func parseSQLReferences(sql, defaultDB, defaultSchema string) []sqlRef {
	tokens := sqltok.Tokenize(sql)

	// Collect CTE names — these are local aliases, not real objects. Comments
	// and strings are inert token kinds, so they never produce false matches.
	cteNames := collectCTENames(sql, tokens)

	var refs []sqlRef

	addRef := func(raw string, isCall bool) {
		parts := splitIdent(raw)
		if len(parts) == 0 {
			return
		}
		name := parts[len(parts)-1]
		nameUpper := strings.ToUpper(name)

		if skipNames[nameUpper] {
			return
		}
		if strings.EqualFold(name, "INFORMATION_SCHEMA") {
			return
		}
		if cteNames[nameUpper] {
			return
		}

		var db, schema string
		switch len(parts) {
		case 1:
			db, schema = defaultDB, defaultSchema
		case 2:
			db, schema = defaultDB, parts[0]
		case 3:
			db, schema = parts[0], parts[1]
		}

		if strings.EqualFold(schema, "INFORMATION_SCHEMA") {
			return
		}

		refs = append(refs, sqlRef{db: db, schema: schema, name: name, isCall: isCall})
	}

	// Table / view references and procedure calls (order does not matter;
	// deduplication runs later).
	for _, r := range extractObjectRefs(sql, tokens) {
		addRef(r.raw, r.isCall)
	}

	return refs
}

// RewriteSQLReferences rewrites Snowflake object references in a SQL body,
// replacing each resolved (database, schema, name) triple with the string
// returned by lookup.  When lookup returns "" the reference is left unchanged.
//
// Only multi-part identifiers (two-part or three-part, e.g. SCHEMA.TABLE or
// DB.SCHEMA.TABLE) are considered for replacement.  Bare single-part names are
// skipped to avoid false-positive rewrites of column aliases or other tokens
// that happen to share a table name.
//
// Replacement is applied longest-first so that a more-qualified reference such
// as "MY_DB.MY_SCHEMA.MY_TABLE" is replaced before a shorter "MY_TABLE" if
// both somehow appear.  String replacement is case-sensitive and uses the
// original text as it appears in the SQL, which is safe for DDL produced by
// Snowflake's GET_DDL (which always emits fully-qualified, consistently-cased
// identifiers).
func RewriteSQLReferences(sql, defaultDB, defaultSchema string, lookup func(db, schema, name string) string) string {
	// Detection runs over the token stream (comments and strings are inert);
	// replacement is applied to the original text via string substitution, so a
	// reference that also appears inside a comment is rewritten as a side effect.
	tokens := sqltok.Tokenize(sql)

	// Collect CTE aliases to exclude from replacement.
	cteNames := collectCTENames(sql, tokens)

	type pair struct{ orig, repl string }
	var pairs []pair
	seen := map[string]bool{}

	consider := func(raw string) {
		if seen[raw] {
			return
		}
		parts := splitIdent(raw)
		if len(parts) < 2 {
			// Skip bare single-part names to avoid ambiguous replacements.
			return
		}
		name := parts[len(parts)-1]
		nameUp := strings.ToUpper(name)
		if skipNames[nameUp] || cteNames[nameUp] || strings.EqualFold(name, "INFORMATION_SCHEMA") {
			return
		}

		var db, schema string
		switch len(parts) {
		case 2:
			db, schema = defaultDB, parts[0]
		case 3:
			db, schema = parts[0], parts[1]
		}
		if strings.EqualFold(schema, "INFORMATION_SCHEMA") {
			return
		}

		repl := lookup(db, schema, name)
		if repl == "" {
			return
		}
		pairs = append(pairs, pair{raw, repl})
		seen[raw] = true
	}

	// Only table/view references are rewritten; CALL targets (isCall) are left
	// untouched, matching the original behavior.
	for _, r := range extractObjectRefs(sql, tokens) {
		if r.isCall {
			continue
		}
		consider(r.raw)
	}

	if len(pairs) == 0 {
		return sql
	}

	// Longest-first so a 3-part "A.B.C" is replaced before "B.C" or "C".
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].orig) > len(pairs[j].orig)
	})

	result := sql
	for _, p := range pairs {
		result = strings.ReplaceAll(result, p.orig, p.repl)
	}
	return result
}

// splitIdent splits a (possibly quoted, possibly multi-part) identifier string
// into its component parts, stripping surrounding double-quotes from each part.
func splitIdent(s string) []string {
	var parts []string
	for _, p := range strings.Split(s, ".") {
		parts = append(parts, stripQuotes(p))
	}
	return parts
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
