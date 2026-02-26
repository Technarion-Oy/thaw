// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	appMenu := buildMenu(app)

	err := wails.Run(&options.App{
		Title:  "Thaw — Snowflake Manager",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Menu: appMenu,
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})
	if err != nil {
		panic(err)
	}
}

// buildMenu constructs the native application menu bar.
// Menu item callbacks emit Wails events so the frontend can react without
// requiring additional bound methods.
func buildMenu(app *App) *menu.Menu {
	appMenu := menu.NewMenu()

	// Standard macOS application menu (About, Services, Hide, Quit, …).
	// This must come first so that macOS renders subsequent submenus correctly.
	appMenu.Append(menu.AppMenu())

	// ── File ─────────────────────────────────────────────────────────────────
	fileMenu := appMenu.AddSubmenu("File")

	fileMenu.AddText("New Tab", keys.CmdOrCtrl("t"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:new-tab")
	})

	fileMenu.AddSeparator()

	fileMenu.AddText("Open…", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "menu:open")
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

	return appMenu
}
