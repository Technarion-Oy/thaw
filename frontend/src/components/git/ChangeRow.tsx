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
// @thaw-domain: Git Integration

import { Button, Tooltip, Popconfirm } from "antd";
import { PlusOutlined, MinusOutlined, UndoOutlined, WarningOutlined } from "@ant-design/icons";
import type { FileChange } from "../../store/gitStore";
import { sigilColor, objectTypeFromPath, splitPath } from "./gitStatusUtil";

const MONO = 'var(--editor-font, "JetBrains Mono", monospace)';

// A single changed-file row — the signature element of the Changes dialog. Reads
// like a typeset `git status` line: a colored status spine + mono sigil on the
// left, the path (directory prefix as faint mono data, filename as the object in
// Inter), the Snowflake object-type ledger column, and reveal-on-hover actions
// overlaid so the resting row stays tight.
export default function ChangeRow({
  file, action, onAction, onDiscard, busy, isNew,
}: {
  file: FileChange;
  action: "stage" | "unstage";
  onAction: (path: string) => void;
  onDiscard: (path: string) => void;
  busy: boolean;
  // Whether the file has no committed version (added/untracked) — passed in
  // because the row's display letter can be "M" for a staged-new-then-modified
  // file, yet discarding it still deletes it. Determined from the staging side.
  isNew: boolean;
}) {
  const { dir, name } = splitPath(file.path);
  const ot = objectTypeFromPath(file.path);
  const color = sigilColor(file.status);

  const discardDesc = isNew
    ? "Permanently deletes this file — it has never been committed and cannot be recovered."
    : "Reverts the file to its last committed state. This cannot be undone.";

  return (
    <div
      className="git-change-row"
      style={{ position: "relative", display: "flex", alignItems: "center", height: 28, borderLeft: `2px solid ${color}` }}
    >
      {/* Left rail: the status sigil, column-aligned down the whole list */}
      <span style={{ width: 26, flexShrink: 0, textAlign: "center", fontFamily: MONO, fontSize: 11.5, fontWeight: 700, color }}>
        {file.status}
      </span>
      {/* Path: directory prefix is data (mono, faint); filename is the object (Inter, bold) */}
      <span title={file.path} style={{ flex: 1, minWidth: 0, fontSize: 12.5, lineHeight: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
        <span style={{ fontFamily: MONO, fontSize: 11.5, color: "var(--text-faint)" }}>{dir}</span>
        <span style={{ fontWeight: 500, color: "var(--text)" }}>{name}</span>
      </span>

      {/* Right ledger column: the Snowflake object type, in its type color */}
      <span style={{ fontFamily: MONO, fontSize: 10.5, color: ot ? ot.color : "transparent", flexShrink: 0, width: 78, textAlign: "right", paddingRight: 10, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
        {ot ? ot.label : ""}
      </span>

      {/* Reveal-on-hover / focus actions. Overlaid (so the resting row stays
          tight) but positioned to the LEFT of the object-type column at right:78,
          so the type label stays visible while the user decides to stage/discard. */}
      <span
        className="git-row-actions"
        style={{
          position: "absolute", right: 78, top: 0, height: "100%",
          display: "flex", alignItems: "center", gap: 2, paddingLeft: 24,
          background: "linear-gradient(to right, transparent, var(--bg-hover) 28%)",
        }}
      >
        <Tooltip title={isNew ? "Delete file" : "Discard changes"}>
          <Popconfirm
            title={isNew ? `Delete ${name}?` : `Discard changes to ${name}?`}
            description={discardDesc}
            onConfirm={() => onDiscard(file.path)}
            okText={isNew ? "Delete" : "Discard"}
            okButtonProps={{ danger: true }}
            icon={<WarningOutlined style={{ color: "var(--danger)" }} />}
          >
            <Button size="small" type="text" disabled={busy} icon={<UndoOutlined />} style={{ height: 22, width: 22, minWidth: 0, padding: 0, color: "var(--text-muted)" }} />
          </Popconfirm>
        </Tooltip>
        <Tooltip title={action === "stage" ? "Stage" : "Unstage"}>
          <Button
            size="small" type="text" disabled={busy}
            icon={action === "stage" ? <PlusOutlined /> : <MinusOutlined />}
            onClick={() => onAction(file.path)}
            style={{ height: 22, width: 22, minWidth: 0, padding: 0, color: "var(--text-muted)" }}
          />
        </Tooltip>
      </span>
    </div>
  );
}
