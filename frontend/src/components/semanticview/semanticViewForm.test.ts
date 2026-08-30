// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from "vitest";

// semanticViewForm.tsx imports ObjectNameCaseControl, which calls
// GetSnowflakeKeywords() at module load time — stub it out rather than
// requiring a browser `window` for this pure-function test.
vi.mock("../../../wailsjs/go/sqleditor/Service", () => ({
  GetSnowflakeKeywords: () => Promise.resolve([]),
}));

import {
  isCompleteRelationship, emptyRelationship, emptyTableRow, toLogicalTable, duplicateAliases,
  duplicateRelationshipNames,
} from "./semanticViewForm";

describe("toLogicalTable", () => {
  it("falls back to the table name when no alias was typed", () => {
    const row = { ...emptyTableRow("DB", "SC"), table: "ORDERS" };
    expect(toLogicalTable(row).alias).toBe("ORDERS");
  });

  it("keeps an explicit alias", () => {
    const row = { ...emptyTableRow("DB", "SC"), table: "ORDERS", alias: "o" };
    expect(toLogicalTable(row).alias).toBe("o");
  });

  it("doesn't leak the form-only db/schema/table parts into the payload", () => {
    const row = { ...emptyTableRow("DB", "SC"), table: "ORDERS" };
    const logical = toLogicalTable(row) as unknown as Record<string, unknown>;
    expect(logical.db).toBeUndefined();
    expect(logical.schema).toBeUndefined();
    expect(logical.table).toBeUndefined();
  });
});

describe("duplicateAliases", () => {
  it("is empty when every alias is unique", () => {
    const rows = [
      { ...emptyTableRow("DB", "SC"), table: "ORDERS" },
      { ...emptyTableRow("DB", "SC"), table: "CUSTOMERS" },
    ];
    expect(duplicateAliases(rows).size).toBe(0);
  });

  it("flags an alias shared by two rows", () => {
    const rows = [
      { ...emptyTableRow("DB", "SC"), table: "ORDERS", alias: "t" },
      { ...emptyTableRow("DB2", "SC2"), table: "ORDERS2", alias: "t" },
    ];
    expect(duplicateAliases(rows)).toEqual(new Set(["t"]));
  });

  it("ignores blank aliases (table not picked yet)", () => {
    const rows = [emptyTableRow("DB", "SC"), emptyTableRow("DB", "SC")];
    expect(duplicateAliases(rows).size).toBe(0);
  });
});

describe("duplicateRelationshipNames", () => {
  it("is empty when every name is unique", () => {
    const rows = [
      { ...emptyRelationship(), name: "r1" },
      { ...emptyRelationship(), name: "r2" },
    ];
    expect(duplicateRelationshipNames(rows).size).toBe(0);
  });

  it("flags a name shared by two rows", () => {
    const rows = [
      { ...emptyRelationship(), name: "r1" },
      { ...emptyRelationship(), name: "r1" },
    ];
    expect(duplicateRelationshipNames(rows)).toEqual(new Set(["r1"]));
  });

  it("ignores blank names", () => {
    const rows = [emptyRelationship(), emptyRelationship()];
    expect(duplicateRelationshipNames(rows).size).toBe(0);
  });
});

describe("isCompleteRelationship", () => {
  const base = {
    ...emptyRelationship(),
    name: "r", table: "orders", refTable: "rates", columns: ["order_id"],
  };

  it("requires name/table/refTable/columns for a standard join", () => {
    expect(isCompleteRelationship(base)).toBe(true);
    expect(isCompleteRelationship({ ...base, name: "" })).toBe(false);
    expect(isCompleteRelationship({ ...base, columns: [] })).toBe(false);
  });

  it("requires refColumns for an ASOF join", () => {
    expect(isCompleteRelationship({ ...base, joinType: "ASOF" })).toBe(false);
    expect(isCompleteRelationship({ ...base, joinType: "ASOF", refColumns: ["ts"] })).toBe(true);
  });

  it("requires both range bounds for a BETWEEN join", () => {
    expect(isCompleteRelationship({ ...base, joinType: "BETWEEN" })).toBe(false);
    expect(isCompleteRelationship({ ...base, joinType: "BETWEEN", rangeStart: "a" })).toBe(false);
    expect(isCompleteRelationship({ ...base, joinType: "BETWEEN", rangeStart: "a", rangeEnd: "b" })).toBe(true);
  });
});
