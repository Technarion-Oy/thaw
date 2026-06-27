// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type PanelId   = "export" | "files" | "objects" | "account";
export type SidebarId = "left" | "right";

const DEFAULT_LEFT:              PanelId[] = ["export", "files"];
const DEFAULT_RIGHT:             PanelId[] = ["objects", "account"];
const DEFAULT_EDITOR_SPLIT                 = 0.4;
const DEFAULT_LEFT_WIDTH                   = 220;
const DEFAULT_RIGHT_WIDTH                  = 260;
const DEFAULT_SPLIT_EDITOR_WIDTH           = 0.5;
const DEFAULT_CELL_DETAIL_WIDTH            = 300;

interface PanelLayoutState {
  left:          PanelId[];
  right:         PanelId[];
  editorSplit:   number;   // 0–1, fraction of space given to SQL editor vs results
  leftWidth:     number;
  rightWidth:    number;
  leftHidden:    boolean;  // ⌘B sidebar toggle

  splitEditorWidth:    number;
  cellDetailWidth:     number;   // width of the result-grid cell detail side panel
  movePanel:           (panelId: PanelId, targetId: PanelId | null, targetSidebar: SidebarId, insertBefore: boolean) => void;
  setEditorSplit:      (v: number) => void;
  setLeftWidth:        (v: number) => void;
  setRightWidth:       (v: number) => void;
  setSplitEditorWidth: (v: number) => void;
  setCellDetailWidth:  (v: number) => void;
  toggleLeftHidden:    () => void;
  reset:               () => void;
}

export const usePanelLayoutStore = create<PanelLayoutState>()(
  persist(
    (set) => ({
      left:             DEFAULT_LEFT,
      right:            DEFAULT_RIGHT,
      editorSplit:      DEFAULT_EDITOR_SPLIT,
      leftWidth:        DEFAULT_LEFT_WIDTH,
      rightWidth:       DEFAULT_RIGHT_WIDTH,
      leftHidden:       false,
      splitEditorWidth: DEFAULT_SPLIT_EDITOR_WIDTH,
      cellDetailWidth:  DEFAULT_CELL_DETAIL_WIDTH,

      movePanel: (panelId, targetId, targetSidebar, insertBefore) =>
        set((state) => {
          const newLeft  = state.left.filter((id) => id !== panelId);
          const newRight = state.right.filter((id) => id !== panelId);
          const target   = [...(targetSidebar === "left" ? newLeft : newRight)];

          if (targetId === null) {
            target.push(panelId);
          } else {
            const idx = target.indexOf(targetId);
            target.splice(idx === -1 ? target.length : insertBefore ? idx : idx + 1, 0, panelId);
          }

          return {
            left:  targetSidebar === "left"  ? target : newLeft,
            right: targetSidebar === "right" ? target : newRight,
          };
        }),

      setEditorSplit:      (editorSplit)      => set({ editorSplit }),
      setLeftWidth:        (leftWidth)        => set({ leftWidth }),
      setRightWidth:       (rightWidth)       => set({ rightWidth }),
      setSplitEditorWidth: (splitEditorWidth) => set({ splitEditorWidth }),
      setCellDetailWidth:  (cellDetailWidth)  => set({ cellDetailWidth }),
      toggleLeftHidden:    ()                 => set((s) => ({ leftHidden: !s.leftHidden })),

      reset: () => set({
        left:             DEFAULT_LEFT,
        right:            DEFAULT_RIGHT,
        editorSplit:      DEFAULT_EDITOR_SPLIT,
        leftWidth:        DEFAULT_LEFT_WIDTH,
        rightWidth:       DEFAULT_RIGHT_WIDTH,
        leftHidden:       false,
        splitEditorWidth: DEFAULT_SPLIT_EDITOR_WIDTH,
        cellDetailWidth:  DEFAULT_CELL_DETAIL_WIDTH,
      }),
    }),
    {
      name: "thaw-panel-layout",
      version: 1,
      // v1 folded the standalone "git" panel into the Files panel. Strip any
      // persisted "git" entries so old layouts don't render an empty panel.
      migrate: (persisted: any, version: number) => {
        if (persisted && version < 1) {
          const strip = (arr: unknown) => Array.isArray(arr) ? arr.filter((id) => id !== "git") : arr;
          persisted.left = strip(persisted.left);
          persisted.right = strip(persisted.right);
        }
        return persisted as PanelLayoutState;
      },
    }
  )
);
