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

import {
  useMemo,
  useRef,
  useCallback,
  useState,
  useEffect,
  useLayoutEffect,
} from "react";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  flexRender,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { message } from "antd";
import type { QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";

export interface ScrollSyncHandle {
  scrollTo: (top: number) => void;
}

interface Props {
  result: QueryResult;
  /** Exposes a scrollTo handle so a sibling grid can drive this grid's scroll position. */
  syncScrollRef?: React.MutableRefObject<ScrollSyncHandle | null>;
  /** Called when this grid scrolls vertically so the sibling can follow. */
  onVerticalScroll?: (top: number) => void;
}

interface CtxMenu {
  x: number;
  y: number;
  cellValue: string;
  rowValues: string[];
  columns: string[];
}

// Maximum column width in px. Prevents wide-content columns (e.g. QUERY_TEXT)
// from consuming the entire grid and hiding all other columns.
const MAX_COL_WIDTH = 300;
const MIN_COL_WIDTH = 60;

// Number of sample rows to inspect for auto-sizing column widths.
const AUTO_SIZE_SAMPLE_ROWS = 100;

// Read a CSS variable from the document root as a number (strip "px").
function cssVar(name: string, fallback: number): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const n = parseInt(raw, 10);
  return isNaN(n) ? fallback : n;
}

// Estimate the pixel width of a string at 11px font size.
// Uses a shared off-screen canvas for text measurement.
let _measureCtx: CanvasRenderingContext2D | null = null;
function measureText(text: string): number {
  if (!_measureCtx) {
    const canvas = document.createElement("canvas");
    _measureCtx = canvas.getContext("2d");
    if (_measureCtx) _measureCtx.font = "11px Inter, SF Pro Text, system-ui, sans-serif";
  }
  if (!_measureCtx) return text.length * 7;
  return _measureCtx.measureText(text).width;
}

// Compute initial column widths from header text + first N rows of data.
function computeColumnWidths(
  columns: string[],
  rows: unknown[][],
  sampleRows: number = AUTO_SIZE_SAMPLE_ROWS
): number[] {
  const widths: number[] = [];
  const slicedRows = rows.slice(0, sampleRows);

  for (let colIdx = 0; colIdx < columns.length; colIdx++) {
    // Start with header width + padding for sort indicator
    let maxW = measureText(columns[colIdx]) + 32;

    for (const row of slicedRows) {
      const val = row[colIdx];
      const text = val == null ? "NULL" : String(val);
      const w = measureText(text) + 16; // cell padding
      if (w > maxW) maxW = w;
    }

    widths.push(Math.max(MIN_COL_WIDTH, Math.min(MAX_COL_WIDTH, Math.ceil(maxW))));
  }
  return widths;
}

// Render NULL/undefined as a distinct faded label so it is never confused
// with an empty string. All other values are stringified normally.
function NullCellRenderer({ value }: { value: unknown }) {
  if (value === null || value === undefined) {
    return (
      <span style={{ color: "var(--text-faint)", fontStyle: "italic", fontSize: 10, letterSpacing: "0.04em" }}>
        NULL
      </span>
    );
  }
  return <>{String(value)}</>;
}

