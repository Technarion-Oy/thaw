// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Form, Input, Select, Space, Typography, Alert, Button } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import { CloneTable, ListSchemas } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface Props {
  db: string;
  sourceSchema: string;
  sourceTable: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CloneTableModal({ db, sourceSchema, sourceTable, onClose, onSuccess }: Props) {
  const [targetSchema, setTargetSchema] = useState(sourceSchema);
  const [targetTable, setTargetTable] = useState(`${sourceTable}_CLONE`);
  const [schemas, setSchemas] = useState<string[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(true);
  const [cloning, setCloning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    ListSchemas(db)
      .then((res) => {
        setSchemas(res);
      })
      .catch((err) => {
        setError(`Failed to load schemas: ${err}`);
      })
      .finally(() => {
        setLoadingSchemas(false);
      });
  }, [db]);

  const handleClone = async () => {
    if (!targetTable.trim()) return;
    setCloning(true);
    setError(null);
    try {
      await CloneTable(db, sourceSchema, sourceTable, targetSchema, targetTable);
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setCloning(false);
    }
  };

  const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
  const previewSql = `CREATE TABLE ${q(db)}.${q(targetSchema)}.${q(targetTable.trim() || "new_table")} 
  CLONE ${q(db)}.${q(sourceSchema)}.${q(sourceTable)};`;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CopyOutlined style={{ color: "var(--link)" }} />
          <span>Clone table</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{sourceSchema}.{sourceTable}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={cloning}>Cancel</Button>
          <Button
            type="primary"
            icon={<CopyOutlined />}
            onClick={handleClone}
            disabled={!targetTable.trim() || loadingSchemas}
            loading={cloning}
          >
            Clone
          </Button>
        </Space>
      }
      width={500}
    >
      {error && (
        <Alert
          type="error"
          message="Cloning failed"
          description={error}
          showIcon
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <Form.Item label="Target Schema" required>
          <Select
            loading={loadingSchemas}
            value={targetSchema}
            onChange={setTargetSchema}
            showSearch
            style={{ width: "100%" }}
            options={schemas.map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>
        <Form.Item label="New Table Name" required>
          <Input
            value={targetTable}
            onChange={(e) => setTargetTable(e.target.value)}
            placeholder="TABLE_NAME_CLONE"
            autoFocus
          />
        </Form.Item>

        <div
          style={{
            padding: "10px 12px",
            background: "var(--bg)",
            borderRadius: 6,
            border: "1px solid var(--border)",
            marginTop: 16,
          }}
        >
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            SQL Preview
          </Text>
          <pre
            style={{
              margin: 0,
              color: "var(--text)",
              fontSize: 11,
              fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {previewSql}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
