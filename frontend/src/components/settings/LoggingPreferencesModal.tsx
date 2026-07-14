// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Core IPC & App Lifecycle

import { useEffect, useState } from "react";
import { Alert, Button, Modal, Select, Switch, Tooltip, Typography, message } from "antd";
import { LockOutlined } from "@ant-design/icons";
import {
  GetLogPrefs,
  GetLogPrefsLocked,
  RevealLogFile,
  UpdateLogPrefs,
} from "../../../wailsjs/go/app/App";
import { useLogPrefsStore } from "../../store/logPrefsStore";
import type { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

const ADMIN_TOOLTIP = "This setting is managed by your IT Administrator.";

interface Props {
  onClose: () => void;
}

export default function LoggingPreferencesModal({ onClose }: Props) {
  const [prefs, setPrefs] = useState<config.LogPrefs>({
    logLevel: "info",
    includeQuerySQL: false,
    includeInternalQueries: false,
  });
  const [locked, setLocked] = useState<config.LogPrefsLocked>({
    logLevel: false,
    includeQuerySQL: false,
    includeInternalQueries: false,
  });
  const [saving, setSaving] = useState(false);
  const loadStore = useLogPrefsStore((s) => s.load);

  useEffect(() => {
    Promise.all([GetLogPrefs(), GetLogPrefsLocked()])
      .then(([p, l]) => {
        setPrefs(p);
        setLocked(l);
      })
      .catch((err) => message.error(`Failed to load logging preferences: ${err}`));
  }, []);

  async function handleSave() {
    setSaving(true);
    try {
      await UpdateLogPrefs(prefs);
      await loadStore();
      message.success("Logging preferences saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  function handleRevealLogFile() {
    RevealLogFile().catch((err) => message.error(`Failed to reveal log file: ${err}`));
  }

  const lockIcon = (
    <Tooltip title={ADMIN_TOOLTIP}>
      <LockOutlined style={{ fontSize: 11, color: "var(--text-muted)", marginLeft: 6 }} />
    </Tooltip>
  );

  return (
    <Modal
      title="Logging Preferences"
      open
      onCancel={onClose}
      footer={[
        <Button key="reveal" onClick={handleRevealLogFile}>
          Reveal Log File
        </Button>,
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={520}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 20, paddingTop: 8 }}>

        {/* ── Log level ─────────────────────────────────────────────────── */}
        <div>
          <Text style={{ fontSize: 13, display: "block", marginBottom: 6 }}>
            Log level
            {locked.logLevel && lockIcon}
          </Text>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 10 }}>
            Minimum severity written to the log file. Applied immediately, no restart needed.
          </div>
          <Select
            value={prefs.logLevel}
            disabled={locked.logLevel}
            style={{ width: 200 }}
            onChange={(v) => setPrefs((p) => ({ ...p, logLevel: v }))}
            options={[
              { value: "debug", label: "Debug (most verbose)" },
              { value: "info", label: "Info (default)" },
              { value: "warn", label: "Warning" },
              { value: "error", label: "Error (least verbose)" },
            ]}
          />
        </div>

        {/* ── Include SQL text ──────────────────────────────────────────── */}
        <div>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Text style={{ fontSize: 13 }}>
              Write executed SQL to the log file
              {locked.includeQuerySQL && lockIcon}
            </Text>
            {(() => {
              const el = (
                <Switch
                  checked={prefs.includeQuerySQL}
                  disabled={locked.includeQuerySQL}
                  size="small"
                  onChange={(checked) =>
                    setPrefs((p) => ({
                      ...p,
                      includeQuerySQL: checked,
                      // Clear the dependent switch when SQL logging is turned off.
                      includeInternalQueries: checked ? p.includeInternalQueries : false,
                    }))
                  }
                />
              );
              return locked.includeQuerySQL ? <Tooltip title={ADMIN_TOOLTIP}>{el}</Tooltip> : el;
            })()}
          </div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
            Records the full SQL text of executed statements in <Text code>thaw.log</Text>.
            Successful queries are written at <b>Info</b> level, so the log level above must be
            Debug or Info to capture them; failed queries are recorded at any level.
          </div>
          {prefs.includeQuerySQL &&
            (prefs.logLevel === "warn" || prefs.logLevel === "error") && (
              <Text type="warning" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
                At log level “{prefs.logLevel === "warn" ? "Warning" : "Error"}”, only failed
                queries are logged. Lower the log level to Info or Debug to also capture
                successful queries.
              </Text>
            )}
          {prefs.includeQuerySQL && (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 10 }}
              message="Sensitive data may be written to disk"
              description="SQL can contain credentials (COPY INTO), secrets (CREATE SECRET), and personal data in WHERE clauses. Keep this off unless you need it for debugging, and clear the log afterwards."
            />
          )}
        </div>

        {/* ── Include internal queries ──────────────────────────────────── */}
        <div>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Text style={{ fontSize: 13, color: !prefs.includeQuerySQL ? "var(--text-muted)" : undefined }}>
              Also log internal / background queries
              {locked.includeInternalQueries && lockIcon}
            </Text>
            {(() => {
              const el = (
                <Switch
                  checked={prefs.includeInternalQueries}
                  disabled={locked.includeInternalQueries || !prefs.includeQuerySQL}
                  size="small"
                  onChange={(checked) => setPrefs((p) => ({ ...p, includeInternalQueries: checked }))}
                />
              );
              return locked.includeInternalQueries ? <Tooltip title={ADMIN_TOOLTIP}>{el}</Tooltip> : el;
            })()}
          </div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
            Includes object listing, DDL fetching, and session setup queries — not just the
            statements you run. Requires SQL logging to be on.
          </div>
        </div>

      </div>
    </Modal>
  );
}
