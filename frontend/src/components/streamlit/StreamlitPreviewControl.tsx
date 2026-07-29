// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Snowpark & Developer Workflows

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
  // Set on unmount: a start already in flight must not subscribe or open a
  // browser tab against a component that is gone (see handleStart). It MUST be
  // reset when the effect (re-)runs — StrictMode mounts, unmounts and remounts in
  // development, so a ref left at true from the throwaway first cleanup would
  // cancel every later start and never clear the button's loading state.
  const cancelled = useRef(false);

  const teardown = () => {
    offs.current.forEach((off) => off());
    offs.current = [];
  };

  // Stop the preview if the modal unmounts while it's still running.
  useEffect(() => {
    cancelled.current = false;
    return () => {
      cancelled.current = true;
      teardown();
      StopStreamlitPreview().catch(() => {});
    };
  }, []);

  const handleStart = async () => {
    if (starting) return;
    setStarting(true);
    setReady(false);
    setLastLine("");
    try {
      const res = await StartStreamlitPreview(localDir, mainFile);
      // The modal can close while the start is in flight: its cleanup already ran
      // its StopStreamlitPreview against a backend that had nothing recorded yet,
      // so stop the process we just learned about and subscribe to nothing —
      // otherwise these listeners outlive the component and the ready event pops
      // a browser tab long after the user closed the dialog.
      if (cancelled.current) {
        StopStreamlitPreview().catch(() => {});
        return;
      }
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
        // The preview started but never answered on its port — no ready event is
        // coming, so surface the reason instead of showing "Starting…" forever.
        EventsOn("snowpark:streamlit-error", (msg: string) => {
          setReady(false);
          if (msg) {
            setLastLine(msg);
            message.error(msg);
          }
        }),
        EventsOn("snowpark:streamlit-output", (line: string) => {
          if (line) setLastLine(line);
        }),
      );
    } catch (e) {
      message.error(String(e));
    } finally {
      if (!cancelled.current) setStarting(false);
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
