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
import { Modal, Input, Button, Space, Typography, message, Tooltip, Alert } from "antd";
import { PlayCircleOutlined, PlusOutlined, DeleteOutlined, WarningOutlined } from "@ant-design/icons";
import { ExecuteNotebook, GetNotebookQueryWarehouse } from "../../../wailsjs/go/app/App";
import SetNotebookWarehouseModal from "./SetNotebookWarehouseModal";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

function buildSql(db: string, schema: string, name: string, params: string[]): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const ref = `"${esc(db)}"."${esc(schema)}"."${esc(name)}"`;
  const args = params.map((p) => `'${p.replace(/'/g, "''")}'`).join(", ");
  return `EXECUTE NOTEBOOK ${ref}(${args});`;
}

export default function ExecuteNotebookModal({ db, schema, name, onClose }: Props) {
  const [params, setParams] = useState<string[]>([]);
  const [executing, setExecuting] = useState(false);
  const [queryWarehouse, setQueryWarehouse] = useState<string | null>(null);
  const [warehouseLoading, setWarehouseLoading] = useState(true);
  const [showSetWarehouse, setShowSetWarehouse] = useState(false);

  useEffect(() => {
    GetNotebookQueryWarehouse(db, schema, name)
      .then(setQueryWarehouse)
      .catch(() => setQueryWarehouse(""))
      .finally(() => setWarehouseLoading(false));
  }, [db, schema, name]);

  const addParam = () => setParams((p) => [...p, ""]);
  const removeParam = (i: number) => setParams((p) => p.filter((_, idx) => idx !== i));
  const setParam = (i: number, v: string) =>
    setParams((p) => { const n = [...p]; n[i] = v; return n; });

  const preview = buildSql(db, schema, name, params);
  const warehouseMissing = !warehouseLoading && !queryWarehouse;

  const handleExecute = async () => {
    setExecuting(true);
    try {
      const queryId = await ExecuteNotebook(db, schema, name, params);
      message.success(
        queryId
          ? `Notebook execution started — Query ID: ${queryId}`
          : "Notebook execution started"
      );
      onClose();
    } catch (e) {
      message.error(String(e));
    } finally {
      setExecuting(false);
    }
  };

  return (
    <>
      <Modal
        open
        title={
          <Space size={6}>
            <PlayCircleOutlined style={{ color: "var(--link)" }} />
            <span>Execute notebook</span>
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
              {db}.{schema}.{name}
            </Text>
          </Space>
        }
        onCancel={onClose}
        footer={
          <Space style={{ justifyContent: "flex-end", display: "flex" }}>
            <Button onClick={onClose}>Cancel</Button>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={executing}
              onClick={handleExecute}
            >
              Execute
            </Button>
          </Space>
        }
        width={520}
        styles={{ body: { paddingTop: 16 } }}
      >
        {/* Warehouse status */}
        {!warehouseLoading && (
          warehouseMissing ? (
            <Alert
              type="warning"
              style={{ marginBottom: 16 }}
              icon={<WarningOutlined />}
              showIcon
              message="No query warehouse set"
              description="The notebook does not have a Query Warehouse configured. Execution will likely fail."
              action={
                <Button size="small" onClick={() => setShowSetWarehouse(true)}>
                  Set Warehouse
                </Button>
              }
            />
          ) : (
            <div style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              marginBottom: 16,
              padding: "8px 12px",
              borderRadius: 6,
              border: "1px solid var(--border)",
              background: "var(--bg)",
            }}>
              <Text type="secondary" style={{ fontSize: 12 }}>Query Warehouse</Text>
              <Text style={{ fontSize: 12, fontWeight: 500 }}>{queryWarehouse}</Text>
              <Button
                type="link"
                size="small"
                style={{ marginLeft: "auto", padding: 0, fontSize: 12 }}
                onClick={() => setShowSetWarehouse(true)}
              >
                Change
              </Button>
            </div>
          )
        )}

        {/* Parameter rows */}
        <div style={{ marginBottom: 12 }}>
          <Space style={{ marginBottom: 8 }}>
            <Text style={{ fontSize: 13 }}>Parameters</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>(string values — auto-quoted)</Text>
          </Space>

          {params.length === 0 ? (
            <div style={{
              padding: "10px 12px",
              borderRadius: 6,
              border: "1px dashed var(--border)",
              marginBottom: 8,
            }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                No parameters — notebook will be executed with an empty argument list.
              </Text>
            </div>
          ) : (
            <Space direction="vertical" style={{ width: "100%", marginBottom: 8 }} size={6}>
              {params.map((val, i) => (
                <Space key={i} style={{ width: "100%" }}>
                  <Text type="secondary" style={{ fontSize: 12, minWidth: 20, textAlign: "right" }}>
                    {i + 1}
                  </Text>
                  <Input
                    value={val}
                    onChange={(e) => setParam(i, e.target.value)}
                    placeholder="parameter value"
                    onPressEnter={i === params.length - 1 ? handleExecute : undefined}
                    style={{ flex: 1 }}
                    autoFocus={i === params.length - 1}
                  />
                  <Tooltip title="Remove parameter">
                    <Button
                      type="text"
                      icon={<DeleteOutlined />}
                      size="small"
                      danger
                      onClick={() => removeParam(i)}
                    />
                  </Tooltip>
                </Space>
              ))}
            </Space>
          )}

          <Button icon={<PlusOutlined />} size="small" onClick={addParam}>
            Add parameter
          </Button>
        </div>

        {/* Live SQL preview */}
        <div style={{
          padding: "10px 12px",
          background: "var(--bg)",
          borderRadius: 6,
          border: "1px solid var(--border)",
        }}>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            Preview
          </Text>
          <pre style={{
            margin: 0,
            color: "var(--text)",
            fontSize: 12,
            fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}>
            {preview}
          </pre>
        </div>
      </Modal>

      {showSetWarehouse && (
        <SetNotebookWarehouseModal
          db={db}
          schema={schema}
          name={name}
          onSaved={(wh) => {
            setQueryWarehouse(wh);
            setShowSetWarehouse(false);
          }}
          onClose={() => setShowSetWarehouse(false)}
        />
      )}
    </>
  );
}
