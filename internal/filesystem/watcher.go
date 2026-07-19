// SPDX-License-Identifier: GPL-3.0-or-later

package filesystem

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"thaw/internal/logger"

	"github.com/rjeczalik/notify"
)

// FSChangeEvent is emitted to the frontend when files in a directory change.
type FSChangeEvent struct {
	Dir string `json:"dir"`
}

// WatchOptions carries user-tunable resource controls into a Watcher. The zero
// value is valid and means "no exclusions, no directory cap" — the historical
// behavior. See config.FileWatchConfig for the persisted, user-facing form.
type WatchOptions struct {
	// ExcludeGlobs are glob patterns; a change is dropped when any tree-relative
	// path component matches a pattern, or when the whole tree-relative path
	// matches (so "node_modules" excludes any node_modules dir at any depth, and
	// ".git/objects" excludes that specific subtree). Patterns use filepath.Match
	// syntax; an invalid pattern is ignored.
	ExcludeGlobs []string
	// MaxWatchedDirs caps the number of distinct directories the watcher will
	// emit events for. 0 = unlimited. Beyond the cap, changes in not-yet-seen
	// directories are ignored; already-tracked directories keep working.
	MaxWatchedDirs int
}

// Watcher monitors a directory tree for file system changes and emits
// debounced per-directory events via the provided callback.
type Watcher struct {
	events    chan notify.EventInfo
	root      string // path as supplied by the caller; the emitted namespace
	watchRoot string // symlink-resolved root that the backend reports paths under
	emit      func(FSChangeEvent)
	opts      WatchOptions
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	mu        sync.Mutex
	timers    map[string]*time.Timer // per-directory debounce timers
	seenDirs  map[string]struct{}    // distinct directories emitted for (MaxWatchedDirs cap)
	capLogged bool                   // one-shot guard so the cap warning is logged once
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
//
// opts carries user-tunable resource controls (exclude globs, a distinct-dir
// cap); the zero value keeps the historical no-exclusion behavior. Because the
// backend is a single recursive watch, exclusions and the cap are applied by
// dropping events after the fact rather than by declining to install a watch.
func NewWatcher(dir string, opts WatchOptions, emit func(FSChangeEvent)) (*Watcher, error) {
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
		opts:      opts,
		stopCh:    make(chan struct{}),
		timers:    make(map[string]*time.Timer),
		seenDirs:  make(map[string]struct{}),
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

	// Drop events under user-configured exclusion globs (node_modules, build,
	// …). The recursive backend still delivers them, but we never re-list or
	// echo them to the frontend.
	if w.excluded(rel) {
		return
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

// excluded reports whether a tree-relative path matches any configured exclude
// glob. Two matching modes, chosen per pattern:
//
//   - A single-name pattern (no separator, e.g. "node_modules" or "*.dist-info")
//     matches when it matches ANY path component at any depth — so a
//     node_modules directory anywhere in the tree is excluded, subtree and all.
//   - A multi-segment pattern (contains a separator, e.g. ".git/objects")
//     matches when it matches any ANCESTOR path prefix — so it excludes the
//     whole subtree rooted at that path. Forward slashes in the pattern are
//     normalized to the OS separator so users can always write "a/b".
//
// Patterns use filepath.Match glob syntax; a pattern with invalid syntax is
// skipped rather than treated as an error.
func (w *Watcher) excluded(rel string) bool {
	if len(w.opts.ExcludeGlobs) == 0 {
		return false
	}
	sep := string(filepath.Separator)
	parts := strings.Split(rel, sep)
	for _, pat := range w.opts.ExcludeGlobs {
		if strings.ContainsAny(pat, `/\`) {
			// Multi-segment: match against every ancestor path prefix.
			p := filepath.FromSlash(pat)
			prefix := ""
			for i, part := range parts {
				if i == 0 {
					prefix = part
				} else {
					prefix += sep + part
				}
				if ok, err := filepath.Match(p, prefix); err == nil && ok {
					return true
				}
			}
			continue
		}
		// Single-name: match any component at any depth.
		for _, part := range parts {
			if ok, err := filepath.Match(pat, part); err == nil && ok {
				return true
			}
		}
	}
	return false
}

// scheduleEmit debounces change events per directory.
func (w *Watcher) scheduleEmit(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.timers == nil {
		return // already closed
	}

	// Enforce the distinct-directory cap. A directory already being tracked (has
	// a live timer or was seen before) always passes; a brand-new directory is
	// dropped once the cap is reached, bounding the timer map and re-list churn.
	if limit := w.opts.MaxWatchedDirs; limit > 0 {
		if _, seen := w.seenDirs[dir]; !seen {
			if len(w.seenDirs) >= limit {
				if !w.capLogged {
					w.capLogged = true
					logger.L.Warn("file watcher: reached max watched directories cap; ignoring changes in new directories",
						"cap", limit, "root", w.root)
				}
				return
			}
			w.seenDirs[dir] = struct{}{}
		}
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
