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
// @thaw-domain: SQL Editor & Diagnostics

import { useState } from "react";
import { Modal, Select, Input, Button, Space, Typography } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { useGridStore, type ConditionalRule } from "../../store/gridStore";

const { Text } = Typography;

interface Props {
  columnId: string;
  columnName: string;
  onClose: () => void;
}

const PRESET_COLORS = [
  { label: "Green → Red", min: "#52c41a", max: "#f5222d" },
  { label: "Blue → Red", min: "#1677ff", max: "#f5222d" },
  { label: "White → Blue", min: "#ffffff", max: "#1677ff" },
  { label: "Yellow → Red", min: "#faad14", max: "#f5222d" },
];

export default function ConditionalFormattingModal({ columnId, columnName, onClose }: Props) {
  const existingRules = useGridStore((s) => s.conditionalRules[columnId] ?? []);
  const setConditionalRules = useGridStore((s) => s.setConditionalRules);
  const clearConditionalRules = useGridStore((s) => s.clearConditionalRules);

  const [rules, setRules] = useState<ConditionalRule[]>(existingRules);

  const addColorScale = () => {
    setRules([...rules, { type: "colorScale", minColor: "#52c41a", maxColor: "#f5222d" }]);
  };

  const addDataBar = () => {
    setRules([...rules, { type: "dataBar", color: "#1677ff" }]);
  };

  const addTextMatch = () => {
    setRules([...rules, { type: "textMatch", pattern: "", backgroundColor: "#fff2e8", textColor: "#d4380d" }]);
  };

  const removeRule = (idx: number) => {
    setRules(rules.filter((_, i) => i !== idx));
  };

  const updateRule = (idx: number, updated: ConditionalRule) => {
    setRules(rules.map((r, i) => (i === idx ? updated : r)));
  };

  const handleApply = () => {
    if (rules.length === 0) {
      clearConditionalRules(columnId);
    } else {
      setConditionalRules(columnId, rules);
    }
    onClose();
  };

  const handleClear = () => {
    clearConditionalRules(columnId);
    onClose();
  };

  return (
    <Modal
      title={`Conditional Formatting: ${columnName}`}
      open
      onCancel={onClose}
      footer={[
        <Button key="clear" onClick={handleClear}>Clear All</Button>,
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="apply" type="primary" onClick={handleApply}>Apply</Button>,
      ]}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 12, maxHeight: 400, overflowY: "auto" }}>
        {rules.map((rule, idx) => (
          <div
            key={idx}
            style={{
              padding: 10,
              border: "1px solid var(--border)",
              borderRadius: 6,
              background: "var(--bg-raised)",
            }}
          >
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <Text strong style={{ fontSize: 12 }}>
                {rule.type === "colorScale" ? "Color Scale" : rule.type === "dataBar" ? "Data Bar" : "Text Match"}
              </Text>
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined />}
                danger
                onClick={() => removeRule(idx)}
              />
            </div>

            {rule.type === "colorScale" && (
              <Space>
                <label style={{ fontSize: 11 }}>
                  Min:
                  <input
                    type="color"
                    value={rule.minColor}
                    onChange={(e) => updateRule(idx, { ...rule, minColor: e.target.value })}
                    style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                  />
                </label>
                <label style={{ fontSize: 11 }}>
                  Max:
                  <input
                    type="color"
                    value={rule.maxColor}
                    onChange={(e) => updateRule(idx, { ...rule, maxColor: e.target.value })}
                    style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                  />
                </label>
                <Select
                  size="small"
                  placeholder="Preset"
                  style={{ width: 120, fontSize: 11 }}
                  onChange={(v: string) => {
                    const preset = PRESET_COLORS.find((p) => p.label === v);
                    if (preset) updateRule(idx, { ...rule, minColor: preset.min, maxColor: preset.max });
                  }}
                  options={PRESET_COLORS.map((p) => ({ label: p.label, value: p.label }))}
                />
              </Space>
            )}

            {rule.type === "dataBar" && (
              <label style={{ fontSize: 11 }}>
                Color:
                <input
                  type="color"
                  value={rule.color}
                  onChange={(e) => updateRule(idx, { ...rule, color: e.target.value })}
                  style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                />
              </label>
            )}

            {rule.type === "textMatch" && (
              <Space direction="vertical" size={4} style={{ width: "100%" }}>
                <Input
                  size="small"
                  placeholder="Match text (e.g. FAILED)"
                  value={rule.pattern}
                  onChange={(e) => updateRule(idx, { ...rule, pattern: e.target.value })}
                  style={{ fontSize: 11 }}
                />
                <Space>
                  <label style={{ fontSize: 11 }}>
                    Background:
                    <input
                      type="color"
                      value={rule.backgroundColor}
                      onChange={(e) => updateRule(idx, { ...rule, backgroundColor: e.target.value })}
                      style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                    />
                  </label>
                  <label style={{ fontSize: 11 }}>
                    Text:
                    <input
                      type="color"
                      value={rule.textColor}
                      onChange={(e) => updateRule(idx, { ...rule, textColor: e.target.value })}
                      style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                    />
                  </label>
                </Space>
              </Space>
            )}
          </div>
        ))}

        {rules.length === 0 && (
          <Text style={{ color: "var(--text-muted)", fontSize: 12, textAlign: "center", padding: 16 }}>
            No formatting rules. Add one below.
          </Text>
        )}
      </div>

      <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
        <Button size="small" icon={<PlusOutlined />} onClick={addColorScale}>
          Color Scale
        </Button>
        <Button size="small" icon={<PlusOutlined />} onClick={addDataBar}>
          Data Bar
        </Button>
        <Button size="small" icon={<PlusOutlined />} onClick={addTextMatch}>
          Text Match
        </Button>
      </div>
    </Modal>
  );
}
