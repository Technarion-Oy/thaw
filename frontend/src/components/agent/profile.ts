// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

// Shared PROFILE JSON helpers for the Agent create + properties modals. The
// PROFILE clause of CREATE/ALTER AGENT takes a JSON object string with the keys
// display_name, avatar, and color.

export interface AgentProfile {
  display_name: string;
  avatar: string;
  color: string;
}

// buildProfileJson assembles a compact JSON object string from the three profile
// fields, including only the non-blank ones. Returns "" when all are blank so the
// builder omits the PROFILE clause entirely. JSON.stringify safely escapes any
// quotes/backslashes the user typed.
export function buildProfileJson(p: AgentProfile): string {
  const obj: Record<string, string> = {};
  if (p.display_name.trim() !== "") obj.display_name = p.display_name.trim();
  if (p.avatar.trim() !== "") obj.avatar = p.avatar.trim();
  if (p.color.trim() !== "") obj.color = p.color.trim();
  return Object.keys(obj).length === 0 ? "" : JSON.stringify(obj);
}

// parseProfileJson reads an existing PROFILE JSON string (as reported by SHOW
// AGENTS / DESCRIBE AGENT) back into the three fields, tolerating blanks and
// malformed JSON (returns all-empty on parse failure).
export function parseProfileJson(raw: string): AgentProfile {
  const empty: AgentProfile = { display_name: "", avatar: "", color: "" };
  const s = (raw ?? "").trim();
  if (s === "") return empty;
  try {
    const o = JSON.parse(s);
    if (o && typeof o === "object") {
      return {
        display_name: typeof o.display_name === "string" ? o.display_name : "",
        avatar: typeof o.avatar === "string" ? o.avatar : "",
        color: typeof o.color === "string" ? o.color : "",
      };
    }
  } catch {
    // fall through to empty on malformed JSON
  }
  return empty;
}
