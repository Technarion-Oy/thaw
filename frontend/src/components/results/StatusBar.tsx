// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: SQL Editor & Diagnostics

import { useMemo } from "react";
import { useGridStore } from "../../store/gridStore";
import type { QueryResult } from "../../store/queryStore";

interface Props {
  result: QueryResult;
}

function formatNumber(n: number): string {
  if (Number.isInteger(n)) return n.toLocaleString();
  return n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
}

export default function StatusBar({ result }: Props) {
  const selectionRange = useGridStore((s) => s.selectionRange);

  const stats = useMemo(() => {
    if (!selectionRange) return null;

    const { startRow, endRow, startCol, endCol } = selectionRange;
    const minRow = Math.min(startRow, endRow);
    const maxRow = Math.max(startRow, endRow);
    const minCol = Math.min(startCol, endCol);
    const maxCol = Math.max(startCol, endCol);

    const numericValues: number[] = [];
    let cellCount = 0;

    for (let r = minRow; r <= maxRow; r++) {
      const row = result.rows[r];
      if (!row) continue;
      for (let c = minCol; c <= maxCol; c++) {
        cellCount++;
        const val = row[c];
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
    const min = Math.min(...numericValues);
    const max = Math.max(...numericValues);

    return {
      sum,
      avg,
      count: cellCount,
      numericCount: numericValues.length,
      min,
      max,
    };
  }, [selectionRange, result.rows]);

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
