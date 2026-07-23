// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useRef, useState } from "react";
import { Button, Space, Typography, Tooltip, message } from "antd";
import { PlayCircleOutlined, StopOutlined, GlobalOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { StartStreamlitPreview, StopStreamlitPreview } from "../../../wailsjs/go/app/App";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";

const { Text } = Typography;

// Snowflake's Streamlit runtime pins specific Python/Streamlit versions and an
// allow-listed Anaconda package set, so a local run is a convenience, not a
// guarantee of parity — this caveat is surfaced next to the control.
const CAVEAT =
  "Local preview runs in your Snowpark Python environment. Snowflake's Streamlit runtime pins specific " +
  "Python/Streamlit versions and an allow-listed Anaconda package set, so “runs locally” ≠ “runs in Snowflake.”";

interface Props {
  localDir: string;
  mainFile: string;
  disabled?: boolean;
}

export default function StreamlitPreviewControl({ localDir, mainFile, disabled }: Props) {
  const [starting, setStarting] = useState(false);
  const [running, setRunning] = useState(false);
  const [url, setUrl] = useState("");
  const [ready, setReady] = useState(false);
  const [lastLine, setLastLine] = useState("");

  // Active event unsubscribers, torn down on stop / unmount.
  const offs = useRef<Array<() => void>>([]);

  const teardown = () => {
    offs.current.forEach((off) => off());
    offs.current = [];
  };

  // Stop the preview if the modal unmounts while it's still running.
  useEffect(() => {
    return () => {
      teardown();
      StopStreamlitPreview().catch(() => {});
    };
  }, []);

  const handleStart = async () => {
    setStarting(true);
    setReady(false);
    setLastLine("");
    try {
      const res = await StartStreamlitPreview(localDir, mainFile);
      setUrl(res.url);
      setRunning(true);
      teardown();
      offs.current.push(
        EventsOn("snowpark:streamlit-ready", (u: string) => {
          setReady(true);
          if (u) BrowserOpenURL(u);
        }),
        EventsOn("snowpark:streamlit-stopped", () => {
          setRunning(false);
          setReady(false);
          teardown();
        }),
        EventsOn("snowpark:streamlit-output", (line: string) => {
          if (line) setLastLine(line);
        }),
      );
    } catch (e) {
      message.error(String(e));
    } finally {
      setStarting(false);
    }
  };

  const handleStop = () => {
    teardown();
    setRunning(false);
    setReady(false);
    StopStreamlitPreview().catch(() => {});
  };

  return (
    <div>
      <Space size={8} wrap>
        {running ? (
          <>
            <Button size="small" danger icon={<StopOutlined />} onClick={handleStop}>
              Stop preview
            </Button>
            <Button
              size="small"
              icon={<GlobalOutlined />}
              disabled={!url}
              onClick={() => url && BrowserOpenURL(url)}
            >
              Open in browser
            </Button>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {ready ? `Running at ${url}` : "Starting…"}
            </Text>
          </>
        ) : (
          <Button
            size="small"
            icon={<PlayCircleOutlined />}
            loading={starting}
            disabled={disabled}
            onClick={handleStart}
          >
            Preview locally
          </Button>
        )}
        <Tooltip title={CAVEAT}>
          <InfoCircleOutlined style={{ color: "var(--text-secondary, #999)" }} />
        </Tooltip>
      </Space>

      {running && lastLine && (
        <div
          style={{
            marginTop: 6,
            fontFamily: "var(--font-mono, monospace)",
            fontSize: 11,
            color: "var(--text-secondary, #999)",
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
          title={lastLine}
        >
          {lastLine}
        </div>
      )}

      <Text type="secondary" style={{ display: "block", marginTop: 6, fontSize: 11 }}>
        {CAVEAT}
      </Text>
    </div>
  );
}
