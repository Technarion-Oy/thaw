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

import { useState, useMemo } from "react";
import { Modal, Segmented } from "antd";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  ScatterChart,
  Scatter,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import type { Row } from "@tanstack/react-table";
import type { SelectionRange } from "../../store/gridStore";
import { visualToOriginalIndex } from "./columnOrderUtils";

type ChartType = "bar" | "line" | "scatter";

interface Props {
  tableRows: Row<unknown[]>[];
  columns: string[];
  selectionRange: SelectionRange;
  /** Maps a visual column position to its original SELECT index. null = default
   *  order. selectionRange columns are visual positions; chart data reads
   *  result columns via this map so it follows the on-screen arrangement. */
  visualToOriginal: number[] | null;
  onClose: () => void;
}

// Detect if a column is primarily numeric by sampling values.
function isNumericColumn(values: unknown[]): boolean {
  let numericCount = 0;
  let total = 0;
  for (const v of values) {
    if (v === null || v === undefined) continue;
    total++;
    if (!isNaN(Number(v)) && v !== "" && v !== true && v !== false) numericCount++;
  }
  return total > 0 && numericCount / total > 0.7;
}

const CHART_COLORS = [
  "#1677ff", "#52c41a", "#fa8c16", "#eb2f96", "#722ed1",
  "#13c2c2", "#f5222d", "#faad14",
];

export default function QuickChartModal({ tableRows, columns, selectionRange, visualToOriginal, onClose }: Props) {
  const [chartType, setChartType] = useState<ChartType>("bar");

  const { data, xKey, valueKeys } = useMemo(() => {
    const minRow = Math.min(selectionRange.startRow, selectionRange.endRow);
    const maxRow = Math.max(selectionRange.startRow, selectionRange.endRow);
    const minCol = Math.min(selectionRange.startCol, selectionRange.endCol);
    const maxCol = Math.max(selectionRange.startCol, selectionRange.endCol);

    // selectionRange columns are visual positions; translate each to its
    // original SELECT index (in visual, left-to-right order) for data reads.
    const colIndices: number[] = [];
    for (let c = minCol; c <= maxCol; c++) colIndices.push(visualToOriginalIndex(visualToOriginal, c));

    const names = colIndices.map((c) => columns[c] ?? `Col ${c}`);

    // Sample values per column to detect types
    const sampleValues = colIndices.map((c) => {
      const vals: unknown[] = [];
      for (let r = minRow; r <= maxRow; r++) {
        vals.push(tableRows[r]?.original[c]);
      }
      return vals;
    });

    const numericFlags = sampleValues.map((vals) => isNumericColumn(vals));

    // Pick x-axis: first non-numeric column, or first column if all numeric
    let xIdx = numericFlags.findIndex((n) => !n);
    if (xIdx < 0) xIdx = 0;

    const xColIndex = colIndices[xIdx];
    const xName = names[xIdx];

    // Value columns: all numeric columns except the x-axis
    const valCols: { index: number; name: string }[] = [];
    for (let i = 0; i < colIndices.length; i++) {
      if (i === xIdx) continue;
      if (numericFlags[i]) {
        valCols.push({ index: colIndices[i], name: names[i] });
      }
    }

    // When all columns are numeric, xIdx is 0 and was excluded from valCols above.
    // Add it back so at least one value column is available for charting.
    if (valCols.length === 0 && numericFlags[xIdx]) {
      valCols.push({ index: colIndices[xIdx], name: names[xIdx] });
    }

    // Build chart data
    const rows: Record<string, unknown>[] = [];
    for (let r = minRow; r <= maxRow; r++) {
      const row = tableRows[r]?.original;
      if (!row) continue;
      const entry: Record<string, unknown> = {
        [xName]: row[xColIndex] != null ? String(row[xColIndex]) : `Row ${r}`,
      };
      for (const vc of valCols) {
        const val = row[vc.index];
        entry[vc.name] = val != null ? Number(val) : null;
      }
      rows.push(entry);
    }

    return {
      data: rows,
      xKey: xName,
      valueKeys: valCols.map((vc) => vc.name),
    };
  }, [tableRows, columns, selectionRange, visualToOriginal]);

  const renderChart = () => {
    if (valueKeys.length === 0) {
      return (
        <div style={{ padding: 24, color: "var(--text-muted)", textAlign: "center" }}>
          No numeric columns found in selection for charting.
        </div>
      );
    }

    const common = { data, margin: { top: 5, right: 20, bottom: 5, left: 20 } };

    switch (chartType) {
      case "bar":
        return (
          <ResponsiveContainer width="100%" height={360}>
            <BarChart {...common}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey={xKey} tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              {valueKeys.map((key, i) => (
                <Bar key={key} dataKey={key} fill={CHART_COLORS[i % CHART_COLORS.length]} />
              ))}
            </BarChart>
          </ResponsiveContainer>
        );
      case "line":
        return (
          <ResponsiveContainer width="100%" height={360}>
            <LineChart {...common}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey={xKey} tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              {valueKeys.map((key, i) => (
                <Line
                  key={key}
                  type="monotone"
                  dataKey={key}
                  stroke={CHART_COLORS[i % CHART_COLORS.length]}
                  dot={data.length < 50}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        );
      case "scatter":
        return (
          <ResponsiveContainer width="100%" height={360}>
            <ScatterChart {...common}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey={xKey} tick={{ fontSize: 11 }} name={xKey} />
              <YAxis
                tick={{ fontSize: 11 }}
                dataKey={valueKeys[0]}
                name={valueKeys[0]}
              />
              <Tooltip cursor={{ strokeDasharray: "3 3" }} />
              <Legend />
              {valueKeys.map((key, i) => (
                <Scatter
                  key={key}
                  name={key}
                  dataKey={key}
                  fill={CHART_COLORS[i % CHART_COLORS.length]}
                />
              ))}
            </ScatterChart>
          </ResponsiveContainer>
        );
    }
  };

  return (
    <Modal
      title="Quick Chart"
      open
      onCancel={onClose}
      footer={null}
      width={640}
    >
      <div style={{ marginBottom: 16, display: "flex", justifyContent: "center" }}>
        <Segmented
          value={chartType}
          onChange={(v) => setChartType(v as ChartType)}
          options={[
            { label: "Bar", value: "bar" },
            { label: "Line", value: "line" },
            { label: "Scatter", value: "scatter" },
          ]}
        />
      </div>
      {renderChart()}
      <div style={{ marginTop: 8, fontSize: 11, color: "var(--text-muted)", textAlign: "center" }}>
        X-axis: {xKey} | Value columns: {valueKeys.join(", ") || "none"} | {data.length} data points
      </div>
    </Modal>
  );
}
