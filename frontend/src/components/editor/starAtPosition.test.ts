import { describe, it, expect } from "vitest";
import { starAtPosition } from "./sqlEditorUtils";

// A stub model — starAtPosition only reads the one line it's given.
const at = (line: string, column: number) =>
  starAtPosition({ getLineContent: () => line }, { lineNumber: 1, column });

describe("starAtPosition", () => {
  it("detects a bare select-list star (cursor on either edge)", () => {
    const line = "SELECT * FROM t";
    const star = { startLineNumber: 1, endLineNumber: 1, startColumn: 8, endColumn: 9 };
    expect(at(line, 8)?.range).toEqual(star); // left edge
    expect(at(line, 9)?.range).toEqual(star); // right edge
    expect(at(line, 8)?.alias).toBeNull();
  });

  it("detects alias.* and captures the alias + full token range", () => {
    const line = "SELECT t.* FROM tbl t";
    const r = at(line, 11); // right edge of the star
    expect(r?.alias).toBe("t");
    expect(r?.range.startColumn).toBe(8);  // covers "t.*"
    expect(r?.range.endColumn).toBe(11);
  });

  it("skips function-argument stars like COUNT(*)", () => {
    expect(at("SELECT COUNT(*) FROM t", 15)).toBeNull();
  });

  it("returns null when the cursor isn't on a star", () => {
    expect(at("SELECT a FROM t", 8)).toBeNull();
  });
});
