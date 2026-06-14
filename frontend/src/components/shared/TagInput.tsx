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

import { useState } from "react";
import { Form, Input, Button, Space, Tag } from "antd";
import { PlusOutlined } from "@ant-design/icons";

export interface TagItem {
  name: string;
  value: string;
}

interface Props {
  tags: TagItem[];
  onChange: (tags: TagItem[]) => void;
  label?: string;
  help?: string;
  itemStyle?: React.CSSProperties;
}

/**
 * Object-level tag editor: a list of removable chips plus name/value inputs and
 * an Add button. The draft inputs are owned internally; only the committed tag
 * list is surfaced via `onChange`. Adding a tag whose name already exists
 * replaces that tag's value.
 */
export default function TagInput({ tags, onChange, label = "Tags", help, itemStyle }: Props) {
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");

  const addTag = () => {
    const name = tagName.trim();
    if (!name) return;
    onChange([...(tags ?? []).filter((t) => t.name !== name), { name, value: tagValue.trim() }]);
    setTagName("");
    setTagValue("");
  };

  const removeTag = (name: string) => onChange((tags ?? []).filter((t) => t.name !== name));

  return (
    <Form.Item label={label} style={itemStyle ?? { marginBottom: 4 }} help={help}>
      <Space direction="vertical" size={6} style={{ width: "100%" }}>
        {(tags ?? []).length > 0 && (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {(tags ?? []).map((t) => (
              <Tag key={t.name} closable onClose={(e) => { e.preventDefault(); removeTag(t.name); }}>
                {t.name}{t.value ? `: ${t.value}` : ""}
              </Tag>
            ))}
          </div>
        )}
        <Space>
          <Input
            size="small"
            value={tagName}
            onChange={(e) => setTagName(e.target.value)}
            placeholder="Tag name"
            style={{ width: 160 }}
          />
          <Input
            size="small"
            value={tagValue}
            onChange={(e) => setTagValue(e.target.value)}
            placeholder="Tag value"
            style={{ width: 180 }}
            onPressEnter={addTag}
          />
          <Button size="small" icon={<PlusOutlined />} onClick={addTag} disabled={!tagName.trim()}>
            Add
          </Button>
        </Space>
      </Space>
    </Form.Item>
  );
}
