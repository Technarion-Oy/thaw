// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Space, Typography, Button, Alert, Radio, Tooltip, Input,
} from "antd";
import {
  FileTextOutlined, PlusOutlined, FileSearchOutlined,
  CloudOutlined, FileOutlined, InfoCircleOutlined,
} from "@ant-design/icons";
import {
  ExecDDL, BuildCreateFileFormatSql,
  PickFileForFormatPreview, GetLocalFilePreview, GetStageFilePreview,
} from "../../../wailsjs/go/app/App";
import type { fileformat } from "../../../wailsjs/go/models";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers } from "../shared/createModalHooks";
import FormatPreviewTable from "./FormatPreviewTable";
import FileFormatFields, { BASE_DEFAULTS } from "./FileFormatFields";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// ── Modal ────────────────────────────────────────────────────────────────────

export default function CreateFileFormatModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<fileformat.FileFormatConfig>(BASE_DEFAULTS);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sqlPreview, setSqlPreview] = useState("");
  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();

  // Preview state
  const [previewSource, setPreviewSource] = useState<"LOCAL" | "STAGE">("LOCAL");
  const [localPath, setLocalPath] = useState("");
  const [stagePath, setStagePath] = useState("");
  const [previewData, setPreviewData] = useState<fileformat.PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  // tracks whether the user has triggered at least one preview (enables auto-refresh)
  const hasPreviewRef = useRef(false);

  useEffect(() => {
    BuildCreateFileFormatSql(db, schema, cfg as any).then(setSqlPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = useCallback(<K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) => {
    setCfg((prev) => ({ ...prev, [key]: value }));
  }, []);

  const handlePickFile = async () => {
    const path = await PickFileForFormatPreview();
    if (path) setLocalPath(path);
  };

  // Core preview fetcher — accepts explicit arguments so debounced callers
  // always use the values captured at schedule time, not stale closure values.
  const runPreview = useCallback(async (
    path: string,
    stagePth: string,
    source: "LOCAL" | "STAGE",
    currentCfg: fileformat.FileFormatConfig,
  ) => {
    setPreviewError(null);
    setPreviewLoading(true);
    try {
      let res: fileformat.PreviewResult;
      if (source === "LOCAL") {
        if (!path) throw new Error("Please select a local file first.");
        res = await GetLocalFilePreview(path, currentCfg as any);
      } else {
        if (!stagePth.trim()) throw new Error("Please enter a stage path, e.g. @MY_STAGE/path/file.csv");
        res = await GetStageFilePreview(stagePth.trim(), currentCfg as any);
      }
      if (res.error) {
        setPreviewError(res.error);
        setPreviewData(null);
      } else {
        setPreviewData(res);
      }
    } catch (err) {
      setPreviewError(String(err));
      setPreviewData(null);
    } finally {
      setPreviewLoading(false);
    }
  }, []);

  // Manual Preview button — marks the session as "preview loaded" and runs immediately.
  const handlePreview = () => {
    hasPreviewRef.current = true;
    runPreview(localPath, stagePath, previewSource, cfg);
  };

  // Auto-refresh: re-run preview (debounced 500 ms) whenever format options,
  // source, or file path changes — but only after the user has triggered at
  // least one manual preview in this session.
  useEffect(() => {
    if (!hasPreviewRef.current) return;
    const hasTarget = previewSource === "LOCAL" ? !!localPath : !!stagePath.trim();
    if (!hasTarget) return;
    const timer = setTimeout(() => {
      runPreview(localPath, stagePath, previewSource, cfg);
    }, 500);
    return () => clearTimeout(timer);
  }, [cfg, previewSource, localPath, stagePath, runPreview]);

  const handleCreate = async () => {
    if (!cfg.name.trim()) return;
    setCreating(true);
    setError(null);
    try {
      await ExecDDL(sqlPreview);
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setCreating(false);
    }
  };

  return (
    <CreateModalShell
      icon={<FileTextOutlined />}
      okIcon={<PlusOutlined />}
      title="Create File Format"
      subtitle={`${db}.${schema}`}
      width={1040}
      bodyMaxHeight="85vh"
      error={error}
      errorTitle="Action failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={!!cfg.name.trim()}
      onClose={onClose}
      onSubmit={handleCreate}
    >
      <div style={{ display: "grid", gridTemplateColumns: "380px minmax(0, 1fr)", gap: 24 }}>
        {/* ── Left: Configuration ─────────────────────────────────────── */}
        <div>
          <FileFormatFields cfg={cfg} set={set} />
          <div style={{ marginTop: -10, marginBottom: 10 }}>
            <ObjectNameCaseControl
              name={cfg.name}
              caseSensitive={cfg.caseSensitive}
              onCaseSensitiveChange={(v) => set("caseSensitive", v)}
              quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
            />
          </div>
        </div>

        {/* ── Right: Preview & SQL ────────────────────────────────────── */}
        <div style={{ display: "flex", flexDirection: "column", gap: 16, minWidth: 0 }}>
          {/* Data Preview */}
          <div style={{
            padding: 14,
            background: "color-mix(in srgb, var(--text) 2%, transparent)",
            borderRadius: 8,
            border: "1px solid var(--border)",
          }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
              <Space size={6}>
                <FileSearchOutlined style={{ color: "var(--accent)" }} />
                <span style={{ fontWeight: 600, fontSize: 13 }}>Data Preview</span>
                <Text type="secondary" style={{ fontSize: 11 }}>up to 50 rows</Text>
              </Space>
              <Radio.Group
                value={previewSource}
                onChange={e => { setPreviewSource(e.target.value); setPreviewData(null); hasPreviewRef.current = false; }}
                size="small"
              >
                <Radio.Button value="LOCAL"><FileOutlined /> Local file</Radio.Button>
                <Radio.Button value="STAGE">
                  <Tooltip title="Stage preview uses Snowflake's compute engine and consumes warehouse credits.">
                    <CloudOutlined /> Stage
                  </Tooltip>
                </Radio.Button>
              </Radio.Group>
            </div>

            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              {previewSource === "LOCAL" ? (
                <Input
                  size="small"
                  value={localPath}
                  placeholder="Click to select a local CSV or JSON file…"
                  readOnly
                  style={{ cursor: "pointer", flex: 1 }}
                  onClick={handlePickFile}
                />
              ) : (
                <Input
                  size="small"
                  value={stagePath}
                  placeholder="@DB.SCHEMA.STAGE/path/to/file.csv"
                  onChange={(e) => setStagePath(e.target.value)}
                  style={{ flex: 1 }}
                />
              )}
              <Button type="primary" size="small" loading={previewLoading} onClick={handlePreview}>
                Preview
              </Button>
            </div>

            {previewSource === "STAGE" && (
              <div style={{ marginTop: 6, fontSize: 11, color: "var(--text-muted)" }}>
                <InfoCircleOutlined style={{ marginRight: 4 }} />
                Stage preview requires an active warehouse and consumes compute credits.
              </div>
            )}

            {previewError && (
              <Alert
                type="error"
                message="Preview failed"
                description={previewError}
                showIcon
                closable
                onClose={() => setPreviewError(null)}
                style={{ marginTop: 10 }}
              />
            )}

            <FormatPreviewTable previewData={previewData} />
          </div>

          {/* Generated SQL */}
          <SqlPreview sql={sqlPreview} label="Generated SQL" variant="prominent" style={{ flexGrow: 1 }} />
        </div>
      </div>
    </CreateModalShell>
  );
}
