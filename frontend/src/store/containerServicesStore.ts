// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { create } from "zustand";

/** The six Container Services tabs, one per Snowflake SPCS command group. */
export const CONTAINER_TABS = [
  "pools",
  "services",
  "jobs",
  "images",
  "snapshots",
  "gateways",
] as const;

export type ContainerTab = (typeof CONTAINER_TABS)[number];

/** Narrow an untrusted tab name (a native-menu event payload) to a known tab. */
export function toContainerTab(value: unknown): ContainerTab {
  return CONTAINER_TABS.includes(value as ContainerTab) ? (value as ContainerTab) : "pools";
}

// Shared open/close state for the account-wide Container Services view. Like
// tagManagementStore, the view is one modal rendered once (in QueryPage) but
// reachable from several places — the six Snowpark → Container Services menu
// items and, later, the object browser — so visibility lives in a store. The requested
// tab travels with `open` so a menu item lands directly on its own tab.
interface ContainerServicesState {
  open: boolean;
  tab: ContainerTab;
  openView: (tab?: ContainerTab) => void;
  setTab: (tab: ContainerTab) => void;
  closeView: () => void;
}

export const useContainerServicesStore = create<ContainerServicesState>((set) => ({
  open: false,
  tab: "pools",
  openView: (tab = "pools") => set({ open: true, tab }),
  setTab: (tab) => set({ tab }),
  closeView: () => set({ open: false }),
}));
