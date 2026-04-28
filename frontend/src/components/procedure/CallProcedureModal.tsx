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
import { Modal, Form, Input, Select, Button, Space, Typography, Tag, Spin } from "antd";
import { PlayCircleOutlined } from "@ant-design/icons";
import { GetProcedureParams, BuildCallStatement } from "../../../wailsjs/go/main/App";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

interface Param {
  name: string;
  dataType: string;
}

function isBoolean(dataType: string): boolean {
  const base = dataType.replace(/\(.*\)/, "").trim().toUpperCase();
  return base === "BOOLEAN" || base === "BOOL";
}

function isNumeric(dataType: string): boolean {
  const base = dataType.replace(/\(.*\)/, "").trim().toUpperCase();
  return /^(NUMBER|INT|INTEGER|BIGINT|SMALLINT|TINYINT|BYTEINT|FLOAT|DOUBLE|DECIMAL|NUMERIC|REAL)$/.test(base);
}

function needsQuotes(dataType: string): boolean {
  return !isBoolean(dataType) && !isNumeric(dataType);
}

interface Props {
  db: string;
  schema: string;
  name: string;
  rawArgs: string;
  onClose: () => void;
}

export default function CallProcedureModal({ db, schema, name, rawArgs, onClose }: Props) {
  const [params, setParams] = useState<Param[] | null>(null);
  const [values, setValues] = useState<string[]>([]);
  const [preview, setPreview] = useState("");
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    GetProcedureParams(db, schema, name, rawArgs)
      .then((result) => {
        setParams(result ?? []);
        setValues((result ?? []).map(() => ""));
      })
      .catch(() => {
        // Fallback: no params available
        setParams([]);
        setValues([]);
      });
  }, [db, schema, name, rawArgs]);

  useEffect(() => {
    if (!params) return;
    const args = params.map((p, i) => ({
      name: p.name,
      dataType: p.dataType,
      value: values[i] || ""
    }));
    BuildCallStatement(db, schema, name, args).then(setPreview);
  }, [db, schema, name, params, values]);

  const setValue = (i: number, v: string) =>
    setValues((prev) => { const next = [...prev]; next[i] = v; return next; });

  const handleExecute = () => {
    if (!params || !preview) return;
    onClose();
    executeInNewTab(preview);
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <PlayCircleOutlined style={{ color: "var(--link)" }} />
          <span>Call procedure</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleExecute} disabled={!params}>
            Execute
          </Button>
        </Space>
      }
      width={540}
      styles={{ body: { paddingTop: 16 } }}
    >
      {params === null ? (
        <div style={{ textAlign: "center", padding: "24px 0" }}>
          <Spin />
        </div>
      ) : params.length === 0 ? (
        <Text type="secondary">This procedure takes no parameters.</Text>
      ) : (
        <Form layout="vertical">
          {params.map((p, i) => (
            <Form.Item
              key={i}
              label={
                <Space size={6}>
                  <Text strong style={{ fontSize: 13 }}>{p.name}</Text>
                  <Tag style={{ fontSize: 11, margin: 0, fontFamily: "monospace" }}>{p.dataType}</Tag>
                </Space>
              }
              style={{ marginBottom: 12 }}
            >
              {isBoolean(p.dataType) ? (
                <Select
                  value={values[i] || undefined}
                  placeholder="Select value"
                  allowClear
                  onChange={(v) => setValue(i, v ?? "")}
                  options={[
                    { value: "TRUE", label: "TRUE" },
                    { value: "FALSE", label: "FALSE" },
                  ]}
                />
              ) : (
                <Input
                  value={values[i]}
                  onChange={(e) => setValue(i, e.target.value)}
                  placeholder={needsQuotes(p.dataType) ? "text — quotes added automatically" : "numeric value"}
                  onPressEnter={handleExecute}
                />
              )}
            </Form.Item>
          ))}
        </Form>
      )}

      {/* Live preview */}
      {params !== null && (
        <div
          style={{
            marginTop: 8,
            padding: "10px 12px",
            background: "var(--bg)",
            borderRadius: 6,
            border: "1px solid var(--border)",
          }}
        >
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            Preview
          </Text>
          <pre
            style={{
              margin: 0,
              color: "var(--text)",
              fontSize: 12,
              fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {preview}
          </pre>
        </div>
      )}
    </Modal>
  );
}
