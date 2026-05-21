import { describe, expect, it } from "vitest";
import {
  searchText,
  getNotebookCellSources,
  replaceInNotebookCell,
  replaceAllInNotebook,
  reserializeCellSource,
} from "./CrossTabSearch";

// ── helpers ──────────────────────────────────────────────────────────────────

/** Collect all matches from searchText into an array. */
function collectMatches(text: string, regex: RegExp) {
  const results: Array<{ line: number; column: number; length: number }> = [];
  searchText(text, regex, (line, column, length) => {
    results.push({ line, column, length });
  });
  return results;
}

/** Build a minimal Jupyter notebook JSON string. */
function notebook(cells: Array<{ source: string | string[] }>): string {
  return JSON.stringify({
    cells: cells.map((c) => ({
      cell_type: "code",
      source: c.source,
    })),
  });
}

// ── searchText ───────────────────────────────────────────────────────────────

describe("searchText", () => {
  it("finds a simple literal match", () => {
    const results = collectMatches("hello world", /world/g);
    expect(results).toEqual([{ line: 1, column: 7, length: 5 }]);
  });

  it("reports 1-based line and column numbers", () => {
    const results = collectMatches("aaa\nbbb\nccc", /bbb/g);
    expect(results).toEqual([{ line: 2, column: 1, length: 3 }]);
  });

  it("finds multiple matches on the same line", () => {
    const results = collectMatches("aa bb aa", /aa/g);
    expect(results).toEqual([
      { line: 1, column: 1, length: 2 },
      { line: 1, column: 7, length: 2 },
    ]);
  });

  it("finds matches across multiple lines", () => {
    const results = collectMatches("foo\nbar\nfoo", /foo/g);
    expect(results).toEqual([
      { line: 1, column: 1, length: 3 },
      { line: 3, column: 1, length: 3 },
    ]);
  });

  it("handles case-insensitive matching", () => {
    const results = collectMatches("Hello HELLO", /hello/gi);
    expect(results).toEqual([
      { line: 1, column: 1, length: 5 },
      { line: 1, column: 7, length: 5 },
    ]);
  });

  it("handles zero-width regex matches without infinite loop", () => {
    const results = collectMatches("abc", /(?=a)/g);
    expect(results).toEqual([{ line: 1, column: 1, length: 0 }]);
  });

  it("handles capture groups in regex", () => {
    const results = collectMatches("foo123bar", /(\d+)/g);
    expect(results).toEqual([{ line: 1, column: 4, length: 3 }]);
  });

  it("returns no matches for empty text", () => {
    const results = collectMatches("", /foo/g);
    expect(results).toEqual([]);
  });

  it("returns no matches when pattern does not match", () => {
    const results = collectMatches("hello world", /xyz/g);
    expect(results).toEqual([]);
  });
});

// ── getNotebookCellSources ───────────────────────────────────────────────────

describe("getNotebookCellSources", () => {
  it("extracts string sources from cells", () => {
    const json = notebook([{ source: "print('hello')" }]);
    const result = getNotebookCellSources(json);
    expect(result).toEqual([{ index: 0, source: "print('hello')" }]);
  });

  it("joins array sources into a single string", () => {
    const json = notebook([{ source: ["line1\n", "line2"] }]);
    const result = getNotebookCellSources(json);
    expect(result).toEqual([{ index: 0, source: "line1\nline2" }]);
  });

  it("handles multiple cells with correct indices", () => {
    const json = notebook([{ source: "a" }, { source: "b" }, { source: "c" }]);
    const result = getNotebookCellSources(json);
    expect(result).toHaveLength(3);
    expect(result[0]).toEqual({ index: 0, source: "a" });
    expect(result[2]).toEqual({ index: 2, source: "c" });
  });

  it("returns empty array for malformed JSON", () => {
    expect(getNotebookCellSources("{invalid")).toEqual([]);
  });

  it("returns empty array when cells is not an array", () => {
    expect(getNotebookCellSources('{"cells": "not-array"}')).toEqual([]);
  });

  it("handles cells with missing source", () => {
    const json = JSON.stringify({ cells: [{ cell_type: "code" }] });
    const result = getNotebookCellSources(json);
    expect(result).toEqual([{ index: 0, source: "" }]);
  });
});

// ── reserializeCellSource ────────────────────────────────────────────────────

