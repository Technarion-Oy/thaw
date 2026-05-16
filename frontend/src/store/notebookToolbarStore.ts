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
// @thaw-domain: Snowpark & Developer Workflows

import { create } from "zustand";

/**
 * Lightweight bridge store that exposes the active notebook tab's kernel state
 * and action callbacks to the unified Toolbar. NotebookTab writes to this store;
 * QueryPage reads from it to render the NotebookToolbarSlot.
 */

interface NotebookToolbarState {
  /** Whether the kernel is ready to execute cells. */
  kernelReady: boolean;
  /** Whether the kernel is currently starting. */
  kernelStarting: boolean;
  /** Kernel error message, or null. */
  kernelError: string | null;
  /** Callbacks (set by NotebookTab, read by QueryPage for the toolbar slot). */
  onRestartKernel: (() => void) | null;
  onAddCell: (() => void) | null;
  onDeploy: (() => void) | null;

  /** Update kernel state (called by NotebookTab). */
  setKernelState: (state: { kernelReady: boolean; kernelStarting: boolean; kernelError: string | null }) => void;
  setCallbacks: (cbs: { onRestartKernel: () => void; onAddCell: () => void; onDeploy: () => void }) => void;
  /** Clear all state when notebook tab is unmounted or deactivated. */
  clear: () => void;
}

export const useNotebookToolbarStore = create<NotebookToolbarState>((set) => ({
  kernelReady: false,
  kernelStarting: false,
  kernelError: null,
  onRestartKernel: null,
  onAddCell: null,
  onDeploy: null,

  setKernelState: ({ kernelReady, kernelStarting, kernelError }) =>
    set({ kernelReady, kernelStarting, kernelError }),
  setCallbacks: ({ onRestartKernel, onAddCell, onDeploy }) =>
    set({ onRestartKernel, onAddCell, onDeploy }),
  clear: () => set({
    kernelReady: false,
    kernelStarting: false,
    kernelError: null,
    onRestartKernel: null,
    onAddCell: null,
    onDeploy: null,
  }),
}));
