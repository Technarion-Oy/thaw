// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"thaw/internal/config"
	"thaw/internal/filesystem"
	"thaw/internal/logger"
)

// StartFileWatcher starts watching the given directory tree for changes.
// Any existing watcher is stopped first. Change events are emitted to
// the frontend as "fs:changed" Wails events.
//
// User-configurable watcher controls (exclude globs, a distinct-directory cap,
// optional FD-limit raising) are read from config.FileWatchConfig and applied
// here. Failure to establish the watch is treated as non-fatal: the folder still
// opens and works, only the auto-refresh-on-external-change convenience is lost.
// The error is logged and swallowed so the frontend never blocks folder opening
// on a watcher that a network drive or FD ceiling prevented from starting.
func (a *App) StartFileWatcher(dir string) error {
	a.fsWatcherMu.Lock()
	defer a.fsWatcherMu.Unlock()

	if a.fsWatcher != nil {
		a.fsWatcher.Close()
		a.fsWatcher = nil
	}

	fw := config.DefaultFileWatchConfig()
	if cfg, err := config.Load(); err == nil {
		fw = config.FileWatchConfigWithDefaults(cfg.FileWatch)
	}

	// Opt-in FD soft-limit raise, before installing the watch so the higher
	// ceiling is in effect for it (and any FD-hungry work that follows).
	if fw.RaiseFDLimit {
		if soft, hard, err := filesystem.RaiseFDLimit(); err != nil {
			logger.L.Warn("file watcher: failed to raise FD soft limit", "err", err)
		} else {
			logger.L.Info("file watcher: raised FD soft limit", "soft", soft, "hard", hard)
		}
	}

	w, err := filesystem.NewWatcher(dir, filesystem.WatchOptions{
		ExcludeGlobs:   fw.ExcludeGlobs,
		MaxWatchedDirs: fw.MaxWatchedDirs,
	}, func(evt filesystem.FSChangeEvent) {
		a.emit("fs:changed", evt)
	})
	if err != nil {
		// Non-fatal: log and continue so the folder still opens.
		logger.L.Warn("file watcher: could not start; external-change auto-refresh disabled", "dir", dir, "err", err)
		return nil
	}
	a.fsWatcher = w
	return nil
}

// StopFileWatcher stops the current file system watcher, if any.
func (a *App) StopFileWatcher() {
	a.fsWatcherMu.Lock()
	defer a.fsWatcherMu.Unlock()

	if a.fsWatcher != nil {
		a.fsWatcher.Close()
		a.fsWatcher = nil
	}
}

// ListDirectory returns the direct children of path (dirs first, then files).
func (a *App) ListDirectory(path string) ([]filesystem.FileEntry, error) {
	return filesystem.ListDir(path)
}

// ReadFile returns the text content of the file at path.
func (a *App) ReadFile(path string) (string, error) {
	return filesystem.ReadFile(path)
}

// ReadFileHead returns the first maxBytes bytes of the file at path.
// It is intended for lightweight file previews and is safe to call on large files.
func (a *App) ReadFileHead(path string, maxBytes int) (string, error) {
	return filesystem.ReadFileHead(path, maxBytes)
}

// SaveFile writes content to path, creating parent directories as needed.
func (a *App) SaveFile(path, content string) error {
	return filesystem.WriteFile(path, content)
}

// SearchFiles walks dir recursively and returns lines matching query.
// If useRegex is true, query is treated as a regular expression;
// otherwise a case-insensitive substring search is performed.
func (a *App) SearchFiles(dir, query string, useRegex bool) ([]filesystem.SearchMatch, error) {
	return filesystem.SearchFiles(dir, query, useRegex)
}

// RevealInFinder opens the platform file manager and selects the given path.
// The path must be inside the configured export directory.
func (a *App) RevealInFinder(path string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.RevealInFinder(path, root)
}

// GetPlatformOS returns the current OS identifier (darwin, windows, linux)
// so the frontend can display platform-appropriate labels.
func (a *App) GetPlatformOS() string {
	return filesystem.RuntimeOS()
}

