# AI Agent Workflow: Snowflake SQL Validator TDD

## 🎯 Objective
Your goal is to autonomously improve the Snowflake SQL validation engine in this repository. You will research specific Snowflake SQL syntax, generate failing test cases in `test.sql` that capture edge cases or incorrect editor behavior, and then modify the underlying Go validation logic to make the test suite pass.

## 🛠️ Required Tools & Capabilities
To execute this workflow, you must have access to:
1.  **Web Search / Retrieval:** To search and read official Snowflake Documentation (docs.snowflake.com).
2.  **File System Read/Write:** To modify `test.sql` (where the queries live) and the underlying source files (e.g., `internal/sqleditor/validation_test.go`, `sqleditor.go`, `patterns.go`).
3.  **CLI Execution:** To run Go tests (`go test`) and read the standard output/error.

---

## 🔄 The Execution Loop

### Phase 1: Research & Syntax Extraction
1.  **Receive Target:** The user will provide a specific SQL command or syntax subset to focus on (e.g., "Snowflake `COPY INTO` command" or "Snowflake `CREATE DYNAMIC TABLE` options").
2.  **Consult Documentation:** Query the official Snowflake documentation for the target syntax. 
3.  **Identify Edge Cases:** Extract exact syntax rules, optional parameters, and common anti-patterns. Pay special attention to:
    * Allowed modifiers (e.g., `OR REPLACE`, `IF NOT EXISTS`).
    * Required keyword pairings (e.g., `TARGET_LAG` requiring `WAREHOUSE`).
    * Data type constraints.

### Phase 2: Test Case Generation (The "Red" Phase)
1.  **Analyze Existing Tests:** Read `test.sql` to understand how test queries are structured.
2.  **Write Valid Cases:** Append 3-5 new perfectly valid, complex SQL strings to the appropriate section of `test.sql`. 
    * **CRITICAL FORMATTING:** You MUST place a `-- PASS` comment directly above every valid statement.
    * *Example:*
      ```sql
      -- PASS
      CREATE TRANSIENT TABLE valid_table (id INT);
      ```
3.  **Write Invalid Cases:** Append 3-5 new structurally invalid SQL strings to the appropriate section of `test.sql`. 
    * **CRITICAL FORMATTING:** You MUST place a `-- FAIL` comment directly above every invalid statement, ideally including the expected error reason.
    * *Example:*
      ```sql
      -- FAIL: Expected syntax error (missing AS)
      CREATE VIEW invalid_view SELECT 1 FROM t;
      ```
4.  **Execute Tests:** Run the test suite in the terminal to parse `test.sql` and run the validation:
    `go test ./internal/sqleditor/ -v`
5.  **Evaluate:** If the tests pass immediately, your test cases were too basic. Generate harder edge cases based on the documentation. If they fail as expected, capture the exact output and proceed to Phase 3.

### Phase 3: Implementation & Patching (The "Green" Phase)
1.  **Analyze Failures:** Review the `go test` output. Determine *why* the validator failed (e.g., missing regex pattern, unhandled token, strict metadata checking without session context).
2.  **Modify Validation Logic:** Open the source file containing the validation engine (e.g., `sqleditor.go` or the regex pattern definitions). 
3.  **Apply Fixes:** Update the AST traversal, regex patterns, or context checks to handle the new syntax rules. 
    * *Constraint:* Ensure you do not break existing logic. Do not downgrade severe errors to warnings globally unless it specifically resolves a known false-positive pattern.
4.  **Re-run Tests:** Execute the test suite again:
    `go test ./internal/sqleditor/ -v`
5.  **Iterate:** If tests still fail, or if you broke previously passing tests, analyze the new errors, revert your Go code if necessary, and attempt a new fix. DO NOT remove the test cases from `test.sql` to force a pass.

### Phase 4: Refactor & Commit
1.  **Clean Up:** Once all tests pass, review your Go code for formatting (`gofmt`), readability, and efficiency.
2.  **Update Documentation:** Update the SQL validation list in `README.md` (the grammar warnings bullet under "Live SQL diagnostics") to include any newly validated statement types. If you added a new internal package or significantly changed the validation architecture, also update the project structure in `README.md`, `CLAUDE.md`, and `GEMINI.md`.
3.  **Report:** Output a summary to the user detailing:
    * The syntax rules learned from the Snowflake docs.
    * The specific SQL queries added to `test.sql`.
    * The exact functions/patterns modified in the Go source code to fix the behavior.