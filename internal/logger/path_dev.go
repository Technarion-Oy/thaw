// SPDX-License-Identifier: GPL-3.0-or-later

//go:build dev

package logger

import "os"

const devMode = true

// logFilePath returns the log file path for local development builds.
// Logs are written to ./logs/ inside the project directory so they are easy
// to find during development. The directory is created if it does not exist.
func logFilePath() string {
	_ = os.MkdirAll("logs", 0o755)
	return "logs/thaw.log"
}
