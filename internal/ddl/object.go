// SPDX-License-Identifier: GPL-3.0-or-later

package ddl

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"thaw/internal/sqltok"
)

// ─── object kinds ────────────────────────────────────────────────────────────

// Kind is a Snowflake DDL object type extracted from a CREATE statement.
type Kind string

const (
	KindDatabase   Kind = "DATABASE"
	KindSchema     Kind = "SCHEMA"
	KindTable      Kind = "TABLE"
	KindView       Kind = "VIEW"
	KindFunction   Kind = "FUNCTION"
	KindProcedure  Kind = "PROCEDURE"
	KindSequence   Kind = "SEQUENCE"
	KindStage      Kind = "STAGE"
	KindStream     Kind = "STREAM"
	KindTask       Kind = "TASK"
	KindFileFormat Kind = "FILE FORMAT"
	KindPipe       Kind = "PIPE"
	KindUnknown    Kind = "UNKNOWN"
)

// dirFor maps a Kind to the plural, lowercase sub-directory name used on disk.
func dirFor(k Kind) string {
	switch k {
	case KindTable:
		return "tables"
	case KindView:
		return "views"
	case KindFunction:
		return "functions"
	case KindProcedure:
		return "procedures"
	case KindSequence:
		return "sequences"
	case KindStage:
		return "stages"
	case KindStream:
		return "streams"
	case KindTask:
		return "tasks"
	case KindFileFormat:
		return "file_formats"
	case KindPipe:
		return "pipes"
	case KindSchema:
		return "schemas"
	default:
		return "other"
	}
}

// ─── Object ──────────────────────────────────────────────────────────────────

// Object holds the metadata extracted from a single DDL statement.
type Object struct {
	Kind     Kind
	Database string // may be empty when not fully-qualified
	Schema   string // may be empty for DB-level objects
	Name     string // bare object name (unquoted)
	ArgSig   string // non-empty only for FUNCTION / PROCEDURE overloads
	SQL      string // full DDL text (without trailing semicolon)
}

// DefaultExportPathTemplate is the path template used when no custom template
// has been configured. Placeholders: {database}, {schema}, {object_type},
// {object_name}.
const DefaultExportPathTemplate = "{database}/{schema}/{object_type}/{object_name}.sql"

// FilePath returns the path relative to the database output directory where
// this object's .sql file should be written.
//
// Layout:
//
//	_database.sql
//	schemas/<SCHEMA>.sql
//	<SCHEMA>/tables/<TABLE>.sql
//	<SCHEMA>/views/<VIEW>.sql
//	<SCHEMA>/functions/<NAME>__<ARGSIG>.sql
//	… etc.
func (o *Object) FilePath() string {
	switch o.Kind {
	case KindDatabase:
		return "_database.sql"
	case KindSchema:
		return filepath.Join("schemas", sanitize(o.Name)+".sql")
	default:
		schema := o.Schema
		if schema == "" {
			schema = "_root"
		}
		fname := sanitize(o.Name)
		if (o.Kind == KindFunction || o.Kind == KindProcedure) && o.ArgSig != "" {
			fname = fname + "__" + o.ArgSig
		}
		return filepath.Join(sanitize(schema), dirFor(o.Kind), fname+".sql")
	}
}

// FilePathFor returns the path relative to OutputDir where this object's .sql
// file should be written, applying the given path template.
//
// For KindDatabase and KindSchema the path is always fixed regardless of the
// template, since they are structural anchors:
//
//	<database>/_database.sql
//	<database>/schemas/<SCHEMA>.sql
//
// For all other kinds the template is applied with the following substitutions:
//
//	{database}    → sanitized database name
//	{schema}      → sanitized schema name (or "_root" when absent)
//	{object_type} → plural lowercase type directory (e.g. "tables", "views")
//	{object_name} → sanitized object name (includes __argsig for overloads)
//
// An empty template falls back to DefaultExportPathTemplate.
func (o *Object) FilePathFor(template, database string) string {
	switch o.Kind {
	case KindDatabase:
		return filepath.Join(sanitize(database), "_database.sql")
	case KindSchema:
		return filepath.Join(sanitize(database), "schemas", sanitize(o.Name)+".sql")
	default:
		if template == "" {
			template = DefaultExportPathTemplate
		}
		schema := o.Schema
		if schema == "" {
			schema = "_root"
		}
		fname := sanitize(o.Name)
		if (o.Kind == KindFunction || o.Kind == KindProcedure) && o.ArgSig != "" {
			fname = fname + "__" + o.ArgSig
		}
		// Four direct replacements on a short template string are faster
		// than constructing a strings.Replacer state machine per object.
		path := strings.ReplaceAll(template, "{database}", sanitize(database))
		path = strings.ReplaceAll(path, "{schema}", sanitize(schema))
		path = strings.ReplaceAll(path, "{object_type}", dirFor(o.Kind))
		path = strings.ReplaceAll(path, "{object_name}", fname)
		return filepath.FromSlash(path)
	}
}