describe("reserializeCellSource", () => {
  it("splits source into lines with trailing newlines (Jupyter convention)", () => {
    const cell: { source: string[] } = { source: [] };
    reserializeCellSource(cell, "line1\nline2\nline3");
    expect(cell.source).toEqual(["line1\n", "line2\n", "line3"]);
  });

  it("handles single-line source", () => {
    const cell: { source: string[] } = { source: [] };
    reserializeCellSource(cell, "single line");
    expect(cell.source).toEqual(["single line"]);
  });

  it("drops trailing empty string from split", () => {
    // "line1\n" splits to ["line1", ""], Jupyter convention drops the trailing ""
    const cell: { source: string[] } = { source: [] };
    reserializeCellSource(cell, "line1\n");
    expect(cell.source).toEqual(["line1\n"]);
  });

  it("handles empty source", () => {
    const cell: { source: string[] } = { source: [] };
    reserializeCellSource(cell, "");
    expect(cell.source).toEqual([""]);
  });
});

// ── replaceInNotebookCell ────────────────────────────────────────────────────

describe("replaceInNotebookCell", () => {
  it("replaces a match at the correct position in a cell", () => {
    const json = notebook([{ source: "hello world" }]);
    const result = replaceInNotebookCell(json, 0, "earth", 1, 7, 5);
    const nb = JSON.parse(result);
    const src = nb.cells[0].source.join("");
    expect(src).toBe("hello earth");
  });

  it("preserves other cells when replacing in one", () => {
    const json = notebook([{ source: "aaa" }, { source: "bbb" }]);
    const result = replaceInNotebookCell(json, 0, "xxx", 1, 1, 3);
    const nb = JSON.parse(result);
    expect(nb.cells[0].source.join("")).toBe("xxx");
    // Unmodified cell retains its original string source (not re-serialized).
    const cell1Src = Array.isArray(nb.cells[1].source)
      ? nb.cells[1].source.join("")
      : nb.cells[1].source;
    expect(cell1Src).toBe("bbb");
  });

  it("handles multi-line cell source", () => {
    const json = notebook([{ source: ["line1\n", "find me\n", "line3"] }]);
    const result = replaceInNotebookCell(json, 0, "REPLACED", 2, 1, 4);
    const nb = JSON.parse(result);
    const src = nb.cells[0].source.join("");
    expect(src).toBe("line1\nREPLACED me\nline3");
  });

  it("returns original JSON for out-of-bounds cell index", () => {
    const json = notebook([{ source: "hello" }]);
    const result = replaceInNotebookCell(json, 5, "x", 1, 1, 1);
    expect(result).toBe(json);
  });

  it("returns original JSON for out-of-bounds line", () => {
    const json = notebook([{ source: "hello" }]);
    const result = replaceInNotebookCell(json, 0, "x", 99, 1, 1);
    expect(result).toBe(json);
  });
});

// ── replaceAllInNotebook ─────────────────────────────────────────────────────

describe("replaceAllInNotebook", () => {
  it("replaces all literal matches across cells", () => {
    const json = notebook([{ source: "aa bb aa" }, { source: "cc aa dd" }]);
    const matches = [
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 1, length: 2, cellIndex: 0 },
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 7, length: 2, cellIndex: 0 },
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 4, length: 2, cellIndex: 1 },
    ];
    const result = replaceAllInNotebook(json, matches, "aa", "XX", false, true);
    const nb = JSON.parse(result);
    expect(nb.cells[0].source.join("")).toBe("XX bb XX");
    expect(nb.cells[1].source.join("")).toBe("cc XX dd");
  });

  it("handles regex replace with capture groups", () => {
    const json = notebook([{ source: "foo123bar" }]);
    const matches = [
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 4, length: 3, cellIndex: 0 },
    ];
    const result = replaceAllInNotebook(json, matches, "(\\d)(\\d)(\\d)", "$3$2$1", true, true);
    const nb = JSON.parse(result);
    expect(nb.cells[0].source.join("")).toBe("foo321bar");
  });

  it("handles case-insensitive regex replace", () => {
    const json = notebook([{ source: "Hello HELLO hello" }]);
    const matches = [
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 1, length: 5, cellIndex: 0 },
    ];
    const result = replaceAllInNotebook(json, matches, "hello", "hi", true, false);
    const nb = JSON.parse(result);
    expect(nb.cells[0].source.join("")).toBe("hi hi hi");
  });

  it("reverse-order literal replacement prevents offset drift", () => {
    // Two matches on the same line with different lengths after replacement
    const json = notebook([{ source: "ab cd ab" }]);
    const matches = [
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 1, length: 2, cellIndex: 0 },
      { tabId: "t1", tabTitle: "T", isNotebook: true, line: 1, column: 7, length: 2, cellIndex: 0 },
    ];
    const result = replaceAllInNotebook(json, matches, "ab", "XYZ", false, true);
    const nb = JSON.parse(result);
    expect(nb.cells[0].source.join("")).toBe("XYZ cd XYZ");
  });

  it("returns original JSON for malformed input", () => {
    const result = replaceAllInNotebook("{bad", [], "a", "b", false, true);
    expect(result).toBe("{bad");
  });
});
