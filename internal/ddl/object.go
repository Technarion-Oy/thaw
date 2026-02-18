package ddl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// ─── CREATE statement pattern ─────────────────────────────────────────────────

// createRE matches the leading portion of any Snowflake CREATE statement and
// captures the object kind keyword(s) in the named group "kind".
//
// It intentionally allows for multiple optional modifier keywords that Snowflake
// supports before the object-type keyword, e.g.:
//
//	CREATE OR REPLACE TRANSIENT TABLE …
//	CREATE OR REPLACE SECURE RECURSIVE VIEW …
//	CREATE OR REPLACE EXTERNAL TABLE …
//	CREATE OR REPLACE MATERIALIZED VIEW …
var createRE = regexp.MustCompile(
	`(?i)^\s*create\s+(?:or\s+replace\s+)?` +
		`(?:(?:transient|temporary|volatile|secure|recursive|materialized|external|dynamic|iceberg|event)\s+)*` +
		`(?P<kind>file\s+format|table|view|schema|function|procedure|sequence|stage|stream|task|pipe|database)\b`,
)

// kindIdx is the submatch index of the "kind" capture group.
var kindIdx = createRE.SubexpIndex("kind")

// ifNotExistsRE matches an optional IF NOT EXISTS clause so we can skip it.
var ifNotExistsRE = regexp.MustCompile(`(?i)^if\s+not\s+exists\s+`)

// ─── Parse ────────────────────────────────────────────────────────────────────

// Parse extracts metadata from a single DDL statement.
// Unknown or non-CREATE statements are returned with Kind == KindUnknown.
func Parse(sql string) Object {
	obj := Object{SQL: sql, Kind: KindUnknown}

	m := createRE.FindStringSubmatchIndex(sql)
	if m == nil {
		return obj
	}

	// Extract and normalise the kind keyword.
	kindRaw := sql[m[2*kindIdx]:m[2*kindIdx+1]]
	kindStr := strings.ToUpper(strings.Join(strings.Fields(kindRaw), " "))
	obj.Kind = Kind(kindStr)

	// Everything after the matched prefix is where the object name begins.
	rest := strings.TrimSpace(sql[m[1]:])

	// Strip optional IF NOT EXISTS.
	if loc := ifNotExistsRE.FindStringIndex(rest); loc != nil {
		rest = strings.TrimSpace(rest[loc[1]:])
	}

	db, schema, name, argSig := extractIdent(rest, obj.Kind)
	obj.Database = db
	obj.Schema = schema
	obj.Name = name
	obj.ArgSig = argSig

	return obj
}

// ─── identifier extraction ────────────────────────────────────────────────────

// extractIdent parses the qualified name that opens rest and returns its parts.
// For FUNCTION / PROCEDURE it also returns a sanitised argument-type signature
// so that overloaded definitions can coexist as separate files.
func extractIdent(rest string, k Kind) (db, schema, name, argSig string) {
	parts, afterName := tokeniseQualifiedIdent(rest)
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		schema, name = parts[0], parts[1]
	case 3:
		db, schema, name = parts[0], parts[1], parts[2]
	}

	if k == KindFunction || k == KindProcedure {
		argSig = parseArgSig(afterName)
	}
	return
}

// tokeniseQualifiedIdent reads up to three dot-separated identifiers from the
// beginning of s, handling both double-quoted and unquoted forms.
// It returns the parts found and the remainder of the string after the last part.
func tokeniseQualifiedIdent(s string) (parts []string, rest string) {
	rs := []rune(strings.TrimSpace(s))
	n := len(rs)
	i := 0

	for len(parts) < 3 && i < n {
		// Skip leading whitespace between dots and the next part.
		for i < n && (rs[i] == ' ' || rs[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		var part strings.Builder

		if rs[i] == '"' {
			// Double-quoted identifier: consume until the unescaped closing ".
			i++ // skip opening "
			for i < n {
				if rs[i] == '"' {
					i++ // consume "
					if i < n && rs[i] == '"' {
						part.WriteRune('"') // "" escape → literal "
						i++
					} else {
						break // end of quoted identifier
					}
				} else {
					part.WriteRune(rs[i])
					i++
				}
			}
		} else {
			// Unquoted: read until '.', '(', whitespace, or end.
			for i < n && rs[i] != '.' && rs[i] != '(' &&
				rs[i] != ' ' && rs[i] != '\t' && rs[i] != '\n' && rs[i] != '\r' {
				part.WriteRune(rs[i])
				i++
			}
		}

		if p := part.String(); p != "" {
			parts = append(parts, p)
		}

		if i < n && rs[i] == '.' {
			i++ // consume the dot and continue for the next part
		} else {
			break
		}
	}

	rest = string(rs[i:])
	return
}

// parseArgSig extracts a simplified, sanitised argument-type signature from
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
	var types []string
	for _, param := range strings.Split(inner, ",") {
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
// sanitised argument signatures, or duplicate objects across schemas).
//
// The first occurrence keeps the plain path; subsequent ones get a numeric
// suffix: foo.sql → foo_2.sql → foo_3.sql …
type nameTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

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
