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
import { Button, Modal, Typography, message } from "antd";
import { GetFeatureFlags, SaveFeatureFlags } from "../../../wailsjs/go/main/App";
import type { config } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  onClose: () => void;
}

export default function FeatureFlagsModal({ onClose }: Props) {
  const [flags, setFlags] = useState<config.FeatureFlags>({} as config.FeatureFlags);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    GetFeatureFlags().then((f) => setFlags(f));
  }, []);

  async function handleSave() {
    setSaving(true);
    try {
      await SaveFeatureFlags(flags);
      message.success("Feature flags saved");
      onClose();
    } catch (err) {
      message.error(String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Feature Flags"
      open
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>Cancel</Button>,
        <Button key="save" type="primary" loading={saving} onClick={handleSave}>Save</Button>,
      ]}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 16, paddingTop: 8 }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          No feature flags are defined yet. Future experimental features will appear here and
          can be individually enabled or disabled.
        </Text>
      </div>
    </Modal>
  );
}
