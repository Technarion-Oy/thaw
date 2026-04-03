// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Table, Typography, Space, Alert } from "antd";
import { DashboardOutlined, ReloadOutlined } from "@ant-design/icons";
import { GetDatabaseObjectSummary } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface ObjectSummariesModalProps {
  db: string;
  onClose: () => void;
}

interface SummaryRow {
  kind: string;
  count: number;
}

export default function ObjectSummariesModal({ db, onClose }: ObjectSummariesModalProps) {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<SummaryRow[]>([]);
  const [error, setError] = useState<string | null>(null);

  const fetchSummary = async () => {
    setLoading(true);
    setError(null);
    try {
      const counts = await GetDatabaseObjectSummary(db);
      const rows: SummaryRow[] = Object.entries(counts).map(([kind, count]) => ({
        kind,
        count,
      }));
      // Sort: Tables first, then alphabet
      rows.sort((a, b) => {
        if (a.kind === "Tables") return -1;
        if (b.kind === "Tables") return 1;
        return a.kind.localeCompare(b.kind);
      });
      setData(rows);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSummary();
  }, [db]);

  const columns = [
    {
      title: "Object Type",
      dataIndex: "kind",
      key: "kind",
      render: (text: string) => <Text strong>{text}</Text>,
    },
    {
      title: "Count",
      dataIndex: "count",
      key: "count",
      align: "right" as const,
      render: (num: number) => <Text>{num.toLocaleString()}</Text>,
    },
  ];

  return (
    <Modal
      title={
        <Space>
          <DashboardOutlined />
          <span>Database Summary: {db}</span>
        </Space>
      }
      open={!!db}
      onCancel={onClose}
      footer={null}
      width={400}
      styles={{ body: { padding: "12px 24px 24px" } }}
    >
      <Space direction="vertical" style={{ width: "100%" }} size={16}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            Object counts in {db}
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
          rowKey="kind"
          bordered
        />
      </Space>
    </Modal>
  );
}
