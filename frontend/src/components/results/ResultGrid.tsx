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
  getFilteredRowModel,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
  type ColumnPinningState,
  flexRender,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { message } from "antd";
import type { QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import { useGridStore, type ConditionalRule } from "../../store/gridStore";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { computeColumnWidths, measureText } from "../../utils/gridMeasure";
import { applyFormat } from "./DataTypeFormatModal";
import { columnFilterFn, type ColumnFilterValue } from "./ColumnFilterDropdown";
import ColumnFilterDropdown from "./ColumnFilterDropdown";
import ConditionalFormattingModal from "./ConditionalFormattingModal";
import DataTypeFormatModal from "./DataTypeFormatModal";
import QuickChartModal from "./QuickChartModal";

export interface ScrollSyncHandle {
  scrollTo: (top: number) => void;
}

export interface ResultGridHandle {
  scrollToRow: (rowIndex: number) => void;
}

interface Props {
  result: QueryResult;
  syncScrollRef?: React.MutableRefObject<ScrollSyncHandle | null>;
  onVerticalScroll?: (top: number) => void;
  gridRef?: React.MutableRefObject<ResultGridHandle | null>;
}

interface CtxMenu {
  x: number;
  y: number;
  cellValue: string;
  rowValues: string[];
  columns: string[];
  rowIndex: number;
  colIndex: number;
}

interface HeaderCtxMenu {
  x: number;
  y: number;
  columnId: string;
  columnName: string;
  colIndex: number;
}

// Maximum column width for initial auto-sizing. Double-click resize removes this cap.
const MAX_COL_WIDTH = 300;
const AUTO_SIZE_MAX_COL_WIDTH = 800;
const MIN_COL_WIDTH = 60;
const GRID_FONT = "11px Inter, SF Pro Text, system-ui, sans-serif";

function cssVar(name: string, fallback: number): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const n = parseInt(raw, 10);
  return isNaN(n) ? fallback : n;
}

// ─── Conditional formatting helpers ───────────────────────────────────────────

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}

function interpolateColor(min: string, max: string, ratio: number): string {
  const [r1, g1, b1] = hexToRgb(min);
  const [r2, g2, b2] = hexToRgb(max);
  const r = Math.round(r1 + (r2 - r1) * ratio);
  const g = Math.round(g1 + (g2 - g1) * ratio);
  const b = Math.round(b1 + (b2 - b1) * ratio);
  return `rgb(${r},${g},${b})`;
}

function getConditionalStyle(
  value: unknown,
  rules: ConditionalRule[],
  colMin: number,
  colMax: number,
): React.CSSProperties {
  const style: React.CSSProperties = {};
  for (const rule of rules) {
    if (rule.type === "colorScale") {
      const n = Number(value);
      if (!isNaN(n) && colMax !== colMin) {
        const ratio = (n - colMin) / (colMax - colMin);
        style.backgroundColor = interpolateColor(rule.minColor, rule.maxColor, Math.max(0, Math.min(1, ratio)));
      }
    } else if (rule.type === "textMatch") {
      const s = value == null ? "" : String(value);
      if (rule.pattern && s.toLowerCase().includes(rule.pattern.toLowerCase())) {
        style.backgroundColor = rule.backgroundColor;
        style.color = rule.textColor;
      }
    }
    // dataBar is handled in the cell renderer JSX
  }
  return style;
}

// ─── Cell content renderer ────────────────────────────────────────────────────

