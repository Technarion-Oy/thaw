// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"thaw/internal/config"
	"thaw/internal/filesystem"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartFileWatcher starts watching the given directory tree for changes.
// Any existing watcher is stopped first. Change events are emitted to
// the frontend as "fs:changed" Wails events.
func (a *App) StartFileWatcher(dir string) error {
	a.fsWatcherMu.Lock()
	defer a.fsWatcherMu.Unlock()

	if a.fsWatcher != nil {
		a.fsWatcher.Close()
		a.fsWatcher = nil
	}

	w, err := filesystem.NewWatcher(dir, func(evt filesystem.FSChangeEvent) {
		wailsruntime.EventsEmit(a.ctx, "fs:changed", evt)
	})
	if err != nil {
		return err
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

// PickOpenFile opens a native open-file dialog filtered to SQL, YAML and
// Python files and returns the chosen path, or an empty string if canceled.
// The dialog opens in the configured export directory when one is set.
func (a *App) PickOpenFile() string {
	defaultDir := ""
	if cfg, err := config.Load(); err == nil {
		defaultDir = cfg.Git.ExportDir
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Open file",
		DefaultDirectory: defaultDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Supported Files (*.sql, *.yml, *.yaml, *.py)", Pattern: "*.sql;*.yml;*.yaml;*.py"},
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
			{DisplayName: "YAML Files (*.yml, *.yaml)", Pattern: "*.yml;*.yaml"},
			{DisplayName: "Python Files (*.py)", Pattern: "*.py"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
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
	defaultDir := ""
	if cfg, err := config.Load(); err == nil {
		defaultDir = cfg.Git.ExportDir
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Open any file",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return ""
	}
	return path
}

// dataFileFilters returns dialog file filters for the given import format.
func dataFileFilters(format string) []wailsruntime.FileFilter {
	switch strings.ToUpper(format) {
	case "JSON":
		return []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "PARQUET":
		return []wailsruntime.FileFilter{
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "AVRO":
		return []wailsruntime.FileFilter{
			{DisplayName: "Avro Files (*.avro)", Pattern: "*.avro"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "ORC":
		return []wailsruntime.FileFilter{
			{DisplayName: "ORC Files (*.orc)", Pattern: "*.orc"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	default: // CSV
		return []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	}
}

// PickDataFile opens a native open-file dialog filtered to common data file
// formats and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickDataFile() string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open data file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Data Files (*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc)", Pattern: "*.csv;*.json;*.jsonl;*.ndjson;*.parquet;*.avro;*.orc"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "JSON Files (*.json;*.jsonl;*.ndjson)", Pattern: "*.json;*.jsonl;*.ndjson"},
			{DisplayName: "Parquet Files (*.parquet)", Pattern: "*.parquet"},
			{DisplayName: "Avro Files (*.avro)", Pattern: "*.avro"},
			{DisplayName: "ORC Files (*.orc)", Pattern: "*.orc"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
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
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open " + format + " file",
		Filters: dataFileFilters(format),
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
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open " + format + " files",
		Filters: filters,
	})
	if err != nil {
		return nil
	}
	return paths
}

// PickSaveFile opens a native save-file dialog pre-populated with defaultName
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickSaveFile(defaultName string) string {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save file",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL Files (*.sql)", Pattern: "*.sql"},
			{DisplayName: "YAML Files (*.yml, *.yaml)", Pattern: "*.yml;*.yaml"},
			{DisplayName: "Python Files (*.py)", Pattern: "*.py"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
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
	var filters []wailsruntime.FileFilter
	title := "Save export file"
	switch format {
	case "csv":
		title = "Save as CSV"
		filters = []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	case "excel":
		title = "Save as Excel"
		filters = []wailsruntime.FileFilter{
			{DisplayName: "Excel Files (*.xlsx)", Pattern: "*.xlsx"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	default:
		filters = []wailsruntime.FileFilter{
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		}
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         filters,
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

// PickDirectory opens a native folder-picker dialog and returns the selected path.
// Returns an empty string if the user cancels.
func (a *App) PickDirectory() string {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select repository directory",
	})
	if err != nil {
		return ""
	}
	return path
}

// OpenFolderInNewInstance launches a second, independent instance of Thaw rooted
// at dir (VS Code's "Open Folder in New Window"). The folder is passed both as
// --workdir=<dir> and via the THAW_WORKDIR env var; the new process treats it as a
// per-instance override and never writes it back to the global config, so the two
// windows can work on different folders.
//
// We deliberately re-exec the running binary directly rather than `open -n
// Thaw.app`: on macOS `open` resolves the target through LaunchServices by bundle
// ID, which can launch a stale or duplicate registered copy of the app (running
// old code that ignores --workdir) instead of the build in hand. Direct exec runs
// exactly this binary and reliably delivers both argv and the environment.
func (a *App) OpenFolderInNewInstance(dir string) error {
	if dir == "" {
		return fmt.Errorf("no directory given")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--workdir="+dir)
	cmd.Env = append(os.Environ(), "THAW_WORKDIR="+dir)
	return cmd.Start()
}

// PickFileForFormatPreview opens a native file-picker filtered to common data
// file extensions. Returns the chosen path, or "" if the user canceled.
func (a *App) PickFileForFormatPreview() string {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select a data file to preview",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Data Files (*.csv, *.tsv, *.json, *.ndjson, *.jsonl)", Pattern: "*.csv;*.tsv;*.txt;*.json;*.ndjson;*.jsonl"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}
