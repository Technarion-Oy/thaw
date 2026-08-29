// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

/**
 * Quoted-identifier helpers shared by the semantic-view create form. The form's
 * pickers produce fully-qualified references that the Go builder emits verbatim
 * (logical table names, Cortex search services), and the Cortex picker has to
 * read its schema/service back out of one.
 */

const esc = (s: string) => s.replace(/"/g, '""');

/** Builds a `"db"."schema"."name"` reference; blank name yields "". */
export function quoteFqn(db: string, schema: string, name: string): string {
  return name ? `"${esc(db)}"."${esc(schema)}"."${esc(name)}"` : "";
}

/**
 * Splits a `"db"."schema"."name"` reference back into its parts, undoubling the
 * escaped quotes. Anything not in that exact shape (including an empty
 * reference) yields blanks.
 */
export function parseQuotedFqn(ref: string): { db: string; schema: string; name: string } {
  const m = ref.match(/^"((?:[^"]|"")*)"\."((?:[^"]|"")*)"\."((?:[^"]|"")*)"$/);
  if (!m) return { db: "", schema: "", name: "" };
  const unesc = (s: string) => s.replace(/""/g, '"');
  return { db: unesc(m[1]), schema: unesc(m[2]), name: unesc(m[3]) };
}
