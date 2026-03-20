// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to patents holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo, useRef } from "react";
import { Modal, Input, Button, Tag, Spin, Segmented, Empty, Typography } from "antd";
import { SearchOutlined, FunctionOutlined, EnterOutlined, SendOutlined, StopOutlined } from "@ant-design/icons";
import { GetAllFunctionNames, GetFunctionTooltip, SendChatMessage, GetAIConfig, CancelChat } from "../../../wailsjs/go/main/App";
import { ClipboardSetText, BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { insertAtCursor } from "../editor/editorRef";
import type { fnmeta, ai } from "../../../wailsjs/go/models";

const { Text } = Typography;

type FnMeta = fnmeta.FunctionMeta;

// ── AI chat helpers ───────────────────────────────────────────────────────────

function parseCodeBlocks(text: string): Array<{ type: "text" | "code"; content: string }> {
  const parts: Array<{ type: "text" | "code"; content: string }> = [];
  const regex = /```(?:\w*)\n?([\s\S]*?)```/g;
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    if (match.index > last) parts.push({ type: "text", content: text.slice(last, match.index) });
    parts.push({ type: "code", content: match[1] });
    last = match.index + match[0].length;
  }
  if (last < text.length) parts.push({ type: "text", content: text.slice(last) });
  return parts;
}

