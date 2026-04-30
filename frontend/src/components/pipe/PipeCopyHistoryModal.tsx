// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo } from "react";
import {
  Modal, Spin, Space, Typography, Button, Alert, Select, Input, DatePicker,
} from "antd";
import { HistoryOutlined, ReloadOutlined } from "@ant-design/icons";
import { AgGridReact } from "ag-grid-react";
import { GetPipeCopyHistory } from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";
import dayjs from "dayjs";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";

const { Text } = Typography;

const STATUS_OPTIONS = [
  { value: "", label: "All statuses" },
  { value: "LOADED", label: "LOADED" },
  { value: "LOAD_FAILED", label: "LOAD_FAILED" },
  { value: "PARTIALLY_LOADED", label: "PARTIALLY_LOADED" },
  { value: "LOAD_IN_PROGRESS", label: "LOAD_IN_PROGRESS" },
];

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

  const columnDefs = useMemo(() => {
    if (!result?.columns?.length) return [];
    return result.columns.map((col) => ({
      field: col,
      headerName: col,
      resizable: true,
      sortable: true,
      filter: true,
      minWidth: 80,
      maxWidth: 400,
    }));
  }, [result]);

  const rowData = useMemo(() => {
    if (!result) return [];
    return result.rows.map((row) =>
      Object.fromEntries((result.columns ?? []).map((col, i) => [col, row[i]]))
    );
  }, [result]);

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
          className="ag-theme-alpine"
          style={{ height: 480, width: "100%" }}
        >
          <AgGridReact
            columnDefs={columnDefs}
            rowData={rowData}
            defaultColDef={{ resizable: true, sortable: true }}
            suppressMovableColumns
            onFirstDataRendered={(params) => params.api.autoSizeAllColumns()}
          />
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
