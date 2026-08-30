// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  diffAliases, hasAliasChange, remapAlias, remapQualified, swappedAliases,
} from "./semanticViewAliases";

const row = (alias: string, table: string) => ({ alias, table });

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

  it("leaves a duplicated alias alone — a reference can't be attributed to one row", () => {
    // Two same-named tables from different schemas both auto-seed "orders".
    // Renaming only the second must not repoint the first row's references.
    const diff = diffAliases(["orders", "orders"], ["orders", "orders2"]);
    expect(hasAliasChange(diff)).toBe(false);
    expect(remapAlias("orders", diff)).toBe("orders");
  });

  it("leaves an alias that still exists on another row alone", () => {
    const diff = diffAliases(["orders", "customers"], ["customers", "customers"]);
    expect(remapAlias("customers", diff)).toBe("customers");
    expect(remapAlias("orders", diff)).toBe("customers");
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

  it("splits on the last dot, so an alias containing a dot survives", () => {
    const renamedDotted = diffAliases(["ord.v2"], ["ord.v3"]);
    expect(remapQualified("ord.v2.order_date", renamedDotted)).toBe("ord.v3.order_date");
  });
});

describe("swappedAliases", () => {
  it("reports a row re-pointed at another table", () => {
    expect(swappedAliases(
      [row("orders", "DB.SC.ORDERS")],
      [row("orders", "DB.SC.ORDERS_V2")],
    )).toEqual(["orders"]);
  });

  it("reports it under the new alias when the alias was auto-reseeded", () => {
    // Picking a new table reseeds an untouched alias, so alias and table change
    // together — the case a rename-only check misses.
    expect(swappedAliases(
      [row("ORDERS", "DB.SC.ORDERS")],
      [row("ORDERS_V2", "DB.SC.ORDERS_V2")],
    )).toEqual(["ORDERS_V2"]);
  });

  it("reports nothing when only the alias was edited", () => {
    expect(swappedAliases(
      [row("orders", "DB.SC.ORDERS")],
      [row("sales", "DB.SC.ORDERS")],
    )).toEqual([]);
  });

  it("reports nothing when a row was added or deleted", () => {
    const before = [row("orders", "DB.SC.ORDERS"), row("customers", "DB.SC.CUSTOMERS")];
    expect(swappedAliases(before, [before[1]])).toEqual([]);
    expect(swappedAliases(before, [...before, row("", "")])).toEqual([]);
  });

  it("ignores a row with no alias yet", () => {
    expect(swappedAliases([row("", "")], [row("", "DB.SC.ORDERS")])).toEqual([]);
  });
});
