/**
 * SQL formatter for the Thaw editor — Snowflake dialect.
 *
 * Uses sql-formatter for structural formatting (indentation, line breaks,
 * comma / operator placement, CTE layout) and applies a custom token-level
 * casing pass on top for separate keyword / identifier / function control.
 */

import { format as sfFormat } from "sql-formatter";
import { ApplySqlCasing } from "../../wailsjs/go/main/App";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface EditorPrefs {
  keywordCase:      "UPPER" | "lower" | "Title" | "Preserve";
  identifierCase:   "Preserve" | "UPPER" | "lower";
  functionCase:     "UPPER" | "lower";
  indentStyle:      "spaces" | "tabs";
  indentSize:       2 | 4;
  commaPosition:    "trailing" | "leading";
  operatorPosition: "before" | "after";
}

export const DEFAULT_EDITOR_PREFS: EditorPrefs = {
  keywordCase:      "UPPER",
  identifierCase:   "Preserve",
  functionCase:     "UPPER",
  indentStyle:      "spaces",
  indentSize:       2,
  commaPosition:    "trailing",
  operatorPosition: "before",
};

// ── Main export ───────────────────────────────────────────────────────────────

/**
 * Format a SQL string using the given editor preferences.
 *
 * Structural formatting (line breaks, indentation, CTE layout, comma / operator
 * placement) is handled by sql-formatter with the Snowflake dialect.
 * Token-level casing is applied by a custom post-processor so keyword, identifier,
 * and function casing can be controlled independently.
 */
export async function formatSQL(sql: string, prefs: EditorPrefs): Promise<string> {
  if (!sql.trim()) return sql;

  try {
    let structured = sfFormat(sql, {
      language: "snowflake",
      // For "Preserve" mode pass through sql-formatter's own preserve so the
      // original keyword casing survives before our no-op ApplySqlCasing pass.
      // For all other modes always emit UPPER so the Go casing pass can identify keywords.
      keywordCase: prefs.keywordCase === "Preserve" ? "preserve" : "upper",
      tabWidth:             prefs.indentSize,
      useTabs:              prefs.indentStyle === "tabs",
      logicalOperatorNewline: prefs.operatorPosition === "before" ? "before" : "after",
      expressionWidth:      60,
      linesBetweenQueries:  1,
    });

    // sql-formatter removed commaPosition support in v15+. Apply leading
    // commas as a post-processing step: move each trailing comma to the
    // start of the following line so `col1,\n  col2` → `col1\n, col2`.
    if (prefs.commaPosition === "leading") {
      structured = structured.replace(/,\n\s*/g, "\n, ");
    }

    return await ApplySqlCasing(structured, prefs.keywordCase, prefs.identifierCase, prefs.functionCase);
  } catch {
    // If the formatter fails (e.g. on a partial/invalid statement), return the
    // original SQL unchanged so the editor never ends up with corrupted content.
    return sql;
  }
}
