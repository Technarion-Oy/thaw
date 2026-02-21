import { useState } from "react";
import { Button, Progress, Typography, Space, Tag, Collapse, Alert } from "antd";
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  CheckCircleOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { ExportAllDatabasesDDL } from "../../../wailsjs/go/main/App";
import { useGitStore } from "../../store/gitStore";
import type { ddl } from "../../../wailsjs/go/models";

type ExportResult = ddl.ExportResult;

interface ProgressEvent {
  done: number;
  total: number;
  result: ExportResult;
}

const { Text } = Typography;

export default function ExportPanel() {
  const exportDir = useGitStore((s) => s.exportDir);

  const [running, setRunning]     = useState(false);
  const [progress, setProgress]   = useState({ done: 0, total: 0 });
  const [results, setResults]     = useState<ExportResult[]>([]);
  const [error, setError]         = useState<string | null>(null);
  const [finished, setFinished]   = useState(false);

  const exportAll = async () => {
    if (!exportDir || running) return;
    setRunning(true);
    setFinished(false);
    setResults([]);
    setError(null);
    setProgress({ done: 0, total: 0 });

    const off = EventsOn("ddl:progress", (payload: ProgressEvent) => {
      setProgress({ done: payload.done, total: payload.total });
      setResults((prev) => [...prev, payload.result]);
    });

    try {
      await ExportAllDatabasesDDL(exportDir);
    } catch (e) {
      setError(String(e));
    } finally {
      off();
      setRunning(false);
      setFinished(true);
      window.dispatchEvent(new CustomEvent("thaw:export-complete"));
    }
  };

  const pct = progress.total > 0
    ? Math.round((progress.done / progress.total) * 100)
    : 0;

  const totalFiles  = results.reduce((s, r) => s + r.files, 0);
  const totalSkipped = results.reduce((s, r) => s + r.skipped, 0);
  const hasErrors   = results.some((r) => (r.errors?.length ?? 0) > 0);

  return (
    <div style={{ padding: "10px 12px", fontSize: 12 }}>
      <Text
        type="secondary"
        style={{
          fontSize: 11,
          textTransform: "uppercase",
          letterSpacing: "0.08em",
          display: "block",
          marginBottom: 8,
        }}
      >
        Export DDL
      </Text>

      {!exportDir && (
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          Set a working directory in the Git section below.
        </Text>
      )}

      {exportDir && (
        <div style={{ marginBottom: 8, fontSize: 11, color: "#8b949e", wordBreak: "break-all" }}>
          {exportDir}
        </div>
      )}

      <Button
        size="small"
        type="primary"
        icon={<CloudUploadOutlined />}
        disabled={!exportDir || running}
        loading={running}
        onClick={exportAll}
        style={{ width: "100%", marginBottom: 8 }}
      >
        {running ? `Exporting… (${progress.done}/${progress.total})` : "Export All Databases"}
      </Button>

      {/* Progress bar */}
      {running && progress.total > 0 && (
        <Progress
          percent={pct}
          size="small"
          style={{ marginBottom: 8 }}
          format={() => `${progress.done} / ${progress.total}`}
        />
      )}

      {/* Error */}
      {error && (
        <Alert
          type="error"
          message={error}
          showIcon
          style={{ fontSize: 11, marginBottom: 8 }}
        />
      )}

      {/* Summary */}
      {finished && results.length > 0 && (
        <div>
          <Space size={4} style={{ marginBottom: 6, flexWrap: "wrap" }}>
            <Tag
              icon={<CheckCircleOutlined />}
              color="green"
              style={{ fontSize: 10, margin: 0 }}
            >
              {totalFiles} files
            </Tag>
            {totalSkipped > 0 && (
              <Tag style={{ fontSize: 10, margin: 0 }}>{totalSkipped} skipped</Tag>
            )}
            {hasErrors && (
              <Tag
                icon={<WarningOutlined />}
                color="red"
                style={{ fontSize: 10, margin: 0 }}
              >
                errors
              </Tag>
            )}
          </Space>

          <Collapse
            size="small"
            ghost
            style={{ fontSize: 11 }}
            items={results.map((r) => ({
              key: r.database,
              label: (
                <Space size={4}>
                  <DatabaseOutlined style={{ fontSize: 11 }} />
                  <span style={{ fontSize: 11 }}>{r.database}</span>
                  <Tag style={{ fontSize: 10, margin: 0 }}>{r.files} files</Tag>
                  {(r.errors?.length ?? 0) > 0 && (
                    <Tag color="red" style={{ fontSize: 10, margin: 0 }}>
                      {r.errors!.length} err
                    </Tag>
                  )}
                </Space>
              ),
              children:
                (r.errors?.length ?? 0) > 0 ? (
                  <div style={{ fontSize: 11, color: "#f85149" }}>
                    {r.errors!.map((e, i) => (
                      <div key={i}>{e}</div>
                    ))}
                  </div>
                ) : (
                  <div style={{ fontSize: 11, color: "#3fb950" }}>
                    {r.files} files written, {r.skipped} skipped
                  </div>
                ),
            }))}
          />
        </div>
      )}
    </div>
  );
}
