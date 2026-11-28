---
name: thaw-sql-validator-tdd
description: >-
  Use this skill when improving, debugging, or fixing false positives/negatives in Thaw's
  Snowflake SQL tokenizer, recursive-descent grammar engine, or SQL editor diagnostics
  (`internal/sqltok`, `internal/sqlgrammar`, `internal/sqleditor`).
---

# Thaw Snowflake SQL Validator TDD Workflow

Use this red-green-refactor loop to fix bugs or add Snowflake SQL grammar and diagnostics coverage across Thaw's three-layer SQL analysis stack:
`internal/sqltok` (leaf tokenizer) → `internal/sqlgrammar` (recursive-descent grammar + `ExpectedAt`) → `internal/sqleditor` (diagnostics orchestrator, anti-patterns, existence checks, autocomplete).

## Phase 1: Research & Locate the Right Layer
1. Consult official Snowflake documentation (`docs.snowflake.com`) for the exact syntax, optional modifiers (`OR REPLACE`, `OR ALTER`, `IF NOT EXISTS`, `CLUSTER BY` placement), required pairings, and edge cases.
2. Read the relevant package README before editing:
   - `internal/sqltok/README.md` — token kinds, comments (`--`, `//`, nested `/* */`), `$$` blocks, statement splitting.
   - `internal/sqlgrammar/README.md` — statement skeleton rules (`create.go`, `alter.go`, `drop.go`, `show.go`, `dml.go`, `scripting.go`, `engine.go`, `dispatch.go`).
   - `internal/sqleditor/README.md` — `ValidateSyntax`, `ValidateGrammar`, `ValidateDataTypes`, `ValidateAntiPatterns`, `ValidateSemantics`, `ValidateTablesExist`, `ValidateBareColumnRefs`, `orchestrator.go`.
3. Decide which layer owns the behavior:
   - Lexing/splitting bug → `internal/sqltok`
   - Statement syntax skeleton / option list / `ExpectedAt` completion → `internal/sqlgrammar`
   - Semantic check, data-type validation, object/column existence, scripting scope, or anti-pattern → `internal/sqleditor`

## Phase 2: Red — Write Failing Regression Tests
1. Add both **valid** (must produce zero false-positive errors/warnings) and **invalid** (must produce a targeted diagnostic at the expected span) SQL test cases:
   - For grammar rules: add to the matching `internal/sqlgrammar/*_test.go` using `assertValid`/`assertInvalid` from `grammar_test.go` (or an `issue<N>_test.go` for a tracked issue).
   - For editor diagnostics: add to `internal/sqleditor/validation_test.go`, `grammar_validation_test.go`, `antipatterns_test.go`, `orchestrator_test.go`, or an `issue<N>_test.go`.
2. Run the targeted test and confirm it fails for the right reason:
   ```bash
   go test -v -run <TestName> ./internal/sqlgrammar/... ./internal/sqleditor/...
   ```

## Phase 3: Green — Implement the Minimal Fix
1. Follow strict SQL analysis invariants:
   - **Never hand-roll SQL lexing** — operate on `sqltok` tokens (`tok.Text(src)`), `MatchWord` for option/clause words, `unorderedOnce` for order-independent options, and `snowflake.SplitQualifiedName` / `snowflake.IdentEqual` for names.
   - **Never re-locate spans with `strings.Index(sql, rawPath)`** — pass token index `pathIdx` or use `sigIndexAtOffset`.
   - **Never introduce catalog false positives** when schema/object metadata was not fetched.
   - Keep `internal/sqlgrammar` importing ONLY `internal/sqltok`.
2. Re-run the full SQL analysis suites (including `create_meta_test.go` and `dispatch_test.go`):
   ```bash
   go test ./internal/sqltok/... ./internal/sqlgrammar/... ./internal/sqleditor/...
   ```
   Do not weaken or delete existing test cases to force a pass.

## Phase 4: Verify Downstream Consumers & Update Docs
1. `internal/mcp` (`validate_sql`) and `internal/snowflake` (`CollectCTENames`) share these packages — verify the whole Go suite:
   ```bash
   go test -race ./...
   ```
2. If you changed any exported method signature on `sqleditor.Service`, run `wails3 generate bindings -ts -i -clean=true` and `cd frontend && npx tsc --noEmit && npm test`.
3. Update `internal/sqleditor/README.md`, `internal/sqlgrammar/README.md`, `internal/sqltok/README.md`, and `README.md` / `FEATURES.md` in the same PR.
