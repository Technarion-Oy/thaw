// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// buildMenu constructs the native application menu bar.
// Menu item callbacks emit Wails events so the frontend can react without
// requiring additional bound methods.
func buildMenu(app *App) *menu.Menu {
	appMenu := menu.NewMenu()

	// Items fronting features that need a live Snowflake connection. They start
	// disabled (the app always launches disconnected) and are toggled by
	// app.setMenuConnected, wired at the bottom of this function and called from
	// Connect/Disconnect. Everything else works offline and stays enabled.
	var connectionItems []*menu.MenuItem

	// addConnectionItem adds a text item that is greyed out while disconnected.
	addConnectionItem := func(m *menu.Menu, label string, accelerator *keys.Accelerator, event string) {
		item := m.AddText(label, accelerator, func(_ *menu.CallbackData) {
			wailsruntime.EventsEmit(app.ctx, event)
		})
		item.Disabled = true
		connectionItems = append(connectionItems, item)
	}

	// Standard macOS application menu (About, Services, Hide, Quit, …).
	// This must come first so that macOS renders subsequent submenus correctly.
	appMenu.Append(menu.AppMenu())

	// Standard Edit menu (Cut, Copy, Paste, Select All, Undo, Redo).
	// Required for Cmd+C/V/X/A to work inside WKWebView text inputs.
	appMenu.Append(menu.EditMenu())

	// ── File ─────────────────────────────────────────────────────────────────
	fileMenu := appMenu.AddSubmenu("File")

	fileMenu.AddText("New Tab", keys.CmdOrCtrl("t"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:new-tab")
	})

	fileMenu.AddSeparator()

	fileMenu.AddText("Open File…", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open")
	})

	fileMenu.AddText("Open Any File…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open-any")
	})

	fileMenu.AddText("Open Folder…", keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open-folder")
	})

	fileMenu.AddText("Open Folder in New Window…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open-folder-new-window")
	})

	fileMenu.AddSeparator()

	fileMenu.AddText("Save", keys.CmdOrCtrl("s"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:save")
	})

	fileMenu.AddText("Save As…", keys.Combo("s", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:save-as")
	})

	// ── View ──────────────────────────────────────────────────────────────────
	viewMenu := appMenu.AddSubmenu("View")

	appearanceMenu := viewMenu.AddSubmenu("Appearance")

	// Declare items first so the closures can reference all three.
	var systemItem, lightItem, darkItem *menu.MenuItem

	setAppearance := func(selected *menu.MenuItem, value string) {
		systemItem.Checked = selected == systemItem
		lightItem.Checked = selected == lightItem
		darkItem.Checked = selected == darkItem
		wailsruntime.MenuUpdateApplicationMenu(app.ctx)
		wailsruntime.EventsEmit(app.ctx, "menu:theme", value)
	}

	systemItem = appearanceMenu.AddRadio("System", true, nil, func(_ *menu.CallbackData) {
		setAppearance(systemItem, "system")
	})
	lightItem = appearanceMenu.AddRadio("Light", false, nil, func(_ *menu.CallbackData) {
		setAppearance(lightItem, "light")
	})
	darkItem = appearanceMenu.AddRadio("Dark", false, nil, func(_ *menu.CallbackData) {
		setAppearance(darkItem, "dark")
	})

	appearanceMenu.AddSeparator()

	appearanceMenu.AddText("Customize Layout…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:customize-layout")
	})

	viewMenu.AddSeparator()
	viewMenu.AddText("Editor Preferences…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:editor-preferences")
	})
	viewMenu.AddText("Logging Preferences…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:logging-preferences")
	})
	viewMenu.AddText("File Watching…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:file-watching")
	})
	viewMenu.AddText("Enabled Features…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:feature-flags")
	})

	// ── Tools ─────────────────────────────────────────────────────────────────
	// Catchall for workflow tools and operational settings. Absorbs the former
	// standalone Git, Terminal, and AI menus plus the operational items that used
	// to live under View (MCP Sessions, Query Log, Session Management).
	toolsMenu := appMenu.AddSubmenu("Tools")
	toolsMenu.AddText("Code Snippets…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:code-snippets")
	})
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Tag Management…", nil, "menu:tag-management")
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Export Database DDL…", nil, "menu:export-ddl")
	toolsMenu.AddText("Export Path Format…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:export-path-format")
	})
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Schema Migration…", nil, "menu:migration")
	addConnectionItem(toolsMenu, "Create dbt Project…", nil, "menu:dbt-create")
	toolsMenu.AddSeparator()
	toolsMenu.AddText("Git Operations…", keys.CmdOrCtrl("g"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:git-operations")
	})
	toolsMenu.AddSeparator()
	toolsMenu.AddText("New Terminal", keys.CmdOrCtrl("`"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open-terminal")
	})
	toolsMenu.AddSeparator()
	toolsMenu.AddText("Configure AI Inline Completions…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:configure-ai")
	})
	addConnectionItem(toolsMenu, "MCP Sessions…", nil, "menu:mcp-sessions")

	toolsMenu.AddSeparator()

	// ── Query Log submenu ────────────────────────────────────────────────
	queryLogMenu := toolsMenu.AddSubmenu("Query Log")

	var queryLogEnabled *menu.MenuItem
	queryLogEnabled = queryLogMenu.AddCheckbox("Enable Query Log", false, nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:query-log-toggle", queryLogEnabled.Checked)
	})
	app.setQueryLogMenuCheck = func(checked bool) {
		queryLogEnabled.Checked = checked
		wailsruntime.MenuUpdateApplicationMenu(app.ctx)
	}

	queryLogMenu.AddSeparator()

	var logAll, logUser, logInternal *menu.MenuItem
	setLogFilter := func(selected *menu.MenuItem, value string) {
		logAll.Checked = selected == logAll
		logUser.Checked = selected == logUser
		logInternal.Checked = selected == logInternal
		wailsruntime.MenuUpdateApplicationMenu(app.ctx)
		wailsruntime.EventsEmit(app.ctx, "menu:query-log-filter", value)
	}
	logAll = queryLogMenu.AddRadio("Log All Queries", true, nil, func(_ *menu.CallbackData) {
		setLogFilter(logAll, "all")
	})
	logUser = queryLogMenu.AddRadio("Log User Queries Only", false, nil, func(_ *menu.CallbackData) {
		setLogFilter(logUser, "user")
	})
	logInternal = queryLogMenu.AddRadio("Log Internal Queries Only", false, nil, func(_ *menu.CallbackData) {
		setLogFilter(logInternal, "internal")
	})

	toolsMenu.AddText("Session Management…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:session-management")
	})

	// ── Snowpark ──────────────────────────────────────────────────────────────
	snowparkMenu := appMenu.AddSubmenu("Snowpark")
	snowparkMenu.AddText("Check Environment…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:snowpark-check")
	})
	snowparkMenu.AddText("Setup Environment…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:snowpark-setup")
	})
	snowparkMenu.AddSeparator()
	addConnectionItem(snowparkMenu, "New Notebook…", nil, "menu:snowpark-new-notebook")
	addConnectionItem(snowparkMenu, "Open Notebook…", nil, "menu:snowpark-open-notebook")
	snowparkMenu.AddSeparator()
	snowparkMenu.AddText("Notebook Preferences…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:notebook-preferences")
	})

	// ── Help ──────────────────────────────────────────────────────────────────
	helpMenu := appMenu.AddSubmenu("Help")
	helpMenu.AddText("About Thaw…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:about")
	})
	helpMenu.AddText("Check for Updates…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:check-for-update")
	})
	helpMenu.AddSeparator()
	helpMenu.AddText("Function Catalog…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:function-catalog")
	})
	helpMenu.AddSeparator()
	helpMenu.AddText("Keyboard Shortcuts…", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:keyboard-shortcuts")
	})
	helpMenu.AddSeparator()
	helpMenu.AddText("Reveal Log File", nil, func(_ *menu.CallbackData) {
		if err := app.RevealLogFile(); err != nil {
			wailsruntime.LogErrorf(app.ctx, "reveal log file failed: %v", err)
		}
	})

	// Connect/Disconnect call this to grey out (or restore) the Snowflake-only
	// items. The ctx guard keeps buildMenu usable from tests, which construct an
	// App without a Wails runtime.
	app.setMenuConnected = func(connected bool) {
		for _, item := range connectionItems {
			item.Disabled = !connected
		}
		if app.ctx != nil {
			wailsruntime.MenuUpdateApplicationMenu(app.ctx)
		}
	}

	return appMenu
}
