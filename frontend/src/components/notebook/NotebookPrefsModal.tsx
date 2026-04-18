// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { Button, Modal, Radio, Space, Typography, message } from "antd";
import { GetNotebookPrefs, SaveNotebookPrefs } from "../../../wailsjs/go/main/App";
import { useNotebookPrefsStore } from "../../store/notebookPrefsStore";
import type { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

export default function NotebookPrefsModal({ onClose }: Props) {
  const [prefs, setPrefs] = useState<config.NotebookPrefs>({ syntaxMode: "kernel" });
  const [saving, setSaving] = useState(false);
  const loadStore = useNotebookPrefsStore((s) => s.load);

  useEffect(() => {
    GetNotebookPrefs().then((p) => setPrefs(p));
  }, []);

  async function handleSave() {
    setSaving(true);
    try {
      await SaveNotebookPrefs(prefs as any);
      await loadStore();
      message.success("Notebook preferences saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Notebook Preferences"
      open
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 18, paddingTop: 8 }}>

        {/* ── Syntax error highlighting ─────────────────────────────────── */}
        <div>
          <Text style={{ fontSize: 13, display: "block", marginBottom: 6 }}>
            Python syntax error highlighting
          </Text>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 10 }}>
            Controls how Python diagnostics (red and yellow underlines) are produced in notebook cells.
          </div>
          <Radio.Group
            value={prefs.syntaxMode}
            onChange={(e) => setPrefs((p) => ({ ...p, syntaxMode: e.target.value }))}
          >
            <Space direction="vertical" size={10}>
              <Radio value="kernel">
                <div>
                  <Text style={{ fontSize: 13 }}>Kernel-aware</Text>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                    Flags errors and undefined names. Variables defined in previously-run
                    cells are not flagged as undefined.
                  </div>
                </div>
              </Radio>
              <Radio value="static">
                <div>
                  <Text style={{ fontSize: 13 }}>Static only</Text>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                    Flags errors and undefined names based on the cell text alone.
                    Variables from other cells are flagged as undefined even if those
                    cells have been run.
                  </div>
                </div>
              </Radio>
              <Radio value="off">
                <div>
                  <Text style={{ fontSize: 13 }}>Disabled</Text>
                  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                    No Python diagnostics are shown.
                  </div>
                </div>
              </Radio>
            </Space>
          </Radio.Group>
        </div>

      </div>
    </Modal>
  );
}
