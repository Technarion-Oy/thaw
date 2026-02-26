// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useMemo } from "react";
import { AgGridReact } from "ag-grid-react";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";
import type { QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";

interface Props {
  result: QueryResult;
}

export default function ResultGrid({ result }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const columnDefs = useMemo(
    () =>
      result.columns.map((col) => ({
        field: col,
        headerName: col,
        resizable: true,
        sortable: true,
        filter: true,
        minWidth: 80,
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

  return (
    <div className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"} style={{ height: "100%", width: "100%" }}>
      <AgGridReact
        columnDefs={columnDefs}
        rowData={rowData}
        defaultColDef={{ flex: 1, minWidth: 80 }}
        animateRows
        enableCellTextSelection
        suppressMenuHide
        pagination
        paginationPageSize={500}
      />
    </div>
  );
}
