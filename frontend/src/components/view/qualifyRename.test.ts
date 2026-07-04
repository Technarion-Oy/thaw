import { describe, it, expect } from "vitest";
import { qualifyRename } from "./ViewPropertiesModal";

describe("qualifyRename", () => {
  it("qualifies a bare name into the current db/schema", () => {
    expect(qualifyRename("v2", "DB", "SC")).toBe(`"DB"."SC"."v2"`);
  });

  it("treats a dotted name as an already-qualified path", () => {
    expect(qualifyRename("otherdb.otherschema.v2", "DB", "SC")).toBe(`"otherdb"."otherschema"."v2"`);
  });

  it("escapes embedded double quotes", () => {
    expect(qualifyRename(`we"ird`, "DB", "SC")).toBe(`"DB"."SC"."we""ird"`);
  });
});
