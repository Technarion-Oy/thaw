// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Button, Input, Space, Typography, Tag, Tooltip } from "antd";
import { PlusOutlined } from "@ant-design/icons";

const { Text } = Typography;

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top",
  width: 220,
};

// A single tag chip. `key` is the opaque identifier passed back to onUnsetTag
// (a bare tag name for objects that don't qualify, or a fully-qualified
// "db"."schema"."tag" path). `removable` false renders a non-closable chip —
// used for tags inherited from a higher level, which can't be unset here.
export interface EditableTag {
  key: string;
  name: string;
  value: string;
  removable?: boolean;
  suffix?: string;
}

interface Props {
  tags: EditableTag[];
  onSetTag: (name: string, value: string) => Promise<void>;
  onUnsetTag: (key: string) => Promise<void>;
}

// Shared inline tag editor row for the object properties modals: shows the
// current tags as chips (removable ones closable), plus a name/value pair and
// an Add Tag button. Both add and remove surface failures inline.
export default function TagsRow({ tags, onSetTag, onUnsetTag }: Props) {
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

  const removeTag = async (key: string) => {
    setError(null);
    try {
      await onUnsetTag(key);
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>Tags</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "top" }}>
        <Space direction="vertical" size={6} style={{ width: "100%" }}>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {tags.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}>(none)</Text>}
            {tags.map((t) => {
              const closable = t.removable !== false;
              const chip = (
                <Tag
                  key={t.key}
                  closable={closable}
                  onClose={(e) => { e.preventDefault(); removeTag(t.key); }}
                >
                  {t.name}: {t.value}{t.suffix ?? ""}
                </Tag>
              );
              return closable ? chip : (
                <Tooltip key={t.key} title="Inherited — unset it where it was applied">
                  {chip}
                </Tooltip>
              );
            })}
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
