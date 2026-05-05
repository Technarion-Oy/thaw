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
import { useThemeStore } from "../../store/themeStore";
import {
  Modal, Space, Typography, Button, Alert, Input, message, Dropdown, MenuProps,
} from "antd";
import {
  InboxOutlined, ReloadOutlined, DownloadOutlined, DeleteOutlined, SearchOutlined,
} from "@ant-design/icons";
import { AgGridReact } from "ag-grid-react";
import { ListStageFiles, RemoveStageFiles, DownloadFileFromStage, PickDirectory } from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function StageBrowserModal({ db, schema, name, onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const [files, setFiles] = useState<snowflake.StageFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [pattern, setPattern] = useState("");
  const gridRef = useRef<AgGridReact>(null);

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

  const columnDefs = useMemo(() => [
    {
      field: "name",
      headerName: "Name",
      flex: 1,
      checkboxSelection: true,
      headerCheckboxSelection: true,
      sort: "asc" as const,
    },
    {
      field: "size",
      headerName: "Size",
      width: 120,
      valueFormatter: (p: any) => {
        if (p.value === undefined || p.value === null) return "-";
        const bytes = p.value;
        if (bytes < 1024) return `${bytes} B`;
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
        if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
        return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
      },
    },
    { field: "md5", headerName: "MD5", width: 280 },
    { field: "lastModified", headerName: "Last Modified", width: 220 },
  ], []);

  const getFullPath = (fileName: string) => {
    const slashIdx = fileName.indexOf('/');
    const relativePath = slashIdx !== -1 ? fileName.substring(slashIdx) : `/${fileName}`;
    return `${stageRef}${relativePath}`;
  };

  const handleDelete = async (selectedRows?: snowflake.StageFile[]) => {
    const selected = selectedRows || gridRef.current?.api.getSelectedRows();
    if (!selected || selected.length === 0) return;

    Modal.confirm({
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

  const handleDownload = async (selectedRows?: snowflake.StageFile[]) => {
    const selected = selectedRows || gridRef.current?.api.getSelectedRows();
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
  const [ctxRows, setCtxRows] = useState<snowflake.StageFile[]>([]);

  const onCellContextMenu = (event: any) => {
    event.event.preventDefault();
    const selected = gridRef.current?.api.getSelectedRows() || [];
    const clickedRow = event.data;

    // If clicked row is not in selection, use only clicked row.
    // Otherwise, use the whole selection.
    let targetRows = selected;
    if (!selected.find((r) => r.name === clickedRow.name)) {
      targetRows = [clickedRow];
    }

    setCtxRows(targetRows);
    setCtxPos({ x: event.event.clientX, y: event.event.clientY });
    setCtxVisible(true);
  };

  const menuItems: MenuProps["items"] = [
    {
      key: "download",
      label: `Download ${ctxRows.length > 1 ? `${ctxRows.length} files` : "file"}…`,
      icon: <DownloadOutlined />,
      onClick: () => handleDownload(ctxRows),
    },
    {
      key: "delete",
      label: `Delete ${ctxRows.length > 1 ? `${ctxRows.length} files` : "file"}…`,
      icon: <DeleteOutlined />,
      danger: true,
      onClick: () => handleDelete(ctxRows),
    },
  ];

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
          <Button
            icon={<DownloadOutlined />}
            onClick={() => handleDownload()}
            disabled={files.length === 0}
          >
            Download Selected
          </Button>
          <Button
            icon={<DeleteOutlined />}
            danger
            onClick={() => handleDelete()}
            disabled={files.length === 0}
          >
            Delete Selected
          </Button>
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
        className={resolved === "dark" ? "ag-theme-alpine-dark" : "ag-theme-alpine"}
        style={{ height: 500, width: "100%" }}
        onContextMenu={(e) => e.preventDefault()}
      >
        <AgGridReact
          ref={gridRef}
          columnDefs={columnDefs}
          rowData={files}
          rowSelection="multiple"
          defaultColDef={{ resizable: true, sortable: true, filter: true }}
          suppressMovableColumns
          onCellContextMenu={onCellContextMenu}
          overlayNoRowsTemplate={loading ? " " : "No files found in this stage."}
        />
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
