// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Select, Button, Space } from "antd";
import { CloseOutlined, ReloadOutlined, PoweroffOutlined } from "@ant-design/icons";
import { GetAvailableShells, StartShell, WriteShell, ResizeShell, StopShell } from "../../../wailsjs/go/app/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { useThemeStore } from "../../store/themeStore";
import { useGitStore } from "../../store/gitStore";

interface Props {
  onClose: () => void;
}

export default function TerminalPanel({ onClose }: Props) {
  const resolved       = useThemeStore((s) => s.resolved);
  const editorFont     = useThemeStore((s) => s.editorFont);
  const editorFontSize = useThemeStore((s) => s.editorFontSize);
  const exportDir      = useGitStore((s) => s.exportDir);

  const containerRef = useRef<HTMLDivElement>(null);
  const termRef      = useRef<Terminal | null>(null);
  const fitRef       = useRef<FitAddon | null>(null);

  const [shells, setShells]           = useState<string[]>([]);
  const [activeShell, setActiveShell] = useState<string>("");

  // Build xterm theme from resolved theme.
  const xtermTheme = resolved === "dark"
    ? {
        background:  "#0d1117",
        foreground:  "#e6edf3",
        cursor:      "#40c8fc",
        cursorAccent:"#0d1117",
        black:       "#21262d",
        red:         "#ff7b72",
        green:       "#3fb950",
        yellow:      "#d29922",
        blue:        "#58a6ff",
        magenta:     "#bc8cff",
        cyan:        "#39c5cf",
        white:       "#b1bac4",
        brightBlack: "#6e7681",
        brightRed:   "#ffa198",
        brightGreen: "#56d364",
        brightYellow:"#e3b341",
        brightBlue:  "#79c0ff",
        brightMagenta:"#d2a8ff",
        brightCyan:  "#56d4dd",
        brightWhite: "#f0f6fc",
      }
    : {
        background:  "#ffffff",
        foreground:  "#24292f",
        cursor:      "#0969da",
        cursorAccent:"#ffffff",
        black:       "#24292f",
        red:         "#cf222e",
        green:       "#116329",
        yellow:      "#4d2d00",
        blue:        "#0550ae",
        magenta:     "#8250df",
        cyan:        "#0e7a6e",
        white:       "#6e7781",
        brightBlack: "#57606a",
        brightRed:   "#a40e26",
        brightGreen: "#1a7f37",
        brightYellow:"#633c01",
        brightBlue:  "#0969da",
        brightMagenta:"#6639ba",
        brightCyan:  "#0e7a6e",
        brightWhite: "#8c959f",
      };

  // Mount: fetch shells, create and open xterm, start shell.
  useEffect(() => {
    let disposed = false;
    let offData: (() => void) | null = null;
    let offExit: (() => void) | null = null;

    const init = async () => {
      const availableShells = await GetAvailableShells();
      if (disposed) return;
      setShells(availableShells);

      const shell = availableShells[0] ?? "/bin/zsh";
      setActiveShell(shell);

      const term = new Terminal({
        cursorBlink:   true,
        fontSize:      editorFontSize,
        fontFamily:    editorFont,
        theme:         xtermTheme,
        allowTransparency: false,
      });
      termRef.current = term;

      const fitAddon = new FitAddon();
      fitRef.current = fitAddon;
      term.loadAddon(fitAddon);

      if (containerRef.current) {
        term.open(containerRef.current);
        fitAddon.fit();
      }

      await StartShell(shell, exportDir);

      term.onData((data) => {
        WriteShell(data);
      });

      offData = EventsOn("terminal:data", (b64: string) => {
        const binary = atob(b64);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        term.write(bytes);
      });

      offExit = EventsOn("terminal:exit", () => {
        term.writeln("\r\n[Process exited]");
      });
    };

    init();

    return () => {
      disposed = true;
      offData?.();
      offExit?.();
      termRef.current?.dispose();
      termRef.current = null;
      StopShell();
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Update xterm theme when resolved theme changes (without full remount).
  useEffect(() => {
    termRef.current?.options.theme && (termRef.current.options = { ...termRef.current.options, theme: xtermTheme });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resolved]);

  // ResizeObserver: refit terminal when container dimensions change.
  useEffect(() => {
    if (!containerRef.current) return;
    const observer = new ResizeObserver(() => {
      if (!fitRef.current || !termRef.current) return;
      try {
        fitRef.current.fit();
        ResizeShell(termRef.current.cols, termRef.current.rows);
      } catch (_) {
        // ignore errors during resize
      }
    });
    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, []);

  const handleNew = async () => {
    await StopShell();
    termRef.current?.clear();
    await StartShell(activeShell, exportDir);
  };

  const handleKill = async () => {
    await StopShell();
  };

  const handleClose = async () => {
    await StopShell();
    onClose();
  };

  const handleShellChange = async (newShell: string) => {
    setActiveShell(newShell);
    await StopShell();
    termRef.current?.clear();
    await StartShell(newShell, exportDir);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", background: xtermTheme.background }}>
      {/* Header bar */}
      <div
        style={{
          height: 30,
          flexShrink: 0,
          display: "flex",
          alignItems: "center",
          gap: 6,
          padding: "0 8px",
          background: resolved === "dark" ? "#161b22" : "#f6f8fa",
          borderBottom: `1px solid ${resolved === "dark" ? "#30363d" : "#d0d7de"}`,
        }}
      >
        <Select
          size="small"
          value={activeShell || undefined}
          options={shells.map((s) => ({ value: s, label: s }))}
          onChange={handleShellChange}
          style={{ minWidth: 120, fontSize: 12 }}
          dropdownStyle={{ minWidth: 200 }}
        />
        <Space size={4}>
          <Button size="small" icon={<ReloadOutlined />} onClick={handleNew} title="New terminal">
            New
          </Button>
          <Button size="small" icon={<PoweroffOutlined />} onClick={handleKill} title="Kill process">
            Kill
          </Button>
        </Space>
        <div style={{ flex: 1 }} />
        <Button
          size="small"
          type="text"
          icon={<CloseOutlined />}
          onClick={handleClose}
          title="Close terminal"
          style={{ color: resolved === "dark" ? "#8b949e" : "#57606a" }}
        />
      </div>

      {/* xterm.js canvas */}
      <div
        ref={containerRef}
        style={{ flex: 1, overflow: "hidden", padding: 4 }}
      />
    </div>
  );
}
