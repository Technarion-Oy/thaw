// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, useState, useCallback, useMemo, type ReactNode } from "react";
import { Button, Modal, Tooltip, Input, Dropdown } from "antd";
import type { MenuProps } from "antd";
import {
  FileOutlined, PlusOutlined, CloseOutlined, DiffOutlined, ExperimentOutlined,
  RobotOutlined, CaretDownOutlined, SearchOutlined, EditOutlined, CloseSquareOutlined,
  DoubleRightOutlined, SaveOutlined, ClearOutlined, SplitCellsOutlined, MergeCellsOutlined,
} from "@ant-design/icons";
import type { Tab } from "../../store/queryStore";
import { useQueryStore } from "../../store/queryStore";
import { GetTabSessionID } from "../../../wailsjs/go/app/App";
import { useConnectionStore } from "../../store/connectionStore";
import { getEditorInstance } from "./editorRef";
import { tabDisplayTitle } from "./tabTitle";

const CLR_BORDER       = "var(--border)";
const CLR_BG_ACTIVE    = "var(--bg-raised)";
const CLR_TEXT         = "var(--text-muted)";
const CLR_TEXT_ACTIVE  = "var(--text)";
const CLR_ACCENT       = "var(--accent)";

// Same platform check QueryPage's global keydown handler and KeyboardShortcutsModal use —
// menu shortcut hints must match the modifier keys actually bound on this platform.
// Guard `navigator` so importing this module under a non-DOM env (vitest's `node`
// environment on Node <21, which has no global navigator) doesn't throw at load.
const isMac = typeof navigator !== "undefined" && /Macintosh/i.test(navigator.userAgent);

// Icon for a tab (diff → mcp → notebook → file). A plain scratch tab gets NO
// icon: when every tab in the strip is a scratch query, an identical glyph on
// each one carries no information and eats ~16px of a 220px cap. Drawing icons
// only for the distinctive kinds is what makes those kinds visible. (#881)
function tabIcon(tab: Tab, size = 11) {
  const style = { fontSize: size, flexShrink: 0 };
  if (tab.diff)       return <DiffOutlined style={style} />;
  if (tab.mcpOrigin)  return <RobotOutlined style={{ ...style, color: "var(--accent)" }} />;
  if (tab.kind === "notebook") return <ExperimentOutlined style={style} />;
  if (tab.path)       return <FileOutlined style={style} />;
  return null;
}

// Dirty/orphan marker: orphan ↺ (warning) wins over dirty • (accent), colored
// separately so the two states are distinguishable at a glance. Only file-backed
// tabs can be dirty — a scratch tab has nowhere to save to.
function tabMark(tab: Tab): { glyph: string; cls: string; title: string } | null {
  if (tab.orphaned) return { glyph: "↺", cls: "qtab-mark-orphan", title: "Backing file is gone" };
  if (tab.path && tab.sql !== tab.savedSql) return { glyph: "•", cls: "qtab-mark-dirty", title: "Unsaved changes" };
  return null;
}

// Same marker as a leading title prefix, for the Active Files rows — a
// full-width list has no layout-shift problem, so the marker stays where the
// eye scans for it.
function tabPrefix(tab: Tab) {
  const mark = tabMark(tab);
  return mark ? <span className={mark.cls}>{mark.glyph} </span> : null;
}