// DeleteFile removes the file at path. The path must be inside the configured export directory.
func (a *App) DeleteFile(path string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.DeleteFile(path, root)
}

// DeleteDirectory removes the directory at path and all its contents.
// The path must be inside the configured export directory.
func (a *App) DeleteDirectory(path string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.DeleteDirectory(path, root)
}

// RenameFile renames (moves) oldPath to newPath. Both must be inside the export directory.
func (a *App) RenameFile(oldPath, newPath string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.RenameFile(oldPath, newPath, root)
}

// CreateDirectory creates a new directory at path, including any necessary parents.
// The path must be inside the configured export directory.
func (a *App) CreateDirectory(path string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.MkDir(path, root)
}

// CreateFile creates an empty file at path. The path must be inside the export directory.
func (a *App) CreateFile(path string) error {
	root, err := a.exportRoot()
	if err != nil {
		return err
	}
	return filesystem.WriteFileInRoot(path, "", root)
}

// DuplicateFile creates a copy of the file at path in the same directory with a unique name.
// Returns the full path of the new copy.
func (a *App) DuplicateFile(path string) (string, error) {
	root, err := a.exportRoot()
	if err != nil {
		return "", err
	}
	return filesystem.DuplicateFile(path, root)
}

// CopyFile copies srcPath to dstPath (both inside the export directory). srcPath
// may be a file or a directory; dstPath must not already exist. Returns the new path.
func (a *App) CopyFile(srcPath, dstPath string) (string, error) {
	root, err := a.exportRoot()
	if err != nil {
		return "", err
	}
	return filesystem.CopyFile(srcPath, dstPath, root)
}

// exportRoot returns the cached export directory, or an error if not set.
func (a *App) exportRoot() (string, error) {
	a.exportDirMu.RLock()
	dir := a.cachedExportDir
	a.exportDirMu.RUnlock()
	if dir == "" {
		return "", fmt.Errorf("no export directory configured — set one via Git → Export Directory")
	}
	return dir, nil
}

// setExportDir updates the cached export directory under write lock.
func (a *App) setExportDir(dir string) {
	a.exportDirMu.Lock()
	a.cachedExportDir = dir
	a.exportDirMu.Unlock()
}

// currentWorkdir returns this process's working directory for dialog defaults
// (never errors on empty, unlike exportRoot). An override window's folder lives
// only in memory (it's never persisted), so read cachedExportDir there; an ordinary
// window reads fresh from disk so it picks up a folder change made by another
// ordinary window since startup, matching the pre-override behavior.
func (a *App) currentWorkdir() string {
	if a.workdirOverridden {
		a.exportDirMu.RLock()
		defer a.exportDirMu.RUnlock()
		return a.cachedExportDir
	}
	if cfg, err := config.Load(); err == nil {
		return cfg.Git.ExportDir
	}
	return ""
}

