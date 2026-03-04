// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useMemo, useRef, useCallback } from "react";
import { AgGridReact } from "ag-grid-react";
import type { GridApi, FirstDataRenderedEvent, RowDataUpdatedEvent } from "ag-grid-community";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";
import type { QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";

interface Props {
  result: QueryResult;
}

// Maximum column width in px. Prevents wide-content columns (e.g. QUERY_TEXT)
// from consuming the entire grid and hiding all other columns.
const MAX_COL_WIDTH = 300;

export default function ResultGrid({ result }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const apiRef = useRef<GridApi | null>(null);

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

  return (
    <div
      className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"}
      style={{ height: "100%", width: "100%", "--ag-font-size": "11px" } as React.CSSProperties}
    >
      <AgGridReact
        columnDefs={columnDefs}
        rowData={rowData}
        defaultColDef={{ resizable: true, minWidth: 60, maxWidth: MAX_COL_WIDTH }}
        rowHeight={24}
        headerHeight={28}
        animateRows
        enableCellTextSelection
        suppressMenuHide
        pagination
        paginationPageSize={500}
        onGridReady={(e) => { apiRef.current = e.api; }}
        onFirstDataRendered={autoSize}
        onRowDataUpdated={autoSize}
      />
    </div>
  );
}
