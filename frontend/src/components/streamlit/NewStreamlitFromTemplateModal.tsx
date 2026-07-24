// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Snowpark & Developer Workflows

import { useEffect, useMemo, useState } from "react";
import { Modal, Input, List, Typography, Button, Alert, Spin, Empty, Space, Result } from "antd";
import { AppstoreAddOutlined, CloudUploadOutlined, FolderOpenOutlined, SearchOutlined } from "@ant-design/icons";
import {
  ListStreamlitTemplates,
  CreateStreamlitFromTemplate,
  PickDirectory,
  RevealInFinder,
} from "../../../wailsjs/go/app/App";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import type { streamlittemplate } from "../../../wailsjs/go/models";

const { Text, Link, Paragraph } = Typography;

// Source repo for the templates. Attribution to it is required (Apache-2.0).
const REPO_URL = "https://github.com/Snowflake-Labs/snowflake-demo-streamlit";

interface Props {
  onClose: () => void;
  /** When provided, the success screen offers a "Deploy now" button that hands
   * the scaffolded folder to the caller (which opens the deploy modal pre-filled). */
  onDeployNow?: (destDir: string, templateName: string) => void;
}

export default function NewStreamlitFromTemplateModal({ onClose, onDeployNow }: Props) {
  const [templates, setTemplates] = useState<streamlittemplate.Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [degradedNote, setDegradedNote] = useState<string>("");
  const [loadError, setLoadError] = useState<string>("");

  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string>("");
  const [destDir, setDestDir] = useState("");

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>("");
  const [done, setDone] = useState(false);

  useEffect(() => {
    setLoading(true);
    ListStreamlitTemplates()
      .then((cat) => {
        setTemplates(cat.templates ?? []);
        setDegradedNote(cat.degraded ? cat.note : "");
      })
      .catch((e) => setLoadError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return templates;
    return templates.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        (t.description ?? "").toLowerCase().includes(q),
    );
  }, [templates, search]);

  const handleBrowse = async () => {
    let dir = "";
    try {
      dir = await PickDirectory();
    } catch {
      return;
    }
    if (dir) setDestDir(dir);
  };

  const canCreate = selected.length > 0 && destDir.trim().length > 0 && !creating;

  const handleCreate = async () => {
    if (!canCreate) return;
    setCreating(true);
    setCreateError("");
    try {
      await CreateStreamlitFromTemplate(selected, destDir);
      setDone(true);
    } catch (e) {
      setCreateError(String(e));
    } finally {
      setCreating(false);
    }
  };

  return (
    <Modal
      open
      title={<Space size={6}><AppstoreAddOutlined style={{ color: "var(--link)" }} /> New Streamlit from Template</Space>}
      width={640}
      onCancel={() => { if (!creating) onClose(); }}
      styles={{ body: { paddingTop: 12, maxHeight: "70vh", overflowY: "auto" } }}
      footer={
        done ? (
          <Button type="primary" onClick={onClose}>Done</Button>
        ) : (
          <Space style={{ justifyContent: "flex-end", display: "flex" }}>
            <Button onClick={onClose} disabled={creating}>Cancel</Button>
            <Button type="primary" icon={<AppstoreAddOutlined />} onClick={handleCreate} disabled={!canCreate} loading={creating}>
              Create
            </Button>
          </Space>
        )
      }
    >
      {done ? (
        <Result
          status="success"
          title="Template scaffolded"
          subTitle={
            <Text type="secondary" style={{ fontSize: 13 }}>
              {selected} → {destDir}
            </Text>
          }
          extra={[
            onDeployNow && (
              <Button
                key="deploy"
                type="primary"
                icon={<CloudUploadOutlined />}
                onClick={() => { onDeployNow(destDir, selected); onClose(); }}
              >
                Deploy now
              </Button>
            ),
            <Button key="reveal" icon={<FolderOpenOutlined />} onClick={() => RevealInFinder(destDir).catch(() => {})}>
              Open folder
            </Button>,
          ]}
        />
      ) : (
        <>
          {/* Attribution to the source repo is required (Apache-2.0). */}
          <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 12 }}>
            Templates from{" "}
            <Link onClick={() => BrowserOpenURL(REPO_URL)}>Snowflake-Labs/snowflake-demo-streamlit</Link>{" "}
            (Apache-2.0). The chosen app is downloaded to a local folder, along with the license and a
            provenance note; deploy it with “Deploy local Streamlit…”.
          </Paragraph>

          {degradedNote && (
            <Alert type="warning" showIcon closable style={{ marginBottom: 12 }} message={degradedNote} />
          )}
          {loadError && (
            <Alert type="error" showIcon style={{ marginBottom: 12 }} message="Couldn't load templates" description={loadError} />
          )}

          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="Search templates…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ marginBottom: 8 }}
          />

          <div style={{ border: "1px solid var(--border, #d9d9d9)", borderRadius: 6, maxHeight: 260, overflowY: "auto" }}>
            {loading ? (
              <div style={{ padding: 32, textAlign: "center" }}><Spin /></div>
            ) : filtered.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No templates" style={{ padding: 24 }} />
            ) : (
              <List
                size="small"
                dataSource={filtered}
                renderItem={(t) => {
                  const isSel = t.name === selected;
                  return (
                    <List.Item
                      onClick={() => setSelected(t.name)}
                      style={{
                        cursor: "pointer",
                        padding: "8px 12px",
                        // Theme-aware selection highlight (works in light & dark);
                        // a transparent border on unselected rows avoids a shift.
                        borderLeft: isSel ? "3px solid var(--accent)" : "3px solid transparent",
                        background: isSel ? "var(--bg-hover)" : undefined,
                      }}
                    >
                      <List.Item.Meta
                        title={<Text strong={isSel}>{t.name}</Text>}
                        description={
                          t.description
                            ? <Text type="secondary" style={{ fontSize: 12 }}>{t.description}</Text>
                            : <Text type="secondary" style={{ fontSize: 12, fontStyle: "italic" }}>No description</Text>
                        }
                      />
                    </List.Item>
                  );
                }}
              />
            )}
          </div>

          <div style={{ marginTop: 12 }}>
            <Text style={{ fontSize: 13 }}>Destination folder</Text>
            <Input
              value={destDir}
              readOnly
              placeholder="No folder selected — its contents must be empty"
              style={{ marginTop: 4 }}
              addonAfter={
                <Button type="text" size="small" icon={<FolderOpenOutlined />} onClick={handleBrowse} style={{ height: "auto", padding: 0 }}>
                  Browse…
                </Button>
              }
            />
          </div>

          {createError && (
            <Alert type="error" showIcon style={{ marginTop: 12 }} message="Scaffold failed" description={createError} closable onClose={() => setCreateError("")} />
          )}
        </>
      )}
    </Modal>
  );
}
