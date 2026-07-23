// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// stagePut is a single planned PUT command produced by planStageUploads. Source
// is an absolute local path — a single file when Glob is false, or a directory
// whose files are uploaded via a "<dir>/*" wildcard when Glob is true. RelDir is
// the '/'-separated stage subdirectory the files land in ("" for the app root).
type stagePut struct {
	Source string
	RelDir string
	Glob   bool
}

// isJunkDir reports whether a directory name should be skipped entirely when
// walking a local app folder: version-control metadata, Python bytecode caches,
// and any hidden (dot-prefixed) directory.
func isJunkDir(name string) bool {
	return name == ".git" || name == "__pycache__" || strings.HasPrefix(name, ".")
}

// isJunkFile reports whether a file should be skipped: OS junk and any hidden
// (dot-prefixed) file. ".DS_Store" is already covered by the dot rule but is
// named for clarity.
func isJunkFile(name string) bool {
	return name == ".DS_Store" || strings.HasPrefix(name, ".")
}

// planStageUploads walks the app folder rooted at root and returns an ordered,
// deterministic set of PUT commands that reproduce its non-junk file tree in a
// stage, preserving relative paths. It performs no I/O beyond the walk, so it is
// unit-testable without a live Snowflake connection.
//
// Files are grouped by their containing directory. A directory that physically
// contains nothing but its surviving files (no subdirectories, no junk, no hidden
// entries) is uploaded with a single "<dir>/*" wildcard PUT. Any directory that
// also holds subdirectories or junk falls back to one PUT per surviving file,
// because Go's filepath.Glob — which gosnowflake uses to expand local PUT paths —
// matches dotfiles and subdirectory entries too, so a blanket wildcard could not
// exclude the junk we filtered out.
//
// Skipped: .git/, __pycache__/, other hidden directories, hidden files, and
// .DS_Store (see isJunkDir / isJunkFile).
func planStageUploads(root string) ([]stagePut, error) {
	type dirInfo struct {
		files     []string // absolute paths of surviving files in this directory
		blockGlob bool     // true if the dir also holds a subdir or junk → can't wildcard
	}
	dirs := map[string]*dirInfo{}
	ensure := func(dir string) *dirInfo {
		di := dirs[dir]
		if di == nil {
			di = &dirInfo{}
			dirs[dir] = di
		}
		return di
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			parent := filepath.Dir(path)
			if isJunkDir(d.Name()) {
				// A junk subdirectory still exists on disk, so its parent can't
				// be uploaded with a wildcard without dragging it along.
				ensure(parent).blockGlob = true
				return filepath.SkipDir
			}
			// A real subdirectory gets its own PUT; the parent can't wildcard.
			ensure(parent).blockGlob = true
			return nil
		}
		dir := filepath.Dir(path)
		di := ensure(dir)
		if isJunkFile(d.Name()) {
			di.blockGlob = true
			return nil
		}
		di.files = append(di.files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Emit in a deterministic order: directories sorted, files within sorted.
	dirPaths := make([]string, 0, len(dirs))
	for dir := range dirs {
		dirPaths = append(dirPaths, dir)
	}
	sort.Strings(dirPaths)

	var plan []stagePut
	for _, dir := range dirPaths {
		di := dirs[dir]
		if len(di.files) == 0 {
			continue
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		relDir := filepath.ToSlash(rel)
		if relDir == "." {
			relDir = ""
		}
		if di.blockGlob {
			sort.Strings(di.files)
			for _, f := range di.files {
				plan = append(plan, stagePut{Source: f, RelDir: relDir})
			}
			continue
		}
		plan = append(plan, stagePut{Source: dir, RelDir: relDir, Glob: true})
	}
	return plan, nil
}

// uploadDirToStage recursively uploads the local app folder at localDir to the
// given stage location (e.g. "@db.schema.stage"), preserving each file's path
// relative to localDir and skipping VCS metadata, hidden files, and OS junk. It
// plans the PUT commands with planStageUploads, then issues each with
// AUTO_COMPRESS=FALSE and OVERWRITE=TRUE.
func (c *Client) uploadDirToStage(ctx context.Context, localDir, stageAt string) error {
	root, err := filepath.Abs(localDir)
	if err != nil {
		return fmt.Errorf("resolve app dir: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat app dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("app path is not a directory: %s", root)
	}

	plan, err := planStageUploads(root)
	if err != nil {
		return fmt.Errorf("plan upload: %w", err)
	}
	if len(plan) == 0 {
		return fmt.Errorf("no files to upload in %s", root)
	}

	for _, p := range plan {
		fileURL, err := localFileURLForFile(p.Source)
		if err != nil {
			return fmt.Errorf("build file url for %s: %w", p.Source, err)
		}
		if p.Glob {
			fileURL += "/*"
		}
		escapedURL := strings.ReplaceAll(fileURL, "'", "\\'")

		target := stageAt
		if p.RelDir != "" {
			target = stageAt + "/" + p.RelDir
		}

		putSQL := fmt.Sprintf("PUT '%s' %s AUTO_COMPRESS=FALSE OVERWRITE=TRUE", escapedURL, target)
		putRows, err := c.queryCtx(ctx, putSQL)
		if err != nil {
			return fmt.Errorf("upload %s: %w", p.Source, err)
		}
		putRows.Close() //nolint:errcheck
	}
	return nil
}
