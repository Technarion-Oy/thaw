// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useMemo, useRef, useCallback, useState, useEffect, useLayoutEffect } from "react";
import { AgGridReact } from "ag-grid-react";
import type { GridApi, FirstDataRenderedEvent, RowDataUpdatedEvent, CellContextMenuEvent } from "ag-grid-community";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";
import { message } from "antd";
import type { QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";

interface Props {
  result: QueryResult;
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

// Read a CSS variable from the document root as a number (strip "px").
function cssVar(name: string, fallback: number): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const n = parseInt(raw, 10);
  return isNaN(n) ? fallback : n;
}

export default function ResultGrid({ result }: Props) {
  const resolved  = useThemeStore((s) => s.resolved);
  // Subscribe to uiDensity so the grid re-renders (and re-reads CSS vars) when
  // the user changes the density setting.
  useThemeStore((s) => s.uiDensity);
  const apiRef  = useRef<GridApi | null>(null);
  const ctxRef  = useRef<HTMLDivElement>(null);
  const [ctxMenu, setCtxMenu] = useState<CtxMenu | null>(null);

  // Dismiss context menu on outside mousedown or Escape.
  useEffect(() => {
    if (!ctxMenu) return;
    const dismiss = () => setCtxMenu(null);
    const onKey   = (e: KeyboardEvent) => { if (e.key === "Escape") dismiss(); };
    document.addEventListener("mousedown", dismiss);
    document.addEventListener("keydown",   onKey);
    return () => {
      document.removeEventListener("mousedown", dismiss);
      document.removeEventListener("keydown",   onKey);
    };
  }, [ctxMenu]);

  // Clamp context menu inside viewport before first paint.
  useLayoutEffect(() => {
    if (!ctxMenu || !ctxRef.current) return;
    const el = ctxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad  = 8;
    const left = Math.max(pad, Math.min(ctxMenu.x, window.innerWidth  - width  - pad));
    const top  = Math.max(pad, Math.min(ctxMenu.y, window.innerHeight - height - pad));
    el.style.left = `${left}px`;
    el.style.top  = `${top}px`;
  }, [ctxMenu]);

  const columnDefs = useMemo(
    () =>
      result.columns.map((col) => ({
        field: col,
        headerName: col,
        resizable: true,
        sortable: true,
        filter: true,
        minWidth: 60,
        maxWidth: MAX_COL_WIDTH,
      })),
    [result.columns]
  );

  const rowData = useMemo(
    () =>
      result.rows.map((row) =>
        Object.fromEntries(result.columns.map((col, i) => [col, row[i]]))
      ),
    [result.rows, result.columns]
  );

  // Auto-size every column to fit its header and cell content, capped at MAX_COL_WIDTH.
  const autoSize = useCallback((e: FirstDataRenderedEvent | RowDataUpdatedEvent) => {
    (e.api as GridApi).autoSizeAllColumns();
  }, []);

  const onCellContextMenu = useCallback((e: CellContextMenuEvent) => {
    const mouse = e.event as MouseEvent | undefined;
    if (!mouse) return;
    // Prevent the document-level contextmenu handler from firing (it would
    // call e.preventDefault which is fine, but we also don't need it).
    mouse.stopPropagation();

    const cellValue = e.value == null ? "" : String(e.value);
    const rowValues = result.columns.map((col) => {
      const v = e.data?.[col];
      return v == null ? "" : String(v);
    });

    setCtxMenu({ x: mouse.clientX, y: mouse.clientY, cellValue, rowValues, columns: result.columns });
  }, [result.columns]);

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

  const menuItem = (label: string, action: () => void) => (
    <div
      style={{ padding: "6px 14px", cursor: "pointer", color: "var(--text)", whiteSpace: "nowrap" }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      // onMouseDown instead of onClick: dismiss fires on mousedown (document
      // listener), so onClick would never fire. stopPropagation prevents
      // the dismiss handler from running before the action.
      onMouseDown={(e) => { e.stopPropagation(); action(); }}
    >
      {label}
    </div>
  );

  return (
    <div style={{ height: "100%", width: "100%", position: "relative" }}>
      <div
        className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"}
        style={{ height: "100%", width: "100%", "--ag-font-size": "11px" } as React.CSSProperties}
      >
        <AgGridReact
          columnDefs={columnDefs}
          rowData={rowData}
          defaultColDef={{ resizable: true, minWidth: 60, maxWidth: MAX_COL_WIDTH }}
          rowHeight={cssVar("--row-height", 24)}
          headerHeight={cssVar("--header-height", 28)}
          animateRows
          enableCellTextSelection
          suppressMenuHide
          pagination
          paginationPageSize={500}
          onGridReady={(e) => { apiRef.current = e.api; }}
          onFirstDataRendered={autoSize}
          onRowDataUpdated={autoSize}
          onCellContextMenu={onCellContextMenu}
        />
      </div>

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
          {menuItem("Copy cell value",        copyCell)}
          {menuItem("Copy row (tab-separated)", copyRow)}
          {menuItem("Copy row with headers",  copyRowWithHeaders)}
        </div>
      )}
    </div>
  );
}
