// SPDX-License-Identifier: GPL-3.0-or-later

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// waitFor polls a condition with a timeout, returning true if it was met.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestWatcher_LifecycleAndClose(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWatcher(dir, WatchOptions{}, func(FSChangeEvent) {})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	w.Close()
	// Double-close must not panic.
	w.Close()
}

func TestWatcher_EmitsOnFileCreate(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(dir, WatchOptions{}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	// Create a file to trigger an event.
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	})
	if !ok {
		t.Fatal("expected at least one event after file creation, got none")
	}

	mu.Lock()
	defer mu.Unlock()
	if events[0].Dir != dir {
		t.Errorf("expected event dir %q, got %q", dir, events[0].Dir)
	}
}

func TestWatcher_DebounceCoalescing(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(dir, WatchOptions{}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	// Create 10 files as fast as possible — all writes target the same
	// directory so they should coalesce into far fewer debounced events.
	for i := 0; i < 10; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for at least one debounced event to arrive.
	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	})
	if !ok {
		t.Fatal("expected at least one debounced event, got 0")
	}

	// Allow any remaining timers to settle.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := len(events)
	mu.Unlock()

	// Even on a loaded CI machine the 10 writes should coalesce into
	// fewer than 10 events. We use a generous threshold to avoid flakiness.
	if count >= 10 {
		t.Errorf("expected debounce coalescing (< 10 events for 10 writes), got %d", count)
	}
}

func TestWatcher_HiddenDirectoryExcluded(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(dir, WatchOptions{}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	// Write inside the hidden directory — should NOT trigger an event.
	if err := os.WriteFile(filepath.Join(hidden, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := len(events)
	mu.Unlock()

	if count > 0 {
		t.Errorf("expected no events for hidden directory, got %d", count)
	}
}

func TestWatcher_NewDirectoryAutoWatched(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(dir, WatchOptions{}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	// Create a new subdirectory.
	sub := filepath.Join(dir, "newdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Wait for the watcher to pick up the new directory.
	time.Sleep(400 * time.Millisecond)

	// Clear prior events from the directory creation.
	mu.Lock()
	events = nil
	mu.Unlock()

	// Create a file inside the new subdirectory.
	if err := os.WriteFile(filepath.Join(sub, "inner.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Dir == sub {
				return true
			}
		}
		return false
	})
	if !ok {
		mu.Lock()
		t.Errorf("expected event for new subdirectory %q, got events: %v", sub, events)
		mu.Unlock()
	}
}

// TestWatcher_DeepTreeNoFDExhaustion is the regression test for issue #485:
// opening a folder with a large/deep tree (e.g. a venv or node_modules) used to
// register one kqueue watch per directory and exhaust the macOS FD limit
// (commonly 256). With a single recursive watch this must succeed and still
// detect changes deep in the tree.
func TestWatcher_DeepTreeNoFDExhaustion(t *testing.T) {
	root := t.TempDir()

	// Create many sibling directories — comfortably above the macOS default
	// soft FD limit of 256 — plus a deeper nested path.
	const numDirs = 400
	for i := 0; i < numDirs; i++ {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("d%03d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deep := filepath.Join(root, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(root, WatchOptions{}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher on deep tree: %v", err)
	}
	defer w.Close()

	// A change deep in the tree must still be reported.
	if err := os.WriteFile(filepath.Join(deep, "inner.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Dir == deep {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Errorf("expected event for deep directory %q", deep)
	}
}

func TestWatcher_ExcludeGlobsDropsEvents(t *testing.T) {
	dir := t.TempDir()
	excluded := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(excluded, 0o755); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(dir, "src")
	if err := os.MkdirAll(included, 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []FSChangeEvent

	w, err := NewWatcher(dir, WatchOptions{ExcludeGlobs: []string{"node_modules"}}, func(evt FSChangeEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	// Drain any historical events FSEvents replays for the pre-start mkdirs, then
	// start from a clean slate so the assertions only see our post-start writes.
	time.Sleep(400 * time.Millisecond)
	mu.Lock()
	events = nil
	mu.Unlock()

	// A write under the excluded directory must NOT emit an event for that
	// subtree. (We assert on the excluded path specifically rather than "zero
	// events" because FSEvents can still replay unrelated historical root-level
	// noise from the pre-start mkdirs, especially under -race's slowdown.)
	if err := os.WriteFile(filepath.Join(excluded, "index.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	for _, e := range events {
		if strings.Contains(e.Dir, "node_modules") {
			t.Errorf("expected no events under excluded node_modules, got %q", e.Dir)
		}
	}
	mu.Unlock()

	// A write under a non-excluded directory must still emit.
	if err := os.WriteFile(filepath.Join(included, "main.go"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Dir == included {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Error("expected an event for the non-excluded src directory")
	}
}

func TestWatcherExcluded(t *testing.T) {
	w := &Watcher{opts: WatchOptions{ExcludeGlobs: []string{
		"node_modules", "target", ".git/objects", "*.dist-info",
	}}}
	sep := string(filepath.Separator)
	tests := []struct {
		rel  string
		want bool
	}{
		{"node_modules" + sep + "pkg" + sep + "index.js", true}, // component match at depth
		{"target", true}, // component match at root
		{"src" + sep + "target" + sep + "out", true},  // component match deeper
		{".git" + sep + "objects" + sep + "ab", true}, // whole-path prefix pattern
		{"foo.dist-info" + sep + "METADATA", true},    // glob component match
		{"src" + sep + "main.go", false},              // no match
		{"targeted" + sep + "x", false},               // must not partial-match "target"
	}
	for _, tc := range tests {
		if got := w.excluded(tc.rel); got != tc.want {
			t.Errorf("excluded(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}

	// No configured globs → nothing excluded.
	none := &Watcher{}
	if none.excluded("node_modules" + sep + "x") {
		t.Error("excluded should be false when no globs are configured")
	}
}

func TestWatcher_MaxWatchedDirsCap(t *testing.T) {
	// With a cap of 1, the first directory emitted for is tracked; a second,
	// distinct directory is dropped. Drive scheduleEmit directly to keep the
	// assertion deterministic (no dependence on OS event ordering).
	var mu sync.Mutex
	var emitted []string
	w := &Watcher{
		emit:     func(e FSChangeEvent) { mu.Lock(); emitted = append(emitted, e.Dir); mu.Unlock() },
		stopCh:   make(chan struct{}),
		timers:   make(map[string]*time.Timer),
		seenDirs: make(map[string]struct{}),
		opts:     WatchOptions{MaxWatchedDirs: 1},
	}
	w.scheduleEmit("/a") // first distinct dir → tracked
	w.scheduleEmit("/b") // second distinct dir → dropped by cap
	w.scheduleEmit("/a") // already tracked → still allowed

	ok := waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(emitted) >= 1
	})
	if !ok {
		t.Fatal("expected at least one debounced emit")
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, d := range emitted {
		if d == "/b" {
			t.Errorf("directory /b should have been dropped by the cap, but was emitted; emitted=%v", emitted)
		}
	}
	if len(emitted) == 0 {
		t.Error("expected /a to be emitted")
	}
}

func TestIsHidden(t *testing.T) {
	tests := []struct {
		name   string
		hidden bool
	}{
		{".git", true},
		{".DS_Store", true},
		{"visible", false},
		{"file.txt", false},
		{".", true},
		{"..", true},
	}
	for _, tc := range tests {
		if got := isHidden(tc.name); got != tc.hidden {
			t.Errorf("isHidden(%q) = %v, want %v", tc.name, got, tc.hidden)
		}
	}
}
