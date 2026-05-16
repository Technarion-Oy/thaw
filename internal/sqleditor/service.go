// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sqleditor

import "thaw/internal/snowflake"

// Service is a Wails-bound service that exposes SQL editor diagnostics,
// autocomplete helpers, and JOIN suggestion endpoints to the frontend.
// All methods are stateless — no Snowflake connection is required.
type Service struct{}

// NewService creates a new sqleditor Service instance for Wails binding.
func NewService() *Service {
	return &Service{}
}

// AnalyzeSqlSyntax runs the custom Snowflake SQL tokenizer on the given text
// and returns structural error markers (unclosed strings, unmatched parens,
// bad scripting assignments, etc.).
func (s *Service) AnalyzeSqlSyntax(sql string) []DiagMarker {
	return ValidateSyntax(sql)
}

// ParseJoinTableRefs extracts all FROM/JOIN table references (with aliases)
// from the given SQL text.
func (s *Service) ParseJoinTableRefs(sql string) []JoinTableRef {
	return ParseJoinTables(sql)
}

// ComputeJoinOnConditions computes JOIN ON / USING condition suggestions using
// FK constraints, PK naming heuristics, and type-compatible same-name columns.
// The caller is responsible for fetching and passing FK and column data.
func (s *Service) ComputeJoinOnConditions(req JoinOnSuggestionsReq) []JoinCondition {
	return ComputeJoinOnConditions(req)
}

// AnalyzeSqlSemantics validates alias.column references in SQL against the
// provided column info, returning Warning markers for unrecognized column names.
func (s *Service) AnalyzeSqlSemantics(sql string, resolvedRefs []ResolvedRef, colEntries []ColEntry) []DiagMarker {
	return ValidateSemantics(sql, resolvedRefs, colEntries)
}

// ValidateSnowflakePatterns runs custom Snowflake anti-pattern checks and
// statement preamble validation.
func (s *Service) ValidateSnowflakePatterns(sql string, stmtRanges []StatementRange) []DiagMarker {
	return ValidateSnowflakePatterns(sql, stmtRanges)
}

// ValidateDataTypes checks CREATE TABLE, ALTER TABLE ADD COLUMN, CAST(), and
// shorthand cast (::) expressions for unrecognized Snowflake data type names.
func (s *Service) ValidateDataTypes(sql string, stmtRanges []StatementRange) []DiagMarker {
	return ValidateDataTypes(sql, stmtRanges)
}

// ValidateTablesExist checks SELECT/CREATE/ALTER/DROP/UNDROP statements for
// references to databases, schemas, or tables that are absent from the resolved
// references or known catalogs.
func (s *Service) ValidateTablesExist(req ValidateTablesExistRequest) []DiagMarker {
	return ValidateTablesExist(req)
}

// ValidateBareColumnRefs checks INSERT column lists and CREATE TABLE REFERENCES
// column lists against the column info cache.  It also builds an in-script
// column cache from CREATE TABLE statements so that subsequent INSERTs can
// validate against tables created earlier in the same script.
func (s *Service) ValidateBareColumnRefs(req ValidateBareColsRequest) []DiagMarker {
	return ValidateBareColumnRefs(req)
}

// GetScriptingCompletions extracts declared Snowflake Scripting variables
// visible at cursorOffset and determines whether a ':' prefix is required for
// completions.
func (s *Service) GetScriptingCompletions(sql string, cursorOffset int) ScriptingCompletionResult {
	return GetScriptingCompletions(sql, cursorOffset)
}

// GetSqlStatementRanges splits sql into per-statement line ranges and byte offsets.
func (s *Service) GetSqlStatementRanges(sql string) []StatementRange {
	return GetStatementRanges(sql)
}

// GetIdentifierAtColumn parses the dot-separated identifier (e.g. db.schema.table)
// under the zero-indexed cursor column col within a single line of SQL.
// Returns nil when the column is not on any identifier.
func (s *Service) GetIdentifierAtColumn(line string, col int) []string {
	return GetIdentifierAtColumn(line, col)
}

// GetActiveFunctionCall parses the SQL prefix (text from document start to
// cursor) and returns the innermost open function call with its active parameter
// index.  Returns nil when the cursor is not inside a named function call.
func (s *Service) GetActiveFunctionCall(prefix string) *FunctionCallContext {
	return GetActiveFunctionCall(prefix)
}

// ParseSignatureParams extracts the byte spans of each parameter within a
// function signature string for Monaco parameter-label highlighting.
func (s *Service) ParseSignatureParams(sig string) []SignatureParam {
	return ParseSignatureParams(sig)
}

// ApplySqlCasing applies token-level keyword/identifier/function casing to a
// formatted SQL string.  Quoted identifiers, string literals, dollar-quoted
// blocks, and comments are passed through unchanged.
func (s *Service) ApplySqlCasing(sql, keywordCase, identifierCase, functionCase string) string {
	return ApplyCasing(sql, keywordCase, identifierCase, functionCase)
}

// GetAutocompleteContext bundles statement ranges, scripting completions, table
// references, and CTE column projections for the cursor position into a single
// response, reducing IPC round-trips for the frontend completion provider.
func (s *Service) GetAutocompleteContext(sql string, cursorOffset int) AutocompleteContext {
	return GetAutocompleteContext(sql, cursorOffset)
}

// GetAutocompleteContextFull extends GetAutocompleteContext with ref resolution
// and in-editor CREATE TABLE column extraction, reducing the frontend to a thin
// wrapper. It resolves unqualified table refs against store objects, UseContext,
// and session context, and extracts columns from CREATE TABLE statements in the
// editor text.
func (s *Service) GetAutocompleteContextFull(req AutocompleteContextRequest) AutocompleteContext {
	return GetAutocompleteContextFull(req)
}

// ResolveTableRefs resolves an array of table references against store objects,
// UseContext, and session context. Used by hover/diagnostics paths that already
// have refs but need qualification.
func (s *Service) ResolveTableRefs(refs []JoinTableRef, storeObjects []StoreObject, useCtx *UseContext, session *SessionContext) []ResolvedRef {
	return ResolveTableRefs(refs, storeObjects, useCtx, session)
}

// GetSnowflakeKeywords returns the full list of Snowflake SQL reserved keywords.
func (s *Service) GetSnowflakeKeywords() []string {
	return snowflake.ReservedKeywords()
}
