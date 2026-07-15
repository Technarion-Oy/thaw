// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useState, useRef } from "react";
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

  // IDs are only used as React keys within this modal. Start the counter
  // past the initial rule IDs (0..n-1) so newly added rules never collide.
  const nextId = useRef(existingRules.length);
  const [rules, setRules] = useState(() =>
    existingRules.map((rule, i) => ({ id: i, rule })),
  );

  const addColorScale = () => {
    setRules([...rules, { id: nextId.current++, rule: { type: "colorScale" as const, minColor: "#52c41a", maxColor: "#f5222d" } }]);
  };

  const addDataBar = () => {
    setRules([...rules, { id: nextId.current++, rule: { type: "dataBar" as const, color: "#1677ff" } }]);
  };

  const addTextMatch = () => {
    setRules([...rules, { id: nextId.current++, rule: { type: "textMatch" as const, pattern: "", backgroundColor: "#fff2e8", textColor: "#d4380d" } }]);
  };

  const removeRule = (id: number) => {
    setRules(rules.filter((e) => e.id !== id));
  };

  const updateRule = (id: number, updated: ConditionalRule) => {
    setRules(rules.map((e) => (e.id === id ? { ...e, rule: updated } : e)));
  };

  const handleApply = () => {
    const extracted = rules.map((e) => e.rule);
    if (extracted.length === 0) {
      clearConditionalRules(columnId);
    } else {
      setConditionalRules(columnId, extracted);
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
        {rules.map((entry) => {
          const rule = entry.rule;
          return (
          <div
            key={entry.id}
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
                onClick={() => removeRule(entry.id)}
              />
            </div>

            {rule.type === "colorScale" && (
              <Space>
                <label style={{ fontSize: 11 }}>
                  Min:
                  <input
                    type="color"
                    value={rule.minColor}
                    onChange={(e) => updateRule(entry.id, { ...rule, minColor: e.target.value })}
                    style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                  />
                </label>
                <label style={{ fontSize: 11 }}>
                  Max:
                  <input
                    type="color"
                    value={rule.maxColor}
                    onChange={(e) => updateRule(entry.id, { ...rule, maxColor: e.target.value })}
                    style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                  />
                </label>
                <Select
                  size="small"
                  placeholder="Preset"
                  style={{ width: 120, fontSize: 11 }}
                  onChange={(v: string) => {
                    const preset = PRESET_COLORS.find((p) => p.label === v);
                    if (preset) updateRule(entry.id, { ...rule, minColor: preset.min, maxColor: preset.max });
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
                  onChange={(e) => updateRule(entry.id, { ...rule, color: e.target.value })}
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
                  onChange={(e) => updateRule(entry.id, { ...rule, pattern: e.target.value })}
                  style={{ fontSize: 11 }}
                />
                <Space>
                  <label style={{ fontSize: 11 }}>
                    Background:
                    <input
                      type="color"
                      value={rule.backgroundColor}
                      onChange={(e) => updateRule(entry.id, { ...rule, backgroundColor: e.target.value })}
                      style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                    />
                  </label>
                  <label style={{ fontSize: 11 }}>
                    Text:
                    <input
                      type="color"
                      value={rule.textColor}
                      onChange={(e) => updateRule(entry.id, { ...rule, textColor: e.target.value })}
                      style={{ marginLeft: 4, width: 30, height: 20, border: "none", cursor: "pointer" }}
                    />
                  </label>
                </Space>
              </Space>
            )}
          </div>
          );
        })}

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
