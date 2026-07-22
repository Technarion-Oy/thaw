// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, useState, useCallback, useMemo, type ReactNode } from "react";
import { Button, Modal, Tooltip, Input, Dropdown } from "antd";
import type { MenuProps } from "antd";
import {
  FileOutlined, CodeOutlined, PlusOutlined, CloseOutlined, DiffOutlined, ExperimentOutlined,
  RobotOutlined, CaretDownOutlined, SearchOutlined, EditOutlined, CloseSquareOutlined,
  DoubleRightOutlined, SaveOutlined, ClearOutlined, SplitCellsOutlined, MergeCellsOutlined,
} from "@ant-design/icons";
import type { Tab } from "../../store/queryStore";
import { useQueryStore } from "../../store/queryStore";
import { GetTabSessionID } from "../../../wailsjs/go/app/App";
import { useConnectionStore } from "../../store/connectionStore";
import { getEditorInstance } from "./editorRef";

const CLR_BORDER       = "var(--border)";
const CLR_BG           = "var(--bg)";
const CLR_BG_ACTIVE    = "var(--bg-raised)";
const CLR_TEXT         = "var(--text-muted)";
const CLR_TEXT_ACTIVE  = "var(--text)";
const CLR_ACCENT       = "var(--accent)";

// Same platform check QueryPage's global keydown handler and KeyboardShortcutsModal use —
// menu shortcut hints must match the modifier keys actually bound on this platform.
// Guard `navigator` so importing this module under a non-DOM env (vitest's `node`
// environment on Node <21, which has no global navigator) doesn't throw at load.
const isMac = typeof navigator !== "undefined" && /Macintosh/i.test(navigator.userAgent);

// Icon for a tab, matching the tab-strip logic (diff → mcp → notebook → file → scratch).
function tabIcon(tab: Tab, size = 11) {
  const style = { fontSize: size, flexShrink: 0 };
  if (tab.diff)       return <DiffOutlined style={style} />;
  if (tab.mcpOrigin)  return <RobotOutlined style={{ ...style, color: "var(--accent)" }} />;
  if (tab.kind === "notebook") return <ExperimentOutlined style={style} />;
  if (tab.path)       return <FileOutlined style={style} />;
  return <CodeOutlined style={style} />;
}

// Title prefix matching the tab strip: orphan ↺ (warning) or dirty • (accent),
// colored separately so the two states are distinguishable at a glance.
function tabPrefix(tab: Tab) {
  if (tab.orphaned) return <span style={{ color: "var(--warning)" }}>↺ </span>;
  if (tab.path && tab.sql !== tab.savedSql) return <span style={{ color: "var(--accent)" }}>• </span>;
  return null;
}

// Signature of exactly the fields the tab strip renders (id/title/path/kind, the
// three icon flags, and the dirty flag). TabBar subscribes to the joined signature
// of all tabs so per-keystroke SQL edits — which change none of these once the
// dirty flag has flipped — don't re-render the strip. (#762)
//
// INVARIANT: every field the strip renders MUST appear here, or that field going
// stale won't trigger a re-render. Exported and unit-tested (TabBar.test.ts) so a
// future edit that adds a rendered field without adding it here fails the test.
// Full label to show in a truncation tooltip: the backing file's full path for
// file tabs (so several files from the same directory are distinguishable),
// otherwise the tab title. (issue #829)
function tabFullLabel(t: Tab): string {
  return t.path ?? t.title;
}

