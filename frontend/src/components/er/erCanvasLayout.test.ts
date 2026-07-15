// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { mergeAITablesIntoDesigner, changedTableIdsFromMerge } from "./erCanvasLayout";
import type { AITableIn } from "./erCanvasLayout";
import type { DesignerTable } from "./erTypes";

function table(schema: string, name: string, cols: string[]): DesignerTable {
  return {
    id: `${schema}.${name}`, // deterministic id for assertions
    schema,
    name,
    columns: cols.map((c) => ({
      id: `${schema}.${name}.${c}`,
      name: c,
      dataType: "VARCHAR",
      isPK: false,
      notNull: false,
      fkRef: "",
      defaultValue: "",
    })),
  };
}

describe("changedTableIdsFromMerge", () => {
  it("marks replaced and appended tables, not untouched ones", () => {
    const current = [table("PUBLIC", "USERS", ["ID"]), table("PUBLIC", "KEEP", ["ID"])];
    // The AI echoes normalized (uppercase) identifiers, matching how the
    // designer stores them — mergeAITablesIntoDesigner matches by exact
    // SCHEMA.NAME, so USERS is replaced and ORDERS appended.
    const ai: AITableIn[] = [
      { schema: "PUBLIC", name: "USERS", columns: [{ name: "ID", dataType: "NUMBER" }] },
      { schema: "PUBLIC", name: "ORDERS", columns: [{ name: "ID", dataType: "NUMBER" }] },
    ];
    const merged = mergeAITablesIntoDesigner(current, ai);
    const changed = changedTableIdsFromMerge(merged, ai);

    // USERS keeps its original id through replacement; ORDERS gets a fresh uuid.
    const usersId = merged.find((t) => t.name.toUpperCase() === "USERS")!.id;
    const ordersId = merged.find((t) => t.name.toUpperCase() === "ORDERS")!.id;
    const keepId = merged.find((t) => t.name === "KEEP")!.id;

    expect(changed.has(usersId)).toBe(true);
    expect(changed.has(ordersId)).toBe(true);
    expect(changed.has(keepId)).toBe(false);
    expect(changed.size).toBe(2);
  });

  it("returns an empty set when no AI tables are provided", () => {
    const current = [table("PUBLIC", "USERS", ["ID"])];
    const merged = mergeAITablesIntoDesigner(current, []);
    expect(changedTableIdsFromMerge(merged, []).size).toBe(0);
  });
});
