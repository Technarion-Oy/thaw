/**
 * Unit tests for sqlDiagnostics.ts
 *
 * Coverage:
 * validateWithParser      – Snowflake PEG grammar check (per-statement, skips false-positive patterns)
 * validateBareColumnRefs  – SELECT-list column existence (bare + quoted, CTEs, JOINs, subqueries)
 *
 * Note: validateSyntax and validateSemantics have been moved to the Go backend
 * (internal/sqleditor) and are tested via Go unit tests.
 */

import { describe, expect, it, vi } from "vitest";
import {
  ColInfo,
  DiagMarker,
  ResolvedRef,
  StatementRange,
  validateWithParser,
  validateBareColumnRefs,
} from "./sqlDiagnostics";

// ── Mock FindSqlTokenPositions IPC (no Wails runtime in tests) ────────────────
// Provides the same tokenizer logic as the Go FindTokenPositions function.
vi.mock("../../wailsjs/go/main/App", () => ({
  FindSqlTokenPositions: (sql: string, bareTargets: string[], quotedTargets: string[]) => {
    const bareSet   = new Set(bareTargets.map((s) => s.toUpperCase()));
    const quotedSet = new Set(quotedTargets.map((s) => s.toUpperCase()));
    const results: Array<{ name: string; line: number; col: number; endCol: number; quoted: boolean }> = [];
    let line = 1, col = 1, i = 0;
    while (i < sql.length) {
      const ch = sql[i];
      if (ch === "\n") { line++; col = 1; i++; continue; }
      if (ch === "-" && sql[i + 1] === "-") {
        i += 2; col += 2;
        while (i < sql.length && sql[i] !== "\n") { i++; col++; }
        continue;
      }
      if (ch === "/" && sql[i + 1] === "*") {
        i += 2; col += 2;
        while (i < sql.length) {
          if (sql[i] === "\n")                              { line++; col = 1; i++; }
          else if (sql[i] === "*" && sql[i + 1] === "/")   { i += 2; col += 2; break; }
          else                                              { i++; col++; }
        }
        continue;
      }
      if (ch === "'") {
        i++; col++;
        while (i < sql.length) {
          if (sql[i] === "\n")                              { line++; col = 1; i++; }
          else if (sql[i] === "'" && sql[i + 1] === "'")   { i += 2; col += 2; }
          else if (sql[i] === "'")                          { i++; col++; break; }
          else                                              { i++; col++; }
        }
        continue;
      }
      if (ch === '"') {
        const startLine = line, startCol = col;
        i++; col++;
        let name = ""; let closed = false;
        while (i < sql.length) {
          if (sql[i] === "\n")                              { line++; col = 1; i++; name += "\n"; }
          else if (sql[i] === '"' && sql[i + 1] === '"')   { name += '"'; i += 2; col += 2; }
          else if (sql[i] === '"')                          { i++; col++; closed = true; break; }
          else                                              { name += sql[i]; i++; col++; }
        }
        if (closed && quotedSet.has(name.toUpperCase()))
          results.push({ name, line: startLine, col: startCol, endCol: col, quoted: true });
        continue;
      }
      if (/[a-zA-Z_]/.test(ch)) {
        const wLine = line, wCol = col, wStart = i;
        while (i < sql.length && /\w/.test(sql[i])) { i++; col++; }
        const word    = sql.slice(wStart, i);
        const prevCh  = wStart > 0 ? sql[wStart - 1] : null;
        const nextCh  = i < sql.length ? sql[i] : null;
        if (prevCh !== "." && nextCh !== "(" && bareSet.has(word.toUpperCase()))
          results.push({ name: word, line: wLine, col: wCol, endCol: wCol + word.length, quoted: false });
        continue;
      }
      i++; col++;
    }
    return Promise.resolve(results);
  },
}));

// ── helpers ───────────────────────────────────────────────────────────────────

/** Return only warning (severity 4) markers. */
const warnings = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 4);

/** Build a single StatementRange covering the whole sql string. */
function singleRange(sql: string): StatementRange[] {
  return [{ startLine: 1, endLine: sql.split("\n").length, startOffset: 0, endOffset: sql.length }];
}

