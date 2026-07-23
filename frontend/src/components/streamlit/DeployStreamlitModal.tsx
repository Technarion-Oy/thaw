// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Form, Input, Checkbox, Select, AutoComplete, Button, Typography, message } from "antd";
import { CloudUploadOutlined, FolderOpenOutlined } from "@ant-design/icons";
import {
  DeployStreamlit,
  DetectStreamlitMainFile,
  ListWarehouses,
  PickDirectory,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import { useQuotedIdentifiers, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  /**
   * When set, the modal runs in "update existing app" mode: the name is fixed to
   * this app and OR REPLACE is enforced. Streamlit copies files once at CREATE
   * time (snapshot semantics), so a plain re-upload can't refresh a running app —
   * updating means CREATE OR REPLACE against a fresh temp stage.
   */
  initialName?: string;
  /**
   * When set, the modal opens with this folder already selected and its main file
   * auto-detected — used by the "Deploy now" flow after scaffolding a template.
   */
  initialLocalDir?: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// The last path segment of a local folder path, handling both / and \ so a
// Windows path yields a sensible default object name.
function folderBaseName(dir: string): string {
  const parts = dir.split(/[/\\]/).filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : "";
}

export default function DeployStreamlitModal({ db, schema, initialName, initialLocalDir, onClose, onSuccess }: Props) {
  // "Update existing app" mode: the target app name is fixed and OR REPLACE is
  // enforced (see the initialName prop doc).
  const updateMode = Boolean(initialName);

  const [localDir, setLocalDir] = useState(initialLocalDir ?? "");
  const [mainFile, setMainFile] = useState("");
  const [candidates, setCandidates] = useState<string[]>([]);
  const [detecting, setDetecting] = useState(false);

  const [name, setName] = useState(initialName ?? "");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(updateMode);
  const [queryWarehouse, setQueryWarehouse] = useState("");
  const [title, setTitle] = useState("");
  const [comment, setComment] = useState("");

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const { creating, error, setError, submit } = useCreateSubmit();

  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  // Detect the app's entry point in the given folder. The detected main file
  // (streamlit_app.py / app.py) pre-fills; otherwise the *.py candidates are
  // offered for the user to choose. The object name defaults to the folder name
  // unless the user has already typed one (and never in update mode).
  const detectMainFile = async (dir: string) => {
    if (!updateMode && !name.trim()) setName(folderBaseName(dir));
    setDetecting(true);
    try {
      const res = await DetectStreamlitMainFile(dir);
      setCandidates(res.candidates ?? []);
      setMainFile(res.mainFile ?? "");
      if (!res.mainFile && (res.candidates?.length ?? 0) === 0) {
        message.warning("No Python file found at the folder root — enter the main file path manually.");
      }
    } catch (e) {
      setCandidates([]);
      setMainFile("");
      message.warning(`Could not scan the folder for a main file: ${String(e)}`);
    } finally {
      setDetecting(false);
    }
  };

  // When opened pre-filled (e.g. "Deploy now" after scaffolding a template),
  // detect the entry point of the supplied folder immediately.
  useEffect(() => {
    if (initialLocalDir) detectMainFile(initialLocalDir);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Pick a local folder, then detect its entry point.
  const handleBrowse = async () => {
    let dir = "";
    try {
      dir = await PickDirectory();
    } catch {
      return;
    }
    if (!dir) return;
    setLocalDir(dir);
    await detectMainFile(dir);
  };

  const canSubmit =
    localDir.trim().length > 0 &&
    name.trim().length > 0 &&
    mainFile.trim().length > 0;

  const handleDeploy = () => {
    if (!canSubmit) return;
    submit(async () => {
      await DeployStreamlit({
        database: db,
        schema,
        name: name.trim(),
        caseSensitive,
        localDir,
        mainFile: mainFile.trim(),
        orReplace,
        queryWarehouse: queryWarehouse || "",
        title: title || "",
        comment: comment || "",
      } as any);
      message.success(`Streamlit app "${name.trim()}" ${updateMode ? "redeployed" : "deployed"} to Snowflake`);
      onSuccess?.();
      onClose();
    });
  };

  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const mainFileOptions = (candidates || []).map((c) => ({ value: c }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<CloudUploadOutlined />}
      title={updateMode ? "Redeploy Streamlit" : "Deploy Streamlit"}
      subtitle={updateMode ? `${db}.${schema}.${initialName}` : `${db}.${schema}`}
      width={720}
      okText={updateMode ? "Redeploy" : "Deploy"}
      error={error}
      errorTitle="Streamlit deploy failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      lockWhileBusy
      onClose={onClose}
      onSubmit={handleDeploy}
    >
      <Form layout="vertical" size="small">
        <Text type="secondary" style={{ fontSize: 12 }}>
          The selected folder is uploaded to a temporary stage and deployed as a Snowflake
          Streamlit object. The temporary stage is dropped automatically afterwards.
        </Text>

        <Form.Item
          label="App folder"
          required
          style={{ marginBottom: 8, marginTop: 12 }}
          help="Local folder holding the Streamlit app; its files are uploaded, preserving subdirectories (.git/, __pycache__/, hidden files, and .DS_Store are skipped)."
        >
          <Input
            value={localDir}
            readOnly
            placeholder="No folder selected"
            addonAfter={
              <Button
                type="text"
                size="small"
                icon={<FolderOpenOutlined />}
                onClick={handleBrowse}
                style={{ height: "auto", padding: 0 }}
              >
                Browse…
              </Button>
            }
          />
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Streamlit name" required style={{ marginBottom: 4 }}>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="MY_APP"
              readOnly={updateMode}
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={orReplace}
              disabled={updateMode}
              onChange={(e) => setOrReplace(e.target.checked)}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item
          label="Main file"
          required
          style={itemStyle}
          help="Entry-point Python file, relative to the folder root. Detected automatically (streamlit_app.py / app.py) when present."
        >
          <AutoComplete
            value={mainFile}
            options={mainFileOptions}
            onChange={(v) => setMainFile(v)}
            placeholder="streamlit_app.py"
            disabled={detecting}
            filterOption={(input, opt) =>
              String(opt?.value ?? "").toLowerCase().includes(input.toLowerCase())
            }
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item label="Query warehouse" style={itemStyle} help="Warehouse the app uses to run its SQL queries.">
          <Select
            showSearch
            allowClear
            value={queryWarehouse || undefined}
            onChange={(v) => setQueryWarehouse(v ?? "")}
            options={warehouseOptions}
            placeholder="(optional)"
            loading={loadingWarehouses}
            notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
          />
        </Form.Item>

        <Form.Item label="Title" style={itemStyle} help="Display name shown in Snowsight.">
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="optional display title" />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="optional comment" />
        </Form.Item>
      </Form>
    </CreateModalShell>
  );
}
