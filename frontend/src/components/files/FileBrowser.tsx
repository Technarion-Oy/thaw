// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useLayoutEffect, useRef, useMemo, useCallback } from "react";
import { createPortal } from "react-dom";
import { Tree, Typography, Spin, Button, Input, Switch, Tooltip, Dropdown, App as AntApp } from "antd";
import type { MenuProps } from "antd";
import {
  FolderOutlined,
  FolderOpenOutlined,
  FolderAddOutlined,
  FileOutlined,
  FileAddOutlined,
  ReloadOutlined,
  SearchOutlined,
  DiffOutlined,
  DeleteOutlined,
  EditOutlined,
  CopyOutlined,
  SnippetsOutlined,
  FolderViewOutlined,
  CaretRightFilled,
  CaretDownFilled,
  PlusOutlined,
  MinusOutlined,
  UndoOutlined,
  BranchesOutlined,
  ScissorOutlined,
  BlockOutlined,
  ExportOutlined,
} from "@ant-design/icons";
import type { DataNode, EventDataNode } from "antd/es/tree";
import type { Key } from "rc-tree/lib/interface";
import {
  ListDirectory,
  ReadFile,
  SearchFiles,
  RevealInFinder,
  DeleteFile,
  DeleteDirectory,
  RenameFile,
  CopyFile,
  CreateDirectory,
  CreateFile,
  DuplicateFile,
  GitGetHeadFileContent,
} from "../../../wailsjs/go/app/App";
import { ClipboardSetText, EventsOn } from "../../../wailsjs/runtime/runtime";
import { useGitStore } from "../../store/gitStore";
import { sigilColor, deriveNewAndPartial } from "../git/gitStatusUtil";
import { useQueryStore } from "../../store/queryStore";
import { openFileInTab } from "../../utils/openFileInTab";
import { useDiffStore } from "../../store/diffStore";
import { getPlatformOS, getCachedPlatformOS, revealLabel } from "./platformUtil";
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import { useEditorTabPrefsStore } from "../../store/editorTabPrefsStore";
import {
  type NewItemKind,
  newItemKey,
  isNewItemKey,
  insertSorted,
  addChild,
  findNode,
  childrenOf,
  insertPlaceholder,
  finalNewName,
  validateNewName,
  validateRenameName,
  activeEditSession,
  type InlineEditSession,
} from "./fileTreeUtils";
import type { filesystem } from "../../../wailsjs/go/models";

type FileEntry    = filesystem.FileEntry;
type SearchMatch  = filesystem.SearchMatch;

const { Text } = Typography;
const CLR_SECONDARY = "var(--text-muted)";
/** Upper bound for the inline-validation box (~two wrapped lines) — only used to
 *  decide whether it still fits below the field. See InlineNameInput. */
const ERROR_BOX_MAX_H = 44;

/** Extract the directory portion of a path, handling both / and \ separators. */
function pathDir(p: string): string {
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  if (i < 0) return ".";
  // i == 0 means root separator (e.g. "/filename") — preserve the separator.
  return i === 0 ? p.substring(0, 1) : p.substring(0, i);
}

/**
 * Normalize a directory for self-change suppression so a canonical, symlink-
 * resolved path (what macOS save/open dialogs return — e.g. `/private/tmp/…`) and
 * the watcher's pre-resolution event path (e.g. `/tmp/…`) collapse to one key.
 * Applied symmetrically to the stored key and the `fs:changed` lookup.
 * ponytail: covers only macOS's auto-symlinked roots (`/private/{tmp,var,etc}`),
 * the realistic case; an arbitrary user symlink still costs one redundant
 * ListDirectory — not worth a backend round-trip to resolve.
 */
function suppressKey(dir: string): string {
  return dir.replace(/^\/private(?=\/(?:tmp|var|etc)(?:\/|$))/, "");
}

/** Detect the path separator used in a path (backslash on Windows, forward slash otherwise). */
function pathSep(p: string): string {
  return p.includes("\\") ? "\\" : "/";
}

/** Extract the filename from a path, handling both / and \ separators.
 *  Trailing separators are stripped first so "/projects/" yields "projects". */
function pathBase(p: string): string {
  p = p.replace(/[/\\]+$/, "");
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return i >= 0 ? p.substring(i + 1) : p;
}

/** Build tree nodes from a directory listing.
 *  Tolerates null/undefined: a nil Go slice crosses the Wails bridge as JSON
 *  `null`, so an empty directory used to arrive here as `null` (issue #875).
 *  The backend now always returns `[]`; this stays as belt-and-braces, matching
 *  the `?? []` treatment other bridge results get (search matches, recentDirs). */
function entriesToNodes(entries: FileEntry[] | null | undefined): DataNode[] {
  return (entries ?? []).map((e) => ({
    key:    e.path,
    title:  e.name,
    icon:   (props: { expanded?: boolean }) =>
      e.isDir
        ? (props.expanded ? <FolderOpenOutlined /> : <FolderOutlined />)
        : <FileOutlined style={{ color: CLR_SECONDARY }} />,
    isLeaf: !e.isDir,
  }));
}

/** Merge fresh entries into existing ones, preserving children of nodes that
 *  still exist so expanded subtrees aren't lost. Works for both root-level
 *  and subdirectory refreshes. */
function mergeNodes(prev: DataNode[], fresh: DataNode[]): DataNode[] {
  const oldByKey = new Map(prev.map((n) => [String(n.key), n]));
  return fresh.map((f) => {
    const existing = oldByKey.get(String(f.key));
    // Keep expanded children only if the fresh node is still a directory.
    // If a directory was replaced by a file with the same name, drop the stale children.
    return existing?.children && !f.isLeaf ? { ...f, children: existing.children } : f;
  });
}

function updateNode(nodes: DataNode[], targetKey: string, children: DataNode[], merge?: boolean): DataNode[] {
  return nodes.map((node) => {
    if (node.key === targetKey) {
      const merged = merge && node.children ? mergeNodes(node.children, children) : children;
      return { ...node, children: merged };
    }
    if ((node as any).children) {
      return { ...node, children: updateNode((node as any).children, targetKey, children, merge) };
    }
    return node;
  });
}

/** Create a DataNode for a new file or directory. */
function makeNode(path: string, name: string, isDir: boolean): DataNode {
  return {
    key: path,
    title: name,
    icon: (props: { expanded?: boolean }) =>
      isDir
        ? (props.expanded ? <FolderOpenOutlined /> : <FolderOutlined />)
        : <FileOutlined style={{ color: CLR_SECONDARY }} />,
    isLeaf: !isDir,
  };
}

/** Remove a node by key from the tree. */
function removeNode(nodes: DataNode[], key: string): DataNode[] {
  return nodes
    .filter((n) => n.key !== key)
    .map((n) =>
      n.children ? { ...n, children: removeNode(n.children, key) } : n
    );
}

/** Rename a node (update key + title) and recursively re-key all descendants. */
function renameTreeNode(
  nodes: DataNode[],
  oldKey: string,
  newKey: string,
  newTitle: string,
): DataNode[] {
  return nodes.map((n) => {
    if (n.key === oldKey) {
      return {
        ...n,
        key: newKey,
        title: newTitle,
        children: n.children ? reKeyChildren(n.children, String(oldKey), newKey) : undefined,
      };
    }
    return n.children
      ? { ...n, children: renameTreeNode(n.children, oldKey, newKey, newTitle) }
      : n;
  });
}

/** Recursively update descendant keys when a parent path changes. */
function reKeyChildren(nodes: DataNode[], oldPrefix: string, newPrefix: string): DataNode[] {
  return nodes.map((n) => ({
    ...n,
    key: newPrefix + String(n.key).substring(oldPrefix.length),
    children: n.children ? reKeyChildren(n.children, oldPrefix, newPrefix) : undefined,
  }));
}

// Returns a context window around the match so long lines display usefully.
function getSnippet(
  line: string,
  start: number,
  end: number,
  ctx = 50,
): { before: string; match: string; after: string; ellipsisBefore: boolean; ellipsisAfter: boolean } {
  const snippetStart = Math.max(0, start - ctx);
  const snippetEnd   = Math.min(line.length, end + ctx);
  return {
    before:         line.slice(snippetStart, start),
    match:          line.slice(start, end),
    after:          line.slice(end, snippetEnd),
    ellipsisBefore: snippetStart > 0,
    ellipsisAfter:  snippetEnd < line.length,
  };
}

function groupByPath(matches: SearchMatch[]): Map<string, SearchMatch[]> {
  const map = new Map<string, SearchMatch[]>();
  for (const m of matches) {
    if (!map.has(m.path)) map.set(m.path, []);
    map.get(m.path)!.push(m);
  }
  return map;
}