// A single-line, ellipsis-truncated label that shows an AntD Tooltip with the
// full text ONLY when the text actually overflows its container — so short names
// that fully fit don't get a redundant tooltip. Truncation is measured on hover
// (`scrollWidth > clientWidth`) rather than at render, so it stays correct as the
// container resizes. Callers pass `overlayStyle` to lift the portal above the
// position:fixed Active Files panel (z-index 9999). (issue #829)
function OverflowTooltip({
  fullText,
  overlayStyle,
  onDoubleClick,
  children,
}: {
  fullText: string;
  overlayStyle?: React.CSSProperties;
  onDoubleClick?: (e: React.MouseEvent) => void;
  children: ReactNode;
}) {
  const spanRef = useRef<HTMLSpanElement>(null);
  const [truncated, setTruncated] = useState(false);
  return (
    <Tooltip
      title={truncated ? fullText : undefined}
      mouseEnterDelay={0.5}
      placement="bottom"
      overlayStyle={overlayStyle}
    >
      <span
        ref={spanRef}
        onMouseEnter={() => {
          const el = spanRef.current;
          if (el) setTruncated(el.scrollWidth > el.clientWidth);
        }}
        onDoubleClick={onDoubleClick}
        style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1 }}
      >
        {children}
      </span>
    </Tooltip>
  );
}

export function tabStripSignature(t: Tab): string {
  return [
    t.id, t.title, t.path ?? "", t.kind ?? "",
    `${t.diff ? 1 : 0}${t.mcpOrigin ? 1 : 0}${t.orphaned ? 1 : 0}${t.sql !== t.savedSql ? 1 : 0}`,
  ].join("\u0000");
}

