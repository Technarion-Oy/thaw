// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/crashreport"
	"thaw/internal/session"
	"thaw/internal/sqleditor"
	"thaw/internal/version"
)

// Run is the application entry point. It initializes crash reporting, restores
// the persisted window state, builds the native menu, and hands control to the
// Wails runtime. The embedded frontend assets and third-party license notices
// are passed in from the root package because //go:embed paths cannot reference
// parent directories.
func Run(assets embed.FS, thirdPartyNotices string) error {
	crashreport.Init(version.Version)
	defer crashreport.Recover()

	app := NewApp(thirdPartyNotices)

	winW, winH := 1400, 900
	if saved, ok := session.LoadWindowState(); ok {
		winW, winH = saved.Width, saved.Height
		app.savedWindowState = &saved
	}

	appMenu := buildMenu(app)

	return wails.Run(&options.App{
		Title:  "Thaw — Snowflake Manager",
		Width:  winW,
		Height: winH,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			if !app.isQueryRunning() {
				return false // nothing running — allow close
			}
			result, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Type:          wailsruntime.QuestionDialog,
				Title:         "Query running",
				Message:       "A query is currently running. Close anyway?",
				Buttons:       []string{"Close anyway", "Cancel"},
				DefaultButton: "Cancel",
				CancelButton:  "Cancel",
			})
			if err != nil || result != "Close anyway" {
				return true // prevent close
			}
			return false // user confirmed — allow close (shutdown will cancel the query)
		},
		Bind: []interface{}{
			app,
			sqleditor.NewService(),
		},
		Menu: appMenu,
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})
}
