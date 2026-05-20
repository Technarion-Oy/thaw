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
	"testing"
)

// ─── validateExistingPath / checkStrictlyInside ─────────────────────────────

func TestValidateExistingPath_InsideRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateExistingPath(child, root); err != nil {
		t.Errorf("expected no error for path inside root, got: %v", err)
	}
}

func TestValidateExistingPath_RootEqualsPath(t *testing.T) {
	root := t.TempDir()
	err := validateExistingPath(root, root)
	if err == nil {
		t.Error("expected error when path equals root (should be strictly inside), got nil")
	}
}

func TestValidateExistingPath_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	err := validateExistingPath(outside, root)
	if err == nil {
		t.Error("expected error for path outside root, got nil")
	}
}

func TestValidateExistingPath_DotDotSegments(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	// Path that uses .. to escape: root/sub/../../<something outside>
	outside := t.TempDir()
	sneakyPath := filepath.Join(child, "..", "..", filepath.Base(outside))
	// The sneaky path may or may not exist; validateExistingPath requires it to exist.
	// Create a file at the resolved location to make the test meaningful.
	err := validateExistingPath(sneakyPath, root)
	if err == nil {
		t.Error("expected error for path with .. segments escaping root, got nil")
	}
}

func TestValidateExistingPath_SymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside root that points outside
	symlink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideFile, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	err := validateExistingPath(symlink, root)
	if err == nil {
		t.Error("expected error for symlink pointing outside root, got nil")
	}
}

func TestValidateExistingPath_SymlinkDirEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// Create a directory symlink inside root that points to an outside directory
	symlink := filepath.Join(root, "linked_dir")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	err := validateExistingPath(symlink, root)
	if err == nil {
		t.Error("expected error for directory symlink pointing outside root, got nil")
	}
}

// ─── validateNewPath ────────────────────────────────────────────────────────

func TestValidateNewPath_InsideRoot(t *testing.T) {
	root := t.TempDir()
	newPath := filepath.Join(root, "newfile.txt")
	if err := validateNewPath(newPath, root); err != nil {
		t.Errorf("expected no error for new path inside root, got: %v", err)
	}
}

func TestValidateNewPath_Nested(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(sub, "b", "c", "newfile.txt")
	if err := validateNewPath(newPath, root); err != nil {
		t.Errorf("expected no error for deeply nested new path, got: %v", err)
	}
}

func TestValidateNewPath_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	newPath := filepath.Join(outside, "newfile.txt")
	err := validateNewPath(newPath, root)
	if err == nil {
		t.Error("expected error for new path outside root, got nil")
	}
}

func TestValidateNewPath_SymlinkAncestorEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// Create a symlink to an outside dir inside root
	symlink := filepath.Join(root, "linked")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// New file under the symlinked dir — should be rejected
	newPath := filepath.Join(symlink, "newfile.txt")
	err := validateNewPath(newPath, root)
	if err == nil {
		t.Error("expected error for new path under symlink escaping root, got nil")
	}
}

// ─── DeleteFile ─────────────────────────────────────────────────────────────

func TestDeleteFile_Success(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "test.txt")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DeleteFile(f, root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteFile_RejectsDirectory(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	err := DeleteFile(sub, root)
	if err == nil {
		t.Error("expected error when trying to delete a directory with DeleteFile")
	}
}

func TestDeleteFile_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	f := filepath.Join(outside, "test.txt")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := DeleteFile(f, root)
	if err == nil {
		t.Error("expected error for file outside root")
	}
	// File should still exist
	if _, statErr := os.Stat(f); statErr != nil {
		t.Error("file outside root should not have been deleted")
	}
}

// ─── DeleteDirectory ────────────────────────────────────────────────────────

func TestDeleteDirectory_Success(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Put a file inside
	if err := os.WriteFile(filepath.Join(sub, "inner.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DeleteDirectory(sub, root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestDeleteDirectory_RejectsRoot(t *testing.T) {
	root := t.TempDir()
	err := DeleteDirectory(root, root)
	if err == nil {
		t.Error("expected error when trying to delete the root itself")
	}
}

func TestDeleteDirectory_RejectsFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "file.txt")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := DeleteDirectory(f, root)
	if err == nil {
		t.Error("expected error when trying to delete a file with DeleteDirectory")
	}
}

