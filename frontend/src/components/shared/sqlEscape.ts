// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * Escape a free-text value for use inside a single-quoted SQL string literal —
 * the frontend counterpart of the backend's `snowflake.EscapeTextLit`.
 *
 * Snowflake treats the backslash as an escape character within single-quoted
 * literals, so a lone backslash must be doubled or it is swallowed (e.g.
 * `C:\temp` would otherwise read as `C:temp`, and a value ending in `\` would
 * escape the closing quote). Single-quotes are doubled as well. Backslashes are
 * escaped first so the doubled quotes are not themselves mistaken for an escape
 * sequence. Does NOT add the surrounding single-quote delimiters.
 *
 * Use this for human-entered text (comments, schedules); keep it in sync with
 * the backend `EscapeTextLit` — this is the single shared copy so the callers
 * can't drift.
 */
export function escapeTextLit(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/'/g, "''");
}

/**
 * Wrap `s` in single-quotes for use as a free-text SQL string literal, escaping
 * via {@link escapeTextLit}. The counterpart of the backend's
 * `snowflake.QuoteTextLit`.
 */
export function quoteTextLit(s: string): string {
  return "'" + escapeTextLit(s) + "'";
}
