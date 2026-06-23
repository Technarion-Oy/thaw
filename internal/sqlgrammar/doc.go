// Package sqlgrammar is a recursive-descent grammar engine for Snowflake SQL.
//
// It consumes the significant-token stream produced by internal/sqltok and
// validates that a statement conforms to a known Snowflake grammar. Because SQL
// allows nested structures (parenthesised expressions, subqueries, nested option
// lists), the engine is a pushdown automaton realised as recursive descent:
// Go's call stack is the automaton's memory, the pos cursor is the state pointer,
// and the per-command Parse* rules are the state transitions.
//
// The package is a leaf: it imports only internal/sqltok and is imported by
// internal/sqleditor (diagnostics + autocomplete). It must never import
// internal/sqleditor (import cycle).
//
// The recursive-descent bodies are implemented incrementally per issue #556:
// ParseCreateDatabase (internal/sqlgrammar/create.go) is the first real rule;
// the remaining per-command func (v *Validator) ParseXxx() bool rules are still
// stubs that return true until they are translated from their doc syntax.
//
// thaw:domain: SQL Editor & Diagnostics
package sqlgrammar
