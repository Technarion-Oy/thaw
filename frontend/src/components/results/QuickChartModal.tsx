// SPDX-License-Identifier: GPL-3.0-or-later
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
import type { ResultGridFeatures } from "../../utils/tableFeatures";
import type { SelectionRange } from "../../store/gridStore";
import { buildQuickChartData } from "./quickChartUtils";

type ChartType = "bar" | "line" | "scatter";

interface Props {
  tableRows: Row<ResultGridFeatures, unknown[]>[];
  columns: string[];
  selectionRange: SelectionRange;
  /** Maps a visual column position to its original SELECT index. null = default
   *  order. selectionRange columns are visual positions; chart data reads
   *  result columns via this map so it follows the on-screen arrangement. */
  visualToOriginal: number[] | null;
  onClose: () => void;
}

const CHART_COLORS = [
  "#1677ff", "#52c41a", "#fa8c16", "#eb2f96", "#722ed1",
  "#13c2c2", "#f5222d", "#faad14",
];

export default function QuickChartModal({ tableRows, columns, selectionRange, visualToOriginal, onClose }: Props) {
  const [chartType, setChartType] = useState<ChartType>("bar");

  const { data, xKey, valueKeys, totalRows } = useMemo(
    () => buildQuickChartData(tableRows, columns, selectionRange, visualToOriginal),
    [tableRows, columns, selectionRange, visualToOriginal],
  );
  const reduced = data.length < totalRows;

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
                <Bar key={key} dataKey={key} fill={CHART_COLORS[i % CHART_COLORS.length]} isAnimationActive={!reduced} />
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
                  isAnimationActive={!reduced}
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
                  isAnimationActive={!reduced}
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
        X-axis: {xKey} | Value columns: {valueKeys.join(", ") || "none"} | {" "}
        {reduced
          ? `showing ${data.length.toLocaleString()} of ${totalRows.toLocaleString()} rows (evenly sampled)`
          : `${data.length} data points`}
      </div>
    </Modal>
  );
}
