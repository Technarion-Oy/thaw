/**
 * Unit tests for sqlDiagnostics.ts
 *
 * Coverage:
 *   validateSyntax          – structural tokenizer (unclosed literals, unmatched parens, post-; keyword)
 *   validateWithParser      – Snowflake PEG grammar check (per-statement, skips false-positive patterns)
 *   validateBareColumnRefs  – SELECT-list column existence (bare + quoted, CTEs, JOINs, subqueries)
 *   validateSemantics       – alias.column two-part reference validation
 */

import { describe, expect, it } from "vitest";
import {
  ColInfo,
  DiagMarker,
  ResolvedRef,
  validateSyntax,
  validateWithParser,
  validateBareColumnRefs,
  validateSemantics,
} from "./sqlDiagnostics";

// ── helpers ───────────────────────────────────────────────────────────────────

/** Return only error (severity 8) markers. */
const errors = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 8);

/** Return only warning (severity 4) markers. */
const warnings = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 4);

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

// ── 1. validateSyntax ─────────────────────────────────────────────────────────

describe("validateSyntax", () => {
  // ── 1a. clean SQL produces no markers ──────────────────────────────────────
  describe("no markers on valid SQL", () => {
    it("simple SELECT", () => {
      expect(validateSyntax("SELECT 1")).toHaveLength(0);
    });

    it("single-quoted strings with escape", () => {
      expect(validateSyntax("SELECT 'it''s fine' AS x")).toHaveLength(0);
    });

    it("double-quoted identifier", () => {
      expect(validateSyntax('SELECT "My Column" FROM t')).toHaveLength(0);
    });

    it("dollar-quoted string", () => {
      expect(validateSyntax("SELECT $$hello world$$ AS x")).toHaveLength(0);
    });

    it("named dollar-quoted string", () => {
      expect(validateSyntax("SELECT $body$some text$body$ AS x")).toHaveLength(0);
    });

    it("block comment", () => {
      expect(validateSyntax("/* comment */ SELECT 1")).toHaveLength(0);
    });

    it("line comment", () => {
      expect(validateSyntax("-- comment\nSELECT 1")).toHaveLength(0);
    });

    it("nested parens", () => {
      expect(validateSyntax("SELECT (1 + (2 * 3))")).toHaveLength(0);
    });

    it("multiple statements separated by semicolons", () => {
      expect(validateSyntax("SELECT 1;\nSELECT 2;\nSELECT 3")).toHaveLength(0);
    });

    it("CTE", () => {
      expect(
        validateSyntax("WITH cte AS (SELECT 1 AS x) SELECT x FROM cte"),
      ).toHaveLength(0);
    });

    it("all SQL_STMT_KEYWORDS are accepted after semicolon", () => {
      const stmts = [
        "SELECT 1", "WITH cte AS (SELECT 1) SELECT * FROM cte",
        "INSERT INTO t VALUES (1)", "UPDATE t SET a = 1",
        "DELETE FROM t", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.v = s.v",
        "CREATE TABLE t (id INT)", "ALTER TABLE t ADD COLUMN c INT",
        "DROP TABLE t", "TRUNCATE TABLE t", "UNDROP TABLE t",
        "GRANT SELECT ON t TO ROLE r", "REVOKE SELECT ON t FROM ROLE r",
        "USE DATABASE mydb", "SET x = 1", "UNSET x",
        "SHOW TABLES", "DESCRIBE TABLE t", "DESC t", "EXPLAIN SELECT 1",
        "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT sp1",
        "CALL my_proc()", "EXECUTE IMMEDIATE 'SELECT 1'", "RETURN",
        "COPY INTO t FROM @stage", "PUT file://local @stage",
        "GET @stage file://local", "LIST @stage", "REMOVE @stage/file",
        "DECLARE x INT DEFAULT 0", "LET x := 1",
        "FOR i IN 1..10 DO SELECT i; END FOR",
        "WHILE TRUE DO SELECT 1; END WHILE",
        "IF TRUE THEN SELECT 1; END IF",
        "CASE WHEN TRUE THEN SELECT 1; END CASE",
        "RAISE EXCEPTION 'oops'",
        "ANALYZE TABLE t",
        "COMMENT ON TABLE t IS 'desc'",
      ];
      for (const stmt of stmts) {
        // prefix each with a semicolon so the "post-; keyword" check fires
        const sql = `SELECT 0;\n${stmt}`;
        const markers = validateSyntax(sql).filter((m) => m.severity === 8);
        expect(markers, `keyword check failed for: ${stmt}`).toHaveLength(0);
      }
    });
  });

  // ── 1b. unclosed literals ──────────────────────────────────────────────────
  describe("unclosed literals → Error", () => {
    it("unclosed single-quoted string", () => {
      const m = validateSyntax("SELECT 'unterminated");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed string/i);
      expect(errors(m)[0].startLineNumber).toBe(1);
      expect(errors(m)[0].startColumn).toBe(8); // position of opening '
    });

    it("unclosed double-quoted identifier", () => {
      const m = validateSyntax('SELECT "bad ident');
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed quoted identifier/i);
    });

    it("unclosed dollar-quoted string $$", () => {
      const m = validateSyntax("SELECT $$no close");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed dollar-quoted/i);
    });

    it("unclosed named dollar-quoted string", () => {
      const m = validateSyntax("SELECT $tag$no close");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed dollar-quoted/i);
    });

    it("unclosed block comment", () => {
      const m = validateSyntax("SELECT 1 /* unclosed");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed block comment/i);
    });

    it("multi-line unclosed string reports correct opening line", () => {
      const m = validateSyntax("SELECT 1;\nSELECT 'bad");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].startLineNumber).toBe(2);
    });
  });

  // ── 1c. unmatched parentheses ──────────────────────────────────────────────
  describe("unmatched parentheses → Error", () => {
    it("extra closing paren", () => {
      const m = validateSyntax("SELECT (1 + 2))");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unmatched '\)'/i);
    });

    it("unclosed opening paren", () => {
      const m = validateSyntax("SELECT (1 + 2");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/unclosed '\('/i);
    });

    it("mismatched bracket vs paren produces two errors (mismatched ) + unclosed [)", () => {
      const m = validateSyntax("SELECT [1)");
      expect(errors(m)).toHaveLength(2);
    });

    it("multiple unclosed parens", () => {
      const m = validateSyntax("SELECT ((1");
      expect(errors(m)).toHaveLength(2);
    });

    it("paren inside string is NOT flagged", () => {
      expect(validateSyntax("SELECT '(unclosed in string'")).toHaveLength(0);
    });

    it("paren inside line comment is NOT flagged", () => {
      expect(validateSyntax("SELECT 1 -- (comment\n+ 2")).toHaveLength(0);
    });

    it("paren inside block comment is NOT flagged", () => {
      expect(validateSyntax("SELECT 1 /* ( */ + 2")).toHaveLength(0);
    });
  });

  // ── 1d. post-semicolon keyword check ──────────────────────────────────────
  describe("post-; garbage token → Error", () => {
    it("bare garbage after semicolon", () => {
      const m = validateSyntax("SELECT 1; garbage");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toContain("garbage");
    });

    it("garbage on second line after semicolon", () => {
      const m = validateSyntax("SELECT 1;\nsdadasd");
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].startLineNumber).toBe(2);
      expect(errors(m)[0].startColumn).toBe(1);
    });

    it("whitespace-only between semicolon and keyword is OK", () => {
      expect(validateSyntax("SELECT 1;   \n  SELECT 2")).toHaveLength(0);
    });

    it("garbage token inside quoted string after ; is NOT flagged", () => {
      // The semicolon resets atStmtStart but string content is opaque
      expect(validateSyntax("SELECT 1; SELECT 'garbage_inside'")).toHaveLength(0);
    });

    it("non-alpha non-whitespace after ; resets flag silently (no false positive)", () => {
      // e.g. a bare number literal — not a keyword but also not a word token
      expect(validateSyntax("SELECT 1; SELECT 2")).toHaveLength(0);
    });
  });
});

