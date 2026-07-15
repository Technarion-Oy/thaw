// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Form, Input, Typography } from "antd";
import { EditOutlined } from "@ant-design/icons";
import { SetGitCommitFilter, GetGitCommitFilter } from "../../../wailsjs/go/app/App";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function SetGitCommitFilterModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [applying, setApplying] = useState(false);

  useEffect(() => {
    setLoading(true);
    GetGitCommitFilter(db, schema, name)
      .then((hash) => form.setFieldsValue({ hash }))
      .finally(() => setLoading(false));
  }, [db, schema, name, form]);

  const onFinish = async (values: { hash: string }) => {
    setApplying(true);
    try {
      await SetGitCommitFilter(db, schema, name, values.hash || "");
      if (onSuccess) onSuccess();
      onClose();
    } finally {
      setApplying(false);
    }
  };

  return (
    <Modal
      title={
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <EditOutlined />
          <Text strong>Set Commit Filter</Text>
        </div>
      }
      open
      onCancel={onClose}
      onOk={form.submit}
      okText="Apply Filter"
      confirmLoading={applying || loading}
      destroyOnClose
    >
      <div style={{ marginBottom: 16 }}>
        <Text type="secondary">
          Enter a full commit hash to view files at that specific version.
          The hash must be exactly what Snowflake expects in the <code>commits/</code> folder.
        </Text>
      </div>

      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ hash: "" }}>
        <Form.Item
          label="Commit Hash"
          name="hash"
          rules={[{ required: true, message: "Please enter a commit hash" }]}
        >
          <Input placeholder="e.g. 5d576a137683796d194c259837a7f45a03975549" allowClear />
        </Form.Item>
      </Form>
    </Modal>
  );
}
