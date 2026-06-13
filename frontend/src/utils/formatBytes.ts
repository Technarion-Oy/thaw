// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
