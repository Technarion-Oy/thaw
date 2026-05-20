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
// The path must be inside allowedRoot for consistency with other file operations.
func RevealInFinder(path, allowedRoot string) error {
	if err := validateExistingPath(path, allowedRoot); err != nil {
		return err
	}
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

// RuntimeOS returns the current OS identifier (darwin, windows, linux).
// Used by the frontend to display platform-appropriate labels.
func RuntimeOS() string {
	return runtime.GOOS
}

// DeleteFile removes the file at path. It refuses to delete directories;
// use DeleteDirectory for that. The path must be inside allowedRoot.
func DeleteFile(path, allowedRoot string) error {
	if err := validateExistingPath(path, allowedRoot); err != nil {
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
// The path must be strictly inside allowedRoot (cannot delete the root itself).
func DeleteDirectory(path, allowedRoot string) error {
	if err := validateExistingPath(path, allowedRoot); err != nil {
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
// For newPath (which doesn't exist yet), the parent directory is validated instead.
func RenameFile(oldPath, newPath, allowedRoot string) error {
	if err := validateExistingPath(oldPath, allowedRoot); err != nil {
		return err
	}
	if err := validateNewPath(newPath, allowedRoot); err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", filepath.Base(newPath))
	}
	return os.Rename(oldPath, newPath)
}

// MkDir creates a directory (and any necessary parents) at path.
// The path must be inside allowedRoot. Since the target doesn't exist yet,
// the parent directory is validated instead.
func MkDir(path, allowedRoot string) error {
	if err := validateNewPath(path, allowedRoot); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

// WriteFileInRoot creates or overwrites the file at path with content.
// Parent directories are created if needed. The path must be inside allowedRoot.
func WriteFileInRoot(path, content, allowedRoot string) error {
	if err := validateNewPath(path, allowedRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// validateExistingPath checks that an existing path is strictly inside allowedRoot,
// resolving symlinks to prevent escaping the root via symbolic links.
func validateExistingPath(path, allowedRoot string) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(allowedRoot)
	if err != nil {
		return fmt.Errorf("invalid root: %w", err)
	}
	return checkStrictlyInside(realPath, realRoot)
}

// validateNewPath checks that a path that doesn't exist yet is inside allowedRoot
// by resolving symlinks on the nearest existing ancestor directory.
func validateNewPath(path, allowedRoot string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// Walk up to find the nearest existing ancestor and resolve symlinks on it.
	ancestor := absPath
	for {
		if _, statErr := os.Stat(ancestor); statErr == nil {
			break
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			// Reached filesystem root without finding an existing ancestor.
			return fmt.Errorf("no existing ancestor for path: %s", path)
		}
		ancestor = parent
	}
	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// Reconstruct the full path with the resolved ancestor prefix.
	remaining, _ := filepath.Rel(ancestor, absPath)
	realPath := filepath.Join(realAncestor, remaining)

	realRoot, err := filepath.EvalSymlinks(allowedRoot)
	if err != nil {
		return fmt.Errorf("invalid root: %w", err)
	}
	return checkStrictlyInside(realPath, realRoot)
}

// checkStrictlyInside verifies that resolvedPath is strictly inside resolvedRoot
// (not equal to it, not outside it).
func checkStrictlyInside(resolvedPath, resolvedRoot string) error {
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return fmt.Errorf("path is outside allowed directory")
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path is outside allowed directory: %s", resolvedRoot)
	}
	return nil
}
