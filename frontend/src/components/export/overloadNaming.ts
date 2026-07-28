// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * Overloaded FUNCTION / PROCEDURE file-naming strategies for DDL export.
 *
 * Values must match the `ddl.OverloadNaming` constants on the Go side; an
 * unknown or empty value falls back to the default (`argtypes`) there, so the
 * frontend never has to validate what it loaded from config.
 */
export type OverloadNaming = "argtypes" | "signature" | "grouped";

export const DEFAULT_OVERLOAD_NAMING: OverloadNaming = "argtypes";

export const OVERLOAD_NAMING_OPTIONS: {
  value: OverloadNaming;
  label: string;
  /** Example filenames for FOO(X VARCHAR(16)) and FOO(X VARCHAR(256)). */
  example: string;
  hint: string;
}[] = [
  {
    value: "argtypes",
    label: "Argument types",
    example: "FOO__VARCHAR.sql, FOO__VARCHAR_2.sql",
    hint: "Argument type list without size qualifiers. Overloads that differ only in a size get a numbered suffix.",
  },
  {
    value: "signature",
    label: "Full signature",
    example: "FOO__VARCHAR_16.sql, FOO__VARCHAR_256.sql",
    hint: "Argument type list with size qualifiers kept, so size-only overloads land in separate files.",
  },
  {
    value: "grouped",
    label: "One file per name",
    example: "FOO.sql (both overloads)",
    hint: "All overloads of a name share one file, ordered by signature.",
  },
];

/** Falls back to the default for an empty or unrecognized stored value. */
export function normalizeOverloadNaming(value: string | undefined): OverloadNaming {
  return OVERLOAD_NAMING_OPTIONS.some((o) => o.value === value)
    ? (value as OverloadNaming)
    : DEFAULT_OVERLOAD_NAMING;
}
