// SPDX-License-Identifier: GPL-3.0-or-later

/** Formats an ISO timestamp as a localized date-time with seconds; "—" when empty, the input when unparsable. */
export function formatTime(ts: string): string {
  if (!ts) return "—";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toLocaleString(undefined, {
    year: "numeric", month: "short", day: "numeric",
    hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

/** Human-readable elapsed time between two ISO timestamps (`42s`, `3m 5s`, `1h 2m`); "—" when either is missing/invalid or end < start. */
export function formatDuration(startTs: string, endTs: string): string {
  if (!startTs || !endTs) return "—";
  const start = new Date(startTs);
  const end = new Date(endTs);
  if (isNaN(start.getTime()) || isNaN(end.getTime())) return "—";
  const diffMs = end.getTime() - start.getTime();
  if (diffMs < 0) return "—";
  const totalSecs = Math.floor(diffMs / 1000);
  if (totalSecs < 60) return `${totalSecs}s`;
  const mins = Math.floor(totalSecs / 60);
  const secs = totalSecs % 60;
  if (mins < 60) return `${mins}m ${secs}s`;
  const hrs = Math.floor(mins / 60);
  const remMins = mins % 60;
  return `${hrs}h ${remMins}m`;
}
