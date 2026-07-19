// SPDX-License-Identifier: GPL-3.0-or-later

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

// L is the application-wide structured logger. Init installs the real
// file-backed handler; until then (and in tests that don't call Init) it defaults
// to a no-op logger that discards output, so logging is always safe to call and
// never nil-panics. Safe to use from multiple goroutines.
var L = slog.New(slog.NewTextHandler(io.Discard, nil))

// Dir is the directory where log files are written. Set by Init; other
// packages (e.g. crashreport) may use it to co-locate related files.
var Dir string

// Path is the absolute path to the active log file. Set by Init; used by the
// "Reveal Log File" action and the logging-preferences UI.
var Path string

// levelVar backs the active handler's minimum level so it can be changed at
// runtime (from LogPrefs) via SetLevel without rebuilding the handler.
var levelVar slog.LevelVar

// SetLevel changes the minimum severity written to the log file at runtime.
// name is one of "debug", "info", "warn", "error"; an empty or unrecognized
// value leaves the current level unchanged (so the build default is kept when
// the user has expressed no preference).
func SetLevel(name string) {
	switch name {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "info":
		levelVar.Set(slog.LevelInfo)
	case "warn":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	}
}

// Init sets up file-based logging with rotation and returns a cleanup function
// that flushes and closes the log file. The caller should defer the cleanup.
//
// Dev builds (//go:build dev) log to ./logs/thaw.log and additionally write
// to stderr. Production builds log to the OS-specific application log directory.
func Init() func() {
	path := logFilePath()
	Dir = filepath.Dir(path)
	Path = path

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

	// Seed the runtime-adjustable level with the build default. LogPrefs may
	// override it later via SetLevel once config is loaded at startup.
	if devMode {
		levelVar.Set(slog.LevelDebug)
	} else {
		levelVar.Set(slog.LevelInfo)
	}

	handler := slog.NewTextHandler(appWriter, &slog.HandlerOptions{
		Level:     &levelVar,
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
// cancellation, result-row truncation (50 k cap), or per-connection auth
// retries.  These messages are not actionable from the application's
// perspective and would otherwise mislead users scanning the terminal output.
type driverNoiseFilter struct{ inner slog.Handler }

func (f *driverNoiseFilter) Enabled(ctx context.Context, level slog.Level) bool {
	return f.inner.Enabled(ctx, level)
}

// driverAuthNoise are ERROR message prefixes the gosnowflake driver logs for
// every failed login handshake (auth.go / driver.go, for *any* authenticator).
// During MFA connection bursts the pool re-auths many connections and each
// failure emits both lines, flooding thaw.log. The real, detailed connect error
// is always surfaced separately — App.Connect logs "connection failed" with the
// underlying SnowflakeError and returns it to the UI — so these bare,
// reason-less driver lines add only noise. See issue #804.
var driverAuthNoise = []string{
	"Authentication FAILED",
	"Failed to authenticate. Connection failed after",
}

// SerializedLoginLogKey is the log attribute the snowflake package tags onto the
// context of a serialized single-use-credential login (MFA, or password auth
// with a TOTP passcode). The gosnowflake driver copies registered context keys
// onto its log records (via SetLogKeys/WithContext), so this attribute is present
// exactly on the driver log records emitted while such a login is in progress —
// letting the filter drop that login's expected auth-failure churn per-connection
// rather than via a process-global flag. See internal/snowflake connectBase.
const SerializedLoginLogKey = "THAW_SERIALIZED_LOGIN"

// recordHasAttr reports whether r carries an attribute with the given key.
func recordHasAttr(r slog.Record, key string) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			found = true
			return false
		}
		return true
	})
	return found
}

func (f *driverNoiseFilter) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		// Suppress Arrow chunk download errors that arise when rows.Close() is
		// called asynchronously after a cancellation or the 50k row cap is hit.
		// The driver logs these at ERROR even though they are expected and harmless.
		if strings.Contains(r.Message, "failed to extract HTTP response body") {
			return nil
		}
		// Auth-failure lines are dropped only when the record is tagged as a
		// serialized single-use-credential login (the expected re-auth churn) —
		// scoped to that specific login, so genuine connect failures for other
		// authenticators, other connections, or outside such a login keep their
		// trace in thaw.log.
		if recordHasAttr(r, SerializedLoginLogKey) {
			for _, prefix := range driverAuthNoise {
				if strings.HasPrefix(r.Message, prefix) {
					return nil
				}
			}
		}
	}
	return f.inner.Handle(ctx, r)
}

func (f *driverNoiseFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &driverNoiseFilter{inner: f.inner.WithAttrs(attrs)}
}

func (f *driverNoiseFilter) WithGroup(name string) slog.Handler {
	return &driverNoiseFilter{inner: f.inner.WithGroup(name)}
}
