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

import { useEffect, useRef, useMemo, useState } from "react";
import { Tag } from "antd";
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
  type MigrationExecEvent,
  statusColor,
  gridTableStyle,
  gridHeaderStyle,
} from "./MigrationModal";

interface ExecGridProps {
  events: MigrationExecEvent[];
  deployDone: boolean;
}

export default function ExecGrid({ events, deployDone }: ExecGridProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [sorting, setSorting] = useState<SortingState>([]);

  const terminalEvents = useMemo(
    () => events.filter(
      (e) => e.status === "success" || e.status === "failed" || e.status === "skipped"
    ),
    [events]
  );

  const columns = useMemo<ColumnDef<MigrationExecEvent>[]>(() => [
    {
      id: "pass",
      accessorKey: "pass",
      header: "Pass",
      size: 70,
    },
    {
      id: "kind",
      header: "Kind",
      size: 130,
      accessorFn: (row) => {
        const parts = (row.object ?? "").split(".");
        return parts[2] ?? "";
      },
    },
    {
      id: "name",
      header: "Name",
      size: 200,
      accessorFn: (row) => {
        const parts = (row.object ?? "").split(".");
        return parts[3] ?? row.object ?? "";
      },
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
      id: "error",
      accessorKey: "error",
      header: "Error",
      size: 300,
    },
  ], []);

  const table = useReactTable({
    data: terminalEvents,
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

  // Auto-scroll to the latest event during deployment
  useEffect(() => {
    if (!deployDone && terminalEvents.length > 0) {
      virtualizer.scrollToIndex(terminalEvents.length - 1, { align: "end" });
    }
  }, [terminalEvents.length, deployDone, virtualizer]);

  return (
    <div
      ref={scrollRef}
      className="thaw-grid"
      tabIndex={0}
      style={{ height: 320, width: "100%", overflow: "auto", outline: "none" }}
    >
      <table role="grid" aria-label="Migration execution" style={gridTableStyle}>
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
                      background: "var(--bg-raised)",
                      position: "relative",
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