// Signature of exactly the fields the tab strip renders (id/title/derived
// title/path/kind, the three icon flags, the dirty flag and the preview flag).
// TabBar subscribes to the joined signature
// of all tabs so per-keystroke SQL edits — which change none of these once the
// dirty flag has flipped — don't re-render the strip. (#762)
//
// INVARIANT: every field the strip renders MUST appear here, or that field going
// stale won't trigger a re-render. Exported and unit-tested (TabBar.test.ts) so a
// future edit that adds a rendered field without adding it here fails the test.
// Full label to show in a truncation tooltip: the backing file's full path for
// file tabs (so several files from the same directory are distinguishable),
// otherwise the tab title. (issue #829)
// `display` lets a caller that already computed the rendered title (the strip
// does, once per tab) pass it in rather than re-running the derivation.
function tabFullLabel(t: Tab, display = tabDisplayTitle(t)): string {
  return t.path ?? display;
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
    t.id, t.title,
    // The rendered title, which for a still-unnamed scratch tab is derived from
    // its SQL (#881). This is the one signature field that can change while
    // typing — but only when the *derived* label changes (a handful of times
    // while the first statement is written), not once per keystroke.
    tabDisplayTitle(t),
    t.path ?? "", t.kind ?? "",
    `${t.diff ? 1 : 0}${t.mcpOrigin ? 1 : 0}${t.orphaned ? 1 : 0}${t.sql !== t.savedSql ? 1 : 0}${t.preview ? 1 : 0}`,
  ].join("\u0000");
}

