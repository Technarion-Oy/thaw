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
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/lumberjack.v2"
)

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
