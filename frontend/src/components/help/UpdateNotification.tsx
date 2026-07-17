// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Core IPC & App Lifecycle

import { useEffect, useState } from "react";
import { Alert, Modal, Button, Spin, message } from "antd";
import { CloudDownloadOutlined, CheckCircleOutlined } from "@ant-design/icons";
import { CheckForUpdate } from "../../../wailsjs/go/app/App";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import type { updater } from "../../../wailsjs/go/models";

// UpdateNotification renders the app-update UI: a dismissible top banner when the
// background check finds a newer release, and a modal (opened from the banner or
// the "Check for Updates…" menu item) showing the release notes and a
// "Download update" button that opens the GitHub release page in the browser.
//
// It is notification-only — there is no in-app download or apply (see #568). Two
// triggers drive it:
//   - "update:available" event: emitted by the Go background checker with a
//     ready CheckResult; shows the banner.
//   - "menu:check-for-update" event: Help → Check for Updates…; runs a live
//     CheckForUpdate() and shows either the update modal or an "up to date"
//     confirmation.
export default function UpdateNotification() {
  const [result, setResult] = useState<updater.CheckResult | null>(null);
  const [bannerVisible, setBannerVisible] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [checking, setChecking] = useState(false);

  // Background check found a newer version — surface the banner.
  useEffect(() => {
    const off = EventsOn("update:available", (res: updater.CheckResult) => {
      if (!res?.available) return;
      setResult(res);
      setBannerVisible(true);
    });
    return () => off();
  }, []);

  // On-demand check from the Help menu.
  useEffect(() => {
    const off = EventsOn("menu:check-for-update", () => {
      setChecking(true);
      CheckForUpdate()
        .then((res) => {
          setResult(res);
          if (res.available) {
            setBannerVisible(true);
            setModalOpen(true);
          } else {
            void message.success(`You're up to date (v${res.currentVersion}).`);
          }
        })
        .catch(() => {
          void message.error("Couldn't check for updates. Please try again later.");
        })
        .finally(() => setChecking(false));
    });
    return () => off();
  }, []);

  const download = () => {
    if (result?.releasePageURL) BrowserOpenURL(result.releasePageURL);
  };

  return (
    <>
      {bannerVisible && result?.available && (
        <div
          style={{
            position: "fixed",
            top: 12,
            left: "50%",
            transform: "translateX(-50%)",
            zIndex: 1100,
            maxWidth: "min(640px, calc(100vw - 32px))",
            boxShadow: "0 4px 16px rgba(0,0,0,0.25)",
            borderRadius: 8,
          }}
        >
          <Alert
            type="info"
            showIcon
            icon={<CloudDownloadOutlined />}
            style={{ borderRadius: 8 }}
            message={
              <span>
                Thaw <strong>v{result.latestVersion}</strong> is available
                {result.currentVersion ? ` (you have v${result.currentVersion})` : ""}.
              </span>
            }
            action={
              <Button size="small" type="link" onClick={() => setModalOpen(true)}>
                View update
              </Button>
            }
            closable
            onClose={() => setBannerVisible(false)}
          />
        </div>
      )}

      {checking && (
        <Modal open footer={null} closable={false} width={280} centered>
          <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "8px 0" }}>
            <Spin />
            <span>Checking for updates…</span>
          </div>
        </Modal>
      )}

      {modalOpen && result && (
        <Modal
          open
          title={
            <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
              {result.available ? <CloudDownloadOutlined /> : <CheckCircleOutlined />}
              {result.available ? `Update available — v${result.latestVersion}` : "You're up to date"}
            </span>
          }
          onCancel={() => setModalOpen(false)}
          width={560}
          footer={
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
              <Button onClick={() => setModalOpen(false)}>Close</Button>
              {result.available && (
                <Button type="primary" icon={<CloudDownloadOutlined />} onClick={download}>
                  Download update
                </Button>
              )}
            </div>
          }
        >
          <div style={{ marginBottom: 8, fontSize: 13, color: "var(--text-muted, #888)" }}>
            {result.available
              ? `You have v${result.currentVersion}. Download opens the release page in your browser.`
              : `You have the latest version (v${result.currentVersion}).`}
          </div>
          {result.releaseNotes && (
            <>
              <div style={{ fontWeight: 600, margin: "8px 0 4px" }}>Release notes</div>
              <pre
                style={{
                  maxHeight: 320,
                  overflow: "auto",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  fontSize: 12,
                  lineHeight: 1.5,
                  margin: 0,
                  padding: 12,
                  borderRadius: 6,
                  background: "var(--bg-subtle, rgba(127,127,127,0.08))",
                  border: "1px solid var(--border-color, #303030)",
                  fontFamily:
                    "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
                }}
              >
                {result.releaseNotes}
              </pre>
            </>
          )}
        </Modal>
      )}
    </>
  );
}
