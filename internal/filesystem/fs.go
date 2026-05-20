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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FileEntry describes a single file or directory.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// ReadFile returns the full text content of the file at path.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadFileHead returns the first maxBytes bytes of the file at path as a string.
// If the file is smaller than maxBytes, the full content is returned.
// This is intended for lightweight previews and is safe to call on large files.
func ReadFileHead(path string, maxBytes int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(f, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

// WriteFile creates or overwrites the file at path with content.
// Parent directories are created if they do not exist.
func WriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ListDir returns the direct children of dir, directories first then files,
// both groups sorted alphabetically.
func ListDir(dir string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs, files []FileEntry
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil && !e.IsDir() {
			size = info.Size()
		}
		fe := FileEntry{
			Name:  e.Name(),
			Path:  filepath.Join(dir, e.Name()),
			IsDir: e.IsDir(),
			Size:  size,
		}
		if e.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
	}

	return append(dirs, files...), nil
}

// RevealInFinder opens the platform file manager and selects the given path.
// On macOS it runs `open -R`, on Windows `explorer /select,`, on Linux `xdg-open`
// (opens the parent directory since most Linux file managers don't support select).
func RevealInFinder(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", abs).Start()
	case "windows":
		return exec.Command("explorer", "/select,", abs).Start()
	default: // linux and others
		return exec.Command("xdg-open", filepath.Dir(abs)).Start()
	}
}

// DeleteFile removes the file at path. It refuses to delete directories;
// use DeleteDirectory for that. The path must be inside allowedRoot.
func DeleteFile(path, allowedRoot string) error {
	if err := validateInsideRoot(path, allowedRoot); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, use DeleteDirectory instead")
	}
	return os.Remove(path)
}

// DeleteDirectory removes the directory at path and all its contents.
// The path must be inside allowedRoot.
func DeleteDirectory(path, allowedRoot string) error {
	if err := validateInsideRoot(path, allowedRoot); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return os.RemoveAll(path)
}

// RenameFile renames (moves) oldPath to newPath. Both must be inside allowedRoot.
func RenameFile(oldPath, newPath, allowedRoot string) error {
	if err := validateInsideRoot(oldPath, allowedRoot); err != nil {
		return err
	}
	if err := validateInsideRoot(newPath, allowedRoot); err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", filepath.Base(newPath))
	}
	return os.Rename(oldPath, newPath)
}

// MkDir creates a directory (and any necessary parents) at path.
// The path must be inside allowedRoot.
func MkDir(path, allowedRoot string) error {
	if err := validateInsideRoot(path, allowedRoot); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

// validateInsideRoot checks that path is inside allowedRoot after resolving symlinks.
func validateInsideRoot(path, allowedRoot string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		return fmt.Errorf("invalid root: %w", err)
	}
	// Ensure the path starts with the root directory.
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("path is outside allowed directory")
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path is outside allowed directory: %s", allowedRoot)
	}
	return nil
}
