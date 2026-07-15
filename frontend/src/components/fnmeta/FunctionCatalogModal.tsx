// SPDX-License-Identifier: GPL-3.0-or-later
// Commercial use of this software is restricted to patents holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo, useRef } from "react";
import { Modal, Input, Button, Tag, Spin, Segmented, Empty, Typography, Tooltip } from "antd";
import { SearchOutlined, FunctionOutlined, EnterOutlined } from "@ant-design/icons";
import { GetAllFunctionNames, GetFunctionTooltip } from "../../../wailsjs/go/app/App";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { insertAtCursor } from "../editor/editorRef";
import type { fnmeta } from "../../../wailsjs/go/models";

const { Text } = Typography;

type FnMeta = fnmeta.FunctionMeta;

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
  const latestSelectRef               = useRef<string | null>(null);

  // Load full name list once on mount
  useEffect(() => {
    GetAllFunctionNames()
      .then((fns) => {
        const sorted = [...fns].sort((a, b) => a.functionName.localeCompare(b.functionName));
        setAllFns(sorted);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  function selectFn(fn: FnMeta) {
    const name = fn.functionName;
    latestSelectRef.current = name;
    setSelected(fn);
    setOverloads([]);
    setDetailLoading(true);
    GetFunctionTooltip(name)
      .then((ol)  => { if (latestSelectRef.current === name) setOverloads(ol ?? []); })
      .catch(()   => { if (latestSelectRef.current === name) setOverloads([]); })
      .finally(() => { if (latestSelectRef.current === name) setDetailLoading(false); });
  }

  const filtered = useMemo(() => {
    const q = search.trim().toUpperCase();
    return allFns.filter((fn) => {
      if (typeFilter === "BUILTIN" && fn.functionType !== "BUILTIN") return false;
      if (typeFilter === "UDF"     && fn.functionType !== "UDF")     return false;
      return q === "" || fn.functionName.startsWith(q);
    });
  }, [allFns, search, typeFilter]);

  // Always select the first visible item when the filter changes.
  // This also resets selection when the search is cleared, avoiding stale state.
  useEffect(() => {
    if (filtered.length === 0) return;
    const first = filtered[0];
    if (selected?.functionName === first.functionName && selected?.functionType === first.functionType) return;
    selectFn(first);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtered]);

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
          borderRight: "1px solid var(--border)",
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
                const isSelected = selected?.functionName === fn.functionName && selected?.functionType === fn.functionType;
                return (
                  <div
                    key={fn.functionName + "::" + fn.functionType}
                    onClick={() => selectFn(fn)}
                    style={{
                      padding: "5px 10px",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: 6,
                      backgroundColor: isSelected ? "color-mix(in oklab, var(--accent) 12%, transparent)" : "transparent",
                      borderLeft: isSelected ? "2px solid var(--accent)" : "2px solid transparent",
                    }}
                  >
                    <span style={{
                      fontSize: 12,
                      fontFamily: "monospace",
                      fontWeight: isSelected ? 600 : 400,
                      color: fn.functionType === "UDF" ? "var(--cell-accent-sql)" : undefined,
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
          <div style={{ padding: "6px 10px", fontSize: 11, color: "var(--text-muted, #888)", borderTop: "1px solid var(--border)" }}>
            {filtered.length} / {allFns.length} functions
          </div>
        </div>

        {/* ── Right: detail ─────────────────────────────────────────── */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
          {selected ? (
            <>
              {/* Header */}
              <div style={{
                padding: "12px 16px 10px",
                borderBottom: "1px solid var(--border)",
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

              {/* Detail content */}
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
                        borderBottom: i < overloads.length - 1 ? "1px dashed var(--border)" : "none",
                      }}>
                        <div style={{
                          fontFamily: "monospace",
                          fontSize: 12,
                          background: "var(--cell-editor-bg)",
                          color: "var(--text)",
                          borderRadius: 4,
                          padding: "8px 10px",
                          whiteSpace: "pre-wrap",
                          wordBreak: "break-word",
                          lineHeight: 1.6,
                          border: "1px solid var(--border)",
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
                  borderTop: "1px solid var(--border)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "flex-end",
                  flexShrink: 0,
                }}>
                  {selected.functionType === "BUILTIN" && (
                    <button
                      onClick={() => BrowserOpenURL(`https://docs.snowflake.com/en/sql-reference/functions/${selected.functionName.toLowerCase()}`)}
                      style={{
                        background: "none",
                        border: "none",
                        padding: 0,
                        cursor: "pointer",
                        fontSize: 12,
                        color: "var(--accent)",
                        textDecoration: "underline",
                        textUnderlineOffset: 2,
                        textAlign: "left",
                        marginRight: "auto",
                      }}
                    >
                      📖 Snowflake documentation for {selected.functionName}
                    </button>
                  )}
                  <Tooltip title={<>Inserts <code style={{ fontSize: 11 }}>{selected.functionName}(</code> at the cursor position</>}>
                    <Button type="primary" icon={<EnterOutlined />} onClick={handleInsert}>
                      Insert into Editor
                    </Button>
                  </Tooltip>
                </div>
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
