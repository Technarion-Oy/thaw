// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package filesystem

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
)

// FSChangeEvent is emitted to the frontend when files in a directory change.
type FSChangeEvent struct {
	Dir string `json:"dir"`
}

// Watcher monitors a directory tree for file system changes and emits
// debounced per-directory events via the provided callback.
type Watcher struct {
	events    chan notify.EventInfo
	root      string // path as supplied by the caller; the emitted namespace
	watchRoot string // symlink-resolved root that the backend reports paths under
	emit      func(FSChangeEvent)
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	mu     sync.Mutex
	timers map[string]*time.Timer // per-directory debounce timers
}

const debounceDelay = 200 * time.Millisecond

// eventBufferSize is the capacity of the channel buffering events from the
// notify library. A generous buffer absorbs the burst that arrives when a
// large tree is first touched, so events are not dropped while debouncing.
const eventBufferSize = 1024

// NewWatcher creates a file system watcher rooted at dir and calls emit with
// debounced change events. The caller must call Close to release resources.
//
// It installs a single recursive watch on the whole tree. On macOS this maps
// to one FSEvents stream covering the entire subtree (no per-directory file
// descriptor, so deep dependency trees such as venv/node_modules no longer
// exhaust the process FD limit), on Windows to a recursive
// ReadDirectoryChangesW handle, and on Linux to inotify (still per-directory,
// managed internally by the library). Events inside hidden directories are
// filtered out after the fact rather than excluded from the watch.
func NewWatcher(dir string, emit func(FSChangeEvent)) (*Watcher, error) {
	// Buffered so the library's internal goroutine never blocks on us.
	events := make(chan notify.EventInfo, eventBufferSize)

	// macOS FSEvents reports canonical paths (e.g. /private/var/… for /var/…),
	// so resolve the root up front to keep event paths comparable to it.
	watchRoot := dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		watchRoot = resolved
	}

	// The "/..." suffix requests a recursive watch of the entire subtree.
	if err := notify.Watch(filepath.Join(watchRoot, "..."), events, notify.All); err != nil {
		return nil, err
	}

	w := &Watcher{
		events:    events,
		root:      dir,
		watchRoot: watchRoot,
		emit:      emit,
		stopCh:    make(chan struct{}),
		timers:    make(map[string]*time.Timer),
	}

	w.wg.Add(1)
	go w.run()

	return w, nil
}

// Close stops the watcher and releases all resources. It is safe to call
// multiple times.
func (w *Watcher) Close() {
	w.closeOnce.Do(func() {
		notify.Stop(w.events)
		close(w.stopCh)
		w.wg.Wait()

		w.mu.Lock()
		for _, t := range w.timers {
			t.Stop()
		}
		w.timers = nil
		w.mu.Unlock()
	})
}

// run is the main event loop goroutine.
func (w *Watcher) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return
		case ev := <-w.events:
			w.handleEvent(ev)
		}
	}
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(ev notify.EventInfo) {
	rel, err := filepath.Rel(w.watchRoot, ev.Path())
	if err != nil {
		return
	}

	// The recursive watch reports events for the whole tree, so filter here:
	// drop anything outside the root and anything within a hidden directory or
	// a hidden file (a path component starting with a dot).
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == ".." {
			return // outside the watched tree
		}
		if isHidden(part) {
			return // hidden file or directory (also covers the root itself, ".")
		}
	}

	// Map the canonical path back into the caller's namespace and emit for the
	// parent directory that changed. Write events are included so open editor
	// tabs can re-read changed files; they coalesce by directory like any other
	// change.
	// ponytail: re-listing the dir on a pure content change is mildly redundant
	// (listing is unchanged), but it's debounced and one IPC — not worth a
	// per-file event channel to skip it.
	w.scheduleEmit(filepath.Dir(filepath.Join(w.root, rel)))
}

// scheduleEmit debounces change events per directory.
func (w *Watcher) scheduleEmit(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.timers == nil {
		return // already closed
	}

	if t, ok := w.timers[dir]; ok {
		t.Stop()
	}

	w.timers[dir] = time.AfterFunc(debounceDelay, func() {
		w.mu.Lock()
		delete(w.timers, dir)
		w.mu.Unlock()

		select {
		case <-w.stopCh:
			return
		default:
		}
		w.emit(FSChangeEvent{Dir: dir})
	})
}

// isHidden returns true if the file or directory name starts with a dot.
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
