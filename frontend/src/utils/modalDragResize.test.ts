// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { describe, it, expect } from "vitest";
import { parseTranslate } from "./modalDragResize";

describe("parseTranslate", () => {
  it("returns [0,0] for an un-dragged element (empty transform)", () => {
    expect(parseTranslate("")).toEqual([0, 0]);
  });

  it("parses a prior translate, including negatives and decimals", () => {
    expect(parseTranslate("translate(120px, -33.5px)")).toEqual([120, -33.5]);
  });
});
