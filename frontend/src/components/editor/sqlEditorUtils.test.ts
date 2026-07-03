import { describe, it, expect } from "vitest";
import { identifierRangeAt } from "./sqlEditorUtils";

// identifierRangeAt(line, idx0) → 1-based Monaco {start, end} (end exclusive) of the
// dotted identifier at 0-based char index idx0, quote-aware. Substring is
// line.slice(start-1, end-1).
const span = (line: string, idx0: number) => {
  const r = identifierRangeAt(line, idx0);
  return r ? line.slice(r.start - 1, r.end - 1) : null;
};

describe("identifierRangeAt", () => {
  it("spans a bare dotted identifier", () => {
    const line = "SELECT * FROM DB.SCHEMA.NAME x";
    expect(span(line, line.indexOf("SCHEMA"))).toBe("DB.SCHEMA.NAME");
    expect(span(line, line.indexOf("DB."))).toBe("DB.SCHEMA.NAME");
    expect(span(line, line.indexOf("NAME"))).toBe("DB.SCHEMA.NAME");
  });

  it("includes double-quoted segments with spaces", () => {
    const line = 'FROM DB."MY TABLE".COL';
    // hovering inside the quoted part spans the whole dotted identifier
    expect(span(line, line.indexOf("MY TABLE"))).toBe('DB."MY TABLE".COL');
    expect(span(line, line.indexOf("COL"))).toBe('DB."MY TABLE".COL');
  });

  it("handles a lone quoted identifier and the escaped-quote case", () => {
    expect(span('"a b"', 2)).toBe('"a b"');
    expect(span('"a""b c"', 3)).toBe('"a""b c"');
  });

  it("does not merge identifiers separated by unquoted whitespace", () => {
    const line = "orders customers";
    expect(span(line, 2)).toBe("orders");
    expect(span(line, 10)).toBe("customers");
  });

  it("returns null off any identifier", () => {
    expect(identifierRangeAt("a + b", 2)).toBeNull(); // the '+' / spaces
    expect(identifierRangeAt("   ", 1)).toBeNull();
  });

  it("resolves when the cursor sits just past the last char", () => {
    const line = "FROM foo";
    expect(span(line, line.length)).toBe("foo"); // idx0 == length → step back one
  });
});
