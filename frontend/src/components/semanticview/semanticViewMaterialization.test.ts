// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  isMaterializationValid, qualifiedOptionsFromResult, NEW_MATERIALIZATION,
} from "./semanticViewMaterialization";

describe("isMaterializationValid", () => {
  it("requires a name, warehouse, and at least one dimension and metric", () => {
    expect(isMaterializationValid(NEW_MATERIALIZATION)).toBe(false);
    expect(isMaterializationValid({ ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS" })).toBe(false);
    expect(isMaterializationValid({
      ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS", dimensions: ["customers.region"], metrics: ["orders.revenue"],
    })).toBe(true);
  });
});

describe("qualifiedOptionsFromResult", () => {
  it("qualifies each option by its table_name column", () => {
    const res = { columns: ["table_name", "name"], rows: [["customers", "region"], ["orders", "segment"]], rowsAffected: 0, queryID: "", truncated: false };
    expect(qualifiedOptionsFromResult(res)).toEqual(["customers.region", "orders.segment"]);
  });

  it("matches the name/table_name columns case-insensitively", () => {
    const res = { columns: ["TABLE_NAME", "NAME"], rows: [["customers", "region"]], rowsAffected: 0, queryID: "", truncated: false };
    expect(qualifiedOptionsFromResult(res)).toEqual(["customers.region"]);
  });

  it("falls back to a bare name when table_name is missing", () => {
    const res = { columns: ["name"], rows: [["region"]], rowsAffected: 0, queryID: "", truncated: false };
    expect(qualifiedOptionsFromResult(res)).toEqual(["region"]);
  });

  it("returns no options when the name column is missing entirely", () => {
    const res = { columns: ["table_name"], rows: [["customers"]], rowsAffected: 0, queryID: "", truncated: false };
    expect(qualifiedOptionsFromResult(res)).toEqual([]);
  });

  it("returns no options for a null result", () => {
    expect(qualifiedOptionsFromResult(null)).toEqual([]);
  });

  it("degrades to no options rather than throwing when columns is null/undefined", () => {
    const res = { columns: null, rows: [["region"]], rowsAffected: 0, queryID: "", truncated: false };
    // @ts-expect-error columns is typed as string[], but a QueryResult that
    // actually arrives with a null columns field must still not throw.
    expect(qualifiedOptionsFromResult(res)).toEqual([]);
  });

  it("degrades to no options rather than throwing when rows is null/undefined", () => {
    const res = { columns: ["name"], rows: null, rowsAffected: 0, queryID: "", truncated: false };
    // @ts-expect-error rows is typed as any[][], but a QueryResult that
    // actually arrives with a null rows field must still not throw.
    expect(qualifiedOptionsFromResult(res)).toEqual([]);
  });
});
