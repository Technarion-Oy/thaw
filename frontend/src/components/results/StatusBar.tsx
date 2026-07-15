// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useMemo } from "react";
import { useGridStore } from "../../store/gridStore";
import { visualToOriginalIndex } from "./columnOrderUtils";

function formatNumber(n: number): string {
  if (Number.isInteger(n)) return n.toLocaleString();
  return n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
}

export default function StatusBar() {
  const selectionRange = useGridStore((s) => s.selectionRange);
  const tableRows = useGridStore((s) => s.tableRows);
  const columnVisualOrder = useGridStore((s) => s.columnVisualOrder);

  const stats = useMemo(() => {
    if (!selectionRange || !tableRows) return null;

    const { startRow, endRow, startCol, endCol } = selectionRange;
    const minRow = Math.min(startRow, endRow);
    const maxRow = Math.max(startRow, endRow);
    const minCol = Math.min(startCol, endCol);
    const maxCol = Math.max(startCol, endCol);

    const numericValues: number[] = [];
    let cellCount = 0;

    for (let r = minRow; r <= maxRow; r++) {
      const row = tableRows[r];
      if (!row) continue;
      const orig = row.original;
      // selectionRange columns are visual positions; translate to the original
      // SELECT index before reading the row so aggregations cover the columns
      // the user actually selected (reorder/pinning aware).
      for (let c = minCol; c <= maxCol; c++) {
        cellCount++;
        const val = orig[visualToOriginalIndex(columnVisualOrder, c)];
        if (val === null || val === undefined) continue;
        const num = Number(val);
        if (!isNaN(num) && val !== "" && val !== true && val !== false) {
          numericValues.push(num);
        }
      }
    }

    if (numericValues.length === 0) return null;

    const sum = numericValues.reduce((a, b) => a + b, 0);
    const avg = sum / numericValues.length;
    const min = numericValues.reduce((a, b) => Math.min(a, b), Infinity);
    const max = numericValues.reduce((a, b) => Math.max(a, b), -Infinity);

    return {
      sum,
      avg,
      count: cellCount,
      numericCount: numericValues.length,
      min,
      max,
    };
  }, [selectionRange, tableRows, columnVisualOrder]);

  if (!stats) return null;

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 16,
        padding: "3px 12px",
        background: "var(--bg-raised)",
        borderTop: "1px solid var(--border)",
        fontSize: 11,
        color: "var(--text-muted)",
        flexShrink: 0,
        overflow: "hidden",
      }}
    >
      <span>
        <strong>Sum:</strong> {formatNumber(stats.sum)}
      </span>
      <span>
        <strong>Avg:</strong> {formatNumber(stats.avg)}
      </span>
      <span>
        <strong>Count:</strong> {stats.count}
      </span>
      <span>
        <strong>Min:</strong> {formatNumber(stats.min)}
      </span>
      <span>
        <strong>Max:</strong> {formatNumber(stats.max)}
      </span>
    </div>
  );
}
