// SPDX-License-Identifier: GPL-3.0-or-later

// @thaw-domain: Core IPC & App Lifecycle

import { useEffect, useState } from "react";
import { Modal, Button, Spin, message } from "antd";
import { SafetyCertificateOutlined } from "@ant-design/icons";
import { GetLicenseText, AcceptLicense, DeclineLicense } from "../../../wailsjs/go/app/App";

interface Props {
  /** Called after the user accepts and the choice is persisted, to reveal the workspace. */
  onAccept: () => void;
}

/**
 * LicenseAgreement is the first-launch license gate. It renders a non-dismissible,
 * full-screen modal over the entire workspace: the user must Accept (persisted via
 * AcceptLicense, then the gate dismisses) or Decline (DeclineLicense quits the app).
 * There is no close affordance — the agreement cannot be bypassed.
 */
export default function LicenseAgreement({ onAccept }: Props) {
  const [text, setText] = useState<string | null>(null);
  // Tracked separately from `text` so that when the fetch fails the error
  // message we display doesn't count as loaded license text — otherwise a
  // truthy fallback string would enable Accept and let the user accept a
  // license they never saw.
  const [loadError, setLoadError] = useState(false);
  const [accepting, setAccepting] = useState(false);

  useEffect(() => {
    GetLicenseText()
      .then((t) => {
        setText(t);
        setLoadError(false);
      })
      .catch(() => {
        setText("Unable to load the license text. Please restart Thaw and try again.");
        setLoadError(true);
      });
  }, []);

  const handleAccept = () => {
    setAccepting(true);
    AcceptLicense()
      .then(() => onAccept())
      .catch(() => {
        void message.error("Failed to save your choice. Please try again.");
        setAccepting(false);
      });
  };

  const handleDecline = () => {
    void DeclineLicense();
  };

  return (
    <Modal
      open
      // A first-launch gate: not closable by the user in any way.
      closable={false}
      keyboard={false}
      maskClosable={false}
      centered
      width={720}
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <SafetyCertificateOutlined />
          License Agreement
        </span>
      }
      styles={{ body: { padding: "12px 20px" } }}
      footer={
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
          <Button onClick={handleDecline} disabled={accepting}>
            Decline &amp; Quit
          </Button>
          <Button
            type="primary"
            onClick={handleAccept}
            loading={accepting}
            // Only enabled once the real license text has loaded — never on the
            // error fallback string, and never while it is still loading.
            disabled={!text || loadError}
          >
            Accept
          </Button>
        </div>
      }
    >
      <p style={{ margin: "0 0 12px", color: "var(--text-muted, #888)", fontSize: 13 }}>
        Please review and accept the following license before using Thaw.
      </p>
      {text === null ? (
        <div style={{ padding: "48px 0", textAlign: "center" }}>
          <Spin />
        </div>
      ) : (
        <pre
          style={{
            height: "50vh",
            overflow: "auto",
            margin: 0,
            padding: "12px 14px",
            fontFamily: "var(--mono-font, monospace)",
            fontSize: 12,
            lineHeight: 1.5,
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
            border: "1px solid var(--border-color, #303030)",
            borderRadius: 6,
            background: "var(--bg-subtle, rgba(127,127,127,0.06))",
          }}
        >
          {text}
        </pre>
      )}
    </Modal>
  );
}