function FnChatMessage({ msg }: { msg: ai.UIMessage }) {
  const isUser = msg.role === "user";
  const [copied, setCopied] = useState(false);
  const segments = parseCodeBlocks(msg.text);
  const handleCopy = () => {
    ClipboardSetText(msg.text).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: isUser ? "flex-end" : "flex-start", marginBottom: 10 }}>
      {isUser ? (
        <div style={{ background: "var(--ant-color-primary, #177ddc)", color: "#fff", borderRadius: "10px 10px 2px 10px", padding: "6px 10px", fontSize: 12, maxWidth: "85%", wordBreak: "break-word" }}>
          {msg.text}
        </div>
      ) : (
        <div style={{ maxWidth: "100%", width: "100%", fontSize: 12, lineHeight: 1.6, color: "var(--text, inherit)" }}>
          {segments.map((seg, i) =>
            seg.type === "code" ? (
              <pre key={i} style={{ background: "var(--code-bg, #1e1e1e)", border: "1px solid var(--border-color, #303030)", borderRadius: 4, padding: "6px 8px", fontSize: 11, fontFamily: "monospace", overflowX: "auto", whiteSpace: "pre-wrap", wordBreak: "break-word", margin: "4px 0" }}>
                {seg.content}
              </pre>
            ) : (
              <span key={i} style={{ whiteSpace: "pre-wrap" }}>{seg.content}</span>
            )
          )}
          {msg.text && (
            <button onClick={handleCopy} style={{ marginTop: 2, background: "none", border: "1px solid var(--border-color, #303030)", borderRadius: 3, padding: "1px 6px", fontSize: 10, color: "var(--text-muted, #888)", cursor: "pointer" }}>
              {copied ? "✓ Copied" : "Copy"}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

interface FnChatProps {
  fn: FnMeta;
  overloads: FnMeta[];
}

function FnChat({ fn, overloads }: FnChatProps) {
  const [messages, setMessages] = useState<ai.UIMessage[]>([]);
  const [input, setInput]       = useState("");
  const [loading, setLoading]   = useState(false);
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null);
  const [error, setError]       = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Reset history when the selected function changes.
  useEffect(() => {
    setMessages([]);
    setInput("");
    setError(null);
  }, [fn.functionName]);

  useEffect(() => {
    GetAIConfig().then((c) => setAiEnabled(c.enabled));
  }, []);

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages, loading]);

  // Build function context to inject as currentSQL so the AI knows what function we're asking about.
  const fnContext = useMemo(() => {
    if (overloads.length === 0) return `Function: ${fn.functionName}`;
    return overloads
      .map((ol) => [ol.functionSignature, ol.description].filter(Boolean).join("\n"))
      .join("\n\n");
  }, [fn.functionName, overloads]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || loading) return;
    setInput("");
    setError(null);
    setLoading(true);
    try {
      const newMsgs = await SendChatMessage(messages, text, fnContext, "", false);
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

  const handleStop = () => { CancelChat().catch(() => {}); };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  if (aiEnabled === false) {
    return (
      <div style={{ display: "flex", flex: 1, alignItems: "center", justifyContent: "center", flexDirection: "column", gap: 8, padding: 24, color: "var(--text-muted, #888)", fontSize: 12, textAlign: "center" }}>
        <span>AI is not enabled.</span>
        <span>Configure it via <strong>AI → Configure AI…</strong> in the menu bar.</span>
      </div>
    );
  }

  const docsUrl = fn.functionType === "BUILTIN"
    ? `https://docs.snowflake.com/en/sql-reference/functions/${fn.functionName.toLowerCase()}`
    : null;

  return (
    <div style={{ display: "flex", flexDirection: "column", flex: 1, overflow: "hidden" }}>
      {/* Docs link (built-ins only) */}
      {docsUrl && (
        <div style={{ padding: "6px 14px", borderBottom: "1px solid var(--border-color, #303030)", flexShrink: 0 }}>
          <button
            onClick={() => BrowserOpenURL(docsUrl)}
            style={{
              background: "none",
              border: "none",
              padding: 0,
              cursor: "pointer",
              fontSize: 12,
              color: "var(--ant-color-primary, #177ddc)",
              textDecoration: "underline",
              textUnderlineOffset: 2,
            }}
          >
            📖 Snowflake documentation for {fn.functionName}
          </button>
        </div>
      )}

      {/* Message list */}
      <div ref={scrollRef} style={{ flex: 1, overflowY: "auto", padding: "12px 14px" }}>
        {messages.length === 0 && !loading && (
          <div style={{ color: "var(--text-muted, #888)", fontSize: 12, textAlign: "center", paddingTop: 24 }}>
            Ask anything about <strong style={{ fontFamily: "monospace" }}>{fn.functionName}</strong>
          </div>
        )}
        {messages.map((msg, i) => <FnChatMessage key={i} msg={msg} />)}
        {loading && (
          <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, color: "var(--text-muted, #888)" }}>
            <Spin size="small" /> Thinking…
          </div>
        )}
        {error && (
          <div style={{ fontSize: 12, color: "#f56565", marginTop: 4 }}>{error}</div>
        )}
      </div>

      {/* Input */}
      <div style={{ borderTop: "1px solid var(--border-color, #303030)", padding: "8px 10px", display: "flex", gap: 6, alignItems: "flex-end" }}>
        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={`Ask about ${fn.functionName}…`}
          disabled={loading}
          rows={2}
          style={{
            flex: 1,
            resize: "none",
            fontSize: 12,
            padding: "6px 8px",
            borderRadius: 4,
            border: "1px solid var(--border-color, #303030)",
            background: "var(--input-bg, transparent)",
            color: "inherit",
            fontFamily: "inherit",
            outline: "none",
          }}
        />
        {loading ? (
          <Button size="small" icon={<StopOutlined />} onClick={handleStop} danger>Stop</Button>
        ) : (
          <Button size="small" type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={!input.trim()}>Send</Button>
        )}
      </div>
    </div>
  );
}

interface Props {
  onClose: () => void;
}

