// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"slices"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

// connectionGatedLabels are the native menu items that front IPC requiring a
// live Snowflake connection. Adding a Snowflake-only menu item without gating it
// resurrects the bug in #876 (clickable item → dialog that errors with
// ErrNotConnected), so this list is pinned exactly.
var connectionGatedLabels = []string{
	"Create dbt Project…",
	"Export Database DDL…",
	"MCP Sessions…",
	"New Notebook…",
	"Open Notebook…",
	"Schema Migration…",
	"Tag Management…",
}

// collectDisabled walks the menu tree and returns the labels of every text item,
// split into disabled and enabled.
func collectDisabled(m *menu.Menu) (disabled, enabled []string) {
	var walk func(items []*menu.MenuItem)
	walk = func(items []*menu.MenuItem) {
		for _, item := range items {
			if item.SubMenu != nil {
				walk(item.SubMenu.Items)
				continue
			}
			if item.Type != menu.TextType {
				continue
			}
			if item.Disabled {
				disabled = append(disabled, item.Label)
			} else {
				enabled = append(enabled, item.Label)
			}
		}
	}
	walk(m.Items)
	slices.Sort(disabled)
	slices.Sort(enabled)
	return disabled, enabled
}

// The app always launches disconnected, so the Snowflake-only items must be
// greyed out from the start — and nothing else may be.
func TestBuildMenuStartsDisconnected(t *testing.T) {
	app := &App{}
	disabled, _ := collectDisabled(buildMenu(app))

	if len(disabled) != len(connectionGatedLabels) {
		t.Fatalf("disabled items = %v, want exactly %v", disabled, connectionGatedLabels)
	}
	for i, want := range connectionGatedLabels {
		if disabled[i] != want {
			t.Errorf("disabled[%d] = %q, want %q", i, disabled[i], want)
		}
	}
}

// setMenuConnected must flip exactly the gated items, both ways, and leave the
// offline-capable ones (File/View/Help, terminal, git, Snowpark env setup) alone.
func TestSetMenuConnectedTogglesOnlyGatedItems(t *testing.T) {
	app := &App{}
	m := buildMenu(app)

	if app.setMenuConnected == nil {
		t.Fatal("buildMenu did not wire setMenuConnected")
	}
	_, offlineEnabled := collectDisabled(m)

	app.setMenuConnected(true)
	disabled, enabled := collectDisabled(m)
	if len(disabled) != 0 {
		t.Errorf("after connect, still disabled: %v", disabled)
	}
	for _, label := range connectionGatedLabels {
		if !slices.Contains(enabled, label) {
			t.Errorf("after connect, %q was not enabled", label)
		}
	}

	app.setMenuConnected(false)
	disabled, enabled = collectDisabled(m)
	if len(disabled) != len(connectionGatedLabels) {
		t.Errorf("after disconnect, disabled = %v, want %v", disabled, connectionGatedLabels)
	}
	// Offline-capable items are untouched by both transitions.
	for _, label := range offlineEnabled {
		if !slices.Contains(enabled, label) {
			t.Errorf("offline-capable item %q was disabled by the connection toggle", label)
		}
	}
}

// A few representative offline-capable items must never be gated, so the menu
// stays usable before the first connection.
func TestOfflineItemsAlwaysEnabled(t *testing.T) {
	app := &App{}
	disabled, _ := collectDisabled(buildMenu(app))

	for _, label := range []string{
		"Open File…", "Save", "Editor Preferences…", "Code Snippets…",
		"Export Path Format…", "Git Operations…", "New Terminal",
		"Check Environment…", "Setup Environment…", "Notebook Preferences…",
		"Function Catalog…", "About Thaw…",
	} {
		if slices.Contains(disabled, label) {
			t.Errorf("%q is disabled while disconnected, but works offline", label)
		}
	}
}
