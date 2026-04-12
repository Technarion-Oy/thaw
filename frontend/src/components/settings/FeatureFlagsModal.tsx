// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { Button, Modal, Switch, Typography, message } from "antd";
import { GetFeatureFlags, SaveFeatureFlags } from "../../../wailsjs/go/main/App";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import type { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

// ─── Feature row ─────────────────────────────────────────────────────────────
// Renders a single labelled toggle. Replicate this pattern for each new flag.

interface FlagRowProps {
  label: string;
  description: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}

function FlagRow({ label, description, checked, onChange }: FlagRowProps) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16 }}>
      <div>
        <Text style={{ fontSize: 13 }}>{label}</Text>
        {description && (
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>{description}</div>
        )}
      </div>
      <Switch checked={checked} onChange={onChange} size="small" style={{ flexShrink: 0, marginTop: 2 }} />
    </div>
  );
}

// ─── Modal ────────────────────────────────────────────────────────────────────

export default function FeatureFlagsModal({ onClose }: Props) {
  const [flags, setFlags] = useState<config.FeatureFlags>({ initialized: true, exportTableData: true });
  const [saving, setSaving] = useState(false);
  const loadStore = useFeatureFlagsStore((s) => s.load);

  useEffect(() => {
    GetFeatureFlags().then((f) => setFlags(f));
  }, []);

  function set<K extends keyof config.FeatureFlags>(key: K, value: config.FeatureFlags[K]) {
    setFlags((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    setSaving(true);
    try {
      await SaveFeatureFlags(flags);
      await loadStore(); // propagate changes to all consumers immediately
      message.success("Enabled features saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Enabled Features"
      open
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 18, paddingTop: 8 }}>

        {/* ── Add a <FlagRow> here for each new feature flag ── */}

        <FlagRow
          label="Export Table Data"
          description="Allow exporting table data to local files (CSV, JSON, Parquet)."
          checked={flags.exportTableData}
          onChange={(v) => set("exportTableData", v)}
        />

      </div>
    </Modal>
  );
}
