// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"embed"

	"thaw/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

// main is the application entry point. The embedded frontend assets are passed
// to app.Run, which wires up crash reporting, the native menu, and the Wails
// runtime. The //go:embed directive must live in the root package because its
// path cannot reference parent directories.
func main() {
	if err := app.Run(assets); err != nil {
		panic(err)
	}
}