export default function TabBar() {
  // Re-render only when tab *metadata* the strip actually shows changes — NOT on
  // every keystroke. A per-keystroke SQL edit rebuilds the `tabs` array, but the
  // tab strip renders only id/title/path/kind/diff/mcpOrigin/orphaned, the dirty
  // flag (`sql !== savedSql`, which flips at most once) and the preview flag (which
  // flips at most once, on promotion). Subscribing to a
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
  const promoteTab  = useQueryStore((s) => s.promoteTab);
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

  // Hover styling (tab tint, close-button reveal, "+"/caret tint) is pure CSS —
  // see `.qtab:hover` in global.css. It used to be a `hoveredId` React state,
  // which re-rendered the whole strip on every pointer move across it, undoing
  // the very thing #762's signature work bought. (#881)

  // Which edges of the scroll region have tabs hidden past them, so the strip
  // can show that it continues instead of just ending at the panel edge.
  const scrollRef = useRef<HTMLDivElement>(null);
  const [clipped, setClipped] = useState({ left: false, right: false });

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
    // Orphaned tabs (lost their backing file, ↺ marker) are pending a save/discard
    // decision, not a free-form scratch tab — don't allow renaming them.
    if (tab.path || tab.diff || tab.orphaned) return;
    renameDoneRef.current = false;
    setRenamingId(tab.id);
    // Seed with what the strip actually shows — which for an unnamed scratch tab
    // is the derived title, not "SQL 3". The input selects it on focus, so the
    // common case (type a real name) is unaffected.
    setRenameValue(tabDisplayTitle(tab));
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

  // Track the clipped edges. Scroll position, container width and the tab set
  // are the only three things that can move them — pointer movement can't, so
  // this never runs on the hover path.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const update = () => {
      const maxScroll = el.scrollWidth - el.clientWidth;
      const next = { left: el.scrollLeft > 1, right: el.scrollLeft < maxScroll - 1 };
      setClipped((prev) => (prev.left === next.left && prev.right === next.right ? prev : next));
    };
    update();
    el.addEventListener("scroll", update, { passive: true });
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => { el.removeEventListener("scroll", update); ro.disconnect(); };
  }, [tabsSig]);

  // Arrow-key navigation over the tablist (WAI-ARIA tabs pattern with automatic
  // activation: moving focus activates the tab, so focus and content never
  // disagree). Bound on the tablist, which holds only tabs — the "+" button is a
  // sibling, so Arrow keys there don't hijack anything. The rename input stops
  // its own keydowns from bubbling, so renaming is unaffected.
  const onTabListKeyDown = (e: React.KeyboardEvent) => {
    if (tabs.length === 0) return;
    const idx = Math.max(0, tabs.findIndex((t) => t.id === activeTabId));
    const next =
      e.key === "ArrowRight" ? (idx + 1) % tabs.length
      : e.key === "ArrowLeft" ? (idx - 1 + tabs.length) % tabs.length
      : e.key === "Home" ? 0
      : e.key === "End" ? tabs.length - 1
      : -1;
    if (next < 0) return;
    e.preventDefault();
    activateTab(tabs[next].id);
    tabRefs.current[tabs[next].id]?.focus(); // also scrolls it into view
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

  // Whether the strip is hiding tabs in either direction — drives the caret's
  // count badge and its tooltip.
  const anyClipped = clipped.left || clipped.right;

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
        children: splitCandidates.map((t) => ({ key: `split-${t.id}`, label: tabDisplayTitle(t), onClick: () => setSplitTab(t.id) })),
      });
    }

    return items;
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "stretch",
        background: "var(--bg)",
        borderBottom: `1px solid ${CLR_BORDER}`,
        flexShrink: 0,
      }}
    >
      {/* Scrolling region: tabs + the "+" button. The Active Files arrow lives
          outside this so it stays pinned when tabs overflow the bar. The
          wrapper is the positioning context for the edge fades. */}
      <div style={{ position: "relative", display: "flex", flex: 1, minWidth: 0 }}>
      <div ref={scrollRef} className="qtab-scroll">
      <div className="qtab-list" role="tablist" aria-label="Open editor tabs" onKeyDown={onTabListKeyDown}>
      {tabs.map((tab) => {
        const active  = tab.id === activeTabId;
        const mark    = tabMark(tab);
        const title   = tabDisplayTitle(tab);

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
              {showFullTitle && <div style={{ fontWeight: 600 }}>{tabFullLabel(tab, title)}</div>}
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
            className={`qtab${active ? " qtab-active" : ""}${mark ? " qtab-marked" : ""}`}
            role="tab"
            aria-selected={active}
            // Roving tabindex: one stop for the whole strip, arrows move within it.
            tabIndex={active ? 0 : -1}
            draggable={renamingId !== tab.id}
            onClick={() => activateTab(tab.id)}
            // Double-click a preview tab (italic) to pin it, mirroring VS Code and the
            // file browser's double-click-to-promote. Non-preview tabs are unaffected;
            // renameable (non-file) tabs stop the span's own dbl-click before it bubbles.
            onDoubleClick={() => { if (tab.preview) promoteTab(tab.id); }}
            // Hover is styled in CSS; this only warms the session-ID cache.
            onMouseEnter={() => fetchTab(tab.id)}
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
          >
            {tabIcon(tab)}

            {renamingId === tab.id ? (
              <input
                autoFocus
                className="qtab-rename"
                value={renameValue}
                // The seed is the *rendered* title (possibly derived from the SQL),
                // so select it — typing replaces, editing refines.
                onFocus={(e) => e.currentTarget.select()}
                onChange={(e) => setRenameValue(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                onBlur={commitRename}
                onKeyDown={(e) => {
                  e.stopPropagation();
                  if (e.key === "Enter") commitRename();
                  else if (e.key === "Escape") cancelRename();
                }}
              />
            ) : (
              <span
                className={`qtab-title${tab.preview ? " qtab-preview" : ""}`}
                // File/diff/orphan tabs aren't renameable — let their double-click
                // bubble to the tab div's promote handler. Only renameable (non-file)
                // tabs consume it to start an inline rename.
                onDoubleClick={(e) => {
                  if (tab.path || tab.diff || tab.orphaned) return;
                  e.stopPropagation();
                  startRename(tab);
                }}
                onMouseEnter={(e) =>
                  setTruncatedTabId(e.currentTarget.scrollWidth > e.currentTarget.clientWidth ? tab.id : null)}
              >
                {title}
              </span>
            )}

            {/* The reserved trailing slot: dirty/orphan marker at rest, ✕ on
                hover (VS Code's convention). One 16px slot for both means a tab
                going dirty shifts nothing, and the marker survives while the
                pointer is elsewhere in the strip. Which of the two shows is
                decided in CSS — see `.qtab-slot` in global.css. */}
            <button
              type="button"
              className="qtab-slot"
              aria-label={`Close ${title}`}
              // Not a tab stop: the tablist owns arrow-key navigation, and ⌘W
              // closes the focused tab. A stop per tab would double the strip's
              // Tab-key cost for no gain.
              tabIndex={-1}
              onClick={(e) => {
                e.stopPropagation();
                window.dispatchEvent(new CustomEvent("thaw:request-close-tab", { detail: { tabId: tab.id } }));
              }}
            >
              {mark && <span className={`qtab-mark ${mark.cls}`} title={mark.title}>{mark.glyph}</span>}
              {/* Wrapper span, not a className on the icon: antd's own `.anticon`
                  display rule is injected after this stylesheet and would win. */}
              <span className="qtab-close"><CloseOutlined style={{ fontSize: 10 }} /></span>
            </button>

            {/* Drop indicators */}
            {isDropBefore && <div style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
            {isDropAfter  && <div style={{ position: "absolute", right: 0, top: 0, bottom: 0, width: 2, background: CLR_ACCENT, pointerEvents: "none" }} />}
          </div>
          </Dropdown>
          </Tooltip>
        );
      })}

      </div>

      {/* New scratch tab — a sibling of the tablist, not a member of it. */}
      <Tooltip title="New query tab" mouseEnterDelay={0.6} placement="bottom">
        <button type="button" className="qtab-btn" aria-label="New query tab" onClick={openScratch}>
          <PlusOutlined style={{ fontSize: 11 }} />
        </button>
      </Tooltip>
      </div>

      {/* Overflow cues: a fade on whichever edge still has tabs behind it. The
          scrollbar is hidden, so without these the strip just stops mid-tab
          with nothing to say that it continues. */}
      {clipped.left  && <div className="qtab-fade qtab-fade-left" />}
      {clipped.right && <div className="qtab-fade qtab-fade-right" />}
      </div>

      {/* Active Files dropdown — searchable list of every open tab (issue #468).
          Pinned to the right, outside the scroll region, so it's always visible. */}
      <div ref={activeFilesBtnRef} style={{ display: "flex", flexShrink: 0, borderLeft: `1px solid ${CLR_BORDER}` }}>
        <Tooltip
          title={anyClipped ? `Active files — ${tabs.length} open, some hidden (⌘⇧E)` : "Active files (⌘⇧E)"}
          mouseEnterDelay={0.6}
          placement="bottom"
        >
          <button
            type="button"
            className={`qtab-btn${activeFilesOpen ? " open" : ""}`}
            style={{ height: "100%", fontSize: 11 }}
            aria-label="Active files"
            aria-haspopup="menu"
            aria-expanded={activeFilesOpen}
            onClick={openActiveFiles}
          >
            <CaretDownOutlined />
            {/* Tab count, but only while tabs are clipped — this dropdown is the
                escape hatch for the ones the strip can't show. */}
            {anyClipped && <span className="qtab-count">{tabs.length}</span>}
          </button>
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
                // Rendered title per tab, computed once and used for the filter,
                // the row label and the truncation tooltip. Filtering on the
                // rendered title matters: matching a "SQL 3" the user can't see
                // anywhere would be baffling.
                const matches = tabs
                  .map((t) => ({ t, title: tabDisplayTitle(t) }))
                  .filter(({ title }) => !f || title.toLowerCase().includes(f));
                if (matches.length === 0) {
                  return <div style={{ padding: "8px 12px", color: "var(--text-faint)", fontSize: 12 }}>No matching tabs</div>;
                }
                return matches.map(({ t, title }) => (
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
                    {/* Fixed-width icon column: scratch tabs draw no icon (#881),
                        and rows still have to line up. */}
                    <span className="qtab-panel-icon">{tabIcon(t)}</span>
                    {/* Tooltip with the full title/path, shown only when the row
                        is truncated. overlayStyle lifts the portal above the panel
                        (z-index 9999), same reason the context menu needs it. (#829) */}
                    <OverflowTooltip fullText={tabFullLabel(t, title)} overlayStyle={{ zIndex: 10000 }}>
                      <span style={{ fontStyle: t.preview ? "italic" : undefined }}>{tabPrefix(t)}{title}</span>
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
