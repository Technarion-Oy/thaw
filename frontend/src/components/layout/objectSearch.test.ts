import { describe, it, expect } from "vitest";
import type { DataNode } from "antd/es/tree";
import { buildSearchPredicate, filterTree, filterTreeLimited } from "./objectSearch";

describe("buildSearchPredicate", () => {
  it("does a case-insensitive substring match by default", () => {
    const p = buildSearchPredicate("cust", false, false, []);
    expect(p.active).toBe(true);
    expect(p.regexError).toBe(false);
    expect(p.matches("CUSTOMERS", "TABLE")).toBe(true);
    expect(p.matches("orders", "TABLE")).toBe(false);
  });

  it("honors the case-sensitive toggle", () => {
    const insensitive = buildSearchPredicate("CUST", false, false, []);
    expect(insensitive.matches("customers", "TABLE")).toBe(true);
    const sensitive = buildSearchPredicate("CUST", false, true, []);
    expect(sensitive.matches("customers", "TABLE")).toBe(false);
    expect(sensitive.matches("CUSTOMERS", "TABLE")).toBe(true);
  });

  it("treats the query as a regex in regex mode", () => {
    const p = buildSearchPredicate("^dim_.*_v[0-9]+$", true, false, []);
    expect(p.regexError).toBe(false);
    expect(p.matches("DIM_DATE_V2", "TABLE")).toBe(true);
    expect(p.matches("fact_sales", "TABLE")).toBe(false);
  });

  it("regex mode respects case sensitivity", () => {
    expect(buildSearchPredicate("^dim", true, false, []).matches("DIM_X", "TABLE")).toBe(true);
    expect(buildSearchPredicate("^dim", true, true, []).matches("DIM_X", "TABLE")).toBe(false);
  });

  it("falls back to literal matching on invalid regex without throwing", () => {
    const p = buildSearchPredicate("cust(", true, false, []);
    expect(p.regexError).toBe(true);
    // "cust(" is invalid regex → literal substring against the raw text
    expect(p.matches("my_cust(omers", "TABLE")).toBe(true);
    expect(p.matches("customers", "TABLE")).toBe(false);
  });

  it("filters by object kind", () => {
    const p = buildSearchPredicate("", false, false, ["PROCEDURE", "TASK"]);
    expect(p.active).toBe(true);
    expect(p.matches("anything", "PROCEDURE")).toBe(true);
    expect(p.matches("anything", "TASK")).toBe(true);
    expect(p.matches("anything", "TABLE")).toBe(false);
  });

  it("combines a query and a kind filter (AND)", () => {
    const p = buildSearchPredicate("load", false, false, ["PROCEDURE"]);
    expect(p.matches("LOAD_STAGING", "PROCEDURE")).toBe(true);
    expect(p.matches("LOAD_STAGING", "TABLE")).toBe(false); // wrong kind
    expect(p.matches("SALES", "PROCEDURE")).toBe(false);    // wrong name
  });

  it("is inactive when both query and kind filter are empty", () => {
    const p = buildSearchPredicate("   ", false, false, []);
    expect(p.active).toBe(false);
    expect(p.matches("anything", "TABLE")).toBe(true); // matches everything when idle
  });
});

// Helpers to build a small tree resembling the object browser.
const obj = (db: string, schema: string, kind: string, name: string, children?: DataNode[]): DataNode => ({
  key: `obj:${db}:${schema}:${kind}:${name}`,
  title: name,
  ...(children ? { children } : {}),
});
const col = (db: string, schema: string, table: string, name: string): DataNode => ({
  key: `col:${db}:${schema}:${table}:${name}`,
  title: name,
});
const group = (key: string, title: string, children: DataNode[]): DataNode => ({ key, title, children });

