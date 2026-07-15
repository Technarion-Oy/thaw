// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Form, Input, Select, Button, Space, Typography, Tag, Spin } from "antd";
import { PlayCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { GetProcedureParams, BuildCallStatement, IsBoolean, IsNumeric, NeedsQuotes } from "../../../wailsjs/go/app/App";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

interface Param {
  name: string;
  dataType: string;
}

interface ParamTypeInfo {
  isBoolean: boolean;
  isNumeric: boolean;
  needsQuotes: boolean;
}

interface Props {
  db: string;
  schema: string;
  name: string;
  rawArgs: string;
  onClose: () => void;
  // When provided, the modal offers an "Insert" action that hands the built
  // CALL statement back to the caller (e.g. to drop it into a SQL editor)
  // instead of executing it in a new query tab.
  onInsert?: (sql: string) => void;
}

export default function CallProcedureModal({ db, schema, name, rawArgs, onClose, onInsert }: Props) {
  const [params, setParams] = useState<Param[] | null>(null);
  const [paramTypes, setParamTypes] = useState<ParamTypeInfo[]>([]);
  const [values, setValues] = useState<string[]>([]);
  const [preview, setPreview] = useState("");
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    GetProcedureParams(db, schema, name, rawArgs)
      .then(async (result) => {
        const p = result ?? [];
        setParams(p);
        setValues(p.map(() => ""));
        
        // Fetch type info from backend for all params
        const types = await Promise.all(p.map(async (item) => ({
          isBoolean: await IsBoolean(item.dataType),
          isNumeric: await IsNumeric(item.dataType),
          needsQuotes: await NeedsQuotes(item.dataType),
        })));
        setParamTypes(types);
      })
      .catch(() => {
        setParams([]);
        setValues([]);
        setParamTypes([]);
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

  const handleInsert = () => {
    if (!params || !preview || !onInsert) return;
    onInsert(preview);
    onClose();
  };

  // In insert mode Enter inserts; otherwise it executes (preserves prior UX).
  const handleSubmit = onInsert ? handleInsert : handleExecute;

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
          {onInsert ? (
            <Button type="primary" icon={<PlusOutlined />} onClick={handleInsert} disabled={!params}>
              Insert
            </Button>
          ) : (
            <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleExecute} disabled={!params}>
              Execute
            </Button>
          )}
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
              {paramTypes[i]?.isBoolean ? (
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
                  placeholder={paramTypes[i]?.needsQuotes ? "text — quotes added automatically" : "numeric value"}
                  onPressEnter={handleSubmit}
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
