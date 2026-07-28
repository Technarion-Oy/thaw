// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  OVERLOAD_NAMING_OPTIONS,
  DEFAULT_OVERLOAD_NAMING,
  normalizeOverloadNaming,
} from "./overloadNaming";

describe("normalizeOverloadNaming", () => {
  it("passes through every offered option", () => {
    for (const o of OVERLOAD_NAMING_OPTIONS) {
      expect(normalizeOverloadNaming(o.value)).toBe(o.value);
    }
  });

  it("falls back to the default for empty, unset, or unknown values", () => {
    for (const v of ["", undefined, "bogus", "ARGTYPES", "name"]) {
      expect(normalizeOverloadNaming(v)).toBe(DEFAULT_OVERLOAD_NAMING);
    }
  });

  it("keeps argtypes as the default so existing exports do not get renamed", () => {
    expect(DEFAULT_OVERLOAD_NAMING).toBe("argtypes");
  });

  it("offers exactly the strategies the Go side accepts", () => {
    expect(OVERLOAD_NAMING_OPTIONS.map((o) => o.value)).toEqual([
      "argtypes",
      "signature",
      "grouped",
    ]);
  });
});
