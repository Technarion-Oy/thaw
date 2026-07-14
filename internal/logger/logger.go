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
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/lumberjack.v2"
)

const (
	// maxSizeMB is the size threshold that triggers lumberjack's built-in
	// size-based rotation. It acts only as a safety valve; day-to-day rotation
	// is driven by age (see rotationInterval).
	maxSizeMB = 10
	// maxAgeDays is the retention window: rotated backups older than this are
	// deleted by lumberjack's cleanup pass.
	maxAgeDays = 30
	// rotationInterval is how often the active log file is rotated so that
	// age-based cleanup has backups to prune. Without this, Thaw's low log
	// volume means the active file rarely reaches maxSizeMB, so nothing ever
	// rotates and >maxAgeDays-old entries live forever in the active file.
	rotationInterval = 24 * time.Hour
)

// L is the application-wide structured logger. It is initialized by Init and
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
		Filename: path,
		MaxSize:  maxSizeMB,  // MB per file before size-based rotation (safety valve)
		MaxAge:   maxAgeDays, // days to retain rotated backups
		// MaxBackups is intentionally 0 (retain all) so that MaxAge alone
		// governs retention. With daily rotation the backup count is naturally
		// bounded to ~maxAgeDays files.
		Compress: true, // gzip old files
	}

	// lumberjack only rotates on size, and MaxAge/MaxBackups prune rotated
	// backups only — never the active file. Force an age-based rotation on
	// startup when the active file already holds entries older than
	// rotationInterval, so the existing MaxAge cleanup has backups to act on
	// and stale entries stop accumulating in the active file.
	maybeRotateByAge(rot, path)

	// For long-running sessions, keep rotating on a schedule so age-based
	// cleanup continues to run even when the app stays open for days.
	stopTicker := startRotationTicker(rot)

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

	L.Info("logger initialized", "path", path, "dev", devMode)

	return func() {
		stopTicker()
		_ = rot.Close()
	}
}

// maybeRotateByAge rotates the active log file if its oldest entry is older
// than rotationInterval. lumberjack never prunes the active file itself and
// only rotates on size, so without this a low-volume log accumulates entries
// far past maxAgeDays. After rotating, lumberjack's cleanup deletes backups
// older than maxAgeDays.
func maybeRotateByAge(rot *lumberjack.Logger, path string) {
	oldest, ok := firstEntryTime(path)
	if !ok {
		return
	}
	if time.Since(oldest) > rotationInterval {
		_ = rot.Rotate()
	}
}

// startRotationTicker rotates the log on rotationInterval until the returned
// stop function is called. Each rotation triggers lumberjack's age-based
// cleanup, keeping retention bounded during long-running sessions.
//
// The ticker fires after Init returns, so L is set and rotation failures
// (disk full, permissions, log dir removed underfoot) are surfaced rather
// than lost — otherwise a stuck rotation would silently defeat retention.
//
// The goroutine's fire-on-tick / stop-cleanly lifecycle is not unit tested:
// it would require a mockable clock, and adding one just for this isn't worth
// re-introducing a time seam.
func startRotationTicker(rot *lumberjack.Logger) func() {
	ticker := time.NewTicker(rotationInterval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := rot.Rotate(); err != nil {
					L.Warn("log rotation failed", "err", err)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(done) }
}

// firstEntryTime returns the timestamp of the first entry in the log file at
// path. Entries are written by slog's TextHandler, which prefixes each line
// with `time=<RFC3339>`. Returns ok=false if the file is missing, empty, or
// the first line has no parseable timestamp.
func firstEntryTime(path string) (time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	// Raise the line-length cap well above bufio.Scanner's 64KB default so an
	// unusually verbose first entry doesn't make Scan fail silently and skip
	// age-based rotation (only the leading `time=` token is needed regardless).
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return time.Time{}, false
	}
	return parseSlogTime(sc.Text())
}

// parseSlogTime extracts the timestamp from a slog TextHandler line of the form
// `time=2026-07-14T13:45:00.000+03:00 level=INFO msg=...`.
func parseSlogTime(line string) (time.Time, bool) {
	_, val, found := strings.Cut(line, "time=")
	if !found {
		return time.Time{}, false
	}
	// slog leaves RFC3339 timestamps unquoted; the value ends at the next space.
	if sp := strings.IndexByte(val, ' '); sp >= 0 {
		val = val[:sp]
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, val); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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