export default function FunctionCatalogModal({ onClose }: Props) {
  const [allFns, setAllFns]           = useState<FnMeta[]>([]);
  const [loading, setLoading]         = useState(true);
  const [search, setSearch]           = useState("");
  const [typeFilter, setTypeFilter]   = useState<"ALL" | "BUILTIN" | "UDF">("ALL");
  const [selected, setSelected]       = useState<FnMeta | null>(null);
  const [overloads, setOverloads]     = useState<FnMeta[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [activeTab, setActiveTab]     = useState<"details" | "ask-ai">("details");

  // Load full name list once on mount
  useEffect(() => {
    GetAllFunctionNames()
      .then((fns) => {
        const sorted = [...fns].sort((a, b) => a.functionName.localeCompare(b.functionName));
        setAllFns(sorted);
        if (sorted.length > 0) selectFn(sorted[0]);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function selectFn(fn: FnMeta) {
    setSelected(fn);
    setOverloads([]);
    setDetailLoading(true);
    GetFunctionTooltip(fn.functionName)
      .then((ol) => setOverloads(ol ?? []))
      .catch(() => setOverloads([]))
      .finally(() => setDetailLoading(false));
  }

  const filtered = useMemo(() => {
    const q = search.trim().toUpperCase();
    return allFns.filter((fn) => {
      if (typeFilter === "BUILTIN" && fn.functionType !== "BUILTIN") return false;
      if (typeFilter === "UDF"     && fn.functionType !== "UDF")     return false;
      return q === "" || fn.functionName.includes(q);
    });
  }, [allFns, search, typeFilter]);

  function handleInsert() {
    if (!selected) return;
    insertAtCursor(selected.functionName + "(");
    onClose();
  }

  return (
    <Modal
      open
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <FunctionOutlined />
          Function Catalog
        </span>
      }
      onCancel={onClose}
      width={860}
      styles={{ body: { padding: 0 } }}
      footer={null}
    >
      <div style={{ display: "flex", height: 540 }}>

        {/* ── Left: list ─────────────────────────────────────────────── */}
        <div style={{
          width: 260,
          borderRight: "1px solid var(--border-color, #303030)",
          display: "flex",
          flexDirection: "column",
          flexShrink: 0,
        }}>
          {/* Search + filter */}
          <div style={{ padding: "10px 10px 6px" }}>
            <Input
              prefix={<SearchOutlined style={{ color: "var(--text-muted, #888)" }} />}
              placeholder="Search functions…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              allowClear
              autoFocus
              size="small"
            />
          </div>
          <div style={{ padding: "0 10px 8px" }}>
            <Segmented
              size="small"
              block
              value={typeFilter}
              onChange={(v) => setTypeFilter(v as typeof typeFilter)}
              options={[
                { label: "All", value: "ALL" },
                { label: "Built-in", value: "BUILTIN" },
                { label: "UDF", value: "UDF" },
              ]}
            />
          </div>

          {/* Function list */}
          <div style={{ flex: 1, overflowY: "auto" }}>
            {loading ? (
              <div style={{ padding: 20, textAlign: "center" }}>
                <Spin size="small" />
              </div>
            ) : filtered.length === 0 ? (
              <div style={{ padding: 16, fontSize: 12, color: "var(--text-muted, #888)", textAlign: "center" }}>
                No functions found
              </div>
            ) : (
              filtered.map((fn) => {
                const isSelected = selected?.functionName === fn.functionName;
                return (
                  <div
                    key={fn.functionName}
                    onClick={() => selectFn(fn)}
                    style={{
                      padding: "5px 10px",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: 6,
                      backgroundColor: isSelected ? "var(--item-active-bg, #177ddc22)" : "transparent",
                      borderLeft: isSelected ? "2px solid var(--ant-color-primary, #177ddc)" : "2px solid transparent",
                    }}
                  >
                    <span style={{
                      fontSize: 12,
                      fontFamily: "monospace",
                      fontWeight: isSelected ? 600 : 400,
                      color: fn.functionType === "UDF" ? "var(--udf-color, #4ec9b0)" : undefined,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}>
                      {fn.functionName}
                    </span>
                    {fn.functionType === "UDF" && (
                      <Tag color="teal" style={{ fontSize: 10, lineHeight: "16px", padding: "0 4px", flexShrink: 0 }}>
                        UDF
                      </Tag>
                    )}
                  </div>
                );
              })
            )}
          </div>

          {/* Count */}
          <div style={{ padding: "6px 10px", fontSize: 11, color: "var(--text-muted, #888)", borderTop: "1px solid var(--border-color, #303030)" }}>
            {filtered.length} / {allFns.length} functions
          </div>
        </div>

        {/* ── Right: detail + AI chat ────────────────────────────────── */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
          {selected ? (
            <>
              {/* Header */}
              <div style={{
                padding: "12px 16px 10px",
                borderBottom: "1px solid var(--border-color, #303030)",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 8,
                flexShrink: 0,
              }}>
                <span style={{ fontFamily: "monospace", fontWeight: 700, fontSize: 15 }}>
                  {selected.functionName}
                </span>
                <Tag color={selected.functionType === "UDF" ? "teal" : "gold"} style={{ flexShrink: 0 }}>
                  {selected.functionType === "UDF" ? "User-defined" : "Built-in"}
                </Tag>
              </div>

              {/* Tab bar */}
              <div style={{
                display: "flex",
                borderBottom: "1px solid var(--border-color, #303030)",
                paddingLeft: 12,
                flexShrink: 0,
              }}>
                {(["details", "ask-ai"] as const).map((tab) => (
                  <button
                    key={tab}
                    onClick={() => setActiveTab(tab)}
                    style={{
                      background: "none",
                      border: "none",
                      borderBottom: activeTab === tab ? "2px solid var(--ant-color-primary, #177ddc)" : "2px solid transparent",
                      padding: "8px 12px",
                      fontSize: 13,
                      cursor: "pointer",
                      color: activeTab === tab ? "var(--ant-color-primary, #177ddc)" : "var(--text-muted, #888)",
                      fontWeight: activeTab === tab ? 500 : 400,
                      marginBottom: -1,
                    }}
                  >
                    {tab === "details" ? "Details" : "Ask AI"}
                  </button>
                ))}
              </div>

              {/* Tab content — flex: 1 with minHeight: 0 so scroll works */}
              <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>

                {activeTab === "details" && (
                  <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
                    <div style={{ flex: 1, overflowY: "auto", padding: "12px 16px" }}>
                      {detailLoading ? (
                        <div style={{ textAlign: "center", paddingTop: 40 }}>
                          <Spin size="small" />
                        </div>
                      ) : overloads.length === 0 ? (
                        <Empty description="No details available" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                      ) : (
                        overloads.map((ol, i) => (
                          <div key={i} style={{
                            marginBottom: i < overloads.length - 1 ? 20 : 0,
                            paddingBottom: i < overloads.length - 1 ? 20 : 0,
                            borderBottom: i < overloads.length - 1 ? "1px dashed var(--border-color, #303030)" : "none",
                          }}>
                            <div style={{
                              fontFamily: "monospace",
                              fontSize: 12,
                              background: "var(--code-bg, #1e1e1e)",
                              borderRadius: 4,
                              padding: "8px 10px",
                              whiteSpace: "pre-wrap",
                              wordBreak: "break-word",
                              lineHeight: 1.6,
                              border: "1px solid var(--border-color, #303030)",
                            }}>
                              {ol.functionSignature}
                            </div>
                            {ol.description && (
                              <Text style={{ fontSize: 12, display: "block", marginTop: 8, lineHeight: 1.6 }}>
                                {ol.description}
                              </Text>
                            )}
                          </div>
                        ))
                      )}
                    </div>
                    <div style={{
                      padding: "10px 16px",
                      borderTop: "1px solid var(--border-color, #303030)",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      flexShrink: 0,
                    }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        Inserts <code style={{ fontSize: 11 }}>{selected.functionName}(</code> at the cursor position
                      </Text>
                      <Button type="primary" icon={<EnterOutlined />} onClick={handleInsert}>
                        Insert into Editor
                      </Button>
                    </div>
                  </div>
                )}

                {activeTab === "ask-ai" && (
                  <FnChat fn={selected} overloads={overloads} />
                )}

              </div>
            </>
          ) : (
            <div style={{ display: "flex", alignItems: "center", justifyContent: "center", flex: 1 }}>
              <Empty description="Select a function" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
