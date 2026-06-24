// Package sqlgrammar is a recursive-descent grammar engine for Snowflake SQL.
//
// It consumes the significant-token stream produced by internal/sqltok and
// validates that a statement conforms to a known Snowflake grammar. Because SQL
// allows nested structures (parenthesized expressions, subqueries, nested option
// lists), the engine is a pushdown automaton realized as recursive descent:
// Go's call stack is the automaton's memory, the pos cursor is the state pointer,
// and the per-command Parse* rules are the state transitions.
//
// The package is a leaf: it imports only internal/sqltok and is imported by
// internal/sqleditor (diagnostics + autocomplete). It must never import
// internal/sqleditor (import cycle).
//
// Per issue #556 every per-command func (v *Validator) ParseXxx() bool rule is
// implemented (CREATE/ALTER/DROP/SHOW/DESCRIBE/UNDROP/GRANT/REVOKE plus DML and
// the session/transaction/data-loading/execute commands). The top-level
// dispatch (Recognized / ParseTopLevel, dispatch.go) routes a statement to the
// candidate rules selected by its leading keyword and accepts it when any rule
// consumes it whole. The diagnostics consumer is internal/sqleditor.ValidateGrammar.
//
// thaw:domain: SQL Editor & Diagnostics
package sqlgrammar
