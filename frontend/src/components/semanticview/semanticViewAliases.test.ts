// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  diffAliases, hasAliasChange, remapAlias, remapQualified,
} from "./semanticViewAliases";

describe("diffAliases", () => {
  it("reports no change when the aliases are untouched", () => {
    const diff = diffAliases(["orders", "customers"], ["orders", "customers"]);
    expect(hasAliasChange(diff)).toBe(false);
  });

  it("detects an in-place rename", () => {
    expect(diffAliases(["orders", "customers"], ["sales", "customers"]))
      .toEqual({ renames: { orders: "sales" }, removed: [] });
  });

  it("detects a deleted row", () => {
    expect(diffAliases(["orders", "customers"], ["customers"]))
      .toEqual({ renames: {}, removed: ["orders"] });
  });

  it("treats an added row as no change to existing aliases", () => {
    const diff = diffAliases(["orders"], ["orders", "customers"]);
    expect(hasAliasChange(diff)).toBe(false);
  });

  it("ignores blank aliases from rows whose table isn't picked yet", () => {
    const diff = diffAliases(["", "orders"], ["customers", "orders"]);
    expect(hasAliasChange(diff)).toBe(false);
  });

  it("treats an alias cleared in place as removed", () => {
    expect(diffAliases(["orders"], [""]))
      .toEqual({ renames: {}, removed: ["orders"] });
  });
});

describe("remapAlias", () => {
  const diff = diffAliases(["orders", "customers"], ["sales", "customers"]);

  it("follows a rename", () => {
    expect(remapAlias("orders", diff)).toBe("sales");
  });

  it("leaves an untouched alias alone", () => {
    expect(remapAlias("customers", diff)).toBe("customers");
  });

  it("clears a removed alias", () => {
    const dropped = diffAliases(["orders"], []);
    expect(remapAlias("orders", dropped)).toBe("");
  });

  it("passes a blank reference through", () => {
    expect(remapAlias("", diff)).toBe("");
  });
});

describe("remapQualified", () => {
  const renamed = diffAliases(["orders"], ["sales"]);
  const dropped = diffAliases(["orders"], []);

  it("rewrites only the alias half of a dotted reference", () => {
    expect(remapQualified("orders.order_date", renamed)).toBe("sales.order_date");
  });

  it("clears the whole reference when the alias is gone", () => {
    expect(remapQualified("orders.order_date", dropped)).toBe("");
  });

  it("treats a dotless reference as a bare alias", () => {
    expect(remapQualified("orders", renamed)).toBe("sales");
  });
});
