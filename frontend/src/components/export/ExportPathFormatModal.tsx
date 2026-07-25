// SPDX-License-Identifier: GPL-3.0-or-later

import { useState } from "react";
import { Modal, Button, Typography, Space, Radio, Tooltip } from "antd";
import { useGitStore } from "../../store/gitStore";
import {
  OVERLOAD_NAMING_OPTIONS,
  DEFAULT_OVERLOAD_NAMING,
  normalizeOverloadNaming,
  type OverloadNaming,
} from "./overloadNaming";
import PathTemplateField from "./PathTemplateField";
import { validateTemplate } from "./pathTemplate";

const { Text } = Typography;

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

  const namingOption = OVERLOAD_NAMING_OPTIONS.find((o) => o.value === naming)!;
  // A template missing the ".sql" extension is rejected, not fixed up — see
  // validateTemplate.
  const templateError = validateTemplate(value);

  function handleSave() {
    if (templateError) return;
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
          <Tooltip title={templateError ?? undefined}>
            <Button type="primary" onClick={handleSave} disabled={!!templateError}>
              Save
            </Button>
          </Tooltip>
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

        <PathTemplateField label="Template" value={value} onChange={setValue} />

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
      </Space>
    </Modal>
  );
}
