import { useEffect } from "react";
import { Button, Space, Typography, Alert, Spin, Tag } from "antd";
import { PlayCircleOutlined, DisconnectOutlined } from "@ant-design/icons";
import { ExecuteQuery, Disconnect } from "../../wailsjs/go/main/App";
import SqlEditor from "../components/editor/SqlEditor";
import ResultGrid from "../components/results/ResultGrid";
import { useQueryStore } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, result, isRunning, error, setResult, setRunning, setError } = useQueryStore();
  const { params, disconnect } = useConnectionStore();

  const runQuery = async () => {
    setRunning(true);
    try {
      const res = await ExecuteQuery(sql);
      setResult(res);
    } catch (e) {
      setError(String(e));
    } finally {
      setRunning(false);
    }
  };

  const handleDisconnect = async () => {
    await Disconnect();
    disconnect();
  };

  // Listen for Cmd+Enter from the editor
  useEffect(() => {
    const handler = () => runQuery();
    window.addEventListener("run-query", handler);
    return () => window.removeEventListener("run-query", handler);
  });

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", background: "#0d1117" }}>
      {/* Toolbar */}
      <div
        style={{
          padding: "6px 12px",
          borderBottom: "1px solid #30363d",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          background: "#161b22",
        }}
      >
        <Space>
          <Button
            type="primary"
            icon={<PlayCircleOutlined />}
            loading={isRunning}
            onClick={runQuery}
            size="small"
          >
            Run
          </Button>
          <Text type="secondary" style={{ fontSize: 11 }}>
            ⌘↵ to run
          </Text>
        </Space>

        <Space>
          {params && (
            <Tag color="blue" style={{ fontSize: 11 }}>
              {params.account} · {params.user}
            </Tag>
          )}
          <Button
            icon={<DisconnectOutlined />}
            size="small"
            danger
            onClick={handleDisconnect}
          >
            Disconnect
          </Button>
        </Space>
      </div>

      {/* SQL Editor — top half */}
      <div style={{ flex: "0 0 40%", borderBottom: "1px solid #30363d" }}>
        <SqlEditor />
      </div>

      {/* Results — bottom half */}
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {isRunning && (
          <div style={{ position: "absolute", inset: 0, display: "flex", alignItems: "center", justifyContent: "center", zIndex: 10, background: "rgba(0,0,0,0.4)" }}>
            <Spin size="large" />
          </div>
        )}

        {error && (
          <Alert
            type="error"
            message={error}
            showIcon
            closable
            style={{ margin: 12 }}
          />
        )}

        {result && !error && <ResultGrid result={result} />}

        {!result && !error && !isRunning && (
          <div style={{ padding: 24, color: "#484f58", fontSize: 13 }}>
            Run a query to see results here.
          </div>
        )}
      </div>
    </div>
  );
}
