// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Core IPC & App Lifecycle

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Form,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Typography,
} from "antd";
import {
  GetFileWatchConfig,
  SaveFileWatchConfig,
  GetDefaultFileWatchConfig,
} from "../../../wailsjs/go/app/App";
import { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

type FileWatchConfig = config.FileWatchConfig;

interface Props {
  onClose: () => void;
}

export default function FileWatchingModal({ onClose }: Props) {
  const [cfg, setCfg] = useState<FileWatchConfig>({
    excludeGlobs: [],
    maxWatchedDirs: 0,
    raiseFDLimit: false,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    GetFileWatchConfig()
      .then((c) => setCfg(c))
      .catch((e) => setError(String(e)));
  }, []);

  function set<K extends keyof FileWatchConfig>(key: K, value: FileWatchConfig[K]) {
    setCfg((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await SaveFileWatchConfig(cfg);
      // Ask QueryPage (which owns the watcher lifecycle) to restart the watcher
      // so the new controls apply without an app restart.
      window.dispatchEvent(new Event("thaw:filewatch-config-saved"));
      onClose();
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleReset() {
    try {
      const defaults = await GetDefaultFileWatchConfig();
      setCfg(defaults);
    } catch (e: unknown) {
      setError(String(e));
    }
  }

  return (
    <Modal
      open
      title="File Watching"
      onCancel={onClose}
      width={560}
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
      <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 16 }}>
        Thaw watches your open folder to auto-refresh the file browser and reload
        externally edited tabs. These controls tune that watching for large or
        atypical workspaces (network drives, huge dependency trees). Changes apply
        the next time the watcher restarts, which happens automatically on save.
      </Text>

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
          label={<Text style={{ fontSize: 12 }}>Exclude patterns</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Changes under a matching path are ignored (no re-list, no tab
              reload). A single name (e.g. <Text code>node_modules</Text>) matches
              that directory at any depth; a slashed pattern (e.g.{" "}
              <Text code>.git/objects</Text>) excludes that subtree. Type a
              pattern and press Enter to add it.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <Select
            mode="tags"
            value={cfg.excludeGlobs}
            onChange={(vals) => set("excludeGlobs", vals)}
            placeholder="e.g. node_modules, dist, target, *.dist-info"
            tokenSeparators={[",", " "]}
            style={{ width: "100%" }}
            open={false}
          />
        </Form.Item>

        <Form.Item
          label={<Text style={{ fontSize: 12 }}>Max watched directories</Text>}
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Cap the number of distinct directories that emit change events.
              Beyond the cap, changes in not-yet-seen directories are ignored.
              0 = unlimited.
            </Text>
          }
          style={{ marginBottom: 20 }}
        >
          <InputNumber
            min={0}
            max={1000000}
            step={1000}
            value={cfg.maxWatchedDirs}
            onChange={(v) => set("maxWatchedDirs", v ?? 0)}
            style={{ width: 140 }}
          />
        </Form.Item>

        <Form.Item
          label={
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", width: "100%" }}>
              <Text style={{ fontSize: 12 }}>Raise file-descriptor limit on startup</Text>
              <Switch
                size="small"
                checked={cfg.raiseFDLimit}
                onChange={(checked) => set("raiseFDLimit", checked)}
              />
            </div>
          }
          help={
            <Text type="secondary" style={{ fontSize: 11 }}>
              Bump the process file-descriptor soft limit toward the hard limit
              when the watcher starts. A macOS/Linux mitigation for FD-hungry
              workspaces; no effect on Windows.
            </Text>
          }
          style={{ marginBottom: 8 }}
        />
      </Form>
    </Modal>
  );
}
