// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { Modal, Select, Segmented, Divider, Button, Space, Typography } from "antd";
import {
  useThemeStore,
  UI_FONTS,
  EDITOR_FONTS,
  EDITOR_FONT_SIZES,
  type UIDensity,
} from "../../store/themeStore";
import { usePanelLayoutStore } from "../../store/panelLayoutStore";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

// Preset configurations
const PRESETS: Array<{
  label: string;
  uiFont: string;
  editorFont: string;
  editorFontSize: number;
  uiDensity: UIDensity;
}> = [
  {
    label: "Modern",
    uiFont:         UI_FONTS[0].value,   // Inter
    editorFont:     EDITOR_FONTS[0].value,
    editorFontSize: 14,
    uiDensity:      "default",
  },
  {
    label: "Classic",
    uiFont:         UI_FONTS[3].value,   // Roboto
    editorFont:     EDITOR_FONTS[4].value, // Courier New
    editorFontSize: 13,
    uiDensity:      "compact",
  },
  {
    label: "Comfortable",
    uiFont:         UI_FONTS[0].value,   // Inter
    editorFont:     EDITOR_FONTS[0].value,
    editorFontSize: 15,
    uiDensity:      "comfortable",
  },
];

export default function LayoutSettingsModal({ onClose }: Props) {
  const uiFont         = useThemeStore((s) => s.uiFont);
  const editorFont     = useThemeStore((s) => s.editorFont);
  const editorFontSize = useThemeStore((s) => s.editorFontSize);
  const uiDensity      = useThemeStore((s) => s.uiDensity);
  const setUiFont         = useThemeStore((s) => s.setUiFont);
  const setEditorFont     = useThemeStore((s) => s.setEditorFont);
  const setEditorFontSize = useThemeStore((s) => s.setEditorFontSize);
  const setUIDensity      = useThemeStore((s) => s.setUIDensity);
  const resetPanelLayout  = usePanelLayoutStore((s) => s.reset);

  const applyPreset = (preset: (typeof PRESETS)[number]) => {
    setUiFont(preset.uiFont);
    setEditorFont(preset.editorFont);
    setEditorFontSize(preset.editorFontSize);
    setUIDensity(preset.uiDensity);
  };

  return (
    <Modal
      open
      title="Customize Layout"
      onCancel={onClose}
      footer={[
        <Button key="reset" onClick={resetPanelLayout}>
          Reset Layout
        </Button>,
        <Button key="close" type="primary" onClick={onClose}>
          Done
        </Button>,
      ]}
      width={460}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 20, padding: "8px 0" }}>
        {/* Presets */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
            PRESETS
          </Text>
          <Space>
            {PRESETS.map((p) => (
              <Button key={p.label} size="small" onClick={() => applyPreset(p)}>
                {p.label}
              </Button>
            ))}
          </Space>
        </div>

        <Divider style={{ margin: 0 }} />

        {/* UI font family */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
            UI FONT
          </Text>
          <Select
            value={uiFont}
            onChange={setUiFont}
            style={{ width: "100%" }}
            options={UI_FONTS.map((f) => ({
              label: <span style={{ fontFamily: f.value }}>{f.label}</span>,
              value: f.value,
            }))}
          />
          {/* Preview */}
          <div
            style={{
              fontFamily: uiFont,
              fontSize: 13,
              marginTop: 8,
              padding: "8px 12px",
              borderRadius: 6,
              background: "var(--bg-raised)",
              border: "1px solid var(--border)",
              color: "var(--text)",
              lineHeight: 1.6,
            }}
          >
            Buttons · Labels · Menus · Dialogs · 0123456789
          </div>
        </div>

        <Divider style={{ margin: 0 }} />

        {/* Editor font family */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
            EDITOR FONT
          </Text>
          <Select
            value={editorFont}
            onChange={setEditorFont}
            style={{ width: "100%" }}
            options={EDITOR_FONTS.map((f) => ({
              label: (
                <span style={{ fontFamily: f.value }}>{f.label}</span>
              ),
              value: f.value,
            }))}
          />
          {/* Preview */}
          <div
            style={{
              fontFamily: editorFont,
              fontSize: editorFontSize,
              marginTop: 8,
              padding: "8px 12px",
              borderRadius: 6,
              background: "var(--bg-raised)",
              border: "1px solid var(--border)",
              color: "var(--text)",
              whiteSpace: "pre",
              lineHeight: 1.6,
            }}
          >
            {"SELECT id, name\nFROM orders\nWHERE status = 'active';"}
          </div>
        </div>

        {/* Editor font size */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
            EDITOR FONT SIZE
          </Text>
          <Segmented
            value={editorFontSize}
            onChange={(v) => setEditorFontSize(v as number)}
            options={EDITOR_FONT_SIZES.map((s) => ({ label: String(s), value: s }))}
          />
        </div>

        {/* UI density */}
        <div>
          <Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
            TABLE ROW DENSITY
          </Text>
          <Segmented
            value={uiDensity}
            onChange={(v) => setUIDensity(v as UIDensity)}
            options={[
              { label: "Compact",     value: "compact" },
              { label: "Default",     value: "default" },
              { label: "Comfortable", value: "comfortable" },
            ]}
          />
        </div>
      </div>
    </Modal>
  );
}