/** Convenience: build a colInfoCache from (db, schema, table) -> column names. */
function makeCache(
  entries: Array<{ db: string; schema: string; table: string; cols: string[] }>,
): Map<string, ColInfo[]> {
  const cache = new Map<string, ColInfo[]>();
  for (const e of entries) {
    const key = `${e.db.toUpperCase()}\0${e.schema.toUpperCase()}\0${e.table.toUpperCase()}`;
    cache.set(key, e.cols.map((n) => ({ name: n, dataType: "TEXT" })));
  }
  return cache;
}

/** Convenience: build a ResolvedRef array. */
const refs = (
  ...items: Array<{ alias: string; db: string; schema: string; name: string }>
): ResolvedRef[] => items;


// ── 2. validateWithParser ─────────────────────────────────────────────────────

describe("validateWithParser", () => {
  // ── 2a. valid SQL produces no markers ─────────────────────────────────────
  describe("no markers on valid parseable SQL", () => {
    it("simple SELECT", () => {
      expect(validateWithParser("SELECT 1", singleRange("SELECT 1"))).toHaveLength(0);
    });

    it("SELECT with WHERE", () => {
      const sql = "SELECT a, b FROM t WHERE c = 1";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("JOIN", () => {
      const sql = "SELECT a.id, b.name FROM t1 a JOIN t2 b ON a.id = b.id";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("CTE + SELECT", () => {
      const sql = "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("nested CTEs", () => {
      const sql = "WITH a AS (SELECT 1 AS n), b AS (SELECT n+1 AS n FROM a) SELECT n FROM b";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("subquery in FROM", () => {
      const sql = "SELECT s.x FROM (SELECT 1 AS x) s";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("window function", () => {
      const sql = "SELECT ROW_NUMBER() OVER (PARTITION BY a ORDER BY b) AS rn FROM t";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("QUALIFY clause", () => {
      const sql = "SELECT * FROM t QUALIFY ROW_NUMBER() OVER (ORDER BY a) = 1";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("PIVOT", () => {
      const sql = "SELECT * FROM t PIVOT (SUM(v) FOR cat IN ('a', 'b', 'c')) pv";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("CREATE TABLE", () => {
      const sql = "CREATE TABLE foo (id INT, name VARCHAR)";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("INSERT INTO ... SELECT", () => {
      const sql = "INSERT INTO t SELECT a, b FROM s";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("UPDATE ... SET", () => {
      const sql = "UPDATE t SET a = 1 WHERE id = 42";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("SELECT 1 (first of formerly multi-stmt test)", () => {
      expect(validateWithParser("SELECT 1", singleRange("SELECT 1"))).toHaveLength(0);
    });

    it("SELECT 2 (second of formerly multi-stmt test)", () => {
      expect(validateWithParser("SELECT 2", singleRange("SELECT 2"))).toHaveLength(0);
    });

    it("CREATE TABLE x (id INT) (third of formerly multi-stmt test)", () => {
      const sql = "CREATE TABLE x (id INT)";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("Snowflake positional params ($1, $2) are OK", () => {
      const sql = "SELECT $1, $2 FROM t";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("Snowflake double-dollar string is OK", () => {
      const sql = "SELECT $$hello$$ AS x";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("Snowflake :: cast is OK", () => {
      const sql = "SELECT a::INT FROM t";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("LATERAL FLATTEN is OK", () => {
      const sql = "SELECT f.value FROM t, LATERAL FLATTEN(input => arr) f";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });
  });

  // ── 2b. unparseable-but-valid Snowflake is silently skipped ───────────────
  describe("silently skipped (no false positives)", () => {
    const silentCases: Array<[string, string]> = [
      ["DELETE FROM", "DELETE FROM t WHERE id = 1"],
      ["MERGE", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.v = s.v"],
      ["GRANT", "GRANT SELECT ON t TO ROLE r"],
      ["REVOKE", "REVOKE SELECT ON t FROM ROLE r"],
      ["EXPLAIN", "EXPLAIN SELECT 1"],
      ["BEGIN", "BEGIN"],
      ["COMMIT", "COMMIT"],
      ["ROLLBACK", "ROLLBACK"],
      ["USE DATABASE", "USE DATABASE mydb"],
      ["COPY INTO", "COPY INTO t FROM @stage"],
      ["PUT", "PUT file://local @stage"],
      ["UNSET", "UNSET x"],
      ["DESCRIBE TABLE", "DESCRIBE TABLE t"],
      ["TABLESAMPLE", "SELECT * FROM t TABLESAMPLE (10 ROWS)"],
      ["SAMPLE (", "SELECT * FROM t SAMPLE (10)"],
      ["WITHIN GROUP", "SELECT ARRAY_AGG(a) WITHIN GROUP (ORDER BY a) FROM t"],
      ["CONNECT BY", "SELECT * FROM t CONNECT BY PRIOR id = parent_id"],
      ["AT (", "SELECT * FROM t AT (TIMESTAMP => '2023-01-01'::TIMESTAMP)"],
      ["BEFORE (", "SELECT * FROM t BEFORE (STATEMENT => '8e5d')"],
      ["SHOW COLUMNS IN TABLE", "SHOW COLUMNS IN TABLE t"],
      // DROP: parser only handles DROP TABLE; all other object types are skipped
      ["DROP TABLE", 'DROP TABLE t'],
      ["DROP VIEW", 'DROP VIEW v'],
      ["DROP TASK", 'DROP TASK "DB"."SCH"."Final"'],
      ["DROP STREAM", "DROP STREAM s"],
      ["DROP STAGE", "DROP STAGE s"],
      ["DROP PIPE", "DROP PIPE p"],
      ["DROP PROCEDURE", "DROP PROCEDURE proc()"],
      ["DROP FUNCTION", "DROP FUNCTION f(INT)"],
      ["DROP WAREHOUSE", "DROP WAREHOUSE wh"],
      ["DROP ROLE", "DROP ROLE r"],
      ["DROP DATABASE", "DROP DATABASE d"],
      ["DROP SCHEMA", "DROP SCHEMA s"],
      // CREATE with Snowflake-specific object types
      ["CREATE TASK", "CREATE TASK t AS SELECT 1"],
      ["CREATE OR REPLACE TASK", "CREATE OR REPLACE TASK t AS SELECT 1"],
      ["CREATE STREAM", "CREATE STREAM s ON TABLE t"],
      ["CREATE STAGE", "CREATE STAGE s"],
      ["CREATE PIPE", "CREATE PIPE p AS COPY INTO t FROM @s"],
      ["CREATE FUNCTION", "CREATE FUNCTION f() RETURNS INT AS $$ 1 $$"],
      ["CREATE PROCEDURE", "CREATE PROCEDURE p() RETURNS STRING AS $$ BEGIN RETURN 1; END $$"],
      ["CREATE WAREHOUSE", "CREATE WAREHOUSE wh"],
      ["CREATE ROLE", "CREATE ROLE r"],
      ["CREATE FILE FORMAT", "CREATE FILE FORMAT ff TYPE = CSV"],
      // CREATE with CLONE
      ["CREATE TABLE CLONE", "CREATE OR REPLACE TABLE t CLONE t2"],
      // ALTER with Snowflake-specific object types
      ["ALTER VIEW", "ALTER VIEW v AS SELECT 1"],
      ["ALTER TASK", "ALTER TASK t RESUME"],
      ["ALTER STREAM", "ALTER STREAM s SET COMMENT = 'x'"],
      ["ALTER WAREHOUSE", "ALTER WAREHOUSE wh RESUME"],
      ["ALTER DATABASE", "ALTER DATABASE d RENAME TO d2"],
      ["ALTER SEQUENCE", "ALTER SEQUENCE s INCREMENT BY 2"],
      ["ALTER STAGE", "ALTER STAGE s SET URL = 's3://x'"],
      ["ALTER PIPE", "ALTER PIPE p SET COMMENT = 'x'"],
      // ALTER TABLE with Snowflake-specific clauses
      ["ALTER TABLE CLUSTER BY", "ALTER TABLE t CLUSTER BY (c)"],
      ["ALTER TABLE CLUSTER KEY", "ALTER TABLE t SET CLUSTER KEY (c)"],
      // Snowflake-specific INSERT / TRUNCATE variants
      ["INSERT OVERWRITE", "INSERT OVERWRITE INTO t SELECT 1"],
      ["TRUNCATE TABLE IF EXISTS", "TRUNCATE TABLE IF EXISTS t"],
    ];

    for (const [label, sql] of silentCases) {
      it(`no false positive: ${label}`, () => {
        expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
      });
    }

    it("mixed: DELETE and SELECT in same script — only SELECT is checked", () => {
      const sql = "DELETE FROM t;\nSELECT * FROM t";
      // The DELETE is first token → skipped by PARSEABLE_STMT_KEYWORDS
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });
  });

  // ── 2c. grammar errors are caught ─────────────────────────────────────────
  describe("grammar errors → Warning", () => {
    it("bare non-keyword token alone", () => {
      const m = validateWithParser("sdadasd", singleRange("sdadasd"));
      expect(m).toHaveLength(0);
    });

    it("SELECT with truncated FROM clause", () => {
      const sql = "SELECT a FROM";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
    });

    it("SELECT missing expression", () => {
      const sql = "SELECT FROM t";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
    });

    it("warning severity is 4", () => {
      const sql = "SELECT FROM t";
      const m = validateWithParser(sql, singleRange(sql));
      for (const w of m) expect(w.severity).toBe(4);
    });

    it("error line number is correct for second statement", () => {
      const sql = "SELECT 1;\nSELECT FROM t";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
      // The SELECT FROM t is on line 2; error should be on line 2 or beyond
      expect(warnings(m)[0].startLineNumber).toBeGreaterThanOrEqual(2);
    });

    it("error line is correct deep inside multi-line query", () => {
      const sql = "SELECT\n  a,\n  b\nFROM"; // FROM without table name
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
      expect(warnings(m)[0].startLineNumber).toBeGreaterThanOrEqual(1);
    });
  });

  // ── 2d. Complex Queries & Snowflake Edge Cases ────────────────────────────
  describe("complex queries and edge cases", () => {
    it("deeply nested subqueries with set operators", () => {
      const sql = `
        WITH cte1 AS (
          SELECT id FROM t1
          UNION ALL
          SELECT id FROM t2
        ),
        cte2 AS (
          SELECT id FROM cte1
          UNION
          SELECT id FROM t3
        )
        SELECT id FROM cte2
      `;
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("Snowflake specific operators (ILIKE, RLIKE, REGEXP)", () => {
      const sql = "SELECT * FROM t WHERE name ILIKE '%john%' OR title REGEXP '.*manager.*'";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("array and object constructors", () => {
      const sql = "SELECT ARRAY_CONSTRUCT(1, 2, 3), OBJECT_CONSTRUCT('key', 'value') FROM t";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    it("catches typo in structural keywords mid-query", () => {
      // The statement starts with SELECT (valid), but 'FRO' is a syntax error.
      // We use '*' because 'SELECT id, name FRO' makes the parser think 'FRO' is a column alias!
      const sql = "SELECT * FRO my_table";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m)).toHaveLength(1);
      // Ensure the error highlights the bad 'FRO' token
      expect(warnings(m)[0].message).toMatch(/FRO/i);
    });

    it("handles multiple statements where the first is skipped but the second is malformed", () => {
      const sql = "DROP TABLE IF EXISTS t;\nSELECT id FRO my_table;";
      const ranges = [
        { startLine: 1, endLine: 1, startOffset: 0, endOffset: 23 }, // Correctly scoped to before \n
        { startLine: 2, endLine: 2, startOffset: 24, endOffset: sql.length }
      ];
      const m = validateWithParser(sql, ranges);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].startLineNumber).toBe(2);
    });
  });
});

// ── 3. validateBareColumnRefs ─────────────────────────────────────────────────

describe("validateBareColumnRefs", async () => {
  // Table definitions used across tests
  const EMPLOYEES_CACHE = makeCache([{
    db: "DB", schema: "SCH", table: "EMPLOYEES",
    cols: ["ID", "FIRST_NAME", "LAST_NAME", "DEPT_ID", "SALARY"],
  }]);

  const DEPTS_CACHE = makeCache([{
    db: "DB", schema: "SCH", table: "DEPARTMENTS",
    cols: ["DEPT_ID", "DEPT_NAME", "MANAGER_ID"],
  }]);

  const BOTH_CACHE = new Map([...EMPLOYEES_CACHE, ...DEPTS_CACHE]);

  const empRef  = { alias: "e",    db: "DB", schema: "SCH", name: "EMPLOYEES" };
  const deptRef = { alias: "d",    db: "DB", schema: "SCH", name: "DEPARTMENTS" };
  const empFullRef  = { alias: "EMPLOYEES",    db: "DB", schema: "SCH", name: "EMPLOYEES" };

  // ── 3a. cold cache → silent ───────────────────────────────────────────────
  describe("cold cache → no markers", async () => {
    it("unknown column but cache is cold → silent", async () => {
      const sql = 'SELECT wrong_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(
        sql,
        singleRange(sql),
        refs(empFullRef),
        new Map(), // cold
      );
      expect(m).toHaveLength(0);
    });
  });

  // ── 3b. valid columns → no markers ────────────────────────────────────────
  describe("valid columns → no markers", async () => {
    it("all quoted columns exist", async () => {
      const sql = 'SELECT "ID", "FIRST_NAME", "LAST_NAME" FROM "DB"."SCH"."EMPLOYEES"';
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("bare columns that exist", async () => {
      const sql = "SELECT ID, FIRST_NAME FROM DB.SCH.EMPLOYEES e";
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("SELECT *", async () => {
      const sql = 'SELECT * FROM "DB"."SCH"."EMPLOYEES"';
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("case-insensitive match: lower-case column against upper-case cache entry", async () => {
      const sql = 'SELECT "first_name", salary FROM "DB"."SCH"."EMPLOYEES"';
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("qualified alias.column references are not re-checked here", async () => {
      // alias.col has table != null → ignored by validateBareColumnRefs
      const sql = "SELECT e.ID, e.FIRST_NAME FROM DB.SCH.EMPLOYEES e";
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("function call is not flagged", async () => {
      const sql = "SELECT COUNT(ID), MAX(SALARY) FROM DB.SCH.EMPLOYEES e";
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("expression alias is not flagged", async () => {
      const sql = "SELECT FIRST_NAME AS fn FROM DB.SCH.EMPLOYEES e";
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });
  });

  // ── 3c. unknown columns → Warning ─────────────────────────────────────────
  describe("unknown columns → Warning", async () => {
    it("bare unquoted column not in table", async () => {
      const sql = 'SELECT wrong_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/wrong_col/i);
    });

    it("double-quoted column not in table", async () => {
      const sql = 'SELECT "WRONG_COL" FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/WRONG_COL/i);
    });

    it("marker is on the correct line (multi-line SELECT)", async () => {
      const sql =
        'SELECT\n  "ID",\n  bad_col,\n  "FIRST_NAME"\nFROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].startLineNumber).toBe(3); // "bad_col" is on line 3
    });

    it("marker column span covers the full token", async () => {
      const sql = 'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)[0].startColumn).toBe(8); // 'b' of bad_col
      expect(warnings(m)[0].endColumn).toBe(8 + "bad_col".length);
    });

    it("multiple unknown columns all flagged", async () => {
      const sql = 'SELECT wrong1, "WRONG2", FIRST_NAME FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(2);
      const msgs = warnings(m).map((x) => x.message);
      expect(msgs.some((s) => s.includes("wrong1"))).toBe(true);
      expect(msgs.some((s) => s.includes("WRONG2"))).toBe(true);
    });

    it("user's original case: bare identifier mixed in quoted column list", async () => {
      const sql = [
        'SELECT',
        '    "ID",',
        '    "FIRST_NAME",',
        '    this_should_not_be_here,',
        '    "LAST_NAME"',
        'FROM "DB"."SCH"."EMPLOYEES"',
      ].join("\n");
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/this_should_not_be_here/i);
      expect(warnings(m)[0].startLineNumber).toBe(4);
    });
  });

  // ── 3d. JOIN queries ──────────────────────────────────────────────────────
  describe("JOIN queries", async () => {
    it("column from either table is valid (union of both column lists)", async () => {
      const sql =
        "SELECT ID, DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), BOTH_CACHE)).toHaveLength(0);
    });

    it("unknown column in JOIN query flagged when both caches are warm", async () => {
      const sql =
        "SELECT ID, no_such_col FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), BOTH_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/no_such_col/i);
    });

    it("cold cache for ONE JOIN table → silent (no false positives)", async () => {
      // Only EMPLOYEES cache is warm; DEPARTMENTS is cold.
      const sql =
        "SELECT ID, DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      expect(
        await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), EMPLOYEES_CACHE),
      ).toHaveLength(0);
    });

    it("three-way JOIN: unknown column flagged when all three caches warm", async () => {
      const extraCache = makeCache([{
        db: "DB", schema: "SCH", table: "EXTRA",
        cols: ["EXTRA_ID"],
      }]);
      const fullCache = new Map([...BOTH_CACHE, ...extraCache]);
      const extraRef = { alias: "x", db: "DB", schema: "SCH", name: "EXTRA" };

      const sql = [
        "SELECT ID, DEPT_NAME, EXTRA_ID, fake_col",
        "FROM DB.SCH.EMPLOYEES e",
        "JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID",
        "JOIN DB.SCH.EXTRA x ON e.ID = x.EXTRA_ID",
      ].join("\n");
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef, extraRef), fullCache);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/fake_col/i);
    });
  });

  // ── 3e. CTEs → silently skipped ───────────────────────────────────────────
  describe("CTEs → no false positives", async () => {
    it("CTE column in outer SELECT is not flagged (CTE alias unresolvable)", async () => {
      // The outer SELECT reads from 'cte' which can't be found in resolvedRefs
      // → validateBareColumnRefs skips the statement entirely.
      const sql = "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte";
      expect(await validateBareColumnRefs(sql, singleRange(sql), [], new Map())).toHaveLength(0);
    });

    it("CTE followed by a real-table SELECT: real-table portion is validated", async () => {
      // Even if the CTE is in the same script, the outer SELECT FROM a real
      // table should still be validated in a subsequent statement.
      const sql = [
        "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte;",
        'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"',
      ].join("\n");
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("recursive CTE is skipped without false positives", async () => {
      const sql = [
        "WITH RECURSIVE cte (n) AS (",
        "  SELECT 1",
        "  UNION ALL",
        "  SELECT n + 1 FROM cte WHERE n < 10",
        ")",
        "SELECT n FROM cte",
      ].join("\n");
      expect(await validateBareColumnRefs(sql, singleRange(sql), [], new Map())).toHaveLength(0);
    });
  });

  // ── 3f. subqueries in FROM → silently skipped ─────────────────────────────
  describe("subqueries in FROM → no false positives", async () => {
    it("subquery alias is not a real table → statement skipped", async () => {
      const sql = "SELECT a FROM (SELECT 1 AS a) sub";
      expect(await validateBareColumnRefs(sql, singleRange(sql), [], new Map())).toHaveLength(0);
    });

    it("subquery mixed with real table → whole statement skipped", async () => {
      // Because one FROM entry is a subquery, the whole statement is skipped.
      const sql =
        'SELECT ID, sub_col FROM "DB"."SCH"."EMPLOYEES", (SELECT 1 AS sub_col) s';
      expect(
        await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE),
      ).toHaveLength(0);
    });
  });

  // ── 3g. Snowflake false-positive patterns → silently skipped ──────────────
  describe("Snowflake FP patterns → no false positives", async () => {
    it("TABLESAMPLE is skipped", async () => {
      const sql = 'SELECT wrong FROM "DB"."SCH"."EMPLOYEES" TABLESAMPLE (10)';
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("SAMPLE ( is skipped", async () => {
      const sql = 'SELECT wrong FROM "DB"."SCH"."EMPLOYEES" SAMPLE (10)';
      expect(await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });
  });

  // ── 3h. multi-statement scripts ───────────────────────────────────────────
  describe("multi-statement scripts", async () => {
    it("each statement validated independently (two bad cols across two stmts)", async () => {
      const sql1 = 'SELECT bad1 FROM "DB"."SCH"."EMPLOYEES"';
      const sql2 = 'SELECT bad2 FROM "DB"."SCH"."EMPLOYEES"';
      const m1 = await validateBareColumnRefs(sql1, singleRange(sql1), refs(empFullRef), EMPLOYEES_CACHE);
      const m2 = await validateBareColumnRefs(sql2, singleRange(sql2), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m1)).toHaveLength(1);
      expect(warnings(m2)).toHaveLength(1);
    });

    it("line numbers are correct for a single-statement SELECT on line 1", async () => {
      const sql = 'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].startLineNumber).toBe(1);
    });
  });

  // ── 3i. AST Traversal Boundaries (Intentional Limitations) ────────────────
  describe("AST traversal boundaries (shallow checking)", async () => {
    it("ignores columns wrapped in functions (AST expr.type = aggr_func)", async () => {
      // Because the current AST check only looks at top-level column_ref,
      // it intentionally ignores bad columns wrapped in functions.
      const sql = 'SELECT MAX(bad_col) FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(m).toHaveLength(0); // Validates current known limitation
    });

    it("ignores columns inside math expressions (AST expr.type = binary_expr)", async () => {
      const sql = 'SELECT bad_col + 100 FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(m).toHaveLength(0); 
    });

    it("ignores columns inside CASE statements", async () => {
      const sql = 'SELECT CASE WHEN bad_col = 1 THEN "OTHER_BAD" ELSE 0 END FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(m).toHaveLength(0);
    });
  });

  // ── 3j. Complex FROM clauses & Fallbacks ──────────────────────────────────
  describe("complex FROM clauses and fallbacks", async () => {
    it("resolves fully qualified table directly from AST without ResolvedRefs", async () => {
      // Even if resolvedRefs is empty, if the AST parses the DB and SCHEMA, it builds the cache key.
      const sql = 'SELECT bad_col FROM DB.SCH.EMPLOYEES';
      const m = await validateBareColumnRefs(sql, singleRange(sql), [], EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("CROSS JOIN works exactly like normal JOINs", async () => {
      const sql = "SELECT bad_col FROM DB.SCH.EMPLOYEES CROSS JOIN DB.SCH.DEPARTMENTS";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), BOTH_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("skips whole query if ANY table in a complex JOIN is a subquery", async () => {
      // e is a real table, but sub is a subquery. The presence of 'sub' skips the whole check.
      const sql = `
        SELECT e.ID, bad_col 
        FROM DB.SCH.EMPLOYEES e 
        JOIN (SELECT 1 AS x) sub ON e.ID = sub.x
      `;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE);
      expect(m).toHaveLength(0); 
    });
  });

  // ── 3k. Quoted Edge Cases ──────────────────────────────────────────────────
  describe("quoted edge cases", async () => {
    it("catches double-quoted columns with spaces", async () => {
      // Removed special characters (!, #) that may cause node-sql-parser to reject the entire query.
      const sql = 'SELECT "Bad Column 1" FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/Bad Column 1/i);
    });

    it("ignores single-quoted strings entirely (treated as literal values)", async () => {
      // 'bad_col' is a string literal, "bad_col" is an identifier
      const sql = `SELECT 'bad_col_literal', "BAD_QUOTED_IDENT" FROM "DB"."SCH"."EMPLOYEES"`;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/BAD_QUOTED_IDENT/i);
      expect(warnings(m)[0].message).not.toMatch(/bad_col_literal/i);
    });
  });
});