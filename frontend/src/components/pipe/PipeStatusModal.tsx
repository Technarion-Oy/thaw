// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Button, Spin, Alert, Space, Typography } from "antd";
import { DashboardOutlined, ReloadOutlined } from "@ant-design/icons";
import { GetPipeStatus } from "../../../wailsjs/go/app/App";

const { Text } = Typography;

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0",
  color: "var(--text-muted)",
  fontSize: 12,
  whiteSpace: "nowrap",
  verticalAlign: "middle",
  width: 220,
};

const VALUE_TD: React.CSSProperties = {
  padding: "6px 0",
  fontSize: 12,
  color: "var(--text)",
  verticalAlign: "middle",
};

interface PipeStatus {
  executionState?: string;
  pendingFileCount?: number;
  notificationChannelName?: string | null;
  numOutstandingMessagesOnChannel?: number;
  lastReceivedMessageTimestamp?: string | null;
  lastForwardedMessageTimestamp?: string | null;
  lastSuccessfulIngestion?: string | null;
  error?: string | null;
  fault?: string | null;
  [key: string]: unknown;
}

const STATUS_FIELDS: { key: keyof PipeStatus; label: string }[] = [
  { key: "executionState",                   label: "Execution State" },
  { key: "pendingFileCount",                 label: "Pending File Count" },
  { key: "notificationChannelName",          label: "Notification Channel" },
  { key: "numOutstandingMessagesOnChannel",  label: "Outstanding Messages on Channel" },
  { key: "lastReceivedMessageTimestamp",     label: "Last Received Message" },
  { key: "lastForwardedMessageTimestamp",    label: "Last Forwarded Message" },
  { key: "lastSuccessfulIngestion",          label: "Last Successful Ingestion" },
  { key: "error",                            label: "Error" },
  { key: "fault",                            label: "Fault" },
];

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function PipeStatusModal({ db, schema, name, onClose }: Props) {
  const [status, setStatus] = useState<PipeStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const pipeRef = `"${db}"."${schema}"."${name}"`;

  const load = async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const json = await GetPipeStatus(db, schema, name);
      setStatus(json ? JSON.parse(json) : null);
    } catch (e) {
      setLoadError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const renderValue = (key: keyof PipeStatus, val: unknown) => {
    if (val === null || val === undefined || val === "") {
      return <Text type="secondary">(none)</Text>;
    }
    if (key === "executionState") {
      const color = val === "RUNNING"
        ? "var(--success, #52c41a)"
        : val === "PAUSED"
        ? "var(--warning, #faad14)"
        : "var(--text-muted)";
      return <span style={{ color, fontWeight: 500 }}>{String(val)}</span>;
    }
    if (key === "error" || key === "fault") {
      return <Text type="danger">{String(val)}</Text>;
    }
    return <span>{String(val)}</span>;
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <DashboardOutlined style={{ color: "var(--link)" }} />
          <span>Pipe Status</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {pipeRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={load}
            loading={loading}
          >
            Refresh
          </Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={560}
      styles={{ body: { paddingTop: 12 } }}
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Snowflake metadata lag"
        description={
          "SYSTEM$PIPE_STATUS may reflect a state that is up to several minutes "
          + "behind the actual pipe state. Use this view as a guide, not as a "
          + "real-time guarantee."
        }
      />

      {loading && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin />
        </div>
      )}

      {!loading && loadError && (
        <Alert
          type="error"
          message="Failed to load pipe status"
          description={loadError}
          showIcon
        />
      )}

      {!loading && status && (
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <tbody>
            {STATUS_FIELDS.map(({ key, label }) => (
              <tr key={key}>
                <td style={LABEL_TD}>{label}</td>
                <td style={VALUE_TD}>{renderValue(key, status[key])}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Modal>
  );
}
