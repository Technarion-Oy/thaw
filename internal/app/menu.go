// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	goruntime "runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"thaw/internal/config"
)

// buildMenu constructs the native application menu bar.
// Menu item callbacks emit Wails events so the frontend can react without
// requiring additional bound methods.
func buildMenu(app *App) *application.Menu {
	appMenu := application.NewMenu()

	// Items fronting features that need a live Snowflake connection. They start
	// disabled (the app always launches disconnected) and are toggled by
	// app.setMenuConnected, wired at the bottom of this function and called from
	// Connect/Disconnect. Everything else works offline and stays enabled.
	var connectionItems []*application.MenuItem

	// Container Services items are gated on the connection *and* on the
	// containerServices feature flag, which the user can flip at runtime. They
	// start optimistically enabled-by-flag (mirroring featureFlagsStore) and are
	// corrected by setMenuFeatureFlags as soon as the real flags are read.
	var containerItems []*application.MenuItem

	// The two inputs below are written by two independently-triggered IPC paths
	// (Connect/Disconnect vs. startup/SaveFeatureFlags), so they and the item
	// mutation in refresh are guarded — same shape as the mutex-held state in
	// app.go.
	var menuMu sync.Mutex
	containerEnabled := true
	connected := false

	addItem := func(m *application.Menu, label, accelerator string, cb func(*application.Context)) *application.MenuItem {
		item := m.Add(label).OnClick(cb)
		if accelerator != "" {
			item.SetAccelerator(accelerator)
		}
		return item
	}

	// addConnectionItem adds a text item that is greyed out while disconnected.
	// Any extra args are emitted as the event payload (the frontend uses this to
	// tell six otherwise identical Container Services items apart).
	addConnectionItem := func(m *application.Menu, label, accelerator, event string, data ...any) *application.MenuItem {
		item := addItem(m, label, accelerator, func(_ *application.Context) {
			app.emit(event, data...)
		})
		item.SetEnabled(false)
		connectionItems = append(connectionItems, item)
		return item
	}

	// Standard macOS application menu (About, Services, Hide, Quit, …).
	// This must come first so that macOS renders subsequent submenus correctly.
	// application.NewAppMenu() reads globalApplication.options.Name on darwin,
	// so skip it when unit tests invoke buildMenu without application.New().
	if application.Get() != nil {
		appMenu.AddRole(application.AppMenu)
	}

	// Standard macOS Edit menu (Cut, Copy, Paste, Select All, Undo, Redo).
	// Required on macOS so AppKit responder selectors work inside WKWebView text
	// inputs. Gated to darwin because on Windows/Linux Wails v3's EditMenu role
	// registers Go callbacks with Ctrl+C/V/X/A/Z (and Backspace on Linux GTK4)
	// accelerators that hijack editing from Monaco, ResultGrid, and xterm.js.
	if goruntime.GOOS == "darwin" {
		appMenu.AddRole(application.EditMenu)
	}

	// ── File ─────────────────────────────────────────────────────────────────
	fileMenu := appMenu.AddSubmenu("File")

	addItem(fileMenu, "New Tab", "CmdOrCtrl+t", func(_ *application.Context) {
		app.emit("menu:new-tab")
	})

	fileMenu.AddSeparator()

	addItem(fileMenu, "Open File…", "CmdOrCtrl+o", func(_ *application.Context) {
		app.emit("menu:open")
	})

	addItem(fileMenu, "Open Any File…", "", func(_ *application.Context) {
		app.emit("menu:open-any")
	})

	addItem(fileMenu, "Open Folder…", "CmdOrCtrl+Shift+o", func(_ *application.Context) {
		app.emit("menu:open-folder")
	})

	addItem(fileMenu, "Open Folder in New Window…", "", func(_ *application.Context) {
		app.emit("menu:open-folder-new-window")
	})

	fileMenu.AddSeparator()

	addItem(fileMenu, "Save", "CmdOrCtrl+s", func(_ *application.Context) {
		app.emit("menu:save")
	})

	addItem(fileMenu, "Save As…", "CmdOrCtrl+Shift+s", func(_ *application.Context) {
		app.emit("menu:save-as")
	})

	// ── View ──────────────────────────────────────────────────────────────────
	viewMenu := appMenu.AddSubmenu("View")

	appearanceMenu := viewMenu.AddSubmenu("Appearance")

	// Declare items first so the closures can reference all three.
	var systemItem, lightItem, darkItem *application.MenuItem

	setAppearance := func(selected *application.MenuItem, value string) {
		systemItem.SetChecked(selected == systemItem)
		lightItem.SetChecked(selected == lightItem)
		darkItem.SetChecked(selected == darkItem)
		app.emit("menu:theme", value)
	}

	systemItem = appearanceMenu.AddRadio("System", true).OnClick(func(_ *application.Context) {
		setAppearance(systemItem, "system")
	})
	lightItem = appearanceMenu.AddRadio("Light", false).OnClick(func(_ *application.Context) {
		setAppearance(lightItem, "light")
	})
	darkItem = appearanceMenu.AddRadio("Dark", false).OnClick(func(_ *application.Context) {
		setAppearance(darkItem, "dark")
	})

	appearanceMenu.AddSeparator()

	addItem(appearanceMenu, "Customize Layout…", "", func(_ *application.Context) {
		app.emit("menu:customize-layout")
	})

	viewMenu.AddSeparator()
	addItem(viewMenu, "Editor Preferences…", "", func(_ *application.Context) {
		app.emit("menu:editor-preferences")
	})
	addItem(viewMenu, "Logging Preferences…", "", func(_ *application.Context) {
		app.emit("menu:logging-preferences")
	})
	addItem(viewMenu, "File Watching…", "", func(_ *application.Context) {
		app.emit("menu:file-watching")
	})
	addItem(viewMenu, "Enabled Features…", "", func(_ *application.Context) {
		app.emit("menu:feature-flags")
	})

	// ── Tools ─────────────────────────────────────────────────────────────────
	// Catchall for workflow tools and operational settings. Absorbs the former
	// standalone Git, Terminal, and AI menus plus the operational items that used
	// to live under View (MCP Sessions, Query Log, Session Management).
	toolsMenu := appMenu.AddSubmenu("Tools")
	addItem(toolsMenu, "Code Snippets…", "", func(_ *application.Context) {
		app.emit("menu:code-snippets")
	})
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Tag Management…", "", "menu:tag-management")
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Export Database DDL…", "", "menu:export-ddl")
	addItem(toolsMenu, "Export Path Format…", "", func(_ *application.Context) {
		app.emit("menu:export-path-format")
	})
	toolsMenu.AddSeparator()
	addConnectionItem(toolsMenu, "Schema Migration…", "", "menu:migration")
	addConnectionItem(toolsMenu, "Create dbt Project…", "", "menu:dbt-create")
	// Scaffolding a template is a purely local workflow — no connection needed.
	addItem(toolsMenu, "New Streamlit App from Template…", "", func(_ *application.Context) {
		app.emit("menu:streamlit-template")
	})
	toolsMenu.AddSeparator()
	addItem(toolsMenu, "Git Operations…", "CmdOrCtrl+g", func(_ *application.Context) {
		app.emit("menu:git-operations")
	})
	toolsMenu.AddSeparator()
	addItem(toolsMenu, "New Terminal", "CmdOrCtrl+`", func(_ *application.Context) {
		app.emit("menu:open-terminal")
	})
	toolsMenu.AddSeparator()
	addItem(toolsMenu, "Configure AI Inline Completions…", "", func(_ *application.Context) {
		app.emit("menu:configure-ai")
	})
	addConnectionItem(toolsMenu, "MCP Sessions…", "", "menu:mcp-sessions")

	toolsMenu.AddSeparator()

	// ── Query Log submenu ────────────────────────────────────────────────
	queryLogMenu := toolsMenu.AddSubmenu("Query Log")

	var queryLogEnabled *application.MenuItem
	queryLogEnabled = queryLogMenu.AddCheckbox("Enable Query Log", false).OnClick(func(_ *application.Context) {
		app.emit("menu:query-log-toggle", queryLogEnabled.Checked())
	})
	app.setQueryLogMenuCheck = func(checked bool) {
		queryLogEnabled.SetChecked(checked)
	}

	queryLogMenu.AddSeparator()

	var logAll, logUser, logInternal *application.MenuItem
	setLogFilter := func(selected *application.MenuItem, value string) {
		logAll.SetChecked(selected == logAll)
		logUser.SetChecked(selected == logUser)
		logInternal.SetChecked(selected == logInternal)
		app.emit("menu:query-log-filter", value)
	}
	logAll = queryLogMenu.AddRadio("Log All Queries", true).OnClick(func(_ *application.Context) {
		setLogFilter(logAll, "all")
	})
	logUser = queryLogMenu.AddRadio("Log User Queries Only", false).OnClick(func(_ *application.Context) {
		setLogFilter(logUser, "user")
	})
	logInternal = queryLogMenu.AddRadio("Log Internal Queries Only", false).OnClick(func(_ *application.Context) {
		setLogFilter(logInternal, "internal")
	})

	addItem(toolsMenu, "Session Management…", "", func(_ *application.Context) {
		app.emit("menu:session-management")
	})

	// ── Snowpark ──────────────────────────────────────────────────────────────
	snowparkMenu := appMenu.AddSubmenu("Snowpark")
	addItem(snowparkMenu, "Check Environment…", "", func(_ *application.Context) {
		app.emit("menu:snowpark-check")
	})
	addItem(snowparkMenu, "Setup Environment…", "", func(_ *application.Context) {
		app.emit("menu:snowpark-setup")
	})
	snowparkMenu.AddSeparator()
	addConnectionItem(snowparkMenu, "New Notebook…", "", "menu:snowpark-new-notebook")
	addConnectionItem(snowparkMenu, "Open Notebook…", "", "menu:snowpark-open-notebook")
	snowparkMenu.AddSeparator()
	addItem(snowparkMenu, "Notebook Preferences…", "", func(_ *application.Context) {
		app.emit("menu:notebook-preferences")
	})
	snowparkMenu.AddSeparator()

	// One item per Snowflake SPCS command group. All six open the same
	// ContainerServicesModal on the tab named by the payload. See issue #939.
	containerMenu := snowparkMenu.AddSubmenu("Container Services")
	for _, it := range []struct{ label, tab string }{
		{"Compute Pools…", "pools"},
		{"Services…", "services"},
		{"Run Job…", "jobs"},
		{"Image Repositories…", "images"},
		{"Snapshots…", "snapshots"},
		{"Gateways…", "gateways"},
	} {
		containerItems = append(containerItems,
			addConnectionItem(containerMenu, it.label, "", "menu:container-services", it.tab))
	}

	// ── Help ──────────────────────────────────────────────────────────────────
	helpMenu := appMenu.AddSubmenu("Help")
	addItem(helpMenu, "About Thaw…", "", func(_ *application.Context) {
		app.emit("menu:about")
	})
	addItem(helpMenu, "Check for Updates…", "", func(_ *application.Context) {
		app.emit("menu:check-for-update")
	})
	helpMenu.AddSeparator()
	addItem(helpMenu, "Function Catalog…", "", func(_ *application.Context) {
		app.emit("menu:function-catalog")
	})
	helpMenu.AddSeparator()
	addItem(helpMenu, "Keyboard Shortcuts…", "", func(_ *application.Context) {
		app.emit("menu:keyboard-shortcuts")
	})
	helpMenu.AddSeparator()
	addItem(helpMenu, "Reveal Log File", "", func(_ *application.Context) {
		if err := app.RevealLogFile(); err != nil {
			app.logErrorf("reveal log file failed: %v", err)
		}
	})

	// Recompute every gated item from the two inputs (connection + feature
	// flags). Callers hold menuMu. MenuItem.SetEnabled/SetChecked already update
	// the live native item in place on macOS, Windows, and Linux; do NOT call
	// appMenu.Update() after startup (on Windows it detaches items onto a fresh
	// off-screen HMENU, and on Linux it mutates GTK4 off the main thread).
	refresh := func() {
		for _, item := range connectionItems {
			item.SetEnabled(connected)
		}
		// Runs after the loop above, which also covers the container items.
		for _, item := range containerItems {
			item.SetEnabled(connected && containerEnabled)
		}
	}

	// Connect/Disconnect call this to grey out (or restore) the Snowflake-only items.
	app.setMenuConnected = func(c bool) {
		menuMu.Lock()
		defer menuMu.Unlock()
		connected = c
		refresh()
	}

	// Called at startup and after the user saves feature flags.
	app.setMenuFeatureFlags = func(f config.FeatureFlags) {
		menuMu.Lock()
		defer menuMu.Unlock()
		containerEnabled = f.ContainerServices
		refresh()
	}

	return appMenu
}
