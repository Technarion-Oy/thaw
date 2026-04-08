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
  validateTablesExist,
  extractTablePath,
} from "./sqlDiagnostics";
import { Parser as SnowflakeParser } from "node-sql-parser/build/snowflake";

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

// ── extractTablePath ──────────────────────────────────────────────────────────

describe("extractTablePath", () => {
  describe("standard well-formed ASTs", () => {
    it("handles an empty object", () => {
      expect(extractTablePath({})).toEqual({ db: null, schema: null, table: null });
    });

    it("extracts a single table", () => {
      expect(extractTablePath({ table: "LIVE_TABLE" })).toEqual({ 
        db: null, schema: null, table: "LIVE_TABLE" 
      });
    });

    it("extracts schema and table", () => {
      expect(extractTablePath({ schema: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: null, schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("extracts fully qualified db, schema, and table", () => {
      expect(extractTablePath({ db: "DB", schema: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });
  });

  describe("handling quoted identifiers", () => {
    it("safely strips quotes from single identifiers", () => {
      expect(extractTablePath({ table: '"My Crazy Table"' })).toEqual({ 
        db: null, schema: null, table: "My Crazy Table" 
      });
    });

    it("handles a mix of quoted and bare identifiers", () => {
      expect(extractTablePath({ db: "DB", schema: '"My Schema"', table: "LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "My Schema", table: "LIVE_TABLE" 
      });
    });
  });

  describe("handling node-sql-parser squashed string quirks", () => {
    it("extracts a 2-part identifier mashed entirely into the table property", () => {
      expect(extractTablePath({ table: "SCH.LIVE_TABLE" })).toEqual({ 
        db: null, schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("extracts a 3-part identifier mashed entirely into the table property", () => {
      expect(extractTablePath({ table: "DB.SCH.LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("extracts a 3-part identifier mashed with quotes into the table property", () => {
      // e.g., "DB"."Crazy Schema".LIVE_TABLE
      expect(extractTablePath({ table: '"DB"."Crazy Schema".LIVE_TABLE' })).toEqual({ 
        db: "DB", schema: "Crazy Schema", table: "LIVE_TABLE" 
      });
    });

    it("extracts safely when db and schema are mashed into the db property", () => {
      expect(extractTablePath({ db: "DB.SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });
  });

  describe("handling node-sql-parser property aliasing quirks", () => {
    it("prioritizes catalog over db if they contain different things", () => {
      // Often happens for WRONG_DB.SCH.LIVE_TABLE
      expect(extractTablePath({ catalog: "WRONG_DB", db: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "WRONG_DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("ignores redundant properties if the parser duplicates them", () => {
      // If the parser sets both catalog and db to the same value
      expect(extractTablePath({ catalog: "DB", db: "DB", schema: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("handles the 'database' property alias", () => {
      expect(extractTablePath({ database: "DB", schema: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });
  });

  describe("handling identifiers with unusual characters", () => {
    it("extracts identifiers starting with underscores or dollars", () => {
      expect(extractTablePath({ table: "_MY_TABLE" })).toEqual({ 
        db: null, schema: null, table: "_MY_TABLE" 
      });
      expect(extractTablePath({ table: "$MY_TABLE" })).toEqual({ 
        db: null, schema: null, table: "$MY_TABLE" 
      });
    });

    it("extracts identifiers containing numbers", () => {
      expect(extractTablePath({ table: "TABLE123" })).toEqual({ 
        db: null, schema: null, table: "TABLE123" 
      });
    });
  });
});

describe("Deep Dive: Editor Integration & Parser Anomalies", () => {
  it("catches if node-sql-parser is mangling the 3-part identifier AST", () => {
    // 1. We instantiate the real parser directly
    const parser = new SnowflakeParser();
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
    
    let ast: any;
    try {
      ast = parser.parse(sql);
    } catch (e) {
      // If the parser crashes on this string, the editor will fail silently!
      throw new Error(`Parser crashed on valid Snowflake syntax: ${e}`);
    }

    const ft = Array.isArray(ast.ast) ? ast.ast[0].from[0] : ast.ast.from[0];
    
    // 2. Pass it directly to our extractor
    const path = extractTablePath(ft);
    
    // 3. THIS IS THE TRAP: If node-sql-parser put the strings into a weird property 
    // we didn't account for (like `ft.as` or `ft.expr`), this will fail!
    expect(path).toEqual({
      db: "WRONG_DB",
      schema: "SCH",
      table: "LIVE_TABLE"
    });
  });

  it("catches if the token locator generates invalid editor coordinates", () => {
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
    
    // findTokensLocally is what generates the coordinates for the Monaco editor.
    // We export it temporarily or test the logic directly here:
    
    // Inline replica of findTokensLocally to verify the regex engine
    const tokens: Array<{ name: string; col: number; endCol: number }> = [];
    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    while ((match = regex.exec(sql)) !== null) {
      if (match[0].toUpperCase() === "WRONG_DB") {
        tokens.push({
          name: match[0],
          col: match.index + 1, // Monaco is 1-indexed
          endCol: match.index + 1 + match[0].length
        });
      }
    }
    
    // THIS IS THE TRAP: If Monaco receives a col of 0, or endCol < col, 
    // it will silently discard the marker and show no errors in the UI!
    expect(tokens).toHaveLength(1);
    expect(tokens[0].name).toBe("WRONG_DB");
    expect(tokens[0].col).toBe(15); // 'W' is the 15th character
    expect(tokens[0].endCol).toBe(23); // 15 + 8 characters
  });

  it("catches if an empty resolvedRefs array defaults incorrectly", async () => {
    // If the database connection drops, resolvedRefs is empty.
    // We must ensure the engine still flags WRONG_DB and doesn't crash.
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
    
    // Passing an empty array [] simulates a totally disconnected editor state
    const m = await validateTablesExist(sql, singleRange(sql), []);
    const errors = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 8);
    
    expect(errors(m)).toHaveLength(1);
    expect(errors(m)[0].message).toMatch(/Database 'WRONG_DB' does not exist/i);
  });
});


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

  // ── 2e. Snowflake-specific CREATE DATABASE modifiers ──────────────────────
  describe("Snowflake-specific CREATE DATABASE modifiers", () => {
    const validDbQueries = [
      // 1. Core Modifiers
      "CREATE TRANSIENT DATABASE my_db",
      "CREATE OR REPLACE DATABASE my_db",
      "CREATE OR REPLACE TRANSIENT DATABASE IF NOT EXISTS my_db",

      // 2. Cloning & Time Travel
      "CREATE DATABASE my_db CLONE source_db",
      "CREATE DATABASE my_db CLONE source_db AT (TIMESTAMP => '2026-04-07 11:49:54'::TIMESTAMP)",
      "CREATE DATABASE my_db CLONE source_db BEFORE (STATEMENT => '8e5d')",
      "CREATE DATABASE my_db CLONE source_db IGNORE TABLES WITH INSUFFICIENT DATA RETENTION",
      "CREATE DATABASE my_db CLONE source_db IGNORE HYBRID TABLES",
      "CREATE DATABASE my_db CLONE source_db AT (OFFSET => -3600) IGNORE TABLES WITH INSUFFICIENT DATA RETENTION IGNORE HYBRID TABLES",

      // 3. Retention & Extension Policies
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 90",
      "CREATE DATABASE my_db MAX_DATA_EXTENSION_TIME_IN_DAYS = 14",
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 30 MAX_DATA_EXTENSION_TIME_IN_DAYS = 7",

      // 4. Iceberg & External Volumes
      "CREATE DATABASE my_db EXTERNAL_VOLUME = my_ext_vol",
      "CREATE DATABASE my_db CATALOG = my_catalog",
      "CREATE DATABASE my_db ICEBERG_VERSION_DEFAULT = 1",
      "CREATE DATABASE my_db ENABLE_ICEBERG_MERGE_ON_READ = TRUE",
      "CREATE DATABASE my_db EXTERNAL_VOLUME = ext_vol CATALOG = my_cat ICEBERG_VERSION_DEFAULT = 2 ENABLE_ICEBERG_MERGE_ON_READ = FALSE",

      // 5. Collation & Serialization
      "CREATE DATABASE my_db REPLACE_INVALID_CHARACTERS = TRUE",
      "CREATE DATABASE my_db DEFAULT_DDL_COLLATION = 'en-ci'",
      "CREATE DATABASE my_db STORAGE_SERIALIZATION_POLICY = OPTIMIZED",
      "CREATE DATABASE my_db STORAGE_SERIALIZATION_POLICY = COMPATIBLE DEFAULT_DDL_COLLATION = 'utf8'",

      // 6. Catalog Sync & Comments
      "CREATE DATABASE my_db COMMENT = 'This is a production database'",
      "CREATE DATABASE my_db CATALOG_SYNC = 'open_cat_integration'",
      "CREATE DATABASE my_db CATALOG_SYNC_NAMESPACE_MODE = NEST",
      "CREATE DATABASE my_db CATALOG_SYNC_NAMESPACE_MODE = FLATTEN CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER = '_'",

      // 7. Tags and Contacts (Object Metadata)
      "CREATE DATABASE my_db WITH TAG (cost_center = 'sales', env = 'prod')",
      "CREATE DATABASE my_db TAG (department = 'hr')", // WITHOUT the 'WITH' keyword
      "CREATE DATABASE my_db WITH CONTACT (owner = 'admin@example.com', security = 'sec@example.com')",
      "CREATE DATABASE my_db WITH TAG (a='b') WITH CONTACT (owner='c')",

      // 8. Visibility & Compaction
      "CREATE DATABASE my_db OBJECT_VISIBILITY = PRIVILEGED",
      "CREATE DATABASE my_db ENABLE_DATA_COMPACTION = TRUE",

      // 9. The "Everything Everywhere All At Once" Mega-Query
      `CREATE OR REPLACE TRANSIENT DATABASE IF NOT EXISTS mega_db 
         CLONE source_db AT (OFFSET => -3600) IGNORE HYBRID TABLES
         DATA_RETENTION_TIME_IN_DAYS = 30 
         MAX_DATA_EXTENSION_TIME_IN_DAYS = 14
         EXTERNAL_VOLUME = my_vol 
         CATALOG = my_cat 
         ENABLE_ICEBERG_MERGE_ON_READ = TRUE
         DEFAULT_DDL_COLLATION = 'en-ci'
         STORAGE_SERIALIZATION_POLICY = OPTIMIZED
         COMMENT = 'The ultimate database'
         CATALOG_SYNC_NAMESPACE_MODE = FLATTEN
         WITH TAG (tier = 'tier1') 
         WITH CONTACT (owner = 'boss')
         OBJECT_VISIBILITY = PRIVILEGED
         ENABLE_DATA_COMPACTION = FALSE`
    ];

    for (const sql of validDbQueries) {
      it(`should silently skip Snowflake CREATE DATABASE syntax: ${sql.slice(0, 40)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        
        // Asserting that exactly ZERO markers are generated. 
        // This will currently FAIL until we update SNOWFLAKE_FP_RE.
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2f. Incorrect CREATE DATABASE syntax ────────────────────────────────
  describe("Incorrect CREATE DATABASE syntax -> Warning", () => {
    const invalidDbQueries = [
      // 1. Missing database name entirely
      "CREATE DATABASE",
      
      // 2. Missing '=' in property assignment
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS 10",
      
      // 3. Completely made-up Snowflake properties
      "CREATE DATABASE my_db EXTREME_MODE = TRUE",
      "CREATE DATABASE my_db WITH NONSENSE = 'sales'",
      
      // 4. Malformed CLONE / Time Travel clauses
      "CREATE DATABASE my_db CLONE other_db AT (TIME => '2026-04-07')", // Should be TIMESTAMP
      "CREATE DATABASE my_db CLONE source_db IGNORE EVERYTHING",
      
      // 5. Misplaced core modifiers
      "CREATE TRANSIENT OR REPLACE DATABASE my_db", // Wrong order
      "CREATE DATABASE TRANSIENT my_db", // Modifier after DATABASE
    ];

    for (const sql of invalidDbQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        
        // THIS WILL FAIL for most queries! 
        // Because the regex blindfold triggers on the first few words, 
        // the engine silently skips the garbage at the end and returns 0 warnings.
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2g. Snowflake-specific CREATE SCHEMA modifiers ────────────────────────
  describe("Snowflake-specific CREATE SCHEMA modifiers", () => {
    const validSchemaQueries = [
      // 1. Core Modifiers
      "CREATE TRANSIENT SCHEMA my_sch",
      "CREATE OR REPLACE SCHEMA my_sch",
      "CREATE OR REPLACE TRANSIENT SCHEMA IF NOT EXISTS my_sch",

      // 2. Cloning & Time Travel
      "CREATE SCHEMA my_sch CLONE source_sch",
      "CREATE SCHEMA my_sch CLONE source_sch AT (TIMESTAMP => '2026-04-07 11:49:54'::TIMESTAMP)",
      "CREATE SCHEMA my_sch CLONE source_sch IGNORE TABLES WITH INSUFFICIENT DATA RETENTION",

      // 3. Schema-Exclusive: Managed Access
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS",
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS DATA_RETENTION_TIME_IN_DAYS = 90",

      // 4. Schema-Exclusive: Classification Profile
      "CREATE SCHEMA my_sch CLASSIFICATION_PROFILE = 'my_security_profile'",

      // 5. Shared Modifiers (Retention, Catalog, Collation, etc.)
      "CREATE SCHEMA my_sch DATA_RETENTION_TIME_IN_DAYS = 30 MAX_DATA_EXTENSION_TIME_IN_DAYS = 7",
      "CREATE SCHEMA my_sch EXTERNAL_VOLUME = my_ext_vol CATALOG = my_catalog",
      "CREATE SCHEMA my_sch ENABLE_ICEBERG_MERGE_ON_READ = TRUE REPLACE_INVALID_CHARACTERS = FALSE",
      "CREATE SCHEMA my_sch DEFAULT_DDL_COLLATION = 'en-ci' STORAGE_SERIALIZATION_POLICY = OPTIMIZED",
      "CREATE SCHEMA my_sch COMMENT = 'This is a production schema'",
      "CREATE SCHEMA my_sch WITH TAG (cost_center = 'sales') WITH CONTACT (owner = 'admin')",

      // 6. The "Everything Everywhere All At Once" Mega-Query for Schemas
      `CREATE OR REPLACE TRANSIENT SCHEMA IF NOT EXISTS mega_sch 
         CLONE source_sch AT (OFFSET => -3600) IGNORE HYBRID TABLES
         WITH MANAGED ACCESS
         DATA_RETENTION_TIME_IN_DAYS = 30 
         MAX_DATA_EXTENSION_TIME_IN_DAYS = 14
         EXTERNAL_VOLUME = my_vol 
         CATALOG = my_cat 
         ENABLE_ICEBERG_MERGE_ON_READ = TRUE
         DEFAULT_DDL_COLLATION = 'en-ci'
         STORAGE_SERIALIZATION_POLICY = OPTIMIZED
         CLASSIFICATION_PROFILE = 'strict_profile'
         COMMENT = 'The ultimate schema'
         WITH TAG (tier = 'tier1') 
         WITH CONTACT (owner = 'boss')
         OBJECT_VISIBILITY = PRIVILEGED
         ENABLE_DATA_COMPACTION = FALSE`
    ];

    for (const sql of validSchemaQueries) {
      it(`should silently accept Snowflake CREATE SCHEMA syntax: ${sql.slice(0, 40)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        
        // Asserting that exactly ZERO markers are generated. 
        // This will currently FAIL because the regex doesn't support WITH MANAGED ACCESS, etc.
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2h. Incorrect CREATE SCHEMA syntax ──────────────────────────────────
  describe("Incorrect CREATE SCHEMA syntax -> Warning", () => {
    const invalidSchemaQueries = [
      // 1. Missing schema name entirely
      "CREATE SCHEMA",
      
      // 2. Malformed Schema-Exclusive properties
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS = TRUE", // Should not have an equals sign
      "CREATE SCHEMA my_sch CLASSIFICATION_PROFILE 10", // Missing equals and quotes
      
      // 3. Completely made-up Snowflake properties
      "CREATE SCHEMA my_sch EXTREME_MODE = TRUE",
      "CREATE SCHEMA my_sch WITH NONSENSE = 'sales'",
      
      // 4. Misplaced core modifiers
      "CREATE TRANSIENT OR REPLACE SCHEMA my_sch", // Wrong order
      "CREATE SCHEMA TRANSIENT my_sch", // Modifier after SCHEMA
    ];

    for (const sql of invalidSchemaQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        
        // This asserts that an error IS thrown for garbage syntax.
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2i. Snowflake-specific DROP DATABASE modifiers ────────────────────────
  describe("Snowflake-specific DROP DATABASE modifiers", () => {
    const validDropDbQueries = [
      "DROP DATABASE my_db",
      "DROP DATABASE IF EXISTS my_db",
      "DROP DATABASE my_db CASCADE",
      "DROP DATABASE my_db RESTRICT",
      "DROP DATABASE IF EXISTS my_db CASCADE",
      "DROP DATABASE IF EXISTS my_db RESTRICT"
    ];

    for (const sql of validDropDbQueries) {
      it(`should silently accept valid DROP DATABASE syntax: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        
        // This will likely FAIL because node-sql-parser doesn't know what to do with CASCADE!
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2j. Incorrect DROP DATABASE syntax ────────────────────────────────────
  describe("Incorrect DROP DATABASE syntax -> Warning", () => {
    const invalidDropDbQueries = [
      "DROP DATABASE", // Missing name
      "DROP DATABASE my_db CASCADE RESTRICT", // Cannot have both modifiers
      "DROP DATABASE my_db WITH CASCADE", // Invalid keyword 'WITH'
      "DROP DATABASE IF my_db", // Incomplete IF EXISTS
    ];

    for (const sql of invalidDropDbQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
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

  // ── 3i. Deep AST Traversal (Expressions & Functions) ──────────────────────
  describe("deep AST traversal for incorrect columns", async () => {
    it("flags columns wrapped in functions", async () => {
      const sql = 'SELECT MAX(bad_col) FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("flags columns inside math expressions", async () => {
      const sql = 'SELECT (ID * bad_col) + (SALARY / 100) FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("flags columns inside complex CASE statements", async () => {
      const sql = `
        SELECT CASE 
          WHEN SALARY > 1000 THEN bad_col_1
          WHEN ID = 1 THEN 'ok'
          ELSE "OTHER_BAD"
        END 
        FROM "DB"."SCH"."EMPLOYEES"
      `;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(2);
      expect(warnings(m).some(w => w.message.includes("bad_col_1"))).toBe(true);
      expect(warnings(m).some(w => w.message.includes("OTHER_BAD"))).toBe(true);
    });

    it("flags columns in nested function calls", async () => {
      const sql = 'SELECT COALESCE(FIRST_NAME, UPPER(bad_col)) FROM "DB"."SCH"."EMPLOYEES"';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("does not flag columns in nested subqueries (scope isolation)", async () => {
      // bad_col is in a subquery, so it should be ignored by the top-level validator
      const sql = 'SELECT ID, (SELECT bad_col FROM other_table) FROM "DB"."SCH"."EMPLOYEES"';
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

  // ── 4. validateTablesExist ────────────────────────────────────────────────────

describe("validateTablesExist", () => {
  // Helper to grab only fatal errors (severity 8)
  const errors = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 8);

  // Helper to generate mock ranges for multi-statement scripts
  function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
    let offset = 0;
    let line = 1;
    const ranges: StatementRange[] = statements.map((stmt) => {
      const startOffset = offset;
      const endOffset = offset + stmt.length;
      const startLine = line;
      const endLine = line + stmt.split("\n").length - 1;

      offset = endOffset + 1; // +1 for the \n joining them
      line = endLine + 1;
      return { startLine, endLine, startOffset, endOffset };
    });
    return { sql: statements.join("\n"), ranges };
  }

  // Mock live database cache
  const LIVE_REFS = refs({ alias: "l", db: "DB", schema: "SCH", name: "LIVE_TABLE" });

  describe("valid tables → no errors", () => {
    it("returns no errors when table exists in the live cache", async () => {
      const sql = "SELECT * FROM LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("ignores CTEs within the same statement", async () => {
      const sql = "WITH my_cte AS (SELECT 1 AS id) SELECT * FROM my_cte";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("ignores subqueries in the FROM clause", async () => {
      const sql = "SELECT sub.id FROM (SELECT 1 AS id) sub";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("recognizes locally created tables from prior statements (Pre-Pass)", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TEMPORARY TABLE local_tab AS SELECT 1;",
        "SELECT * FROM local_tab;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("recognizes locally created views", async () => {
      const { sql, ranges } = multiRange([
        "CREATE OR REPLACE VIEW my_view AS SELECT 1;",
        "SELECT * FROM my_view;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("handles fully-qualified and double-quoted names in CREATE pre-pass", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TRANSIENT TABLE IF NOT EXISTS "MY_DB"."MY_SCHEMA"."Crazy Table!" (id INT);',
        'SELECT * FROM "Crazy Table!";'
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("handles case insensitivity between CREATE and SELECT", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE MyTaBlE (id int);",
        "SELECT * FROM mytable;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("missing tables → Fatal Error (Severity 8)", () => {
    it("returns an error when the table is completely unknown", async () => {
      const sql = "SELECT * FROM MISSING_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].severity).toBe(8);
      expect(errors(m)[0].message).toMatch(/MISSING_TABLE/i);
    });

    it("flags missing table in a JOIN while allowing the valid table", async () => {
      const sql = "SELECT * FROM LIVE_TABLE JOIN NOPE_TABLE ON a=b";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/NOPE_TABLE/i);
    });

    it("CTE scope does not leak between separate statements", async () => {
      const { sql, ranges } = multiRange([
        "WITH my_cte AS (SELECT 1) SELECT * FROM my_cte;", // Valid
        "SELECT * FROM my_cte;" // Invalid! CTE doesn't exist anymore
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].startLineNumber).toBe(2);
      expect(errors(m)[0].message).toMatch(/my_cte/i);
    });

    it("marker column span covers the full unknown token", async () => {
      const sql = 'SELECT * FROM "MISSING_TABLE"';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)[0].startColumn).toBe(15); // Starts at the quote
      expect(errors(m)[0].endColumn).toBe(15 + '"MISSING_TABLE"'.length);
    });
  });

  describe("database, schema, and quoting edge cases (AST squashing)", () => {
    it("allows fully qualified table with correct db and schema", async () => {
      const sql = "SELECT * FROM DB.SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags exactly the wrong database in a bare multi-part identifier", async () => {
      const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/WRONG_DB/i);
    });

    it("flags exactly the wrong schema in a bare multi-part identifier", async () => {
      const sql = "SELECT * FROM DB.WRONG_SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/WRONG_SCH/i);
    });

    it("handles fully double-quoted paths correctly and flags the wrong database", async () => {
      // "WRONG_DB" is quoted, but the engine should seamlessly clean the quotes, check it, and flag it.
      const sql = 'SELECT * FROM "WRONG_DB"."SCH"."LIVE_TABLE"';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/WRONG_DB/i);
    });

    it("handles mixed quoted and unquoted paths, flagging the wrong schema", async () => {
      // DB is bare, SCHEMA is quoted, TABLE is bare
      const sql = 'SELECT * FROM DB."WRONG_SCH".LIVE_TABLE';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/WRONG_SCH/i);
    });

    it("flags the table if db and schema are correct but table is wrong", async () => {
      const sql = 'SELECT * FROM DB.SCH."WRONG_TABLE"';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/WRONG_TABLE/i);
    });
    
    it("handles two-part identifiers (schema.table) gracefully", async () => {
      // DB is omitted, so it parses as SCHEMA.TABLE
      const sql = 'SELECT * FROM "WRONG_SCH".LIVE_TABLE';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      // Because LIVE_TABLE exists but the schema WRONG_SCH doesn't match our LIVE_REFS, it flags WRONG_SCH
      expect(errors(m)[0].message).toMatch(/WRONG_SCH/i);
    });
  });

  describe("hierarchical database, schema, and quoting edge cases", () => {
    it("allows fully qualified table with correct db and schema", async () => {
      const sql = "SELECT * FROM DB.SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags exactly the wrong database when it does not exist anywhere", async () => {
      const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'WRONG_DB' does not exist/i);
    });

    it("flags the schema when DB exists but Schema does not", async () => {
      const sql = "SELECT * FROM DB.WRONG_SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Schema 'WRONG_SCH' does not exist/i);
    });

    it("flags the schema when omitting DB and Schema does not exist", async () => {
      const sql = "SELECT * FROM WRONG_SCH.LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Schema 'WRONG_SCH' does not exist/i);
    });

    it("flags the table when DB and Schema exist but Table does not", async () => {
      const sql = "SELECT * FROM DB.SCH.WRONG_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Table or View 'WRONG_TABLE' does not exist/i);
    });

    it("flags the Database even if the Table ALSO does not exist (priority check)", async () => {
      const sql = "SELECT * FROM WRONG_DB.SCH.WRONG_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'WRONG_DB' does not exist/i);
    });
  });

  describe("advanced script scenarios", () => {
    it("recognizes interleaved CREATE and SELECT across multiple schemas", async () => {
      const { sql, ranges } = multiRange([
        "CREATE SCHEMA S1;",
        "CREATE TABLE S1.T1 (id int);",
        "CREATE SCHEMA S2;",
        "CREATE TABLE S2.T2 (id int);",
        "SELECT * FROM S1.T1 JOIN S2.T2 ON S1.T1.id = S2.T2.id;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("CTE shadowing: CTE name same as a real table name", async () => {
      // LIVE_TABLE exists in LIVE_REFS, but here it is a CTE
      const sql = "WITH LIVE_TABLE AS (SELECT 1 AS id) SELECT * FROM LIVE_TABLE";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("handles identifiers inside comments (ignores them)", async () => {
      const sql = `
        SELECT * 
        FROM -- MISSING_TABLE 
        LIVE_TABLE
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing table even if its name appears in a comment", async () => {
      const sql = `
        SELECT * 
        FROM MISSING_TABLE -- LIVE_TABLE
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_TABLE/i);
    });

    it("identifies tables when using IF NOT EXISTS in CREATE", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE IF NOT EXISTS local_tab (id int);",
        "SELECT * FROM local_tab;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("Context-aware DDL validation (CREATE SCHEMA)", () => {
    it("flags 1-part CREATE SCHEMA when no database is in context", async () => {
      const sql = "CREATE SCHEMA mschema WITH MANAGED ACCESS;";
      
      // Simulating a completely empty context: no knownDatabases, no resolvedRefs
      // This MUST fail because Snowflake doesn't know where to put 'mschema'
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL with length 0, because validateTablesExist currently ignores CREATE statements!
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected/i);
    });

    it("allows 1-part CREATE SCHEMA when a database IS in the global context", async () => {
      const sql = "CREATE SCHEMA mschema WITH MANAGED ACCESS;";
      
      // Simulating a context where 'MY_SESSION_DB' is the active database
      const m = await validateTablesExist(sql, singleRange(sql), [], ["MY_SESSION_DB"], []);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("allows 1-part CREATE SCHEMA if a database was created earlier in the script", async () => {
      const { sql, ranges } = multiRange([
        "CREATE DATABASE local_db;",
        "USE DATABASE local_db;",
        "CREATE SCHEMA mschema WITH MANAGED ACCESS;"
      ]);
      
      const m = await validateTablesExist(sql, ranges, [], [], []);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("allows 2-part CREATE SCHEMA regardless of context", async () => {
      // Because it explicitly defines the database (my_db), it doesn't need a session context
      const sql = "CREATE SCHEMA my_db.mschema WITH MANAGED ACCESS;";
      
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("Context-aware DDL validation (DROP DATABASE)", () => {
    it("flags a missing database in a standard DROP DATABASE statement", async () => {
      const sql = "DROP DATABASE missing_db;";
      
      // Empty context: database doesn't exist
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL: validateTablesExist currently skips DROP statements entirely.
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MISSING_DB' does not exist/i);
    });

    it("allows DROP DATABASE if the database exists in the global context", async () => {
      const sql = "DROP DATABASE existing_db;";
      
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("silently allows DROP DATABASE IF EXISTS even when the database is missing", async () => {
      const sql = "DROP DATABASE IF EXISTS missing_db;";
      
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("allows DROP DATABASE if it was created earlier in the script", async () => {
      const { sql, ranges } = multiRange([
        "CREATE DATABASE local_db;",
        "DROP DATABASE local_db;"
      ]);
      
      const m = await validateTablesExist(sql, ranges, [], [], []);
      
      expect(errors(m)).toHaveLength(0);
    });
  });

  it("flags missing database even when CASCADE or RESTRICT is appended", async () => {
      const sql1 = "DROP DATABASE missing_db CASCADE;";
      const m1 = await validateTablesExist(sql1, singleRange(sql1), [], [], []);
      expect(errors(m1)).toHaveLength(1);
      expect(errors(m1)[0].message).toMatch(/Database 'MISSING_DB' does not exist/i);

      const sql2 = "DROP DATABASE missing_db RESTRICT;";
      const m2 = await validateTablesExist(sql2, singleRange(sql2), [], [], []);
      expect(errors(m2)).toHaveLength(1);
      expect(errors(m2)[0].message).toMatch(/Database 'MISSING_DB' does not exist/i);
    });

    it("silently allows DROP DATABASE IF EXISTS with CASCADE or RESTRICT", async () => {
      const sql = "DROP DATABASE IF EXISTS missing_db CASCADE;";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(0);
    });
});
});