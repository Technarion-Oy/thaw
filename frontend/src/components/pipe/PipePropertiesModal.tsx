// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  ApiOutlined, EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterPipe } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// ─── EditRow ─────────────────────────────────────────────────────────────────

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
                <Button
                  size="small"
                  icon={<CheckOutlined />}
                  type="primary"
                  onClick={save}
                  loading={saving}
                />
              </Tooltip>
              {canUnset && onUnset && (
                <Tooltip title="Unset (remove)">
                  <Button size="small" onClick={unset} loading={saving}>Unset</Button>
                </Tooltip>
              )}
              <Tooltip title="Cancel">
                <Button
                  size="small"
                  icon={<CloseOutlined />}
                  onClick={() => { setEditing(false); setDraft(value); setError(null); }}
                />
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

// ─── Tag editor ──────────────────────────────────────────────────────────────

interface TagsRowProps {
  tags: { name: string; value: string }[];
  onSetTag: (name: string, value: string) => Promise<void>;
  onUnsetTag: (name: string) => Promise<void>;
}

function TagsRow({ tags, onSetTag, onUnsetTag }: TagsRowProps) {
  const [newName, setNewName] = useState("");
  const [newValue, setNewValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const addTag = async () => {
    if (!newName.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onSetTag(newName.trim(), newValue.trim());
      setNewName("");
      setNewValue("");
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>Tags</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "top" }}>
        <Space direction="vertical" size={6} style={{ width: "100%" }}>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {tags.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}>(none)</Text>}
            {tags.map((t) => (
              <Tag
                key={t.name}
                closable
                onClose={async (e) => {
                  e.preventDefault();
                  await onUnsetTag(t.name);
                }}
              >
                {t.name}: {t.value}
              </Tag>
            ))}
          </div>
          <Space>
            <Input
              size="small"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Tag name"
              style={{ width: 140 }}
            />
            <Input
              size="small"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
              placeholder="Tag value"
              style={{ width: 160 }}
              onPressEnter={addTag}
            />
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={addTag}
              loading={saving}
              disabled={!newName.trim()}
            >
              Add Tag
            </Button>
          </Space>
          {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
        </Space>
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

export default function PipePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<main.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tags, setTags] = useState<{ name: string; value: string }[]>([]);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "PIPE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const pipeRef = `"${db}"."${schema}"."${name}"`;

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterPipe(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterPipe(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const setTag = async (tagName: string, tagValue: string) => {
    await AlterPipe(db, schema, name, `SET TAG "${tagName.replace(/"/g, '""')}" = ${q1(tagValue)}`);
    setTags((prev) => {
      const next = prev.filter((t) => t.name !== tagName);
      return [...next, { name: tagName, value: tagValue }];
    });
  };

  const unsetTag = async (tagName: string) => {
    await AlterPipe(db, schema, name, `UNSET TAG "${tagName.replace(/"/g, '""')}"`);
    setTags((prev) => prev.filter((t) => t.name !== tagName));
  };

  const comment = rows ? (rows.find((r) => r.key.toUpperCase() === "COMMENT")?.value ?? "") : "";
  const readOnlyKeys = new Set(["COMMENT"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApiOutlined style={{ color: "var(--link)" }} />
          <span>Pipe Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {pipeRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={680}
      styles={{ body: { maxHeight: "72vh", overflowY: "auto", paddingTop: 16 } }}
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
          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !readOnlyKeys.has(r.key.toUpperCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>

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
              <TagsRow
                tags={tags}
                onSetTag={setTag}
                onUnsetTag={unsetTag}
              />
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
