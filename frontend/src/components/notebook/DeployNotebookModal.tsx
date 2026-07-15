// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useState } from "react";
import {
  Modal, Form, Input, Select, InputNumber, Checkbox, Space, Typography, Divider, message,
} from "antd";
import { CloudUploadOutlined } from "@ant-design/icons";
import { DeployNotebook, ListUserSchemas, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/app/App";
import { useSessionStore } from "../../store/sessionStore";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

interface Props {
  open: boolean;
  /** Absolute local path of the .ipynb file. Empty string for unsaved notebooks. */
  filePath: string;
  /** Serialized nbformat JSON; used when filePath is empty (unsaved notebooks). */
  content: string;
  /** Base filename / tab title shown as the default notebook name. */
  defaultName: string;
  onClose: () => void;
  onDeployed: () => void;
}

export default function DeployNotebookModal({ open, filePath, content, defaultName, onClose, onDeployed }: Props) {
  const [form] = Form.useForm();
  const [deploying, setDeploying] = useState(false);
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const nameValue: string = Form.useWatch("name", form) ?? "";

  const [schemas, setSchemas]       = useState<string[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);

  const activeDb     = useSessionStore((s) => s.database);
  const activeSchema = useSessionStore((s) => s.schema);
  const databases    = useSessionStore((s) => s.databases);
  const warehouses   = useSessionStore((s) => s.warehouses);
  const loadDatabases  = useSessionStore((s) => s.loadDatabases);
  const loadWarehouses = useSessionStore((s) => s.loadWarehouses);

  // Strip .ipynb extension for the default notebook name.
  const baseName = defaultName.replace(/\.ipynb$/i, "");

  // Load databases and warehouses once when the modal opens.
  useEffect(() => {
    if (!open) return;
    loadDatabases();
    loadWarehouses();
    setCaseSensitive(false);
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});

    form.setFieldsValue({
      name: baseName,
      database: activeDb || undefined,
      schema: activeSchema || undefined,
      orReplace: false,
      ifNotExists: false,
      comment: "",
      queryWarehouse: "",
      idleAutoShutdownSeconds: undefined,
      runtimeName: "",
      computePool: "",
      warehouse: "",
    });

    if (activeDb) {
      loadSchemas(activeDb);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  function loadSchemas(db: string) {
    setLoadingSchemas(true);
    setSchemas([]);
    ListUserSchemas(db)
      .then(setSchemas)
      .catch(() => {})
      .finally(() => setLoadingSchemas(false));
  }

  function handleDatabaseChange(db: string) {
    form.setFieldValue("schema", undefined);
    loadSchemas(db);
  }

  async function handleDeploy() {
    let values: Record<string, unknown>;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }

    setDeploying(true);
    try {
      await DeployNotebook({
        database:                values.database as string,
        schema:                  values.schema as string,
        name:                    values.name as string,
        caseSensitive,
        filePath,
        content,
        orReplace:               (values.orReplace as boolean) ?? false,
        ifNotExists:             (values.ifNotExists as boolean) ?? false,
        comment:                 (values.comment as string) ?? "",
        queryWarehouse:          (values.queryWarehouse as string) ?? "",
        idleAutoShutdownSeconds: (values.idleAutoShutdownSeconds as number) ?? 0,
        runtimeName:             (values.runtimeName as string) ?? "",
        computePool:             (values.computePool as string) ?? "",
        warehouse:               (values.warehouse as string) ?? "",
      } as any);
      message.success(`Notebook "${values.name}" deployed to Snowflake`);
      onDeployed();
      onClose();
    } catch (e) {
      message.error(String(e));
    } finally {
      setDeploying(false);
    }
  }

  return (
    <Modal
      open={open}
      title={<Space><CloudUploadOutlined /> Deploy Notebook to Snowflake</Space>}
      okText="Deploy"
      cancelText="Cancel"
      confirmLoading={deploying}
      onOk={handleDeploy}
      onCancel={onClose}
      width={560}
      destroyOnClose
    >
      <Text type="secondary" style={{ fontSize: 12 }}>
        The notebook will be uploaded to a temporary stage and deployed as a Snowflake Notebook object.
      </Text>

      <Form
        form={form}
        layout="vertical"
        style={{ marginTop: 16 }}
        size="small"
      >
        {/* Location */}
        <Form.Item label="Database" name="database" rules={[{ required: true, message: "Required" }]}>
          <Select
            showSearch
            placeholder="Select database"
            options={databases.map((d) => ({ value: d, label: d }))}
            onChange={handleDatabaseChange}
            filterOption={(input, opt) =>
              (opt?.label as string ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>

        <Form.Item label="Schema" name="schema" rules={[{ required: true, message: "Required" }]}>
          <Select
            showSearch
            placeholder="Select schema"
            loading={loadingSchemas}
            options={schemas.map((s) => ({ value: s, label: s }))}
            filterOption={(input, opt) =>
              (opt?.label as string ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>

        <Form.Item label="Notebook name" name="name" rules={[{ required: true, message: "Required" }]} style={{ marginBottom: 4 }}>
          <Input placeholder="MY_NOTEBOOK" />
        </Form.Item>
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={nameValue}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Space>
          <Form.Item name="orReplace" valuePropName="checked" noStyle>
            <Checkbox>OR REPLACE</Checkbox>
          </Form.Item>
          <Form.Item name="ifNotExists" valuePropName="checked" noStyle>
            <Checkbox>IF NOT EXISTS</Checkbox>
          </Form.Item>
        </Space>

        <Divider style={{ margin: "12px 0" }}>
          <Text type="secondary" style={{ fontSize: 11 }}>Optional settings</Text>
        </Divider>

        <Form.Item label="Comment" name="comment">
          <Input placeholder="Optional description" />
        </Form.Item>

        <Form.Item
          label="Query warehouse"
          name="queryWarehouse"
          tooltip="Warehouse used to run SQL queries inside the notebook (QUERY_WAREHOUSE)"
        >
          <Select
            showSearch
            allowClear
            placeholder="Select warehouse (optional)"
            options={warehouses.map((w) => ({ value: w, label: w }))}
            filterOption={(input, opt) =>
              (opt?.label as string ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>

        <Form.Item
          label="Python runtime warehouse"
          name="warehouse"
          tooltip="Warehouse used to run Python cells (WAREHOUSE)"
        >
          <Select
            showSearch
            allowClear
            placeholder="Select warehouse (optional)"
            options={warehouses.map((w) => ({ value: w, label: w }))}
            filterOption={(input, opt) =>
              (opt?.label as string ?? "").toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>

        <Form.Item
          label="Idle auto-shutdown (seconds)"
          name="idleAutoShutdownSeconds"
          tooltip="Automatically shut down the notebook kernel after this many idle seconds (IDLE_AUTO_SHUTDOWN_TIME_SECONDS)"
        >
          <InputNumber min={0} style={{ width: "100%" }} placeholder="e.g. 3600" />
        </Form.Item>

        <Form.Item
          label="Runtime name"
          name="runtimeName"
          tooltip="Snowflake runtime image name (RUNTIME_NAME)"
        >
          <Input placeholder="e.g. SYSTEM$RUNTIME/python3.10" />
        </Form.Item>

        <Form.Item
          label="Compute pool"
          name="computePool"
          tooltip="Compute pool for the notebook (COMPUTE_POOL)"
        >
          <Input placeholder="e.g. MY_COMPUTE_POOL" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
