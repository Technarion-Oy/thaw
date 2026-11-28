// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

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
func Run(assets embed.FS, thirdPartyNotices, licenseText string) error {
	crashreport.Init(version.Version)
	defer crashreport.Recover()

	app := NewApp(thirdPartyNotices, licenseText)

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
			Assets:     assets,
			Middleware: limitAssetConcurrency(32),
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(_ context.Context) bool {
			if !app.isQueryRunning() {
				return false // nothing running — allow close
			}
			result, err := app.confirmDialog(confirmDialogOpts{
				title:         "Query running",
				message:       "A query is currently running. Close anyway?",
				buttons:       []string{"Close anyway", "Cancel"},
				defaultButton: "Cancel",
				cancelButton:  "Cancel",
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

// limitAssetConcurrency caps in-flight asset requests. Under `wails dev` the
// WKWebView requests the whole unbundled module graph (300+ files) at once and
// Wails proxies each over its own TCP connection to Vite; that burst overflows
// Vite's listen backlog (kern.ipc.somaxconn = 128 on macOS), connections are
// reset, module imports fail, and the window stays blank white. Harmless for
// production builds, where assets are served from the embedded FS.
func limitAssetConcurrency(n int) assetserver.Middleware {
	sem := make(chan struct{}, n)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sem <- struct{}{}
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		})
	}
}
