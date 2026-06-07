// Copyright (c) 2026 Technarion Oy. All rights reserved.

import { describe, expect, it } from "vitest";
import { buildJoinSQL } from "./buildJoinQuery";
import { formatPathLabel } from "./JoinQueryPanel";
import type { JoinQueryState, JoinPath } from "./erTypes";

function makeState(overrides: Partial<JoinQueryState> = {}): JoinQueryState {
  return {
    database: "MY_DB",
    baseTable: { schema: "S", name: "ORDERS" },
    joins: [
      {
        table: { schema: "S", name: "USERS" },
        joinType: "INNER",
        onCondition: "S.ORDERS.USER_ID = S.USERS.ID",
        isIntermediate: false,
      },
    ],
    selectedColumns: new Map(),
    ...overrides,
  };
}

describe("buildJoinSQL", () => {
  it("generates basic INNER JOIN with database-qualified table names", () => {
    const sql = buildJoinSQL(makeState());
    expect(sql).toContain("SELECT");
    expect(sql).toContain("t1.*");
    expect(sql).toContain("t2.*");
    expect(sql).toContain("FROM MY_DB.S.ORDERS t1");
    expect(sql).toContain("INNER JOIN MY_DB.S.USERS t2 ON t1.USER_ID = t2.ID");
  });

  it("respects LEFT join type", () => {
    const sql = buildJoinSQL(makeState({
      joins: [{
        table: { schema: "S", name: "USERS" },
        joinType: "LEFT",
        onCondition: "S.ORDERS.USER_ID = S.USERS.ID",
        isIntermediate: false,
      }],
    }));
    expect(sql).toContain("LEFT JOIN MY_DB.S.USERS t2");
  });

  it("respects RIGHT join type", () => {
    const sql = buildJoinSQL(makeState({
      joins: [{
        table: { schema: "S", name: "USERS" },
        joinType: "RIGHT",
        onCondition: "S.ORDERS.USER_ID = S.USERS.ID",
        isIntermediate: false,
      }],
    }));
    expect(sql).toContain("RIGHT JOIN MY_DB.S.USERS t2");
  });

  it("respects FULL OUTER join type", () => {
    const sql = buildJoinSQL(makeState({
      joins: [{
        table: { schema: "S", name: "USERS" },
        joinType: "FULL OUTER",
        onCondition: "S.ORDERS.USER_ID = S.USERS.ID",
        isIntermediate: false,
      }],
    }));
    expect(sql).toContain("FULL OUTER JOIN MY_DB.S.USERS t2");
  });

  it("uses specific columns when selectedColumns is set", () => {
    const selectedColumns = new Map<string, string[]>();
    selectedColumns.set("S.ORDERS", ["ID", "TOTAL"]);
    selectedColumns.set("S.USERS", ["NAME", "EMAIL"]);

    const sql = buildJoinSQL(makeState({ selectedColumns }));
    expect(sql).toContain("t1.ID");
    expect(sql).toContain("t1.TOTAL");
    expect(sql).toContain("t2.NAME");
    expect(sql).toContain("t2.EMAIL");
    expect(sql).not.toContain("t1.*");
    expect(sql).not.toContain("t2.*");
  });

  it("handles composite FK with AND in ON condition", () => {
    const sql = buildJoinSQL(makeState({
      joins: [{
        table: { schema: "S", name: "DETAILS" },
        joinType: "INNER",
        onCondition: "S.ORDERS.ID = S.DETAILS.ORDER_ID AND S.ORDERS.REGION = S.DETAILS.REGION",
        isIntermediate: false,
      }],
    }));
    expect(sql).toContain("ON t1.ID = t2.ORDER_ID AND t1.REGION = t2.REGION");
  });

  it("handles multiple joins with correct aliases", () => {
    const sql = buildJoinSQL({
      database: "MY_DB",
      baseTable: { schema: "S", name: "ORDER_ITEMS" },
      joins: [
        {
          table: { schema: "S", name: "ORDERS" },
          joinType: "INNER",
          onCondition: "S.ORDER_ITEMS.ORDER_ID = S.ORDERS.ID",
          isIntermediate: true,
        },
        {
          table: { schema: "S", name: "USERS" },
          joinType: "LEFT",
          onCondition: "S.ORDERS.USER_ID = S.USERS.ID",
          isIntermediate: false,
        },
      ],
      selectedColumns: new Map(),
    });
    expect(sql).toContain("FROM MY_DB.S.ORDER_ITEMS t1");
    expect(sql).toContain("INNER JOIN MY_DB.S.ORDERS t2 ON t1.ORDER_ID = t2.ID");
    expect(sql).toContain("LEFT JOIN MY_DB.S.USERS t3 ON t2.USER_ID = t3.ID");
    expect(sql).toContain("t1.*");
    expect(sql).toContain("t2.*");
    expect(sql).toContain("t3.*");
  });

  it("includes LIMIT 1000 by default", () => {
    const sql = buildJoinSQL(makeState());
    expect(sql).toContain("LIMIT 1000");
  });

  it("handles cross-schema references", () => {
    const sql = buildJoinSQL({
      database: "MY_DB",
      baseTable: { schema: "SALES", name: "ORDERS" },
      joins: [{
        table: { schema: "CATALOG", name: "PRODUCTS" },
        joinType: "INNER",
        onCondition: "SALES.ORDERS.PRODUCT_ID = CATALOG.PRODUCTS.ID",
        isIntermediate: false,
      }],
      selectedColumns: new Map(),
    });
    expect(sql).toContain("FROM MY_DB.SALES.ORDERS t1");
    expect(sql).toContain("INNER JOIN MY_DB.CATALOG.PRODUCTS t2 ON t1.PRODUCT_ID = t2.ID");
  });
});

describe("formatPathLabel", () => {
  it("formats a normal path", () => {
    const path: JoinPath = {
      tables: [
        { schema: "S", name: "A" },
        { schema: "S", name: "B" },
      ],
      edges: [
        { from: { schema: "S", table: "A", col: "B_ID" }, to: { schema: "S", table: "B", col: "ID" } },
      ],
    };
    expect(formatPathLabel(path)).toBe("S.A --(B_ID = ID)--> S.B");
  });

  it("returns empty string for empty path", () => {
    expect(formatPathLabel({ tables: [], edges: [] })).toBe("");
  });

  it("handles malformed path where edges outnumber tables", () => {
    const path: JoinPath = {
      tables: [{ schema: "S", name: "A" }],
      edges: [
        { from: { schema: "S", table: "A", col: "X" }, to: { schema: "S", table: "B", col: "Y" } },
      ],
    };
    // Should not throw — stops gracefully when nextTable is undefined
    const label = formatPathLabel(path);
    expect(label).toBe("S.A");
  });
});