export default function TabBar() {
  // Re-render only when tab *metadata* the strip actually shows changes — NOT on
  // every keystroke. A per-keystroke SQL edit rebuilds the `tabs` array, but the
  // tab strip renders only id/title/path/kind/diff/mcpOrigin/orphaned and the
  // dirty flag (`sql !== savedSql`, which flips at most once). Subscribing to a
  // signature of exactly those fields, then snapshotting the live array via a
  // `useMemo` keyed on that signature, keeps the strip off the typing hot path
  // while staying display-correct (every rendered field is in the signature, so a
  // snapshot is equivalent until the signature changes). (#762)
  const tabsSig = useQueryStore((s) => s.tabs.map(tabStripSignature).join("\u0001"));
  const tabs = useMemo(() => useQueryStore.getState().tabs, [tabsSig]);
  const activeTabId = useQueryStore((s) => s.activeTabId);
  const activateTab = useQueryStore((s) => s.activateTab);
  // closeTab is invoked via "thaw:request-close-tab" event handled in QueryPage.
  const moveTab     = useQueryStore((s) => s.moveTab);
  const renameTab   = useQueryStore((s) => s.renameTab);
  const openScratch = useQueryStore((s) => s.openScratch);
  const splitTabId  = useQueryStore((s) => s.splitTabId);
  const setSplitTab = useQueryStore((s) => s.setSplitTab);

  const draggingId  = useRef<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ id: string; before: boolean } | null>(null);
  const [bulkCloseConfirm, setBulkCloseConfirm] = useState<{ ids: string[]; dirtyCount: number } | null>(null);
  // Which tab's context menu is open, if any — lets buildTabMenuItems run only for
  // the tab actually showing a menu instead of once per tab on every render.
  const [openTabMenuId, setOpenTabMenuId] = useState<string | null>(null);
  // Same, but for the Active Files dropdown's per-row context menu. Kept separate
  // from openTabMenuId so right-clicking a panel row doesn't also pop the strip
  // tab's menu for the same id.
  const [openPanelMenuId, setOpenPanelMenuId] = useState<string | null>(null);

  // DOM nodes of the strip's tab elements, so a rename started from the Active
  // Files dropdown can scroll its (possibly overflowed) tab into view.
  const tabRefs = useRef<Record<string, HTMLDivElement | null>>({});

  // Track which tab the pointer is hovering over so the close button
  // only appears on hover (less cluttered when many tabs are open).
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  // Id of the strip tab whose title span is currently overflowing its 220px cap,
  // measured on hover. When set, that tab's tooltip (which otherwise shows only the
  // per-tab session ID) also surfaces the full title/path so overflowed names are
  // readable without activating the tab. (issue #829)
  const [truncatedTabId, setTruncatedTabId] = useState<string | null>(null);

  // Inline tab rename (non-file tabs only — file tabs derive their title from the path).
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");
  // Guards against the blur that fires after the input is removed (on Enter/Escape)
  // re-running the rename — and lets Escape cancel without committing.
  const renameDoneRef = useRef(false);
  const startRename = (tab: Tab) => {
    // Orphaned tabs (lost their backing file, ↺ prefix) are pending a save/discard
    // decision, not a free-form scratch tab — don't allow renaming them.
    if (tab.path || tab.diff || tab.orphaned) return;
    renameDoneRef.current = false;
    setRenamingId(tab.id);
    setRenameValue(tab.title);
  };
  const commitRename = () => {
    if (renamingId && !renameDoneRef.current) {
      renameDoneRef.current = true;
      renameTab(renamingId, renameValue);
    }
    setRenamingId(null);
  };
  const cancelRename = () => {
    renameDoneRef.current = true; // suppress the trailing onBlur commit
    setRenamingId(null);
  };
  // Rename triggered from the Active Files dropdown: close the panel and scroll
  // the tab into view, since the inline rename input lives in the strip and the
  // tab is often the very one that overflowed out of sight.
  const startRenameFromPanel = (tab: Tab) => {
    setActiveFilesOpen(false);
    startRename(tab);
    requestAnimationFrame(() =>
      tabRefs.current[tab.id]?.scrollIntoView({ inline: "nearest", block: "nearest" }));
  };

  // Active Files dropdown — searchable list of all open tabs (issue #468).
  // Each row has a hover close button and a right-click context menu (issue #767)
  // so tabs that overflow the strip can still be closed, renamed, etc.
  // The panel is position:fixed (anchored to the trigger) because the tab bar's
  // overflow-x:auto forces overflow-y:auto, which would otherwise clip it.
  const [activeFilesOpen, setActiveFilesOpen] = useState(false);
  const [activeFilesFilter, setActiveFilesFilter] = useState("");
  const activeFilesBtnRef = useRef<HTMLDivElement>(null);
  const activeFilesPanelRef = useRef<HTMLDivElement>(null);
  const [activeFilesPos, setActiveFilesPos] = useState<{ top: number; right: number }>({ top: 0, right: 0 });

  // Close and return focus to the editor, so keyboard-driven flows (⌘⇧E, Esc)
  // don't strand focus on document.body.
  const closeActiveFiles = useCallback(() => {
    setActiveFilesOpen(false);
    getEditorInstance()?.focus();
  }, []);

  const openActiveFiles = useCallback(() => {
    setActiveFilesOpen((prev) => {
      if (prev) { getEditorInstance()?.focus(); return false; }
      const rect = activeFilesBtnRef.current?.getBoundingClientRect();
      if (rect) setActiveFilesPos({ top: rect.bottom, right: window.innerWidth - rect.right });
      return true;
    });
  }, []);

  // Open via ⌘⇧E / Ctrl+Shift+E (dispatched from QueryPage's global handler).
  useEffect(() => {
    window.addEventListener("thaw:open-active-files", openActiveFiles);
    return () => window.removeEventListener("thaw:open-active-files", openActiveFiles);
  }, [openActiveFiles]);

  // Close on outside click and Escape; reset the filter when closing.
  useEffect(() => {
    if (!activeFilesOpen) { setActiveFilesFilter(""); return; }
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") closeActiveFiles(); };
    const dismiss = (e: MouseEvent) => {
      const t = e.target as Node;
      // A row's context menu renders in a portal on document.body, outside the
      // panel — clicking one of its items must not dismiss the panel first, or
      // the unmount would race (and swallow) the menu action's own click.
      if (t instanceof Element && t.closest(".ant-dropdown")) return;
      if (!activeFilesPanelRef.current?.contains(t) && !activeFilesBtnRef.current?.contains(t)) {
        setActiveFilesOpen(false);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", dismiss);
    return () => { document.removeEventListener("keydown", onKey); document.removeEventListener("mousedown", dismiss); };
  }, [activeFilesOpen, closeActiveFiles]);

  // Session ID cache for tab tooltips (fetched lazily on hover).
  // Only caches non-empty results; tabs without sessions are re-checked on hover.
  const isConnected = useConnectionStore((s) => s.isConnected);
  const [sessionIds, setSessionIds] = useState<Record<string, string>>({});
  const sessionIdsRef = useRef(sessionIds);
  sessionIdsRef.current = sessionIds;
  // Clear stale session IDs when disconnecting (old IDs are invalid after reconnect).
  useEffect(() => {
    if (!isConnected) setSessionIds({});
  }, [isConnected]);
  const fetchingRef = useRef<Set<string>>(new Set());
  const fetchTab = useCallback((tabId: string) => {
    if (!isConnected) return;
    if (sessionIdsRef.current[tabId]) return; // already have a session ID
    if (fetchingRef.current.has(tabId)) return; // in-flight
    fetchingRef.current.add(tabId);
    GetTabSessionID(tabId)
      .then((id) => {
        if (id) setSessionIds((prev) => ({ ...prev, [tabId]: id }));
      })
      .catch(() => {})
      .finally(() => fetchingRef.current.delete(tabId));
  }, [isConnected]);

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
    if (dirtyCount > 0) {
      setBulkCloseConfirm({ ids, dirtyCount });
    } else {
      closeDirect(ids);
    }
  };

  // Tab-independent, so computed once per render rather than once per tab inside
  // buildTabMenuItems.
  const savedTabs = tabs.filter((t) => t.sql === t.savedSql);

  // Right-click tab menu — shared visual language with the Active Files dropdown
  // and (as far as Monaco's API allows) the editor's own context menu: icons,
  // dividers, danger styling on destructive actions, keybinding hints via `extra`.
  // Only called for the tab whose menu is actually open (see the Dropdown below) —
  // not on every render of every tab.
  // onRename defaults to the strip's inline rename; the Active Files dropdown
  // passes startRenameFromPanel so rename works even for an overflowed tab.
  const buildTabMenuItems = (tab: Tab, onRename: (t: Tab) => void = startRename): MenuProps["items"] => {
    const tabIdx     = tabs.findIndex((t) => t.id === tab.id);
    const rightTabs  = tabs.slice(tabIdx + 1);
    const otherTabs  = tabs.filter((t) => t.id !== tab.id);
    const splitCandidates = otherTabs.filter((t) => !t.diff);

    const items: MenuProps["items"] = [];

    if (!tab.path && !tab.diff && !tab.orphaned) {
      items.push({ key: "rename", icon: <EditOutlined />, label: "Rename", onClick: () => onRename(tab) });
      items.push({ type: "divider" });
    }

    items.push({
      key: "close",
      icon: <CloseOutlined />,
      label: "Close",
      extra: isMac ? "⌘W" : "Ctrl+W",
      onClick: () => window.dispatchEvent(new CustomEvent("thaw:request-close-tab", { detail: { tabId: tab.id } })),
    });
    if (otherTabs.length > 0) {
      items.push({ key: "close-others", icon: <CloseSquareOutlined />, danger: true, label: "Close Others", onClick: () => requestCloseMany(otherTabs.map((t) => t.id)) });
    }
    if (rightTabs.length > 0) {
      items.push({ key: "close-right", icon: <DoubleRightOutlined />, label: "Close to the Right", onClick: () => requestCloseMany(rightTabs.map((t) => t.id)) });
    }
    if (savedTabs.length > 0) {
      items.push({ key: "close-saved", icon: <SaveOutlined />, label: "Close Saved", onClick: () => requestCloseMany(savedTabs.map((t) => t.id)) });
    }
    items.push({ key: "close-all", icon: <ClearOutlined />, danger: true, label: "Close All", onClick: () => requestCloseMany(tabs.map((t) => t.id)) });

    items.push({ type: "divider" });

    if (splitTabId) {
      items.push({ key: "close-split", icon: <MergeCellsOutlined />, label: "Close split view", onClick: () => setSplitTab(null) });
    } else {
      items.push({
        key: "split",
        icon: <SplitCellsOutlined />,
        label: "Split with",
        disabled: splitCandidates.length === 0,
        children: splitCandidates.map((t) => ({ key: `split-${t.id}`, label: t.title, onClick: () => setSplitTab(t.id) })),
      });
    }

    return items;
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "stretch",
        background: CLR_BG,
        borderBottom: `1px solid ${CLR_BORDER}`,
        flexShrink: 0,
      }}
    >
      {/* Scrolling region: tabs + the "+" button. The Active Files arrow lives
          outside this so it stays pinned when tabs overflow the bar. */}
      <div
        style={{
          display: "flex",
          alignItems: "stretch",
          overflowX: "auto",
          flex: 1,
          minWidth: 0,
          scrollbarWidth: "none",
        }}
      >
      {tabs.map((tab) => {
        const active  = tab.id === activeTabId;
        const hovered = tab.id === hoveredId;

        const isDropBefore = dropTarget?.id === tab.id && dropTarget.before;
        const isDropAfter  = dropTarget?.id === tab.id && !dropTarget.before;

        const sessionId = sessionIds[tab.id];
        const sessionLine = !isConnected
          ? undefined
          : sessionId
          ? `Session ID: ${sessionId}`
          : "No active session";
        // Show the full title/path first (bold) only when the strip title is
        // truncated, then the session line — either may be absent. (#829)
        const showFullTitle = truncatedTabId === tab.id;
        const tooltipText: ReactNode =
          showFullTitle || sessionLine ? (
            <>
              {showFullTitle && <div style={{ fontWeight: 600 }}>{tabFullLabel(tab)}</div>}
              {sessionLine && <div>{sessionLine}</div>}
            </>
          ) : undefined;

        return (
          <Tooltip key={tab.id} title={tooltipText} mouseEnterDelay={0.6} placement="bottom">
          <Dropdown
            trigger={["contextMenu"]}
            open={openTabMenuId === tab.id}
            onOpenChange={(open) => setOpenTabMenuId(open ? tab.id : null)}
            menu={{ items: openTabMenuId === tab.id ? buildTabMenuItems(tab) : [] }}
          >
          <div
            ref={(el) => { tabRefs.current[tab.id] = el; }}
            draggable={renamingId !== tab.id}
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
            {tabIcon(tab)}

            {renamingId === tab.id ? (
              <input
                autoFocus
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                onBlur={commitRename}
                onKeyDown={(e) => {
                  e.stopPropagation();
                  if (e.key === "Enter") commitRename();
                  else if (e.key === "Escape") cancelRename();
                }}
                style={{
                  flex: 1,
                  minWidth: 0,
                  background: "var(--bg)",
                  color: CLR_TEXT_ACTIVE,
                  border: `1px solid ${CLR_ACCENT}`,
                  borderRadius: 3,
                  fontSize: 12,
                  padding: "0 4px",
                  outline: "none",
                }}
              />
            ) : (
              <span
                onDoubleClick={(e) => { e.stopPropagation(); startRename(tab); }}
                onMouseEnter={(e) =>
                  setTruncatedTabId(e.currentTarget.scrollWidth > e.currentTarget.clientWidth ? tab.id : null)}
                style={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  flex: 1,
                }}>
                {tabPrefix(tab)}{tab.title}
              </span>
            )}

            {/* Close button — always reserve space so layout doesn't shift,
                but only show the icon on hover or when this is the active tab
                (and there is more than one tab). */}
            <span
              style={{
                width: 16,
                height: 16,
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
                <CloseOutlined style={{ fontSize: 10, opacity: 0.7 }} />
              )}
            </span>

            {/* Drop indicators */}
            {isDropBefore && <div style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
            {isDropAfter  && <div style={{ position: "absolute", right: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
          </div>
          </Dropdown>
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
      </div>

      {/* Active Files dropdown — searchable list of every open tab (issue #468).
          Pinned to the right, outside the scroll region, so it's always visible. */}
      <div ref={activeFilesBtnRef} style={{ display: "flex", flexShrink: 0, borderLeft: `1px solid ${CLR_BORDER}` }}>
        <Tooltip title="Active files (⌘⇧E)" mouseEnterDelay={0.6} placement="bottom">
          <div
            onClick={openActiveFiles}
            onMouseEnter={() => setHoveredId("__active__")}
            onMouseLeave={() => setHoveredId(null)}
            style={{
              display: "flex",
              alignItems: "center",
              height: "100%",
              padding: "0 10px",
              cursor: "pointer",
              color: activeFilesOpen ? CLR_TEXT_ACTIVE : CLR_TEXT,
              fontSize: 11,
              background: (activeFilesOpen || hoveredId === "__active__") ? "color-mix(in srgb, var(--text) 5%, transparent)" : "transparent",
            }}
          >
            <CaretDownOutlined />
          </div>
        </Tooltip>

        {activeFilesOpen && (
          <div
            ref={activeFilesPanelRef}
            style={{
              position: "fixed",
              top: activeFilesPos.top,
              right: activeFilesPos.right,
              zIndex: 9999,
              width: 280,
              background: "var(--bg-overlay)",
              border: "1px solid var(--border)",
              borderRadius: 4,
              boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
            }}
          >
            <div style={{ padding: 6, borderBottom: "1px solid var(--border)" }}>
              <Input
                size="small"
                autoFocus
                allowClear
                prefix={<SearchOutlined style={{ color: CLR_TEXT }} />}
                placeholder="Filter open tabs…"
                value={activeFilesFilter}
                onChange={(e) => setActiveFilesFilter(e.target.value)}
              />
            </div>
            <div style={{ maxHeight: 360, overflowY: "auto", padding: "2px 0" }}>
              {(() => {
                const f = activeFilesFilter.trim().toLowerCase();
                const matches = tabs.filter((t) => !f || t.title.toLowerCase().includes(f));
                if (matches.length === 0) {
                  return <div style={{ padding: "8px 12px", color: "var(--text-faint)", fontSize: 12 }}>No matching tabs</div>;
                }
                return matches.map((t) => (
                  <Dropdown
                    key={t.id}
                    trigger={["contextMenu"]}
                    open={openPanelMenuId === t.id}
                    onOpenChange={(open) => setOpenPanelMenuId(open ? t.id : null)}
                    menu={{ items: openPanelMenuId === t.id ? buildTabMenuItems(t, startRenameFromPanel) : [] }}
                    // The Active Files panel is z-index 9999; without this the
                    // context-menu portal (default ~1050) renders behind it.
                    overlayStyle={{ zIndex: 10000 }}
                  >
                  <div
                    className="ctx-item"
                    onClick={() => { activateTab(t.id); closeActiveFiles(); }}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 8,
                      background: t.id === activeTabId ? CLR_BG_ACTIVE : undefined,
                      color: t.id === activeTabId ? CLR_TEXT_ACTIVE : undefined,
                    }}
                  >
                    {tabIcon(t)}
                    {/* Tooltip with the full title/path, shown only when the row
                        is truncated. overlayStyle lifts the portal above the panel
                        (z-index 9999), same reason the context menu needs it. (#829) */}
                    <OverflowTooltip fullText={tabFullLabel(t)} overlayStyle={{ zIndex: 10000 }}>
                      {tabPrefix(t)}{t.title}
                    </OverflowTooltip>
                    {/* Close button — revealed on row hover (see .ctx-item-close).
                        Routes through the same request-close-tab flow as the strip
                        so dirty tabs still prompt before closing. */}
                    <span
                      className="ctx-item-close"
                      title="Close tab"
                      onClick={(e) => {
                        e.stopPropagation();
                        window.dispatchEvent(new CustomEvent("thaw:request-close-tab", { detail: { tabId: t.id } }));
                      }}
                      style={{ display: "flex", alignItems: "center", justifyContent: "center", width: 16, height: 16, flexShrink: 0 }}
                    >
                      <CloseOutlined style={{ fontSize: 10 }} />
                    </span>
                  </div>
                  </Dropdown>
                ));
              })()}
            </div>
          </div>
        )}
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
    </div>
  );
}
