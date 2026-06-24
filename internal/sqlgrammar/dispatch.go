package sqlgrammar

import (
	"reflect"
	"strings"
	"sync"

	"thaw/internal/sqltok"
)

// ruleFn is a grammar rule bound as a plain function of its receiver — the shape
// of a method expression such as (*Validator).ParseCreateDatabase.
type ruleFn = func(*Validator) bool

// The dispatch registry maps a statement's leading keyword (uppercased) to the
// set of grammar rules that can begin with it. A statement is valid when ANY
// candidate consumes it to the end (see ParseTopLevel) — so overlapping variants
// (CREATE DATABASE vs CREATE DATABASE ROLE, the several CREATE TABLE forms, …)
// disambiguate by longest full match rather than by hand-ordered precedence.
var (
	registryOnce sync.Once
	registry     map[string][]ruleFn
)

// prefixFamilies groups the bulk command families by Parse* method-name prefix,
// discovered via reflection so the registry never needs hand-maintenance as new
// per-command rules are added. Each prefix maps to the leading keyword(s) that
// select it.
var prefixFamilies = []struct {
	prefix   string
	keywords []string
}{
	{"ParseCreate", []string{"CREATE"}},
	{"ParseAlter", []string{"ALTER"}},
	{"ParseDrop", []string{"DROP"}},
	{"ParseUndrop", []string{"UNDROP"}},
	{"ParseShow", []string{"SHOW"}},
	{"ParseDescribe", []string{"DESC", "DESCRIBE"}},
	{"ParseGrant", []string{"GRANT"}},
	{"ParseRevoke", []string{"REVOKE"}},
}

// dispatchExclude lists the generic "CREATE/ALTER/DROP/… <object>" index-page
// rules. They are deliberately kept OUT of top-level dispatch: each accepts
// almost any leader + name + arbitrary tail, which would mask real errors
// (missing column lists, malformed column defs, unknown object types). The
// specific per-command rules — which now also accept the CREATE OR ALTER form
// via orReplace — govern validation instead. The functions still exist (and are
// covered by the reflection meta-test); they are just not dispatch entry points.
var dispatchExclude = map[string]bool{
	"ParseCreateObj":        true,
	"ParseCreateObjClone":   true,
	"ParseCreateOrAlterObj": true,
	"ParseAlterObj":         true,
	"ParseDropObj":          true,
	"ParseShowObjs":         true,
	"ParseDescribeObj":      true,
	"ParseUndropObj":        true,
	// Redundant duplicate of ParseCreateTable for the CREATE-TABLE-with-constraint
	// doc page: its CREATE form validates the same column/constraint list (now with
	// the inline/out-of-line constraint clauses modeled), so ParseCreateTable
	// already governs that leader; and its ALTER form is unreachable here (ALTER
	// routes to ParseAlter*). Kept defined (and meta-tested), just not dispatched.
	"ParseCreateAlterTableConstraint": true,
}

