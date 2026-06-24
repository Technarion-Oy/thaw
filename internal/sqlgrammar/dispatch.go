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
	// Lenient duplicate of ParseCreateTable for the CREATE-TABLE-with-constraint
	// doc page; its CREATE form (balanced-paren body) masks malformed column
	// lists, and its ALTER form is unreachable here (ALTER routes to ParseAlter*).
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
