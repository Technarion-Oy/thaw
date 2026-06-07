// Copyright (c) 2026 Technarion Oy. All rights reserved.

import { describe, expect, it } from "vitest";
import { findJoinPaths, buildJoinState } from "./joinPathfinder";

// Helper to create FK edges matching the snowflake.ERForeignKey shape
function fk(
  fromSchema: string, fromTable: string, fromCol: string,
  toSchema: string, toTable: string, toCol: string,
) {
  return { fromSchema, fromTable, fromCol, toSchema, toTable, toCol };
}

function tbl(schema: string, name: string) {
  return { schema, name };
}

describe("findJoinPaths", () => {
  it("returns empty for fewer than 2 tables", () => {
    const fks = [fk("S", "A", "id", "S", "B", "a_id")];
    expect(findJoinPaths([tbl("S", "A")], fks)).toEqual([]);
    expect(findJoinPaths([], fks)).toEqual([]);
  });

  it("finds direct path between 2 tables connected by FK", () => {
    const fks = [fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID")];
    const paths = findJoinPaths(
      [tbl("S", "ORDERS"), tbl("S", "USERS")],
      fks,
    );
    expect(paths).toHaveLength(1);
    expect(paths[0].tables).toHaveLength(2);
    expect(paths[0].edges).toHaveLength(1);
    expect(paths[0].edges[0].from.table).toBe("ORDERS");
    expect(paths[0].edges[0].to.table).toBe("USERS");
  });

  it("finds path through intermediate table", () => {
    // ORDERS -> USERS, ORDER_ITEMS -> ORDERS
    // Selecting ORDER_ITEMS and USERS should route through ORDERS
    const fks = [
      fk("S", "ORDER_ITEMS", "ORDER_ID", "S", "ORDERS", "ID"),
      fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID"),
    ];
    const paths = findJoinPaths(
      [tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")],
      fks,
    );
    expect(paths).toHaveLength(1);
    expect(paths[0].tables).toHaveLength(3);
    // Intermediate ORDERS should be in the path
    const tableNames = paths[0].tables.map((t) => t.name);
    expect(tableNames).toContain("ORDER_ITEMS");
    expect(tableNames).toContain("ORDERS");
    expect(tableNames).toContain("USERS");
  });

  it("returns multiple paths when disambiguation needed", () => {
    // Two FKs from EMPLOYEES to USERS: created_by and updated_by
    const fks = [
      fk("S", "EMPLOYEES", "CREATED_BY", "S", "USERS", "ID"),
      fk("S", "EMPLOYEES", "UPDATED_BY", "S", "USERS", "ID"),
    ];
    const paths = findJoinPaths(
      [tbl("S", "EMPLOYEES"), tbl("S", "USERS")],
      fks,
    );
    expect(paths.length).toBeGreaterThanOrEqual(2);
    // All paths should be length 2 (direct connection)
    for (const p of paths) {
      expect(p.tables).toHaveLength(2);
    }
  });

  it("returns empty for disconnected tables", () => {
    const fks = [fk("S", "A", "id", "S", "B", "a_id")];
    const paths = findJoinPaths(
      [tbl("S", "A"), tbl("S", "C")],
      fks,
    );
    expect(paths).toEqual([]);
  });

  it("handles 3+ tables via Steiner tree", () => {
    // A -> B -> C (chain)
    const fks = [
      fk("S", "A", "B_ID", "S", "B", "ID"),
      fk("S", "B", "C_ID", "S", "C", "ID"),
    ];
    const paths = findJoinPaths(
      [tbl("S", "A"), tbl("S", "B"), tbl("S", "C")],
      fks,
    );
    expect(paths).toHaveLength(1);
    expect(paths[0].tables).toHaveLength(3);
    expect(paths[0].edges).toHaveLength(2);
  });

  it("handles cross-schema FKs", () => {
    const fks = [fk("SALES", "ORDERS", "PRODUCT_ID", "CATALOG", "PRODUCTS", "ID")];
    const paths = findJoinPaths(
      [tbl("SALES", "ORDERS"), tbl("CATALOG", "PRODUCTS")],
      fks,
    );
    expect(paths).toHaveLength(1);
    expect(paths[0].edges[0].from.schema).toBe("SALES");
    expect(paths[0].edges[0].to.schema).toBe("CATALOG");
  });

  it("table not in FK graph returns empty", () => {
    const fks = [fk("S", "A", "id", "S", "B", "a_id")];
    const paths = findJoinPaths(
      [tbl("S", "A"), tbl("S", "NONEXISTENT")],
      fks,
    );
    expect(paths).toEqual([]);
  });
});

describe("buildJoinState", () => {
  it("builds correct state from a 2-table path", () => {
    const fks = [fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID")];
    const paths = findJoinPaths(
      [tbl("S", "ORDERS"), tbl("S", "USERS")],
      fks,
    );
    const state = buildJoinState(
      paths[0],
      [tbl("S", "ORDERS"), tbl("S", "USERS")],
      "MY_DB",
    );

    expect(state.database).toBe("MY_DB");
    expect(state.baseTable.name).toBe("ORDERS");
    expect(state.joins).toHaveLength(1);
    expect(state.joins[0].table.name).toBe("USERS");
    expect(state.joins[0].joinType).toBe("INNER");
    expect(state.joins[0].isIntermediate).toBe(false);
    expect(state.joins[0].onCondition).toContain("USER_ID");
    expect(state.joins[0].onCondition).toContain("ID");
  });

  it("marks intermediate tables correctly", () => {
    const fks = [
      fk("S", "ORDER_ITEMS", "ORDER_ID", "S", "ORDERS", "ID"),
      fk("S", "ORDERS", "USER_ID", "S", "USERS", "ID"),
    ];
    const paths = findJoinPaths(
      [tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")],
      fks,
    );
    const state = buildJoinState(
      paths[0],
      [tbl("S", "ORDER_ITEMS"), tbl("S", "USERS")],
      "MY_DB",
    );

    // ORDERS should be intermediate (not in user's selection)
    const ordersJoin = state.joins.find((j) => j.table.name === "ORDERS");
    expect(ordersJoin?.isIntermediate).toBe(true);

    const usersJoin = state.joins.find((j) => j.table.name === "USERS");
    expect(usersJoin?.isIntermediate).toBe(false);
  });

  it("initializes with empty selectedColumns", () => {
    const fks = [fk("S", "A", "B_ID", "S", "B", "ID")];
    const paths = findJoinPaths([tbl("S", "A"), tbl("S", "B")], fks);
    const state = buildJoinState(paths[0], [tbl("S", "A"), tbl("S", "B")], "MY_DB");
    expect(state.selectedColumns.size).toBe(0);
  });
});
