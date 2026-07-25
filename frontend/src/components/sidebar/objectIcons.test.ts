import { describe, it, expect } from "vitest";

import globalCss from "../../styles/global.css?raw";
import { KIND_ICON, KIND_VAR } from "./objectIcons";
import { OBJECT_KINDS } from "../../generated/objectKinds";

// Icons and colours are the only object-kind metadata that is NOT generated from
// the Go registry (internal/objectkind) — they are React components and
// theme-dependent CSS variables. These tests are the guard that keeps the
// hand-maintained half in step with the generated half: adding a kind to the
// registry without an icon, a colour, or the colour's global.css definition
// fails here instead of quietly rendering a grey generic file icon.
describe("object kind icons", () => {
  it("has an icon for every registered object kind", () => {
    const missing = OBJECT_KINDS.filter((k) => !KIND_ICON[k.name]).map((k) => k.name);
    expect(missing).toEqual([]);
  });

  it("has a colour variable for every registered object kind", () => {
    const missing = OBJECT_KINDS.filter((k) => !KIND_VAR[k.name]).map((k) => k.name);
    expect(missing).toEqual([]);
  });

  it("maps every registered kind to a CSS variable defined in global.css", () => {
    const undefinedVars = OBJECT_KINDS.map((k) => KIND_VAR[k.name])
      .filter((v) => v && !globalCss.includes(`${v}:`));
    expect(undefinedVars).toEqual([]);
  });

  it("does not map kinds that are not in the registry", () => {
    // A stale entry means a kind was renamed or dropped from the registry and
    // its icon was left behind — harmless at runtime, but it hides the fact that
    // the kind is gone.
    const known = new Set(OBJECT_KINDS.map((k) => k.name));
    expect(Object.keys(KIND_ICON).filter((k) => !known.has(k))).toEqual([]);
    expect(Object.keys(KIND_VAR).filter((k) => !known.has(k))).toEqual([]);
  });
});