// PickOpenFile opens a native open-file dialog filtered to SQL, YAML, Python
// and Markdown files and returns the chosen path, or an empty string if
// canceled. Any text file can still be opened via the "All Files" entry (or
// PickAnyFile). The dialog opens in the configured export directory when set.
func (a *App) PickOpenFile() string {
	defaultDir := a.currentWorkdir()
	path, err := a.openFileDialog(openDialogOpts{
		title:            "Open file",
		defaultDirectory: defaultDir,
		filters: []fileFilter{
			{displayName: "Supported Files (*.sql, *.yml, *.yaml, *.py, *.md)", pattern: "*.sql;*.yml;*.yaml;*.py;*.md;*.markdown"},
			{displayName: "SQL Files (*.sql)", pattern: "*.sql"},
			{displayName: "YAML Files (*.yml, *.yaml)", pattern: "*.yml;*.yaml"},
			{displayName: "Python Files (*.py)", pattern: "*.py"},
			{displayName: "Markdown Files (*.md, *.markdown)", pattern: "*.md;*.markdown"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickAnyFile opens a native open-file dialog with no extension filter and
// returns the chosen path, or "" if canceled. Passing zero filters is the only
// way Wails sets allowsOtherFileTypes on macOS, so this is the explicit opt-in
// for opening files other than SQL/YAML/Python. The dialog opens in the
// configured export directory when one is set.
func (a *App) PickAnyFile() string {
	defaultDir := a.currentWorkdir()
	path, err := a.openFileDialog(openDialogOpts{
		title:            "Open any file",
		defaultDirectory: defaultDir,
	})
	if err != nil {
		return ""
	}
	return path
}

// dataFileFilters returns dialog file filters for the given import format.
func dataFileFilters(format string) []fileFilter {
	switch strings.ToUpper(format) {
	case "JSON":
		return []fileFilter{
			{displayName: "JSON Files (*.json;*.jsonl;*.ndjson)", pattern: "*.json;*.jsonl;*.ndjson"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	case "PARQUET":
		return []fileFilter{
			{displayName: "Parquet Files (*.parquet)", pattern: "*.parquet"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	case "AVRO":
		return []fileFilter{
			{displayName: "Avro Files (*.avro)", pattern: "*.avro"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	case "ORC":
		return []fileFilter{
			{displayName: "ORC Files (*.orc)", pattern: "*.orc"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	default: // CSV
		return []fileFilter{
			{displayName: "CSV Files (*.csv)", pattern: "*.csv"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	}
}

// PickDataFile opens a native open-file dialog filtered to common data file
// formats and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickDataFile() string {
	path, err := a.openFileDialog(openDialogOpts{
		title: "Open data file",
		filters: []fileFilter{
			{displayName: "Data Files (*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc)", pattern: "*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc"},
			{displayName: "CSV Files (*.csv)", pattern: "*.csv"},
			{displayName: "JSON Files (*.json;*.jsonl;*.ndjson)", pattern: "*.json;*.jsonl;*.ndjson"},
			{displayName: "Parquet Files (*.parquet)", pattern: "*.parquet"},
			{displayName: "Avro Files (*.avro)", pattern: "*.avro"},
			{displayName: "ORC Files (*.orc)", pattern: "*.orc"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFileByFormat opens a native open-file dialog filtered to the file
// extensions that match the given format.
func (a *App) PickDataFileByFormat(format string) string {
	path, err := a.openFileDialog(openDialogOpts{
		title:   "Open " + format + " file",
		filters: dataFileFilters(format),
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDataFilesByFormat opens a native open-file dialog (multi-select) filtered
// to the extensions that match the given format. Returns the selected paths, or
// nil if the user cancels.
func (a *App) PickDataFilesByFormat(format string) []string {
	filters := dataFileFilters(format)
	paths, err := a.openMultipleFilesDialog(openDialogOpts{
		title:   "Open " + format + " files",
		filters: filters,
	})
	if err != nil {
		return nil
	}
	return paths
}

// PickSaveFile opens a native save-file dialog pre-populated with defaultName
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickSaveFile(defaultName string) string {
	path, err := a.saveFileDialog(saveDialogOpts{
		title:           "Save file",
		defaultFilename: defaultName,
		filters: []fileFilter{
			{displayName: "SQL Files (*.sql)", pattern: "*.sql"},
			{displayName: "YAML Files (*.yml, *.yaml)", pattern: "*.yml;*.yaml"},
			{displayName: "Python Files (*.py)", pattern: "*.py"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickSaveExportFile opens a native save-file dialog with filters appropriate
// for the requested format ("csv" or "excel") and returns the chosen path, or
// an empty string if the user cancels.
func (a *App) PickSaveExportFile(defaultName, format string) string {
	var filters []fileFilter
	title := "Save export file"
	switch format {
	case "csv":
		title = "Save as CSV"
		filters = []fileFilter{
			{displayName: "CSV Files (*.csv)", pattern: "*.csv"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	case "excel":
		title = "Save as Excel"
		filters = []fileFilter{
			{displayName: "Excel Files (*.xlsx)", pattern: "*.xlsx"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	default:
		filters = []fileFilter{
			{displayName: "All Files (*.*)", pattern: "*.*"},
		}
	}
	path, err := a.saveFileDialog(saveDialogOpts{
		title:           title,
		defaultFilename: defaultName,
		filters:         filters,
	})
	if err != nil {
		return ""
	}
	return path
}

// SaveBinaryFile decodes the base64-encoded content and writes the raw bytes
// to path. Used for binary export formats such as Excel (.xlsx).
func (a *App) SaveBinaryFile(path, base64Content string) error {
	return filesystem.WriteBinaryFile(path, base64Content)
}

// PickPrivateKeyFile opens a native open-file dialog filtered to private-key
// files (*.p8, *.pem, *.key) for Snowflake key-pair authentication and returns
// the chosen path, or "" if the user cancels.
//
// Selecting the key through the native panel matters beyond convenience on
// macOS: a user-driven NSOpenPanel selection grants Thaw path-scoped read
// consent that the OS persists across launches (via the com.apple.macl extended
// attribute on the file). That lets a key kept in a TCC-protected folder such as
// ~/Documents, ~/Desktop, ~/Downloads or iCloud Drive load on later reconnects,
// where a plain os.ReadFile of a hand-typed path is denied with EPERM. See the
// "macOS TCC blocks reads of key files" note in docs/concepts/gotchas.md.
func (a *App) PickPrivateKeyFile() string {
	path, err := a.openFileDialog(openDialogOpts{
		title: "Select private key",
		filters: []fileFilter{
			{displayName: "Private Key Files (*.p8, *.pem, *.key)", pattern: "*.p8;*.pem;*.key"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// PickDirectory opens a native folder-picker dialog and returns the selected path.
// Returns an empty string if the user cancels. The dialog opens in the currently
// opened folder when set (matching PickOpenFile/PickAnyFile); `currentWorkdir`
// returns "" when no folder is open, which lets the OS pick the location as before.
// CanCreateDirectories enables the "New Folder" button so the user can create a
// fresh (empty) destination in place — e.g. a scaffold target for a template.
func (a *App) PickDirectory() string {
	path, err := a.openDirectoryDialog(openDialogOpts{
		title:                "Select folder",
		defaultDirectory:     a.currentWorkdir(),
		canCreateDirectories: true,
	})
	if err != nil {
		return ""
	}
	return path
}

// OpenFolderInNewInstance launches a second, independent instance of Thaw rooted
// at dir (VS Code's "Open Folder in New Window"), passing it as --workdir=<dir>;
// the new process treats it as a per-instance override and never writes it back to
// the global config, so the two windows can work on different folders.
//
// We deliberately re-exec the running binary directly rather than `open -n
// Thaw.app`: on macOS `open` resolves the target through LaunchServices by bundle
// ID, which can launch a stale or duplicate registered copy of the app (running
// old code that ignores --workdir) instead of the build in hand. Direct exec runs
// exactly this binary and reliably delivers argv.
func (a *App) OpenFolderInNewInstance(dir string) error {
	if dir == "" {
		return fmt.Errorf("no directory given")
	}
	// Fail up front on a stale Recent entry (deleted/renamed/unmounted) rather than
	// spawning a window rooted at a nonexistent folder that then degrades silently.
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("folder is not available: %s", dir)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--workdir="+dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the child once it exits so it doesn't linger as a zombie in this
	// long-lived parent's process table after its window is closed.
	go func() { _ = cmd.Wait() }()
	return nil
}

// PickFileForFormatPreview opens a native file-picker filtered to common data
// file extensions. Returns the chosen path, or "" if the user canceled.
func (a *App) PickFileForFormatPreview() string {
	path, err := a.openFileDialog(openDialogOpts{
		title: "Select a data file to preview",
		filters: []fileFilter{
			{displayName: "Data Files (*.csv, *.tsv, *.json, *.ndjson, *.jsonl)", pattern: "*.csv;*.tsv;*.txt;*.json;*.ndjson;*.jsonl"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}
