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
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Table,
} from "antd";
import {
  ContainerOutlined, EditOutlined, CheckOutlined, CloseOutlined, CopyOutlined, ReloadOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterImageRepository, ListImagesInRepository } from "../../../wailsjs/go/app/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal (doubles embedded single quotes).
const q1 = (s: string) => `'${s.replace(/'/g, "''")}'`;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 200,
};

// ─── EditRow (single-line settings, e.g. comment) ────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({ label, value, canUnset, onSave, onUnset }: EditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draft);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const unset = async () => {
    if (!onUnset) return;
    setSaving(true);
    setError(null);
    try {
      await onUnset();
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Input
                size="small"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                style={{ width: 280 }}
                onPressEnter={save}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} />
              </Tooltip>
              {canUnset && onUnset && (
                <Tooltip title="Unset (remove)">
                  <Button size="small" onClick={unset} loading={saving}>Unset</Button>
                </Tooltip>
              )}
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button
                type="text"
                size="small"
                icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={() => { setDraft(value); setEditing(true); }}
                style={{ color: "var(--text-muted)" }}
              />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function ImageRepositoryPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Lazily-loaded image listing (SHOW IMAGES IN IMAGE REPOSITORY).
  const [images, setImages] = useState<snowflake.QueryResult | null>(null);
  const [imagesLoading, setImagesLoading] = useState(false);
  const [imagesError, setImagesError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "IMAGE REPOSITORY", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const repoRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const saveComment = async (comment: string) => {
    setActionError(null);
    try {
      if (comment.trim() === "") {
        await AlterImageRepository(db, schema, name, "UNSET COMMENT");
      } else {
        await AlterImageRepository(db, schema, name, `SET COMMENT = ${q1(comment)}`);
      }
      await reload();
    } catch (e) {
      setActionError(`Update comment failed: ${String(e)}`);
      throw e;
    }
  };

  const loadImages = useCallback(async () => {
    setImagesLoading(true);
    setImagesError(null);
    try {
      const res = await ListImagesInRepository(db, schema, name);
      setImages(res ?? null);
    } catch (e) {
      setImagesError(String(e));
    } finally {
      setImagesLoading(false);
    }
  }, [db, schema, name]);

  const comment = find("comment");
  const repositoryUrl = find("repository_url");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "repository_url"]);

  // Build antd Table columns/data from the QueryResult shape.
  const imageColumns = (images?.columns ?? []).map((col, idx) => ({
    title: col,
    dataIndex: String(idx),
    key: String(idx),
    ellipsis: true,
    render: (v: unknown) => (
      <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v == null ? "" : String(v)}</span>
    ),
  }));
  const imageData = (images?.rows ?? []).map((row, ri) => {
    const obj: Record<string, unknown> = { key: ri };
    row.forEach((cell, ci) => { obj[String(ci)] = cell; });
    return obj;
  });

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ContainerOutlined style={{ color: "var(--link)" }} />
          <span>Image Repository Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {repoRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={820}
      styles={{ body: { maxHeight: "74vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin />
        </div>
      )}
      {error && (
        <Alert type="error" message="Failed to load properties" description={error} showIcon />
      )}
      {rows && (
        <>
          {actionError && (
            <Alert
              type="error"
              message={actionError}
              showIcon
              closable
              onClose={() => setActionError(null)}
              style={{ marginBottom: 12 }}
            />
          )}

          <div style={SECTION_HEAD}>Repository URL</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Push and pull container images using this URL (assigned by Snowflake).
          </Text>
          {repositoryUrl ? (
            <Space>
              <Text style={{ fontFamily: "var(--font-mono)", fontSize: 12, wordBreak: "break-all" }}>
                {repositoryUrl}
              </Text>
              <Tooltip title="Copy URL">
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined style={{ fontSize: 12 }} />}
                  onClick={() => ClipboardSetText(repositoryUrl)}
                />
              </Tooltip>
            </Space>
          ) : (
            <Text type="secondary">(unavailable)</Text>
          )}

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Images</div>
          {imagesError && (
            <Alert type="error" message="Failed to load images" description={imagesError} showIcon style={{ marginBottom: 8 }} />
          )}
          {images ? (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {imageData.length === 0 ? "No images in this repository." : `${imageData.length} image${imageData.length === 1 ? "" : "s"}.`}
                </Text>
                <Button size="small" icon={<ReloadOutlined />} onClick={loadImages} loading={imagesLoading}>
                  Refresh
                </Button>
              </Space>
              {imageData.length > 0 && (
                <Table
                  size="small"
                  columns={imageColumns}
                  dataSource={imageData}
                  pagination={false}
                  scroll={{ x: true }}
                />
              )}
            </>
          ) : (
            <Button size="small" icon={<ReloadOutlined />} onClick={loadImages} loading={imagesLoading}>
              Load images
            </Button>
          )}

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