function ResultGrid({ result, syncScrollRef, onVerticalScroll }: Props) {
  // Subscribe to uiDensity so the grid re-renders (and re-reads CSS vars) when
  // the user changes the density setting.
  useThemeStore((s) => s.uiDensity);

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const isSyncingRef = useRef(false);
  const lastScrollTopRef = useRef(0);
  const ctxRef = useRef<HTMLDivElement>(null);
  const [ctxMenu, setCtxMenu] = useState<CtxMenu | null>(null);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<Record<string, number>>({});

  const rowHeight = cssVar("--row-height", 24);
  const headerHeight = cssVar("--header-height", 28);

  // Pass the raw row arrays directly to TanStack Table — no per-row object
  // conversion.  Column accessors read by index, avoiding the O(rows * cols)
  // Object.fromEntries that previously blocked the main thread for ~1 s on
  // large result sets when switching history entries.
  const data = result.rows;

  // Compute initial column widths from data
  const initialWidths = useMemo(
    () => computeColumnWidths(result.columns, result.rows),
    [result.columns, result.rows]
  );

  // Reset column sizing and sorting when the result changes.
  // Column ids use the format `${colIdx}_${name}` to handle duplicate column
  // names from JOINs (e.g. `SELECT a.id, b.id`).
  useEffect(() => {
    const sizing: Record<string, number> = {};
    result.columns.forEach((col, i) => {
      sizing[`${i}_${col}`] = initialWidths[i];
    });
    setColumnSizing(sizing);
    setSorting([]);
  }, [result.columns, initialWidths]);

  // Column definitions — use accessorFn to read from the raw unknown[] arrays
  // instead of accessorKey which requires row objects.  Column ids include the
  // index prefix so duplicate column names (common in JOIN results) are unique.
  const columns = useMemo<ColumnDef<unknown[]>[]>(
    () =>
      result.columns.map((col, colIdx) => ({
        id: `${colIdx}_${col}`,
        accessorFn: (row: unknown[]) => row[colIdx],
        header: col,
        size: initialWidths[colIdx],
        minSize: MIN_COL_WIDTH,
        maxSize: MAX_COL_WIDTH,
        cell: ({ getValue }) => <NullCellRenderer value={getValue()} />,
      })),
    [result.columns, initialWidths]
  );

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnSizing },
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    columnResizeMode: "onChange",
  });

  const { rows: tableRows } = table.getRowModel();

  // Row virtualizer
  const rowVirtualizer = useVirtualizer({
    count: tableRows.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => rowHeight,
    overscan: 10,
  });

  // Column virtualizer for horizontal scrolling with wide tables
  const visibleColumns = table.getVisibleLeafColumns();
  const columnVirtualizer = useVirtualizer({
    horizontal: true,
    count: visibleColumns.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: (index) => visibleColumns[index].getSize(),
    overscan: 3,
  });

  // Register a scrollTo handle so the parent can programmatically scroll this grid.
  useEffect(() => {
    if (!syncScrollRef) return;
    syncScrollRef.current = {
      scrollTo: (top: number) => {
        const el = scrollContainerRef.current;
        if (!el) return;
        isSyncingRef.current = true;
        el.scrollTop = top;
        requestAnimationFrame(() => { isSyncingRef.current = false; });
      },
    };
    return () => { if (syncScrollRef) syncScrollRef.current = null; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [syncScrollRef]);

  // Handle scroll events for sync — only fire when scrollTop actually changes
  // to avoid unnecessary no-op calls during horizontal-only scrolling.
  const handleScroll = useCallback(() => {
    if (isSyncingRef.current) return;
    const el = scrollContainerRef.current;
    if (!el) return;
    const top = el.scrollTop;
    if (top === lastScrollTopRef.current) return;
    lastScrollTopRef.current = top;
    onVerticalScroll?.(top);
  }, [onVerticalScroll]);

  // Dismiss context menu on outside mousedown or Escape.
  useEffect(() => {
    if (!ctxMenu) return;
    const dismiss = () => setCtxMenu(null);
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") dismiss(); };
    document.addEventListener("mousedown", dismiss);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", dismiss);
      document.removeEventListener("keydown", onKey);
    };
  }, [ctxMenu]);

  // Clamp context menu inside viewport before first paint.
  useLayoutEffect(() => {
    if (!ctxMenu || !ctxRef.current) return;
    const el = ctxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    const left = Math.max(pad, Math.min(ctxMenu.x, window.innerWidth - width - pad));
    const top = Math.max(pad, Math.min(ctxMenu.y, window.innerHeight - height - pad));
    el.style.left = `${left}px`;
    el.style.top = `${top}px`;
  }, [ctxMenu]);

  const handleCellContextMenu = useCallback(
    (e: React.MouseEvent, rowData: unknown[], columnId: string) => {
      e.preventDefault();
      e.stopPropagation();

      // Column ids use the format `${colIdx}_${name}` — extract the numeric prefix.
      const underscoreIdx = columnId.indexOf("_");
      const colIdx = underscoreIdx >= 0 ? parseInt(columnId.substring(0, underscoreIdx), 10) : -1;
      const cellValue = colIdx >= 0 && rowData[colIdx] != null ? String(rowData[colIdx]) : "";
      const rowValues = rowData.map((v) => (v == null ? "" : String(v)));

      setCtxMenu({ x: e.clientX, y: e.clientY, cellValue, rowValues, columns: result.columns });
    },
    [result.columns]
  );

  const copyCell = async () => {
    if (!ctxMenu) return;
    setCtxMenu(null);
    await ClipboardSetText(ctxMenu.cellValue);
    message.success("Copied");
  };

  const copyRow = async () => {
    if (!ctxMenu) return;
    setCtxMenu(null);
    await ClipboardSetText(ctxMenu.rowValues.join("\t"));
    message.success("Row copied");
  };

  const copyRowWithHeaders = async () => {
    if (!ctxMenu) return;
    setCtxMenu(null);
    await ClipboardSetText(`${ctxMenu.columns.join("\t")}\n${ctxMenu.rowValues.join("\t")}`);
    message.success("Row copied with headers");
  };

  const menuItemEl = (label: string, action: () => void) => (
    <div
      style={{ padding: "6px 14px", cursor: "pointer", color: "var(--text)", whiteSpace: "nowrap" }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      onMouseDown={(e) => { e.stopPropagation(); action(); }}
    >
      {label}
    </div>
  );

  const totalColumnWidth = columnVirtualizer.getTotalSize();
  const totalRowHeight = rowVirtualizer.getTotalSize();
  const virtualRows = rowVirtualizer.getVirtualItems();

  // Compute left/right spacer widths for column virtualisation.
  // The colgroup declares ALL columns so the browser knows the full layout;
  // each row renders only the visible cells plus padding <td> spacers on the
  // left and right to fill the off-screen column space.
  const virtualCols = columnVirtualizer.getVirtualItems();
  const firstVirtCol = virtualCols[0];
  const lastVirtCol = virtualCols[virtualCols.length - 1];

  // Number of columns before and after the visible window
  const leftColCount = firstVirtCol ? firstVirtCol.index : 0;
  const rightColCount = lastVirtCol ? visibleColumns.length - lastVirtCol.index - 1 : 0;

  // Pixel widths for the left/right spacer cells
  let leftSpacerWidth = 0;
  for (let i = 0; i < leftColCount; i++) leftSpacerWidth += visibleColumns[i].getSize();
  let rightSpacerWidth = 0;
  for (let i = visibleColumns.length - rightColCount; i < visibleColumns.length; i++)
    rightSpacerWidth += visibleColumns[i].getSize();

  return (
    <div style={{ height: "100%", width: "100%", position: "relative" }}>
      <div
        ref={scrollContainerRef}
        className="thaw-grid"
        onScroll={handleScroll}
        style={{
          height: "100%",
          width: "100%",
          overflow: "auto",
          // WKWebView compat
          ["--wails-draggable" as string]: "no-drag",
        }}
      >
        <table
          style={{
            width: Math.max(totalColumnWidth, scrollContainerRef.current?.clientWidth ?? 0),
            borderCollapse: "collapse",
            tableLayout: "fixed",
            fontSize: 11,
            fontFamily: "var(--ui-font, 'Inter', 'SF Pro Text', system-ui, sans-serif)",
          }}
        >
          {/* Declare ALL columns so the browser knows every column's width */}
          <colgroup>
            {visibleColumns.map((column) => (
              <col key={column.id} style={{ width: column.getSize() }} />
            ))}
          </colgroup>

          {/* Header */}
          <thead
            style={{
              position: "sticky",
              top: 0,
              zIndex: 2,
              background: "var(--bg-raised)",
            }}
          >
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {/* Left spacer header */}
                {leftColCount > 0 && (
                  <th
                    colSpan={leftColCount}
                    style={{ width: leftSpacerWidth, padding: 0, border: "none" }}
                  />
                )}
                {virtualCols.map((virtualCol) => {
                  const header = headerGroup.headers[virtualCol.index];
                  if (!header) return null;
                  const isSorted = header.column.getIsSorted();
                  return (
                    <th
                      key={header.id}
                      style={{
                        height: headerHeight,
                        padding: "0 8px",
                        textAlign: "left",
                        fontWeight: 600,
                        fontSize: 11,
                        color: "var(--text-muted)",
                        borderBottom: "1px solid var(--border)",
                        borderRight: "1px solid var(--border)",
                        cursor: "pointer",
                        userSelect: "none",
                        position: "relative",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        width: header.column.getSize(),
                      }}
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      <span style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
                        {flexRender(header.column.columnDef.header, header.getContext())}
                      </span>
                      {isSorted && (
                        <span style={{ marginLeft: 4, fontSize: 9 }}>
                          {isSorted === "asc" ? "\u25B2" : "\u25BC"}
                        </span>
                      )}
                      {/* Resize handle */}
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
                          background: header.column.getIsResizing()
                            ? "var(--accent)"
                            : "transparent",
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
                    </th>
                  );
                })}
                {/* Right spacer header */}
                {rightColCount > 0 && (
                  <th
                    colSpan={rightColCount}
                    style={{ width: rightSpacerWidth, padding: 0, border: "none" }}
                  />
                )}
              </tr>
            ))}
          </thead>

          {/* Body */}
          <tbody>
            {/* Top row spacer */}
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
                    height: rowHeight,
                    background: virtualRow.index % 2 === 1
                      ? "color-mix(in srgb, var(--bg-raised) 50%, transparent)"
                      : undefined,
                  }}
                >
                  {/* Left column spacer */}
                  {leftColCount > 0 && (
                    <td
                      colSpan={leftColCount}
                      style={{ width: leftSpacerWidth, padding: 0, border: "none" }}
                    />
                  )}
                  {virtualCols.map((virtualCol) => {
                    const cell = row.getVisibleCells()[virtualCol.index];
                    if (!cell) return null;
                    return (
                      <td
                        key={cell.id}
                        onContextMenu={(e) =>
                          handleCellContextMenu(e, row.original, cell.column.id)
                        }
                        style={{
                          padding: "0 8px",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          borderBottom: "1px solid var(--border)",
                          borderRight: "1px solid color-mix(in srgb, var(--border) 40%, transparent)",
                          color: "var(--text)",
                          height: rowHeight,
                          width: cell.column.getSize(),
                        }}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    );
                  })}
                  {/* Right column spacer */}
                  {rightColCount > 0 && (
                    <td
                      colSpan={rightColCount}
                      style={{ width: rightSpacerWidth, padding: 0, border: "none" }}
                    />
                  )}
                </tr>
              );
            })}
            {/* Bottom row spacer */}
            {virtualRows.length > 0 && (
              <tr>
                <td
                  style={{
                    height:
                      totalRowHeight -
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

      {/* Context menu */}
      {ctxMenu && (
        <div
          ref={ctxRef}
          onMouseDown={(e) => e.stopPropagation()}
          style={{
            position: "fixed",
            top: ctxMenu.y,
            left: ctxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 190,
            padding: "4px 0",
            fontSize: 13,
          }}
        >
          {menuItemEl("Copy cell value", copyCell)}
          {menuItemEl("Copy row (tab-separated)", copyRow)}
          {menuItemEl("Copy row with headers", copyRowWithHeaders)}
        </div>
      )}
    </div>
  );
}

export default ResultGrid;
