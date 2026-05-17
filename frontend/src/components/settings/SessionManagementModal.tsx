// @thaw-domain: Core IPC & App Lifecycle
import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Form,
  InputNumber,
  Modal,
  Radio,
  Space,
  Typography,
} from "antd";
import {
  GetSessionConfig,
  SaveSessionConfig,
  GetDefaultSessionConfig,
} from "../../../wailsjs/go/main/App";
import { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

type SessionConfig = config.SessionConfig;

interface Props {
  onClose: () => void;
}

export default function SessionManagementModal({ onClose }: Props) {
  const [cfg, setCfg] = useState<SessionConfig>({
    maxSessions: 8,
    maxOpenConnsPerSession: 4,
    maxIdleConnsPerSession: 1,
    initMode: "lazy",
    idleTimeoutMinutes: 0,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    GetSessionConfig()
      .then((c) => setCfg(c))
      .catch(() => {});
  }, []);

  function set<K extends keyof SessionConfig>(key: K, value: SessionConfig[K]) {
    setCfg((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await SaveSessionConfig(cfg);
      window.dispatchEvent(new Event("thaw:session-config-saved"));
      onClose();
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleReset() {
    try {
      const defaults = await GetDefaultSessionConfig();
      setCfg(defaults);
    } catch (e: unknown) {
      setError(String(e));
    }
  }

  return (
    <Modal
      open
      title="Session Management"
      onCancel={onClose}
      width={520}
      styles={{ body: { paddingTop: 8, maxHeight: "70vh", overflowY: "auto" } }}
      footer={
        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <Button onClick={handleReset}>Reset to Defaults</Button>
          <Space>
            <Button onClick={onClose}>Cancel</Button>
            <Button type="primary" loading={saving} onClick={handleSave}>
              Save & Apply
            </Button>
          </Space>
        </div>
      }
    >
      {error && (
        <Alert
          type="error"
          message={error}
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 12 }}
        />
      )}

      <Form layout="vertical" size="small">
        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Max concurrent sessions</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Maximum number of open Snowflake sessions. Excess idle sessions are evicted via LRU.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <InputNumber
            min={1}
            max={32}
            value={cfg.maxSessions}
            onChange={(v) => set("maxSessions", v ?? 8)}
            style={{ width: 100 }}
          />
        </Form.Item>

        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Max open connections per session</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              database/sql MaxOpenConns for each tab&apos;s connection pool.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <InputNumber
            min={1}
            max={16}
            value={cfg.maxOpenConnsPerSession}
            onChange={(v) => set("maxOpenConnsPerSession", v ?? 4)}
            style={{ width: 100 }}
          />
        </Form.Item>

        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Max idle connections per session</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              database/sql MaxIdleConns for each tab&apos;s connection pool. Should be ≤ max open connections.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <InputNumber
            min={1}
            max={cfg.maxOpenConnsPerSession}
            value={cfg.maxIdleConnsPerSession}
            onChange={(v) => set("maxIdleConnsPerSession", v ?? 1)}
            style={{ width: 100 }}
          />
        </Form.Item>

        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Session initialization mode</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Lazy: session created on first query. Eager: session created when tab opens.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <Radio.Group
            value={cfg.initMode}
            onChange={(e) => set("initMode", e.target.value)}
          >
            <Radio value="lazy">
              <Text style={{ fontSize: 12 }}>Lazy (on first query)</Text>
            </Radio>
            <Radio value="eager">
              <Text style={{ fontSize: 12 }}>Eager (on tab open)</Text>
            </Radio>
          </Radio.Group>
        </Form.Item>

        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Idle timeout (minutes)</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Evict idle sessions after this duration. 0 = disabled (LRU eviction only).
            </Text>
          }
          style={{ marginBottom: 8 }}
        >
          <InputNumber
            min={0}
            max={480}
            value={cfg.idleTimeoutMinutes}
            onChange={(v) => set("idleTimeoutMinutes", v ?? 0)}
            style={{ width: 100 }}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
