// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { provisionalTitle, tabDisplayTitle } from "./tabTitle";
import type { Tab } from "../../store/queryStore";

describe("provisionalTitle", () => {
  it("returns null when there is nothing to describe yet", () => {
    // An empty (or comment-only) scratch tab has to keep its counter title.
    expect(provisionalTitle("")).toBeNull();
    expect(provisionalTitle("   \n\t ")).toBeNull();
    expect(provisionalTitle("-- just a note\n")).toBeNull();
    expect(provisionalTitle("/* header\n   block */")).toBeNull();
  });

  it.each<[string, string]>([
    // Queries name what they read from, not the projection.
    ["select * from orders", "SELECT · ORDERS"],
    ["SELECT id FROM analytics.public.customers c JOIN o ON …", "SELECT · CUSTOMERS"],
    ["SELECT 1 + 1", "SELECT"],
    // A table function is not an object name — better no object than "TABLE".
    ["SELECT * FROM TABLE(FLATTEN(input => raw)) f", "SELECT"],
    ["SELECT (SELECT max(id) FROM audit) FROM orders", "SELECT · AUDIT"],
    ["WITH recent AS (SELECT * FROM orders) SELECT * FROM recent", "WITH · RECENT"],
    // DML names the write target.
    ["INSERT INTO staging.orders SELECT * FROM raw", "INSERT · ORDERS"],
    ["insert overwrite into daily_totals values (1)", "INSERT · DAILY_TOTALS"],
    ["UPDATE orders SET status = 'x'", "UPDATE · ORDERS"],
    ["DELETE FROM orders WHERE id = 1", "DELETE · ORDERS"],
    ["MERGE INTO target t USING src s ON t.id = s.id", "MERGE · TARGET"],
    ["COPY INTO orders FROM @my_stage", "COPY · ORDERS"],
    ["TRUNCATE TABLE staging.events", "TRUNCATE · EVENTS"],
    // DDL folds the object kind into the verb.
    ["CREATE TABLE customers (id INT)", "CREATE TABLE · CUSTOMERS"],
    ["create or replace temporary table db.sch.t1 (id int)", "CREATE TABLE · T1"],
    ["CREATE OR REPLACE DYNAMIC TABLE dt TARGET_LAG = '1 min'", "CREATE DYNAMIC TABLE · DT"],
    ["CREATE VIEW IF NOT EXISTS v AS SELECT 1", "CREATE VIEW · V"],
    ["CREATE OR REPLACE SECURE MATERIALIZED VIEW mv AS SELECT 1", "CREATE MATERIALIZED VIEW · MV"],
    ["ALTER TABLE orders ADD COLUMN c INT", "ALTER TABLE · ORDERS"],
    ["ALTER WAREHOUSE compute_wh SET WAREHOUSE_SIZE = 'X'", "ALTER WAREHOUSE · COMPUTE_WH"],
    ["DROP TABLE IF EXISTS staging.tmp", "DROP TABLE · TMP"],
    ["UNDROP SCHEMA analytics.public", "UNDROP SCHEMA · PUBLIC"],
    // Introspection and session.
    ["DESCRIBE TABLE orders", "DESCRIBE · ORDERS"],
    ["desc orders", "DESCRIBE · ORDERS"],
    ["SHOW TABLES IN SCHEMA analytics.public", "SHOW TABLES"],
    ["SHOW TERSE DYNAMIC TABLES", "SHOW DYNAMIC TABLES"],
    ["USE WAREHOUSE compute_wh", "USE WAREHOUSE · COMPUTE_WH"],
    ["USE analytics.public", "USE · PUBLIC"],
    ["CALL my_proc(1)", "CALL · MY_PROC"],
    // Privileges and stage files.
    ["GRANT SELECT ON TABLE orders TO ROLE analyst", "GRANT · ORDERS"],
    ["REVOKE USAGE ON WAREHOUSE compute_wh FROM ROLE analyst", "REVOKE · COMPUTE_WH"],
    ["PUT file:///tmp/data.csv @my_stage/raw", "PUT · MY_STAGE"],
    ["LIST @my_stage", "LIST · MY_STAGE"],
  ])("labels %s as %s", (sql, expected) => {
    expect(provisionalTitle(sql)).toBe(expected);
  });

  it("keeps the case of a quoted identifier but upper-cases a bare one", () => {
    // Quoting is exactly how a user asks for a specific case — don't undo it.
    expect(provisionalTitle('SELECT * FROM "myOrders"')).toBe("SELECT · myOrders");
    expect(provisionalTitle("SELECT * FROM myOrders")).toBe("SELECT · MYORDERS");
  });

  it("describes the first statement only", () => {
    expect(provisionalTitle("USE WAREHOUSE wh;\nSELECT * FROM orders;")).toBe("USE WAREHOUSE · WH");
  });

  it("looks past leading comments", () => {
    expect(provisionalTitle("-- daily revenue\n/* v2 */\nSELECT * FROM orders")).toBe("SELECT · ORDERS");
  });

  it("truncates a very long object name", () => {
    const title = provisionalTitle(`SELECT * FROM ${"a".repeat(60)}`)!;
    expect(title.length).toBeLessThanOrEqual("SELECT · ".length + 22);
    expect(title.endsWith("…")).toBe(true);
  });

  it("falls back to the leading keyword for statements it does not model", () => {
    // Better a rough word than "SQL 3" — and it can never be wrong about the verb.
    expect(provisionalTitle("BEGIN TRANSACTION")).toBe("BEGIN");
    expect(provisionalTitle("EXPLAIN SELECT 1")).toBe("EXPLAIN");
  });

  it("never throws on nonsense input", () => {
    expect(() => provisionalTitle("((((")).not.toThrow();
    expect(() => provisionalTitle('"unterminated')).not.toThrow();
    expect(() => provisionalTitle("/* unterminated")).not.toThrow();
  });
});

const base: Tab = {
  id: "t1",
  title: "SQL 1",
  path: null,
  sql: "",
  savedSql: "",
  result: null,
  error: null,
  isDefaultTitle: true,
};

describe("tabDisplayTitle", () => {
  it("derives a title only while the tab still carries its auto-generated one", () => {
    expect(tabDisplayTitle({ ...base, sql: "SELECT * FROM orders" })).toBe("SELECT · ORDERS");
    // Renamed (isDefaultTitle cleared): the user's title wins, always.
    expect(tabDisplayTitle({ ...base, sql: "SELECT * FROM orders", title: "Revenue", isDefaultTitle: false }))
      .toBe("Revenue");
  });

  it("falls back to the counter title for an empty tab", () => {
    expect(tabDisplayTitle(base)).toBe("SQL 1");
  });

  it("leaves file-backed and non-SQL tabs alone", () => {
    // A file tab's title is its filename; a notebook/python tab isn't SQL at all.
    expect(tabDisplayTitle({ ...base, path: "/tmp/q.sql", title: "q.sql", sql: "SELECT * FROM orders" }))
      .toBe("q.sql");
    expect(tabDisplayTitle({ ...base, kind: "python", title: "scratch.py", sql: "SELECT * FROM orders" }))
      .toBe("scratch.py");
  });
});
