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
	"unicode/utf8"
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
// The path must be inside or equal to allowedRoot. Unlike mutating operations,
// reveal is read-only so allowing the root itself is safe.
func RevealInFinder(path, allowedRoot string) error {
	if err := validateInsideOrEqual(path, allowedRoot); err != nil {
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
		// explorer treats commas as argument delimiters, so the path must be
		// quoted within the /select, argument. Go's syscall.EscapeArg does not
		// handle this explorer-specific behavior.
		return exec.Command("explorer", fmt.Sprintf(`/select,"%s"`, abs)).Start()
	default: // linux and others
		return exec.Command("xdg-open", filepath.Dir(abs)).Start()
	}
}

// RuntimeOS returns the current OS identifier (darwin, windows, linux).
// Used by the frontend to display platform-appropriate labels.
func RuntimeOS() string {
	return runtime.GOOS
}

// DeleteFile removes the file (or symlink) at path. It refuses to delete
// directories; use DeleteDirectory for that. The path must be inside allowedRoot.
// Uses Lstat so symlinks pointing to directories can still be deleted as files.
// Note: validateExistingPath resolves symlinks and checks the resolved path, while
// the actual deletion operates on the original path. A symlink swap between
// validation and deletion is theoretically possible but impractical on a desktop app.
func DeleteFile(path, allowedRoot string) error {
	if err := validateExistingPath(path, allowedRoot); err != nil {
		return err
	}
	info, err := os.Lstat(path)
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
// Uses Lstat so that a symlink pointing to a directory is treated as a symlink
// (removed via os.Remove) rather than recursively deleting the target.
// Note: same TOCTOU gap as DeleteFile — resolved path is validated, original is deleted.
func DeleteDirectory(path, allowedRoot string) error {
	if err := validateExistingPath(path, allowedRoot); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	// Symlink: remove the link itself regardless of what it points to.
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return os.RemoveAll(path)
}

// RenameFile renames (moves) oldPath to newPath. Both must be inside allowedRoot.
// For newPath (which doesn't exist yet), the parent directory is validated instead.
// Note: the destination-exists check is not atomic with os.Rename (TOCTOU window).
// On a single-user desktop app this is acceptable; concurrent creation of the
// same filename between the check and rename is extremely unlikely.
func RenameFile(oldPath, newPath, allowedRoot string) error {
	if err := validateExistingPath(oldPath, allowedRoot); err != nil {
		return err
	}
	if err := validateNewPath(newPath, allowedRoot); err != nil {
		return err
	}
	if newInfo, err := os.Stat(newPath); err == nil {
		// Allow case-only renames (e.g. File.sql → file.sql) on any filesystem:
		// os.SameFile checks inode/file-ID, so it works correctly regardless of
		// whether the FS is case-sensitive or case-insensitive.
		oldInfo, errOld := os.Stat(oldPath)
		if errOld != nil || !os.SameFile(oldInfo, newInfo) {
			return fmt.Errorf("destination already exists: %s", filepath.Base(newPath))
		}
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

// WriteFileInRoot creates a new file at path with content.
// Parent directories are created if needed. The path must be inside allowedRoot.
// Returns an error if the file already exists (prevents accidental overwrites).
func WriteFileInRoot(path, content, allowedRoot string) error {
	if err := validateNewPath(path, allowedRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) // #nosec G302 — SQL files need group/other read
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file already exists: %s", filepath.Base(path))
		}
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close() //nolint:errcheck
		return err
	}
	return f.Close()
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
	remaining, err := filepath.Rel(ancestor, absPath)
	if err != nil {
		return fmt.Errorf("cannot compute relative path: %w", err)
	}
	// Defense-in-depth: reject if remaining somehow escapes (should not happen
	// since absPath is always deeper than ancestor, but guard explicitly).
	if strings.HasPrefix(remaining, "..") {
		return fmt.Errorf("path escapes ancestor: %s", path)
	}
	realPath := filepath.Join(realAncestor, remaining)

	realRoot, err := filepath.EvalSymlinks(allowedRoot)
	if err != nil {
		return fmt.Errorf("invalid root: %w", err)
	}
	return checkStrictlyInside(realPath, realRoot)
}

// ValidateInsideOrEqual checks that an existing path is inside or equal to
// allowedRoot, resolving symlinks. Used for read-only operations like Reveal
// and MCP workspace tool sandboxing.
func ValidateInsideOrEqual(path, allowedRoot string) error {
	return validateInsideOrEqual(path, allowedRoot)
}

// validateInsideOrEqual is the unexported implementation of ValidateInsideOrEqual.
func validateInsideOrEqual(path, allowedRoot string) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(allowedRoot)
	if err != nil {
		return fmt.Errorf("invalid root: %w", err)
	}
	return checkInsideOrEqual(realPath, realRoot)
}

