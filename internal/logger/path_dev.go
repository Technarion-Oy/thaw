// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
