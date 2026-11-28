// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"embed"
	"net/http"
	goruntime "runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"thaw/internal/crashreport"
	"thaw/internal/session"
	"thaw/internal/sqleditor"
	"thaw/internal/version"
)

// Run is the application entry point. It initializes crash reporting, restores
// the persisted window state, builds the native menu, and hands control to the
// Wails v3 runtime. The embedded frontend assets, third-party license notices,
// and license text are passed in from the root package because //go:embed paths
// cannot reference parent directories.
func Run(assets embed.FS, thirdPartyNotices, licenseText string) error {
	crashreport.Init(version.Version)
	defer crashreport.Recover()

	app := NewApp(thirdPartyNotices, licenseText)

	winW, winH := 1400, 900
	if saved, ok := session.LoadWindowState(); ok {
		winW, winH = saved.Width, saved.Height
		app.savedWindowState = &saved
	}

	wailsApp := application.New(application.Options{
		Name:        "Thaw",
		Description: "Thaw — Snowflake Manager",
		Services: []application.Service{
			application.NewService(app),
			application.NewService(sqleditor.NewService()),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: limitAssetConcurrency(32),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		ShouldQuit: func() bool {
			// Note (Linux GTK4): ShouldQuit runs on the GTK main thread via
			// InvokeSync, whereas GTK4 question dialogs dispatch via InvokeAsync.
			// Normal window close goes through the WindowClosing hook (goroutine)
			// first and sets closeConfirmed, and DeclineLicense has no active
			// queries; do not add a Linux quit() caller while queries may run
			// without first confirming off the GTK main thread.
			if !app.confirmClose() {
				return false
			}
			app.saveWindowStateOnce()
			return true
		},
	})
	app.wailsApp = wailsApp

	appMenu := buildMenu(app)
	wailsApp.Menu.Set(appMenu)

	winOpts := application.WebviewWindowOptions{
		Title:              "Thaw — Snowflake Manager",
		Width:              winW,
		Height:             winH,
		BackgroundColour:   application.NewRGB(18, 18, 18),
		UseApplicationMenu: true,
		URL:                "/",
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInset,
		},
	}
	restoreMaximizedOnMac := false
	if app.savedWindowState != nil {
		if app.savedWindowState.Maximized {
			if goruntime.GOOS == "darwin" {
				// On macOS Wails v3 calls windowCenter() right after
				// StartState=WindowStateMaximised on the still-hidden window,
				// canceling the zoom animation before Show(). Maximize once
				// the window is shown in WindowRuntimeReady instead.
				restoreMaximizedOnMac = true
			} else {
				// Do not combine WindowStateMaximised with WindowXY: Wails calls
				// setPosition(X, Y) after maximizing on Windows, shifting the
				// maximized window off-frame.
				winOpts.StartState = application.WindowStateMaximised
			}
		} else {
			winOpts.InitialPosition = application.WindowXY
			winOpts.X = app.savedWindowState.X
			winOpts.Y = app.savedWindowState.Y
		}
	}

	mainWindow := wailsApp.Window.NewWithOptions(winOpts)
	app.mainWindow = mainWindow

	// On macOS, Wails v3 activates the app at ApplicationDidFinishLaunching
	// (before the window is visible) and later calls Show() using only
	// makeKeyAndOrderFront: without re-activating the app — leaving the window
	// behind the launching terminal/IDE during `wails3 dev`. Focus once on
	// initial runtime ready to bring the window to the foreground (and apply
	// any deferred macOS maximize).
	var initialReadyOnce sync.Once
	mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		initialReadyOnce.Do(func() {
			if restoreMaximizedOnMac {
				mainWindow.Maximise()
			}
			mainWindow.Focus()
		})
	})

	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !app.confirmClose() {
			e.Cancel()
			return
		}
		app.saveWindowStateOnce()
	})

	return wailsApp.Run()
}

// limitAssetConcurrency caps in-flight asset requests. Under `wails3 dev` the
// WKWebView requests the whole unbundled module graph (300+ files) at once and
// Wails proxies each over its own TCP connection to Vite; that burst overflows
// Vite's listen backlog (kern.ipc.somaxconn = 128 on macOS), connections are
// reset, module imports fail, and the window stays blank white.
//
// In Wails v3, Assets.Middleware sits outside the /wails/runtime IPC transport
// handler, and bound calls are HTTP POSTs held open until Go returns. Bypassing
// /wails/ paths guarantees long-running IPC calls (WaitForQueryResult,
// migration, pip install) never consume asset slots or block cancel calls.
func limitAssetConcurrency(n int) application.Middleware {
	sem := make(chan struct{}, n)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/wails/") {
				next.ServeHTTP(w, r)
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		})
	}
}
