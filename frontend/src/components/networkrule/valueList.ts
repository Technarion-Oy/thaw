// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration
//
// Parsing / serialization for a network rule's VALUE_LIST, shared by the
// properties modal and unit-tested in valueList.test.ts.

// q1 quotes a string as a SQL single-quoted literal (doubling embedded quotes).
export function q1(s: string): string {
  return "'" + s.replace(/'/g, "''") + "'";
}

// parseValueList parses the `value_list` cell from `DESCRIBE NETWORK RULE` into a
// list of identifiers.
//
// Snowflake does not document this column's exact format. Two shapes are handled
// so the parse → edit → re-serialize round-trip can never corrupt the rule:
//   - a JSON array (the form many Snowflake list-valued columns use, possibly
//     multiline), e.g. `["example.com:443","company.com:80"]`; and
//   - a bare comma-separated string, e.g. `example.com:443,company.com:80`.
//
// A leading `[` selects the JSON path; anything that fails to parse as a JSON
// array falls back to comma-splitting. An unparseable / empty / null cell yields
// an empty list.
export function parseValueList(raw: string): string[] {
  const s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null") return [];
  if (s.startsWith("[")) {
    try {
      const parsed = JSON.parse(s);
      if (Array.isArray(parsed)) {
        return parsed.map((v) => String(v).trim()).filter((v) => v !== "");
      }
    } catch {
      /* not valid JSON — fall through to comma-split */
    }
  }
  return s.split(",").map((v) => v.trim()).filter((v) => v !== "");
}

// setValueListClause builds the ALTER NETWORK RULE clause that replaces the whole
// list. SET VALUE_LIST is not additive, so every edit resends the full list; an
// empty list maps to UNSET VALUE_LIST.
export function setValueListClause(values: string[]): string {
  if (values.length === 0) return "UNSET VALUE_LIST";
  return `SET VALUE_LIST = (${values.map(q1).join(", ")})`;
}
