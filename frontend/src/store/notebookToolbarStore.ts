// SPDX-License-Identifier: GPL-3.0-or-later
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
  /** Python version string from the kernel venv (e.g. "3.11.9"). */
  kernelPythonVersion: string | null;
  /** Callbacks (set by NotebookTab, read by QueryPage for the toolbar slot). */
  onRestartKernel: (() => void) | null;
  onDeploy: (() => void) | null;
  onRunAll: (() => void) | null;

  /** Update kernel state (called by NotebookTab). */
  setKernelState: (state: { kernelReady: boolean; kernelStarting: boolean; kernelError: string | null }) => void;
  setKernelPythonVersion: (version: string) => void;
  setCallbacks: (cbs: { onRestartKernel: () => void; onDeploy: () => void; onRunAll: () => void }) => void;
  /** Clear all state when notebook tab is unmounted or deactivated. */
  clear: () => void;
}

export const useNotebookToolbarStore = create<NotebookToolbarState>((set) => ({
  kernelReady: false,
  kernelStarting: false,
  kernelError: null,
  kernelPythonVersion: null,
  onRestartKernel: null,
  onDeploy: null,
  onRunAll: null,

  setKernelState: ({ kernelReady, kernelStarting, kernelError }) =>
    set({ kernelReady, kernelStarting, kernelError }),
  setKernelPythonVersion: (version) => set({ kernelPythonVersion: version }),
  setCallbacks: ({ onRestartKernel, onDeploy, onRunAll }) =>
    set({ onRestartKernel, onDeploy, onRunAll }),
  clear: () => set({
    kernelReady: false,
    kernelStarting: false,
    kernelError: null,
    kernelPythonVersion: null,
    onRestartKernel: null,
    onDeploy: null,
    onRunAll: null,
  }),
}));
