// SPDX-License-Identifier: GPL-3.0-or-later

package version

// Version is the application version string. Override at build time with:
//
//	wails build -ldflags "-X thaw/internal/version.Version=1.2.3"
//	go build   -ldflags "-X thaw/internal/version.Version=1.2.3" .
var Version = "dev"
