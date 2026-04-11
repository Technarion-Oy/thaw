// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

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

const warnings = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 4);
const errors = (markers: DiagMarker[]) => markers.filter((m) => m.severity === 8);

function singleRange(sql: string): StatementRange[] {
  return [{ startLine: 1, endLine: sql.split("\n").length, startOffset: 0, endOffset: sql.length }];
}

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
      expect(extractTablePath({ catalog: "WRONG_DB", db: "SCH", table: "LIVE_TABLE" })).toEqual({ 
        db: "WRONG_DB", schema: "SCH", table: "LIVE_TABLE" 
      });
    });

    it("ignores redundant properties if the parser duplicates them", () => {
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
    const parser = new SnowflakeParser();
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
    
    let ast: any;
    try {
      ast = parser.parse(sql);
    } catch (e) {
      throw new Error(`Parser crashed on valid Snowflake syntax: ${e}`);
    }

    const ft = Array.isArray(ast.ast) ? ast.ast[0].from[0] : ast.ast.from[0];
    const path = extractTablePath(ft);
    expect(path).toEqual({
      db: "WRONG_DB",
      schema: "SCH",
      table: "LIVE_TABLE"
    });
  });

  it("catches if the token locator generates invalid editor coordinates", () => {
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
    const tokens: Array<{ name: string; col: number; endCol: number }> = [];
    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    while ((match = regex.exec(sql)) !== null) {
      if (match[0].toUpperCase() === "WRONG_DB") {
        tokens.push({
          name: match[0],
          col: match.index + 1, 
          endCol: match.index + 1 + match[0].length
        });
      }
    }
    
    expect(tokens).toHaveLength(1);
    expect(tokens[0].name).toBe("WRONG_DB");
    expect(tokens[0].col).toBe(15); 
    expect(tokens[0].endCol).toBe(23); 
  });

  it("catches if an empty resolvedRefs array defaults incorrectly", async () => {
    const sql = "SELECT * FROM WRONG_DB.SCH.LIVE_TABLE";
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
      ["CREATE TABLE CLONE", "CREATE OR REPLACE TABLE t CLONE t2"],
      ["ALTER VIEW", "ALTER VIEW v AS SELECT 1"],
      ["ALTER TASK", "ALTER TASK t RESUME"],
      ["ALTER STREAM", "ALTER STREAM s SET COMMENT = 'x'"],
      ["ALTER WAREHOUSE", "ALTER WAREHOUSE wh RESUME"],
      ["ALTER DATABASE", "ALTER DATABASE d RENAME TO d2"],
      ["ALTER SEQUENCE", "ALTER SEQUENCE s INCREMENT BY 2"],
      ["ALTER STAGE", "ALTER STAGE s SET URL = 's3://x'"],
      ["ALTER PIPE", "ALTER PIPE p SET COMMENT = 'x'"],
      ["ALTER TABLE CLUSTER BY", "ALTER TABLE t CLUSTER BY (c)"],
      ["ALTER TABLE CLUSTER KEY", "ALTER TABLE t SET CLUSTER KEY (c)"],
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
    it("flags structural error: QUALIFY after ORDER BY", () => {
      const sql = "SELECT id FROM t ORDER BY id QUALIFY ROW_NUMBER() OVER(ORDER BY id) = 1";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThan(0);
      expect(warnings(m)[0].message).toMatch(/Unexpected: 'QUALIFY'/i);
    });

    it("flags missing LATERAL keyword before FLATTEN", () => {
      const sql = "SELECT f.value FROM my_table, FLATTEN(input => col) f";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m).length).toBeGreaterThan(0);
    });
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
      const sql = "SELECT * FRO my_table";
      const m = validateWithParser(sql, singleRange(sql));
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/FRO/i);
    });

    it("handles multiple statements where the first is skipped but the second is malformed", () => {
      const sql = "DROP TABLE IF EXISTS t;\nSELECT id FRO my_table;";
      const ranges = [
        { startLine: 1, endLine: 1, startOffset: 0, endOffset: 23 }, 
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
      "CREATE TRANSIENT DATABASE my_db",
      "CREATE OR REPLACE DATABASE my_db",
      "CREATE OR REPLACE TRANSIENT DATABASE IF NOT EXISTS my_db",
      "CREATE DATABASE my_db CLONE source_db",
      "CREATE DATABASE my_db CLONE source_db AT (TIMESTAMP => '2026-04-07 11:49:54'::TIMESTAMP)",
      "CREATE DATABASE my_db CLONE source_db BEFORE (STATEMENT => '8e5d')",
      "CREATE DATABASE my_db CLONE source_db IGNORE TABLES WITH INSUFFICIENT DATA RETENTION",
      "CREATE DATABASE my_db CLONE source_db IGNORE HYBRID TABLES",
      "CREATE DATABASE my_db CLONE source_db AT (OFFSET => -3600) IGNORE TABLES WITH INSUFFICIENT DATA RETENTION IGNORE HYBRID TABLES",
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 90",
      "CREATE DATABASE my_db MAX_DATA_EXTENSION_TIME_IN_DAYS = 14",
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS = 30 MAX_DATA_EXTENSION_TIME_IN_DAYS = 7",
      "CREATE DATABASE my_db EXTERNAL_VOLUME = my_ext_vol",
      "CREATE DATABASE my_db CATALOG = my_catalog",
      "CREATE DATABASE my_db ICEBERG_VERSION_DEFAULT = 1",
      "CREATE DATABASE my_db ENABLE_ICEBERG_MERGE_ON_READ = TRUE",
      "CREATE DATABASE my_db EXTERNAL_VOLUME = ext_vol CATALOG = my_cat ICEBERG_VERSION_DEFAULT = 2 ENABLE_ICEBERG_MERGE_ON_READ = FALSE",
      "CREATE DATABASE my_db REPLACE_INVALID_CHARACTERS = TRUE",
      "CREATE DATABASE my_db DEFAULT_DDL_COLLATION = 'en-ci'",
      "CREATE DATABASE my_db STORAGE_SERIALIZATION_POLICY = OPTIMIZED",
      "CREATE DATABASE my_db STORAGE_SERIALIZATION_POLICY = COMPATIBLE DEFAULT_DDL_COLLATION = 'utf8'",
      "CREATE DATABASE my_db COMMENT = 'This is a production database'",
      "CREATE DATABASE my_db CATALOG_SYNC = 'open_cat_integration'",
      "CREATE DATABASE my_db CATALOG_SYNC_NAMESPACE_MODE = NEST",
      "CREATE DATABASE my_db CATALOG_SYNC_NAMESPACE_MODE = FLATTEN CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER = '_'",
      "CREATE DATABASE my_db WITH TAG (cost_center = 'sales', env = 'prod')",
      "CREATE DATABASE my_db TAG (department = 'hr')", 
      "CREATE DATABASE my_db WITH CONTACT (owner = 'admin@example.com', security = 'sec@example.com')",
      "CREATE DATABASE my_db WITH TAG (a='b') WITH CONTACT (owner='c')",
      "CREATE DATABASE my_db OBJECT_VISIBILITY = PRIVILEGED",
      "CREATE DATABASE my_db ENABLE_DATA_COMPACTION = TRUE",
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
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2f. Incorrect CREATE DATABASE syntax ────────────────────────────────
  describe("Incorrect CREATE DATABASE syntax -> Warning", () => {
    const invalidDbQueries = [
      "CREATE DATABASE",
      "CREATE DATABASE my_db DATA_RETENTION_TIME_IN_DAYS 10",
      "CREATE DATABASE my_db EXTREME_MODE = TRUE",
      "CREATE DATABASE my_db WITH NONSENSE = 'sales'",
      "CREATE DATABASE my_db CLONE other_db AT (TIME => '2026-04-07')", 
      "CREATE DATABASE my_db CLONE source_db IGNORE EVERYTHING",
      "CREATE TRANSIENT OR REPLACE DATABASE my_db", 
      "CREATE DATABASE TRANSIENT my_db", 
    ];

    for (const sql of invalidDbQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2g. Snowflake-specific CREATE SCHEMA modifiers ────────────────────────
  describe("Snowflake-specific CREATE SCHEMA modifiers", () => {
    const validSchemaQueries = [
      "CREATE TRANSIENT SCHEMA my_sch",
      "CREATE OR REPLACE SCHEMA my_sch",
      "CREATE OR REPLACE TRANSIENT SCHEMA IF NOT EXISTS my_sch",
      "CREATE SCHEMA my_sch CLONE source_sch",
      "CREATE SCHEMA my_sch CLONE source_sch AT (TIMESTAMP => '2026-04-07 11:49:54'::TIMESTAMP)",
      "CREATE SCHEMA my_sch CLONE source_sch IGNORE TABLES WITH INSUFFICIENT DATA RETENTION",
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS",
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS DATA_RETENTION_TIME_IN_DAYS = 90",
      "CREATE SCHEMA my_sch CLASSIFICATION_PROFILE = 'my_security_profile'",
      "CREATE SCHEMA my_sch DATA_RETENTION_TIME_IN_DAYS = 30 MAX_DATA_EXTENSION_TIME_IN_DAYS = 7",
      "CREATE SCHEMA my_sch EXTERNAL_VOLUME = my_ext_vol CATALOG = my_catalog",
      "CREATE SCHEMA my_sch ENABLE_ICEBERG_MERGE_ON_READ = TRUE REPLACE_INVALID_CHARACTERS = FALSE",
      "CREATE SCHEMA my_sch DEFAULT_DDL_COLLATION = 'en-ci' STORAGE_SERIALIZATION_POLICY = OPTIMIZED",
      "CREATE SCHEMA my_sch COMMENT = 'This is a production schema'",
      "CREATE SCHEMA my_sch WITH TAG (cost_center = 'sales') WITH CONTACT (owner = 'admin')",
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
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2h. Incorrect CREATE SCHEMA syntax ──────────────────────────────────
  describe("Incorrect CREATE SCHEMA syntax -> Warning", () => {
    const invalidSchemaQueries = [
      "CREATE SCHEMA",
      "CREATE SCHEMA my_sch WITH MANAGED ACCESS = TRUE", 
      "CREATE SCHEMA my_sch CLASSIFICATION_PROFILE 10", 
      "CREATE SCHEMA my_sch EXTREME_MODE = TRUE",
      "CREATE SCHEMA my_sch WITH NONSENSE = 'sales'",
      "CREATE TRANSIENT OR REPLACE SCHEMA my_sch", 
      "CREATE SCHEMA TRANSIENT my_sch", 
    ];

    for (const sql of invalidSchemaQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2i. Snowflake-specific CREATE VIEW modifiers ──────────────────────────
  describe("Snowflake-specific CREATE VIEW modifiers", () => {
    const validViewQueries = [
      "CREATE VIEW v AS SELECT 1 FROM t",
      "CREATE OR REPLACE SECURE VIEW v AS SELECT 1 FROM t",
      "CREATE LOCAL TEMP VIEW v AS SELECT 1 FROM t",
      "CREATE GLOBAL TEMPORARY VIEW v AS SELECT 1 FROM t",
      "CREATE VOLATILE VIEW v AS SELECT 1 FROM t",
      "CREATE RECURSIVE VIEW v AS SELECT 1 FROM t",
      "CREATE OR REPLACE SECURE GLOBAL TEMPORARY RECURSIVE VIEW IF NOT EXISTS v AS SELECT 1 FROM t",
      "CREATE VIEW v (c1, c2) AS SELECT a, b FROM t",
      "CREATE VIEW v (c1 MASKING POLICY mp1, c2 PROJECTION POLICY pp1) AS SELECT a, b FROM t",
      "CREATE VIEW v (c1 WITH MASKING POLICY mp1 USING (c1, c2), c2 WITH TAG (t1='v1', t2='v2')) AS SELECT a, b FROM t",
      "CREATE VIEW v COPY GRANTS AS SELECT 1 FROM t",
      "CREATE VIEW v COMMENT = 'Test view comment' AS SELECT 1 FROM t",
      "CREATE VIEW v CHANGE_TRACKING = TRUE AS SELECT 1 FROM t",
      "CREATE VIEW v CHANGE_TRACKING = FALSE AS SELECT 1 FROM t",
      "CREATE VIEW v ROW ACCESS POLICY rap ON (c1) AS SELECT 1 FROM t",
      "CREATE VIEW v WITH ROW ACCESS POLICY rap ON (c1, c2) AS SELECT 1 FROM t",
      "CREATE VIEW v AGGREGATION POLICY ap AS SELECT 1 FROM t",
      "CREATE VIEW v WITH AGGREGATION POLICY ap ENTITY KEY (c1, c2) AS SELECT 1 FROM t",
      "CREATE VIEW v JOIN POLICY jp AS SELECT 1 FROM t",
      "CREATE VIEW v WITH JOIN POLICY jp ALLOWED JOIN KEYS (c1, c2) AS SELECT 1 FROM t",
      "CREATE VIEW v TAG (t1='v1') AS SELECT 1 FROM t",
      "CREATE VIEW v WITH TAG (t1='v1', t2='v2') AS SELECT 1 FROM t",
      "CREATE VIEW v WITH CONTACT (owner='admin@example.com', support='help@example.com') AS SELECT 1 FROM t",
      `CREATE OR REPLACE SECURE VOLATILE RECURSIVE VIEW IF NOT EXISTS mega_view (
        id MASKING POLICY my_mask,
        name WITH PROJECTION POLICY my_proj,
        email TAG (pii='true')
      )
      COPY GRANTS
      COMMENT = 'Mega View'
      CHANGE_TRACKING = TRUE
      WITH ROW ACCESS POLICY my_rap ON (id)
      WITH AGGREGATION POLICY my_agg ENTITY KEY (id)
      WITH JOIN POLICY my_join ALLOWED JOIN KEYS (id)
      WITH TAG (env='prod')
      WITH CONTACT (owner='boss')
      AS SELECT id, name, email FROM employees`
    ];

    for (const sql of validViewQueries) {
      it(`should silently accept Snowflake CREATE VIEW syntax: ${sql.slice(0, 50)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2j. Incorrect CREATE VIEW syntax ────────────────────────────────────
  describe("Incorrect CREATE VIEW syntax -> Warning", () => {
    const invalidViewQueries = [
      "CREATE VIEW", 
      "CREATE VIEW v SELECT 1", 
      "CREATE VIEW v CHANGE_TRACKING = MAYBE AS SELECT 1", 
      "CREATE VIEW v WITH TAG t1='v1' AS SELECT 1", 
      "CREATE VIEW v ROW ACCESS POLICY rap ON c1 AS SELECT 1", 
    ];

    for (const sql of invalidViewQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2k. Snowflake-specific CREATE MATERIALIZED VIEW modifiers ─────────────
  describe("Snowflake-specific CREATE MATERIALIZED VIEW modifiers", () => {
    const validMatViewQueries = [
      "CREATE MATERIALIZED VIEW mv AS SELECT 1 FROM t",
      "CREATE OR REPLACE SECURE INTERACTIVE MATERIALIZED VIEW IF NOT EXISTS mv AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv COPY GRANTS AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv (c1, c2) AS SELECT a, b FROM t",
      "CREATE MATERIALIZED VIEW mv (c1 MASKING POLICY mp1, c2 PROJECTION POLICY pp1) AS SELECT a, b FROM t",
      "CREATE MATERIALIZED VIEW mv (c1 WITH MASKING POLICY mp1 USING (c1, c2), c2 WITH TAG (t1='v1', t2='v2')) AS SELECT a, b FROM t",
      "CREATE MATERIALIZED VIEW mv COMMENT = 'Test mv comment' AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv ROW ACCESS POLICY rap ON (c1) AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv WITH ROW ACCESS POLICY rap ON (c1, c2) AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv AGGREGATION POLICY ap AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv WITH AGGREGATION POLICY ap ENTITY KEY (c1, c2) AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv CLUSTER BY (c1, c2) AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv TAG (t1='v1') AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv WITH TAG (t1='v1', t2='v2') AS SELECT 1 FROM t",
      "CREATE MATERIALIZED VIEW mv WITH CONTACT (owner='admin@example.com', support='help@example.com') AS SELECT 1 FROM t",
      `CREATE OR REPLACE SECURE INTERACTIVE MATERIALIZED VIEW IF NOT EXISTS mega_mv (
        id MASKING POLICY my_mask,
        name WITH PROJECTION POLICY my_proj,
        email TAG (pii='true')
      )
      COPY GRANTS
      COMMENT = 'Mega MV'
      WITH ROW ACCESS POLICY my_rap ON (id)
      WITH AGGREGATION POLICY my_agg ENTITY KEY (id)
      CLUSTER BY (id, name)
      WITH TAG (env='prod')
      WITH CONTACT (owner='boss')
      AS SELECT id, name, email FROM employees`
    ];

    for (const sql of validMatViewQueries) {
      it(`should silently accept Snowflake CREATE MATERIALIZED VIEW syntax: ${sql.slice(0, 50).replace(/\n/g, " ")}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2l. Incorrect CREATE MATERIALIZED VIEW syntax ────────────────────────
  describe("Incorrect CREATE MATERIALIZED VIEW syntax -> Warning", () => {
    const invalidMatViewQueries = [
      "CREATE MATERIALIZED VIEW", 
      "CREATE MATERIALIZED mv AS SELECT 1", 
      "CREATE MATERIALIZED VIEW mv SELECT 1", // Missing AS
      "CREATE MATERIALIZED VIEW mv CLUSTER BY c1 AS SELECT 1", // Missing parens for CLUSTER BY
      "CREATE MATERIALIZED VIEW mv WITH TAG t1='v1' AS SELECT 1", // Missing parens for TAG
      "CREATE MATERIALIZED VIEW mv ROW ACCESS POLICY rap ON c1 AS SELECT 1", // Missing parens for ON
    ];

    for (const sql of invalidMatViewQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2m. Snowflake-specific CREATE DYNAMIC TABLE modifiers ─────────────
  describe("Snowflake-specific CREATE DYNAMIC TABLE modifiers", () => {
    const validDynamicTableQueries = [
      "CREATE DYNAMIC TABLE dt TARGET_LAG = '1 minute' WAREHOUSE = wh AS SELECT 1 FROM t",
      "CREATE OR REPLACE DYNAMIC TABLE dt TARGET_LAG = DOWNSTREAM WAREHOUSE = wh COMMENT = 'test' AS SELECT 1 FROM t"
    ];

    for (const sql of validDynamicTableQueries) {
      it(`should silently accept Snowflake CREATE DYNAMIC TABLE syntax: ${sql.slice(0, 50).replace(/\n/g, " ")}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2n. Incorrect CREATE DYNAMIC TABLE syntax ────────────────────────
  describe("Incorrect CREATE DYNAMIC TABLE syntax -> Warning", () => {
    const invalidDynamicTableQueries = [
      "CREATE DYNAMIC TABLE dt AS SELECT 1", // Missing TARGET_LAG and WAREHOUSE
      "CREATE DYNAMIC TABLE dt TARGET_LAG = '1 minute' AS SELECT 1", // Missing WAREHOUSE
      "CREATE DYNAMIC TABLE dt WAREHOUSE = wh AS SELECT 1", // Missing TARGET_LAG
    ];

    for (const sql of invalidDynamicTableQueries) {
      it(`should flag syntax errors in: ${sql.slice(0, 40)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        // THIS WILL FAIL: Engine doesn't support DYNAMIC TABLE syntax validation yet
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2o. Snowflake-specific DROP DATABASE modifiers ────────────────────────
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
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2p. Incorrect DROP DATABASE syntax ────────────────────────────────────
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

  // ── 2q. Snowflake-specific DROP SCHEMA modifiers ────────────────────────
  describe("Snowflake-specific DROP SCHEMA modifiers", () => {
    const validDropSchQueries = [
      "DROP SCHEMA my_sch",
      "DROP SCHEMA IF EXISTS my_sch",
      "DROP SCHEMA my_sch CASCADE",
      "DROP SCHEMA my_sch RESTRICT",
      "DROP SCHEMA IF EXISTS my_sch CASCADE",
      "DROP SCHEMA IF EXISTS my_sch RESTRICT",
      "DROP SCHEMA my_db.my_sch CASCADE" // 2-part identifier
    ];

    for (const sql of validDropSchQueries) {
      it(`should silently accept valid DROP SCHEMA syntax: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2r. Incorrect DROP SCHEMA syntax ────────────────────────────────────
  describe("Incorrect DROP SCHEMA syntax -> Warning", () => {
    const invalidDropSchQueries = [
      "DROP SCHEMA", // Missing name
      "DROP SCHEMA my_sch CASCADE RESTRICT", // Cannot have both modifiers
      "DROP SCHEMA my_sch WITH CASCADE", // Invalid keyword 'WITH'
      "DROP SCHEMA IF my_sch", // Incomplete IF EXISTS
    ];

    for (const sql of invalidDropSchQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── Snowflake-specific CREATE SEQUENCE modifiers ──────────────────────────
  describe("Snowflake-specific CREATE SEQUENCE modifiers", () => {
    const validSeqQueries = [
      "CREATE SEQUENCE my_seq",
      "CREATE OR REPLACE SEQUENCE IF NOT EXISTS my_seq",
      "CREATE SEQUENCE my_seq START WITH 1",
      "CREATE SEQUENCE my_seq INCREMENT BY 2",
      "CREATE SEQUENCE my_seq START = 1 INCREMENT = 2",
      "CREATE SEQUENCE my_seq ORDER",
      "CREATE SEQUENCE my_seq NOORDER",
      "CREATE SEQUENCE my_seq COMMENT = 'My sequence'"
    ];

    for (const sql of validSeqQueries) {
      it(`should silently accept valid CREATE SEQUENCE syntax: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── Incorrect CREATE SEQUENCE syntax -> Warning ─────────────────────────
  describe("Incorrect CREATE SEQUENCE syntax -> Warning", () => {
    const invalidSeqQueries = [
      "CREATE SEQUENCE",
      "CREATE SEQUENCE my_seq START WITH 'abc'",
      "CREATE SEQUENCE my_seq ORDER NOORDER"
    ];

    for (const sql of invalidSeqQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── Snowflake-specific ALTER SEQUENCE modifiers ───────────────────────────
  describe("Snowflake-specific ALTER SEQUENCE modifiers", () => {
    const validAlterSeqQueries = [
      "ALTER SEQUENCE my_seq RENAME TO new_seq",
      "ALTER SEQUENCE IF EXISTS my_seq SET INCREMENT BY 5",
      "ALTER SEQUENCE my_seq INCREMENT = 10",
      "ALTER SEQUENCE my_seq SET ORDER",
      "ALTER SEQUENCE my_seq SET NOORDER COMMENT = 'updated'",
      "ALTER SEQUENCE my_seq UNSET COMMENT"
    ];

    for (const sql of validAlterSeqQueries) {
      it(`should silently accept valid ALTER SEQUENCE syntax: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── Incorrect ALTER SEQUENCE syntax -> Warning ──────────────────────────
  describe("Incorrect ALTER SEQUENCE syntax -> Warning", () => {
    const invalidAlterSeqQueries = [
      "ALTER SEQUENCE",
      "ALTER SEQUENCE my_seq SET NOTHING",
      "ALTER SEQUENCE my_seq UNSET INCREMENT"
    ];

    for (const sql of invalidAlterSeqQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── Snowflake-specific DROP SEQUENCE modifiers ────────────────────────────
  describe("Snowflake-specific DROP SEQUENCE modifiers", () => {
    const validDropSeqQueries = [
      "DROP SEQUENCE my_seq",
      "DROP SEQUENCE IF EXISTS my_seq",
      "DROP SEQUENCE my_seq CASCADE",
      "DROP SEQUENCE my_seq RESTRICT",
      "DROP SEQUENCE IF EXISTS my_seq CASCADE"
    ];

    for (const sql of validDropSeqQueries) {
      it(`should silently accept valid DROP SEQUENCE syntax: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── Incorrect DROP SEQUENCE syntax -> Warning ───────────────────────────
  describe("Incorrect DROP SEQUENCE syntax -> Warning", () => {
    const invalidDropSeqQueries = [
      "DROP SEQUENCE",
      "DROP SEQUENCE my_seq CASCADE RESTRICT",
      "DROP SEQUENCE my_seq WITH CASCADE"
    ];

    for (const sql of invalidDropSeqQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── 2s. Snowflake-specific CREATE TABLE modifiers ─────────────────────────
  describe("Snowflake-specific CREATE TABLE modifiers", () => {
    // Each entry: [label, sql].  All are valid Snowflake SQL → 0 warnings expected.
    const validTableQueries: Array<[string, string]> = [
      // 1.1 — basic form with 3-part qualified name and IF NOT EXISTS
      [
        "basic CREATE TABLE IF NOT EXISTS with 3-part name",
        `CREATE TABLE IF NOT EXISTS my_database.public.basic_employees (
    emp_id NUMBER,
    first_name VARCHAR,
    last_name VARCHAR
)`,
      ],

      // 1.2 — OR REPLACE + TRANSIENT modifier (currently produces a false-positive
      //        "WARNING — Unexpected: 'TRANSIENT'" in the live editor)
      [
        "CREATE OR REPLACE TRANSIENT TABLE with AUTOINCREMENT, COLLATE, DEFAULT, CHECK, and out-of-line PRIMARY KEY",
        `CREATE OR REPLACE TRANSIENT TABLE temporary_ledger (
    transaction_id NUMBER AUTOINCREMENT START 1 INCREMENT 1,
    account_code VARCHAR(10) NOT NULL COLLATE 'en-ci',
    amount NUMBER(10, 2) DEFAULT 0.00,
    created_at TIMESTAMP_LTZ DEFAULT CURRENT_TIMESTAMP(),
    status VARCHAR CHECK (status IN ('PENDING', 'CLEARED', 'FAILED')),
    CONSTRAINT pk_trans PRIMARY KEY (transaction_id)
)`,
      ],

      // 1.3 — GLOBAL TEMPORARY + out-of-line FOREIGN KEY + CLUSTER BY
      [
        "CREATE GLOBAL TEMPORARY TABLE with out-of-line FOREIGN KEY and CLUSTER BY",
        `CREATE GLOBAL TEMPORARY TABLE session_events (
    session_id VARCHAR,
    event_time TIMESTAMP_NTZ,
    event_type VARCHAR,
    user_id NUMBER,
    CONSTRAINT pk_session PRIMARY KEY (session_id, event_time),
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES basic_employees (emp_id)
)
CLUSTER BY (DATE_TRUNC('DAY', event_time), event_type)`,
      ],

      // 1.4 — VOLATILE modifier + full set of table-level options (currently produces
      //        a false-positive "WARNING — Unexpected: 'VOLATILE'" in the live editor)
      [
        "CREATE VOLATILE TABLE with MASKING POLICY, WITH TAG, ENABLE_SCHEMA_EVOLUTION, DATA_RETENTION, COPY GRANTS, ROW ACCESS POLICY, AGGREGATION POLICY, TAG",
        `CREATE VOLATILE TABLE secure_patient_data (
    ssn VARCHAR MASKING POLICY ssn_mask USING (ssn, 'US'),
    patient_name VARCHAR WITH TAG (phi_level = 'high', department = 'oncology'),
    dob DATE COMMENT 'Patient Date of Birth',
    blood_type VARCHAR
)
ENABLE_SCHEMA_EVOLUTION = TRUE
DATA_RETENTION_TIME_IN_DAYS = 90
CHANGE_TRACKING = TRUE
DEFAULT_DDL_COLLATION = 'en-ci'
COPY GRANTS
ERROR_LOGGING = TRUE
COMMENT = 'Highly sensitive patient records'
WITH ROW ACCESS POLICY patient_access_policy ON (patient_name, dob)
WITH AGGREGATION POLICY strict_agg_policy ENTITY KEY (ssn)
WITH TAG (data_classification = 'restricted')`,
      ],

      // 1.5 — FROM BACKUP SET syntax (currently produces a false-positive
      //        "WARNING — Unexpected: 'FROM'" in the live editor)
      [
        "CREATE TABLE ... FROM BACKUP SET ... IDENTIFIER ...",
        "CREATE TABLE restored_archive FROM BACKUP SET my_backup_set IDENTIFIER 'backup_id_12345'",
      ],

      // Additional valid forms from the Snowflake CREATE TABLE syntax reference
      ["CREATE LOCAL TEMP TABLE",           "CREATE LOCAL TEMP TABLE t (id INT, name VARCHAR)"],
      ["CREATE TEMPORARY TABLE",            "CREATE TEMPORARY TABLE t (id INT)"],
      ["CREATE OR REPLACE TEMP TABLE",      "CREATE OR REPLACE TEMP TABLE t (id INT)"],
      ["CREATE TABLE with DATA_RETENTION_TIME_IN_DAYS",
        "CREATE TABLE t (id INT) DATA_RETENTION_TIME_IN_DAYS = 7"],
      ["CREATE TABLE with ENABLE_SCHEMA_EVOLUTION = FALSE",
        "CREATE TABLE t (id INT) ENABLE_SCHEMA_EVOLUTION = FALSE"],
      ["CREATE TABLE with CHANGE_TRACKING and MAX_DATA_EXTENSION_TIME_IN_DAYS",
        "CREATE TABLE t (id INT) CHANGE_TRACKING = TRUE MAX_DATA_EXTENSION_TIME_IN_DAYS = 14"],
      ["CREATE TABLE with COPY GRANTS and ERROR_LOGGING",
        "CREATE TABLE t (id INT) COPY GRANTS ERROR_LOGGING = FALSE"],
      ["CREATE TABLE with COMMENT",
        "CREATE TABLE t (id INT) COMMENT = 'A table'"],
      ["CREATE TABLE with ROW ACCESS POLICY",
        "CREATE TABLE t (id INT) WITH ROW ACCESS POLICY rap ON (id)"],
      ["CREATE TABLE with AGGREGATION POLICY and ENTITY KEY",
        "CREATE TABLE t (id INT) WITH AGGREGATION POLICY agg_pol ENTITY KEY (id)"],
      ["CREATE TABLE with TAG",
        "CREATE TABLE t (id INT) WITH TAG (env = 'prod', cost_center = 'sales')"],
    ];

    for (const [label, sql] of validTableQueries) {
      it(`should silently accept Snowflake CREATE TABLE syntax: ${label}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2t. Incorrect CREATE TABLE syntax ────────────────────────────────────
  describe("Incorrect CREATE TABLE syntax -> Warning", () => {
    const invalidTableQueries = [
      // Missing name and column list
      "CREATE TABLE",
      // Wrong modifier order (TRANSIENT must precede TABLE, not after OR REPLACE in reversed order)
      "CREATE TRANSIENT OR REPLACE TABLE foo (id INT)",
      // Modifier placed after TABLE keyword instead of before it
      "CREATE TABLE TRANSIENT foo (id INT)",
      // Unrecognised table-level option
      "CREATE TABLE foo (id INT) NONEXISTENT_OPTION = TRUE",
      // Missing = sign on a numeric property
      "CREATE TABLE foo (id INT) DATA_RETENTION_TIME_IN_DAYS 10",
    ];

    for (const sql of invalidTableQueries) {
      it(`should flag syntax errors in: ${sql}`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        expect(warnings(m).length).toBeGreaterThan(0);
      });
    }
  });

  // ── Snowflake-specific ALTER TABLE constraint modifiers ───────────────────
  describe("Snowflake-specific ALTER TABLE constraint modifiers", () => {
    const validAlterQueries = [
      "ALTER TABLE existing_table ADD COLUMN new_id INT NOT NULL CONSTRAINT pk_new PRIMARY KEY DEFERRABLE RELY;",
      "ALTER TABLE existing_table ADD COLUMN new_parent_id INT FOREIGN KEY REFERENCES parent (id) ON DELETE CASCADE NOT ENFORCED;",
      "ALTER TABLE existing_table ADD COLUMN new_flag BOOLEAN CHECK (new_flag = TRUE) ENABLE NOVALIDATE;"
    ];

    for (const sql of validAlterQueries) {
      it(`should silently accept ALTER TABLE constraint syntax: ${sql.slice(0, 45)}...`, () => {
        const m = validateWithParser(sql, singleRange(sql));
        // THESE WILL FAIL: The parser currently doesn't bypass or understand these constraint properties
        expect(warnings(m)).toHaveLength(0);
      });
    }
  });

  // ── 2u. CREATE TABLE — one test per temp.sql entry ───────────────────────
  describe("CREATE TABLE — temp.sql numbered test cases", () => {
    // test_1: standard create — no modifier, expected clean
    it("test_1: standard CREATE TABLE", () => {
      const sql = "CREATE TABLE test_1 (id INT)";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    // test_2: OR REPLACE without any table-kind modifier
    it("test_2: CREATE OR REPLACE TABLE", () => {
      const sql = "CREATE OR REPLACE TABLE test_2 (id INT)";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    // test_3: IF NOT EXISTS guard
    it("test_3: CREATE TABLE IF NOT EXISTS", () => {
      const sql = "CREATE TABLE IF NOT EXISTS test_3 (id INT)";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    // test_4: false positive — "WARNING — Unexpected: 'LOCAL'"
    it("test_4: CREATE LOCAL TEMP TABLE — no false positive on LOCAL", () => {
      const sql = "CREATE LOCAL TEMP TABLE test_4 (id INT)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_5: false positive — "WARNING — Unexpected: 'GLOBAL'"
    it("test_5: CREATE GLOBAL TEMPORARY TABLE — no false positive on GLOBAL", () => {
      const sql = "CREATE GLOBAL TEMPORARY TABLE test_5 (id INT)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_6: false positive — "WARNING — Unexpected: 'VOLATILE'"
    it("test_6: CREATE VOLATILE TABLE — no false positive on VOLATILE", () => {
      const sql = "CREATE VOLATILE TABLE test_6 (id INT)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_7: false positive — "WARNING — Unexpected: 'TRANSIENT'"
    it("test_7: CREATE TRANSIENT TABLE — no false positive on TRANSIENT", () => {
      const sql = "CREATE TRANSIENT TABLE test_7 (id INT)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_8: DEFAULT string literal — expected clean
    it("test_8: CREATE TABLE with DEFAULT string literal column value", () => {
      const sql = "CREATE TABLE test_8 (status VARCHAR DEFAULT 'ACTIVE')";
      expect(validateWithParser(sql, singleRange(sql))).toHaveLength(0);
    });

    // test_9: false positive — "WARNING — Unexpected: 'AUTOINCREMENT'"
    it("test_9: CREATE TABLE with bare AUTOINCREMENT — no false positive", () => {
      const sql = "CREATE TABLE test_9 (id INT AUTOINCREMENT)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_10: false positive — "WARNING — Unexpected: 'IDENTITY'"
    it("test_10: CREATE TABLE with IDENTITY (start, step) — no false positive", () => {
      const sql = "CREATE TABLE test_10 (id INT IDENTITY (100, 5))";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_11: false positive — "WARNING — Unexpected: 'AUTOINCREMENT'"
    it("test_11: CREATE TABLE with AUTOINCREMENT START 1 INCREMENT 1 — no false positive", () => {
      const sql = "CREATE TABLE test_11 (id INT AUTOINCREMENT START 1 INCREMENT 1)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_12: false positive — "WARNING — Unexpected: 'IDENTITY'"
    it("test_12: CREATE TABLE with IDENTITY ORDER — no false positive", () => {
      const sql = "CREATE TABLE test_12 (id INT IDENTITY ORDER)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_13: false positive — "WARNING — Unexpected: 'AUTOINCREMENT'"
    it("test_13: CREATE TABLE with AUTOINCREMENT NOORDER — no false positive", () => {
      const sql = "CREATE TABLE test_13 (id INT AUTOINCREMENT NOORDER)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_15: false positive — "WARNING — Unexpected: 'CONSTRAINT'"
    it("test_15: CREATE TABLE with named inline CONSTRAINT PRIMARY KEY — no false positive", () => {
      const sql = "CREATE TABLE test_15 (id INT CONSTRAINT pk_inline PRIMARY KEY)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_17: false positive — "WARNING — Unexpected: 'CHECK'"
    it("test_17: CREATE TABLE with inline CHECK constraint — no false positive", () => {
      const sql = "CREATE TABLE test_17 (age INT CHECK (age >= 18))";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_19: false positive — "WARNING — Unexpected: 'WITH'"
    it("test_19: CREATE TABLE with column WITH MASKING POLICY USING — no false positive", () => {
      const sql = "CREATE TABLE test_19 (ssn VARCHAR WITH MASKING POLICY ssn_mask USING (ssn, 'US'))";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_20: false positive — "WARNING — Unexpected: 'PROJECTION'"
    it("test_20: CREATE TABLE with column PROJECTION POLICY — no false positive", () => {
      const sql = "CREATE TABLE test_20 (revenue NUMBER PROJECTION POLICY revenue_proj)";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_21: false positive — "WARNING — Unexpected: 'WITH'"
    it("test_21: CREATE TABLE with column WITH TAG — no false positive", () => {
      const sql = "CREATE TABLE test_21 (email VARCHAR WITH TAG (pii_level = 'high'))";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_26: false positive — "WARNING — Unexpected: 'DATA_RETENTION_TIME_IN_DAYS'"
    it("test_26: CREATE TABLE with DATA_RETENTION_TIME_IN_DAYS and MAX_DATA_EXTENSION_TIME_IN_DAYS — no false positive", () => {
      const sql = `CREATE TABLE test_26 (id INT)
    DATA_RETENTION_TIME_IN_DAYS = 90
    MAX_DATA_EXTENSION_TIME_IN_DAYS = 14`;
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_27: false positive — "WARNING — Unexpected: 'ENABLE_SCHEMA_EVOLUTION'"
    it("test_27: CREATE TABLE with ENABLE_SCHEMA_EVOLUTION and CHANGE_TRACKING — no false positive", () => {
      const sql = `CREATE TABLE test_27 (id INT)
    ENABLE_SCHEMA_EVOLUTION = TRUE
    CHANGE_TRACKING = TRUE`;
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_28: false positive — "WARNING — Unexpected: 'DEFAULT_DDL_COLLATION'"
    it("test_28: CREATE TABLE with DEFAULT_DDL_COLLATION, COPY GRANTS, ERROR_LOGGING, COPY TAGS, COMMENT, ROW_TIMESTAMP — no false positive", () => {
      const sql = `CREATE TABLE test_28 (id INT, ts TIMESTAMP)
    DEFAULT_DDL_COLLATION = 'utf8'
    COPY GRANTS
    ERROR_LOGGING = TRUE
    COPY TAGS
    COMMENT = 'Audit table'
    ROW_TIMESTAMP = TRUE`;
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_29: false positive — "WARNING — Unexpected: 'ACCESS'"
    it("test_29: CREATE TABLE with ROW ACCESS POLICY, AGGREGATION POLICY, JOIN POLICY, STORAGE LIFECYCLE POLICY, TAG, CONTACT — no false positive", () => {
      const sql = `CREATE TABLE test_29 (id INT, user_id INT, group_id INT)
    WITH ROW ACCESS POLICY row_pol ON (user_id)
    WITH AGGREGATION POLICY agg_pol ENTITY KEY (user_id, group_id)
    WITH JOIN POLICY join_pol ALLOWED JOIN KEYS (id)
    WITH STORAGE LIFECYCLE POLICY life_pol ON (id)
    WITH TAG (department = 'finance', cost_center = '123')
    WITH CONTACT (owner = 'admin@company.com', business_steward = 'manager@company.com')`;
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
    });

    // test_30: false positive noted as "WARNING — Unexpected: 'ACCESS'" in editor
    it("test_30: CREATE TABLE FROM BACKUP SET ... IDENTIFIER ... — no false positive", () => {
      const sql = "CREATE TABLE test_30 FROM BACKUP SET my_disaster_recovery_set IDENTIFIER 'backup_2026_04_10'";
      expect(warnings(validateWithParser(sql, singleRange(sql)))).toHaveLength(0);
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

    it("qualified alias.column references are checked when table restriction is lifted", async () => {
      // Because we enabled aliased column validation, e.ID and e.FIRST_NAME are valid.
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

  // ── 3l. Locally Created Tables (Script Pre-Pass) ──────────────────────────
  describe("Locally created tables (Script Pre-Pass)", async () => {
    // Helper to generate mock ranges for multi-statement scripts
    function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
      let offset = 0;
      let line = 1;
      const ranges: StatementRange[] = statements.map((stmt) => {
        const startOffset = offset;
        const endOffset = offset + stmt.length;
        const startLine = line;
        const endLine = line + stmt.split("\n").length - 1;

        offset = endOffset + 1; 
        line = endLine + 1;
        return { startLine, endLine, startOffset, endOffset };
      });
      return { sql: statements.join("\n"), ranges };
    }

    it("flags an unknown column on a locally created table", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE local_tab (amount NUMBER);",
        "SELECT amountdd FROM local_tab;"
      ]);
      
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/amountdd/i);
    });

    it("silently allows a valid column on a locally created table", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE local_tab (amount NUMBER);",
        "SELECT amount FROM local_tab;"
      ]);
      
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(0);
    });

    it("flags an unknown column but allows valid ones in a multi-column local table", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE local_tab (amount NUMBER, status VARCHAR);",
        "SELECT amount, bad_col, status FROM local_tab;"
      ]);
      
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("flags missing column when created with lowercase quotes but queried unquoted", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE local_tab ("amount" NUMBER);',
        'SELECT amount FROM local_tab;'
      ]);
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/amount/i);
    });

    it("flags missing column when created unquoted but queried with lowercase quotes", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE local_tab (amount NUMBER);',
        'SELECT "amount" FROM local_tab;'
      ]);
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/amount/i);
    });
  });

  // ── 3m. INSERT Statements (Target Column Validation) ────────────────────────
  describe("INSERT statements (Target Column Validation)", async () => {
    function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
      let offset = 0;
      let line = 1;
      const ranges: StatementRange[] = statements.map((stmt) => {
        const startOffset = offset;
        const endOffset = offset + stmt.length;
        const startLine = line;
        const endLine = line + stmt.split("\n").length - 1;

        offset = endOffset + 1; 
        line = endLine + 1;
        return { startLine, endLine, startOffset, endOffset };
      });
      return { sql: statements.join("\n"), ranges };
    }

    it("flags an unknown target column in an INSERT statement (global table)", async () => {
      const sql = 'INSERT INTO "DB"."SCH"."EMPLOYEES" (ID, FAKE_COL) SELECT 1, 2;';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/FAKE_COL/i);
    });

    it("flags an unknown target column in an INSERT statement (locally created table)", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE my_table (a varchar);",
        "INSERT INTO my_table (aaa) SELECT '1';"
      ]);
      
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/aaa/i);
    });

    it("silently allows valid target columns in an INSERT statement", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE my_table (a varchar);",
        "INSERT INTO my_table (a) SELECT '1';"
      ]);
      
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(0);
    });

    it("silently allows INSERT without a target column list", async () => {
      const sql = 'INSERT INTO "DB"."SCH"."EMPLOYEES" SELECT * FROM OTHER_TAB;';
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      expect(warnings(m)).toHaveLength(0);
    });

    it("flags missing target column in INSERT when cases mismatch due to quotes", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE my_table ("aaa" varchar);',
        'INSERT INTO my_table (aaa) SELECT \'1\';'
      ]);
      const m = await validateBareColumnRefs(sql, ranges, [], new Map());
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/aaa/i);
    });
  });

  // ── 3n. CREATE VIEW Statements (Inner SELECT Validation) ──────────────────
  describe("CREATE VIEW statements (Inner SELECT Validation)", async () => {
    it("flags unknown columns inside a CREATE VIEW statement", async () => {
      const sql = `CREATE OR REPLACE VIEW my_view AS SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"`;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("silently allows valid columns inside a CREATE VIEW statement", async () => {
      const sql = `CREATE VIEW my_view AS SELECT FIRST_NAME, LAST_NAME FROM "DB"."SCH"."EMPLOYEES"`;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      expect(warnings(m)).toHaveLength(0);
    });

    it("flags columns from JOINs inside a CREATE VIEW statement", async () => {
      const sql = `
        CREATE SECURE VIEW vw_high_value AS
        SELECT e.FIRST_NAME, d.fake_col
        FROM DB.SCH.EMPLOYEES e
        JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID
      `;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), BOTH_CACHE);
      
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/fake_col/i);
    });

    it("allows CTE columns inside a CREATE VIEW statement", async () => {
      const sql = `
        CREATE OR REPLACE VIEW vw_with_cte AS
        WITH my_cte AS (SELECT 1 AS fake_col)
        SELECT fake_col FROM my_cte;
      `;
      const m = await validateBareColumnRefs(sql, singleRange(sql), [], new Map());
      
      expect(warnings(m)).toHaveLength(0);
    });
  });

  // Mock live database cache
  const LIVE_REFS = refs({ alias: "l", db: "DB", schema: "SCH", name: "LIVE_TABLE" });

  describe("CREATE MATERIALIZED VIEW statements (Inner SELECT Validation)", async () => {
    it("flags unknown columns inside a CREATE MATERIALIZED VIEW statement", async () => {
      const sql = `CREATE OR REPLACE MATERIALIZED VIEW my_mv AS SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"`;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL: Engine doesn't convert MATERIALIZED VIEW to a parsable AST
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });

    it("flags columns from JOINs inside a CREATE MATERIALIZED VIEW statement", async () => {
      const sql = `
        CREATE SECURE INTERACTIVE MATERIALIZED VIEW vw_high_value AS
        SELECT e.FIRST_NAME, d.fake_col
        FROM DB.SCH.EMPLOYEES e
        JOIN DB.SCH.DEPARTMENTS d ON e.DEPT_ID = d.DEPT_ID
      `;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef, deptRef), BOTH_CACHE);
      
      // THIS WILL FAIL
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/fake_col/i);
    });

    it("flags missing database in a 3-part CREATE TABLE IF NOT EXISTS statement", async () => {
      const sql = "CREATE TABLE IF NOT EXISTS missing_database.public.basic_employees (id INT);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL: The engine incorrectly lets IF NOT EXISTS bypass database/schema checks
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MISSING_DATABASE' does not exist/i);
    });

    it("flags missing database in a 3-part CREATE TABLE statement", async () => {
      const sql = "CREATE TABLE my_database.public.basic_employees (id INT);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL: The engine currently ignores context checks if parts.length === 3
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MY_DATABASE' does not exist/i);
    });

    it("flags missing referenced table in an out-of-line FOREIGN KEY constraint", async () => {
      const sql = `
        CREATE TABLE session_events (
            user_id NUMBER,
            CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES missing_fk_table (id)
        );
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      // THIS WILL FAIL: The engine doesn't scan CREATE TABLE constraints for table references
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_FK_TABLE/i);
    });
  });

  describe("CREATE DYNAMIC TABLE statements (Inner SELECT Validation)", async () => {
    it("flags unknown columns inside a CREATE DYNAMIC TABLE statement", async () => {
      const sql = `CREATE DYNAMIC TABLE dt TARGET_LAG = '1 day' WAREHOUSE = wh AS SELECT bad_col FROM "DB"."SCH"."EMPLOYEES"`;
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      // THIS WILL FAIL
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_col/i);
    });
  });

  // ── CREATE TABLE statements (Constraint Column Validation) ────────────────
  describe("CREATE TABLE statements (Constraint Column Validation)", async () => {
    it("flags an unknown column in a FOREIGN KEY constraint even with CLUSTER BY present", async () => {
      const sql = `\n        CREATE GLOBAL TEMPORARY TABLE session_events (\n            user_id NUMBER,\n            CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES basic_employees (emp_id)\n        )\n        CLUSTER BY (user_id);\n      `;
      const BASIC_EMP_CACHE = makeCache([{
        db: "DB", schema: "SCH", table: "BASIC_EMPLOYEES",
        cols: ["WRONG_ID", "FIRST_NAME"],
      }]);
      const basicEmpRef = { alias: "", db: "DB", schema: "SCH", name: "BASIC_EMPLOYEES" };
      
      const m = await validateBareColumnRefs(sql, singleRange(sql), [basicEmpRef], BASIC_EMP_CACHE);
      
      // THIS WILL FAIL: The engine bails out completely because CLUSTER BY hits the false-positive regex
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/emp_id/i);
    });

    it("flags an unknown column in an out-of-line FOREIGN KEY REFERENCES constraint", async () => {
      const sql = `
        CREATE GLOBAL TEMPORARY TABLE session_events (
            user_id NUMBER,
            CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES basic_employees (emp_id)
        );
      `;
      // Mock cache for basic_employees without 'emp_id'
      const BASIC_EMP_CACHE = makeCache([{
        db: "DB", schema: "SCH", table: "BASIC_EMPLOYEES",
        cols: ["WRONG_ID", "FIRST_NAME"],
      }]);
      const basicEmpRef = { alias: "", db: "DB", schema: "SCH", name: "BASIC_EMPLOYEES" };
      
      const m = await validateBareColumnRefs(sql, singleRange(sql), [basicEmpRef], BASIC_EMP_CACHE);
      
      // THIS WILL FAIL: The engine currently doesn't parse column references inside CREATE TABLE constraints
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/emp_id/i);
    });
  });

  // ── 3o. Columns in WHERE, GROUP BY, ORDER BY, HAVING ──────────────────────
  describe("Columns in WHERE, GROUP BY, ORDER BY, and HAVING clauses", async () => {
    it("flags an unaliased unknown column in a WHERE clause", async () => {
      const sql = "SELECT ID FROM DB.SCH.EMPLOYEES WHERE bad_where = 1";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL: Engine currently ignores node.where
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_where/i);
    });

    it("flags an aliased unknown column in a WHERE clause", async () => {
      const sql = "SELECT e.ID FROM DB.SCH.EMPLOYEES e WHERE e.bad_where = 'John'";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_where/i);
    });

    it("silently allows valid aliased and unaliased columns in a WHERE clause", async () => {
      const sql = "SELECT e.ID FROM DB.SCH.EMPLOYEES e WHERE e.FIRST_NAME = 'John' AND SALARY > 1000";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empRef), EMPLOYEES_CACHE);
      
      expect(warnings(m)).toHaveLength(0);
    });

    it("flags an unknown column in a GROUP BY clause", async () => {
      const sql = "SELECT DEPT_ID, COUNT(ID) FROM DB.SCH.EMPLOYEES GROUP BY bad_group";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL: Engine currently ignores node.groupby
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_group/i);
    });

    it("flags an unknown column in a HAVING clause", async () => {
      const sql = "SELECT DEPT_ID, COUNT(ID) FROM DB.SCH.EMPLOYEES GROUP BY DEPT_ID HAVING MAX(bad_having) > 100";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL: Engine currently ignores node.having
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_having/i);
    });

    it("flags an unknown column in an ORDER BY clause", async () => {
      const sql = "SELECT ID FROM DB.SCH.EMPLOYEES ORDER BY bad_order DESC";
      const m = await validateBareColumnRefs(sql, singleRange(sql), refs(empFullRef), EMPLOYEES_CACHE);
      
      // THIS WILL FAIL: Engine currently ignores node.orderby
      expect(warnings(m)).toHaveLength(1);
      expect(warnings(m)[0].message).toMatch(/bad_order/i);
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
        'CREATE TRANSIENT TABLE IF NOT EXISTS "DB"."SCH"."Crazy Table!" (id INT);',
        'SELECT * FROM "Crazy Table!";'
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("handles case insensitivity between CREATE and SELECT (both unquoted)", async () => {
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

    it("flags missing table when created with lowercase quotes but queried unquoted", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE "my_table" (a varchar);',
        'SELECT * FROM my_table;'
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_table/i);
    });

    it("flags missing table when created unquoted but queried with lowercase quotes", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE my_table (a varchar);',
        'SELECT * FROM "my_table";'
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_table/i);
    });

    it("flags missing tables inside CTE definitions of a standard SELECT", async () => {
      const sql = `
        WITH my_cte AS (
            SELECT * FROM MISSING_CTE_TABLE
        )
        SELECT * FROM my_cte;
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_CTE_TABLE/i);
    });

    it("flags a missing table in an ALTER TABLE ADD COLUMN statement", async () => {
      const sql = `ALTER TABLE existing_table ADD COLUMN 
    new_id INT NOT NULL CONSTRAINT pk_new PRIMARY KEY DEFERRABLE RELY;`;
      
      // LIVE_REFS only contains 'LIVE_TABLE', so 'existing_table' should trigger a missing table error
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      // Expect exactly 1 fatal error (severity 8) complaining about the missing table
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].severity).toBe(8);
      expect(errors(m)[0].message).toMatch(/Table or View 'EXISTING_TABLE' does not exist|EXISTING_TABLE/i);
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

    it("flags valid global table if queried with mismatched lowercase quotes", async () => {
      // LIVE_TABLE exists as uppercase in LIVE_REFS
      const sql = 'SELECT * FROM "live_table"';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/live_table/i);
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
        SELECT * FROM -- MISSING_TABLE 
        LIVE_TABLE
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing table even if its name appears in a comment", async () => {
      const sql = `
        SELECT * FROM MISSING_TABLE -- LIVE_TABLE
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
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected/i);
    });

    it("allows 1-part CREATE SCHEMA when a database IS in the global context", async () => {
      const sql = "CREATE SCHEMA mschema WITH MANAGED ACCESS;";
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
      const sql = "CREATE SCHEMA my_db.mschema WITH MANAGED ACCESS;";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags 2-part CREATE SCHEMA when database case mismatches due to quotes", async () => {
      const sql = 'CREATE SCHEMA "my_db".mschema;';
      // Global context knows MY_DB (uppercase)
      const m = await validateTablesExist(sql, singleRange(sql), [], ["MY_DB"], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_db/i);
    });

    it("flags missing tables inside CTE definitions within a CREATE VIEW", async () => {
      const sql = `
        CREATE OR REPLACE VIEW vw_vip AS
        WITH CustomerTotals AS (
            SELECT * FROM MISSING_ORDERS_TABLE
        )
        SELECT * FROM CustomerTotals JOIN LIVE_TABLE ON a = b;
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_ORDERS_TABLE/i);
    });
  });

  describe("Context-aware DDL validation (CREATE TABLE)", () => {
    it("flags missing database in a 3-part CREATE TABLE IF NOT EXISTS statement (from snippet)", async () => {
      const sql = `CREATE TABLE IF NOT EXISTS my_database.public.basic_employees (
          empp_id NUMBER,
          first_name VARCHAR,
          last_name VARCHAR
      );`;
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MY_DATABASE' does not exist/i);
    });

    it("flags missing database context for a 2-part CREATE TABLE IF NOT EXISTS statement", async () => {
      const sql = `CREATE TABLE IF NOT EXISTS public.basic_employees (
          empp_id NUMBER,
          first_name VARCHAR,
          last_name VARCHAR
      );`;
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // 2-part identifier lacks a database. If none is selected globally, it should flag the missing DB context.
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected\. Cannot create table using schema 'PUBLIC'/i);
    });

    it("flags missing schema for a 2-part CREATE TABLE IF NOT EXISTS statement", async () => {
      const sql = `CREATE TABLE IF NOT EXISTS missing_schema.basic_employees (
          empp_id NUMBER,
          first_name VARCHAR,
          last_name VARCHAR
      );`;
      
      // Provide a known database so the DB context passes, but pass a different known schema so schema validation runs
      const m = await validateTablesExist(sql, singleRange(sql), [], ["MY_DB"], [{db: "MY_DB", name: "PUBLIC"}]);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Schema 'MISSING_SCHEMA' does not exist/i);
    });

    it("flags 1-part CREATE TABLE when no database or schema is in context", async () => {
      const sql = "CREATE TABLE my_table (a varchar);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected/i);
    });

    it("flags 1-part CREATE TABLE when database exists but no schema is in context", async () => {
      const sql = "CREATE TABLE my_table (a varchar);";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No schema selected/i);
    });

    it("flags 2-part CREATE TABLE when no database is in context", async () => {
      const sql = "CREATE TABLE my_sch.my_table (a varchar);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected/i);
    });

    it("allows 1-part CREATE TABLE if database and schema are in global context", async () => {
      const sql = "CREATE TABLE my_table (a varchar);";
      const mockRefs = [{ alias: "", db: "EXISTING_DB", schema: "EXISTING_SCH", name: "" }];
      const m = await validateTablesExist(sql, singleRange(sql), mockRefs, ["EXISTING_DB"], [{ db: "EXISTING_DB", name: "EXISTING_SCH" }]);
      expect(errors(m)).toHaveLength(0);
    });

    it("allows 2-part CREATE TABLE if database and schema are in global context", async () => {
      const sql = "CREATE TABLE existing_sch.my_table (a varchar);";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], [{ db: "EXISTING_DB", name: "EXISTING_SCH" }]);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing database in a 3-part CREATE TABLE IF NOT EXISTS statement", async () => {
      const sql = "CREATE TABLE IF NOT EXISTS missing_database.public.basic_employees (id INT);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL: The engine incorrectly lets IF NOT EXISTS bypass database/schema checks
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MISSING_DATABASE' does not exist/i);
    });

    it("flags missing database in a 3-part CREATE TABLE statement", async () => {
      const sql = "CREATE TABLE my_database.public.basic_employees (id INT);";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      
      // THIS WILL FAIL: The engine currently ignores context checks if parts.length === 3
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MY_DATABASE' does not exist/i);
    });

    it("flags missing referenced table in an out-of-line FOREIGN KEY constraint", async () => {
      const sql = `
        CREATE TABLE session_events (
            user_id NUMBER,
            CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES missing_fk_table (id)
        );
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      // THIS WILL FAIL: The engine doesn't scan CREATE TABLE constraints for table references
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_FK_TABLE/i);
    });

    it("allows 1-part CREATE TABLE if context is created earlier in the script", async () => {
      const { sql, ranges } = multiRange([
        "CREATE DATABASE local_db;",
        "CREATE SCHEMA local_sch;", 
        "CREATE TABLE my_table (a varchar);"
      ]);
      const m = await validateTablesExist(sql, ranges, [], [], []);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags 2-part CREATE TABLE when schema case mismatches due to quotes", async () => {
      const sql = 'CREATE TABLE "my_sch".my_table (a varchar);';
      // Global context knows MY_SCH (uppercase)
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], [{ db: "EXISTING_DB", name: "MY_SCH" }]);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_sch/i);
    });
  });

  describe("Context-aware DDL validation (DROP DATABASE)", () => {
    it("flags a missing database in a standard DROP DATABASE statement", async () => {
      const sql = "DROP DATABASE missing_db;";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
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

    it("flags DROP DATABASE when case mismatches due to quotes", async () => {
      const sql = 'DROP DATABASE "my_db";';
      const m = await validateTablesExist(sql, singleRange(sql), [], ["MY_DB"], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_db/i);
    });
  });

  describe("Context-aware DDL validation (DROP SCHEMA)", () => {
    it("flags 1-part DROP SCHEMA when no database is in context", async () => {
      const sql = "DROP SCHEMA missing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/No database selected|does not exist/i);
    });

    it("flags a missing schema in a 1-part DROP SCHEMA statement with global DB context", async () => {
      const sql = "DROP SCHEMA missing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Schema 'MISSING_SCH' does not exist/i);
    });

    it("allows 1-part DROP SCHEMA if the schema exists in the global context", async () => {
      const sql = "DROP SCHEMA existing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], [{ db: "EXISTING_DB", name: "EXISTING_SCH" }]);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags a missing database in a 2-part DROP SCHEMA statement", async () => {
      const sql = "DROP SCHEMA missing_db.missing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], [], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Database 'MISSING_DB' does not exist/i);
    });

    it("flags a missing schema in a 2-part DROP SCHEMA statement when DB exists", async () => {
      const sql = "DROP SCHEMA existing_db.missing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/Schema 'MISSING_SCH' does not exist/i);
    });

    it("silently allows DROP SCHEMA IF EXISTS even when the schema is missing", async () => {
      const sql = "DROP SCHEMA IF EXISTS missing_sch;";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      expect(errors(m)).toHaveLength(0);
    });

    it("allows DROP SCHEMA if it was created earlier in the script", async () => {
      const { sql, ranges } = multiRange([
        "CREATE DATABASE local_db;",
        "CREATE SCHEMA local_db.local_sch;",
        "DROP SCHEMA local_db.local_sch;"
      ]);
      const m = await validateTablesExist(sql, ranges, [], [], []);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing schema even when CASCADE or RESTRICT is appended", async () => {
      const sql1 = "DROP SCHEMA missing_sch CASCADE;";
      const m1 = await validateTablesExist(sql1, singleRange(sql1), [], ["EXISTING_DB"], []);
      expect(errors(m1)).toHaveLength(1);
      expect(errors(m1)[0].message).toMatch(/Schema 'MISSING_SCH' does not exist/i);

      const sql2 = "DROP SCHEMA missing_sch RESTRICT;";
      const m2 = await validateTablesExist(sql2, singleRange(sql2), [], ["EXISTING_DB"], []);
      expect(errors(m2)).toHaveLength(1);
      expect(errors(m2)[0].message).toMatch(/Schema 'MISSING_SCH' does not exist/i);
    });

    it("silently allows DROP SCHEMA IF EXISTS with CASCADE or RESTRICT", async () => {
      const sql = "DROP SCHEMA IF EXISTS missing_sch CASCADE;";
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], []);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags DROP SCHEMA when case mismatches due to quotes", async () => {
      const sql = 'DROP SCHEMA "my_sch";';
      const m = await validateTablesExist(sql, singleRange(sql), [], ["EXISTING_DB"], [{ db: "EXISTING_DB", name: "MY_SCH" }]);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/my_sch/i);
    });
  });

  describe("Context-aware DDL validation (CREATE VIEW Inner SELECT)", () => {
    it("flags a missing table inside a CREATE VIEW statement", async () => {
      const sql = "CREATE VIEW my_view AS SELECT * FROM MISSING_TABLE;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_TABLE/i);
    });

    it("allows a CREATE VIEW statement if the inner table exists", async () => {
      const sql = "CREATE OR REPLACE VIEW my_view AS SELECT * FROM LIVE_TABLE;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing table in a JOIN inside a CREATE VIEW statement", async () => {
      const sql = "CREATE VIEW my_view AS SELECT * FROM LIVE_TABLE JOIN NOPE_TABLE ON a=b;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/NOPE_TABLE/i);
    });

    it("allows CTEs inside the SELECT of a CREATE VIEW statement", async () => {
      const sql = `
        CREATE OR REPLACE VIEW vw_with_cte AS
        WITH my_cte AS (SELECT 1 AS id)
        SELECT * FROM my_cte JOIN LIVE_TABLE ON my_cte.id = LIVE_TABLE.id;
      `;
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("Context-aware DDL validation (CREATE MATERIALIZED VIEW Inner SELECT)", () => {
    it("flags a missing table inside a CREATE MATERIALIZED VIEW statement", async () => {
      const sql = "CREATE MATERIALIZED VIEW my_mv AS SELECT * FROM MISSING_TABLE;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      // THIS WILL FAIL
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_TABLE/i);
    });

    it("flags missing table in a JOIN inside a CREATE MATERIALIZED VIEW statement", async () => {
      const sql = "CREATE MATERIALIZED VIEW my_mv AS SELECT * FROM LIVE_TABLE JOIN NOPE_TABLE ON a=b;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      
      // THIS WILL FAIL
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/NOPE_TABLE/i);
    });
  });

  describe("Context-aware DDL validation (CREATE DYNAMIC TABLE Inner SELECT)", () => {
    it("flags a missing table inside a CREATE DYNAMIC TABLE statement", async () => {
      const sql = "CREATE DYNAMIC TABLE dt TARGET_LAG = DOWNSTREAM WAREHOUSE = wh AS SELECT * FROM MISSING_TABLE;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      // THIS WILL FAIL
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/MISSING_TABLE/i);
    });

    it("allows a CREATE DYNAMIC TABLE statement if the inner table exists", async () => {
      const sql = "CREATE DYNAMIC TABLE dt TARGET_LAG = '1 minute' WAREHOUSE = wh AS SELECT * FROM LIVE_TABLE;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      // THIS WILL FAIL until engine is updated
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("Context-aware DDL validation (UNDROP)", () => {
    it("flags missing dropped table in UNDROP TABLE", async () => {
      const sql = "UNDROP TABLE my_missing_table;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/is not available to undrop/i);
    });

    it("allows UNDROP TABLE if it was dropped earlier in the script", async () => {
      const { sql, ranges } = multiRange([
        "CREATE TABLE t1 (id int);",
        "DROP TABLE t1;",
        "UNDROP TABLE t1;"
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS);
      expect(errors(m)).toHaveLength(0);
    });

    it("allows UNDROP TABLE if it is in the global dropped list", async () => {
      const sql = "UNDROP TABLE my_dropped_table;";
      const droppedTables = [{ db: "DB", schema: "SCH", name: "MY_DROPPED_TABLE" }];
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS, ["DB"], [{ db: "DB", name: "SCH" }], false, [], [], droppedTables);
      expect(errors(m)).toHaveLength(0);
    });

    it("flags missing dropped schema in UNDROP SCHEMA", async () => {
      const sql = "UNDROP SCHEMA my_missing_schema;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS, ["DB"]);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/is not available to undrop/i);
    });

    it("flags missing dropped database in UNDROP DATABASE", async () => {
      const sql = "UNDROP DATABASE my_missing_db;";
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS);
      expect(errors(m)).toHaveLength(1);
      expect(errors(m)[0].message).toMatch(/is not available to undrop/i);
    });
  });

  describe("Context-aware DDL validation (USE statements)", () => {
    function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
      let offset = 0;
      let line = 1;
      const ranges: StatementRange[] = statements.map((stmt) => {
        const startOffset = offset;
        const endOffset = offset + stmt.length;
        const startLine = line;
        const endLine = line + stmt.split("\n").length - 1;

        offset = endOffset + 1; 
        line = endLine + 1;
        return { startLine, endLine, startOffset, endOffset };
      });
      return { sql: statements.join("\n"), ranges };
    }

    it("allows 1-part CREATE TABLE when preceded by USE SCHEMA db.schema", async () => {
      const { sql, ranges } = multiRange([
        "USE SCHEMA LINEAGE_SOURCE_DB.PUBLIC;",
        "CREATE TABLE test_1 (id INT);"
      ]);
      const m = await validateTablesExist(sql, ranges, []);
      // THIS WILL FAIL until engine supports USE SCHEMA setting DB
      expect(errors(m)).toHaveLength(0);
    });

    it("allows 1-part CREATE TABLE when preceded by bare USE db.schema", async () => {
      const { sql, ranges } = multiRange([
        "use LINEAGE_SOURCE_DB.PUBLIC;",
        "CREATE TABLE test_1 (id INT);"
      ]);
      const m = await validateTablesExist(sql, ranges, []);
      // THIS WILL FAIL until engine supports bare USE setting DB & Schema
      expect(errors(m)).toHaveLength(0);
    });
  });

  describe("QUOTED_IDENTIFIERS_IGNORE_CASE session parameter (Columns)", async () => {
    function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
      let offset = 0;
      let line = 1;
      const ranges: StatementRange[] = statements.map((stmt) => {
        const startOffset = offset;
        const endOffset = offset + stmt.length;
        const startLine = line;
        const endLine = line + stmt.split("\n").length - 1;

        offset = endOffset + 1; 
        line = endLine + 1;
        return { startLine, endLine, startOffset, endOffset };
      });
      return { sql: statements.join("\n"), ranges };
    }

    it("allows querying unquoted column when created with lowercase quotes if ignoreCase is true", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE local_tab ("amount" NUMBER);',
        'SELECT amount FROM local_tab;'
      ]);
      const m = await validateBareColumnRefs(sql, ranges, [], new Map(), true);
      
      expect(warnings(m)).toHaveLength(0); 
    });

    it("allows querying quoted lowercase column when created unquoted if ignoreCase is true", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE local_tab (amount NUMBER);',
        'SELECT "amount" FROM local_tab;'
      ]);
      const m = await validateBareColumnRefs(sql, ranges, [], new Map(), true);
      
      expect(warnings(m)).toHaveLength(0);
    });
  });

  describe("QUOTED_IDENTIFIERS_IGNORE_CASE session parameter (Tables)", () => {
    function multiRange(statements: string[]): { sql: string; ranges: StatementRange[] } {
      let offset = 0;
      let line = 1;
      const ranges: StatementRange[] = statements.map((stmt) => {
        const startOffset = offset;
        const endOffset = offset + stmt.length;
        const startLine = line;
        const endLine = line + stmt.split("\n").length - 1;

        offset = endOffset + 1; 
        line = endLine + 1;
        return { startLine, endLine, startOffset, endOffset };
      });
      return { sql: statements.join("\n"), ranges };
    }
    
    const LIVE_REFS = refs({ alias: "l", db: "DB", schema: "SCH", name: "LIVE_TABLE" });

    it("allows querying valid global table with mismatched lowercase quotes if ignoreCase is true", async () => {
      const sql = 'SELECT * FROM "live_table"';
      const m = await validateTablesExist(sql, singleRange(sql), LIVE_REFS, [], [], true);
      
      expect(errors(m)).toHaveLength(0);
    });

    it("allows querying locally created table with mismatched lowercase quotes if ignoreCase is true", async () => {
      const { sql, ranges } = multiRange([
        'CREATE TABLE my_table (a varchar);',
        'SELECT * FROM "my_table";'
      ]);
      const m = await validateTablesExist(sql, ranges, LIVE_REFS, [], [], true);
      
      expect(errors(m)).toHaveLength(0);
    });
  });
})
});