func TestDeleteDirectory_SymlinkToDir(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "realdir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "inner.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Symlink inside root pointing to a directory inside root.
	symlink := filepath.Join(root, "link_to_dir")
	if err := os.Symlink(realDir, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	// DeleteDirectory on a symlink should remove only the symlink, not the target.
	if err := DeleteDirectory(symlink, root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if _, err := os.Lstat(symlink); !os.IsNotExist(err) {
		t.Error("symlink should have been deleted")
	}
	// Real directory and its contents should still exist.
	if _, err := os.Stat(realDir); err != nil {
		t.Error("real directory should still exist after deleting symlink")
	}
	data, err := os.ReadFile(filepath.Join(realDir, "inner.txt"))
	if err != nil || string(data) != "data" {
		t.Error("contents of real directory should be intact")
	}
}

// ─── RenameFile ─────────────────────────────────────────────────────────────

func TestRenameFile_Success(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old.txt")
	new_ := filepath.Join(root, "new.txt")
	if err := os.WriteFile(old, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RenameFile(old, new_, root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}
	if _, err := os.Stat(new_); err != nil {
		t.Error("new file should exist after rename")
	}
}

func TestRenameFile_DestinationExists(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old.txt")
	new_ := filepath.Join(root, "new.txt")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(new_, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RenameFile(old, new_, root)
	if err == nil {
		t.Error("expected error when destination already exists")
	}
}

func TestRenameFile_SourceOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(outside, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "dst.txt")
	err := RenameFile(src, dst, root)
	if err == nil {
		t.Error("expected error when source is outside root")
	}
}

func TestRenameFile_DestOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(root, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(outside, "dst.txt")
	err := RenameFile(src, dst, root)
	if err == nil {
		t.Error("expected error when destination is outside root")
	}
}

// ─── MkDir ──────────────────────────────────────────────────────────────────

func TestMkDir_Success(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b", "c")
	if err := MkDir(dir, root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal("directory should exist after MkDir")
	}
	if !info.IsDir() {
		t.Error("created path should be a directory")
	}
}

func TestMkDir_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(outside, "newdir")
	err := MkDir(dir, root)
	if err == nil {
		t.Error("expected error for directory outside root")
	}
}

// ─── WriteFileInRoot ────────────────────────────────────────────────────────

func TestWriteFileInRoot_Success(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "sub", "test.sql")
	if err := WriteFileInRoot(f, "SELECT 1;", root); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal("file should exist after WriteFileInRoot")
	}
	if string(data) != "SELECT 1;" {
		t.Errorf("content mismatch: got %q", string(data))
	}
}

func TestWriteFileInRoot_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	f := filepath.Join(outside, "test.sql")
	err := WriteFileInRoot(f, "data", root)
	if err == nil {
		t.Error("expected error for file outside root")
	}
}

// ─── validateInsideOrEqual ──────────────────────────────────────────────────

func TestValidateInsideOrEqual_RootEqualsPath(t *testing.T) {
	root := t.TempDir()
	// Should succeed — the root itself is allowed for read-only operations.
	if err := validateInsideOrEqual(root, root); err != nil {
		t.Errorf("expected no error when path equals root, got: %v", err)
	}
}

func TestValidateInsideOrEqual_InsideRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateInsideOrEqual(child, root); err != nil {
		t.Errorf("expected no error for path inside root, got: %v", err)
	}
}

func TestValidateInsideOrEqual_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	err := validateInsideOrEqual(outside, root)
	if err == nil {
		t.Error("expected error for path outside root, got nil")
	}
}

// ─── DeleteFile with symlink to directory ───────────────────────────────────

func TestDeleteFile_SymlinkToDir(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "realdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Symlink inside root pointing to a directory inside root.
	symlink := filepath.Join(root, "link_to_dir")
	if err := os.Symlink(subdir, symlink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	// DeleteFile should succeed (Lstat sees the symlink, not the directory).
	if err := DeleteFile(symlink, root); err != nil {
		t.Errorf("expected no error deleting symlink to directory, got: %v", err)
	}
	// The symlink should be gone, but the real directory should still exist.
	if _, err := os.Lstat(symlink); !os.IsNotExist(err) {
		t.Error("symlink should have been deleted")
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Error("real directory should still exist after deleting symlink")
	}
}
