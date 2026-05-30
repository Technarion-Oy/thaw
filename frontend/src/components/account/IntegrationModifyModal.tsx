// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties   holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Spin, Button, Alert, Input } from "antd";
import { GetIntegrationProperties, ExecuteQuery } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { TextArea } = Input;

interface Props {
  name: string;
  onClose: () => void;
  onSuccess: () => void;
}

export default function IntegrationModifyModal({ name, onClose, onSuccess }: Props) {
  const [props, setProps]     = useState<snowflake.PropertyPair[] | null>(null);
  const [propsErr, setPropsErr] = useState<string | null>(null);
  const [sql, setSql]         = useState(
    `ALTER INTEGRATION "${name.replace(/"/g, '""')}" SET\n  ENABLED = TRUE\n  -- COMMENT = 'new comment'`
  );
  const [running, setRunning] = useState(false);
  const [runErr, setRunErr]   = useState<string | null>(null);

  useEffect(() => {
    GetIntegrationProperties(name)
      .then(setProps)
      .catch((e) => setPropsErr(String(e)));
  }, [name]);

  const run = async () => {
    setRunning(true);
    setRunErr(null);
    try {
      await ExecuteQuery(sql);
      onSuccess();
      onClose();
    } catch (e) {
      setRunErr(String(e));
    } finally {
      setRunning(false);
    }
  };

  return (
    <Modal
      open
      title={`Modify Integration: ${name}`}
      onCancel={onClose}
      width={680}
      footer={[
        <Button key="cancel" onClick={onClose} disabled={running}>Cancel</Button>,
        <Button key="run" type="primary" loading={running} onClick={run}>Run</Button>,
      ]}
    >
      {/* Properties table */}
      <div style={{ maxHeight: "35vh", overflowY: "auto", marginBottom: 16, borderBottom: "1px solid var(--border)", paddingBottom: 12 }}>
        <div style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", letterSpacing: "0.05em", marginBottom: 8, textTransform: "uppercase" }}>
          Current Properties
        </div>
        {props === null && !propsErr && (
          <div style={{ textAlign: "center", padding: "16px 0" }}><Spin size="small" /></div>
        )}
        {propsErr && (
          <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>{propsErr}</div>
        )}
        {props && props.length === 0 && !propsErr && (
          <div style={{ color: "var(--text-muted)", fontSize: 12 }}>No properties found.</div>
        )}
        {props && props.length > 0 && (
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              {props.map((row) => (
                <tr key={row.key} style={{ borderBottom: "1px solid var(--border)" }}>
                  <td style={{ padding: "4px 12px 4px 0", color: "var(--text-muted)", fontFamily: "monospace", whiteSpace: "nowrap", verticalAlign: "top", width: 220, minWidth: 160 }}>
                    {row.key}
                  </td>
                  <td style={{ padding: "4px 0", color: "var(--text)", fontFamily: "monospace", wordBreak: "break-word", verticalAlign: "top" }}>
                    {row.value || <span style={{ color: "var(--text-muted)", fontStyle: "italic" }}>—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* ALTER SQL editor */}
      <div style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", letterSpacing: "0.05em", marginBottom: 8, textTransform: "uppercase" }}>
        ALTER Statement
      </div>
      <TextArea
        value={sql}
        onChange={(e) => setSql(e.target.value)}
        rows={6}
        style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
        disabled={running}
      />

      {runErr && (
        <Alert
          type="error"
          message={runErr}
          style={{ marginTop: 8, fontSize: 12, fontFamily: "monospace" }}
        />
      )}
    </Modal>
  );
}