// caseInsensitiveFS reports whether the current platform typically uses
// case-insensitive filesystem paths (macOS APFS default, Windows NTFS).
// Note: this is an OS-level heuristic, not filesystem-aware. macOS supports
// case-sensitive APFS volumes and Windows supports case-sensitive directories
// via fsutil. This is acceptable as a defense-in-depth layer (used alongside
// filepath.Rel which handles case correctly on all platforms). For operations
// where correctness is critical (e.g. case-only renames), use os.SameFile instead.
func caseInsensitiveFS() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

// hasPathPrefix checks whether path starts with prefix, respecting platform
// case sensitivity. On macOS/Windows the comparison is case-folded.
// Note: the byte-length slice before EqualFold is safe for filesystem paths
// which are overwhelmingly ASCII. Unicode characters whose case-folded forms
// have different byte widths (e.g. ß/SS) are not valid in typical FS paths.
// Guard: if the slice point falls mid-rune the check returns false and the
// filepath.Rel defense-in-depth layer handles correctness.
func hasPathPrefix(path, prefix string) bool {
	if caseInsensitiveFS() {
		if len(path) < len(prefix) {
			return false
		}
		if !utf8.ValidString(path[:len(prefix)]) {
			return false
		}
		return strings.EqualFold(path[:len(prefix)], prefix)
	}
	return strings.HasPrefix(path, prefix)
}

// pathsEqual checks path equality, respecting platform case sensitivity.
func pathsEqual(a, b string) bool {
	if caseInsensitiveFS() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// checkStrictlyInside verifies that resolvedPath is strictly inside resolvedRoot
// (not equal to it, not outside it). Uses both a string-prefix check and
// filepath.Rel as defense-in-depth.
func checkStrictlyInside(resolvedPath, resolvedRoot string) error {
	// Defense-in-depth: string-prefix check prevents prefix-collision edge cases.
	// Uses case-aware comparison for macOS/Windows.
	if !hasPathPrefix(resolvedPath, resolvedRoot+string(filepath.Separator)) {
		return fmt.Errorf("path is outside allowed directory: %s", resolvedRoot)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return fmt.Errorf("path is outside allowed directory")
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path is outside allowed directory: %s", resolvedRoot)
	}
	return nil
}

// uniqueCopyName generates a unique copy name for srcPath by appending _copy, _copy_2, etc.
// Returns an error if no unique name can be found within 999 attempts.
func uniqueCopyName(srcPath string) (string, error) {
	dir := filepath.Dir(srcPath)
	base := filepath.Base(srcPath)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]

	candidate := filepath.Join(dir, stem+"_copy"+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	} else if err != nil {
		return "", err
	}
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s_copy_%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find a unique copy name for %s", filepath.Base(srcPath))
}

// DuplicateFile creates a copy of srcPath in the same directory with a unique name.
// The source path must be strictly inside allowedRoot. Returns the path of the new copy.
func DuplicateFile(srcPath, allowedRoot string) (string, error) {
	if err := validateExistingPath(srcPath, allowedRoot); err != nil {
		return "", err
	}
	info, err := os.Lstat(srcPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot duplicate a directory")
	}

	dstPath, err := uniqueCopyName(srcPath)
	if err != nil {
		return "", err
	}
	if err := validateNewPath(dstPath, allowedRoot); err != nil {
		return "", err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close() //nolint:errcheck

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()        //nolint:errcheck // close before remove (required on Windows)
		os.Remove(dstPath) //nolint:errcheck
		return "", err
	}
	if err := dst.Close(); err != nil {
		os.Remove(dstPath) //nolint:errcheck
		return "", err
	}
	return dstPath, nil
}

// checkInsideOrEqual verifies that resolvedPath is inside or equal to resolvedRoot.
// Uses both a string-prefix check and filepath.Rel as defense-in-depth.
func checkInsideOrEqual(resolvedPath, resolvedRoot string) error {
	// Defense-in-depth: string-prefix check prevents prefix-collision edge cases.
	// Uses case-aware comparison for macOS/Windows.
	if !pathsEqual(resolvedPath, resolvedRoot) && !hasPathPrefix(resolvedPath, resolvedRoot+string(filepath.Separator)) {
		return fmt.Errorf("path is outside allowed directory: %s", resolvedRoot)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return fmt.Errorf("path is outside allowed directory")
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path is outside allowed directory: %s", resolvedRoot)
	}
	return nil
}
