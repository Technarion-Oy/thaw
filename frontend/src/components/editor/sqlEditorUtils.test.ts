import { describe, it, expect } from "vitest";
import { identifierRangeAt, starMenuEligible, normId, byteColToUtf16Col } from "./sqlEditorUtils";

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

  it("does not let an unterminated quote swallow the rest of the line", () => {
    // Mid-typing a quoted identifier: the open quote must not mark the tail as ident.
    const line = 'SELECT * FROM DB.SCHEMA."MY';
    expect(identifierRangeAt(line, line.length - 1)).toBeNull(); // inside the open quote
    // A bare segment before the open quote still resolves on its own.
    expect(span(line, 14)).toBe("DB.SCHEMA."); // trailing dot is bare; open quote excluded
  });
});

describe("starMenuEligible", () => {
  // column = 1-based Monaco cursor column of the first `*` in the line.
  const atStar = (line: string) => starMenuEligible(line, line.indexOf("*") + 1);

  it("shows for a bare select-list star", () => {
    expect(atStar("SELECT * FROM t")).toBe(true);
    expect(atStar("SELECT a, * FROM t")).toBe(true);
  });

  it("shows for alias.* — the star is not inside the alias identifier", () => {
    expect(atStar("SELECT t.* FROM tbl t")).toBe(true);
  });

  it("hides when the star is inside a quoted object name", () => {
    expect(atStar(`SELECT "ID" FROM "DB"."PUBLIC"."Testin*table"`)).toBe(false);
  });

  it("hides when the star is inside a single-quoted string literal", () => {
    expect(atStar("SELECT x FROM t WHERE s = 'a*b'")).toBe(false);
  });

  it("stays eligible when an apostrophe lives in a double-quoted identifier", () => {
    expect(atStar(`SELECT "it's", * FROM t`)).toBe(true); // the ' is inside "it's", not a string
  });

  it("is false when the cursor isn't on a star", () => {
    expect(starMenuEligible("SELECT a FROM t", 8)).toBe(false);
  });

  it("resolves on either edge of the star", () => {
    const line = "SELECT * FROM t";
    const starCol = line.indexOf("*") + 1;
    expect(starMenuEligible(line, starCol)).toBe(true);     // on the star
    expect(starMenuEligible(line, starCol + 1)).toBe(true); // right edge
  });
});

describe("normId", () => {
  it("upper-cases a bare identifier (Snowflake folds unquoted names)", () => {
    expect(normId("foo")).toBe("FOO");
    expect(normId("Foo")).toBe("FOO");
  });

  it("preserves case of a quoted identifier and strips the quotes", () => {
    expect(normId('"Foo"')).toBe("Foo");
    expect(normId('"foo"')).toBe("foo");
    expect(normId('"Foo"')).not.toBe(normId('"foo"')); // stay distinct
  });

  it("unescapes doubled quotes inside a quoted identifier", () => {
    expect(normId('"a""b"')).toBe('a"b');
  });
});

// byteColToUtf16Col(lineText, byteCol): backend validators emit 1-based UTF-8 byte
// columns; Monaco wants 1-based UTF-16 code units. See issue #702.
describe("byteColToUtf16Col", () => {
  it("passes ASCII columns through unchanged", () => {
    const line = "SELECT foo, bar";
    expect(byteColToUtf16Col(line, 1)).toBe(1);
    expect(byteColToUtf16Col(line, 8)).toBe(8);
  });

  it("shifts back over earlier multi-byte chars (issue #702 repro)", () => {
    // 'äöå€' is 9 bytes but 4 UTF-16 units; the unclosed string starts at
    // byte col 21 which is Monaco col 16.
    const line = "SELECT 'äöå€', 'unclosed";
    expect(byteColToUtf16Col(line, 21)).toBe(16);
  });

  it("counts an astral emoji as a 4-byte / 2-unit char", () => {
    // 😀 = 4 bytes, 2 UTF-16 units. "a😀b": 'b' is byte col 6, Monaco col 4.
    const line = "a😀b";
    expect(byteColToUtf16Col(line, 6)).toBe(4);
  });

  it("clamps a byte column past the line end to end+1", () => {
    expect(byteColToUtf16Col("äö", 10)).toBe(3);
  });
});
