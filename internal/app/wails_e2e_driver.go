// SPDX-License-Identifier: GPL-3.0-or-later

//go:build mcp && !production

package app

import "sync"

// wailsE2EState holds native menu click callbacks solely for the Wails v3
// end-to-end smoke suite (compiled only with `-tags mcp` and never with
// `production`). Unrelated to Thaw's Snowflake MCP server in internal/mcp.
type wailsE2EState struct {
	mu          sync.Mutex
	menuActions map[string]func()
}

// recordWailsE2EMenuAction records a native menu item's OnClick closure so the
// E2E harness can trigger it through the live application menu tree.
func (a *App) recordWailsE2EMenuAction(label string, fn func()) {
	if label == "" || fn == nil {
		return
	}
	a.wailsE2E.mu.Lock()
	defer a.wailsE2E.mu.Unlock()
	if a.wailsE2E.menuActions == nil {
		a.wailsE2E.menuActions = make(map[string]func())
	}
	a.wailsE2E.menuActions[label] = fn
}

// registerWailsE2EMenuDriver subscribes to the test-only "wails-e2e:trigger-menu"
// Wails event. When invoked with a menu item label, it verifies the item is
// present and enabled in the live native application menu before invoking its
// registered OnClick callback, then emits the outcome on
// "wails-e2e:trigger-menu-result".
func (a *App) registerWailsE2EMenuDriver(inspect func(label string) (found, enabled bool)) {
	a.onEvent("wails-e2e:trigger-menu", func(data any) {
		label := extractE2EMenuLabel(data)
		found, enabled := inspect(label)
		triggered := false
		if found && enabled {
			a.wailsE2E.mu.Lock()
			fn := a.wailsE2E.menuActions[label]
			a.wailsE2E.mu.Unlock()
			if fn != nil {
				fn()
				triggered = true
			}
		}
		a.emit("wails-e2e:trigger-menu-result", map[string]any{
			"label":     label,
			"found":     found,
			"enabled":   enabled,
			"triggered": triggered,
		})
	})
}

func extractE2EMenuLabel(data any) string {
	switch v := data.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	case map[string]any:
		if s, ok := v["label"].(string); ok {
			return s
		}
	}
	return ""
}
