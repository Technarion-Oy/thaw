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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"thaw/internal/logger"
)

// FSChangeEvent is emitted to the frontend when files in a directory change.
type FSChangeEvent struct {
	Dir string `json:"dir"`
}

// Watcher monitors a directory tree for file system changes and emits
// debounced per-directory events via the provided callback.
type Watcher struct {
	watcher   *fsnotify.Watcher
	root      string
	emit      func(FSChangeEvent)
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	mu     sync.Mutex
	timers map[string]*time.Timer // per-directory debounce timers
}

const debounceDelay = 200 * time.Millisecond

// NewWatcher creates a file system watcher rooted at dir. It recursively
// watches all non-hidden subdirectories and calls emit with debounced
// change events. The caller must call Close to release resources.
func NewWatcher(dir string, emit func(FSChangeEvent)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fsw,
		root:    dir,
		emit:    emit,
		stopCh:  make(chan struct{}),
		timers:  make(map[string]*time.Timer),
	}

	// Recursively add all non-hidden directories.
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if path != dir && isHidden(d.Name()) {
				return filepath.SkipDir
			}
			if addErr := fsw.Add(path); addErr != nil {
				// Non-fatal — on Linux this typically means the inotify watch limit is hit.
				logger.L.Warn("file watcher: failed to watch directory", "path", path, "error", addErr)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		fsw.Close() //nolint:errcheck
		return nil, err
	}

	w.wg.Add(1)
	go w.run()

	return w, nil
}

// Close stops the watcher and releases all resources. It is safe to call
// multiple times.
func (w *Watcher) Close() {
	w.closeOnce.Do(func() {
		close(w.stopCh)
		w.wg.Wait()

		w.mu.Lock()
		for _, t := range w.timers {
			t.Stop()
		}
		w.timers = nil
		w.mu.Unlock()

		w.watcher.Close() //nolint:errcheck
	})
}

// run is the main event loop goroutine.
func (w *Watcher) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return

		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.L.Warn("file watcher error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(ev fsnotify.Event) {
	name := filepath.Base(ev.Name)
	if isHidden(name) {
		return
	}

	// For new directories, start watching them recursively.
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			w.addRecursive(ev.Name)
		}
	}

	// Clean up watches for removed paths. Remove is a no-op for non-watched
	// paths, so calling it unconditionally is safe and avoids tracking which
	// paths are directories.
	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		w.watcher.Remove(ev.Name) //nolint:errcheck
	}

	// Determine the parent directory that changed.
	parentDir := filepath.Dir(ev.Name)

	w.scheduleEmit(parentDir)
}

// addRecursive adds a newly created directory and all its non-hidden
// subdirectories to the watch list.
func (w *Watcher) addRecursive(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && isHidden(d.Name()) {
				return filepath.SkipDir
			}
			if addErr := w.watcher.Add(path); addErr != nil {
				logger.L.Warn("file watcher: failed to watch new directory", "path", path, "error", addErr)
			}
		}
		return nil
	})
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
