// SPDX-License-Identifier: GPL-3.0-or-later

import type { DataNode } from "antd/es/tree";

/** What an inline "new item" row is going to create. */
export type NewItemKind = "newFolder" | "newFile";

/**
 * One open inline editor — a rename or a creation — from the moment it opens
 * until it submits or is cancelled.
 *
 * A bare `"idle" | "submitting" | "cancelled"` ref cannot do this job: opening
 * the *next* editor has to reset it to `"idle"`, which silently re-arms every
 * stale closure left over from the previous one. An awaited IPC that resolves
 * after the user cancelled *and* started a different edit would then pass its
 * "was I cancelled?" check, clobber the newer editor's state and fire its own
 * toast. Comparing the captured session **by identity** against the live one
 * makes both "cancelled" (`null`) and "superseded" (a different object) read
 * correctly, whatever the ref has been set to since.
 */
export type InlineEditSession = {
  id: number;
  phase: "editing" | "submitting";
  /**
   * Creation only: the eager `ListDirectory` of the target parent directory,
   * resolving to the children it revealed — or `null` if it failed or the
   * directory has since been deleted, meaning nothing was materialized in the
   * tree for `addChild` to insert into.
   *
   * The children come back rather than just a flag because the submit handler
   * has to re-run the duplicate check against them: until this settles, the
   * live sibling list is empty and the inline check has nothing to collide
   * with. Returning them dodges React's update timing entirely — reading
   * `treeData` back after the `await` races the render that applies it.
   *
   * It hangs off the session for the same reason `id` does. A component-level
   * ref is a single slot that the *next* editor resets, so a completion
   * resuming after its own session was cancelled and another one opened would
   * read someone else's listing — and conclude the parent was listed when it
   * wasn't. `addChild` then silently drops the created node (see its doc) and
   * the file stays invisible until a manual reload. Reached only through the
   * session object the entry guard already validated, that can't happen.
   */
  listing?: Promise<DataNode[] | null> | null;
};

/**
 * The live session, if the state a handler captured still belongs to it and it
 * isn't already in flight — the entry guard for the submit/blur handlers.
 * `null` when no editor is open, when one is already submitting (the
 * double-submit guard), or when `captured` came from an earlier session.
 */
export function activeEditSession(
  live: InlineEditSession | null,
  captured: { id: number } | null | undefined,
): InlineEditSession | null {
  if (!live || live.phase !== "editing") return null;
  if (!captured || captured.id !== live.id) return null;
  return live;
}

/**
 * Key prefix for the synthetic placeholder row that hosts the inline name input
 * while a new file/folder is being created (VS Code style).
 *
 * Every real node key is an absolute filesystem path, so it starts with `/` on
 * POSIX or a drive letter/UNC prefix on Windows — a key beginning with `^@new:`
 * can therefore never collide with one. The placeholder lives only in the
 * render-time tree (see `insertPlaceholder`), never in `treeData`, so refreshes
 * and watcher events can't duplicate or wipe it mid-edit.
 */
export const NEW_ITEM_KEY_PREFIX = "^@new:";

/** Reserved key for the placeholder row being created under `parentDir`. */
export function newItemKey(parentDir: string): string {
  return NEW_ITEM_KEY_PREFIX + parentDir;
}

/** True for the synthetic placeholder key — used to keep it out of selection,
 *  Shift+range order, context menus and duplicate checks. */
export function isNewItemKey(key: unknown): boolean {
  return typeof key === "string" && key.startsWith(NEW_ITEM_KEY_PREFIX);
}

/** Insert a node into a sibling list, maintaining dirs-first alphabetical order. */
export function insertSorted(siblings: DataNode[], child: DataNode): DataNode[] {
  const kids = [...siblings];
  const isDir = !child.isLeaf;
  const name = String(child.title ?? "");
  let i = 0;
  if (isDir) {
    while (i < kids.length && !kids[i].isLeaf && String(kids[i].title ?? "").localeCompare(name) < 0) i++;
  } else {
    while (i < kids.length && !kids[i].isLeaf) i++;
    while (i < kids.length && String(kids[i].title ?? "").localeCompare(name) < 0) i++;
  }
  kids.splice(i, 0, child);
  return kids;
}