// ─── CREATE statement classification ─────────────────────────────────────────

// createModifiers are the optional keywords Snowflake allows between CREATE
// (or CREATE OR REPLACE) and the object-type keyword, e.g.:
//
//	CREATE OR REPLACE TRANSIENT TABLE …
//	CREATE OR REPLACE SECURE RECURSIVE VIEW …
//	CREATE OR REPLACE MATERIALIZED VIEW …
var createModifiers = map[string]struct{}{
	"TRANSIENT": {}, "TEMPORARY": {}, "TEMP": {}, "VOLATILE": {},
	"SECURE": {}, "RECURSIVE": {}, "MATERIALIZED": {}, "EXTERNAL": {},
	"DYNAMIC": {}, "ICEBERG": {}, "EVENT": {}, "LOCAL": {}, "GLOBAL": {},
}

// createKinds maps the object-type keyword to its Kind. FILE FORMAT is the one
// two-word type and is handled separately.
var createKinds = map[string]Kind{
	"TABLE": KindTable, "VIEW": KindView, "SCHEMA": KindSchema,
	"FUNCTION": KindFunction, "PROCEDURE": KindProcedure,
	"SEQUENCE": KindSequence, "STAGE": KindStage, "STREAM": KindStream,
	"TASK": KindTask, "PIPE": KindPipe, "DATABASE": KindDatabase,
}

// ─── Parse ────────────────────────────────────────────────────────────────────

// Parse extracts metadata from a single DDL statement.
// Unknown or non-CREATE statements are returned with Kind == KindUnknown.
//
// Classification runs over the [sqltok] token stream rather than an anchored
// regex, so leading header comments and comments between the keywords —
// normal in user-authored migration scripts — do not hide the statement:
//
//	-- header
//	CREATE /* modifier */ TABLE t (i INT)   →  kind=TABLE name=t
func Parse(sql string) Object {
	obj := Object{SQL: sql, Kind: KindUnknown}

	sig := sqltok.SignificantTokens(sql)
	if len(sig) == 0 || !eqFold(sig, sql, 0, "CREATE") {
		return obj
	}
	i := 1

	// Optional OR REPLACE.
	if eqFold(sig, sql, i, "OR") && eqFold(sig, sql, i+1, "REPLACE") {
		i += 2
	}

	// Any number of modifier keywords (TRANSIENT, SECURE, MATERIALIZED, …).
	for i < len(sig) {
		if _, isMod := createModifiers[upperText(sig, sql, i)]; !isMod {
			break
		}
		i++
	}

	// The object-type keyword.
	if i >= len(sig) {
		return obj
	}
	if upperText(sig, sql, i) == "FILE" && eqFold(sig, sql, i+1, "FORMAT") {
		obj.Kind = KindFileFormat
		i += 2
	} else if k, ok := createKinds[upperText(sig, sql, i)]; ok {
		obj.Kind = k
		i++
	} else {
		return obj
	}

	// Optional IF NOT EXISTS.
	if eqFold(sig, sql, i, "IF") && eqFold(sig, sql, i+1, "NOT") && eqFold(sig, sql, i+2, "EXISTS") {
		i += 3
	}

	parts, next := sqltok.ReadIdentParts(sig, sql, i, 3)
	for j, p := range parts {
		parts[j] = sqltok.Unquote(p)
	}
	switch len(parts) {
	case 1:
		obj.Name = parts[0]
	case 2:
		obj.Schema, obj.Name = parts[0], parts[1]
	case 3:
		obj.Database, obj.Schema, obj.Name = parts[0], parts[1], parts[2]
	}

	// For overloadable objects, derive a signature from the argument list that
	// follows the name so that overloads land in distinct files.
	if obj.Kind == KindFunction || obj.Kind == KindProcedure {
		if next < len(sig) {
			obj.ArgSig = parseArgSig(sql[sig[next].Start:])
		}
	}

	return obj
}

