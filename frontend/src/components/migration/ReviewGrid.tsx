// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties found in a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Schema Migration

import { useRef, useMemo, useState } from "react";
import { Checkbox, Tag } from "antd";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  flexRender,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  type MigrationDiffItem,
  statusColor,
  objectLabel,
  gridTableStyle,
  gridHeaderStyle,
} from "./MigrationModal";

interface ReviewGridProps {
  data: MigrationDiffItem[];
  selectedKeys: Set<string>;
  activeDiff: MigrationDiffItem | null;
  onCheck: (item: MigrationDiffItem, checked: boolean) => void;
  onRowClick: (item: MigrationDiffItem) => void;
}

export default function ReviewGrid({
  data,
  selectedKeys,
  activeDiff,
  onCheck,
  onRowClick,
}: ReviewGridProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [sorting, setSorting] = useState<SortingState>([]);

  const columns = useMemo<ColumnDef<MigrationDiffItem>[]>(() => [
    {
      id: "checkbox",
      header: "",
      size: 44,
      enableSorting: false,
      enableResizing: false,
    },
    {
      id: "status",
      accessorKey: "status",
      header: "Status",
      size: 110,
      cell: ({ getValue }) => {
        const v = (getValue() as string) ?? "";
        return <Tag color={statusColor(v)}>{v.toUpperCase()}</Tag>;
      },
    },
    {
      id: "kind",
      header: "Kind",
      size: 120,
      accessorFn: (row) => row.object.objectKind,
    },
    {
      id: "name",
      header: "Name",
      size: 200,
      accessorFn: (row) => row.object.objectName,
    },
    {
      id: "schema",
      header: "Schema",
      size: 120,
      accessorFn: (row) => row.object.schema,
    },
    {
      id: "database",
      header: "Database",
      size: 130,
      accessorFn: (row) => row.object.database,
    },
    {
      id: "file",
      header: "File",
      size: 200,
      accessorFn: (row) =>
        row.object.filePath ? row.object.filePath.split("/").pop() ?? "" : "",
    },
  ], []);

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    columnResizeMode: "onChange",
  });

  const rows = table.getRowModel().rows;

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 32,
    overscan: 5,
  });
  const virtualRows = virtualizer.getVirtualItems();
  const visibleCols = table.getVisibleLeafColumns();

  return (
    <div
      ref={scrollRef}
      className="thaw-grid"
      tabIndex={0}
      style={{ height: 260, width: "100%", overflow: "auto", outline: "none" }}
    >
      <table role="grid" aria-label="Migration review" style={gridTableStyle}>
        <colgroup>
          {visibleCols.map((column) => (
            <col key={column.id} style={{ width: column.getSize() }} />
          ))}
        </colgroup>
        <thead style={gridHeaderStyle}>
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
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      position: header.id === "checkbox" ? "sticky" : "relative",
                      left: header.id === "checkbox" ? 0 : undefined,
                      zIndex: header.id === "checkbox" ? 3 : undefined,
                      background: "var(--bg-raised)",
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
                colSpan={visibleCols.length}
              />
            </tr>
          )}
          {virtualRows.map((virtualRow) => {
            const row = rows[virtualRow.index];
            return (
              <tr
                key={row.id}
                style={{
                  height: 32,
                  cursor: "pointer",
                  background: activeDiff && objectLabel(activeDiff.object) === objectLabel(row.original.object)
                    ? "color-mix(in srgb, var(--accent) 12%, transparent)"
                    : virtualRow.index % 2 === 1
                      ? "color-mix(in srgb, var(--bg-raised) 50%, transparent)"
                      : undefined,
                }}
                onClick={() => onRowClick(row.original)}
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
                      position: cell.column.id === "checkbox" ? "sticky" : undefined,
                      left: cell.column.id === "checkbox" ? 0 : undefined,
                      zIndex: cell.column.id === "checkbox" ? 1 : undefined,
                      background: cell.column.id === "checkbox" ? "var(--bg-overlay)" : undefined,
                    }}
                  >
                    {cell.column.id === "checkbox" ? (
                      <Checkbox
                        checked={selectedKeys.has(objectLabel(row.original.object))}
                        disabled={row.original.status === "removed"}
                        onChange={(e) => onCheck(row.original, e.target.checked)}
                      />
                    ) : (
                      flexRender(cell.column.columnDef.cell, cell.getContext())
                    )}
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
                    virtualizer.getTotalSize() -
                    (virtualRows[virtualRows.length - 1]?.end ?? 0),
                  padding: 0,
                  border: "none",
                }}
                colSpan={visibleCols.length}
              />
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
