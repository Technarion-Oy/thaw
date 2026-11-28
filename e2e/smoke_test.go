// SPDX-License-Identifier: GPL-3.0-or-later

//go:build wails_e2e

package e2e_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestWailsE2ESmoke exercises the real compiled Thaw desktop application
// (Go backend + embedded React/TypeScript frontend + native Wails v3 runtime)
// across four smoke checks:
//  1. App startup & first-launch license gate flow
//  2. Live Wails IPC round-trips (both App and sqleditor.Service bindings)
//  3. Native application menu event delivery to the frontend
//  4. Window geometry persistence on close and restore on relaunch
func TestWailsE2ESmoke(t *testing.T) {
	binaryPath := buildOrLocateE2EBinary(t)
	stateDir := t.TempDir()

	app1 := launchThaw(t, binaryPath, stateDir)

	// ── 1. App startup ────────────────────────────────────────────────────────
	// Gate the remaining subtests on AppStartup: they reuse app1 and assume
	// the license gate has been accepted.
	if !t.Run("AppStartup", func(t *testing.T) {
		tools, err := app1.listTools()
		if err != nil {
			t.Fatalf("tools/list failed: %v", err)
		}
		for _, required := range []string{"windows_list", "window_control", "js_eval", "call_bound_method", "emit_event", "wait_for_event"} {
			if !slices.Contains(tools, required) {
				t.Errorf("Wails app-control server missing expected tool %q (got %v)", required, tools)
			}
		}

		windows, err := app1.windowsList()
		if err != nil {
			t.Fatalf("windows_list failed: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("expected 1 main window at startup, got %d: %+v", len(windows), windows)
		}
		if !windows[0].Visible {
			t.Errorf("expected main window to be visible, got %+v", windows[0])
		}

		// A fresh isolated config must present the first-launch License Agreement
		// gate, load the embedded GPL text via real GetLicenseText() IPC, and
		// enable the Accept button.
		pollUntil(t, 15*time.Second, "License Agreement modal with enabled Accept button", func() (bool, error) {
			var state struct {
				HasTitle      bool `json:"hasTitle"`
				HasGPLText    bool `json:"hasGPLText"`
				AcceptEnabled bool `json:"acceptEnabled"`
			}
			err := app1.evalJS(`
				const text = document.body.innerText || '';
				const btn = Array.from(document.querySelectorAll('button')).find(
					(b) => (b.textContent || '').trim() === 'Accept'
				);
				return {
					hasTitle: text.includes('License Agreement'),
					hasGPLText: text.includes('GNU GENERAL PUBLIC LICENSE'),
					acceptEnabled: Boolean(btn && !btn.disabled),
				};
			`, &state)
			if err != nil {
				return false, err
			}
			return state.HasTitle && state.HasGPLText && state.AcceptEnabled, nil
		})

		// Click Accept inside the real webview so LicenseAgreement invokes
		// AcceptLicense() via generated bindings ($Call.ByID) and reveals the
		// main workspace.
		var clicked bool
		if err := app1.evalJS(`
			const btn = Array.from(document.querySelectorAll('button')).find(
				(b) => (b.textContent || '').trim() === 'Accept'
			);
			if (!btn) return false;
			btn.click();
			return true;
		`, &clicked); err != nil || !clicked {
			t.Fatalf("failed to click License Agreement Accept button: clicked=%v err=%v", clicked, err)
		}

		pollUntil(t, 15*time.Second, "license gate dismissal and workspace mount", func() (bool, error) {
			var dismissed bool
			err := app1.evalJS(`
				const text = document.body.innerText || '';
				return !text.includes('Please review and accept the following license');
			`, &dismissed)
			return dismissed, err
		})

		if _, err := os.Stat(isolatedConfigPath(stateDir)); err != nil {
			t.Errorf("expected AcceptLicense to persist config.json at %s: %v", isolatedConfigPath(stateDir), err)
		}
	}) {
		return
	}

	// ── 2. Native menu event delivery to frontend ────────────────────────────
	// Run BEFORE LiveIPCRoundTrip because Wails v3's `call_bound_method` tool
	// runs `await import('/wails/runtime.js')` inside the webview; in apps that
	// bundle `@wailsio/runtime` with Vite, that dynamic import loads a second
	// runtime module copy that overwrites `window._wails.dispatchWailsEvent`.
	// Running NativeMenuEvents first tests the pristine bundled event pipeline.
	t.Run("NativeMenuEvents", func(t *testing.T) {
		// 2a. Connection-gated native menu item must stay disabled while offline.
		gated, err := app1.triggerNativeMenu("Tag Management…")
		if err != nil {
			t.Fatalf("triggerNativeMenu(Tag Management…) failed: %v", err)
		}
		if !gated.Found || gated.Enabled || gated.Triggered {
			t.Errorf("expected Tag Management… to be found=true, enabled=false, triggered=false while disconnected; got %+v", gated)
		}

		// 2b. Radio menu item with payload: View -> Appearance -> Light, then Dark.
		// Proves Go menu callback -> Wails event with string payload ->
		// wailsRuntime.EventsOn -> App.tsx -> useThemeStore -> DOM data-theme.
		for _, tc := range []struct {
			label string
			theme string
		}{
			{label: "Light", theme: "light"},
			{label: "Dark", theme: "dark"},
		} {
			res, err := app1.triggerNativeMenu(tc.label)
			if err != nil {
				t.Fatalf("triggerNativeMenu(%q) failed: %v", tc.label, err)
			}
			if !res.Found || !res.Enabled || !res.Triggered {
				t.Fatalf("expected %q to be found/enabled/triggered, got %+v", tc.label, res)
			}
			pollUntil(t, 10*time.Second, "data-theme="+tc.theme, func() (bool, error) {
				var got string
				err := app1.evalJS(`return document.documentElement.getAttribute('data-theme') || '';`, &got)
				return got == tc.theme, err
			})
		}

		// 2c. Offline menu item: Help -> About Thaw… opens AboutModal, which
		// fetches GetAppInfo() via generated bindings ($Call.ByID) and renders
		// product info in the DOM.
		about, err := app1.triggerNativeMenu("About Thaw…")
		if err != nil {
			t.Fatalf("triggerNativeMenu(About Thaw…) failed: %v", err)
		}
		if !about.Found || !about.Enabled || !about.Triggered {
			t.Fatalf("expected About Thaw… to be found/enabled/triggered, got %+v", about)
		}

		pollUntil(t, 15*time.Second, "About Thaw modal rendered with GetAppInfo data", func() (bool, error) {
			var body string
			err := app1.evalJS(`return document.body.innerText || '';`, &body)
			if err != nil {
				return false, err
			}
			return strings.Contains(body, "About Thaw") && strings.Contains(body, "Technarion Oy"), nil
		})
	})

	// ── 3. Live IPC round-trip ───────────────────────────────────────────────
	t.Run("LiveIPCRoundTrip", func(t *testing.T) {
		// Service 1: thaw/internal/app.App — static metadata, connection state,
		// license persistence, and stateful config round-trip.
		var info struct {
			CompanyName    string `json:"companyName"`
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
		}
		if err := app1.callBoundMethod("thaw/internal/app.App.GetAppInfo", &info); err != nil {
			t.Fatalf("GetAppInfo IPC failed: %v", err)
		}
		if info.ProductName != "Thaw" || info.CompanyName != "Technarion Oy" || info.ProductVersion == "" {
			t.Errorf("unexpected GetAppInfo response: %+v", info)
		}

		var connected bool
		if err := app1.callBoundMethod("thaw/internal/app.App.IsConnected", &connected); err != nil {
			t.Fatalf("IsConnected IPC failed: %v", err)
		}
		if connected {
			t.Errorf("expected IsConnected() == false without Snowflake login")
		}

		var accepted bool
		if err := app1.callBoundMethod("thaw/internal/app.App.IsLicenseAccepted", &accepted); err != nil {
			t.Fatalf("IsLicenseAccepted IPC failed: %v", err)
		}
		if !accepted {
			t.Errorf("expected IsLicenseAccepted() == true after accepting license")
		}

		// Stateful round-trip: SaveEditorPrefs -> GetEditorPrefs.
		var prefs map[string]any
		if err := app1.callBoundMethod("thaw/internal/app.App.GetEditorPrefs", &prefs); err != nil {
			t.Fatalf("GetEditorPrefs IPC failed: %v", err)
		}
		prefs["keywordCase"] = "lower"
		prefs["indentSize"] = 4
		if err := app1.callBoundMethod("thaw/internal/app.App.SaveEditorPrefs", nil, prefs); err != nil {
			t.Fatalf("SaveEditorPrefs IPC failed: %v", err)
		}
		var roundTripped struct {
			KeywordCase string `json:"keywordCase"`
			IndentSize  int    `json:"indentSize"`
		}
		if err := app1.callBoundMethod("thaw/internal/app.App.GetEditorPrefs", &roundTripped); err != nil {
			t.Fatalf("second GetEditorPrefs IPC failed: %v", err)
		}
		if roundTripped.KeywordCase != "lower" || roundTripped.IndentSize != 4 {
			t.Errorf("editor prefs did not round-trip: got %+v, want keywordCase=lower indentSize=4", roundTripped)
		}

		// Service 2: thaw/internal/sqleditor.Service — verify second bound
		// service works over the live Wails IPC bridge too.
		var invalidMarkers []map[string]any
		if err := app1.callBoundMethod("thaw/internal/sqleditor.Service.AnalyzeSqlSyntax", &invalidMarkers, "SELECT 'unterminated"); err != nil {
			t.Fatalf("sqleditor.AnalyzeSqlSyntax(invalid) IPC failed: %v", err)
		}
		if len(invalidMarkers) == 0 {
			t.Errorf("expected syntax diagnostic marker for unterminated string literal, got none")
		}

		var ranges []map[string]any
		if err := app1.callBoundMethod("thaw/internal/sqleditor.Service.GetSqlStatementRanges", &ranges, "SELECT 1;\nSELECT 2;"); err != nil {
			t.Fatalf("sqleditor.GetSqlStatementRanges IPC failed: %v", err)
		}
		if len(ranges) != 2 {
			t.Errorf("expected 2 SQL statement ranges, got %d: %+v", len(ranges), ranges)
		}

		var keywords []string
		if err := app1.callBoundMethod("thaw/internal/sqleditor.Service.GetSnowflakeKeywords", &keywords); err != nil {
			t.Fatalf("sqleditor.GetSnowflakeKeywords IPC failed: %v", err)
		}
		if !slices.Contains(keywords, "SELECT") {
			t.Errorf("expected GetSnowflakeKeywords() to contain SELECT, got %d items", len(keywords))
		}
	})

	// ── 4. Window state persistence and restore ──────────────────────────────
	t.Run("WindowStatePersistenceAndRestore", func(t *testing.T) {
		const targetW, targetH = 1080, 720
		if _, err := app1.callToolRaw("window_control", map[string]any{
			"action": "set_size",
			"width":  targetW,
			"height": targetH,
		}); err != nil {
			t.Fatalf("window_control set_size failed: %v", err)
		}

		var settled windowDescriptor
		pollUntil(t, 10*time.Second, "window resize to 1080x720", func() (bool, error) {
			list, err := app1.windowsList()
			if err != nil || len(list) == 0 {
				return false, err
			}
			settled = list[0]
			return absInt(settled.Width-targetW) <= 50 && absInt(settled.Height-targetH) <= 80, nil
		})

		// Close cleanly so WindowClosing / ShouldQuit runs saveWindowStateOnce().
		if err := app1.closeAndWait(15 * time.Second); err != nil {
			t.Fatalf("app1 did not close cleanly: %v", err)
		}

		sessionPath := isolatedSessionPath(stateDir)
		rawSession, err := os.ReadFile(sessionPath)
		if err != nil {
			t.Fatalf("expected persisted window state at %s after clean close: %v", sessionPath, err)
		}
		var saved struct {
			X         int  `json:"x"`
			Y         int  `json:"y"`
			Width     int  `json:"width"`
			Height    int  `json:"height"`
			Maximized bool `json:"maximised"` //nolint:misspell // matches persisted session.json key
		}
		if err := json.Unmarshal(rawSession, &saved); err != nil {
			t.Fatalf("parsing %s: %v (raw=%s)", sessionPath, err, string(rawSession))
		}
		if absInt(saved.Width-settled.Width) > 50 || absInt(saved.Height-settled.Height) > 80 {
			t.Fatalf("persisted window state %+v does not match pre-close geometry %dx%d", saved, settled.Width, settled.Height)
		}
		if saved.Width == 1400 && saved.Height == 900 {
			t.Fatalf("persisted window state stayed at default 1400x900 instead of resized %dx%d", targetW, targetH)
		}

		// Relaunch with the same isolated state directory and confirm the
		// persisted window geometry (and accepted license) are restored.
		app2 := launchThaw(t, binaryPath, stateDir)

		var accepted bool
		if err := app2.callBoundMethod("thaw/internal/app.App.IsLicenseAccepted", &accepted); err != nil {
			t.Fatalf("app2 IsLicenseAccepted failed: %v", err)
		}
		if !accepted {
			t.Errorf("expected license acceptance to persist across relaunch")
		}

		var restored windowDescriptor
		pollUntil(t, 10*time.Second, "restored window size on relaunch", func() (bool, error) {
			list, err := app2.windowsList()
			if err != nil || len(list) == 0 {
				return false, err
			}
			restored = list[0]
			return absInt(restored.Width-saved.Width) <= 50 && absInt(restored.Height-saved.Height) <= 80, nil
		})
		t.Logf("window state round-trip verified: resized=%dx%d persisted=%dx%d restored=%dx%d",
			settled.Width, settled.Height, saved.Width, saved.Height, restored.Width, restored.Height)

		if err := app2.closeAndWait(15 * time.Second); err != nil {
			t.Fatalf("app2 did not close cleanly: %v", err)
		}
	})
}
