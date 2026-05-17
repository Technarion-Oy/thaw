// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useRef, useState, useCallback } from "react";
import { Button, Modal, Tooltip } from "antd";
import { FileOutlined, CodeOutlined, PlusOutlined, CloseOutlined, DiffOutlined, ExperimentOutlined } from "@ant-design/icons";
import { useQueryStore } from "../../store/queryStore";
import { GetTabSessionID } from "../../../wailsjs/go/main/App";
import { useConnectionStore } from "../../store/connectionStore";

const CLR_BORDER       = "var(--border)";
const CLR_BG           = "var(--bg)";
const CLR_BG_ACTIVE    = "var(--bg-raised)";
const CLR_TEXT         = "var(--text-muted)";
const CLR_TEXT_ACTIVE  = "var(--text)";
const CLR_ACCENT       = "var(--accent)";

export default function TabBar() {
  const tabs        = useQueryStore((s) => s.tabs);
  const activeTabId = useQueryStore((s) => s.activeTabId);
  const activateTab = useQueryStore((s) => s.activateTab);
  // closeTab is invoked via "thaw:request-close-tab" event handled in QueryPage.
  const moveTab     = useQueryStore((s) => s.moveTab);
  const openScratch = useQueryStore((s) => s.openScratch);
  const splitTabId  = useQueryStore((s) => s.splitTabId);
  const setSplitTab = useQueryStore((s) => s.setSplitTab);

  const draggingId  = useRef<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ id: string; before: boolean } | null>(null);
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; tabId: string } | null>(null);
  const [splitSubmenuOpen, setSplitSubmenuOpen] = useState(false);
  const [bulkCloseConfirm, setBulkCloseConfirm] = useState<{ ids: string[]; dirtyCount: number } | null>(null);

  // Track which tab the pointer is hovering over so the close button
  // only appears on hover (less cluttered when many tabs are open).
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  // Session ID cache for tab tooltips (fetched lazily on hover).
  // Only caches non-empty results; tabs without sessions are re-checked on hover.
  const isConnected = useConnectionStore((s) => s.isConnected);
  const [sessionIds, setSessionIds] = useState<Record<string, string>>({});
  const fetchingRef = useRef<Set<string>>(new Set());
  const fetchTab = useCallback((tabId: string) => {
    if (!isConnected) return;
    if (sessionIds[tabId]) return; // already have a session ID
    if (fetchingRef.current.has(tabId)) return; // in-flight
    fetchingRef.current.add(tabId);
    GetTabSessionID(tabId)
      .then((id) => {
        if (id) setSessionIds((prev) => ({ ...prev, [tabId]: id }));
      })
      .catch(() => {})
      .finally(() => fetchingRef.current.delete(tabId));
  }, [isConnected, sessionIds]);

  // Close a set of tabs directly (no confirmation).
  const closeDirect = (ids: string[]) =>
    ids.forEach((id) => useQueryStore.getState().closeTab(id));

  // Close a set of tabs, showing a confirmation dialog if any are dirty.
  const requestCloseMany = (ids: string[]) => {
    const { tabs: currentTabs } = useQueryStore.getState();
    const dirtyCount = ids.filter((id) => {
      const t = currentTabs.find((tab) => tab.id === id);
      return t && t.sql !== t.savedSql;
    }).length;
    setCtxMenu(null);
    if (dirtyCount > 0) {
      setBulkCloseConfirm({ ids, dirtyCount });
    } else {
      closeDirect(ids);
    }
  };

  // Dismiss context menu on next document click.
  useEffect(() => {
    if (!ctxMenu) return;
    const dismiss = () => setCtxMenu(null);
    document.addEventListener("click", dismiss);
    return () => document.removeEventListener("click", dismiss);
  }, [ctxMenu]);

  // Reset submenu state when context menu closes.
  useEffect(() => {
    if (!ctxMenu) setSplitSubmenuOpen(false);
  }, [ctxMenu]);

  return (
    <div
      style={{
        display: "flex",
        alignItems: "stretch",
        background: CLR_BG,
        borderBottom: `1px solid ${CLR_BORDER}`,
        overflowX: "auto",
        flexShrink: 0,
        scrollbarWidth: "none",
      }}
    >
      {tabs.map((tab) => {
        const active  = tab.id === activeTabId;
        const hovered = tab.id === hoveredId;

        const isDropBefore = dropTarget?.id === tab.id && dropTarget.before;
        const isDropAfter  = dropTarget?.id === tab.id && !dropTarget.before;

        const sessionId = sessionIds[tab.id];
        const tooltipText = !isConnected
          ? undefined
          : sessionId
          ? `Session ID: ${sessionId}`
          : "No active session";

        return (
          <Tooltip key={tab.id} title={tooltipText} mouseEnterDelay={0.6} placement="bottom">
          <div
            draggable
            onClick={() => activateTab(tab.id)}
            onMouseEnter={() => { setHoveredId(tab.id); fetchTab(tab.id); }}
            onMouseLeave={() => setHoveredId(null)}
            onDragStart={(e) => {
              draggingId.current = tab.id;
              e.dataTransfer.effectAllowed = "move";
              e.dataTransfer.setData("text/plain", tab.id);
            }}
            onDragEnd={() => { draggingId.current = null; setDropTarget(null); }}
            onDragOver={(e) => {
              if (!draggingId.current || draggingId.current === tab.id) return;
              e.preventDefault();
              const rect = e.currentTarget.getBoundingClientRect();
              setDropTarget({ id: tab.id, before: e.clientX < rect.left + rect.width / 2 });
            }}
            onDragLeave={() => setDropTarget(null)}
            onDrop={(e) => {
              e.preventDefault();
              if (draggingId.current && draggingId.current !== tab.id && dropTarget) {
                moveTab(draggingId.current, tab.id, dropTarget.before);
              }
              draggingId.current = null;
              setDropTarget(null);
            }}
            onContextMenu={(e) => {
              e.preventDefault();
              setCtxMenu({ x: e.clientX, y: e.clientY, tabId: tab.id });
            }}
            style={{ position: "relative",
              display: "flex",
              alignItems: "center",
              gap: 5,
              padding: "0 10px",
              height: 32,
              cursor: "pointer",
              borderRight: `1px solid ${CLR_BORDER}`,
              borderBottom: active ? `2px solid ${CLR_ACCENT}` : "2px solid transparent",
              background: active ? CLR_BG_ACTIVE : hovered ? "color-mix(in srgb, var(--text) 5%, transparent)" : CLR_BG,
              color: active ? CLR_TEXT_ACTIVE : CLR_TEXT,
              fontSize: 12,
              userSelect: "none",
              flexShrink: 0,
              maxWidth: 220,
              boxSizing: "border-box",
            }}
          >
            {tab.diff
              ? <DiffOutlined style={{ fontSize: 11, flexShrink: 0 }} />
              : tab.kind === "notebook"
              ? <ExperimentOutlined style={{ fontSize: 11, flexShrink: 0 }} />
              : tab.path
              ? <FileOutlined style={{ fontSize: 11, flexShrink: 0 }} />
              : <CodeOutlined style={{ fontSize: 11, flexShrink: 0 }} />
            }

            <span style={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              flex: 1,
            }}>
              {tab.orphaned ? "↺ " : (tab.path && tab.sql !== tab.savedSql ? "• " : "")}{tab.title}
            </span>

            {/* Close button — always reserve space so layout doesn't shift,
                but only show the icon on hover or when this is the active tab
                (and there is more than one tab). */}
            <span
              style={{
                width: 14,
                height: 14,
                flexShrink: 0,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
              onClick={(e) => {
                e.stopPropagation();
                window.dispatchEvent(new CustomEvent("thaw:request-close-tab", { detail: { tabId: tab.id } }));
              }}
            >
              {(active || hovered) && (
                <CloseOutlined style={{ fontSize: 9, opacity: 0.7 }} />
              )}
            </span>

            {/* Drop indicators */}
            {isDropBefore && <div style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
            {isDropAfter  && <div style={{ position: "absolute", right: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
          </div>
          </Tooltip>
        );
      })}

      {/* New scratch tab */}
      <div
        onClick={openScratch}
        onMouseEnter={() => setHoveredId("__plus__")}
        onMouseLeave={() => setHoveredId(null)}
        style={{
          display: "flex",
          alignItems: "center",
          padding: "0 10px",
          cursor: "pointer",
          color: CLR_TEXT,
          fontSize: 12,
          flexShrink: 0,
          background: hoveredId === "__plus__" ? "color-mix(in srgb, var(--text) 5%, transparent)" : "transparent",
        }}
      >
        <PlusOutlined style={{ fontSize: 11 }} />
      </div>

      {/* Bulk-close confirmation modal */}
      <Modal
        open={bulkCloseConfirm !== null}
        title="Unsaved Changes"
        onCancel={() => setBulkCloseConfirm(null)}
        footer={[
          <Button key="cancel" onClick={() => setBulkCloseConfirm(null)}>
            Cancel
          </Button>,
          <Button
            key="saved-only"
            onClick={() => {
              if (!bulkCloseConfirm) return;
              const { tabs: currentTabs } = useQueryStore.getState();
              const cleanIds = bulkCloseConfirm.ids.filter((id) => {
                const t = currentTabs.find((tab) => tab.id === id);
                return t && t.sql === t.savedSql;
              });
              closeDirect(cleanIds);
              setBulkCloseConfirm(null);
            }}
          >
            Close Only Saved
          </Button>,
          <Button
            key="close-all"
            danger
            onClick={() => {
              if (!bulkCloseConfirm) return;
              closeDirect(bulkCloseConfirm.ids);
              setBulkCloseConfirm(null);
            }}
          >
            Close All
          </Button>,
        ]}
      >
        <p>
          {bulkCloseConfirm?.dirtyCount === 1
            ? "1 tab has unsaved changes."
            : `${bulkCloseConfirm?.dirtyCount} tabs have unsaved changes.`}{" "}
          Close them without saving?
        </p>
      </Modal>

      {/* Right-click context menu */}
      {ctxMenu && (() => {
        const ctxTabIdx = tabs.findIndex((t) => t.id === ctxMenu.tabId);
        const others    = tabs.filter((t) => t.id !== ctxMenu.tabId && !t.diff);
        const rightTabs = tabs.slice(ctxTabIdx + 1);
        const otherTabs = tabs.filter((t) => t.id !== ctxMenu.tabId);
        const savedTabs = tabs.filter((t) => t.sql === t.savedSql);

        return (
          <div
            style={{
              position: "fixed", zIndex: 9999,
              top: ctxMenu.y, left: ctxMenu.x,
              background: "var(--bg-overlay)",
              border: "1px solid var(--border)",
              borderRadius: 4, padding: "2px 0",
              minWidth: 180,
              boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
            }}
            onMouseDown={(e) => e.stopPropagation()}
          >
            {/* ── Close actions ─────────────────────────────────────── */}
            <div
              className="ctx-item"
              onClick={() => {
                window.dispatchEvent(new CustomEvent("thaw:request-close-tab", { detail: { tabId: ctxMenu.tabId } }));
                setCtxMenu(null);
              }}
            >
              Close
            </div>

            {otherTabs.length > 0 && (
              <div
                className="ctx-item"
                onClick={() => requestCloseMany(otherTabs.map((t) => t.id))}
              >
                Close Others
              </div>
            )}

            {rightTabs.length > 0 && (
              <div
                className="ctx-item"
                onClick={() => requestCloseMany(rightTabs.map((t) => t.id))}
              >
                Close to the Right
              </div>
            )}

            {savedTabs.length > 0 && (
              <div
                className="ctx-item"
                onClick={() => requestCloseMany(savedTabs.map((t) => t.id))}
              >
                Close Saved
              </div>
            )}

            <div
              className="ctx-item"
              onClick={() => requestCloseMany(tabs.map((t) => t.id))}
            >
              Close All
            </div>

            {/* ── Separator ─────────────────────────────────────────── */}
            <div style={{ height: 1, background: "var(--border)", margin: "2px 0" }} />

            {/* ── Split view ────────────────────────────────────────── */}
            {splitTabId ? (
              <div className="ctx-item" onClick={() => { setSplitTab(null); setCtxMenu(null); }}>
                Close split view
              </div>
            ) : (
              <div
                className="ctx-item"
                style={{ display: "flex", alignItems: "center", justifyContent: "space-between", position: "relative" }}
                onMouseEnter={() => setSplitSubmenuOpen(true)}
                onMouseLeave={() => setSplitSubmenuOpen(false)}
              >
                <span>Split with</span>
                <span style={{ fontSize: 8, marginLeft: 8, opacity: 0.6 }}>▶</span>
                {splitSubmenuOpen && (
                  <div
                    style={{
                      position: "absolute",
                      left: "100%",
                      top: -2,
                      background: "var(--bg-overlay)",
                      border: "1px solid var(--border)",
                      borderRadius: 4,
                      padding: "2px 0",
                      minWidth: 160,
                      boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
                      zIndex: 10000,
                    }}
                  >
                    {others.length > 0
                      ? others.map((t) => (
                          <div
                            key={t.id}
                            className="ctx-item"
                            onClick={() => { setSplitTab(t.id); setCtxMenu(null); }}
                          >
                            {t.title}
                          </div>
                        ))
                      : (
                          <div style={{ padding: "4px 12px", color: "var(--text-faint)", fontSize: 11, whiteSpace: "nowrap" }}>
                            No other tabs
                          </div>
                        )
                    }
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })()}
    </div>
  );
}