// ── 2. validateWithParser ─────────────────────────────────────────────────────

describe("validateWithParser", () => {
  // ── 2a. valid SQL produces no markers ─────────────────────────────────────
  describe("no markers on valid parseable SQL", () => {
    it("simple SELECT", () => {
      expect(validateWithParser("SELECT 1")).toHaveLength(0);
    });

    it("SELECT with WHERE", () => {
      expect(validateWithParser("SELECT a, b FROM t WHERE c = 1")).toHaveLength(0);
    });

    it("JOIN", () => {
      expect(
        validateWithParser("SELECT a.id, b.name FROM t1 a JOIN t2 b ON a.id = b.id"),
      ).toHaveLength(0);
    });

    it("CTE + SELECT", () => {
      expect(
        validateWithParser(
          "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte",
        ),
      ).toHaveLength(0);
    });

    it("nested CTEs", () => {
      expect(
        validateWithParser(
          "WITH a AS (SELECT 1 AS n), b AS (SELECT n+1 AS n FROM a) SELECT n FROM b",
        ),
      ).toHaveLength(0);
    });

    it("subquery in FROM", () => {
      expect(
        validateWithParser("SELECT s.x FROM (SELECT 1 AS x) s"),
      ).toHaveLength(0);
    });

    it("window function", () => {
      expect(
        validateWithParser(
          "SELECT ROW_NUMBER() OVER (PARTITION BY a ORDER BY b) AS rn FROM t",
        ),
      ).toHaveLength(0);
    });

    it("QUALIFY clause", () => {
      expect(
        validateWithParser(
          "SELECT * FROM t QUALIFY ROW_NUMBER() OVER (ORDER BY a) = 1",
        ),
      ).toHaveLength(0);
    });

    it("PIVOT", () => {
      expect(
        validateWithParser(
          "SELECT * FROM t PIVOT (SUM(v) FOR cat IN ('a', 'b', 'c')) pv",
        ),
      ).toHaveLength(0);
    });

    it("CREATE TABLE", () => {
      expect(
        validateWithParser("CREATE TABLE foo (id INT, name VARCHAR)"),
      ).toHaveLength(0);
    });

    it("INSERT INTO ... SELECT", () => {
      expect(
        validateWithParser("INSERT INTO t SELECT a, b FROM s"),
      ).toHaveLength(0);
    });

    it("UPDATE ... SET", () => {
      expect(
        validateWithParser("UPDATE t SET a = 1 WHERE id = 42"),
      ).toHaveLength(0);
    });

    it("multiple parseable statements separated by semicolons", () => {
      expect(
        validateWithParser("SELECT 1;\nSELECT 2;\nCREATE TABLE x (id INT)"),
      ).toHaveLength(0);
    });

    it("Snowflake positional params ($1, $2) are OK", () => {
      expect(validateWithParser("SELECT $1, $2 FROM t")).toHaveLength(0);
    });

    it("Snowflake double-dollar string is OK", () => {
      expect(validateWithParser("SELECT $$hello$$ AS x")).toHaveLength(0);
    });

    it("Snowflake :: cast is OK", () => {
      expect(validateWithParser("SELECT a::INT FROM t")).toHaveLength(0);
    });

    it("LATERAL FLATTEN is OK", () => {
      expect(
        validateWithParser(
          "SELECT f.value FROM t, LATERAL FLATTEN(input => arr) f",
        ),
      ).toHaveLength(0);
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
    ];

    for (const [label, sql] of silentCases) {
      it(`no false positive: ${label}`, () => {
        expect(validateWithParser(sql)).toHaveLength(0);
      });
    }

    it("mixed: DELETE and SELECT in same script — only SELECT is checked", () => {
      const sql = "DELETE FROM t;\nSELECT * FROM t";
      // The DELETE is skipped, the SELECT is valid → zero warnings
      expect(validateWithParser(sql)).toHaveLength(0);
    });
  });

  // ── 2c. grammar errors are caught ─────────────────────────────────────────
  describe("grammar errors → Warning", () => {
    it("bare non-keyword token alone", () => {
      const m = validateWithParser("sdadasd");
      // 'sdadasd' is not a recognisable SQL statement keyword →
      // validateSyntax would catch it, but validateWithParser skips it
      // (its firstToken is not in PARSEABLE_STMT_KEYWORDS).
      // Confirm: zero warnings from validateWithParser alone.
      expect(m).toHaveLength(0);
    });

    it("SELECT with truncated FROM clause", () => {
      const m = validateWithParser("SELECT a FROM");
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
    });

    it("SELECT missing expression", () => {
      const m = validateWithParser("SELECT FROM t");
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
    });

    it("warning severity is 4", () => {
      const m = validateWithParser("SELECT FROM t");
      for (const w of m) expect(w.severity).toBe(4);
    });

    it("error line number is correct for second statement", () => {
      const sql = "SELECT 1;\nSELECT FROM t";
      const m = validateWithParser(sql);
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
      // The SELECT FROM t is on line 2; error should be on line 2 or beyond
      expect(warnings(m)[0].startLineNumber).toBeGreaterThanOrEqual(2);
    });

    it("error line is correct deep inside multi-line query", () => {
      const sql = "SELECT\n  a,\n  b\nFROM"; // FROM without table name
      const m = validateWithParser(sql);
      expect(warnings(m).length).toBeGreaterThanOrEqual(1);
      expect(warnings(m)[0].startLineNumber).toBeGreaterThanOrEqual(1);
    });
  });
});

// ── 3. validateBareColumnRefs ─────────────────────────────────────────────────

describe("validateBareColumnRefs", () => {
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
  describe("cold cache → no markers", () => {
    it("unknown column but cache is cold → silent", () => {
      const m = validateBareColumnRefs(
        'SELECT wrong_col FROM "DB"."SCH"."EMPLOYEES"',
        refs(empFullRef),
        new Map(), // cold
      );
      expect(m).toHaveLength(0);
    });
  });

  // ── 3b. valid columns → no markers ────────────────────────────────────────
  describe("valid columns → no markers", () => {
    it("all quoted columns exist", () => {
      const sql = 'SELECT "ID", "FIRST_NAME", "LAST_NAME" FROM "DB"."SCH"."EMPLOYEES"';
      expect(validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("bare columns that exist", () => {
      const sql = "SELECT ID, FIRST_NAME FROM DB.SCH.EMPLOYEES e";
      expect(validateBareColumnRefs(sql, refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("SELECT *", () => {
      const sql = 'SELECT * FROM "DB"."SCH"."EMPLOYEES"';
      expect(validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("case-insensitive match: lower-case column against upper-case cache entry", () => {
      const sql = 'SELECT "first_name", salary FROM "DB"."SCH"."EMPLOYEES"';
      expect(validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("qualified alias.column references are not re-checked here", () => {
      // alias.col has table != null → ignored by validateBareColumnRefs
      const sql = "SELECT e.ID, e.FIRST_NAME FROM DB.SCH.EMPLOYEES e";
      expect(validateBareColumnRefs(sql, refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("function call is not flagged", () => {
      const sql = "SELECT COUNT(ID), MAX(SALARY) FROM DB.SCH.EMPLOYEES e";
      expect(validateBareColumnRefs(sql, refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("expression alias is not flagged", () => {
      const sql = "SELECT FIRST_NAME AS fn FROM DB.SCH.EMPLOYEES e";
      expect(validateBareColumnRefs(sql, refs(empRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });
  });

  // ── 3c. unknown columns → Warning ─────────────────────────────────────────
  describe("unknown columns → Warning", () => {
    it("bare unquoted column not in table", () => {
      const sql = 'SELECT wrong_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/wrong_col/i);
    });

    it("double-quoted column not in table", () => {
      const sql = 'SELECT "WRONG_COL" FROM "DB"."SCH"."EMPLOYEES"';
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/WRONG_COL/i);
    });

    it("marker is on the correct line (multi-line SELECT)", () => {
      const sql =
        'SELECT\n  "ID",\n  bad_col,\n  "FIRST_NAME"\nFROM "DB"."SCH"."EMPLOYEES"';
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].startLineNumber).toBe(3); // "bad_col" is on line 3
    });

    it("marker column span covers the full token", () => {
      const sql = 'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"';
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)[0].startColumn).toBe(8); // 'b' of bad_col
      expect(warnings(m)[0].endColumn).toBe(8 + "bad_col".length);
    });

    it("multiple unknown columns all flagged", () => {
      const sql = 'SELECT wrong1, "WRONG2", FIRST_NAME FROM "DB"."SCH"."EMPLOYEES"';
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(2);
      const msgs = warnings(m).map((x) => x.message);
      expect(msgs.some((s) => s.includes("wrong1"))).toBe(true);
      expect(msgs.some((s) => s.includes("WRONG2"))).toBe(true);
    });

    it("user's original case: bare identifier mixed in quoted column list", () => {
      const sql = [
        'SELECT',
        '    "ID",',
        '    "FIRST_NAME",',
        '    this_should_not_be_here,',
        '    "LAST_NAME"',
        'FROM "DB"."SCH"."EMPLOYEES"',
      ].join("\n");
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/this_should_not_be_here/i);
      expect(warnings(m)[0].startLineNumber).toBe(4);
    });
  });

  // ── 3d. JOIN queries ──────────────────────────────────────────────────────
  describe("JOIN queries", () => {
    it("column from either table is valid (union of both column lists)", () => {
      const sql =
        "SELECT ID, DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      expect(validateBareColumnRefs(sql, refs(empRef, deptRef), BOTH_CACHE)).toHaveLength(0);
    });

    it("unknown column in JOIN query flagged when both caches are warm", () => {
      const sql =
        "SELECT ID, no_such_col FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      const m = validateBareColumnRefs(sql, refs(empRef, deptRef), BOTH_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/no_such_col/i);
    });

    it("cold cache for ONE JOIN table → silent (no false positives)", () => {
      // Only EMPLOYEES cache is warm; DEPARTMENTS is cold.
      const sql =
        "SELECT ID, DEPT_NAME FROM DB.SCH.EMPLOYEES e JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID";
      expect(
        validateBareColumnRefs(sql, refs(empRef, deptRef), EMPLOYEES_CACHE),
      ).toHaveLength(0);
    });

    it("three-way JOIN: unknown column flagged when all three caches warm", () => {
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
      const m = validateBareColumnRefs(sql, refs(empRef, deptRef, extraRef), fullCache);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/fake_col/i);
    });
  });

  // ── 3e. CTEs → silently skipped ───────────────────────────────────────────
  describe("CTEs → no false positives", () => {
    it("CTE column in outer SELECT is not flagged (CTE alias unresolvable)", () => {
      // The outer SELECT reads from 'cte' which can't be found in resolvedRefs
      // → validateBareColumnRefs skips the statement entirely.
      const sql = "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte";
      expect(validateBareColumnRefs(sql, [], new Map())).toHaveLength(0);
    });

    it("CTE followed by a real-table SELECT: real-table portion is validated", () => {
      // Even if the CTE is in the same script, the outer SELECT FROM a real
      // table should still be validated in a subsequent statement.
      const sql = [
        "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte;",
        'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"',
      ].join("\n");
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("recursive CTE is skipped without false positives", () => {
      const sql = [
        "WITH RECURSIVE cte (n) AS (",
        "  SELECT 1",
        "  UNION ALL",
        "  SELECT n + 1 FROM cte WHERE n < 10",
        ")",
        "SELECT n FROM cte",
      ].join("\n");
      expect(validateBareColumnRefs(sql, [], new Map())).toHaveLength(0);
    });
  });

  // ── 3f. subqueries in FROM → silently skipped ─────────────────────────────
  describe("subqueries in FROM → no false positives", () => {
    it("subquery alias is not a real table → statement skipped", () => {
      const sql = "SELECT a FROM (SELECT 1 AS a) sub";
      expect(validateBareColumnRefs(sql, [], new Map())).toHaveLength(0);
    });

    it("subquery mixed with real table → whole statement skipped", () => {
      // Because one FROM entry is a subquery, the whole statement is skipped.
      const sql =
        'SELECT ID, sub_col FROM "DB"."SCH"."EMPLOYEES", (SELECT 1 AS sub_col) s';
      expect(
        validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE),
      ).toHaveLength(0);
    });
  });

  // ── 3g. Snowflake false-positive patterns → silently skipped ──────────────
  describe("Snowflake FP patterns → no false positives", () => {
    it("TABLESAMPLE is skipped", () => {
      const sql = 'SELECT wrong FROM "DB"."SCH"."EMPLOYEES" TABLESAMPLE (10)';
      expect(validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });

    it("SAMPLE ( is skipped", () => {
      const sql = 'SELECT wrong FROM "DB"."SCH"."EMPLOYEES" SAMPLE (10)';
      expect(validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE)).toHaveLength(0);
    });
  });

  // ── 3h. multi-statement scripts ───────────────────────────────────────────
  describe("multi-statement scripts", () => {
    it("each statement is validated independently", () => {
      const sql = [
        'SELECT bad1 FROM "DB"."SCH"."EMPLOYEES";',
        'SELECT bad2 FROM "DB"."SCH"."EMPLOYEES"',
      ].join("\n");
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(2);
    });

    it("line numbers are correct across semicolons", () => {
      const sql = [
        'SELECT ID FROM "DB"."SCH"."EMPLOYEES";',  // line 1 — valid
        'SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"', // line 2
      ].join("\n");
      const m = validateBareColumnRefs(sql, refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].startLineNumber).toBe(2);
    });
  });
});

// ── 4. validateSemantics ──────────────────────────────────────────────────────

describe("validateSemantics", () => {
  const cache = makeCache([{
    db: "DB", schema: "SCH", table: "EMPLOYEES",
    cols: ["ID", "FIRST_NAME", "LAST_NAME"],
  }]);

  const resolvedList = refs({
    alias: "e", db: "DB", schema: "SCH", name: "EMPLOYEES",
  });

  // ── 4a. no markers on valid references ────────────────────────────────────
  describe("valid alias.column → no markers", () => {
    it("known column", () => {
      expect(
        validateSemantics("SELECT e.ID FROM t e", resolvedList, cache),
      ).toHaveLength(0);
    });

    it("case-insensitive match", () => {
      expect(
        validateSemantics("SELECT e.first_name FROM t e", resolvedList, cache),
      ).toHaveLength(0);
    });

    it("three-part reference is NOT checked (db.schema.table)", () => {
      expect(
        validateSemantics("SELECT DB.SCH.EMPLOYEES FROM t", resolvedList, cache),
      ).toHaveLength(0);
    });

    it("cold cache → silent", () => {
      expect(
        validateSemantics("SELECT e.bad FROM t e", resolvedList, new Map()),
      ).toHaveLength(0);
    });
  });

  // ── 4b. unknown column → Warning ──────────────────────────────────────────
  describe("unknown alias.column → Warning", () => {
    it("unknown column flagged", () => {
      const m = validateSemantics("SELECT e.nonexistent FROM t e", resolvedList, cache);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/nonexistent/i);
      expect(warnings(m)[0].severity).toBe(4);
    });

    it("marker is on the column token, not the alias", () => {
      const sql = "SELECT e.bad FROM t e";
      const m = validateSemantics(sql, resolvedList, cache);
      expect(warnings(m)[0].startColumn).toBe(10); // 'b' of 'bad'
    });

    it("multiple unknown columns all flagged", () => {
      const sql = "SELECT e.bad1, e.ID, e.bad2 FROM t e";
      const m = validateSemantics(sql, resolvedList, cache);
      expect(warnings(m)).toHaveLength(2);
    });

    it("alias inside string is NOT flagged", () => {
      const sql = "SELECT 'e.bad' FROM t e";
      expect(validateSemantics(sql, resolvedList, cache)).toHaveLength(0);
    });

    it("alias inside comment is NOT flagged", () => {
      const sql = "SELECT e.ID -- e.bad\nFROM t e";
      expect(validateSemantics(sql, resolvedList, cache)).toHaveLength(0);
    });

    it("alias inside block comment is NOT flagged", () => {
      const sql = "SELECT e.ID /* e.bad */ FROM t e";
      expect(validateSemantics(sql, resolvedList, cache)).toHaveLength(0);
    });

    it("unknown column in JOIN query", () => {
      const cache2 = makeCache([
        { db: "DB", schema: "SCH", table: "EMPLOYEES",   cols: ["ID", "DEPT_ID"] },
        { db: "DB", schema: "SCH", table: "DEPARTMENTS", cols: ["DEPT_ID", "DEPT_NAME"] },
      ]);
      const refList = refs(
        { alias: "e", db: "DB", schema: "SCH", name: "EMPLOYEES" },
        { alias: "d", db: "DB", schema: "SCH", name: "DEPARTMENTS" },
      );
      const sql = "SELECT e.ID, d.DEPT_NAME, e.no_col FROM t e JOIN t2 d ON e.ID = d.DEPT_ID";
      const m = validateSemantics(sql, refList, cache2);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/no_col/i);
    });
  });
});