/**
 * Insert a child into a parent's children, maintaining dirs-first alphabetical
 * order.
 *
 * **Silently no-ops when the parent has no `children` array yet** — a directory
 * that has never been listed. That's deliberate: seeding `children` with just
 * this one node would render the directory as if it contained nothing else, and
 * `onLoadData` skips a node that already has children, so the missing siblings
 * would never be fetched. The node appears naturally when the directory is
 * expanded and listed instead.
 *
 * The caller therefore has to know whether the parent has been listed before
 * reaching for this. `submitCreate` does: a create into a never-listed directory
 * would otherwise be dropped here and stay invisible until a manual reload.
 */
export function addChild(nodes: DataNode[], parentKey: string, child: DataNode): DataNode[] {
  return nodes.map((n) => {
    if (n.key === parentKey) {
      if (!n.children) return n;
      return { ...n, children: insertSorted(n.children, child) };
    }
    return n.children ? { ...n, children: addChild(n.children, parentKey, child) } : n;
  });
}

/** Find a node by key anywhere in the tree (depth-first). */
export function findNode(nodes: DataNode[], key: string): DataNode | null {
  for (const n of nodes) {
    if (n.key === key) return n;
    if (n.children) {
      const f = findNode(n.children, key);
      if (f) return f;
    }
  }
  return null;
}

/**
 * The sibling list a new item under `parentKey` would join. `null` means the
 * workspace root, which isn't itself a tree node — `nodes` *is* its child list.
 * An unloaded (or missing) directory yields `[]`, so the inline duplicate check
 * simply passes and the backend's `O_EXCL` create stays the safety net.
 */
export function childrenOf(nodes: DataNode[], parentKey: string | null): DataNode[] {
  if (parentKey === null) return nodes;
  return findNode(nodes, parentKey)?.children ?? [];
}

/**
 * Inject the placeholder row at the position the finished item will sort into.
 * Returns a new tree; the parent's `children` array is created when the
 * directory has been expanded but is still empty (or hasn't listed yet), so the
 * row is visible either way. A `parentKey` that no longer exists is a no-op.
 */
export function insertPlaceholder(
  nodes: DataNode[],
  parentKey: string | null,
  placeholder: DataNode,
): DataNode[] {
  if (parentKey === null) return insertSorted(nodes, placeholder);
  return nodes.map((n) => {
    if (String(n.key) === parentKey) {
      return { ...n, children: insertSorted(n.children ?? [], placeholder) };
    }
    return n.children ? { ...n, children: insertPlaceholder(n.children, parentKey, placeholder) } : n;
  });
}

/**
 * Case-insensitive sibling name lookup. Case-insensitive because macOS and
 * Windows filesystems are: `Foo.sql` and `foo.sql` are the same file there, and
 * the backend would reject the create anyway — better to say so inline.
 *
 * On a case-sensitive filesystem (Linux, or a case-sensitive macOS volume) this
 * refuses `Reports` next to an existing `reports`, which the OS itself would
 * allow. Same deliberate trade-off as the Windows rules in `validateRawName`
 * below: the repo may be checked out on a case-insensitive machine, where the
 * two would collide.
 */
export function hasSiblingNamed(siblings: DataNode[], name: string): boolean {
  const lower = name.toLowerCase();
  return siblings.some(
    (n) => !isNewItemKey(n.key) && String(n.title ?? "").toLowerCase() === lower,
  );
}

/** Extension given to a new file when the typed name carries none of its own. */
export const DEFAULT_FILE_EXT = ".sql";

/**
 * Does the typed name already carry its own extension? A leading dot marks a
 * dotfile (`.gitignore`, `.env`) — a complete name, not a bare stem — and a
 * trailing dot is no extension at all (and is rejected by `validateNewName`).
 */