// eqFold reports whether sig[i] exists and its source text equals word,
// case-insensitively.
func eqFold(sig []sqltok.Token, src string, i int, word string) bool {
	return i < len(sig) && strings.EqualFold(sig[i].Text(src), word)
}

// upperText returns the upper-cased source text of sig[i], or "" when i is out
// of range.
func upperText(sig []sqltok.Token, src string, i int) string {
	if i >= len(sig) {
		return ""
	}
	return strings.ToUpper(sig[i].Text(src))
}

// splitParamList splits s on commas that are at paren depth 0, so commas
// inside size qualifiers like NUMBER(38,0) are not treated as separators.
func splitParamList(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// parseArgSig extracts a simplified, sanitized argument-type signature from
// the text that immediately follows a function/procedure name, e.g.:
//
//	"(X FLOAT, Y VARCHAR(256))"  →  "FLOAT_VARCHAR"
//	"()"                         →  "noargs"
//	""                           →  ""  (no parens found)
func parseArgSig(after string) string {
	after = strings.TrimSpace(after)
	if len(after) == 0 || after[0] != '(' {
		return ""
	}

	// Find the matching closing parenthesis.
	depth := 0
	end := -1
	for i, r := range after {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return ""
	}

	inner := strings.TrimSpace(after[1:end])
	if inner == "" {
		return "noargs"
	}

	// Each comma-separated param is either "name TYPE" or just "TYPE".
	// We want only the TYPE portion, stripped of size qualifiers like (256).
	// Use a paren-depth-aware split so commas inside NUMBER(38,0) are not
	// treated as parameter separators.
	var types []string
	for _, param := range splitParamList(inner) {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		fields := strings.Fields(param)
		var typeName string
		if len(fields) >= 2 {
			typeName = fields[1] // "name TYPE …"
		} else {
			typeName = fields[0] // "TYPE"
		}
		// Strip any size qualifier: VARCHAR(256) → VARCHAR
		if idx := strings.IndexByte(typeName, '('); idx >= 0 {
			typeName = typeName[:idx]
		}
		types = append(types, sanitize(strings.ToUpper(typeName)))
	}
	if len(types) == 0 {
		return "noargs"
	}
	return strings.Join(types, "_")
}

// ─── sanitize ────────────────────────────────────────────────────────────────

// sanitize converts s into a string safe to use as a file or directory name
// component: only ASCII letters, digits, hyphens, and underscores are kept;
// everything else becomes an underscore.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ─── nameTracker ─────────────────────────────────────────────────────────────

// nameTracker resolves file-path collisions that arise when multiple DDL
// statements map to the same path (e.g. overloaded functions with identical
// sanitized argument signatures, or duplicate objects across schemas).
//
// The first occurrence keeps the plain path; subsequent ones get a numeric
// suffix: foo.sql → foo_2.sql → foo_3.sql …
type nameTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// newNameTracker returns an initialized nameTracker ready for use.
func newNameTracker() *nameTracker {
	return &nameTracker{seen: make(map[string]struct{})}
}

// resolve returns a unique path for the given candidate, updating internal
// state so that the same candidate cannot be returned twice.
// Generated suffixes (_2, _3, …) are also registered, so a legitimately-named
// object that happens to match a suffix is handled without collisions.
func (t *nameTracker) resolve(path string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.seen[path]; !exists {
		t.seen[path] = struct{}{}
		return path
	}

	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]

	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d%s", base, n, ext)
		if _, exists := t.seen[candidate]; !exists {
			t.seen[candidate] = struct{}{}
			return candidate
		}
	}
}
