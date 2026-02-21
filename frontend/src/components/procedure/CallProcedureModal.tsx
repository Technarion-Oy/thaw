import { useState } from "react";
import { Modal, Form, Input, Select, Button, Space, Typography, Tag } from "antd";
import { PlayCircleOutlined } from "@ant-design/icons";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;

interface Param {
  name: string;
  rawType: string;
  baseType: string;
}

// Split an argument list by commas, ignoring commas inside parentheses
// so that NUMBER(38,0) stays together.
function splitArgList(args: string): string[] {
  if (!args.trim()) return [];
  const result: string[] = [];
  let depth = 0;
  let current = "";
  for (const ch of args) {
    if (ch === "(") depth++;
    else if (ch === ")") depth--;
    if (ch === "," && depth === 0) {
      result.push(current.trim());
      current = "";
    } else {
      current += ch;
    }
  }
  if (current.trim()) result.push(current.trim());
  return result;
}

function parseParams(rawArgs: string): Param[] {
  return splitArgList(rawArgs).map((part, idx) => {
    const tokens = part.trim().split(/\s+/);
    let name: string;
    let rawType: string;
    if (tokens.length >= 2) {
      // Snowflake may return "PARAM_NAME TYPE" or just "TYPE"
      // Heuristic: if first token looks like an identifier (no parens), treat as name
      name = tokens[0];
      rawType = tokens.slice(1).join(" ");
    } else {
      name = `param${idx + 1}`;
      rawType = tokens[0] ?? "VARCHAR";
    }
    const baseType = rawType.replace(/\(.*\)/, "").trim().toUpperCase();
    return { name, rawType, baseType };
  });
}

function isBoolean(baseType: string): boolean {
  return baseType === "BOOLEAN" || baseType === "BOOL";
}

function isNumeric(baseType: string): boolean {
  return /^(NUMBER|INT|INTEGER|BIGINT|SMALLINT|TINYINT|BYTEINT|FLOAT|DOUBLE|DECIMAL|NUMERIC|REAL)$/.test(baseType);
}

function needsQuotes(baseType: string): boolean {
  return !isBoolean(baseType) && !isNumeric(baseType);
}

function buildCallSql(db: string, schema: string, name: string, params: Param[], values: string[]): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const args = params
    .map((p, i) => {
      const val = (values[i] ?? "").trim();
      if (val === "") return "NULL";
      if (isBoolean(p.baseType)) return val;
      if (isNumeric(p.baseType)) return val;
      return `'${val.replace(/'/g, "''")}'`;
    })
    .join(", ");
  return `CALL "${esc(db)}"."${esc(schema)}"."${esc(name)}"(${args});`;
}

interface Props {
  db: string;
  schema: string;
  name: string;
  rawArgs: string;
  onClose: () => void;
}

export default function CallProcedureModal({ db, schema, name, rawArgs, onClose }: Props) {
  const params = parseParams(rawArgs);
  const [values, setValues] = useState<string[]>(params.map(() => ""));
  const executeWith = useQueryStore((s) => s.executeWith);

  const setValue = (i: number, v: string) =>
    setValues((prev) => { const next = [...prev]; next[i] = v; return next; });

  const handleExecute = () => {
    const sql = buildCallSql(db, schema, name, params, values);
    onClose();
    executeWith(sql);
  };

  const preview = buildCallSql(db, schema, name, params, values);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <PlayCircleOutlined style={{ color: "#58a6ff" }} />
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
          <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleExecute}>
            Execute
          </Button>
        </Space>
      }
      width={540}
      styles={{ body: { paddingTop: 16 } }}
    >
      {params.length === 0 ? (
        <Text type="secondary">This procedure takes no parameters.</Text>
      ) : (
        <Form layout="vertical">
          {params.map((p, i) => (
            <Form.Item
              key={i}
              label={
                <Space size={6}>
                  <Text strong style={{ fontSize: 13 }}>{p.name}</Text>
                  <Tag style={{ fontSize: 11, margin: 0, fontFamily: "monospace" }}>{p.rawType}</Tag>
                </Space>
              }
              style={{ marginBottom: 12 }}
            >
              {isBoolean(p.baseType) ? (
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
                  placeholder={needsQuotes(p.baseType) ? "text — quotes added automatically" : "numeric value"}
                  onPressEnter={handleExecute}
                />
              )}
            </Form.Item>
          ))}
        </Form>
      )}

      {/* Live preview */}
      <div
        style={{
          marginTop: 8,
          padding: "10px 12px",
          background: "#0d1117",
          borderRadius: 6,
          border: "1px solid #30363d",
        }}
      >
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
          Preview
        </Text>
        <pre
          style={{
            margin: 0,
            color: "#e6edf3",
            fontSize: 12,
            fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}
        >
          {preview}
        </pre>
      </div>
    </Modal>
  );
}
