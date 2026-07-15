// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Form, Input, Select, Button, Space, Typography, Tag, Spin } from "antd";
import { FunctionOutlined } from "@ant-design/icons";
import { GetFunctionInfo, BuildFunctionSelectStatement, IsBoolean, IsNumeric, NeedsQuotes } from "../../../wailsjs/go/app/App";
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
}

export default function SelectFunctionModal({ db, schema, name, rawArgs, onClose }: Props) {
  const [params, setParams] = useState<Param[] | null>(null);
  const [paramTypes, setParamTypes] = useState<ParamTypeInfo[]>([]);
  const [isTableFunction, setIsTableFunction] = useState(false);
  const [values, setValues] = useState<string[]>([]);
  const [preview, setPreview] = useState("");
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    GetFunctionInfo(db, schema, name, rawArgs)
      .then(async (info) => {
        const p = info?.params ?? [];
        setParams(p);
        setIsTableFunction(info?.isTableFunction ?? false);
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
        setIsTableFunction(false);
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
    BuildFunctionSelectStatement(db, schema, name, args, isTableFunction).then(setPreview);
  }, [db, schema, name, params, values, isTableFunction]);

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
          <FunctionOutlined style={{ color: "var(--link)" }} />
          <span>Call function</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" icon={<FunctionOutlined />} onClick={handleExecute} disabled={!params}>
            Run
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
        <Text type="secondary">This function takes no parameters.</Text>
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
