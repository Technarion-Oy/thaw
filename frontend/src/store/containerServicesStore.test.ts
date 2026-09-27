// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  CONTAINER_TABS,
  toContainerTab,
  useContainerServicesStore,
} from "./containerServicesStore";

describe("toContainerTab", () => {
  it("accepts every known tab", () => {
    for (const tab of CONTAINER_TABS) expect(toContainerTab(tab)).toBe(tab);
  });

  // The value arrives as a native-menu event payload, so anything could show up.
  it("falls back to pools for unknown payloads", () => {
    for (const bad of ["Pools", "", undefined, null, 0, {}]) {
      expect(toContainerTab(bad)).toBe("pools");
    }
  });
});

describe("useContainerServicesStore", () => {
  it("opens on the requested tab and closes again", () => {
    const { openView, setTab, closeView } = useContainerServicesStore.getState();

    openView("jobs");
    expect(useContainerServicesStore.getState()).toMatchObject({ open: true, tab: "jobs" });

    setTab("gateways");
    expect(useContainerServicesStore.getState().tab).toBe("gateways");

    // Closing keeps the tab, so reopening from the sidebar lands where you left off.
    closeView();
    expect(useContainerServicesStore.getState()).toMatchObject({ open: false, tab: "gateways" });

    // No argument = the default tab.
    openView();
    expect(useContainerServicesStore.getState().tab).toBe("pools");
    closeView();
  });
});