func buildRegistry() map[string][]ruleFn {
	reg := map[string][]ruleFn{}

	// Bulk families by method-name prefix (reflection).
	vt := reflect.TypeFor[*Validator]()
	for i := 0; i < vt.NumMethod(); i++ {
		m := vt.Method(i)
		if dispatchExclude[m.Name] {
			continue
		}
		fn, ok := m.Func.Interface().(ruleFn)
		if !ok {
			continue
		}
		for _, pf := range prefixFamilies {
			if !strings.HasPrefix(m.Name, pf.prefix) {
				continue
			}
			for _, kw := range pf.keywords {
				reg[kw] = append(reg[kw], fn)
			}
		}
	}

	// DML and other statement leaders are enumerated explicitly: their rules
	// don't share a clean family prefix, and the query-construct rules
	// (ParseWhere/ParseFrom/ParseJoin/…) are sub-clauses, not statements, so
	// they are intentionally absent from top-level dispatch.
	misc := map[string][]ruleFn{
		"SELECT":   {(*Validator).ParseSelect},
		"WITH":     {(*Validator).ParseWith},
		"VALUES":   {(*Validator).ParseValues},
		"INSERT":   {(*Validator).ParseInsert, (*Validator).ParseInsertMultiTable},
		"UPDATE":   {(*Validator).ParseUpdate},
		"DELETE":   {(*Validator).ParseDelete},
		"MERGE":    {(*Validator).ParseMerge},
		"TRUNCATE": {(*Validator).ParseTruncateTable, (*Validator).ParseTruncateMaterializedView},
		"COPY":     {(*Validator).ParseCopyIntoTable, (*Validator).ParseCopyIntoLocation, (*Validator).ParseCopyFiles},
		"GET":      {(*Validator).ParseGet},
		"PUT":      {(*Validator).ParsePut},
		"LIST":     {(*Validator).ParseList},
		"REMOVE":   {(*Validator).ParseRemove},
		"CALL":     {(*Validator).ParseCall, (*Validator).ParseCallWithAnonymousProcedure},
		"COMMENT":  {(*Validator).ParseComment},
		"EXPLAIN":  {(*Validator).ParseExplain},
		"EXECUTE": {
			(*Validator).ParseExecuteImmediate, (*Validator).ParseExecuteImmediateFrom,
			(*Validator).ParseExecuteTask, (*Validator).ParseExecuteAlert,
			(*Validator).ParseExecuteDbtProject, (*Validator).ParseExecuteDcmProject,
			(*Validator).ParseExecuteJobService, (*Validator).ParseExecuteNotebook,
			(*Validator).ParseExecuteNotebookProject,
		},
		"USE": {
			(*Validator).ParseUse, (*Validator).ParseUseDatabase, (*Validator).ParseUseRole,
			(*Validator).ParseUseSchema, (*Validator).ParseUseSecondaryRoles, (*Validator).ParseUseWarehouse,
		},
		"SET":      {(*Validator).ParseSet},
		"UNSET":    {(*Validator).ParseUnset},
		"BEGIN":    {(*Validator).ParseBegin},
		"START":    {(*Validator).ParseBegin}, // START TRANSACTION
		"COMMIT":   {(*Validator).ParseCommit},
		"ROLLBACK": {(*Validator).ParseRollback},
	}
	for kw, rules := range misc {
		reg[kw] = append(reg[kw], rules...)
	}
	return reg
}

func rules() map[string][]ruleFn {
	registryOnce.Do(func() { registry = buildRegistry() })
	return registry
}

// leadKeyword returns the uppercased text of the first significant token, or ""
// when the statement has no significant tokens. The lexer classifies some
// command leaders (SHOW, SET, …) as Identifier rather than Keyword, so the
// dispatch keys off token text, not token kind.
func (v *Validator) leadKeyword() string {
	if len(v.tokens) == 0 {
		return ""
	}
	return strings.ToUpper(v.tokens[0].Text(v.src))
}

// candidates returns the grammar rules whose statements can begin with the
// current leading keyword (nil when none — i.e. an unmodeled statement).
func (v *Validator) candidates() []ruleFn {
	return rules()[v.leadKeyword()]
}

// StatementKind classifies a statement by its effective top-level verb. A leading
// WITH (CTE) prefix is looked past, so a CTE'd query is classified by the
// statement it feeds — SELECT/INSERT/UPDATE/DELETE/MERGE — rather than by WITH.
// DDL, session, transaction, and utility commands are StmtOther; callers needing
// the precise leader for those use leadKeyword instead.
type StatementKind int

const (
	StmtOther StatementKind = iota
	StmtSelect
	StmtInsert
	StmtUpdate
	StmtDelete
	StmtMerge
)

// String renders the kind as its SQL verb, or "OTHER".
func (k StatementKind) String() string {
	switch k {
	case StmtSelect:
		return "SELECT"
	case StmtInsert:
		return "INSERT"
	case StmtUpdate:
		return "UPDATE"
	case StmtDelete:
		return "DELETE"
	case StmtMerge:
		return "MERGE"
	default:
		return "OTHER"
	}
}