describe("filterTree", () => {
  const tree: DataNode[] = [
    group("db:DB", "DB", [
      group("schema:DB:S", "S", [
        group("type:DB:S:TABLE", "Tables", [
          obj("DB", "S", "TABLE", "CUSTOMERS"),
          obj("DB", "S", "TABLE", "ORDERS"),
        ]),
        group("type:DB:S:PROCEDURE", "Procedures", [
          obj("DB", "S", "PROCEDURE", "LOAD_CUSTOMERS"),
        ]),
      ]),
    ]),
  ];

  it("keeps matching obj nodes and prunes empty structural parents", () => {
    const { matches } = buildSearchPredicate("cust", false, false, []);
    const out = filterTree(tree, matches);
    // DB → S kept; only the Tables group and Procedures group that still contain a match survive.
    const db = out[0];
    const schema = (db.children as DataNode[])[0];
    const groupKeys = (schema.children as DataNode[]).map((n) => String(n.key));
    expect(groupKeys).toContain("type:DB:S:TABLE");
    expect(groupKeys).toContain("type:DB:S:PROCEDURE"); // LOAD_CUSTOMERS matches "cust"
    const tableObjs = ((schema.children as DataNode[])[0].children as DataNode[]).map((n) => String(n.key));
    expect(tableObjs).toEqual(["obj:DB:S:TABLE:CUSTOMERS"]); // ORDERS pruned
  });

  it("returns nothing when there is no match", () => {
    const { matches } = buildSearchPredicate("zzz", false, false, []);
    expect(filterTree(tree, matches)).toEqual([]);
  });

  it("applies the kind filter through the tree", () => {
    const { matches } = buildSearchPredicate("", false, false, ["PROCEDURE"]);
    const out = filterTree(tree, matches);
    const schema = (out[0].children as DataNode[])[0];
    const groupKeys = (schema.children as DataNode[]).map((n) => String(n.key));
    expect(groupKeys).toEqual(["type:DB:S:PROCEDURE"]); // Tables group dropped entirely
  });

  it("preserves the loaded subtree (columns) of a matched object — finding 2", () => {
    const withCols: DataNode[] = [
      group("db:DB", "DB", [
        group("schema:DB:S", "S", [
          group("type:DB:S:TABLE", "Tables", [
            obj("DB", "S", "TABLE", "CUSTOMERS", [
              col("DB", "S", "CUSTOMERS", "ID"),
              col("DB", "S", "CUSTOMERS", "NAME"),
            ]),
          ]),
        ]),
      ]),
    ];
    const { matches } = buildSearchPredicate("cust", false, false, []);
    const out = filterTree(withCols, matches);
    const table = ((((out[0].children as DataNode[])[0]).children as DataNode[])[0].children as DataNode[])[0];
    expect(String(table.key)).toBe("obj:DB:S:TABLE:CUSTOMERS");
    // Columns survive so expanding the hit shows real content, not an empty node.
    expect((table.children as DataNode[]).map((c) => String(c.key))).toEqual([
      "col:DB:S:CUSTOMERS:ID",
      "col:DB:S:CUSTOMERS:NAME",
    ]);
  });

  it("matches a parent task on its own name and keeps its subtree", () => {
    const taskTree: DataNode[] = [
      group("db:DB", "DB", [
        group("schema:DB:S", "S", [
          group("type:DB:S:TASK", "Tasks", [
            obj("DB", "S", "TASK", "ROOT_PIPELINE", [
              obj("DB", "S", "TASK", "CHILD_A"),
              obj("DB", "S", "TASK", "CHILD_B"),
            ]),
          ]),
        ]),
      ]),
    ];
    const { matches } = buildSearchPredicate("root", false, false, []);
    const out = filterTree(taskTree, matches);
    const root = ((((out[0].children as DataNode[])[0]).children as DataNode[])[0].children as DataNode[])[0];
    expect(String(root.key)).toBe("obj:DB:S:TASK:ROOT_PIPELINE");
    // Self-matching parent keeps all its children intact.
    expect((root.children as DataNode[]).length).toBe(2);
  });

  it("emits a matched object with empty children as a leaf (no dead expander) — finding 7", () => {
    const emptyKids: DataNode[] = [obj("DB", "S", "STAGE", "MY_STAGE", [])];
    const { matches } = buildSearchPredicate("stage", false, false, []);
    const out = filterTree(emptyKids, matches);
    expect(out).toHaveLength(1);
    expect("children" in out[0]).toBe(false); // children stripped → renders as leaf
  });
});

describe("filterTreeLimited", () => {
  // A schema with 5 matching tables.
  const bigTree: DataNode[] = [
    group("db:DB", "DB", [
      group("schema:DB:S", "S", [
        group("type:DB:S:TABLE", "Tables", [
          obj("DB", "S", "TABLE", "T1"),
          obj("DB", "S", "TABLE", "T2"),
          obj("DB", "S", "TABLE", "T3"),
          obj("DB", "S", "TABLE", "T4"),
          obj("DB", "S", "TABLE", "T5"),
        ]),
      ]),
    ]),
  ];
  const matchAll = buildSearchPredicate(".*", true, false, []).matches; // regex `.*`

  it("caps kept matches at the limit and reports truncation", () => {
    const { nodes, matched, truncated } = filterTreeLimited(bigTree, matchAll, 3);
    expect(matched).toBe(5);      // walked all, counted all
    expect(truncated).toBe(true); // more than the limit
    const kept = ((nodes[0].children as DataNode[])[0].children as DataNode[])[0].children as DataNode[];
    expect(kept.map((n) => String(n.key))).toEqual([
      "obj:DB:S:TABLE:T1",
      "obj:DB:S:TABLE:T2",
      "obj:DB:S:TABLE:T3",
    ]); // first 3 in tree order
  });

  it("keeps everything and reports no truncation when under the limit", () => {
    const { nodes, matched, truncated } = filterTreeLimited(bigTree, matchAll, 100);
    expect(matched).toBe(5);
    expect(truncated).toBe(false);
    const kept = ((nodes[0].children as DataNode[])[0].children as DataNode[])[0].children as DataNode[];
    expect(kept).toHaveLength(5);
  });

  it("prunes a parent whose only matches are beyond the limit", () => {
    // limit 0 → nothing kept, but everything counted.
    const { nodes, matched, truncated } = filterTreeLimited(bigTree, matchAll, 0);
    expect(nodes).toEqual([]);
    expect(matched).toBe(5);
    expect(truncated).toBe(true);
  });
});
