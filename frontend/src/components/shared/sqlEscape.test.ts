// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import { escapeTextLit, quoteTextLit } from "./sqlEscape";

describe("escapeTextLit", () => {
  it("leaves plain text untouched", () => {
    expect(escapeTextLit("hello world")).toBe("hello world");
  });

  it("doubles single-quotes", () => {
    expect(escapeTextLit("it's")).toBe("it''s");
  });

  it("doubles backslashes so they are not swallowed", () => {
    expect(escapeTextLit("C:\\temp")).toBe("C:\\\\temp");
  });

  it("escapes a trailing backslash so it cannot escape the closing quote", () => {
    // `C:\temp\` -> `C:\\temp\\`; wrapped as '…' the pair before the quote is a
    // literal backslash, not an escape of the delimiter.
    expect(escapeTextLit("C:\\temp\\")).toBe("C:\\\\temp\\\\");
  });

  it("escapes backslash before quotes (order matters)", () => {
    // A `\'` sequence must become `\\''`, not `\''` (which would close early).
    expect(escapeTextLit("a\\'b")).toBe("a\\\\''b");
  });
});

describe("quoteTextLit", () => {
  it("wraps the escaped value in single-quotes", () => {
    expect(quoteTextLit("it's")).toBe("'it''s'");
    expect(quoteTextLit("C:\\temp\\")).toBe("'C:\\\\temp\\\\'");
  });
});
