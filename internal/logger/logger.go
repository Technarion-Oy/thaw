// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/lumberjack.v2"
)

// L is the application-wide structured logger. It is initialised by Init and
// safe to use from multiple goroutines.
var L *slog.Logger

// Dir is the directory where log files are written. Set by Init; other
// packages (e.g. crashreport) may use it to co-locate related files.
var Dir string

// Init sets up file-based logging with rotation and returns a cleanup function
// that flushes and closes the log file. The caller should defer the cleanup.
//
// Dev builds (//go:build dev) log to ./logs/thaw.log and additionally write
// to stderr. Production builds log to the OS-specific application log directory.
func Init() func() {
	path := logFilePath()
	Dir = filepath.Dir(path)

	rot := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10,   // MB per file before rotation
		MaxBackups: 5,    // number of old files to retain
		MaxAge:     30,   // days to retain old files
		Compress:   true, // gzip old files
	}

	// In dev mode also echo to stderr; in production write to file only.
	var appWriter io.Writer
	if devMode {
		appWriter = io.MultiWriter(rot, os.Stderr)
	} else {
		appWriter = rot
	}

	level := slog.LevelInfo
	if devMode {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(appWriter, &slog.HandlerOptions{
		Level:     level,
		AddSource: devMode,
	})
	L = slog.New(&driverNoiseFilter{inner: handler})
	slog.SetDefault(L)

	// gosnowflake v2 defaults to slog.Default(), which is already set to L
	// above — no explicit redirect needed.

	L.Info("logger initialised", "path", path, "dev", devMode)

	return func() { _ = rot.Close() }
}

// driverNoiseFilter is a slog.Handler wrapper that suppresses known-noisy
// ERROR messages emitted by the gosnowflake driver as side-effects of query
// cancellation or result-row truncation (50 k cap).  These messages are not
// actionable from the application's perspective and would otherwise mislead
// users scanning the terminal output.
type driverNoiseFilter struct{ inner slog.Handler }

func (f *driverNoiseFilter) Enabled(ctx context.Context, level slog.Level) bool {
	return f.inner.Enabled(ctx, level)
}

func (f *driverNoiseFilter) Handle(ctx context.Context, r slog.Record) error {
	// Suppress Arrow chunk download errors that arise when rows.Close() is
	// called asynchronously after a cancellation or the 50k row cap is hit.
	// The driver logs these at ERROR even though they are expected and harmless.
	if r.Level == slog.LevelError &&
		strings.Contains(r.Message, "failed to extract HTTP response body") {
		return nil
	}
	return f.inner.Handle(ctx, r)
}

func (f *driverNoiseFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &driverNoiseFilter{inner: f.inner.WithAttrs(attrs)}
}

func (f *driverNoiseFilter) WithGroup(name string) slog.Handler {
	return &driverNoiseFilter{inner: f.inner.WithGroup(name)}
}
