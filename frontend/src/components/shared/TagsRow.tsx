// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Button, Input, Space, Typography, Tag, Tooltip, Select } from "antd";
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
  // When provided, the tag-name field becomes a searchable dropdown of these
  // options (e.g. every tag defined in the account) instead of a free-text
  // input. Callers may still type a name not in the list. Omit for the original
  // free-text behaviour, so existing consumers are unaffected. When the selected
  // option carries a non-empty allowedValues list, the value field becomes a
  // dropdown restricted to those values (matching the ColumnPropertiesModal tag
  // editor) so a value the tag rejects can't be submitted.
  nameOptions?: { value: string; label: string; allowedValues?: string[] }[];
  // Drops the leading "Tags" label cell and spans the full row — for callers that
  // already label the editor with a section header (e.g. UserPropertiesModal's
  // two-column layout). Omit to keep the labelled two-cell row.
  hideLabel?: boolean;
}

// Shared inline tag editor row for the object properties modals: shows the
// current tags as chips (removable ones closable), plus a name/value pair and
// an Add Tag button. Both add and remove surface failures inline. When
// nameOptions is passed the name field is a searchable dropdown.
export default function TagsRow({ tags, onSetTag, onUnsetTag, nameOptions, hideLabel }: Props) {
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

  // Allowed values for the currently-selected tag (from its nameOptions entry).
  // Non-empty → the value field is a dropdown restricted to these values.
  const allowedValues = nameOptions?.find((o) => o.value === newName)?.allowedValues ?? [];

  // Selecting a different tag name clears the drafted value, so a value allowed
  // by the previous tag can't be carried over to one that rejects it.
  const pickName = (v: string) => { setNewName(v); setNewValue(""); };

  return (
    <tr>
      {!hideLabel && <td style={LABEL_TD}>Tags</td>}
      <td colSpan={hideLabel ? 2 : 1} style={{ padding: "6px 0", fontSize: 12, verticalAlign: "top" }}>
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
          <Space wrap>
            {nameOptions ? (
              <Select
                size="small"
                showSearch
                allowClear
                value={newName || undefined}
                onChange={(v) => pickName(v ?? "")}
                onSearch={(v) => pickName(v)}
                placeholder="Tag name"
                options={nameOptions}
                filterOption={(input, opt) => (opt?.label ?? "").toLowerCase().includes(input.toLowerCase())}
                style={{ width: 200 }}
              />
            ) : (
              <Input
                size="small"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Tag name"
                style={{ width: 140 }}
              />
            )}
            {allowedValues.length > 0 ? (
              <Select
                size="small"
                showSearch
                placeholder="Tag value"
                value={newValue || undefined}
                onChange={(v) => setNewValue(v ?? "")}
                options={allowedValues.map((v) => ({ value: v, label: v }))}
                filterOption={(input, opt) => (opt?.label ?? "").toLowerCase().includes(input.toLowerCase())}
                style={{ width: 160 }}
              />
            ) : (
              <Input
                size="small"
                value={newValue}
                onChange={(e) => setNewValue(e.target.value)}
                placeholder="Tag value"
                style={{ width: 160 }}
                onPressEnter={addTag}
              />
            )}
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