export function hasExtension(name: string): boolean {
  const dot = name.lastIndexOf(".");
  return dot > 0 && dot < name.length - 1;
}

/**
 * The on-disk name for a typed value. Any file type can be created: whatever
 * extension the user typed is kept verbatim (`model.yml`, `README.md`,
 * `.gitignore`). Only a bare stem gets `.sql` appended — the app's primary file
 * type, so the common case stays a single word.
 */
export function finalNewName(raw: string, kind: NewItemKind): string {
  const trimmed = raw.trim();
  if (kind !== "newFile" || !trimmed) return trimmed;
  if (trimmed.startsWith(".") || hasExtension(trimmed)) return trimmed;
  return trimmed + DEFAULT_FILE_EXT;
}

/** Names Windows reserves for devices — unusable with or without an extension. */
const RESERVED_DEVICE_NAME = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i;

/** True for `CON`, `nul.sql`, `LPT1.txt`, … — reserved on Windows whatever the
 *  extension, because the OS matches on the stem alone. */
export function isReservedDeviceName(name: string): boolean {
  const dot = name.indexOf(".");
  return RESERVED_DEVICE_NAME.test(dot >= 0 ? name.slice(0, dot) : name);
}

/**
 * The rules a name must satisfy whatever it's for — shared by creation and
 * rename so the two inline editors can't disagree about what's typeable.
 * `what` names the thing in the "must be provided" message; omit it when the
 * kind isn't known (rename edits a file or a folder through one editor).
 *
 * The Windows-specific rules are applied on every platform on purpose: these
 * files live in a git repo that may well be checked out on Windows, and the
 * character blacklist has always been enforced that way here.
 *
 * `isReservedDeviceName` is the one rule this editor added rather than
 * inherited, and it follows the same policy by explicit decision — a folder
 * named `aux` or `con` is refused on macOS and Linux too, where the OS itself
 * would allow it, because it would break the moment a collaborator checked the
 * repo out on Windows. Reviewed and kept deliberately; not an oversight.
 */
export function validateRawName(raw: string, what?: "file" | "folder"): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return what ? `A ${what} name must be provided.` : "A name must be provided.";
  if (/[/\\]/.test(trimmed)) return "A name cannot contain path separators.";
  if (/[:"*?<>|]/.test(trimmed)) return 'A name cannot contain : " * ? < > |';
  if (trimmed === "." || trimmed === "..") return 'A name cannot be "." or "..".';
  // Windows silently strips trailing dots, and there is no extension to keep —
  // reject rather than quietly turning "report." into "report..sql".
  if (trimmed.endsWith(".")) return 'A name cannot end with a ".".';
  if (isReservedDeviceName(trimmed)) return `"${trimmed}" is a reserved device name on Windows.`;
  return null;
}

/**
 * Validate a typed name for a **new** item against its future siblings. Returns
 * the message to show under the inline input, or `null` when the name is usable.
 * The duplicate check runs against the resolved name, so `query` collides with
 * an existing `Query.sql`.
 */
export function validateNewName(
  raw: string,
  kind: NewItemKind,
  siblings: DataNode[],
): string | null {
  const err = validateRawName(raw, kind === "newFolder" ? "folder" : "file");
  if (err) return err;
  const name = finalNewName(raw, kind);
  if (hasSiblingNamed(siblings, name)) {
    return `A file or folder "${name}" already exists at this location.`;
  }
  return null;
}

/**
 * Validate a typed name for a **rename**. Same rules as creation minus the
 * extension default (a rename means exactly what it says), and the duplicate
 * check skips a name that only differs from the current one by case — that's a
 * legitimate rename, not a collision with itself.
 */
export function validateRenameName(
  raw: string,
  siblings: DataNode[],
  currentName: string,
): string | null {
  const err = validateRawName(raw);
  if (err) return err;
  const name = raw.trim();
  if (name.toLowerCase() === currentName.toLowerCase()) return null;
  if (hasSiblingNamed(siblings, name)) {
    return `A file or folder "${name}" already exists at this location.`;
  }
  return null;
}
