// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useMemo } from "react";
import { Modal, Table, Typography, Space, Alert, Tag } from "antd";
import { DashboardOutlined, ReloadOutlined } from "@ant-design/icons";
import { GetDatabaseTableSummary } from "../../../wailsjs/go/app/App";
import type { table } from "../../../wailsjs/go/models";
import type { FilterValue } from "antd/es/table/interface";
import { KIND_VAR } from "../sidebar/objectIcons";
import { KIND_FILTERS, ROW_FILTERS, schemaFilters, applyFilters, registryKind } from "./objectSummaryFilters";

const { Text } = Typography;

// Views carry no BYTES; "—" keeps that apart from a genuinely empty 0 B object.
const formatBytes = (bytes: number) => {
  if (bytes < 0) return "—";
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

interface ObjectSummariesModalProps {
  db: string;
  onClose: () => void;
}

export default function ObjectSummariesModal({ db, onClose }: ObjectSummariesModalProps) {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<table.TableSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Record<string, FilterValue | null>>({});

  const fetchSummary = async () => {
    setLoading(true);
    setError(null);
    try {
      const tables = await GetDatabaseTableSummary(db);
      setData(tables);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSummary();
  }, [db]);

  // Filtering is done here, not by antd, so the caption below can never disagree
  // with the rendered rows (an antd filter selection outlives a Reload).
  const rows = useMemo(() => applyFilters(data, filters), [data, filters]);

  const columns = useMemo(() => [
    {
      title: "Table Name",
      dataIndex: "name",
      key: "name",
      fixed: "left" as const,
      width: 200,
      sorter: (a: table.TableSummary, b: table.TableSummary) => a.name.localeCompare(b.name),
      filters: schemaFilters(data),
      filteredValue: filters.name ?? null,
      render: (name: string, record: table.TableSummary) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.schema}</Text>
        </Space>
      ),
    },
    {
      title: "Type",
      dataIndex: "kind",
      key: "kind",
      width: 120,
      filters: KIND_FILTERS,
      filteredValue: filters.kind ?? null,
      // Colour comes from the sidebar's canonical kind palette so a kind looks
      // the same here as it does in the object tree.
      render: (kind: string) => {
        const color = `var(${KIND_VAR[registryKind(kind)] ?? "--text-muted"})`;
        return <Tag style={{ fontSize: 10, color, borderColor: color }}>{kind}</Tag>;
      },
    },
    {
      title: "Rows",
      dataIndex: "rows",
      key: "rows",
      align: "right" as const,
      sorter: (a: table.TableSummary, b: table.TableSummary) => a.rows - b.rows,
      filters: ROW_FILTERS,
      filteredValue: filters.rows ?? null,
      // Snowflake reports no row count for views; "—" keeps that apart from 0.
      render: (num: number) => <Text>{num < 0 ? "—" : num.toLocaleString()}</Text>,
    },
    {
      title: "Size",
      dataIndex: "bytes",
      key: "bytes",
      align: "right" as const,
      sorter: (a: table.TableSummary, b: table.TableSummary) => a.bytes - b.bytes,
      render: (bytes: number) => <Text>{formatBytes(bytes)}</Text>,
    },
    {
      title: "Owner",
      dataIndex: "owner",
      key: "owner",
      width: 120,
      render: (owner: string) => <Tag style={{ fontSize: 10 }}>{owner}</Tag>,
    },
    {
      title: "Retention",
      dataIndex: "retentionTime",
      key: "retentionTime",
      width: 90,
      align: "center" as const,
      render: (days: number) => <Text>{days} d</Text>,
    },
    {
      title: "Created",
      dataIndex: "created",
      key: "created",
      width: 150,
      render: (ts: string) => <Text style={{ fontSize: 11 }}>{new Date(ts).toLocaleString()}</Text>,
    },
    {
      title: "Last Altered",
      dataIndex: "lastAltered",
      key: "lastAltered",
      width: 150,
      render: (ts: string) => <Text style={{ fontSize: 11 }}>{ts ? new Date(ts).toLocaleString() : "-"}</Text>,
    },
    {
      title: "Comment",
      dataIndex: "comment",
      key: "comment",
      ellipsis: true,
      render: (text: string) => text ? (
        <Text type="secondary" style={{ fontSize: 11 }}>{text}</Text>
      ) : (
        <Text type="secondary" italic style={{ fontSize: 11, opacity: 0.5 }}>NULL</Text>
      ),
    },
  ], [data, filters]);

  return (
    <Modal
      title={
        <Space>
          <DashboardOutlined />
          <span>Tables &amp; Views: {db}</span>
        </Space>
      }
      open={!!db}
      onCancel={onClose}
      footer={null}
      width="90vw"
      style={{ top: 20 }}
      styles={{ body: { padding: "12px 24px 24px", maxHeight: "80vh", overflowY: "auto" } }}
    >
      <Space direction="vertical" style={{ width: "100%" }} size={16}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            Found {rows.length} tables &amp; views in {db}
          </Text>
          <ReloadOutlined 
            style={{ cursor: "pointer", fontSize: 12, color: "var(--text-muted)" }} 
            onClick={fetchSummary}
            spin={loading}
          />
        </div>

        {error && <Alert type="error" message={error} showIcon />}

        <Table
          dataSource={rows}
          columns={columns}
          pagination={false}
          size="small"
          loading={loading}
          rowKey={(r) => `${r.schema}.${r.name}`}
          scroll={{ x: 1200, y: "60vh" }}
          onChange={(_pagination, nextFilters) => setFilters(nextFilters)}
          bordered
        />
      </Space>
    </Modal>
  );
}
