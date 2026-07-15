// SPDX-License-Identifier: GPL-3.0-or-later

// Formats a byte count into a human-readable string with binary (1024) units
// and a single decimal place (e.g. "1.5 KB", "2.0 MB"). Used by the stage
// browsers (StageBrowserModal, the External Table location picker).
//
// Note: other components intentionally keep their own variants with different
// rounding / zero-handling (e.g. ExplainModal renders "—" for zero, the table
// object-summaries use a log-based formatter with TB) — they are not collapsed
// onto this helper to preserve their existing display.
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
