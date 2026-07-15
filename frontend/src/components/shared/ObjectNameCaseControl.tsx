// SPDX-License-Identifier: GPL-3.0-or-later

import { Alert, Radio } from "antd";
import { GetSnowflakeKeywords } from "../../../wailsjs/go/sqleditor/Service";

// ── Identifier helpers ────────────────────────────────────────────────────────

/**
 * Regex for a valid Snowflake bare (unquoted) identifier:
 * starts with a letter or underscore, followed by letters, digits, _ or $.
 * Max 255 characters total (Snowflake limit).
 */
export const UNQUOTED_IDENT_RE = /^[A-Za-z_][A-Za-z0-9_$]{0,254}$/;

/**
 * Module-level cache of Snowflake reserved keywords (uppercased).
 * Loaded once from the backend via GetSnowflakeKeywords() so that
 * needsQuoting() uses the same list as the Go NeedsQuoting() function.
 * Starts empty; populated asynchronously on first module import.
 */
let _reservedKeywords: Set<string> = new Set();

(function loadReservedKeywords() {
  GetSnowflakeKeywords()
    .then((kws) => {
      _reservedKeywords = new Set((kws ?? []).map((k) => k.toUpperCase()));
    })
    .catch(() => {
      // Non-fatal: falls back to character-only check until next load attempt
    });
})();

/**
 * Returns true when `name` cannot be expressed as a bare (unquoted)
 * Snowflake identifier and therefore MUST be double-quoted.
 *
 * Mirrors the Go backend NeedsQuoting() function: checks both the character
 * pattern (must match UNQUOTED_IDENT_RE) and whether the name is a Snowflake
 * reserved keyword. The reserved keyword list is loaded from the backend on
 * first module import; until it arrives the character-pattern check alone
 * is used (adequate for the vast majority of inputs).
 */
export function needsQuoting(name: string): boolean {
  if (name.length === 0 || !UNQUOTED_IDENT_RE.test(name)) return true;
  return _reservedKeywords.has(name.toUpperCase());
}

/**
 * Wraps `name` in double-quotes and escapes any embedded double-quotes
 * by doubling them, per the Snowflake SQL convention.
 */
export function quoteIdent(name: string): string {
  return '"' + name.replace(/"/g, '""') + '"';
}

/**
 * Returns the SQL token for a Snowflake object identifier.
 *
 * - If `caseSensitive` is true, or the name cannot be expressed as a bare
 *   identifier (contains special characters, starts with a digit, etc.),
 *   the name is wrapped in double-quotes.
 * - Otherwise the name is returned as-is (Snowflake will uppercase it on
 *   storage — the normal case-insensitive behaviour).
 */
export function identToken(name: string, caseSensitive: boolean): string {
  return caseSensitive || needsQuoting(name) ? quoteIdent(name) : name;
}

// ── Component ─────────────────────────────────────────────────────────────────

interface Props {
  /** Current value of the name field — used only for quoting detection. */
  name: string;
  caseSensitive: boolean;
  onCaseSensitiveChange: (v: boolean) => void;
  /**
   * Pass the result of GetQuotedIdentifiersIgnoreCase() fetched on modal
   * open. When true, an amber warning is shown explaining that
   * double-quoting does not preserve case for this session.
   */
  quotedIdentifiersIgnoreCase: boolean;
}

/**
 * Renders the case-sensitivity Radio group and any relevant warnings for a
 * Snowflake object name field.
 *
 * Place this component immediately after the `<Input>` (or `<Form.Item>`)
 * that collects the object name. It does NOT render the input itself — that
 * stays in the parent modal so existing form layouts and validation are
 * undisturbed.
 *
 * Behaviour:
 * - "Case insensitive" radio: name is used unquoted; Snowflake stores it
 *   uppercase. Disabled (and locked to "Case sensitive") when the name
 *   contains characters that require quoting (e.g. hyphens, spaces, leading
 *   digit).
 * - "Case sensitive" radio: name is wrapped in double-quotes in the
 *   generated SQL, preserving exact case.
 * - Amber inline message when quoting is forced by the name content.
 * - Amber Alert when QUOTED_IDENTIFIERS_IGNORE_CASE is TRUE for the session,
 *   warning that double-quoting will not actually preserve case.
 */
export default function ObjectNameCaseControl({
  name,
  caseSensitive,
  onCaseSensitiveChange,
  quotedIdentifiersIgnoreCase,
}: Props) {
  const forced = needsQuoting(name);
  const effective = caseSensitive || forced;

  return (
    <div style={{ marginTop: 6 }}>
      <Radio.Group
        value={effective ? "sensitive" : "insensitive"}
        onChange={(e) => onCaseSensitiveChange(e.target.value === "sensitive")}
        style={{ marginBottom: forced || quotedIdentifiersIgnoreCase ? 8 : 0 }}
      >
        <Radio value="insensitive" disabled={forced}>
          Case insensitive
          <span style={{ marginLeft: 4, color: "var(--text-muted)", fontSize: 11 }}>
            (unquoted, stored as uppercase)
          </span>
        </Radio>
        <Radio value="sensitive">
          Case sensitive
          <span style={{ marginLeft: 4, color: "var(--text-muted)", fontSize: 11 }}>
            (double-quoted, preserves case)
          </span>
        </Radio>
      </Radio.Group>

      {forced && (
        <div
          style={{
            fontSize: 12,
            color: "#faad14",
            marginBottom: quotedIdentifiersIgnoreCase ? 8 : 0,
            display: "flex",
            gap: 6,
          }}
        >
          <span>⚠</span>
          <span>
            Name requires quoting — it is a reserved keyword or contains characters
            not allowed in unquoted identifiers (must start with a letter or _,
            contain only letters, digits, _ or $). Case-insensitive mode is unavailable.
          </span>
        </div>
      )}

      {quotedIdentifiersIgnoreCase && (
        <Alert
          type="warning"
          showIcon
          style={{ fontSize: 12 }}
          message={
            <span>
              <strong>QUOTED_IDENTIFIERS_IGNORE_CASE</strong> is enabled for this
              session — Snowflake treats all identifiers as case-insensitive regardless
              of quoting. The name will be stored uppercase even when double-quoted.
            </span>
          }
        />
      )}
    </div>
  );
}
