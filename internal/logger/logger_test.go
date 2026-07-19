// SPDX-License-Identifier: GPL-3.0-or-later

package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/lumberjack.v2"
)

func TestSetLevel(t *testing.T) {
	// Restore the level after the test so it can't leak into other tests.
	orig := levelVar.Level()
	defer levelVar.Set(orig)

	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for name, want := range cases {
		SetLevel(name)
		if got := levelVar.Level(); got != want {
			t.Errorf("SetLevel(%q) => level %v, want %v", name, got, want)
		}
	}

	// An empty or unrecognized value leaves the current level unchanged.
	SetLevel("error")
	SetLevel("")
	if got := levelVar.Level(); got != slog.LevelError {
		t.Errorf(`SetLevel("") changed level to %v, want it unchanged at %v`, got, slog.LevelError)
	}
	SetLevel("bogus")
	if got := levelVar.Level(); got != slog.LevelError {
		t.Errorf("SetLevel(bogus) changed level to %v, want it unchanged", got)
	}

	// The level actually gates a handler wired to levelVar.
	SetLevel("warn")
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &levelVar})
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("INFO should be suppressed when level is warn")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("ERROR should pass when level is warn")
	}
}

func TestParseSlogTime(t *testing.T) {
	want := time.Date(2026, 7, 14, 13, 45, 0, 0, time.UTC)
	cases := []struct {
		name string
		line string
		ok   bool
	}{
		{"millis", `time=2026-07-14T13:45:00.000Z level=INFO msg="hello"`, true},
		{"nanos", `time=2026-07-14T13:45:00.000000000Z level=INFO msg=x`, true},
		{"offset", `time=2026-07-14T16:45:00.000+03:00 level=INFO msg=x`, true},
		{"no seconds fraction", `time=2026-07-14T13:45:00Z level=INFO`, true},
		{"no time key", `level=INFO msg="no timestamp here"`, false},
		{"garbage", `time=not-a-timestamp level=INFO`, false},
		{"empty", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseSlogTime(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && !got.Equal(want) {
				t.Fatalf("time = %v, want %v", got.UTC(), want)
			}
		})
	}
}

func TestFirstEntryTime_Missing(t *testing.T) {
	if _, ok := firstEntryTime(filepath.Join(t.TempDir(), "nope.log")); ok {
		t.Fatal("expected ok=false for missing file")
	}
}

func TestFirstEntryTime_ReadsOldestLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thaw.log")
	content := "time=2026-01-01T00:00:00.000Z level=INFO msg=oldest\n" +
		"time=2026-06-01T00:00:00.000Z level=INFO msg=newer\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := firstEntryTime(path)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("time = %v, want %v", got.UTC(), want)
	}
}

func countBackups(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.Name() != "thaw.log" {
			n++
		}
	}
	return n
}

func TestMaybeRotateByAge_RotatesStaleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thaw.log")
	// Oldest entry is ~2 days old — older than rotationInterval.
	old := time.Now().Add(-2 * rotationInterval).UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(path, []byte("time="+old+" level=INFO msg=stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rot := &lumberjack.Logger{Filename: path, MaxSize: maxSizeMB, MaxAge: maxAgeDays}
	maybeRotateByAge(rot, path)
	_ = rot.Close()

	if n := countBackups(t, dir); n != 1 {
		t.Fatalf("expected 1 rotated backup, got %d", n)
	}
}

func TestMaybeRotateByAge_KeepsFreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thaw.log")
	recent := time.Now().UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(path, []byte("time="+recent+" level=INFO msg=fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rot := &lumberjack.Logger{Filename: path, MaxSize: maxSizeMB, MaxAge: maxAgeDays}
	maybeRotateByAge(rot, path)
	_ = rot.Close()

	if n := countBackups(t, dir); n != 0 {
		t.Fatalf("expected no rotation, got %d backups", n)
	}
}

func TestMaybeRotateByAge_NoFileNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thaw.log")
	rot := &lumberjack.Logger{Filename: path, MaxSize: maxSizeMB, MaxAge: maxAgeDays}
	maybeRotateByAge(rot, path)
	_ = rot.Close()

	if n := countBackups(t, dir); n != 0 {
		t.Fatalf("expected no backups for missing file, got %d", n)
	}
}

// capturingHandler records the messages it is asked to emit, so a test can
// assert which records driverNoiseFilter passes through versus suppresses.
type capturingHandler struct{ msgs []string }

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.msgs = append(h.msgs, r.Message)
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func TestDriverNoiseFilter(t *testing.T) {
	tests := []struct {
		name    string
		level   slog.Level
		msg     string
		tagged  bool // record carries the serialized-login attr
		wantOut bool
	}{
		// Auth-failure lines are suppressed ONLY when the record is tagged as a
		// serialized single-use-credential login (MFA / password+TOTP).
		{"auth failed suppressed when tagged", slog.LevelError, "Authentication FAILED", true, false},
		{"failed to authenticate suppressed when tagged", slog.LevelError, "Failed to authenticate. Connection failed after 252ms milliseconds", true, false},
		// Untagged (e.g. password/Okta/browser auth, or a different connection)
		// they pass through so genuine connect failures keep a trace — the core
		// review finding.
		{"auth failed passes through when untagged", slog.LevelError, "Authentication FAILED", false, true},
		{"failed to authenticate passes through when untagged", slog.LevelError, "Failed to authenticate. Connection failed after 252ms milliseconds", false, true},
		// Arrow chunk noise is always suppressed (unrelated to auth).
		{"arrow chunk noise suppressed (tagged)", slog.LevelError, "failed to extract HTTP response body", true, false},
		{"arrow chunk noise suppressed (untagged)", slog.LevelError, "failed to extract HTTP response body", false, false},
		{"real error passes through", slog.LevelError, "connection failed", true, true},
		// The same text at a non-error level (defensive) is not treated as driver noise.
		{"auth phrase at info passes", slog.LevelInfo, "Authentication FAILED", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := &capturingHandler{}
			f := &driverNoiseFilter{inner: cap}
			r := slog.NewRecord(time.Time{}, tt.level, tt.msg, 0)
			if tt.tagged {
				r.AddAttrs(slog.String(SerializedLoginLogKey, "1"))
			}
			if err := f.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			got := len(cap.msgs) == 1
			if got != tt.wantOut {
				t.Errorf("emitted=%v, want %v (msg %q)", got, tt.wantOut, tt.msg)
			}
		})
	}
}