// dmlVerb maps an uppercased leading word to its StatementKind, or StmtOther.
func dmlVerb(word string) StatementKind {
	switch word {
	case "SELECT":
		return StmtSelect
	case "INSERT":
		return StmtInsert
	case "UPDATE":
		return StmtUpdate
	case "DELETE":
		return StmtDelete
	case "MERGE":
		return StmtMerge
	}
	return StmtOther
}

// IdentifyStatement classifies the statement by its effective top-level verb,
// looking past a leading WITH … CTE prefix. A WITH clause may precede SELECT,
// INSERT, UPDATE, DELETE, or MERGE, so keying off the literal first keyword would
// misclassify every CTE'd statement as "WITH". To skip the CTE list safely it
// tracks parenthesis depth and inspects only depth-0 tokens: a verb buried inside
// a CTE's (always parenthesized) subquery is never mistaken for the main verb.
// Because the tokenizer already separated string- and comment-text, a '(' or a
// verb inside a literal cannot perturb the depth count. It does not move the
// validator cursor.
func (v *Validator) IdentifyStatement() StatementKind {
	if len(v.tokens) == 0 {
		return StmtOther
	}
	if !strings.EqualFold(v.tokens[0].Text(v.src), "WITH") {
		return dmlVerb(strings.ToUpper(v.tokens[0].Text(v.src)))
	}
	depth := 0
	for i := 1; i < len(v.tokens); i++ {
		switch v.tokens[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				if k := dmlVerb(strings.ToUpper(v.tokens[i].Text(v.src))); k != StmtOther {
					return k
				}
			}
		}
	}
	return StmtOther
}

// Recognized reports whether the statement's leading keyword maps to at least
// one implemented grammar. Diagnostics only flag a statement when it is
// Recognized, so unmodeled-but-valid SQL is never marked.
func (v *Validator) Recognized() bool {
	return len(v.candidates()) > 0
}

// ParseTopLevel validates the statement against every grammar rule its leading
// keyword selects, accepting it if ANY rule consumes the whole statement (a
// single optional trailing semicolon is tolerated). On failure the furthest
// position reached across all attempts is retained for Failure()/diagnostics.
//
// It returns false for an unrecognized leading keyword; gate with Recognized()
// first if you want to distinguish "unmodeled" from "modeled but malformed".
func (v *Validator) ParseTopLevel() bool {
	for _, rule := range v.candidates() {
		saved := v.save()
		if rule(v) {
			v.Optional(func() bool { return v.Match(sqltok.Semicolon) })
			if v.AtEnd() {
				return true
			}
		}
		v.restore(saved)
	}
	return false
}

// ExpectedAt answers "what is valid next at the cursor?" — the grammar's expected
// set at the byte offset cursorOffset within the statement, which is exactly the
// candidate-completion set an autocomplete provider needs.
//
// It parses the token prefix lying before the cursor and returns the distinct
// labels the grammar expected at the furthest point it reached (via the same
// furthest/expected machinery diagnostics uses). The token straddling — or word-
// like and immediately abutting — the cursor is the half-typed word being
// completed, so it is dropped before parsing: the expectation reflects the
// position, not the partial token (Monaco filters the visible items by it).
//
// Labels are keyword/option words (FROM, TAG), operators (=, ::), or token-kind
// names (Identifier, StringLit, …) — see the engine's expect() calls. Returns nil
// for an empty prefix or an unrecognized leading keyword; gate with Recognized()
// upstream to keep completion leading-keyword-gated.
func (v *Validator) ExpectedAt(cursorOffset int) []string {
	end := 0
	for end < len(v.tokens) && v.tokens[end].End <= cursorOffset {
		end++
	}
	// Drop a word-like token that abuts the cursor with no separating gap — the
	// partially-typed word. A token straddling the cursor (Start < cursor < End)
	// is already excluded by the End <= cursor bound above.
	if end > 0 {
		t := v.tokens[end-1]
		if t.End == cursorOffset && (t.Kind.IsIdentLike() || t.Kind == sqltok.NumberLit) {
			end--
		}
	}
	pv := &Validator{src: v.src, tokens: v.tokens[:end]}
	pv.ParseTopLevel()
	return pv.Failure().Expected
}
