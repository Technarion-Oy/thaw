// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import {
  Modal, Space, Typography, Button, Alert, Input, App, Dropdown, MenuProps,
} from "antd";
import {
  InboxOutlined, ReloadOutlined, DownloadOutlined, DeleteOutlined, SearchOutlined,
} from "@ant-design/icons";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  type ColumnDef,
  type SortingState,
  flexRender,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ListStageFiles, RemoveStageFiles, DownloadFileFromStage, PickDirectory } from "../../../wailsjs/go/app/App";
import { formatBytes } from "../../utils/formatBytes";
import type { stage } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

function formatSize(bytes: number | null | undefined): string {
  if (bytes === undefined || bytes === null) return "-";
  return formatBytes(bytes);
}

export default function StageBrowserModal({ db, schema, name, onClose }: Props) {
  const flags = useFeatureFlagsStore((s) => s.flags);
  const { modal, message } = App.useApp();
  const [files, setFiles] = useState<stage.StageFile[]>([]);

  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [pattern, setPattern] = useState("");
  const [selectedRowIds, setSelectedRowIds] = useState<Set<string>>(new Set());
  const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }]);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const stageRef = `@${db}.${schema}.${name}`;

  const fetchFiles = async (p: string) => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await ListStageFiles(stageRef, p);
      setFiles(res || []);
    } catch (e) {
      setLoadError(String(e));
      setFiles([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchFiles("");
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [db, schema, name]);

  // Clear selection when files change
  useEffect(() => {
    setSelectedRowIds(new Set());
  }, [files]);

  const toggleRow = useCallback((rowName: string) => {
    setSelectedRowIds((prev) => {
      const next = new Set(prev);
      if (next.has(rowName)) next.delete(rowName);
      else next.add(rowName);
      return next;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setSelectedRowIds((prev) => {
      if (prev.size === files.length) return new Set();
      return new Set(files.map((f) => f.name));
    });
  }, [files]);

  const getSelectedRows = useCallback((): stage.StageFile[] => {
    return files.filter((f) => selectedRowIds.has(f.name));
  }, [files, selectedRowIds]);

  // Column defs are kept stable — checkbox rendering is handled inline in the
  // JSX (outside flexRender) so selection state changes don't rebuild columns.
  const columns = useMemo<ColumnDef<stage.StageFile>[]>(() => [
    {
      id: "checkbox",
      header: "",
      size: 40,
      enableSorting: false,
      enableResizing: false,
    },
    {
      id: "name",
      accessorKey: "name",
      header: "Name",
      size: 300,
      minSize: 100,
    },
    {
      id: "size",
      accessorKey: "size",
      header: "Size",
      size: 120,
      cell: ({ getValue }) => formatSize(getValue() as number | null),
    },
    {
      id: "md5",
      accessorKey: "md5",
      header: "MD5",
      size: 280,
    },
    {
      id: "lastModified",
      accessorKey: "lastModified",
      header: "Last Modified",
      size: 220,
    },
  ], []);

  const table = useReactTable({
    data: files,
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

  const getFullPath = (fileName: string) => {
    const slashIdx = fileName.indexOf('/');
    const relativePath = slashIdx !== -1 ? fileName.substring(slashIdx) : `/${fileName}`;
    return `${stageRef}${relativePath}`;
  };

  const handleDelete = async (targetRows?: stage.StageFile[]) => {
    const selected = targetRows || getSelectedRows();
    if (!selected || selected.length === 0) return;

    modal.confirm({
      title: `Remove ${selected.length} file(s)?`,
      content: `Are you sure you want to delete the selected files from ${stageRef}?`,
      okText: "Delete",
      okType: "danger",
      onOk: async () => {
        const hide = message.loading(`Removing ${selected.length} file(s)…`, 0);
        try {
          for (const file of selected) {
            await RemoveStageFiles(getFullPath(file.name), "");
          }
          hide();
          message.success(`Removed ${selected.length} file(s)`);
          fetchFiles(pattern);
        } catch (e) {
          hide();
          message.error(`Failed to remove files: ${String(e)}`);
        }
      },
    });
  };

  const handleDownload = async (targetRows?: stage.StageFile[]) => {
    const selected = targetRows || getSelectedRows();
    if (!selected || selected.length === 0) return;

    const localPath = await PickDirectory();
    if (!localPath) return;

    const hide = message.loading(`Downloading ${selected.length} file(s)…`, 0);
    try {
      for (const file of selected) {
        await DownloadFileFromStage(getFullPath(file.name), localPath, 4, "");
      }
      hide();
      message.success(`Downloaded ${selected.length} file(s) to ${localPath}`);
    } catch (e) {
      hide();
      message.error(`Failed to download files: ${String(e)}`);
    }
  };

  // Context menu for rows
  const [ctxVisible, setCtxVisible] = useState(false);
  const [ctxPos, setCtxPos] = useState({ x: 0, y: 0 });
  const [ctxRows, setCtxRows] = useState<stage.StageFile[]>([]);

  const onCellContextMenu = useCallback((e: React.MouseEvent, rowData: stage.StageFile) => {
    e.preventDefault();
    const selected = getSelectedRows();
    const clickedRow = rowData;

    // If clicked row is not in selection, use only clicked row.
    // Otherwise, use the whole selection.
    let targetRows = selected;
    if (!selected.find((r) => r.name === clickedRow.name)) {
      targetRows = [clickedRow];
    }

    setCtxRows(targetRows);
    setCtxPos({ x: e.clientX, y: e.clientY });
    setCtxVisible(true);
  }, [getSelectedRows]);

  const menuItems: MenuProps["items"] = [
    {
      key: "download",
      label: `Download ${ctxRows.length > 1 ? `${ctxRows.length} files` : "file"}…`,
      icon: <DownloadOutlined />,
      onClick: () => handleDownload(ctxRows),
      disabled: !flags.getCommand,
    },
    {
      key: "delete",
      label: `Delete ${ctxRows.length > 1 ? `${ctxRows.length} files` : "file"}…`,
      icon: <DeleteOutlined />,
      danger: true,
      onClick: () => handleDelete(ctxRows),
      disabled: !flags.removeCommand,
    },
  ];

  const visibleColumns = table.getVisibleLeafColumns();

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <InboxOutlined style={{ color: "var(--link)" }} />
          <span>Stage Browser</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {stageRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>Close</Button>
      ]}
      width={1000}
      styles={{ body: { paddingTop: 12, paddingBottom: 8 } }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 12 }}>
        <Space>
          <Input
            placeholder="Filter by pattern (regex)…"
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            onPressEnter={() => fetchFiles(pattern)}
            style={{ width: 300 }}
            prefix={<SearchOutlined style={{ color: "var(--text-muted)" }} />}
            allowClear
          />
          <Button icon={<ReloadOutlined />} onClick={() => fetchFiles(pattern)} loading={loading}>
            Refresh
          </Button>
        </Space>
        <Space>
          {flags.getCommand && (
            <Button
              icon={<DownloadOutlined />}
              onClick={() => handleDownload()}
              disabled={selectedRowIds.size === 0}
            >
              Download Selected
            </Button>
          )}
          {flags.removeCommand && (
            <Button
              icon={<DeleteOutlined />}
              danger
              onClick={() => handleDelete()}
              disabled={selectedRowIds.size === 0}
            >
              Delete Selected
            </Button>
          )}
        </Space>
      </div>

      {loadError && (
        <Alert
          type="error"
          message="Failed to load stage files"
          description={loadError}
          showIcon
          style={{ marginBottom: 12 }}
        />
      )}

      <div
        ref={scrollContainerRef}
        className="thaw-grid"
        onContextMenu={(e) => e.preventDefault()}
        tabIndex={0}
        style={{
          height: 500,
          width: "100%",
          overflow: "auto",
          outline: "none",
          ["--wails-draggable" as string]: "no-drag",
        }}
      >
        {files.length === 0 && !loading ? (
          <div style={{ padding: 24, textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
            No files found in this stage.
          </div>
        ) : (
          <table
            role="grid"
            aria-label="Stage files"
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
                        {header.column.id === "checkbox" ? (
                          <input
                            type="checkbox"
                            checked={files.length > 0 && selectedRowIds.size === files.length}
                            onChange={toggleAll}
                            style={{ cursor: "pointer" }}
                            ref={(el) => {
                              if (el) el.indeterminate = selectedRowIds.size > 0 && selectedRowIds.size < files.length;
                            }}
                          />
                        ) : (
                          <>
                            {flexRender(header.column.columnDef.header, header.getContext())}
                            {isSorted && (
                              <span style={{ marginLeft: 4, fontSize: 9 }}>
                                {isSorted === "asc" ? "\u25B2" : "\u25BC"}
                              </span>
                            )}
                          </>
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
                const isSelected = selectedRowIds.has(row.original.name);
                return (
                  <tr
                    key={row.id}
                    style={{
                      height: 32,
                      background: isSelected
                        ? "color-mix(in srgb, var(--accent) 12%, transparent)"
                        : virtualRow.index % 2 === 1
                          ? "color-mix(in srgb, var(--bg-raised) 50%, transparent)"
                          : undefined,
                    }}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td
                        key={cell.id}
                        onContextMenu={(e) => onCellContextMenu(e, row.original)}
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
                        {cell.column.id === "checkbox" ? (
                          <input
                            type="checkbox"
                            checked={selectedRowIds.has(row.original.name)}
                            onChange={() => toggleRow(row.original.name)}
                            style={{ cursor: "pointer" }}
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
        )}
      </div>

      <Dropdown
        menu={{ items: menuItems }}
        trigger={["contextMenu"]}
        open={ctxVisible}
        onOpenChange={setCtxVisible}
      >
        <div
          style={{
            position: "fixed",
            left: ctxPos.x,
            top: ctxPos.y,
            width: 1,
            height: 1,
            visibility: "hidden",
          }}
        />
      </Dropdown>
    </Modal>
  );
}
