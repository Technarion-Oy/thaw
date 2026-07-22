// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

// Pure parsing/shape-normalisation helpers for UserPropertiesModal, split out so
// the column-index lookups and quoting are unit-testable (see
// userPropertyUtils.test.ts) — mirroring the keyPairUtils.ts split in this dir.

import type { snowflake } from "../../../wailsjs/go/models";
import type { EditableTag } from "../shared/TagsRow";
import { parseAllowedValues } from "../tag/allowedValues";

// quoteIdent double-quotes an identifier part, doubling embedded quotes — the
// client-side mirror of snowflake.QuoteIdent, used to build the quoted FQN that
// the ALTER USER builders parse back with exact case.
export const quoteIdent = (s: string) => `"${s.replace(/"/g, '""')}"`;

// A dropdown option parsed from a SHOW … result. allowedValues is populated from
// SHOW TAGS' allowed_values column (empty for policies, which have no such
// column) — a non-empty list turns the tag value field into a whitelist dropdown.
export interface NameOption { value: string; label: string; allowedValues: string[] }

// nameOptionsFromShow turns a SHOW … result (which shares the
// name / database_name / schema_name columns across POLICIES and TAGS) into
// dropdown options: the value is the quoted FQN (passed to the ALTER builders so
// mixed-case names round-trip), the label the readable dotted name, and (for
// tags) allowedValues parsed from the allowed_values column.
export function nameOptionsFromShow(res: snowflake.QueryResult | null): NameOption[] {
  const cols = res?.columns ?? [];
  const iName    = cols.indexOf("name");
  const iDb      = cols.indexOf("database_name");
  const iSc      = cols.indexOf("schema_name");
  const iAllowed = cols.indexOf("allowed_values");
  if (iName < 0) return [];
  return (res?.rows ?? []).map((r) => {
    const nm = String(r[iName]);
    const db = iDb >= 0 && r[iDb] != null ? String(r[iDb]) : "";
    const sc = iSc >= 0 && r[iSc] != null ? String(r[iSc]) : "";
    const parts = [db, sc, nm].filter(Boolean);
    const allowedValues = iAllowed >= 0 && r[iAllowed] != null ? parseAllowedValues(String(r[iAllowed])) : [];
    return { value: parts.map(quoteIdent).join("."), label: parts.join("."), allowedValues };
  });
}

// userTagsToEditable maps a GetUserTagReferences result (TAG_DATABASE /
// TAG_SCHEMA / TAG_NAME / TAG_VALUE) into removable chips. The chip key is the
// quoted FQN handed straight to UnsetUserTags.
export function userTagsToEditable(res: snowflake.QueryResult | null): EditableTag[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  const ci = (n: string) => cols.indexOf(n);
  const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"), vlI = ci("tag_value");
  if (nmI < 0) return [];
  return (res?.rows ?? []).map((row): EditableTag => {
    const tdb = dbI >= 0 && row[dbI] != null ? String(row[dbI]) : "";
    const tsc = scI >= 0 && row[scI] != null ? String(row[scI]) : "";
    const tnm = String(row[nmI] ?? "");
    const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
    return { key: qualified, name: tnm, value: vlI >= 0 && row[vlI] != null ? String(row[vlI]) : "", removable: true };
  });
}

// The three attachable user policy kinds. The value is the keyword passed to
// SetUserPolicy/UnsetUserPolicy; PolicyKindLabel is the human label.
export type PolicyKind = "AUTHENTICATION" | "PASSWORD" | "SESSION";

export const PolicyKindLabel: Record<PolicyKind, string> = {
  AUTHENTICATION: "Authentication",
  PASSWORD: "Password",
  SESSION: "Session",
};

// POLICY_REFERENCES' POLICY_KIND column → our kind keyword (other kinds, e.g.
// NETWORK_POLICY / MASKING_POLICY, are managed elsewhere and skipped).
const POLICY_KIND_MAP: Record<string, PolicyKind> = {
  AUTHENTICATION_POLICY: "AUTHENTICATION",
  PASSWORD_POLICY: "PASSWORD",
  SESSION_POLICY: "SESSION",
};

// One policy currently attached to the user. fqn is the quoted FQN (handed to
// UnsetUserPolicy's kind and shown as the chip); label is the readable name.
export interface PolicyRef { kind: PolicyKind; fqn: string; label: string }

// parsePolicyReferences maps a GetUserPolicyReferences result
// (POLICY_DB / POLICY_SCHEMA / POLICY_NAME / POLICY_KIND) into the attached
// authentication/password/session policies, skipping any other policy kind.
export function parsePolicyReferences(res: snowflake.QueryResult | null): PolicyRef[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  const ci = (n: string) => cols.indexOf(n);
  const dbI = ci("policy_db"), scI = ci("policy_schema"), nmI = ci("policy_name"), kI = ci("policy_kind");
  if (nmI < 0 || kI < 0) return [];
  const out: PolicyRef[] = [];
  for (const r of res?.rows ?? []) {
    const kind = POLICY_KIND_MAP[String(r[kI] ?? "").toUpperCase()];
    if (!kind) continue;
    const db = dbI >= 0 && r[dbI] != null ? String(r[dbI]) : "";
    const sc = scI >= 0 && r[scI] != null ? String(r[scI]) : "";
    const nm = String(r[nmI] ?? "");
    const parts = [db, sc, nm].filter(Boolean);
    out.push({ kind, fqn: parts.map(quoteIdent).join("."), label: parts.join(".") });
  }
  return out;
}

// One enrolled MFA factor from SHOW MFA METHODS FOR USER. `name` is the
// system-generated identifier passed to RemoveUserMfaMethod (NOT the type).
export interface MfaMethod { name: string; type: string; comment: string; lastUsed: string }

// parseMfaMethods maps a SHOW MFA METHODS result into enrolled-factor rows. The
// name column is the removal identifier; type/comment/last_used are display-only
// (Duo rows report empty comment/last_used per Snowflake).
export function parseMfaMethods(res: snowflake.QueryResult | null): MfaMethod[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  const ci = (n: string) => cols.indexOf(n);
  const nI = ci("name"), tI = ci("type"), cI = ci("comment"), lI = ci("last_used");
  if (nI < 0) return [];
  return (res?.rows ?? [])
    .map((r): MfaMethod => ({
      name: String(r[nI] ?? ""),
      type: tI >= 0 && r[tI] != null ? String(r[tI]) : "",
      comment: cI >= 0 && r[cI] != null ? String(r[cI]) : "",
      lastUsed: lI >= 0 && r[lI] != null ? String(r[lI]) : "",
    }))
    .filter((m) => m.name !== "");
}
