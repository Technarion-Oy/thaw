// SPDX-License-Identifier: GPL-3.0-or-later

// Shared error-formatting helper for the Properties-modal family. Lives in its
// own module so both PropertyRows and ConfirmSwitch can import it without a
// circular dependency.

// Strip gosnowflake noise — show only the human-readable part after the last
// ":" (e.g. "003001 (42501): SQL access control error:\nInsufficient
// privileges…" → "Insufficient privileges…").
export function friendlyError(e: unknown): string {
  const raw = String(e);
  const priv = raw.match(/Insufficient privileges[^\n]*/i);
  if (priv) return priv[0].trim();
  // The greedy [\s\S]* prefix pushes the match to the LAST colon; capture
  // what follows it (must start with a non-space so an empty tail falls back).
  const m = raw.match(/^[\s\S]*:\s*(\S[\s\S]*)$/);
  return m ? m[1].trim() : raw;
}
