// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * Shared file-path-template metadata for the DDL export dialogs
 * (`ExportOptionsModal` and `ExportPathFormatModal`).
 *
 * The placeholder list must stay in sync with `(*ddl.Object).FilePathFor` on
 * the Go side — it is the only thing that substitutes them.
 */

export const DEFAULT_EXPORT_PATH_TEMPLATE =
  "{database}/{schema}/{object_type}/{object_name}.sql";

export interface TemplateVariable {
  /** The placeholder as it appears in the template, braces included. */
  name: string;
  /** Short description shown next to the insert button. */
  desc: string;
  /** Value substituted in the preview. */
  example: string;
}

export const PATH_TEMPLATE_VARIABLES: TemplateVariable[] = [
  { name: "{database}", desc: "Sanitized database name", example: "MY_DATABASE" },
  { name: "{schema}", desc: "Sanitized schema name", example: "PUBLIC" },
  { name: "{object_type}", desc: "Object type directory (tables, views, …)", example: "tables" },
  { name: "{object_name}", desc: "Sanitized object name", example: "MY_TABLE" },
];

/** Substitutes example values into a template for the live preview. */
export function applyTemplate(template: string): string {
  let result = template.trim() || DEFAULT_EXPORT_PATH_TEMPLATE;
  for (const v of PATH_TEMPLATE_VARIABLES) {
    result = result.split(v.name).join(v.example);
  }
  return result;
}

/** Extension every exported DDL file must carry. */
export const REQUIRED_EXTENSION = ".sql";

/**
 * Returns the reason the template cannot be used, or `null` when it is valid.
 *
 * A blank template is valid — it means "use the default", which already ends in
 * `.sql`. Everything else must carry the extension itself: the export writes
 * the rendered path verbatim, so a template without it produces extension-less
 * files. The extension is *not* appended for the user on purpose — silently
 * adding it turns a template that already ends in `.sql` into `…​.sql.sql`.
 */
export function validateTemplate(template: string): string | null {
  const trimmed = template.trim();
  if (trimmed === "") return null;
  if (!trimmed.toLowerCase().endsWith(REQUIRED_EXTENSION)) {
    return `Template must end in ${REQUIRED_EXTENSION} (e.g. {object_name}${REQUIRED_EXTENSION}).`;
  }
  return null;
}

export interface Insertion {
  /** The template with the placeholder spliced in. */
  value: string;
  /** Where the caret belongs afterwards — just past the inserted text. */
  caret: number;
}

/**
 * Splices `placeholder` into `value` at the given selection, replacing whatever
 * the selection covers. A `null` start (the input is not focused, so there is
 * no meaningful caret) appends instead; out-of-range offsets are clamped, so a
 * stale selection from a previous value can never produce a mangled template.
 */
export function insertPlaceholder(
  value: string,
  placeholder: string,
  selectionStart: number | null,
  selectionEnd: number | null = selectionStart,
): Insertion {
  const clamp = (n: number) => Math.min(Math.max(n, 0), value.length);
  const start = selectionStart == null ? value.length : clamp(selectionStart);
  const end = selectionEnd == null ? start : Math.max(start, clamp(selectionEnd));
  return {
    value: value.slice(0, start) + placeholder + value.slice(end),
    caret: start + placeholder.length,
  };
}
