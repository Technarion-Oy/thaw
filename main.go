// SPDX-License-Identifier: GPL-3.0-or-later

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"embed"

	"thaw/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

// thirdPartyNotices is the generated copyright-notice / license-text bundle for
// every third-party package Thaw redistributes, surfaced in the "About Thaw"
// dialog. Like the frontend assets it is embedded here in the root package
// because //go:embed cannot reference parent directories. Regenerate the file
// with `go run scripts/gen_third_party_notices.go` after changing dependencies.
//
//go:generate go run scripts/gen_third_party_notices.go
//go:embed THIRD_PARTY_NOTICES.md
var thirdPartyNotices string

// main is the application entry point. The embedded frontend assets and
// third-party notices are passed to app.Run, which wires up crash reporting,
// the native menu, and the Wails runtime. The //go:embed directives must live
// in the root package because their paths cannot reference parent directories.
func main() {
	if err := app.Run(assets, thirdPartyNotices); err != nil {
		panic(err)
	}
}
