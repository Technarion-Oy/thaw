// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
