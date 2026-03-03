// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package crashreport captures unexpected panics and writes a structured report
// to the application log directory. A remote-delivery placeholder is provided
// so that a crash-reporting backend can be wired in when one is chosen.
//
// No personally identifiable information, SQL query content, credentials, or
// account-specific identifiers are ever included in crash reports.
package crashreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"thaw/internal/logger"
)

var appVersion string

// Init records the application version for inclusion in crash reports.
// Call once at startup, before deferring Recover in main().
func Init(version string) {
	appVersion = version
}

// Recover must be called with defer at the top of main(). If the goroutine
// panics it writes a crash report to the log directory, then re-panics so
// the process terminates with a non-zero exit code.
func Recover() {
	r := recover()
	if r == nil {
		return
	}
	stack := debug.Stack()
	report(fmt.Sprintf("%v", r), stack)
	panic(r) // re-panic — ensures non-zero exit code and visible crash message
}

// report writes crash details to the application log and to a JSON crash file
// co-located with the rotating log files (logger.Dir).
func report(panicMsg string, stack []byte) {
	payload := map[string]any{
		"version":   appVersion,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"panic":     panicMsg,
		"stack":     string(stack),
	}

	// Log the crash at ERROR level so it appears in the rotated log file.
	if logger.L != nil {
		logger.L.Error("crash detected", "panic", panicMsg, "stack", string(stack))
	}

	// Write a separate JSON crash file alongside the log files for easy
	// collection and future upload to a crash-reporting backend.
	if logger.Dir != "" {
		name := fmt.Sprintf("crash_%s.json", time.Now().UTC().Format("20060102T150405Z"))
		path := filepath.Join(logger.Dir, name)
		if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
			_ = os.WriteFile(path, data, 0o644)
		}
	}

	// TODO: send to a remote crash-reporting backend.
	//
	// Candidate services (self-hostable or SaaS):
	//   - Sentry   https://sentry.io    (go get github.com/getsentry/sentry-go)
	//   - Bugsnag  https://bugsnag.com  (go get github.com/bugsnag/bugsnag-go)
	//   - Custom HTTP endpoint
	//
	// Example with Sentry:
	//   sentry.Init(sentry.ClientOptions{Dsn: "https://<key>@sentry.io/<id>"})
	//   sentry.CurrentHub().Recover(panicMsg)
	//   sentry.Flush(2 * time.Second)
	//
	// Example with a custom HTTP endpoint:
	//   go sendRemote(payload)
}

// sendRemote is the placeholder for delivering the crash payload to a remote
// backend. Implement this function when a crash-reporting service is chosen.
//
//nolint:unused,deadcode
func sendRemote(_ map[string]any) {
	// TODO: implement remote crash report delivery.
}
