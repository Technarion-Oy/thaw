// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo, useRef } from "react";
import {
  Modal, Spin, Space, Typography, Button, Alert, Select, Input, DatePicker,
} from "antd";
import { HistoryOutlined, ReloadOutlined } from "@ant-design/icons";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  flexRender,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { GetPipeCopyHistory } from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { computeColumnWidths } from "../../utils/gridMeasure";
import dayjs from "dayjs";

const { Text } = Typography;

const STATUS_OPTIONS = [
  { value: "", label: "All statuses" },
  { value: "LOADED", label: "LOADED" },
  { value: "LOAD_FAILED", label: "LOAD_FAILED" },
  { value: "PARTIALLY_LOADED", label: "PARTIALLY_LOADED" },
  { value: "LOAD_IN_PROGRESS", label: "LOAD_IN_PROGRESS" },
];

const MIN_COL_WIDTH = 80;
const MAX_COL_WIDTH = 400;
const GRID_FONT = "12px Inter, SF Pro Text, system-ui, sans-serif";

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function PipeCopyHistoryModal({ db, schema, name, onClose }: Props) {
  const [result, setResult] = useState<snowflake.QueryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [startTime, setStartTime] = useState<string>("");
  const [status, setStatus] = useState<string>("");
  const [fileName, setFileName] = useState<string>("");
  const [sorting, setSorting] = useState<SortingState>([]);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const fetchHistory = async (st: string, stat: string, fn: string) => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await GetPipeCopyHistory(db, schema, name, st, stat, fn);
      setResult(res ?? null);
    } catch (e) {
      setLoadError(String(e));
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHistory("", "", "");
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [db, schema, name]);

  const data = result?.rows ?? [];

  const initialWidths = useMemo(() => {
    if (!result?.columns?.length) return [];
    return computeColumnWidths(result.columns, result.rows, {
      font: GRID_FONT, minWidth: MIN_COL_WIDTH, maxWidth: MAX_COL_WIDTH, sampleRows: 50,
    });
  }, [result]);

  const columns = useMemo<ColumnDef<unknown[]>[]>(() => {
    if (!result?.columns?.length) return [];
    return result.columns.map((col, colIdx) => ({
      id: `${colIdx}_${col}`,
      accessorFn: (row: unknown[]) => row[colIdx],
      header: col,
      size: initialWidths[colIdx] ?? MIN_COL_WIDTH,
      minSize: MIN_COL_WIDTH,
      maxSize: MAX_COL_WIDTH,
    }));
  }, [result, initialWidths]);

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    columnResizeMode: "onChange",
  });

  const { rows: tableRows } = table.getRowModel();

  const rowVirtualizer = useVirtualizer({
    count: tableRows.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 32,
    overscan: 10,
  });
  const virtualRows = rowVirtualizer.getVirtualItems();

  const visibleColumns = table.getVisibleLeafColumns();
  const pipeRef = `"${db}"."${schema}"."${name}"`;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <HistoryOutlined style={{ color: "var(--link)" }} />
          <span>Copy History</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {pipeRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={1100}
      styles={{ body: { paddingTop: 12, paddingBottom: 8 } }}
    >
      {/* Filter bar */}
      <Space wrap style={{ marginBottom: 12 }}>
        <DatePicker
          showTime
          placeholder="Start time (default: last 24h)"
          style={{ width: 230 }}
          value={startTime ? dayjs(startTime) : null}
          onChange={(v) => setStartTime(v ? v.toISOString() : "")}
        />
        <Select
          value={status}
          onChange={setStatus}
          options={STATUS_OPTIONS}
          style={{ width: 180 }}
        />
        <Input
          placeholder="File name filter…"
          value={fileName}
          onChange={(e) => setFileName(e.target.value)}
          style={{ width: 200 }}
          allowClear
        />
        <Button
          icon={<ReloadOutlined />}
          onClick={() => fetchHistory(startTime, status, fileName)}
          loading={loading}
        >
          Refresh
        </Button>
      </Space>

      {loadError && (
        <Alert
          type="error"
          message="Failed to load copy history"
          description={loadError}
          showIcon
          style={{ marginBottom: 12 }}
        />
      )}

      {loading && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin />
        </div>
      )}

      {!loading && result && (
        <div
          ref={scrollContainerRef}
          className="thaw-grid"
          tabIndex={0}
          style={{
            height: 480,
            width: "100%",
            overflow: "auto",
            outline: "none",
            ["--wails-draggable" as string]: "no-drag",
          }}
        >
          <table
            role="grid"
            aria-label="Copy history"
            style={{
              width: "100%",
              borderCollapse: "collapse",
              tableLayout: "fixed",
              fontSize: 12,
              fontFamily: "var(--ui-font, 'Inter', 'SF Pro Text', system-ui, sans-serif)",
            }}
          >
            <colgroup>
              {visibleColumns.map((column) => (
                <col key={column.id} style={{ width: column.getSize() }} />
              ))}
            </colgroup>
            <thead style={{ position: "sticky", top: 0, zIndex: 2, background: "var(--bg-raised)" }}>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr key={headerGroup.id}>
                  {headerGroup.headers.map((header) => {
                    const isSorted = header.column.getIsSorted();
                    const canSort = header.column.getCanSort();
                    return (
                      <th
                        key={header.id}
                        style={{
                          height: 32,
                          padding: "0 8px",
                          textAlign: "left",
                          fontWeight: 600,
                          fontSize: 12,
                          color: "var(--text-muted)",
                          borderBottom: "1px solid var(--border)",
                          borderRight: "1px solid var(--border)",
                          cursor: canSort ? "pointer" : "default",
                          userSelect: "none",
                          position: "relative",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          width: header.column.getSize(),
                        }}
                        onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {isSorted && (
                          <span style={{ marginLeft: 4, fontSize: 9 }}>
                            {isSorted === "asc" ? "\u25B2" : "\u25BC"}
                          </span>
                        )}
                        {header.column.getCanResize() && (
                          <div
                            onMouseDown={header.getResizeHandler()}
                            onTouchStart={header.getResizeHandler()}
                            onClick={(e) => e.stopPropagation()}
                            style={{
                              position: "absolute",
                              right: 0,
                              top: 0,
                              bottom: 0,
                              width: 4,
                              cursor: "col-resize",
                              background: header.column.getIsResizing() ? "var(--accent)" : "transparent",
                            }}
                            onMouseEnter={(e) => {
                              if (!header.column.getIsResizing())
                                e.currentTarget.style.background = "var(--border)";
                            }}
                            onMouseLeave={(e) => {
                              if (!header.column.getIsResizing())
                                e.currentTarget.style.background = "transparent";
                            }}
                          />
                        )}
                      </th>
                    );
                  })}
                </tr>
              ))}
            </thead>
            <tbody>
              {virtualRows.length > 0 && (
                <tr>
                  <td
                    style={{ height: virtualRows[0].start, padding: 0, border: "none" }}
                    colSpan={visibleColumns.length}
                  />
                </tr>
              )}
              {virtualRows.map((virtualRow) => {
                const row = tableRows[virtualRow.index];
                return (
                  <tr
                    key={row.id}
                    style={{
                      height: 32,
                      background: virtualRow.index % 2 === 1
                        ? "color-mix(in srgb, var(--bg-raised) 50%, transparent)"
                        : undefined,
                    }}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td
                        key={cell.id}
                        style={{
                          padding: "0 8px",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          borderBottom: "1px solid var(--border)",
                          color: "var(--text)",
                          height: 32,
                        }}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                );
              })}
              {virtualRows.length > 0 && (
                <tr>
                  <td
                    style={{
                      height:
                        rowVirtualizer.getTotalSize() -
                        (virtualRows[virtualRows.length - 1]?.end ?? 0),
                      padding: 0,
                      border: "none",
                    }}
                    colSpan={visibleColumns.length}
                  />
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {!loading && !loadError && result && result.rows?.length === 0 && (
        <div style={{ textAlign: "center", padding: 24, color: "var(--text-muted)", fontSize: 13 }}>
          No copy history found for the selected filters.
        </div>
      )}
    </Modal>
  );
}
