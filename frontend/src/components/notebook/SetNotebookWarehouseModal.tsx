// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Select, Button, Space, Typography, message } from "antd";
import { DatabaseOutlined } from "@ant-design/icons";
import { SetNotebookQueryWarehouse } from "../../../wailsjs/go/app/App";
import { useSessionStore } from "../../store/sessionStore";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onSaved: (warehouse: string) => void;
  onClose: () => void;
}

export default function SetNotebookWarehouseModal({ db, schema, name, onSaved, onClose }: Props) {
  const { warehouse: sessionWarehouse, warehouses, loadWarehouses } = useSessionStore();
  const [selected, setSelected] = useState(sessionWarehouse);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    loadWarehouses();
  }, []);

  const handleSave = async () => {
    if (!selected) {
      message.warning("Please select a warehouse.");
      return;
    }
    setSaving(true);
    try {
      await SetNotebookQueryWarehouse(db, schema, name, selected);
      message.success(`Query warehouse set to ${selected}`);
      onSaved(selected);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <DatabaseOutlined style={{ color: "var(--link)" }} />
          <span>Set Query Warehouse</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" loading={saving} onClick={handleSave}>
            Save
          </Button>
        </Space>
      }
      width={420}
      styles={{ body: { paddingTop: 16, paddingBottom: 8 } }}
    >
      <Text style={{ fontSize: 13, display: "block", marginBottom: 6 }}>Query Warehouse</Text>
      <Select
        value={selected || undefined}
        onChange={setSelected}
        placeholder="Select warehouse"
        style={{ width: "100%" }}
        options={warehouses.map((w) => ({ label: w, value: w }))}
        autoFocus
      />
      <Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 8 }}>
        The warehouse used to execute the notebook's SQL cells.
      </Text>
    </Modal>
  );
}
