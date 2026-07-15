// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import type { ReactNode } from "react";
import { Form, Input, Checkbox, Space } from "antd";

interface Props {
  /** Field label, e.g. "Secret name". */
  label: string;
  placeholder?: string;
  name: string;
  onNameChange: (v: string) => void;
  orReplace: boolean;
  ifNotExists: boolean;
  onOrReplaceChange: (v: boolean) => void;
  onIfNotExistsChange: (v: boolean) => void;
  /** Extra checkboxes rendered beneath IF NOT EXISTS, e.g. TRANSIENT. */
  extra?: ReactNode;
}

/**
 * The shared "object name + OR REPLACE / IF NOT EXISTS" header row used by most
 * create modals. OR REPLACE and IF NOT EXISTS are mutually exclusive (ticking
 * OR REPLACE clears IF NOT EXISTS; IF NOT EXISTS is disabled while OR REPLACE is
 * on), matching Snowflake DDL semantics.
 */
export default function NameWithReplaceOptions({
  label,
  placeholder,
  name,
  onNameChange,
  orReplace,
  ifNotExists,
  onOrReplaceChange,
  onIfNotExistsChange,
  extra,
}: Props) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
      <Form.Item label={label} required style={{ marginBottom: 4 }}>
        <Input
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder={placeholder}
        />
      </Form.Item>
      <Form.Item style={{ marginBottom: 4 }}>
        <Space direction="vertical" size={4}>
          <Checkbox
            checked={orReplace}
            onChange={(e) => {
              onOrReplaceChange(e.target.checked);
              if (e.target.checked) onIfNotExistsChange(false);
            }}
          >
            OR REPLACE
          </Checkbox>
          <Checkbox
            checked={ifNotExists}
            disabled={orReplace}
            onChange={(e) => onIfNotExistsChange(e.target.checked)}
          >
            IF NOT EXISTS
          </Checkbox>
          {extra}
        </Space>
      </Form.Item>
    </div>
  );
}
