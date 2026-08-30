// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from "vitest";

// semanticViewMaterialization.ts pulls in ObjectNameCaseControl.tsx, which
// calls GetSnowflakeKeywords() at module load time — stub it out rather than
// requiring a browser `window` for this pure-function test.
vi.mock("../../../wailsjs/go/sqleditor/Service", () => ({
  GetSnowflakeKeywords: () => Promise.resolve([]),
}));

import {
  buildAddMaterializationClause, isMaterializationValid, NEW_MATERIALIZATION,
} from "./semanticViewMaterialization";

describe("buildAddMaterializationClause", () => {
  it("builds the minimal clause, omitting the default AUTO refresh mode", () => {
    const clause = buildAddMaterializationClause({
      ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS", dimensions: ["region"], metrics: ["revenue"],
    });
    expect(clause).toBe('ADD MATERIALIZATION "mv1" WAREHOUSE = "WH_XS" AS\n  DIMENSIONS "region"\n  METRICS "revenue"');
  });

  it("includes REFRESH_MODE when it isn't AUTO", () => {
    const clause = buildAddMaterializationClause({
      ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS", refreshMode: "FULL", dimensions: ["region"], metrics: ["revenue"],
    });
    expect(clause).toContain("REFRESH_MODE = FULL");
  });

  it("parenthesizes IMMUTABLE WHERE and WHERE as raw SQL, not string literals", () => {
    const clause = buildAddMaterializationClause({
      ...NEW_MATERIALIZATION,
      name: "mv1", warehouse: "WH_XS", dimensions: ["region"], metrics: ["revenue"],
      immutableWhere: "region = 'US'", where: "revenue > 0",
    });
    expect(clause).toContain("IMMUTABLE WHERE (region = 'US')");
    expect(clause).toContain("WHERE (revenue > 0)");
  });

  it("quotes and joins multiple dimensions and metrics", () => {
    const clause = buildAddMaterializationClause({
      ...NEW_MATERIALIZATION,
      name: "mv1", warehouse: "WH_XS", dimensions: ["region", "segment"], metrics: ["revenue", "count"],
    });
    expect(clause).toContain('DIMENSIONS "region", "segment"');
    expect(clause).toContain('METRICS "revenue", "count"');
  });

  it("quotes a materialization or warehouse name containing special characters", () => {
    const clause = buildAddMaterializationClause({
      ...NEW_MATERIALIZATION, name: 'my "mv"', warehouse: "WH_XS", dimensions: ["region"], metrics: ["revenue"],
    });
    expect(clause).toContain('ADD MATERIALIZATION "my ""mv"""');
  });
});

describe("isMaterializationValid", () => {
  it("requires a name, warehouse, and at least one dimension and metric", () => {
    expect(isMaterializationValid(NEW_MATERIALIZATION)).toBe(false);
    expect(isMaterializationValid({ ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS" })).toBe(false);
    expect(isMaterializationValid({
      ...NEW_MATERIALIZATION, name: "mv1", warehouse: "WH_XS", dimensions: ["region"], metrics: ["revenue"],
    })).toBe(true);
  });
});