function CellContent({
  value,
  searchTerm,
  formatConfig,
  rules,
  colMin,
  colMax,
}: {
  value: unknown;
  searchTerm: string;
  formatConfig?: ReturnType<typeof useGridStore.getState>["columnFormats"][string];
  rules?: ConditionalRule[];
  colMin: number;
  colMax: number;
}) {
  if (value === null || value === undefined) {
    return (
      <span style={{ color: "var(--text-faint)", fontStyle: "italic", fontSize: 10, letterSpacing: "0.04em" }}>
        NULL
      </span>
    );
  }

  let displayText = formatConfig ? applyFormat(value, formatConfig) : String(value);

  // Data bar overlay
  const dataBarRule = rules?.find((r) => r.type === "dataBar");
  const dataBarEl = dataBarRule && dataBarRule.type === "dataBar" && colMax !== colMin ? (() => {
    const n = Number(value);
    if (isNaN(n)) return null;
    const ratio = Math.max(0, Math.min(1, (n - colMin) / (colMax - colMin)));
    return (
      <div
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          bottom: 0,
          width: `${ratio * 100}%`,
          backgroundColor: `${dataBarRule.color}33`,
          pointerEvents: "none",
        }}
      />
    );
  })() : null;

  // Search highlighting
  if (searchTerm) {
    const lower = displayText.toLowerCase();
    const searchLower = searchTerm.toLowerCase();
    const idx = lower.indexOf(searchLower);
    if (idx >= 0) {
      return (
        <span style={{ position: "relative" }}>
          {dataBarEl}
          {displayText.slice(0, idx)}
          <mark style={{ backgroundColor: "var(--accent)", color: "#fff", borderRadius: 2, padding: "0 1px" }}>
            {displayText.slice(idx, idx + searchTerm.length)}
          </mark>
          {displayText.slice(idx + searchTerm.length)}
        </span>
      );
    }
  }

  return (
    <span style={{ position: "relative" }}>
      {dataBarEl}
      {displayText}
    </span>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────

function ResultGrid({ result, syncScrollRef, onVerticalScroll, gridRef }: Props) {
  const uiDensity = useThemeStore((s) => s.uiDensity);
  const featureFlags = useFeatureFlagsStore((s) => s.flags);

  // Grid store state
  const selectionRange = useGridStore((s) => s.selectionRange);
  const setSelectionRange = useGridStore((s) => s.setSelectionRange);
  const isSelecting = useGridStore((s) => s.isSelecting);
  const setIsSelecting = useGridStore((s) => s.setIsSelecting);
  const searchTerm = useGridStore((s) => s.searchTerm);
  const columnFormats = useGridStore((s) => s.columnFormats);
  const conditionalRules = useGridStore((s) => s.conditionalRules);
  const setGrouping = useGridStore((s) => s.setGrouping);
  const resetGrid = useGridStore((s) => s.reset);

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const isSyncingRef = useRef(false);
  const lastScrollTopRef = useRef(0);
  const ctxRef = useRef<HTMLDivElement>(null);
  const headerCtxRef = useRef<HTMLDivElement>(null);

  const [ctxMenu, setCtxMenu] = useState<CtxMenu | null>(null);
  const [headerCtxMenu, setHeaderCtxMenu] = useState<HeaderCtxMenu | null>(null);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<Record<string, number>>({});
  const [containerWidth, setContainerWidth] = useState(0);
  const [columnPinning, setColumnPinning] = useState<ColumnPinningState>({ left: [], right: [] });
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  // Modal state
  const [filterDropdown, setFilterDropdown] = useState<{ columnId: string; position: { x: number; y: number } } | null>(null);
  const [formatModal, setFormatModal] = useState<{ columnId: string; columnName: string } | null>(null);
  const [condFormatModal, setCondFormatModal] = useState<{ columnId: string; columnName: string } | null>(null);
  const [chartModal, setChartModal] = useState(false);


  // eslint-disable-next-line react-hooks/exhaustive-deps
  const rowHeight = useMemo(() => cssVar("--row-height", 24), [uiDensity]);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const headerHeight = useMemo(() => cssVar("--header-height", 28), [uiDensity]);

  const data = result.rows;

  const initialWidths = useMemo(
    () => computeColumnWidths(result.columns, result.rows, {
      font: GRID_FONT, minWidth: MIN_COL_WIDTH, maxWidth: MAX_COL_WIDTH, nullText: "NULL",
    }),
    [result.columns, result.rows],
  );

  // Reset grid state synchronously when result columns change.
  // Using React's "adjusting state during rendering" pattern so TanStack Table
  // never processes new column definitions paired with stale state (old column
  // IDs in grouping/columnFilters/columnPinning would crash getGroupedRowModel).
  const [prevResultKey, setPrevResultKey] = useState("");
  const resultKey = result.columns.join("\0");
  if (resultKey !== prevResultKey) {
    setPrevResultKey(resultKey);
    const sizing: Record<string, number> = {};
    result.columns.forEach((col, i) => {
      sizing[`${i}_${col}`] = initialWidths[i];
    });
    setColumnSizing(sizing);
    setSorting([]);
    setColumnPinning({ left: [], right: [] });
    setColumnFilters([]);
    setGrouping([]);
    resetGrid();
  }

  // Pre-compute min/max per column for conditional formatting
  const columnMinMax = useMemo(() => {
    const mm: Record<string, { min: number; max: number }> = {};
    for (const colId of Object.keys(conditionalRules)) {
      const underscoreIdx = colId.indexOf("_");
      const colIdx = underscoreIdx >= 0 ? parseInt(colId.substring(0, underscoreIdx), 10) : -1;
      if (colIdx < 0) continue;
      let min = Infinity;
      let max = -Infinity;
      for (const row of result.rows) {
        const n = Number(row[colIdx]);
        if (!isNaN(n)) {
          if (n < min) min = n;
          if (n > max) max = n;
        }
      }
      if (min !== Infinity) mm[colId] = { min, max };
    }
    return mm;
  }, [conditionalRules, result.rows]);

  // Column definitions
  const columns = useMemo<ColumnDef<unknown[]>[]>(
    () =>
      result.columns.map((col, colIdx) => ({
        id: `${colIdx}_${col}`,
        accessorFn: (row: unknown[]) => row[colIdx],
        header: col,
        size: initialWidths[colIdx],
        minSize: MIN_COL_WIDTH,
        maxSize: AUTO_SIZE_MAX_COL_WIDTH,
        filterFn: columnFilterFn as any,
      })),
    [result.columns, initialWidths],
  );

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnSizing, columnPinning, columnFilters },
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
    onColumnPinningChange: setColumnPinning,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
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

  // Separate pinned and unpinned columns
  const leftPinned = table.getLeftLeafColumns();
  const rightPinned = table.getRightLeafColumns();
  const centerColumns = table.getCenterLeafColumns();

  // Column virtualizer for center (unpinned) columns only
  const columnVirtualizer = useVirtualizer({
    horizontal: true,
    count: centerColumns.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: (index) => centerColumns[index]?.getSize() ?? 100,
    overscan: 3,
  });

  useLayoutEffect(() => {
    const el = scrollContainerRef.current;
    if (el) setContainerWidth(el.clientWidth);
  }, []);

  // Expose scrollToRow for search navigation
  useEffect(() => {
    if (!gridRef) return;
    gridRef.current = {
      scrollToRow: (rowIndex: number) => {
        rowVirtualizer.scrollToIndex(rowIndex, { align: "center" });
      },
    };
    return () => { if (gridRef) gridRef.current = null; };
  }, [gridRef, rowVirtualizer]);

  // Scroll sync handle
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

  const handleScroll = useCallback(() => {
    if (isSyncingRef.current) return;
    const el = scrollContainerRef.current;
    if (!el) return;
    const top = el.scrollTop;
    if (top === lastScrollTopRef.current) return;
    lastScrollTopRef.current = top;
    onVerticalScroll?.(top);
  }, [onVerticalScroll]);

  // ─── Auto-size column on double-click ─────────────────────────────────────

  const autoSizeColumn = useCallback(
    (columnId: string) => {
      const underscoreIdx = columnId.indexOf("_");
      const colIdx = underscoreIdx >= 0 ? parseInt(columnId.substring(0, underscoreIdx), 10) : -1;
      if (colIdx < 0) return;

      const headerText = result.columns[colIdx] ?? "";
      let maxW = measureText(headerText, GRID_FONT) + 32;

      // Measure all rows (up to 500 for performance)
      const sampleRows = result.rows.slice(0, 500);
      for (const row of sampleRows) {
        const val = row[colIdx];
        const text = val == null ? "NULL" : String(val);
        const w = measureText(text, GRID_FONT) + 16;
        if (w > maxW) maxW = w;
      }

      const newWidth = Math.max(MIN_COL_WIDTH, Math.min(AUTO_SIZE_MAX_COL_WIDTH, Math.ceil(maxW)));
      setColumnSizing((prev) => ({ ...prev, [columnId]: newWidth }));
    },
    [result.columns, result.rows],
  );

  // ─── Range selection ──────────────────────────────────────────────────────

  const selectionStartRef = useRef<{ row: number; col: number } | null>(null);

  const handleCellMouseDown = useCallback(
    (e: React.MouseEvent, rowIndex: number, colIndex: number) => {
      if (e.button !== 0) return; // only left click
      if (!featureFlags.multiCellCopy) return;
      selectionStartRef.current = { row: rowIndex, col: colIndex };
      setSelectionRange({ startRow: rowIndex, endRow: rowIndex, startCol: colIndex, endCol: colIndex });
      setIsSelecting(true);
    },
    [featureFlags.multiCellCopy, setSelectionRange, setIsSelecting],
  );

  const handleCellMouseEnter = useCallback(
    (rowIndex: number, colIndex: number) => {
      if (!isSelecting || !selectionStartRef.current) return;
      setSelectionRange({
        startRow: selectionStartRef.current.row,
        endRow: rowIndex,
        startCol: selectionStartRef.current.col,
        endCol: colIndex,
      });
    },
    [isSelecting, setSelectionRange],
  );

  useEffect(() => {
    if (!isSelecting) return;
    const onUp = () => {
      setIsSelecting(false);
      selectionStartRef.current = null;
    };
    document.addEventListener("mouseup", onUp);
    return () => document.removeEventListener("mouseup", onUp);
  }, [isSelecting, setIsSelecting]);

  // ─── Multi-cell copy (Cmd+C / Ctrl+C) ────────────────────────────────────

  useEffect(() => {
    if (!featureFlags.multiCellCopy) return;
    const handler = (e: KeyboardEvent) => {
      const cmd = /Mac|iPhone|iPad/.test(navigator.platform) ? e.metaKey : e.ctrlKey;
      if (!cmd || e.key !== "c") return;
      if (!selectionRange) return;
      // Only handle if focus is inside the grid
      const el = scrollContainerRef.current;
      if (!el || !el.contains(document.activeElement) && document.activeElement !== el) return;

      e.preventDefault();
      const { startRow, endRow, startCol, endCol } = selectionRange;
      const minRow = Math.min(startRow, endRow);
      const maxRow = Math.max(startRow, endRow);
      const minCol = Math.min(startCol, endCol);
      const maxCol = Math.max(startCol, endCol);

      const lines: string[] = [];
      // Add headers
      const headers: string[] = [];
      for (let c = minCol; c <= maxCol; c++) headers.push(result.columns[c] ?? "");
      lines.push(headers.join("\t"));

      for (let r = minRow; r <= maxRow; r++) {
        const row = result.rows[r];
        if (!row) continue;
        const cells: string[] = [];
        for (let c = minCol; c <= maxCol; c++) {
          cells.push(row[c] == null ? "" : String(row[c]));
        }
        lines.push(cells.join("\t"));
      }

      ClipboardSetText(lines.join("\n")).then(() => {
        const count = (maxRow - minRow + 1) * (maxCol - minCol + 1);
        message.success(`Copied ${count} cells`);
      });
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [featureFlags.multiCellCopy, selectionRange, result.rows, result.columns]);

  // ─── Select all ───────────────────────────────────────────────────────────

  const handleSelectAll = useCallback(() => {
    if (!featureFlags.multiCellCopy) return;
    setSelectionRange({
      startRow: 0,
      endRow: result.rows.length - 1,
      startCol: 0,
      endCol: result.columns.length - 1,
    });
  }, [featureFlags.multiCellCopy, result.rows.length, result.columns.length, setSelectionRange]);

  // ─── Context menus ────────────────────────────────────────────────────────

  // Dismiss context menus
  useEffect(() => {
    if (!ctxMenu && !headerCtxMenu) return;
    const dismiss = () => { setCtxMenu(null); setHeaderCtxMenu(null); };
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") dismiss(); };
    document.addEventListener("mousedown", dismiss);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", dismiss);
      document.removeEventListener("keydown", onKey);
    };
  }, [ctxMenu, headerCtxMenu]);

  // Clamp context menus
  useLayoutEffect(() => {
    if (!ctxMenu || !ctxRef.current) return;
    const el = ctxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    el.style.left = `${Math.max(pad, Math.min(ctxMenu.x, window.innerWidth - width - pad))}px`;
    el.style.top = `${Math.max(pad, Math.min(ctxMenu.y, window.innerHeight - height - pad))}px`;
  }, [ctxMenu]);

  useLayoutEffect(() => {
    if (!headerCtxMenu || !headerCtxRef.current) return;
    const el = headerCtxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    el.style.left = `${Math.max(pad, Math.min(headerCtxMenu.x, window.innerWidth - width - pad))}px`;
    el.style.top = `${Math.max(pad, Math.min(headerCtxMenu.y, window.innerHeight - height - pad))}px`;
  }, [headerCtxMenu]);

  const handleCellContextMenu = useCallback(
    (e: React.MouseEvent, rowData: unknown[], columnId: string, rowIndex: number) => {
      e.preventDefault();
      e.stopPropagation();
      const underscoreIdx = columnId.indexOf("_");
      const colIdx = underscoreIdx >= 0 ? parseInt(columnId.substring(0, underscoreIdx), 10) : -1;
      const cellValue = colIdx >= 0 && rowData[colIdx] != null ? String(rowData[colIdx]) : "";
      const rowValues = rowData.map((v) => (v == null ? "" : String(v)));
      setCtxMenu({ x: e.clientX, y: e.clientY, cellValue, rowValues, columns: result.columns, rowIndex, colIndex: colIdx });
    },
    [result.columns],
  );

  const handleHeaderContextMenu = useCallback(
    (e: React.MouseEvent, columnId: string, columnName: string, colIndex: number) => {
      e.preventDefault();
      e.stopPropagation();
      setHeaderCtxMenu({ x: e.clientX, y: e.clientY, columnId, columnName, colIndex });
    },
    [],
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

  const menuItemEl = (label: string, action: () => void, disabled?: boolean) => (
    <div
      style={{
        padding: "6px 14px",
        cursor: disabled ? "default" : "pointer",
        color: disabled ? "var(--text-faint)" : "var(--text)",
        whiteSpace: "nowrap",
        opacity: disabled ? 0.5 : 1,
      }}
      onMouseEnter={(e) => { if (!disabled) e.currentTarget.style.background = "var(--border)"; }}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      onMouseDown={(e) => { e.stopPropagation(); if (!disabled) action(); }}
    >
      {label}
    </div>
  );

  // ─── Selection check helper ───────────────────────────────────────────────

  const isCellSelected = useCallback(
    (rowIdx: number, colIdx: number): boolean => {
      if (!selectionRange) return false;
      const minRow = Math.min(selectionRange.startRow, selectionRange.endRow);
      const maxRow = Math.max(selectionRange.startRow, selectionRange.endRow);
      const minCol = Math.min(selectionRange.startCol, selectionRange.endCol);
      const maxCol = Math.max(selectionRange.startCol, selectionRange.endCol);
      return rowIdx >= minRow && rowIdx <= maxRow && colIdx >= minCol && colIdx <= maxCol;
    },
    [selectionRange],
  );

  // ─── Column pinning helpers ───────────────────────────────────────────────

  const pinColumn = (columnId: string, direction: "left" | "right") => {
    setColumnPinning((prev) => {
      const left = (prev.left ?? []).filter((id) => id !== columnId);
      const right = (prev.right ?? []).filter((id) => id !== columnId);
      if (direction === "left") left.push(columnId);
      else right.push(columnId);
      return { left, right };
    });
    setHeaderCtxMenu(null);
  };

  const unpinColumn = (columnId: string) => {
    setColumnPinning((prev) => ({
      left: (prev.left ?? []).filter((id) => id !== columnId),
      right: (prev.right ?? []).filter((id) => id !== columnId),
    }));
    setHeaderCtxMenu(null);
  };

  const isPinned = (columnId: string) => {
    return (columnPinning.left ?? []).includes(columnId) || (columnPinning.right ?? []).includes(columnId);
  };

  // ─── Layout calculations ──────────────────────────────────────────────────

  const totalColumnWidth = columnVirtualizer.getTotalSize();
  const pinnedLeftWidth = leftPinned.reduce((acc, col) => acc + col.getSize(), 0);
  const pinnedRightWidth = rightPinned.reduce((acc, col) => acc + col.getSize(), 0);
  const totalRowHeight = rowVirtualizer.getTotalSize();
  const virtualRows = rowVirtualizer.getVirtualItems();
  const virtualCols = columnVirtualizer.getVirtualItems();
  const firstVirtCol = virtualCols[0];
  const lastVirtCol = virtualCols[virtualCols.length - 1];

  const leftColCount = firstVirtCol ? firstVirtCol.index : 0;
  const rightColCount = lastVirtCol ? centerColumns.length - lastVirtCol.index - 1 : 0;

  let leftSpacerWidth = 0;
  for (let i = 0; i < leftColCount; i++) {
    if (centerColumns[i]) leftSpacerWidth += centerColumns[i].getSize();
  }
  let rightSpacerWidth = 0;
  for (let i = Math.max(0, centerColumns.length - rightColCount); i < centerColumns.length; i++) {
    if (centerColumns[i]) rightSpacerWidth += centerColumns[i].getSize();
  }

  const selectAllColWidth = featureFlags.multiCellCopy ? 28 : 0;
  const fullTableWidth = selectAllColWidth + pinnedLeftWidth + totalColumnWidth + pinnedRightWidth;

  // ─── Filter dropdown data ─────────────────────────────────────────────────

  const filterColumnValues = useMemo(() => {
    if (!filterDropdown) return [];
    const underscoreIdx = filterDropdown.columnId.indexOf("_");
    const colIdx = underscoreIdx >= 0 ? parseInt(filterDropdown.columnId.substring(0, underscoreIdx), 10) : -1;
    if (colIdx < 0) return [];
    return result.rows.map((r) => r[colIdx]);
  }, [filterDropdown, result.rows]);

  // ─── Render a header cell ─────────────────────────────────────────────────

  const renderHeaderCell = (columnId: string, colIndex: number, header: any, pinned: boolean, stickyLeft?: number, stickyRight?: number) => {
    const column = header.column;
    const isSorted = column.getIsSorted();
    const isFiltered = columnFilters.some((f) => f.id === columnId);

    return (
      <th
        key={columnId}
        draggable
        onDragStart={(e) => {
          e.dataTransfer.setData("text/plain", columnId);
          e.dataTransfer.effectAllowed = "move";
        }}
        onContextMenu={(e) => handleHeaderContextMenu(e, columnId, column.columnDef.header as string, colIndex)}
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
          position: pinned ? "sticky" : "relative",
          left: stickyLeft != null ? stickyLeft : undefined,
          right: stickyRight != null ? stickyRight : undefined,
          zIndex: pinned ? 3 : undefined,
          background: pinned ? "var(--bg-raised)" : "var(--bg-raised)",
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          width: column.getSize(),
        }}
        onClick={column.getToggleSortingHandler()}
      >
        <span style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
          {flexRender(column.columnDef.header, header.getContext())}
        </span>
        {isSorted && (
          <span style={{ marginLeft: 4, fontSize: 9 }}>
            {isSorted === "asc" ? "\u25B2" : "\u25BC"}
          </span>
        )}
        {isFiltered && (
          <span style={{ marginLeft: 4, fontSize: 9, color: "var(--accent)" }}>F</span>
        )}
        {/* Resize handle with double-click auto-size */}
        <div
          onMouseDown={header.getResizeHandler()}
          onTouchStart={header.getResizeHandler()}
          onClick={(e) => e.stopPropagation()}
          onDoubleClick={(e) => { e.stopPropagation(); autoSizeColumn(columnId); }}
          style={{
            position: "absolute",
            right: 0,
            top: 0,
            bottom: 0,
            width: 4,
            cursor: "col-resize",
            background: column.getIsResizing() ? "var(--accent)" : "transparent",
          }}
          onMouseEnter={(e) => { if (!column.getIsResizing()) e.currentTarget.style.background = "var(--border)"; }}
          onMouseLeave={(e) => { if (!column.getIsResizing()) e.currentTarget.style.background = "transparent"; }}
        />
      </th>
    );
  };

  // ─── Render a body cell ───────────────────────────────────────────────────

  const renderBodyCell = (cell: any, rowOriginal: unknown[], rowIndex: number, pinned: boolean, stickyLeft?: number, stickyRight?: number) => {
    const columnId = cell.column.id;
    const underscoreIdx = columnId.indexOf("_");
    const colIdx = underscoreIdx >= 0 ? parseInt(columnId.substring(0, underscoreIdx), 10) : -1;
    const value = cell.getValue();
    const rules = conditionalRules[columnId];
    const mm = columnMinMax[columnId];
    const condStyle = rules ? getConditionalStyle(value, rules, mm?.min ?? 0, mm?.max ?? 1) : {};
    const selected = featureFlags.multiCellCopy && isCellSelected(rowIndex, colIdx);

    return (
      <td
        key={cell.id}
        onContextMenu={(e) => handleCellContextMenu(e, rowOriginal, columnId, rowIndex)}
        onMouseDown={(e) => handleCellMouseDown(e, rowIndex, colIdx)}
        onMouseEnter={() => handleCellMouseEnter(rowIndex, colIdx)}
        onDoubleClick={(e) => {
          // Allow native text selection on double-click
          const td = e.currentTarget;
          td.style.userSelect = "text";
          td.style.webkitUserSelect = "text";
          // Select the text content
          const sel = window.getSelection();
          if (sel) {
            const range = document.createRange();
            range.selectNodeContents(td);
            sel.removeAllRanges();
            sel.addRange(range);
          }
          // Re-disable on the next mousedown anywhere
          const restore = () => {
            td.style.userSelect = "none";
            td.style.webkitUserSelect = "none";
            document.removeEventListener("mousedown", restore);
          };
          // Delay so the current double-click selection isn't cleared
          requestAnimationFrame(() => document.addEventListener("mousedown", restore));
        }}
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
          position: pinned ? "sticky" : "relative",
          left: stickyLeft != null ? stickyLeft : undefined,
          right: stickyRight != null ? stickyRight : undefined,
          zIndex: pinned ? 1 : undefined,
          userSelect: "none",
          WebkitUserSelect: "none",
          background: selected
            ? "color-mix(in srgb, var(--accent) 20%, transparent)"
            : pinned ? "var(--bg)" : undefined,
          ...condStyle,
        }}
      >
        <CellContent
          value={value}
          searchTerm={searchTerm}
          formatConfig={columnFormats[columnId]}
          rules={rules}
          colMin={mm?.min ?? 0}
          colMax={mm?.max ?? 1}
        />
      </td>
    );
  };

  // Compute sticky offsets for pinned columns
  const leftOffsets = useMemo(() => {
    const offsets: number[] = [];
    let acc = 0;
    for (const col of leftPinned) {
      offsets.push(acc);
      acc += col.getSize();
    }
    return offsets;
  }, [leftPinned]);

  const rightOffsets = useMemo(() => {
    const offsets: number[] = [];
    let acc = 0;
    for (let i = rightPinned.length - 1; i >= 0; i--) {
      offsets.unshift(acc);
      acc += rightPinned[i].getSize();
    }
    return offsets;
  }, [rightPinned]);

  return (
    <div style={{ height: "100%", width: "100%", position: "relative", display: "flex", flexDirection: "column" }}>
      <div
        ref={scrollContainerRef}
        className="thaw-grid"
        onScroll={handleScroll}
        tabIndex={0}
        style={{
          flex: 1,
          width: "100%",
          overflow: "auto",
          outline: "none",
          ["--wails-draggable" as string]: "no-drag",
        }}
      >
        <table
          role="grid"
          aria-label="Query results"
          style={{
            width: Math.max(fullTableWidth, containerWidth),
            borderCollapse: "collapse",
            tableLayout: "fixed",
            fontSize: 11,
            fontFamily: "var(--ui-font, 'Inter', 'SF Pro Text', system-ui, sans-serif)",
          }}
        >
          <colgroup>
            {featureFlags.multiCellCopy && <col style={{ width: 28 }} />}
            {leftPinned.map((col) => <col key={col.id} style={{ width: col.getSize() }} />)}
            {centerColumns.map((col) => <col key={col.id} style={{ width: col.getSize() }} />)}
            {rightPinned.map((col) => <col key={col.id} style={{ width: col.getSize() }} />)}
          </colgroup>

          {/* Header */}
          <thead style={{ position: "sticky", top: 0, zIndex: 4, background: "var(--bg-raised)" }}>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {/* Select-all button cell */}
                {featureFlags.multiCellCopy && (
                  <th
                    style={{
                      width: 28,
                      minWidth: 28,
                      maxWidth: 28,
                      height: headerHeight,
                      padding: 0,
                      textAlign: "center",
                      borderBottom: "1px solid var(--border)",
                      borderRight: "1px solid var(--border)",
                      cursor: "pointer",
                      position: "sticky",
                      left: 0,
                      zIndex: 5,
                      background: "var(--bg-raised)",
                      fontSize: 9,
                      color: "var(--text-muted)",
                    }}
                    onClick={handleSelectAll}
                    title="Select all"
                  >
                    ☐
                  </th>
                )}
                {/* Pinned left headers */}
                {leftPinned.map((col, i) => {
                  const header = headerGroup.headers.find((h) => h.column.id === col.id);
                  if (!header) return null;
                  const underscoreIdx = col.id.indexOf("_");
                  const colIdx = underscoreIdx >= 0 ? parseInt(col.id.substring(0, underscoreIdx), 10) : 0;
                  const stickyLeft = (featureFlags.multiCellCopy ? 28 : 0) + leftOffsets[i];
                  return renderHeaderCell(col.id, colIdx, header, true, stickyLeft);
                })}
                {/* Left spacer */}
                {leftColCount > 0 && <th colSpan={leftColCount} style={{ width: leftSpacerWidth, padding: 0, border: "none" }} />}
                {/* Center (virtualized) headers */}
                {virtualCols.map((vc) => {
                  const col = centerColumns[vc.index];
                  if (!col) return null;
                  const header = headerGroup.headers.find((h) => h.column.id === col.id);
                  if (!header) return null;
                  const underscoreIdx = col.id.indexOf("_");
                  const colIdx = underscoreIdx >= 0 ? parseInt(col.id.substring(0, underscoreIdx), 10) : 0;
                  return renderHeaderCell(col.id, colIdx, header, false);
                })}
                {/* Right spacer */}
                {rightColCount > 0 && <th colSpan={rightColCount} style={{ width: rightSpacerWidth, padding: 0, border: "none" }} />}
                {/* Pinned right headers */}
                {rightPinned.map((col, i) => {
                  const header = headerGroup.headers.find((h) => h.column.id === col.id);
                  if (!header) return null;
                  const underscoreIdx = col.id.indexOf("_");
                  const colIdx = underscoreIdx >= 0 ? parseInt(col.id.substring(0, underscoreIdx), 10) : 0;
                  return renderHeaderCell(col.id, colIdx, header, true, undefined, rightOffsets[i]);
                })}
              </tr>
            ))}
          </thead>

          {/* Body */}
          <tbody>
            {virtualRows.length > 0 && (
              <tr>
                <td
                  style={{ height: virtualRows[0].start, padding: 0, border: "none" }}
                  colSpan={leftPinned.length + centerColumns.length + rightPinned.length + (featureFlags.multiCellCopy ? 1 : 0)}
                />
              </tr>
            )}
            {virtualRows.map((virtualRow) => {
              const row = tableRows[virtualRow.index];
              if (!row) return null;
              const cells = row.getVisibleCells();
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
                  {/* Row number / select-all column */}
                  {featureFlags.multiCellCopy && (
                    <td
                      style={{
                        width: 28,
                        minWidth: 28,
                        maxWidth: 28,
                        padding: "0 4px",
                        fontSize: 9,
                        color: "var(--text-faint)",
                        textAlign: "center",
                        borderBottom: "1px solid var(--border)",
                        borderRight: "1px solid var(--border)",
                        position: "sticky",
                        left: 0,
                        zIndex: 1,
                        background: "var(--bg-raised)",
                        userSelect: "none",
                      }}
                    >
                      {virtualRow.index + 1}
                    </td>
                  )}
                  {/* Pinned left cells */}
                  {leftPinned.map((col, i) => {
                    const cell = cells.find((c) => c.column.id === col.id);
                    if (!cell) return null;
                    const stickyLeft = (featureFlags.multiCellCopy ? 28 : 0) + leftOffsets[i];
                    return renderBodyCell(cell, row.original, virtualRow.index, true, stickyLeft);
                  })}
                  {/* Left spacer */}
                  {leftColCount > 0 && <td colSpan={leftColCount} style={{ width: leftSpacerWidth, padding: 0, border: "none" }} />}
                  {/* Center (virtualized) cells */}
                  {virtualCols.map((vc) => {
                    const col = centerColumns[vc.index];
                    if (!col) return null;
                    const cell = cells.find((c) => c.column.id === col.id);
                    if (!cell) return null;
                    return renderBodyCell(cell, row.original, virtualRow.index, false);
                  })}
                  {/* Right spacer */}
                  {rightColCount > 0 && <td colSpan={rightColCount} style={{ width: rightSpacerWidth, padding: 0, border: "none" }} />}
                  {/* Pinned right cells */}
                  {rightPinned.map((col, i) => {
                    const cell = cells.find((c) => c.column.id === col.id);
                    if (!cell) return null;
                    return renderBodyCell(cell, row.original, virtualRow.index, true, undefined, rightOffsets[i]);
                  })}
                </tr>
              );
            })}
            {virtualRows.length > 0 && (
              <tr>
                <td
                  style={{
                    height: totalRowHeight - (virtualRows[virtualRows.length - 1]?.end ?? 0),
                    padding: 0,
                    border: "none",
                  }}
                  colSpan={leftPinned.length + centerColumns.length + rightPinned.length + (featureFlags.multiCellCopy ? 1 : 0)}
                />
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Cell context menu */}
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
          {featureFlags.multiCellCopy && selectionRange && (
            <>
              <div style={{ height: 1, background: "var(--border)", margin: "4px 0" }} />
              {menuItemEl("Create Chart...", () => { setCtxMenu(null); setChartModal(true); })}
            </>
          )}
        </div>
      )}

      {/* Header context menu */}
      {headerCtxMenu && (
        <div
          ref={headerCtxRef}
          onMouseDown={(e) => e.stopPropagation()}
          style={{
            position: "fixed",
            top: headerCtxMenu.y,
            left: headerCtxMenu.x,
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
          {menuItemEl(
            isPinned(headerCtxMenu.columnId) ? "Unpin Column" : "Pin to Left",
            () => isPinned(headerCtxMenu.columnId) ? unpinColumn(headerCtxMenu.columnId) : pinColumn(headerCtxMenu.columnId, "left"),
          )}
          {!isPinned(headerCtxMenu.columnId) && menuItemEl("Pin to Right", () => pinColumn(headerCtxMenu.columnId, "right"))}
          <div style={{ height: 1, background: "var(--border)", margin: "4px 0" }} />
          {menuItemEl("Filter...", () => {
            setFilterDropdown({ columnId: headerCtxMenu.columnId, position: { x: headerCtxMenu.x, y: headerCtxMenu.y } });
            setHeaderCtxMenu(null);
          })}
          {menuItemEl("Format Column...", () => {
            setFormatModal({ columnId: headerCtxMenu.columnId, columnName: headerCtxMenu.columnName });
            setHeaderCtxMenu(null);
          })}
          {menuItemEl("Conditional Formatting...", () => {
            setCondFormatModal({ columnId: headerCtxMenu.columnId, columnName: headerCtxMenu.columnName });
            setHeaderCtxMenu(null);
          })}
          <div style={{ height: 1, background: "var(--border)", margin: "4px 0" }} />
          {menuItemEl("Auto-size Column", () => {
            autoSizeColumn(headerCtxMenu.columnId);
            setHeaderCtxMenu(null);
          })}
        </div>
      )}

      {/* Filter dropdown */}
      {filterDropdown && (
        <ColumnFilterDropdown
          columnValues={filterColumnValues}
          currentFilter={columnFilters.find((f) => f.id === filterDropdown.columnId)?.value as ColumnFilterValue | undefined}
          onApply={(filter) => {
            if (filter) {
              setColumnFilters((prev) => [
                ...prev.filter((f) => f.id !== filterDropdown.columnId),
                { id: filterDropdown.columnId, value: filter },
              ]);
            } else {
              setColumnFilters((prev) => prev.filter((f) => f.id !== filterDropdown.columnId));
            }
          }}
          onClose={() => setFilterDropdown(null)}
          position={filterDropdown.position}
        />
      )}

      {/* Format modal */}
      {formatModal && (
        <DataTypeFormatModal
          columnId={formatModal.columnId}
          columnName={formatModal.columnName}
          sampleValues={(() => {
            const underscoreIdx = formatModal.columnId.indexOf("_");
            const colIdx = underscoreIdx >= 0 ? parseInt(formatModal.columnId.substring(0, underscoreIdx), 10) : -1;
            return colIdx >= 0 ? result.rows.slice(0, 100).map((r) => r[colIdx]) : [];
          })()}
          onClose={() => setFormatModal(null)}
        />
      )}

      {/* Conditional formatting modal */}
      {condFormatModal && (
        <ConditionalFormattingModal
          columnId={condFormatModal.columnId}
          columnName={condFormatModal.columnName}
          onClose={() => setCondFormatModal(null)}
        />
      )}

      {/* Quick chart modal */}
      {chartModal && selectionRange && (
        <QuickChartModal
          result={result}
          selectionRange={selectionRange}
          onClose={() => setChartModal(false)}
        />
      )}
    </div>
  );
}

// ─── Error boundary ──────────────────────────────────────────────────────────

import React from "react";

interface EBState { error: Error | null }

class ResultGridErrorBoundary extends React.Component<
  { children: React.ReactNode },
  EBState
> {
  state: EBState = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("ResultGrid crashed:", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 24, color: "var(--text-muted)", fontSize: 12 }}>
          Unable to display results. Try running the query again.
        </div>
      );
    }
    return this.props.children;
  }
}

function ResultGridWithErrorBoundary(props: Props) {
  return (
    <ResultGridErrorBoundary>
      <ResultGrid {...props} />
    </ResultGridErrorBoundary>
  );
}

export default ResultGridWithErrorBoundary;
