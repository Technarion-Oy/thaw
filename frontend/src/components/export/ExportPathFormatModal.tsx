// SPDX-License-Identifier: GPL-3.0-or-later

import { useState } from "react";
import { Modal, Input, Button, Typography, Space, Tag, Radio } from "antd";
import { useGitStore } from "../../store/gitStore";
import {
  OVERLOAD_NAMING_OPTIONS,
  DEFAULT_OVERLOAD_NAMING,
  normalizeOverloadNaming,
  type OverloadNaming,
} from "./overloadNaming";

const { Text } = Typography;

const DEFAULT_TEMPLATE = "{database}/{schema}/{object_type}/{object_name}.sql";

const VARIABLES = [
  { name: "{database}",    desc: "Sanitized database name" },
  { name: "{schema}",      desc: "Sanitized schema name" },
  { name: "{object_type}", desc: "Object type directory (tables, views, …)" },
  { name: "{object_name}", desc: "Sanitized object name" },
];

const EXAMPLE_VALUES: Record<string, string> = {
  "{database}":    "MY_DATABASE",
  "{schema}":      "PUBLIC",
  "{object_type}": "tables",
  "{object_name}": "MY_TABLE",
};

function applyTemplate(template: string): string {
  let result = template || DEFAULT_TEMPLATE;
  for (const [k, v] of Object.entries(EXAMPLE_VALUES)) {
    result = result.split(k).join(v);
  }
  return result;
}

interface Props { onClose: () => void; }

export default function ExportPathFormatModal({ onClose }: Props) {
  const {
    exportPathTemplate, saveExportPathTemplate,
    exportOverloadNaming, saveExportOverloadNaming,
  } = useGitStore();
  const [value, setValue] = useState(exportPathTemplate || "");
  const [naming, setNaming] = useState<OverloadNaming>(
    normalizeOverloadNaming(exportOverloadNaming),
  );

  const effective = value.trim() || DEFAULT_TEMPLATE;
  const preview = applyTemplate(effective);
  const namingOption = OVERLOAD_NAMING_OPTIONS.find((o) => o.value === naming)!;

  function handleSave() {
    saveExportPathTemplate(value.trim());
    saveExportOverloadNaming(naming);
    onClose();
  }

  function handleReset() {
    setValue("");
    setNaming(DEFAULT_OVERLOAD_NAMING);
  }

  return (
    <Modal
      open
      title="Export Path Format"
      width={620}
      onCancel={onClose}
      footer={
        <Space>
          <Button onClick={handleReset}>Reset to Default</Button>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" onClick={handleSave}>Save</Button>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: "100%", gap: 16 }}>
        <div>
          <Text type="secondary" style={{ fontSize: 12 }}>
            Define the file path template and the overload naming used when exporting DDL.
            Leave the template blank to use the default.
          </Text>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Template</Text>
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={DEFAULT_TEMPLATE}
            style={{ fontFamily: "monospace" }}
          />
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Available variables</Text>
          <Space wrap>
            {VARIABLES.map((v) => (
              <span key={v.name} title={v.desc}>
                <Tag
                  style={{ cursor: "pointer", fontFamily: "monospace" }}
                  onClick={() => setValue((prev) => (prev || DEFAULT_TEMPLATE) + v.name)}
                >
                  {v.name}
                </Tag>
                <Text type="secondary" style={{ fontSize: 12 }}>{v.desc}</Text>
              </span>
            ))}
          </Space>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>
            Overloaded functions &amp; procedures
          </Text>
          <Radio.Group
            value={naming}
            onChange={(e) => setNaming(e.target.value as OverloadNaming)}
            optionType="button"
            buttonStyle="solid"
            size="small"
            options={OVERLOAD_NAMING_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
          />
          <Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 6 }}>
            {namingOption.hint} Same name, different argument signatures — e.g.{" "}
            <span style={{ fontFamily: "monospace" }}>FOO(X VARCHAR(16))</span> and{" "}
            <span style={{ fontFamily: "monospace" }}>FOO(X VARCHAR(256))</span> become{" "}
            <span style={{ fontFamily: "monospace" }}>{namingOption.example}</span>.
          </Text>
        </div>

        <div>
          <Text strong style={{ display: "block", marginBottom: 6 }}>Preview</Text>
          <div
            style={{
              fontFamily: "monospace",
              fontSize: 12,
              padding: "8px 12px",
              background: "var(--bg-raised)",
              borderRadius: 4,
              border: "1px solid var(--border)",
              color: "var(--text)",
              overflowX: "auto",
              whiteSpace: "nowrap",
            }}
          >
            {preview}
          </div>
        </div>
      </Space>
    </Modal>
  );
}