export default function FileBrowser() {
  const { modal, message } = AntApp.useApp();
  const exportDir    = useGitStore((s) => s.exportDir);
  const gitStatus    = useGitStore((s) => s.status);
  const stageFile    = useGitStore((s) => s.stageFile);
  const unstageFile  = useGitStore((s) => s.unstageFile);
  const discardFile  = useGitStore((s) => s.discardFile);
  const resetHard    = useGitStore((s) => s.resetHard);
  const openGitOps   = useGitStore((s) => s.openGitOps);
  const pickExportDir = useGitStore((s) => s.pickExportDir);
  const openFolder    = useGitStore((s) => s.openFolder);
  const clearRecentDirs = useGitStore((s) => s.clearRecentDirs);
  const openInNewWindow = useGitStore((s) => s.openInNewWindow);
  const recentDirs    = useGitStore((s) => s.recentDirs);
  const refreshGitStatus = useGitStore((s) => s.refreshStatus);
  const loadGitConfig = useGitStore((s) => s.loadConfig);
  const gitConfigLoaded = useGitStore((s) => s.configLoaded);
  const currentFile = useQueryStore((s) => s.currentFile);
  const updateTabPath  = useQueryStore((s) => s.updateTabPath);
  const orphanTab      = useQueryStore((s) => s.orphanFileTab);

  // ── Git status overlay ───────────────────────────────────────────────────────
  // Git status paths are repo-relative ("MYDB/PUBLIC/T.sql"); tree node keys are
  // absolute OS paths the explorer built by joining the export dir with each name,
  // so `relOf` recovers a node's repo-relative path by stripping the export-dir
  // prefix (exact — no suffix guessing, which would false-match files that merely
  // share a basename). The uncapped `changedPaths` map drives coloring so the whole
  // tree is covered even in huge change sets; the capped staged/unstaged lists
  // drive the precise Stage/Unstage context menu.
  const gitOverlay = useMemo(() => {
    const byRel       = new Map<string, string>(); // file rel → status letter
    const dirLetter   = new Map<string, string>(); // dir rel → dominant letter
    const stagedRel   = new Set<string>();
    const unstagedRel = new Set<string>();
    // Discard-prompt sets (new-file / partially-staged) from the shared helper, so
    // the classification has a single home (also used by ChangesView).
    const { newFilesRel, partiallyStagedRel: partialRel } = deriveNewAndPartial(gitStatus?.changedPaths);

    // Folder color = the most significant change beneath it. A/U are both "new"
    // (green), so a folder of only-new files stays green rather than reading as
    // modified. Higher rank wins.
    const RANK: Record<string, number> = { M: 5, R: 5, C: 5, D: 4, A: 2, U: 1 };
    const bumpDir = (dir: string, letter: string) => {
      const cur = dirLetter.get(dir);
      if (cur === undefined || (RANK[letter] ?? 0) > (RANK[cur] ?? 0)) dirLetter.set(dir, letter);
    };
    const addDirs = (rel: string, letter: string) => {
      let i = rel.lastIndexOf("/");
      while (i > 0) { bumpDir(rel.slice(0, i), letter); i = rel.lastIndexOf("/", i - 1); }
    };

    if (gitStatus) {
      // Uncapped: drives the tree coloring for every changed file, including
      // beyond the 500-cap arrays.
      for (const [p, cf] of Object.entries(gitStatus.changedPaths ?? {})) {
        const rel = p.replace(/\\/g, "/");
        byRel.set(rel, cf.status);
        addDirs(rel, cf.status);
      }
      for (const fc of (gitStatus.staged   ?? [])) stagedRel.add(fc.path.replace(/\\/g, "/"));
      for (const fc of (gitStatus.unstaged ?? [])) unstagedRel.add(fc.path.replace(/\\/g, "/"));
    }

    // Exact repo-relative path of a tree node, or null when it's outside the repo.
    const base = exportDir.replace(/[/\\]+$/, "").replace(/\\/g, "/");
    const relOf = (nodeKey: string): string | null => {
      const a = nodeKey.replace(/\\/g, "/");
      if (a === base) return "";
      if (base && a.startsWith(base + "/")) return a.slice(base.length + 1);
      return null;
    };

    return { byRel, dirLetter, stagedRel, unstagedRel, newFilesRel, partialRel, relOf };
  }, [gitStatus, exportDir]);

  // ── File tree state ────────────────────────────────────────────────────────
  const [treeData,    setTreeData]    = useState<DataNode[]>([]);
  const [loadedKeys,  setLoadedKeys]  = useState<Key[]>([]);
  // Expansion is CONTROLLED (rather than rc-tree's default uncontrolled, lazy
  // expansion) so `startNewItem` can programmatically open the directory the
  // inline "new item" placeholder is being created in. rc-tree only fires
  // `loadData` from a user-driven expand, so `startNewItem` also lists the
  // directory itself when it isn't in `loadedKeys` yet.
  const [expandedKeys, setExpandedKeys] = useState<Key[]>([]);
  // Multi-selection: the set of selected node keys (drives highlight + bulk ops).
  // `anchorKey` is the pivot for Shift+click range selection.
  const [selKeys,     setSelKeys]     = useState<string[]>([]);
  const [anchorKey,   setAnchorKey]   = useState<string | null>(null);
  const [loading,     setLoading]     = useState(false);
  const [loaded,      setLoaded]      = useState(false);
  // Why the root listing failed (unreadable directory, deleted while open, …).
  // Set on a failed loadRoot/refresh so the auto-load effect below stops
  // retrying — without it a permanently failing root spun an endless
  // ListDirectory loop (issue #875) and the panel stayed blank. Cleared on a
  // successful load, on a workspace switch, and by the inline Retry button.
  const [rootError,   setRootError]   = useState<string | null>(null);
  const treeWrapRef = useRef<HTMLDivElement>(null);

  // ── Internal file clipboard (cut/copy/paste) ────────────────────────────────
  // Frontend-only — never touches the OS text clipboard. Cut is one-shot (cleared
  // after a paste); copy persists. ponytail: local state, not a store — only this
  // component reads it; promote to a slice if another panel ever needs it.
  const [clipboard, setClipboard] = useState<{ mode: "cut" | "copy"; paths: string[] } | null>(null);

  // ── Search state ───────────────────────────────────────────────────────────
  const [searchOpen,    setSearchOpen]    = useState(false);
  const [searchQuery,   setSearchQuery]   = useState("");
  const [useRegex,      setUseRegex]      = useState(false);
  const [searchResults, setSearchResults] = useState<SearchMatch[]>([]);
  const [searching,     setSearching]     = useState(false);
  const [searchError,   setSearchError]   = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ── Context menu ─────────────────────────────────────────────────────────
  const [fileCtxMenu, setFileCtxMenu] = useState<{ x: number; y: number; path: string; name: string; isDir: boolean; isRoot?: boolean } | null>(null);
  const fileCtxRef = useRef<HTMLDivElement>(null);

  // ── Inline editors (VS Code–style editing in the tree) ─────────────────────
  // Rename and creation are the same machine over different state. Each open
  // editor is a *session* (see InlineEditSession): the id is stamped into the
  // state the editor owns, so a handler that captured that state — an awaited
  // IPC resuming, a stray blur from an unmounted input — can prove it is still
  // the live editor before touching anything. A plain enum ref couldn't: opening
  // the next editor resets it, re-arming every stale closure from the last one.
  // At most one session is live at a time; starting either cancels the other.
  const sessionCounterRef = useRef(0);
  const newSession = (): InlineEditSession => ({ id: ++sessionCounterRef.current, phase: "editing" });

  const [pendingRename, setPendingRename] = useState<{ id: number; path: string; value: string } | null>(null);
  const renameSessionRef = useRef<InlineEditSession | null>(null);

  // Creation, VS Code style: an editable placeholder row is injected into the
  // tree under `parent` instead of opening a modal. It lives in the render-time
  // tree only (see `treeForRender`), so `treeData` stays a pure mirror of the
  // filesystem and refreshes/watcher events can't disturb the edit in progress.
  const [pendingCreate, setPendingCreate] = useState<{ id: number; kind: NewItemKind; parent: string; value: string } | null>(null);
  const createSessionRef = useRef<InlineEditSession | null>(null);

  // ── Collapse state ──────────────────────────────────────────────────────────
  const [expanded, setExpanded] = useState(false);

  // The workspace root is not itself a tree node (treeData holds its children
  // directly), so root-level inserts and sibling lookups pass `null` as the
  // parent key. `exportDir` may carry a trailing separator — strip it once here.
  const rootDir = exportDir.replace(/[/\\]+$/, "");

  // ── Platform detection for labels ─────────────────────────────────────────
  const [platformOS, setPlatformOS] = useState<string | null>(getCachedPlatformOS());
  useEffect(() => { getPlatformOS().then(setPlatformOS); }, []);
  const revealText = revealLabel(platformOS);

  const pendingDiff   = useDiffStore((s) => s.pending);
  const selectForComp = useDiffStore((s) => s.selectForComparison);
  const compareWith   = useDiffStore((s) => s.compareWith);

  const fileWatcherEnabled = useFeatureFlagsStore((s) => s.flags.fileWatcher);
  const gitEnabled         = useFeatureFlagsStore((s) => s.flags.gitIntegration);
  // VS Code–style preview tabs: single-click / search-result opens go to the
  // reusable preview tab; double-click promotes to permanent. Toggle in Editor Prefs.
  const previewTabsEnabled = useEditorTabPrefsStore((s) => s.previewTabsEnabled);

  // The standalone Git panel was folded into this panel, so the Files panel now
  // owns loading the git/export config on first mount (idempotent).
  useEffect(() => {
    if (!gitConfigLoaded) loadGitConfig();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gitConfigLoaded]);

  // Keep git status fresh for whatever directory the explorer is showing, so the
  // tree colors don't depend on some other surface having refreshed first.
  useEffect(() => {
    if (exportDir) refreshGitStatus(true); // background: don't surface status errors
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exportDir]);

  const gitRepo       = gitEnabled && !!gitStatus?.isRepo;
  // Empty branch = repo with no commits yet (Head() failed); don't imply "main".
  const gitBranch     = gitStatus?.branch || "(no commits)";
  const gitAhead      = gitStatus?.ahead ?? 0;
  const gitChanged    = gitStatus?.totalChanged ?? 0;
  const gitStagedTot  = gitStatus?.stagedTotal ?? 0;

  // ── Self-change suppression ────────────────────────────────────────────────
  // Tracks directories modified by in-app operations so watcher events don't
  // cause a redundant (flickering) refresh. Entries are auto-cleared after 500ms.
  const selfChangedDirs = useRef(new Set<string>());
  const selfChangeTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  const markSelfChanged = (dir: string) => {
    const key = suppressKey(dir); // normalize so canonical dialog paths match evt.dir
    selfChangedDirs.current.add(key);
    const prev = selfChangeTimers.current.get(key);
    if (prev) clearTimeout(prev);
    selfChangeTimers.current.set(key, setTimeout(() => {
      selfChangedDirs.current.delete(key);
      selfChangeTimers.current.delete(key);
    }, 500));
  };

  // Clear pending self-change suppression timers on unmount.
  useEffect(() => {
    return () => {
      for (const t of selfChangeTimers.current.values()) clearTimeout(t);
    };
  }, []);

  // Stable refs so effects can read current values without re-registering.
  const loadedKeysRef = useRef(loadedKeys);
  loadedKeysRef.current = loadedKeys;
  const selKeysRef = useRef(selKeys);
  selKeysRef.current = selKeys;

  // Coalesce the redundant file opens a click/double-click gesture would otherwise
  // fire. rc-tree runs onSelect on *every* click (including both clicks of a
  // double-click), and a double-click adds a native dblclick on top — so one
  // "open and pin" gesture could fire up to three concurrent ReadFile round-trips for
  // one file. `openingPathsRef` skips a same-path open already in flight;
  // `pendingPromoteRef` lets a double-click that lands before the open resolves
  // request promotion (via onSelect, once the tab exists) instead of firing its own
  // extra read.
  const openingPathsRef   = useRef<Set<string>>(new Set());
  const pendingPromoteRef = useRef<Set<string>>(new Set());

  // Debounced git-status refresh so the tree's status colors update live (on
  // save, external edits, file ops) without a manual refresh, while coalescing
  // bursts so we don't run the (potentially expensive) status scan repeatedly.
  const gitRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const scheduleGitRefresh = useCallback(() => {
    if (!useFeatureFlagsStore.getState().flags.gitIntegration) return; // respect the feature flag
    if (gitRefreshTimerRef.current) clearTimeout(gitRefreshTimerRef.current);
    gitRefreshTimerRef.current = setTimeout(() => { useGitStore.getState().refreshStatus(true); }, 400);
  }, []);
  useEffect(() => () => { if (gitRefreshTimerRef.current) clearTimeout(gitRefreshTimerRef.current); }, []);

  // Refresh git colors when a file is saved in the editor (watcher-independent).
  // Also mark the saved file's directory as self-changed so the watcher's echo of
  // our own write (arriving ~200 ms later) doesn't trigger a redundant tree re-list.
  useEffect(() => {
    const handler = (e: Event) => {
      scheduleGitRefresh();
      const path = (e as CustomEvent<{ path?: string }>).detail?.path;
      if (path) markSelfChanged(pathDir(path));
    };
    window.addEventListener("thaw:file-saved", handler);
    return () => window.removeEventListener("thaw:file-saved", handler);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- markSelfChanged only closes over stable refs
  }, [scheduleGitRefresh]);

  // Close file context menu on outside click or Escape key
  useEffect(() => {
    if (!fileCtxMenu) return;
    const close = () => setFileCtxMenu(null);
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") close(); };
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [fileCtxMenu]);

  // Clamp file context menu inside the viewport and focus the first item (runs before browser paint — no flash)
  useLayoutEffect(() => {
    if (!fileCtxMenu || !fileCtxRef.current) return;
    const el = fileCtxRef.current;
    const { width, height } = el.getBoundingClientRect();
    const pad = 8;
    const left = Math.max(pad, Math.min(fileCtxMenu.x, window.innerWidth  - width  - pad));
    const top  = Math.max(pad, Math.min(fileCtxMenu.y, window.innerHeight - height - pad));
    el.style.left = `${left}px`;
    el.style.top  = `${top}px`;
    // Auto-focus the first menu item for keyboard accessibility.
    const firstItem = el.querySelector<HTMLElement>("[role='menuitem']");
    firstItem?.focus();
  }, [fileCtxMenu]);

  // Reset tree when the working directory changes
  useEffect(() => {
    setLoaded(false);
    setRootError(null);
    setTreeData([]);
    setLoadedKeys([]);
    setExpandedKeys([]);
    setSelKeys([]);
    setAnchorKey(null);
    setClipboard(null);
    // Close any open inline editor and retire its session, so an IPC still in
    // flight against the old workspace can't come back and touch the new one.
    cancelCreate();
    cancelRename();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- cancel* only touch refs/setState
  }, [exportDir]);

  // ── File system watcher lifecycle ──────────────────────────────────────────
  // The watcher's Start/Stop lifecycle lives in QueryPage (always mounted), so
  // hiding the sidebar via ⌘B — which unmounts FileBrowser — doesn't stop it.
  // FileBrowser only consumes the resulting fs:changed events for the tree.

  // On re-expand, refresh the root to pick up changes that occurred while
  // collapsed. loadRoot() handles the first-ever load (and no-ops thereafter),
  // so this fires only on a genuine collapse→expand transition — a prev-expanded
  // ref gates it so the loaded:false→true tick right after the initial load
  // doesn't trigger a redundant second ListDirectory.
  const prevExpandedRef = useRef(expanded);
  useEffect(() => {
    const justExpanded = expanded && !prevExpandedRef.current;
    prevExpandedRef.current = expanded;
    if (!justExpanded || !exportDir || !loaded) return;
    ListDirectory(exportDir)
      .then((entries) => {
        // Build the nodes here, not inside the updater: React runs an updater
        // during the render phase, where a throw tears down the whole React
        // tree instead of landing in .catch (issue #875).
        const fresh = entriesToNodes(entries);
        setTreeData((prev) => mergeNodes(prev, fresh));
      })
      .catch(() => {});
  }, [exportDir, expanded, loaded]);

  // ── File system change listener ────────────────────────────────────────────
  useEffect(() => {
    if (!exportDir || !fileWatcherEnabled) return;
    const off = EventsOn("fs:changed", (evt: { dir: string }) => {
      // Any disk change may alter git status — refresh colors even for the app's
      // own mutations (which suppress the tree update below to avoid flicker).
      scheduleGitRefresh();
      if (selfChangedDirs.current.has(suppressKey(evt.dir))) return;

      // After refreshing a directory, prune key entries that reference children
      // which no longer exist (prevents unbounded stale-key growth). Both key
      // sets are pruned: expansion is controlled, so a stale expandedKeys entry
      // would render a same-named directory that reappears later (branch switch,
      // restore) as already-expanded but unloaded — and programmatic expansion
      // doesn't trigger loadData, so it would look empty until manually toggled.
      const pruneStaleKeys = (freshKeys: Set<string>) => {
        const keep = (k: Key) => {
          const ks = String(k);
          const parent = ks.substring(0, ks.lastIndexOf("/")) || ks.substring(0, ks.lastIndexOf("\\"));
          // Only prune keys whose parent is the refreshed directory.
          if (parent !== evt.dir) return true;
          return freshKeys.has(ks);
        };
        setLoadedKeys((prev) => prev.filter(keep));
        setExpandedKeys((prev) => prev.filter(keep));
      };

      if (evt.dir === exportDir) {
        // Root directory changed — merge new entries into existing tree
        // so expanded subtrees (children) are preserved.
        ListDirectory(exportDir)
          .then((entries) => {
            const list = entries ?? [];
            const fresh = entriesToNodes(list);
            setTreeData((prev) => mergeNodes(prev, fresh));
            pruneStaleKeys(new Set(list.map((e) => e.path)));
          })
          .catch(() => {});
        return;
      }
      // Only refresh directories that are already expanded (in loadedKeys).
      if (!loadedKeysRef.current.some((k) => String(k) === evt.dir)) return;
      ListDirectory(evt.dir)
        .then((entries) => {
          const list = entries ?? [];
          // Nodes built outside the updater — see the collapse→expand refresh above.
          const fresh = entriesToNodes(list);
          setTreeData((prev) => updateNode(prev, evt.dir, fresh, true));
          pruneStaleKeys(new Set(list.map((e) => e.path)));
        })
        .catch(() => {});
    });
    return off;
  }, [exportDir, fileWatcherEnabled, scheduleGitRefresh]);

  // Keep selection in sync with the active tab (tab switches / opens). Skip while a
  // multi-selection is active so an unrelated tab change can't silently collapse a
  // bulk selection the user is about to act on.
  useEffect(() => {
    if (selKeysRef.current.length > 1) return;
    setSelKeys(currentFile ? [String(currentFile)] : []);
    setAnchorKey(currentFile ? String(currentFile) : null);
  }, [currentFile]);

  // Debounced search
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (!searchQuery.trim() || !exportDir || !searchOpen) {
      setSearchResults([]);
      setSearchError(null);
      return;
    }

    debounceRef.current = setTimeout(async () => {
      setSearching(true);
      setSearchError(null);
      try {
        const matches = await SearchFiles(exportDir, searchQuery, useRegex);
        setSearchResults(matches ?? []);
      } catch (e) {
        setSearchError(String(e));
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [searchQuery, useRegex, exportDir, searchOpen]);

  const loadRoot = async () => {
    if (!exportDir || loading || loaded) return;
    setLoading(true);
    setRootError(null);
    try {
      const entries = await ListDirectory(exportDir);
      setTreeData(entriesToNodes(entries));
      setLoaded(true);
    } catch (e) {
      // Non-fatal, but remember it: `loaded` stays false, so without this flag
      // the auto-load effect below would call loadRoot() again on every render.
      setRootError(String(e));
    } finally {
      setLoading(false);
    }
  };

  // Ensure the root loads whenever the panel is expanded but not yet loaded —
  // covers a workspace switch that happens while already expanded (the reset
  // effect clears `loaded`, and toggleExpanded — the only other caller — doesn't
  // fire in that case, leaving the tree blank with no Reload button). loadRoot()
  // self-guards on loading/loaded, so this never double-lists alongside the
  // toggleExpanded path. `rootError` gates the retry: a root that keeps failing
  // is surfaced with a Retry button instead of being re-listed forever.
  useEffect(() => {
    if (exportDir && expanded && !loaded && !loading && !rootError) loadRoot();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exportDir, expanded, loaded, loading, rootError]);

  const refresh = async () => {
    setFileCtxMenu(null); // dismiss stale context menu
    setLoading(true);
    // Refresh git status alongside the tree so the status colors stay current
    // (silent: a status-fetch failure shouldn't pop an error from the file tree).
    refreshGitStatus(true);
    try {
      // Re-fetch the root and every currently-loaded (expanded) directory in
      // parallel, then merge — this picks up external changes while PRESERVING the
      // expanded subtree. Replacing treeData with root-only nodes (the old
      // behavior) dropped every loaded child while `expandedKeys` still named
      // them: folders collapsed and could not be reopened.
      const loaded = loadedKeysRef.current.map(String);
      const [rootEntries, ...childResults] = await Promise.all([
        ListDirectory(exportDir),
        // `entries: null` means the listing FAILED (the stale subtree is kept
        // below); a directory that listed fine but is empty must come through
        // as [] so its stale children are actually cleared.
        ...loaded.map(async (k) => {
          try { return { key: k, entries: (await ListDirectory(k)) ?? [] }; }
          catch { return { key: k, entries: null as FileEntry[] | null }; }
        }),
      ]);
      // Nodes are built before the updater runs — an updater throws during the
      // render phase, which would unmount the React root (issue #875).
      const rootNodes = entriesToNodes(rootEntries);
      const childNodes = childResults
        .filter((r) => r.entries !== null)
        .map((r) => ({ key: r.key, nodes: entriesToNodes(r.entries) }));
      setTreeData((prev) => {
        let tree = mergeNodes(prev, rootNodes);
        for (const c of childNodes) tree = updateNode(tree, c.key, c.nodes, true);
        return tree;
      });
      setLoaded(true);
      setRootError(null);
    } catch (e) {
      // Non-fatal — see loadRoot: record it so the auto-load effect doesn't loop.
      setRootError(String(e));
    } finally {
      setLoading(false);
    }
  };

  // Refresh the tree automatically when an export finishes
  useEffect(() => {
    const handler = () => { if (loaded) refresh(); };
    window.addEventListener("thaw:export-complete", handler);
    return () => window.removeEventListener("thaw:export-complete", handler);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded, exportDir]);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if ((node as any).children) return;
    const path = String(node.key);
    try {
      const entries = await ListDirectory(path);
      // Nodes built here, outside the updater — see refresh(). An empty folder
      // (the #875 repro: create folder → expand) lands here as [].
      const children = entriesToNodes(entries);
      setTreeData((prev) => updateNode(prev, path, children));
    } catch {
      // non-fatal
    }
  };

  // Keys of currently-rendered tree nodes in visual (top-to-bottom) order. Read
  // from the DOM (each title carries a data-fbkey attribute, set in titleRender)
  // rather than walked from expandedKeys + treeData (as the object-store sidebar's
  // flattenVisibleNodes does): the DOM is the one source that already reflects
  // expand/collapse, and it skips the inline-creation placeholder for free — that
  // row deliberately carries no data-fbkey.
  // ponytail: correct as long as the tree isn't virtualized. If the rc-tree
  // `height` prop is ever set, only viewport rows render, so an off-screen anchor
  // would drop from the range — switch to an expandedKeys + treeData walk then.
  const visibleKeysInOrder = (): string[] => {
    const root = treeWrapRef.current;
    if (!root) return [];
    return Array.from(root.querySelectorAll<HTMLElement>("[data-fbkey]"))
      .map((el) => el.dataset.fbkey || "")
      .filter(Boolean);
  };

  const isDirKey = (key: string) => findNode(treeData, key)?.isLeaf === false;

  const onSelect = async (_keys: Key[], info: { node: DataNode; nativeEvent: MouseEvent }) => {
    const node = info.node;
    const path = String(node.key);
    if (isNewItemKey(path)) return; // the inline creation placeholder isn't a real node
    const isDir = (node as any).isLeaf === false;
    const ne = info.nativeEvent;

    // Cmd/Ctrl+click — toggle this node in the selection (don't open).
    if (ne && (ne.metaKey || ne.ctrlKey)) {
      setAnchorKey(path);
      setSelKeys((prev) => (prev.includes(path) ? prev.filter((k) => k !== path) : [...prev, path]));
      return;
    }
    // Shift+click — select the range between the anchor and this node (don't open).
    if (ne && ne.shiftKey) {
      const flat = visibleKeysInOrder();
      const ai = anchorKey ? flat.indexOf(anchorKey) : -1;
      const bi = flat.indexOf(path);
      if (ai >= 0 && bi >= 0) {
        const [lo, hi] = ai < bi ? [ai, bi] : [bi, ai];
        setSelKeys(flat.slice(lo, hi + 1));
      } else {
        // No anchor, or it scrolled out of view — fall back to selecting just this
        // node. Return regardless so a Shift+click never opens the file.
        setAnchorKey(path);
        setSelKeys([path]);
      }
      return;
    }
    // Plain click — single selection; open files, leave folders unexpanded (caret expands).
    setAnchorKey(path);
    setSelKeys([path]);
    if (isDir) return;
    // A same-path open already in flight (e.g. the two clicks of a double-click)
    // shouldn't fire a second read — the first open already covers this file.
    if (openingPathsRef.current.has(path)) return;
    openingPathsRef.current.add(path);
    // Single click opens in the reusable preview tab (when enabled); a double-click
    // promotes it to permanent (see onTreeDoubleClick).
    let err: string | null = null;
    try {
      err = await openFileInTab(path, previewTabsEnabled);
    } finally {
      openingPathsRef.current.delete(path);
    }
    if (err) {
      message.error(`Could not open file: ${err}`);
      setSelKeys([]);
      pendingPromoteRef.current.delete(path); // open failed — drop any pending promote
      return;
    }
    // If a double-click landed while this open was in flight, honor its promote intent
    // now that the tab exists (instead of it having fired a third redundant read).
    if (pendingPromoteRef.current.delete(path)) {
      const tab = useQueryStore.getState().tabs.find((t) => t.path === path);
      if (tab) useQueryStore.getState().promoteTab(tab.id);
    }
  };

  // Double-click a file in the tree → promote it to a permanent tab (VS Code
  // behavior). rc-tree has no double-click prop, so we bind a native handler on the
  // tree wrapper and recover the node key from the `data-fbkey` attribute set in
  // titleRender. The preceding click already opened (or is opening) the file as a
  // preview, so: promote the tab if it already exists, else record the intent and let
  // onSelect promote it once its in-flight open resolves — avoiding a redundant
  // ReadFile of a file the click is already fetching.
  const onTreeDoubleClick = (e: React.MouseEvent) => {
    // A modifier+click is a selection gesture (Cmd/Ctrl toggle, Shift range), not an
    // open — onSelect returns early for it without opening, so recording a pending
    // promote here would never be consumed and would later silently pin an unrelated
    // plain open of the same file. Only plain double-clicks are open-and-pin.
    if (e.metaKey || e.ctrlKey || e.shiftKey) return;
    const el = (e.target as HTMLElement).closest?.("[data-fbkey]") as HTMLElement | null;
    const key = el?.dataset.fbkey;
    if (!key || isDirKey(key)) return;
    const existing = useQueryStore.getState().tabs.find((t) => t.path === key);
    if (existing) {
      useQueryStore.getState().promoteTab(existing.id);
      return;
    }
    pendingPromoteRef.current.add(key);
  };

  const toggleExpanded = () => {
    const next = !expanded;
    setExpanded(next);
    if (next) loadRoot();
  };

  const toggleSearch = (e: React.MouseEvent) => {
    e.stopPropagation();
    const opening = !searchOpen;
    setSearchOpen(opening);
    if (opening) {
      setExpanded(true);
    } else {
      setSearchQuery("");
      setSearchResults([]);
      setSearchError(null);
    }
  };

  const handleResultClick = async (match: SearchMatch) => {
    const err = await openFileInTab(match.path, previewTabsEnabled);
    if (err) {
      message.error(`Could not open file: ${err}`);
      return;
    }
    setTimeout(() => {
      window.dispatchEvent(
        new CustomEvent("thaw:scroll-to-line", {
          detail: {
            line:       match.lineNumber,
            matchStart: match.matchStart,
            matchEnd:   match.matchEnd,
          },
        })
      );
    }, 50);
  };

  const onRightClick = ({ event, node }: { event: React.MouseEvent; node: DataNode }) => {
    event.preventDefault();
    const path = String(node.key);
    if (isNewItemKey(path)) return; // no context menu on the creation placeholder
    // Nor on the row whose own rename editor is open: the menu's actions (Delete,
    // Rename, Cut, …) all act on a node that is mid-edit. The input itself stops
    // contextmenu propagation, so a right-click *inside* the field still gets the
    // native menu for paste.
    if (pendingRename && path === pendingRename.path) return;
    const name = pathBase(path);
    const isDir = (node as any).isLeaf === false;
    // Right-clicking a node outside the current multi-selection acts on just that
    // node (standard file-manager behavior); right-clicking inside it keeps the set.
    if (!selKeys.includes(path)) {
      setSelKeys([path]);
      setAnchorKey(path); // keep the Shift+range pivot aligned with the new selection
    }
    setFileCtxMenu({ x: event.clientX, y: event.clientY, path, name, isDir });
  };

  // Right-click on the empty area of the panel (not a tree node) opens a minimal
  // root context menu so a folder/file can be created at the workspace root — the
  // one place a node context menu can't reach. Node right-clicks are handled by
  // the tree's onRightClick and are skipped here (they carry a data-fbkey title).
  const onRootContextMenu = (event: React.MouseEvent) => {
    if (!exportDir) return;
    // In search mode the content wrapper hosts the search input and result rows —
    // leave those to the native context menu (right-click paste into the field,
    // etc.) rather than popping a folder-creation menu that makes no sense there.
    if (searchOpen) return;
    // A right-click anywhere on a tree node (title, icon, or indent) is handled by
    // the tree's onRightClick — .ant-tree-treenode covers the whole row, so guard
    // on it rather than the narrower data-fbkey title span. An input/textarea guard
    // preserves the native menu for any editable field inside the wrapper.
    if ((event.target as HTMLElement).closest?.("input, textarea, .ant-tree-treenode")) return;
    event.preventDefault();
    setSelKeys([]);
    setAnchorKey(null);
    setFileCtxMenu({
      x: event.clientX,
      y: event.clientY,
      path: exportDir,
      name: pathBase(exportDir),
      isDir: true,
      isRoot: true,
    });
  };

  // Paths the context-menu bulk actions operate on: the whole selection when the
  // right-clicked node is part of a multi-selection, otherwise just that node.
  const opPaths = (): string[] => {
    if (!fileCtxMenu) return [];
    return selKeys.length > 1 && selKeys.includes(fileCtxMenu.path) ? selKeys : [fileCtxMenu.path];
  };

  // Drop any path whose ancestor directory is also in the set. A Shift+range
  // selection naturally spans a folder and its children; the recursive ops
  // (delete / move / copy) already act on the whole subtree via the folder, so
  // keeping the descendants would double-process them (ENOENT on delete/move,
  // duplicate files on copy). Git staging deliberately does NOT dedup — it
  // operates per file and excludes dirs (see opFilePaths).
  const dropDescendants = (paths: string[]): string[] =>
    paths.filter((p) => !paths.some((o) => o !== p && p.startsWith(o + pathSep(o))));

  // Files-only subset of the operation set — git staging operates per file; passing a
  // directory to `git add` would recursively stage everything under it.
  const opFilePaths = (): string[] => opPaths().filter((p) => !isDirKey(p));

  const selectFileForComparison = () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    selectForComp({ category: "file", label: `FILE: ${name}`, path });
    message.success(`Selected for comparison: ${name}`);
  };

  const compareFileWith = () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    compareWith({ category: "file", label: `FILE: ${name}`, path });
  };

  const handleReveal = () => {
    if (!fileCtxMenu) return;
    RevealInFinder(fileCtxMenu.path).catch((e) => message.error(`Could not reveal: ${String(e)}`));
    setFileCtxMenu(null);
  };

  const handleCopyPath = async () => {
    if (!fileCtxMenu) return;
    const p = fileCtxMenu.path;
    setFileCtxMenu(null);
    try {
      await ClipboardSetText(p);
      message.success("Path copied");
    } catch {
      message.error("Failed to copy path");
    }
  };

  // Copy the path relative to the project root (export dir) — useful for @stage
  // references, dbt refs, etc. Falls back to the absolute path if outside the root.
  const handleCopyRelativePath = async () => {
    if (!fileCtxMenu) return;
    const p = fileCtxMenu.path;
    setFileCtxMenu(null);
    const base = exportDir.replace(/[/\\]+$/, "");
    let rel = p;
    if (base && (p === base || p.startsWith(base + "/") || p.startsWith(base + "\\"))) {
      rel = p === base ? "." : p.slice(base.length + 1);
    }
    try {
      await ClipboardSetText(rel);
      message.success("Relative path copied");
    } catch {
      message.error("Failed to copy path");
    }
  };

  // ── Internal clipboard: cut / copy / paste ─────────────────────────────────
  const handleCut = () => {
    const paths = opPaths();
    setFileCtxMenu(null);
    if (!paths.length) return;
    setClipboard({ mode: "cut", paths });
    message.info(`Cut ${paths.length} item${paths.length > 1 ? "s" : ""}`);
  };

  const handleCopy = () => {
    const paths = opPaths();
    setFileCtxMenu(null);
    if (!paths.length) return;
    setClipboard({ mode: "copy", paths });
    message.success(`Copied ${paths.length} item${paths.length > 1 ? "s" : ""}`);
  };

  // Pick a non-colliding name for `base` against the set of names already present
  // in the target dir, appending _copy, _copy_2, … like the backend DuplicateFile.
  // Synchronous: the caller lists the target dir once and updates `names` as it
  // claims each destination, so pasting N items costs one IPC call, not N.
  const uniqueDstName = (names: Set<string>, base: string): string => {
    if (!names.has(base)) return base;
    const dot = base.lastIndexOf(".");
    const ext = dot > 0 ? base.slice(dot) : "";
    const stem = dot > 0 ? base.slice(0, dot) : base;
    let cand = `${stem}_copy${ext}`;
    if (!names.has(cand)) return cand;
    for (let i = 2; i < 1000; i++) {
      cand = `${stem}_copy_${i}${ext}`;
      if (!names.has(cand)) return cand;
    }
    return `${stem}_copy_${Date.now()}${ext}`;
  };

  // Update open tabs after a file/folder moves so they don't dangle.
  const remapTabsForMove = (oldPath: string, newPath: string, isDir: boolean) => {
    const sep = pathSep(oldPath);
    const prefix = oldPath + sep;
    for (const tab of useQueryStore.getState().tabs) {
      if (tab.path === oldPath) {
        updateTabPath(tab.id, newPath, pathBase(newPath));
      } else if (isDir && tab.path?.startsWith(prefix)) {
        const np = newPath + tab.path.substring(oldPath.length);
        updateTabPath(tab.id, np, pathBase(np));
      }
    }
  };

  // Toolbar paste targets the single selected folder, else the project root.
  // Memoized so the JSX (Tooltip title + onClick) doesn't re-walk the tree each render.
  const toolbarPasteTarget = useMemo(
    () => (selKeys.length === 1 && isDirKey(selKeys[0]) ? selKeys[0] : exportDir),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selKeys, treeData, exportDir],
  );

  const handlePaste = async (rawTarget: string) => {
    setFileCtxMenu(null);
    if (!clipboard) return;
    const { mode } = clipboard;
    // A folder copy/move already carries its whole subtree, so drop any clipboard
    // entry nested under another — otherwise the descendant would be processed
    // twice (a duplicate file at the target on copy, an ENOENT on move).
    const paths = dropDescendants(clipboard.paths);
    // Strip any trailing separator (exportDir may be stored with one) so the
    // same-folder no-op guard below matches pathDir(src), which always strips it.
    const targetDir = rawTarget.replace(/[/\\]+$/, "");
    const sep = pathSep(targetDir);
    const join = (n: string) => `${targetDir}${sep}${n}`;
    // List the target dir once; claim each chosen name in `names` so sequential
    // pastes don't collide (replaces one ListDirectory IPC per item). If the target
    // is gone (e.g. deleted between Cut and Paste), bail with one clear error rather
    // than letting every item fail with a confusing per-file toast.
    let names: Set<string>;
    // `?? []` — an empty target directory is a perfectly valid paste target.
    try { names = new Set(((await ListDirectory(targetDir)) ?? []).map((e) => e.name)); }
    catch { message.error("Paste target is not accessible"); return; }
    const failed: string[] = [];
    const skipped: string[] = []; // cut items already in the target folder (no-op)
    let ok = 0;
    for (const src of paths) {
      // Moving an item into the folder it already lives in is a no-op.
      if (mode === "cut" && pathDir(src) === targetDir) { skipped.push(src); continue; }
      const base = pathBase(src);
      const isDir = isDirKey(src);
      try {
        const dstName = uniqueDstName(names, base);
        const dst = join(dstName);
        if (mode === "cut") {
          try {
            await RenameFile(src, dst); // atomic on the same volume
          } catch {
            // ponytail: cross-volume fallback (copy+delete). Effectively dead on a
            // single-root export dir, but the issue requires it; remove if proven moot.
            await CopyFile(src, dst);
            try {
              if (isDir) await DeleteDirectory(src); else await DeleteFile(src);
            } catch (delErr) {
              // Source delete failed — roll back the copy so a retry doesn't leave
              // (and keep accumulating) orphan duplicates at the destination.
              try { if (isDir) await DeleteDirectory(dst); else await DeleteFile(dst); } catch { /* best effort */ }
              throw delErr;
            }
          }
          remapTabsForMove(src, dst, isDir);
        } else {
          await CopyFile(src, dst);
        }
        names.add(dstName); // claim the name for the remaining items
        ok++;
      } catch (e) {
        failed.push(src);
        message.error(`Paste failed for ${base}: ${String(e)}`);
      }
    }
    markSelfChanged(targetDir);
    if (mode === "cut") {
      for (const src of paths) markSelfChanged(pathDir(src));
      // Keep items that didn't move (failed) or that were a no-op for *this* target
      // (skipped — same folder) so the clipboard stays retriable elsewhere; clearing
      // on ok>0 would silently drop them.
      const keep = [...failed, ...skipped];
      setClipboard(keep.length ? { mode: "cut", paths: keep } : null);
      // All items were already here — say so, otherwise the cut vanishes silently.
      if (!ok && !failed.length && skipped.length) message.info("Already in this folder");
    }
    if (ok) message.success(`Pasted ${ok} item${ok > 1 ? "s" : ""}`);
    refresh();
  };

  // ── Bulk git staging ───────────────────────────────────────────────────────
  // ponytail: loops the per-file store actions, so each awaits a status refresh —
  // fine for the handful of files a user selects; add a batch IPC if it ever lags.
  // Each store action resets gitStore.error at its start, so a later iteration
  // would erase an earlier failure — capture error per file, right after each call.
  const runBulkGit = async (
    paths: string[],
    action: (p: string) => Promise<void>,
    failVerb: string,  // imperative, e.g. "Stage"
    pastVerb: string,  // past tense, e.g. "Staged"
  ) => {
    const failed: string[] = [];
    for (const p of paths) {
      await action(p);
      if (useGitStore.getState().error) failed.push(pathBase(p));
    }
    if (failed.length) message.error(`${failVerb} failed: ${failed.join(", ")}`);
    else message.success(`${pastVerb} ${paths.length} file${paths.length > 1 ? "s" : ""}`);
  };

  const handleBulkStage = async () => {
    const paths = opFilePaths();
    setFileCtxMenu(null);
    if (!paths.length || gitBusy()) return;
    await runBulkGit(paths, stageFile, "Stage", "Staged");
  };

  const handleBulkUnstage = async () => {
    const paths = opFilePaths();
    setFileCtxMenu(null);
    if (!paths.length || gitBusy()) return;
    await runBulkGit(paths, unstageFile, "Unstage", "Unstaged");
  };

  const handleBulkDiscard = () => {
    const paths = opFilePaths();
    setFileCtxMenu(null);
    if (!paths.length || gitBusy()) return;
    // Name any never-committed files — discard permanently deletes those (they have
    // no HEAD to revert to), so they deserve an explicit callout, not a generic line.
    const newNames = paths
      .filter((p) => { const rel = gitOverlay.relOf(p); return rel != null && gitOverlay.newFilesRel.has(rel); })
      .map(pathBase);
    modal.confirm({
      title: `Discard changes to ${paths.length} file${paths.length > 1 ? "s" : ""}?`,
      content: newNames.length
        ? `Reverts each file to its last committed state. ${newNames.length} never-committed file${newNames.length > 1 ? "s" : ""} will be permanently deleted (${newNames.join(", ")}). This cannot be undone.`
        : "Reverts each file to its last committed state. This cannot be undone.",
      okText: "Discard",
      okButtonProps: { danger: true },
      // `done` persists across retries so a re-click skips files already discarded —
      // re-discarding a now-deleted new file would error and wedge the modal open.
      onOk: (() => {
        const done = new Set<string>();
        return async () => {
          if (gitBusy()) throw new Error("busy");
          const failed: string[] = [];
          for (const p of paths) {
            if (done.has(p)) continue;
            await discardFile(p);
            if (useGitStore.getState().error) failed.push(pathBase(p));
            else done.add(p);
          }
          if (failed.length) {
            // Surface which files still have changes — a success toast here would be
            // dangerous (the user might commit, unaware a discard silently failed).
            message.error(`Discard failed: ${failed.join(", ")}`);
            throw new Error("discard failed"); // keep the modal open
          }
          message.success(`Discarded changes to ${paths.length} file${paths.length > 1 ? "s" : ""}`);
        };
      })(),
    });
  };

  // gitStore records failures in state.error, which only ChangesView renders — so
  // here we surface it as a toast, otherwise context-menu git actions fail silently.
  const reportGit = (okMsg: string) => {
    const err = useGitStore.getState().error;
    if (err) message.error(err);
    else message.success(okMsg);
  };

  // The store's git index isn't safe to write concurrently; bail if any index op
  // (stage/unstage/discard, commit, or reset --hard) is mid-flight — otherwise
  // overlapping writes race on the index and on the shared `error` flag.
  const gitBusy = () => {
    const s = useGitStore.getState();
    if (s.staging || s.committing || s.resetting) {
      message.warning("A git action is already running — try again in a moment");
      return true;
    }
    return false;
  };

  const handleStage = () => {
    if (!fileCtxMenu || gitBusy()) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    // stageFile never rejects (it stores errors in gitStore.error); reportGit surfaces them.
    stageFile(path).then(() => reportGit(`Staged ${name}`));
  };

  const handleUnstage = () => {
    if (!fileCtxMenu || gitBusy()) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    unstageFile(path).then(() => reportGit(`Unstaged ${name}`));
  };

  // Open a diff of the file's working-tree content against its last-committed
  // (HEAD) state. HEAD content comes from go-git; a deleted file reads as empty
  // on the working side so the diff shows what was removed.
  const handleCompareWithHead = async () => {
    if (!fileCtxMenu) return;
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    try {
      const head = await GitGetHeadFileContent(path);
      let current = "";
      try { current = await ReadFile(path); } catch { /* file deleted in worktree */ }
      useQueryStore.getState().openDiff(`HEAD · ${name}`, head?.content ?? "", `Working tree · ${name}`, current);
    } catch (e) {
      message.error(`Could not compare with last commit: ${String(e)}`);
    }
  };

  const handleDiscardGit = () => {
    if (!fileCtxMenu || gitBusy()) return; // don't open the modal mid-op
    const { path, name } = fileCtxMenu;
    setFileCtxMenu(null);
    // New files (no committed version) get deleted by discard. Use the dedicated
    // set, not the display letter — a staged-new-then-modified file shows "M" but
    // is still new (and would be permanently deleted).
    const rel = gitOverlay.relOf(path);
    const isNew = rel != null && gitOverlay.newFilesRel.has(rel);
    // Discard always reverts to HEAD, so a file with both staged and unstaged
    // changes loses its staged part too — warn about that. From the uncapped set
    // so it's correct beyond the 500-file cap.
    const partiallyStaged = rel != null && gitOverlay.partialRel.has(rel);
    modal.confirm({
      title: isNew ? `Delete ${name}?` : `Discard changes to ${name}?`,
      content: isNew
        ? "Permanently deletes this file — it has never been committed and cannot be recovered."
        : partiallyStaged
          ? "Reverts the file to its last committed state — this also discards your staged changes for this file. This cannot be undone."
          : "Reverts the file to its last committed state. This cannot be undone.",
      okText: isNew ? "Delete" : "Discard",
      okButtonProps: { danger: true },
      onOk: async () => {
        // throw (not return) — a resolved onOk closes the modal, which would read
        // as success even though nothing was discarded. Throwing keeps it open.
        if (gitBusy()) throw new Error("busy");
        await discardFile(path);
        reportGit(isNew ? `Deleted ${name}` : `Discarded changes to ${name}`);
      },
    });
  };

  // Repo-wide reset --hard: discard every staged and unstaged change.
  const handleDiscardAll = () => {
    if (gitBusy()) return; // don't open the modal mid-op
    setFileCtxMenu(null);
    modal.confirm({
      title: "Discard all changes?",
      content: "Resets the entire working tree to the last commit (git reset --hard HEAD). Every staged and unstaged change across all files is permanently lost. This cannot be undone.",
      okText: "Discard all",
      okButtonProps: { danger: true },
      onOk: async () => {
        if (gitBusy()) throw new Error("busy"); // keep modal open; a resolved onOk reads as success
        await resetHard();
        reportGit("Discarded all changes — working tree reset to last commit");
      },
    });
  };

  const handleDuplicate = async () => {
    if (!fileCtxMenu || fileCtxMenu.isDir) return;
    const { path } = fileCtxMenu;
    setFileCtxMenu(null);
    try {
      const newPath = await DuplicateFile(path);
      const name = pathBase(newPath);
      const parentDir = pathDir(newPath);
      markSelfChanged(parentDir);
      setTreeData(prev => addChild(prev, parentDir, makeNode(newPath, name, false)));
      message.success(`Created ${name}`);
    } catch (e) {
      message.error(`Duplicate failed: ${String(e)}`);
    }
  };

  // Retire any inline editor whose subject is about to disappear: the node being
  // renamed, or the directory a creation placeholder lives in (or an ancestor of
  // either). Two things go wrong without it. The editor's row vanishes with the
  // node while its session stays live, so a late blur fires `submitRename`
  // against a path that no longer exists and toasts "Rename failed: …" instead
  // of just closing. And the confirm modal stealing focus *is* that blur — it
  // would submit the rename first, moving the file out from under the delete.
  // Hence: called synchronously before the modal opens, not after the unlink.
  const retireEditorsIn = (paths: string[]) => {
    const within = (p: string) => paths.some((d) => p === d || p.startsWith(d + pathSep(d)));
    if (pendingRename && within(pendingRename.path)) cancelRename();
    if (pendingCreate && within(pendingCreate.parent)) cancelCreate();
  };

  const handleDeleteConfirm = () => {
    if (!fileCtxMenu) return;
    // Deleting a folder removes its children, so drop any selected descendants —
    // otherwise the child delete would ENOENT and wedge the modal open on retry.
    const paths = dropDescendants(opPaths());
    const multi = paths.length > 1;
    const { name, isDir } = fileCtxMenu;
    setFileCtxMenu(null);
    retireEditorsIn(paths);
    modal.confirm({
      title: multi ? `Delete ${paths.length} items` : `Delete ${isDir ? "folder" : "file"}`,
      content: multi
        ? `Are you sure you want to delete these ${paths.length} items? Folders and all their contents will be permanently removed.`
        : `Are you sure you want to delete "${name}"?${isDir ? " This item and all its contents will be permanently removed." : ""}`,
      okText: "Delete",
      okButtonProps: { danger: true },
      // Tracks paths already deleted so a retry (after a partial failure) doesn't
      // re-attempt them — re-deleting would ENOENT and wedge the modal open forever.
      onOk: (() => {
        const done = new Set<string>();
        return async () => {
        const failed: string[] = [];
        for (const path of paths) {
          if (done.has(path)) continue;
          const dir = isDirKey(path);
          try {
            if (dir) await DeleteDirectory(path); else await DeleteFile(path);
            done.add(path);
            markSelfChanged(pathDir(path));
            const sep = pathSep(path);
            // Read fresh tabs from the store (not the stale closure captured at render time).
            for (const tab of useQueryStore.getState().tabs) {
              if (tab.path === path || (dir && tab.path?.startsWith(path + sep))) orphanTab(tab.id);
            }
            // Update tree in-place instead of full refresh.
            setTreeData((prev) => removeNode(prev, path));
            const keepKey = (k: Key) => {
              const ks = String(k);
              return ks !== path && !ks.startsWith(path + sep);
            };
            setLoadedKeys((prev) => prev.filter(keepKey));
            setExpandedKeys((prev) => prev.filter(keepKey));
            setSelKeys((prev) => prev.filter((k) => k !== path && !k.startsWith(path + sep)));
          } catch (e) {
            failed.push(`${pathBase(path)}: ${String(e)}`);
          }
        }
        if (failed.length) {
          message.error(`Delete failed — ${failed.join("; ")}`);
          throw new Error("delete failed"); // keep the modal open
        }
        message.success(multi ? `Deleted ${paths.length} items` : `Deleted ${name}`);
        };
      })(),
    });
  };

  const handleRenameStart = () => {
    if (!fileCtxMenu) return;
    // At most one inline editor at a time — a pending create is abandoned.
    cancelCreate();
    const session = newSession();
    renameSessionRef.current = session;
    setPendingRename({ id: session.id, path: fileCtxMenu.path, value: fileCtxMenu.name });
    setFileCtxMenu(null);
  };

  // Siblings of the node being renamed — drives its inline duplicate check.
  // Keyed on the path, not the whole `pendingRename`: the object identity
  // changes on every keystroke, which would re-walk the tree per character.
  const renamePath = pendingRename?.path ?? null;
  const renameSiblings = useMemo(() => {
    if (renamePath === null) return [];
    const dir = pathDir(renamePath);
    return childrenOf(treeData, dir === rootDir ? null : dir);
  }, [renamePath, treeData, rootDir]);

  // Same live validation the creation editor gets — the two share `InlineNameInput`,
  // so they share the rules too. Suppressed for an empty value: the field opens
  // pre-filled, and clearing it is how you back out.
  const renameError = useMemo(() => {
    if (!pendingRename || !pendingRename.value.trim()) return null;
    return validateRenameName(pendingRename.value, renameSiblings, pathBase(pendingRename.path));
  }, [pendingRename, renameSiblings]);

  const cancelRename = () => {
    renameSessionRef.current = null;
    setPendingRename(null);
  };

  const submitRename = async () => {
    const session = activeEditSession(renameSessionRef.current, pendingRename);
    if (!session || !pendingRename) return;
    const { path, value } = pendingRename;
    const sanitized = value.trim();
    if (!sanitized || sanitized === pathBase(path)) { cancelRename(); return; }
    // Keep the editor open for correction — `renameError` already shows the
    // message inline. This used to silently strip path separators and cancel
    // outright on an invalid character, which threw the typed name away.
    if (validateRenameName(value, renameSiblings, pathBase(path))) return;
    const dir = pathDir(path);
    const sep = pathSep(path);
    const newPath = dir.endsWith(sep) ? `${dir}${sanitized}` : `${dir}${sep}${sanitized}`;
    session.phase = "submitting";
    try {
      await RenameFile(path, newPath);
      markSelfChanged(dir);
      // The file has moved on disk, so the tree, key sets and open tabs must be
      // re-pointed whether or not this session is still the live one.
      const prefix = path + sep;
      remapTabsForMove(path, newPath, isDirKey(path));
      setTreeData(prev => renameTreeNode(prev, path, newPath, sanitized));
      const remapKey = (k: Key) => {
        const ks = String(k);
        if (ks === path) return newPath;
        if (ks.startsWith(prefix)) return newPath + ks.substring(path.length);
        return k;
      };
      setLoadedKeys(prev => prev.map(remapKey));
      setExpandedKeys(prev => prev.map(remapKey));
      setSelKeys(prev => prev.map(k => {
        if (k === path) return newPath;
        if (k.startsWith(prefix)) return newPath + k.substring(path.length);
        return k;
      }));
      // Cancelled mid-flight, or superseded by a newer editor: don't close what
      // is now someone else's editor, and don't toast a result the user backed
      // out of. Identity compare — see InlineEditSession.
      if (renameSessionRef.current !== session) return;
      cancelRename();
      message.success(`Renamed to ${sanitized}`);
    } catch (e) {
      if (renameSessionRef.current !== session) return;
      session.phase = "editing"; // allow retry
      message.error(`Rename failed: ${String(e)}`);
    }
  };

  // Blur drops the editor when what's typed can't be used, rather than leaving an
  // unfocused row stuck open on an error. Same contract as `blurCreate`.
  const blurRename = () => {
    if (!activeEditSession(renameSessionRef.current, pendingRename)) return;
    if (renameError) cancelRename();
    else submitRename();
  };

  // ── Inline creation (VS Code–style placeholder row) ────────────────────────
  // List `dir` and merge the result into the tree, marking it loaded. Returns
  // whether the directory's children are now materialized in `treeData` —
  // `addChild` silently drops a node into a parent that has none (see its doc).
  const listChildrenInto = async (dir: string): Promise<boolean> => {
    try {
      const entries = await ListDirectory(dir);
      // Built outside the updater — a throw inside one unmounts the React tree.
      const children = entriesToNodes(entries);
      setTreeData((prev) => updateNode(prev, dir, children, true));
      setLoadedKeys((prev) => (prev.some((k) => String(k) === dir) ? prev : [...prev, dir]));
      return true;
    } catch {
      return false;
    }
  };

  // The eager listing `startNewItem` kicks off for a not-yet-listed parent.
  // `submitCreate` orders itself behind it, because the two are otherwise
  // independent async flows and *either* interleaving loses the new node: submit
  // first and `addChild` no-ops on a parent that still has no children array;
  // listing first — but resolving after the insert — merges a pre-creation
  // snapshot over the node that was just added. Neither self-heals (`onLoadData`
  // skips a parent that now has children, and the watcher echo is swallowed by
  // the 500 ms self-change suppression), so the item would stay invisible until
  // a manual Reload.
  const createListingRef = useRef<Promise<boolean> | null>(null);

  // Start creating a new folder/file under `dir`. `dir` is passed explicitly so
  // both the context menu (a node or the root) and the header toolbar buttons
  // can share this — the root is just exportDir.
  const startNewItem = async (kind: NewItemKind, dir: string) => {
    if (!dir) return;
    setFileCtxMenu(null);
    // At most one inline editor at a time — a pending rename is abandoned.
    cancelRename();
    createListingRef.current = null;
    const parent = dir.replace(/[/\\]+$/, "");
    const session = newSession();
    createSessionRef.current = session;
    setPendingCreate({ id: session.id, kind, parent, value: "" });
    if (parent === rootDir) return; // the root is always "expanded"
    // Expand first so the placeholder is on screen immediately, then make sure
    // the real siblings are present: rc-tree only runs `loadData` for a
    // user-driven expand, so a programmatic one has to list the directory here.
    setExpandedKeys((prev) => (prev.some((k) => String(k) === parent) ? prev : [...prev, parent]));
    if (loadedKeysRef.current.some((k) => String(k) === parent)) return;
    // A failed listing is non-fatal for the *editor*: the placeholder still
    // works, the inline duplicate check just has nothing to compare against and
    // the backend's O_EXCL create stays the safety net. It does matter to
    // `submitCreate`, which is why the promise is parked rather than dropped.
    createListingRef.current = listChildrenInto(parent);
    await createListingRef.current;
  };

  // Siblings the pending item will join — drives the inline duplicate check.
  // Keyed on the parent, not the whole `pendingCreate`: the object identity
  // changes on every keystroke, which would re-walk the tree per character.
  const createParent = pendingCreate?.parent ?? null;
  const pendingSiblings = useMemo(
    () => (createParent === null ? [] : childrenOf(treeData, createParent === rootDir ? null : createParent)),
    [createParent, treeData, rootDir],
  );

  // Inline validation message, shown under the input while it's still open.
  // Suppressed for the untouched empty value so the row doesn't open nagging.
  const createError = useMemo(() => {
    if (!pendingCreate || !pendingCreate.value.trim()) return null;
    return validateNewName(pendingCreate.value, pendingCreate.kind, pendingSiblings);
  }, [pendingCreate, pendingSiblings]);

  const cancelCreate = () => {
    createSessionRef.current = null;
    setPendingCreate(null);
  };

  const submitCreate = async () => {
    const session = activeEditSession(createSessionRef.current, pendingCreate);
    if (!session || !pendingCreate) return;
    const { kind, parent, value } = pendingCreate;
    // Nothing typed — treat like the rename editor does and just drop the row.
    if (!value.trim()) { cancelCreate(); return; }
    // Keep the input open for correction — `createError` is already showing the
    // same message inline, so a toast on top of it would just be noise.
    if (validateNewName(value, kind, pendingSiblings)) return;
    const name = finalNewName(value, kind);
    const sep = pathSep(parent);
    const fullPath = `${parent}${sep}${name}`;
    // A root-level create inserts into the top-level list rather than via
    // addChild, which only matches an existing parent node.
    const isRoot = parent === rootDir;
    session.phase = "submitting";
    try {
      if (kind === "newFolder") await CreateDirectory(fullPath);
      else await CreateFile(fullPath);
      markSelfChanged(parent);
      // Order behind the eager listing before touching the tree — see
      // `createListingRef`. A null ref means none was needed (the parent was
      // already in `loadedKeys`, so it has children); `false` means it failed,
      // leaving the parent with no children array for `addChild` to insert into.
      const listing = createListingRef.current;
      const listed = isRoot || listing === null || (await listing);
      // The item exists on disk by now and can't be un-created, so the tree gets
      // the node whether or not this session is still the live one — leaving a
      // real file invisible would be a worse lie, especially with the fs watcher
      // off.
      const node = makeNode(fullPath, name, kind === "newFolder");
      if (isRoot) setTreeData((prev) => insertSorted(prev, node));
      else if (listed) setTreeData((prev) => addChild(prev, parent, node));
      // Nothing to insert into: re-list the parent instead. The item is on disk,
      // so the fresh listing carries it.
      else await listChildrenInto(parent);
      // Escape landed while the IPC was in flight, or a newer editor has since
      // opened: skip everything the user was cancelling — the toast, the
      // selection move and (the disruptive one) the editor tab — and leave the
      // newer editor's state alone. Identity compare, see InlineEditSession.
      if (createSessionRef.current !== session) return;
      cancelCreate();
      message.success(kind === "newFolder" ? `Created folder ${name}` : `Created ${name}`);
      if (kind === "newFile") {
        // Open the new file like a single click would (preview-tab aware, #849).
        // The tab-sync effect then moves the tree selection onto it.
        const openErr = await openFileInTab(fullPath, previewTabsEnabled);
        if (openErr) message.error(`Could not open file: ${openErr}`);
      } else {
        setSelKeys([fullPath]);
        setAnchorKey(fullPath);
      }
    } catch (e) {
      // Cancelled or superseded, and nothing was created — say nothing, and
      // leave the live session (whoever it is now) untouched.
      if (createSessionRef.current !== session) return;
      session.phase = "editing"; // allow retry — mirrors the rename editor
      const prefix = kind === "newFolder" ? "Could not create folder" : "Could not create file";
      message.error(`${prefix}: ${String(e)}`);
    }
  };

  // Blur mirrors the rename editor: submit what was typed, drop the row when
  // there's nothing usable (empty or failing validation) rather than nagging.
  const blurCreate = () => {
    if (!activeEditSession(createSessionRef.current, pendingCreate)) return;
    if (!pendingCreate?.value.trim() || createError) cancelCreate();
    else submitCreate();
  };

  // Set of cut paths, derived once per clipboard change — titleRender runs for
  // every visible node on every rc-tree render, so an O(n) Array.includes there
  // would be O(nodes × cut-items).
  const cutSet = useMemo(
    () => (clipboard?.mode === "cut" ? new Set(clipboard.paths) : null),
    [clipboard],
  );

  const titleRender = (nodeData: DataNode) => {
    // The synthetic "new item" placeholder row — its file/folder icon comes from
    // the node's own `icon`, so the title is just the editor. Deliberately no
    // data-fbkey: it must stay invisible to visibleKeysInOrder() / Shift+range.
    if (pendingCreate && nodeData.key === newItemKey(pendingCreate.parent)) {
      return (
        <InlineNameInput
          value={pendingCreate.value}
          error={createError}
          placeholder={pendingCreate.kind === "newFolder" ? "Folder name" : "File name"}
          onChange={(v) => setPendingCreate((p) => (p ? { ...p, value: v } : p))}
          onSubmit={submitCreate}
          onCancel={cancelCreate}
          onBlur={blurCreate}
        />
      );
    }
    if (pendingRename && String(nodeData.key) === pendingRename.path) {
      // Keep the data-fbkey wrapper even while editing so the renaming node stays
      // visible to visibleKeysInOrder() (it may be the Shift+range anchor).
      return (
        <span data-fbkey={String(nodeData.key)}>
          <InlineNameInput
            value={pendingRename.value}
            error={renameError}
            selectStem
            onChange={(v) => setPendingRename((p) => (p ? { ...p, value: v } : p))}
            onSubmit={submitRename}
            onCancel={cancelRename}
            onBlur={blurRename}
          />
        </span>
      );
    }
    // Git status overlay: color changed files (with a trailing sigil) and
    // emphasize directories that contain nested changes.
    const key = String(nodeData.key);
    const rel = gitOverlay.relOf(key);
    const letter = rel != null ? gitOverlay.byRel.get(rel) : undefined;
    let content: React.ReactNode;
    if (letter) {
      const color = sigilColor(letter);
      content = (
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6, width: "100%" }}>
          <span style={{ flex: 1, color, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {nodeData.title as React.ReactNode}
          </span>
          <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, fontWeight: 600, color, flexShrink: 0 }}>
            {letter}
          </span>
        </span>
      );
    } else {
      const dirLetter = rel != null ? gitOverlay.dirLetter.get(rel) : undefined;
      content = dirLetter
        ? <span style={{ color: sigilColor(dirLetter), fontWeight: 600 }}>{nodeData.title as React.ReactNode}</span>
        : <>{nodeData.title}</>;
    }
    // data-fbkey lets visibleKeysInOrder() recover the on-screen order for Shift+range.
    // Cut items are dimmed until pasted (then the clipboard clears).
    const isCut = !!cutSet?.has(key);
    return <span data-fbkey={key} style={isCut ? { opacity: 0.5 } : undefined}>{content}</span>;
  };

  const grouped = groupByPath(searchResults);

  // The inline-creation placeholder is a render-only node: injecting it here
  // rather than into `treeData` keeps `treeData` a pure mirror of the filesystem,
  // so mergeNodes / refresh() / the fs:changed watcher need no knowledge of it and
  // can never wipe or duplicate it mid-edit. Rebuilt only when the target or kind
  // changes — the typed value lives in `pendingCreate` and reaches the input
  // through titleRender, so keystrokes don't re-clone the tree.
  const placeholderNode = useMemo<DataNode | null>(() => {
    if (!pendingCreate) return null;
    const isDir = pendingCreate.kind === "newFolder";
    return {
      key: newItemKey(pendingCreate.parent),
      // Empty title: insertSorted puts the row first among its own kind, giving it
      // a stable position instead of one that jumps around while the user types.
      title: "",
      // isLeaf even for a folder — a switcher on a path that doesn't exist yet
      // would offer to expand (and lazily list) it.
      isLeaf: true,
      selectable: false,
      icon: isDir ? <FolderOutlined /> : <FileOutlined style={{ color: CLR_SECONDARY }} />,
    };
  }, [pendingCreate?.kind, pendingCreate?.parent]);

  // rc-tree memoizes its flattened node list by treeData identity, so a status
  // change alone won't re-run titleRender. Hand it a fresh top-level array
  // reference whenever the git overlay changes so the status colors repaint.
  // eslint-disable-next-line react-hooks/exhaustive-deps -- gitOverlay/cutSet
  // deps are intentional: not read in the body, they exist to force rc-tree to
  // re-run titleRender on status change / cut-dimming. cutSet (not clipboard) so a
  // Copy — which changes no node opacity — doesn't trigger a full re-sweep.
  const treeForRender = useMemo(() => {
    const base = [...treeData];
    if (!placeholderNode || !pendingCreate) return base;
    const parentKey = pendingCreate.parent === rootDir ? null : pendingCreate.parent;
    return insertPlaceholder(base, parentKey, placeholderNode);
  }, [treeData, gitOverlay, cutSet, placeholderNode, pendingCreate?.parent, rootDir]);

  // Git status of the right-clicked file (drives the Stage/Unstage/Discard menu items).
  // ctxChanged: the file is changed at all. When we don't have a precise
  // staged/unstaged side (legacy-only data), offer both Stage and Unstage and let
  // the backend no-op the irrelevant one.
  const ctxRel       = fileCtxMenu && !fileCtxMenu.isDir ? gitOverlay.relOf(fileCtxMenu.path) : null;
  const ctxChanged   = ctxRel != null && gitOverlay.byRel.has(ctxRel);
  const ctxStagedHit   = ctxRel != null && gitOverlay.stagedRel.has(ctxRel);
  const ctxUnstagedHit = ctxRel != null && gitOverlay.unstagedRel.has(ctxRel);
  const ctxUnknownSide = ctxChanged && !ctxStagedHit && !ctxUnstagedHit;
  const ctxStaged   = ctxStagedHit   || ctxUnknownSide; // show Unstage
  const ctxUnstaged = ctxUnstagedHit || ctxUnknownSide; // show Stage
  // Comparable against HEAD only when there's a prior committed version. Gate on
  // the authoritative isNew, not the display letter — a staged-new-then-modified
  // file shows "M" but has no HEAD version, so HEAD diff would be empty/misleading.
  const ctxLetter     = ctxRel != null ? gitOverlay.byRel.get(ctxRel) : undefined;
  const ctxIsNew      = ctxRel != null && gitOverlay.newFilesRel.has(ctxRel);
  const ctxComparable = !ctxIsNew && (ctxLetter === "M" || ctxLetter === "R" || ctxLetter === "C" || ctxLetter === "D");

  // Multi-select context: the right-clicked node is part of a >1 selection, so the
  // menu offers bulk variants. ctxCount labels them.
  const ctxMulti = !!fileCtxMenu && selKeys.length > 1 && selKeys.includes(fileCtxMenu.path);
  const ctxCount = ctxMulti ? selKeys.length : 1;
  // Files-only count for the bulk git actions (directories are excluded — see
  // opFilePaths). Memoized: opFilePaths walks the tree per selected key, and this
  // runs on every render while the menu is open.
  const ctxFileCount = useMemo(
    () => (ctxMulti ? opFilePaths().length : 0),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [ctxMulti, selKeys, treeData, fileCtxMenu],
  );

  // Folder-switch dropdown: "Open Folder…" plus recent working directories, so
  // users can change the operating folder without digging into Git Operations.
  const folderMenu: MenuProps["items"] = useMemo(() => {
    const items: MenuProps["items"] = [
      { key: "__open", icon: <FolderOpenOutlined />, label: "Open Folder…" },
      { key: "__open_new", icon: <ExportOutlined />, label: "Open Folder in New Window…" },
    ];
    if (recentDirs.length) {
      items.push({ type: "divider" });
      items.push({ key: "__recent_label", type: "group", label: "Recent" });
      for (const dir of recentDirs) {
        const isCurrent = dir === exportDir;
        items.push({
          key: `recent:${dir}`,
          // Disable the current folder — reselecting it is a no-op (openFolder guards
          // against re-blanking a manual remote override), so make that visible.
          disabled: isCurrent,
          icon: <FolderOutlined style={{ color: isCurrent ? "var(--link)" : CLR_SECONDARY }} />,
          label: <span title={dir} style={{ color: isCurrent ? "var(--link)" : undefined }}>{pathBase(dir) || dir}</span>,
        });
      }
      items.push({ type: "divider" });
      items.push({ key: "__clear", label: "Clear Recent" });
    }
    return items;
  }, [recentDirs, exportDir]);

  const onFolderMenuClick: MenuProps["onClick"] = ({ key }) => {
    if (key === "__open") pickExportDir();
    else if (key === "__open_new") openInNewWindow();
    else if (key === "__clear") clearRecentDirs();
    else if (key.startsWith("recent:")) openFolder(key.slice("recent:".length));
  };

  return (
    <div style={{ padding: "4px 4px" }}>
      {/* Header — two rows: (1) title + actions always fit; (2) a dedicated git
          status row (repo only) so the branch name has room instead of being
          crushed into the action strip. */}
      <div style={{ padding: "0 4px 0 8px", marginBottom: expanded ? 4 : 0 }}>
        {/* Row 1: folder title + primary actions */}
        <div style={{ display: "flex", alignItems: "center", gap: 2 }}>
          <div
            style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer", flex: 1, minWidth: 0, padding: "2px 4px", borderRadius: 4 }}
            onClick={toggleExpanded}
            // Right-click the header title area to create at the workspace root
            // (New Folder… / New File…) — no toolbar buttons needed.
            onContextMenu={onRootContextMenu}
            title={exportDir || "No folder open"}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
          >
            {expanded
              ? <CaretDownFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
              : <CaretRightFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
            }
            <FolderOutlined style={{ color: "var(--text)", fontSize: 13, flexShrink: 0 }} />
            <Text
              ellipsis
              style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em", minWidth: 0 }}
            >
              {pathBase(exportDir) || "Files"}
            </Text>
          </div>
          {clipboard && exportDir && (
            <Tooltip title={`Paste ${clipboard.paths.length} item${clipboard.paths.length > 1 ? "s" : ""} into ${pathBase(toolbarPasteTarget)}`}>
              <Button
                size="small"
                type="text"
                icon={<BlockOutlined style={{ fontSize: 11, color: "var(--link)" }} />}
                onClick={(e) => { e.stopPropagation(); handlePaste(toolbarPasteTarget); }}
                style={{ height: 20, padding: "0 4px", minWidth: 0 }}
              />
            </Tooltip>
          )}
          <Dropdown
            menu={{ items: folderMenu, onClick: onFolderMenuClick }}
            trigger={["click"]}
          >
            <Tooltip title="Open / change working folder">
              <Button
                size="small"
                type="text"
                icon={<FolderOpenOutlined style={{ fontSize: 11, color: CLR_SECONDARY }} />}
                onClick={(e) => e.stopPropagation()}
                style={{ height: 20, padding: "0 4px", minWidth: 0 }}
              />
            </Tooltip>
          </Dropdown>
          <Button
            size="small"
            type="text"
            icon={<SearchOutlined style={{ fontSize: 11, color: searchOpen ? "var(--link)" : CLR_SECONDARY }} />}
            onClick={toggleSearch}
            style={{ height: 20, padding: "0 4px", minWidth: 0 }}
          />
          {/* Git Operations button only when the folder isn't a repo — a repo's
              entry point is the branch/changes pills on row 2 below. */}
          {gitEnabled && exportDir && !gitRepo && (
            <Tooltip title="Git Operations…">
              <Button
                size="small"
                type="text"
                icon={<BranchesOutlined style={{ fontSize: 11 }} />}
                onClick={(e) => { e.stopPropagation(); openGitOps(); }}
                style={{ height: 20, padding: "0 4px", minWidth: 0 }}
              />
            </Tooltip>
          )}
          {(loaded || rootError) && (
            <Button
              size="small"
              type="text"
              icon={<ReloadOutlined style={{ fontSize: 11 }} />}
              loading={loading}
              onClick={(e) => { e.stopPropagation(); refresh(); }}
              style={{ height: 20, padding: "0 4px", minWidth: 0 }}
            />
          )}
        </div>

        {/* Row 2: git status — branch + changed-file count, each opens Git Operations */}
        {gitRepo && (
          <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "3px 2px 1px", minWidth: 0 }}>
            <div
              onClick={(e) => { e.stopPropagation(); openGitOps(); }}
              title={`On branch ${gitBranch}${gitAhead > 0 ? ` · ${gitAhead} to push` : ""} — open Git Operations`}
              style={{ display: "flex", alignItems: "center", gap: 3, minWidth: 0, flexShrink: 1, cursor: "pointer", padding: "1px 6px", borderRadius: 4, background: "color-mix(in srgb, var(--text) 6%, transparent)" }}
            >
              <BranchesOutlined style={{ fontSize: 10, color: "var(--text-muted)", flexShrink: 0 }} />
              <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {gitBranch}{gitAhead > 0 ? ` ↑${gitAhead}` : ""}
              </span>
            </div>
            {gitChanged > 0 && (
              <div
                onClick={(e) => { e.stopPropagation(); openGitOps(); }}
                title={`${gitChanged} changed${gitStagedTot > 0 ? `, ${gitStagedTot} staged` : ""} — open Git Operations`}
                style={{ display: "flex", alignItems: "center", gap: 3, flexShrink: 0, cursor: "pointer", padding: "1px 6px", borderRadius: 4, background: "color-mix(in srgb, var(--warning) 16%, transparent)" }}
              >
                <span style={{ fontFamily: 'var(--editor-font, monospace)', fontSize: 10, fontWeight: 600, color: "var(--warning)" }}>
                  {gitChanged}{gitStagedTot > 0 ? `·${gitStagedTot}` : ""} changed
                </span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Content */}
      {expanded && (
        <div style={{ padding: "0 4px", minHeight: 40 }} onContextMenu={onRootContextMenu}>
          {!exportDir && (
            <div style={{ display: "flex", flexDirection: "column", gap: 6, padding: "2px 0 6px" }}>
              <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>No working directory selected.</Text>
              <Button size="small" icon={<FolderOpenOutlined />} onClick={pickExportDir}>Pick directory…</Button>
            </div>
          )}

          {exportDir && searchOpen && (
            <>
              {/* ── Search input ── */}
              <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}>
                <Input
                  size="small"
                  placeholder={useRegex ? "Regex pattern…" : "Search files…"}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  prefix={<SearchOutlined style={{ color: CLR_SECONDARY, fontSize: 11 }} />}
                  style={{ flex: 1, fontSize: 12 }}
                  allowClear
                  autoFocus
                />
                <Switch
                  size="small"
                  checked={useRegex}
                  onChange={setUseRegex}
                  title="Regular expression"
                />
                <Text style={{ fontSize: 10, color: useRegex ? "var(--link)" : CLR_SECONDARY, userSelect: "none" }}>
                  .*
                </Text>
              </div>

              {/* ── Search states ── */}
              {searching && (
                <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
              )}

              {!searching && searchError && (
                <Text style={{ fontSize: 11, color: "#f85149", display: "block", wordBreak: "break-word" }}>
                  {searchError}
                </Text>
              )}

              {!searching && !searchError && searchQuery.trim() && searchResults.length === 0 && (
                <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>No results.</Text>
              )}

              {/* ── Search results ── */}
              {searchResults.length > 0 && (
                <div>
                  {Array.from(grouped.entries()).map(([path, matches]) => {
                    const relPath = exportDir
                      ? path.replace(exportDir, "").replace(/^[/\\]/, "")
                      : path;
                    return (
                      <div key={path} style={{ marginBottom: 10 }}>
                        <div
                          title={path}
                          style={{
                            fontSize: 11,
                            color: "var(--link)",
                            fontWeight: 500,
                            marginBottom: 2,
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          <FileOutlined style={{ marginRight: 4, fontSize: 10 }} />
                          {relPath}
                        </div>
                        {matches.map((m) => {
                          const { before, match, after, ellipsisBefore, ellipsisAfter } =
                            getSnippet(m.lineContent, m.matchStart, m.matchEnd);
                          return (
                            <div
                              key={`${m.path}:${m.lineNumber}`}
                              onClick={() => handleResultClick(m)}
                              style={{
                                display: "flex",
                                alignItems: "baseline",
                                gap: 6,
                                padding: "1px 4px",
                                cursor: "pointer",
                                borderRadius: 3,
                                overflow: "hidden",
                              }}
                              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
                              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                            >
                              <span style={{ color: CLR_SECONDARY, fontSize: 10, flexShrink: 0, fontFamily: "monospace" }}>
                                {m.lineNumber}
                              </span>
                              <span
                                style={{
                                  fontFamily: "monospace",
                                  fontSize: 11,
                                  color: "var(--text)",
                                  overflow: "hidden",
                                  whiteSpace: "nowrap",
                                  textOverflow: "ellipsis",
                                  flexShrink: 1,
                                  minWidth: 0,
                                }}
                              >
                                {ellipsisBefore && <span style={{ color: CLR_SECONDARY }}>…</span>}
                                {before}
                                <mark style={{ background: "#ffa657", color: "#0d1117", borderRadius: 2, padding: "0 1px" }}>
                                  {match}
                                </mark>
                                {after}
                                {ellipsisAfter && <span style={{ color: CLR_SECONDARY }}>…</span>}
                              </span>
                            </div>
                          );
                        })}
                      </div>
                    );
                  })}
                  {searchResults.length >= 200 && (
                    <Text style={{ fontSize: 10, color: CLR_SECONDARY }}>
                      Showing first 200 results.
                    </Text>
                  )}
                </div>
              )}
            </>
          )}

          {exportDir && !searchOpen && loading && (
            <Spin size="small" style={{ display: "block", margin: "8px auto" }} />
          )}

          {/* treeForRender, not treeData: a root-level create in an empty folder
              renders nothing but the placeholder row. */}
          {exportDir && !searchOpen && !loading && loaded && treeForRender.length === 0 && (
            <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Directory is empty.</Text>
          )}

          {/* Root listing failed (unreadable folder, deleted while open, …).
              Shown instead of an empty panel that silently re-listed forever. */}
          {exportDir && !searchOpen && !loading && !loaded && rootError && (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-start", gap: 4 }}>
              <Text style={{ fontSize: 11, color: CLR_SECONDARY }} title={rootError}>
                Could not read this folder.
              </Text>
              <Button size="small" onClick={() => setRootError(null)} style={{ fontSize: 11, height: 20 }}>
                Retry
              </Button>
            </div>
          )}

          {!searchOpen && loaded && treeForRender.length > 0 && (
            <div
              style={{ overflow: "hidden", userSelect: "none" }}
              ref={treeWrapRef}
              // Shift+click extends the document's text selection (which WebKit
              // still paints despite user-select:none). preventDefault on the
              // shift mousedown suppresses that without blocking the click.
              onMouseDown={(e) => { if (e.shiftKey) e.preventDefault(); }}
              onDoubleClick={onTreeDoubleClick}
            >
              <Tree
                treeData={treeForRender}
                loadedKeys={loadedKeys}
                expandedKeys={expandedKeys}
                selectedKeys={selKeys}
                multiple
                onExpand={(keys) => setExpandedKeys(keys)}
                onLoad={(keys) => setLoadedKeys(keys)}
                loadData={onLoadData as any}
                onSelect={onSelect as any}
                onRightClick={onRightClick as any}
                titleRender={titleRender}
                showIcon
                blockNode
                style={{ background: "transparent", color: "var(--text)", fontSize: 12 }}
              />
            </div>
          )}
        </div>
      )}

      {/* Context menu */}
      {fileCtxMenu && (
        <div
          ref={fileCtxRef}
          role="menu"
          aria-label="File actions"
          aria-orientation="vertical"
          style={{
            position: "fixed",
            top: fileCtxMenu.y,
            left: fileCtxMenu.x,
            zIndex: 9999,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.5)",
            minWidth: 180,
            padding: "4px 0",
          }}
          onClick={(e) => e.stopPropagation()}
          onContextMenu={(e) => e.preventDefault()}
          onKeyDown={(e) => {
            // WAI-ARIA menu pattern: ArrowDown/ArrowUp/Home/End navigate between items.
            const items = fileCtxRef.current?.querySelectorAll<HTMLElement>("[role='menuitem']");
            if (!items?.length) return;
            const idx = Array.from(items).indexOf(document.activeElement as HTMLElement);
            let next = -1;
            if (e.key === "ArrowDown") next = idx < items.length - 1 ? idx + 1 : 0;
            else if (e.key === "ArrowUp") next = idx > 0 ? idx - 1 : items.length - 1;
            else if (e.key === "Home") next = 0;
            else if (e.key === "End") next = items.length - 1;
            if (next >= 0) { e.preventDefault(); items[next].focus(); }
          }}
          onBlur={(e) => {
            // Dismiss menu when focus leaves the container entirely.
            // relatedTarget can be null in WKWebView during focus transitions
            // between sibling elements — defer check to next microtask.
            if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget as Node)) {
              setFileCtxMenu(null);
            } else if (!e.relatedTarget) {
              setTimeout(() => {
                if (!fileCtxRef.current?.contains(document.activeElement)) {
                  setFileCtxMenu(null);
                }
              }, 0);
            }
          }}
        >
          {/* ── Root menu (right-click on empty space): create at the workspace
                root only — no destructive actions on the root directory itself. ── */}
          {fileCtxMenu.isRoot ? (
            <>
              <CtxItem icon={<FolderAddOutlined />} label="New Folder…" onClick={() => startNewItem("newFolder", fileCtxMenu.path)} />
              <CtxItem icon={<FileAddOutlined />} label="New File…" onClick={() => startNewItem("newFile", fileCtxMenu.path)} />
              {clipboard && (
                <CtxItem
                  icon={<BlockOutlined />}
                  label={`Paste ${clipboard.paths.length} item${clipboard.paths.length > 1 ? "s" : ""}`}
                  onClick={() => handlePaste(fileCtxMenu.path)}
                />
              )}
            </>
          ) : (
          <>
          {/* ── File management actions ── */}
          {!ctxMulti && <CtxItem icon={<FolderViewOutlined />} label={revealText} onClick={handleReveal} />}
          {!ctxMulti && <CtxItem icon={<CopyOutlined />} label="Copy Path" onClick={handleCopyPath} />}
          {!ctxMulti && <CtxItem icon={<CopyOutlined />} label="Copy Relative Path" onClick={handleCopyRelativePath} />}

          {/* ── Internal clipboard (cut / copy / paste) ── */}
          <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
          <CtxItem icon={<ScissorOutlined />} label={ctxMulti ? `Cut ${ctxCount} items` : "Cut"} onClick={handleCut} />
          <CtxItem icon={<CopyOutlined />} label={ctxMulti ? `Copy ${ctxCount} items` : "Copy"} onClick={handleCopy} />
          {fileCtxMenu.isDir && clipboard && (
            <CtxItem
              icon={<BlockOutlined />}
              label={`Paste ${clipboard.paths.length} item${clipboard.paths.length > 1 ? "s" : ""}`}
              onClick={() => handlePaste(fileCtxMenu.path)}
            />
          )}

          {!ctxMulti && !fileCtxMenu.isDir && (
            <CtxItem icon={<SnippetsOutlined />} label="Duplicate" onClick={handleDuplicate} />
          )}
          {!ctxMulti && <CtxItem icon={<EditOutlined />} label="Rename…" onClick={handleRenameStart} />}
          <CtxItem icon={<DeleteOutlined />} label={ctxMulti ? `Delete ${ctxCount} items` : "Delete"} onClick={handleDeleteConfirm} danger />

          {/* ── Bulk git staging (multi-select, files only) ── */}
          {gitEnabled && ctxMulti && ctxFileCount > 0 && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<PlusOutlined />} label={`Stage ${ctxFileCount} file${ctxFileCount > 1 ? "s" : ""}`} onClick={handleBulkStage} />
              <CtxItem icon={<MinusOutlined />} label={`Unstage ${ctxFileCount} file${ctxFileCount > 1 ? "s" : ""}`} onClick={handleBulkUnstage} />
              <CtxItem icon={<UndoOutlined />} label={`Discard ${ctxFileCount} file${ctxFileCount > 1 ? "s" : ""}`} onClick={handleBulkDiscard} danger />
            </>
          )}

          {/* ── Git staging actions (single changed file) ── */}
          {gitEnabled && !ctxMulti && !fileCtxMenu.isDir && (ctxUnstaged || ctxStaged) && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              {ctxComparable && (
                <CtxItem icon={<DiffOutlined />} label="Compare with last commit" onClick={handleCompareWithHead} />
              )}
              {ctxUnstaged && (
                <CtxItem icon={<PlusOutlined />} label="Stage" onClick={handleStage} />
              )}
              {ctxStaged && (
                <CtxItem icon={<MinusOutlined />} label="Unstage" onClick={handleUnstage} />
              )}
              <CtxItem icon={<UndoOutlined />} label="Discard changes" onClick={handleDiscardGit} danger />
            </>
          )}

          {/* ── Repo-wide discard (reset --hard) — shown whenever the repo has changes ── */}
          {gitRepo && gitChanged > 0 && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<UndoOutlined />} label="Discard all changes (reset to last commit)" onClick={handleDiscardAll} danger />
            </>
          )}

          {/* ── Directory-only actions ── */}
          {!ctxMulti && fileCtxMenu.isDir && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<FolderAddOutlined />} label="New Folder…" onClick={() => startNewItem("newFolder", fileCtxMenu.path)} />
              <CtxItem icon={<FileAddOutlined />} label="New File…" onClick={() => startNewItem("newFile", fileCtxMenu.path)} />
            </>
          )}

          {/* ── Comparison actions (files only) ── */}
          {!ctxMulti && !fileCtxMenu.isDir && (
            <>
              <div role="separator" style={{ borderTop: "1px solid var(--border)", margin: "4px 0" }} />
              <CtxItem icon={<DiffOutlined />} label="Select for Comparison" onClick={selectFileForComparison} />
              {pendingDiff !== null && (
                <CtxItem
                  icon={<DiffOutlined style={{ color: "var(--accent)" }} />}
                  label={`Compare with: ${pendingDiff.label}`}
                  onClick={compareFileWith}
                />
              )}
            </>
          )}
          </>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * The in-tree name editor, shared by inline rename and inline creation so the two
 * can't drift apart. Handles the fiddly parts once: Enter/Escape, keydown
 * `stopPropagation` (so tree keyboard navigation doesn't eat the keystrokes),
 * click `stopPropagation` (so clicking into the field doesn't select the node),
 * one-shot focus + name-part selection, and the inline validation message.
 *
 * The Enter→blur double-submit race is guarded by the caller's action ref (both
 * `submitRename` and `submitCreate` no-op while a submit is in flight).
 */
function InlineNameInput({
  value, error, placeholder, selectStem, onChange, onSubmit, onCancel, onBlur,
}: {
  value: string;
  error?: string | null;
  placeholder?: string;
  /** Preselect the name up to the extension dot (rename starts with a value). */
  selectStem?: boolean;
  onChange: (v: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  onBlur: () => void;
}) {
  // Per-instance: the editor unmounts when the edit ends, so the next one
  // re-runs its initial selection.
  const initRef = useRef(false);
  const wrapRef = useRef<HTMLSpanElement>(null);
  // The error box is portaled to <body> and positioned against the input's
  // viewport rect. Absolute positioning inside the row can't work: the tree
  // wrapper is `overflow: hidden` and auto-sizes to its rows, so a message under
  // the LAST visible row would be clipped away entirely (z-index doesn't help —
  // it's clipping, not stacking).
  const [errBox, setErrBox] = useState<{ top: number; left: number; width: number; flip: boolean } | null>(null);
  const measureErr = useCallback(() => {
    const r = wrapRef.current?.getBoundingClientRect();
    if (!r) return;
    // Flip above the field when the message would run off the bottom of the
    // window. ERROR_BOX_MAX_H is only a fit test — the flipped box is pinned by
    // its bottom edge (translateY(-100%)), so its real height never matters.
    const flip = r.bottom + ERROR_BOX_MAX_H > window.innerHeight;
    const next = { top: flip ? r.top : r.bottom, left: r.left, width: r.width, flip };
    // Bail out when nothing moved: this runs on every render, and a fresh object
    // each time would re-render forever.
    setErrBox((prev) =>
      prev && prev.top === next.top && prev.left === next.left
        && prev.width === next.width && prev.flip === next.flip
        ? prev
        : next);
  }, []);
  // Re-anchor after every render — a watcher event can insert a sibling row above
  // this one, or the panel can be resized, while the editor is open.
  useLayoutEffect(() => {
    if (error) measureErr(); else setErrBox(null);
  });
  useEffect(() => {
    if (!error) return;
    // Any ancestor scroll moves the anchor without re-rendering us — the capture
    // phase catches them all without knowing which element actually scrolls.
    window.addEventListener("scroll", measureErr, true);
    window.addEventListener("resize", measureErr);
    return () => {
      window.removeEventListener("scroll", measureErr, true);
      window.removeEventListener("resize", measureErr);
    };
  }, [error, measureErr]);
  return (
    <span ref={wrapRef} style={{ position: "relative", display: "block" }}>
      <Input
        size="small"
        autoFocus
        value={value}
        placeholder={placeholder}
        status={error ? "error" : undefined}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          e.stopPropagation(); // prevent tree keyboard navigation
          if (e.key === "Enter") onSubmit();
          else if (e.key === "Escape") onCancel();
        }}
        onBlur={onBlur}
        onClick={(e) => e.stopPropagation()} // prevent tree selection
        // Keep the native menu (paste a name in) and keep the tree's own
        // right-click handler off a row that is mid-edit.
        onContextMenu={(e) => e.stopPropagation()}
        style={{ fontSize: 12, height: 22, padding: "0 4px", userSelect: "text" }}
        ref={(el) => {
          if (!el || initRef.current) return;
          initRef.current = true;
          if (!selectStem) return;
          const input = (el as any).input ?? el;
          if (input?.setSelectionRange) {
            const dot = value.lastIndexOf(".");
            const end = dot > 0 ? dot : value.length;
            requestAnimationFrame(() => input.setSelectionRange(0, end));
          }
        }}
      />
      {error && errBox && createPortal(
        <div
          role="alert"
          style={{
            position: "fixed",
            top: errBox.top, left: errBox.left, width: errBox.width, zIndex: 1050,
            transform: errBox.flip ? "translateY(-100%)" : undefined,
            background: "var(--bg-overlay)", border: "1px solid #f85149",
            borderTop: errBox.flip ? undefined : "none",
            borderBottom: errBox.flip ? "none" : undefined,
            borderRadius: errBox.flip ? "4px 4px 0 0" : "0 0 4px 4px",
            color: "#f85149", fontSize: 11,
            padding: "2px 6px", whiteSpace: "normal", userSelect: "none",
            pointerEvents: "none", // never intercept a click aimed at the tree
          }}
        >
          {error}
        </div>,
        document.body,
      )}
    </span>
  );
}

/** Reusable context menu item */
function CtxItem({ icon, label, onClick, danger }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <div
      role="menuitem"
      tabIndex={0}
      style={{
        display: "flex", alignItems: "center", gap: 8,
        padding: "6px 14px", fontSize: 13, cursor: "pointer",
        color: danger ? "#f85149" : "var(--text)",
        outline: "none",
      }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--border)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      onFocus={(e) => (e.currentTarget.style.background = "var(--border)")}
      // Stop blur propagation so moving focus between items doesn't dismiss the parent menu.
      onBlur={(e) => { e.stopPropagation(); e.currentTarget.style.background = "transparent"; }}
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onClick(); } }}
    >
      <span style={{ fontSize: 12, display: "flex" }}>{icon}</span>
      {label}
    </div>
  );
}
