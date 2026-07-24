// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { tabStripSignature } from "./TabBar";
import type { Tab } from "../../store/queryStore";

// A baseline scratch tab; helpers below clone it with one field changed.
const base: Tab = {
  id: "t1",
  title: "SQL (1)",
  path: null,
  sql: "SELECT 1",
  savedSql: "SELECT 1",
  result: null,
  error: null,
};

const sig = (patch: Partial<Tab>): string => tabStripSignature({ ...base, ...patch });

describe("tabStripSignature", () => {
  it("is stable when only sql changes but the dirty flag does not flip", () => {
    // Clean tab: sql === savedSql. Editing both together (e.g. a disk re-read)
    // keeps it clean, so the strip shows nothing new — signature must not change.
    expect(sig({ sql: "SELECT 2", savedSql: "SELECT 2" })).toBe(sig({}));
  });

  it("changes when the dirty flag flips (sql diverges from savedSql)", () => {
    // The whole point (#762): typing makes a clean tab dirty exactly once; that
    // transition is the only sql-driven change the strip must react to.
    expect(sig({ sql: "SELECT 2" })).not.toBe(sig({}));
  });

  it("is stable across further edits once already dirty", () => {
    const dirtyA = sig({ sql: "SELECT 2" });
    const dirtyB = sig({ sql: "SELECT 22222" });
    expect(dirtyA).toBe(dirtyB); // both dirty vs. the same savedSql
  });

  // Every field the strip renders must be in the signature, or that field going
  // stale won't re-render the strip. One case per rendered field guards the
  // invariant documented on tabStripSignature.
  it.each<[string, Partial<Tab>]>([
    ["id", { id: "t2" }],
    ["title", { title: "renamed" }],
    ["path", { path: "/tmp/a.sql" }],
    ["kind", { kind: "python" }],
    ["diff", { diff: { leftLabel: "L", rightLabel: "R", leftText: "", rightText: "" } }],
    ["mcpOrigin", { mcpOrigin: true }],
    ["orphaned", { orphaned: true }],
    ["preview", { preview: true }],
  ])("changes when the rendered field %s changes", (_field, patch) => {
    expect(sig(patch)).not.toBe(sig({}));
  });

  it("does not alias distinct states when title/path contain spaces", () => {
    // A NUL field separator (not a space) prevents "a b"/"" from colliding with
    // "a"/"b" across the title|path boundary.
    const a = sig({ title: "a b", path: "" });
    const b = sig({ title: "a", path: " b" });
    expect(a).not.toBe(b);
  });
});
