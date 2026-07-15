// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Table, Typography, Space, Alert, Tag } from "antd";
import { DashboardOutlined, ReloadOutlined } from "@ant-design/icons";
import { GetDatabaseTableSummary } from "../../../wailsjs/go/app/App";
import type { table } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface ObjectSummariesModalProps {
  db: string;
  onClose: () => void;
}

export default function ObjectSummariesModal({ db, onClose }: ObjectSummariesModalProps) {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<table.TableSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

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

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const columns = [
    {
      title: "Table Name",
      dataIndex: "name",
      key: "name",
      fixed: "left" as const,
      width: 200,
      sorter: (a: table.TableSummary, b: table.TableSummary) => a.name.localeCompare(b.name),
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
      render: (kind: string) => {
        let color = "blue";
        if (kind === "TRANSIENT") color = "orange";
        if (kind === "TEMPORARY") color = "purple";
        return <Tag color={color} style={{ fontSize: 10 }}>{kind}</Tag>;
      },
    },
    {
      title: "Rows",
      dataIndex: "rows",
      key: "rows",
      align: "right" as const,
      sorter: (a: table.TableSummary, b: table.TableSummary) => a.rows - b.rows,
      render: (num: number) => <Text>{num.toLocaleString()}</Text>,
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
  ];

  return (
    <Modal
      title={
        <Space>
          <DashboardOutlined />
          <span>Table Summary: {db}</span>
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
            Found {data.length} tables in {db}
          </Text>
          <ReloadOutlined 
            style={{ cursor: "pointer", fontSize: 12, color: "var(--text-muted)" }} 
            onClick={fetchSummary}
            spin={loading}
          />
        </div>

        {error && <Alert type="error" message={error} showIcon />}

        <Table
          dataSource={data}
          columns={columns}
          pagination={false}
          size="small"
          loading={loading}
          rowKey={(r) => `${r.schema}.${r.name}`}
          scroll={{ x: 1200, y: "60vh" }}
          bordered
        />
      </Space>
    </Modal>
  );
}
