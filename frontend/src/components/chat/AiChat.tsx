// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: AI Tooling

import { useEffect, useRef, useState } from "react";
import { ai } from "../../../wailsjs/go/models";
import { SendChatMessage, GetAIConfig, CancelChat } from "../../../wailsjs/go/main/App";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useQueryStore } from "../../store/queryStore";

function copyText(text: string) {
  // Use Wails' native clipboard API — works regardless of user-select CSS.
  ClipboardSetText(text).catch(() => {
    navigator.clipboard.writeText(text).catch(() => {});
  });
}

function buildResultSummary(result: { columns: string[]; rows: unknown[][] } | null): string {
  if (!result || result.columns.length === 0) return "";
  const header = result.columns.join(" | ");
  const limit = Math.min(result.rows.length, 20);
  const rowLines = result.rows.slice(0, limit).map((row) =>
    row.map((v) => (v === null || v === undefined ? "NULL" : String(v))).join(" | ")
  );
  let out = header + "\n" + rowLines.join("\n");
  if (result.rows.length > 20) {
    out += `\n... (${result.rows.length} rows total)`;
  }
  return out;
}

// Split text into alternating plain/code segments
function parseCodeBlocks(text: string): Array<{ type: "text" | "code"; content: string; lang?: string }> {
  const parts: Array<{ type: "text" | "code"; content: string; lang?: string }> = [];
  const regex = /```(\w*)\n?([\s\S]*?)```/g;
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    if (match.index > last) {
      parts.push({ type: "text", content: text.slice(last, match.index) });
    }
    parts.push({ type: "code", lang: match[1] || undefined, content: match[2] });
    last = match.index + match[0].length;
  }
  if (last < text.length) {
    parts.push({ type: "text", content: text.slice(last) });
  }
  return parts;
}

function CodeBlock({ code, lang }: { code: string; lang?: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    copyText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const handleRun = () => {
    window.dispatchEvent(new CustomEvent("run-ai-sql", { detail: { sql: code, run: true } }));
  };

  const isSql = !lang || lang.toLowerCase() === "sql";

  return (
    <div style={{ position: "relative", marginTop: 6, marginBottom: 6 }}>
      <pre style={{
        background: "var(--bg-overlay)",
        border: "1px solid var(--border)",
        borderRadius: 4,
        padding: "8px 10px",
        fontSize: 12,
        fontFamily: "monospace",
        overflowX: "auto",
        whiteSpace: "pre-wrap",
        wordBreak: "break-all",
        margin: 0,
        color: "var(--text)",
      }}>
        {code}
      </pre>
      <div style={{ display: "flex", gap: 4, marginTop: 4 }}>
        <button
          onClick={handleCopy}
          style={{
            fontSize: 11,
            padding: "2px 8px",
            background: "none",
            border: "1px solid var(--border)",
            borderRadius: 3,
            color: "var(--text-muted)",
            cursor: "pointer",
          }}
        >
          {copied ? "Copied!" : "Copy"}
        </button>
        {isSql && (
          <button
            onClick={handleRun}
            style={{
              fontSize: 11,
              padding: "2px 8px",
              background: "none",
              border: "1px solid var(--border)",
              borderRadius: 3,
              color: "var(--accent)",
              cursor: "pointer",
            }}
          >
            Run
          </button>
        )}
      </div>
    </div>
  );
}

function ToolCallRow({ tc }: { tc: ai.UIToolCall }) {
  const [expanded, setExpanded] = useState(false);

  const outputLines = tc.output.split("\n").filter(Boolean);
  const rowCount = outputLines.length > 1 ? outputLines.length - 1 : 0; // minus header

  const summary = tc.name === "run_sql"
    ? `${tc.name} · ${rowCount} row${rowCount !== 1 ? "s" : ""}`
    : tc.name;

  return (
    <div style={{ marginBottom: 4 }}>
      <button
        onClick={() => setExpanded((e) => !e)}
        style={{
          background: "none",
          border: "none",
          padding: "2px 0",
          cursor: "pointer",
          fontSize: 11,
          color: tc.isError ? "#f56565" : "var(--text-muted)",
          display: "flex",
          alignItems: "center",
          gap: 4,
        }}
      >
        <span style={{ fontSize: 9 }}>{expanded ? "▼" : "▶"}</span>
        {summary}
      </button>
      {expanded && (
        <div style={{
          marginLeft: 14,
          marginTop: 2,
          fontSize: 11,
          color: "var(--text-muted)",
        }}>
          <div style={{ marginBottom: 2, fontFamily: "monospace" }}>{tc.input}</div>
          <pre style={{
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 3,
            padding: "4px 8px",
            fontSize: 11,
            fontFamily: "monospace",
            overflowX: "auto",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            maxHeight: 200,
            overflowY: "auto",
            color: tc.isError ? "#f56565" : "var(--text)",
            margin: 0,
          }}>
            {tc.output || "(no output)"}
          </pre>
        </div>
      )}
    </div>
  );
}

const copyBtnStyle: React.CSSProperties = {
  background: "none",
  border: "1px solid var(--border)",
  borderRadius: 3,
  padding: "1px 6px",
  fontSize: 10,
  color: "var(--text-muted)",
  cursor: "pointer",
  flexShrink: 0,
  lineHeight: "16px",
};

function MessageBubble({ msg }: { msg: ai.UIMessage }) {
  const isUser = msg.role === "user";
  const [copied, setCopied] = useState(false);
  const segments = parseCodeBlocks(msg.text);

  const handleCopy = () => {
    copyText(msg.text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div style={{
      display: "flex",
      flexDirection: "column",
      alignItems: isUser ? "flex-end" : "flex-start",
      marginBottom: 12,
    }}>
      {isUser ? (
        <div style={{ display: "flex", alignItems: "flex-start", gap: 6, maxWidth: "85%" }}>
          <button onClick={handleCopy} style={copyBtnStyle} title="Copy message">
            {copied ? "✓" : "Copy"}
          </button>
          <div style={{
            background: "var(--accent)",
            color: "#fff",
            borderRadius: "12px 12px 2px 12px",
            padding: "6px 12px",
            fontSize: 13,
            wordBreak: "break-word",
          }}>
            {msg.text}
          </div>
        </div>
      ) : (
        <div style={{ maxWidth: "100%", width: "100%" }}>
          {msg.toolCalls && msg.toolCalls.length > 0 && (
            <div style={{ marginBottom: 6 }}>
              {msg.toolCalls.map((tc, i) => (
                <ToolCallRow key={i} tc={tc} />
              ))}
            </div>
          )}
          <div style={{ fontSize: 13, color: "var(--text)", lineHeight: 1.5 }}>
            {segments.map((seg, i) =>
              seg.type === "code" ? (
                <CodeBlock key={i} code={seg.content} lang={seg.lang} />
              ) : (
                <span key={i} style={{ whiteSpace: "pre-wrap" }}>{seg.content}</span>
              )
            )}
          </div>
          {msg.text && (
            <div style={{ marginTop: 4 }}>
              <button onClick={handleCopy} style={copyBtnStyle} title="Copy message">
                {copied ? "✓ Copied" : "Copy"}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function AiChat() {
  const [messages, setMessages] = useState<ai.UIMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [agentMode, setAgentMode] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const sql = useQueryStore((s) => s.sql);
  const result = useQueryStore((s) => s.result);

  useEffect(() => {
    GetAIConfig().then((c) => setAiEnabled(c.enabled));
  }, []);

  // ⌘L / Ctrl+L — focus the AI chat input from a keyboard shortcut.
  useEffect(() => {
    const handler = () => textareaRef.current?.focus();
    window.addEventListener("thaw:focus-ai-chat", handler);
    return () => window.removeEventListener("thaw:focus-ai-chat", handler);
  }, []);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, loading]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || loading) return;

    setInput("");
    setError(null);
    setLoading(true);

    const summary = buildResultSummary(result);

    try {
      const newMsgs = await SendChatMessage(messages, text, sql, summary, agentMode);
      setMessages((prev) => [...prev, ...newMsgs]);
    } catch (e) {
      const msg = String(e);
      if (!msg.includes("context canceled") && !msg.includes("context cancelled")) {
        setError(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleStop = () => {
    CancelChat().catch(() => {});
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  if (aiEnabled === null) {
    return (
      <div style={{ padding: 24, color: "var(--text-muted)", fontSize: 13 }}>
        Loading…
      </div>
    );
  }

  if (!aiEnabled) {
    return (
      <div style={{ padding: 24, color: "var(--text-muted)", fontSize: 13 }}>
        Enable AI in the AI menu to start chatting.
      </div>
    );
  }

  return (
    <div className="ai-chat-selectable" style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%", overflow: "hidden" }}>
      {/* Messages */}
      <div
        ref={scrollRef}
        style={{
          flex: 1,
          overflowY: "auto",
          padding: 12,
        }}
      >
        {messages.length === 0 && (
          <div style={{ color: "var(--text-muted)", fontSize: 13, textAlign: "center", marginTop: 24 }}>
            Ask a question about your data.
          </div>
        )}
        {messages.map((msg, i) => (
          <MessageBubble key={i} msg={msg} />
        ))}
        {loading && (
          <div style={{ display: "flex", alignItems: "center", gap: 8, paddingBottom: 8 }}>
            <span style={{ color: "var(--text-muted)", fontSize: 12 }}>Thinking…</span>
            <button
              onClick={handleStop}
              style={{
                fontSize: 11,
                padding: "2px 8px",
                background: "none",
                border: "1px solid var(--border)",
                borderRadius: 3,
                color: "var(--text-muted)",
                cursor: "pointer",
              }}
            >
              Stop
            </button>
          </div>
        )}
        {error && (
          <div style={{
            fontSize: 12,
            padding: "6px 10px",
            background: "rgba(245,101,101,0.1)",
            borderRadius: 4,
            marginBottom: 8,
            display: "flex",
            alignItems: "flex-start",
            gap: 8,
          }}>
            <span style={{ color: "#f56565", flex: 1 }}>{error}</span>
            <button
              onClick={() => copyText(error)}
              style={copyBtnStyle}
              title="Copy error"
            >
              Copy
            </button>
          </div>
        )}
      </div>

      {/* Input */}
      <div style={{
        padding: "8px 12px",
        borderTop: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        gap: 6,
        flexShrink: 0,
      }}>
        {/* Mode toggle */}
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <button
            onClick={() => setAgentMode((v) => !v)}
            title={agentMode ? "Agent mode: AI can explore your database and run queries" : "Chat mode: conversational only, no database access"}
            style={{
              fontSize: 11,
              padding: "2px 8px",
              borderRadius: 10,
              border: `1px solid ${agentMode ? "var(--accent)" : "var(--border)"}`,
              background: agentMode ? "color-mix(in srgb, var(--accent) 15%, transparent)" : "none",
              color: agentMode ? "var(--accent)" : "var(--text-muted)",
              cursor: "pointer",
              fontWeight: agentMode ? 600 : 400,
              transition: "all 0.15s",
            }}
          >
            {agentMode ? "⚡ Agent" : "Agent"}
          </button>
          <span style={{ fontSize: 11, color: "var(--text-faint)" }}>
            {agentMode ? "Can explore schema and run queries" : "Chat only · enable Agent for database access"}
          </span>
        </div>

        <div style={{ display: "flex", gap: 8, alignItems: "flex-end" }}>
          <textarea
            ref={textareaRef}
            rows={2}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question… (Enter to send, Shift+Enter for newline)"
            disabled={loading}
            style={{
              flex: 1,
              resize: "none",
              fontSize: 13,
              padding: "6px 8px",
              background: "var(--bg-overlay)",
              border: "1px solid var(--border)",
              borderRadius: 4,
              color: "var(--text)",
              fontFamily: "inherit",
              outline: "none",
            }}
          />
          <button
            onClick={handleSend}
            disabled={loading || !input.trim()}
            style={{
              padding: "6px 14px",
              fontSize: 13,
              background: "var(--accent)",
              color: "#fff",
              border: "none",
              borderRadius: 4,
              cursor: loading || !input.trim() ? "not-allowed" : "pointer",
              opacity: loading || !input.trim() ? 0.5 : 1,
              flexShrink: 0,
              alignSelf: "flex-end",
            }}
          >
            Send ↵
          </button>
        </div>
      </div>
    </div>
  );
}
