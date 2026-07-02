// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ddl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"thaw/internal/sqltok"
)

// ─── public types ─────────────────────────────────────────────────────────────

// ExportOptions configures the parallel DDL export pipeline.
type ExportOptions struct {
	// OutputDir is the root directory under which per-database sub-trees are written.
	OutputDir string

	// PathTemplate controls the file path produced for each exported object.
	// Supported placeholders: {database}, {schema}, {object_type}, {object_name}.
	// An empty value falls back to DefaultExportPathTemplate.
	PathTemplate string

	// ObjectTypes restricts which object kinds are written. Empty = all.
	// KindDatabase and KindSchema are structural anchors and always written.
	// This is a post-fetch filter: GET_DDL('DATABASE', …) always returns the
	// whole database regardless.
	ObjectTypes []Kind

	// Schemas restricts export to the named schemas (case-insensitive).
	// An entry is either a bare schema name ("PUBLIC" — matches in every
	// exported database) or qualified as "DATABASE.SCHEMA" (matches only in
	// that database). Empty = all. Like ObjectTypes, this filters parsed
	// statements only.
	Schemas []string

	// SkipExisting leaves files that already exist on disk untouched instead
	// of overwriting them. Skipped files are counted in ExportResult.Skipped.
	SkipExisting bool

	// DBConcurrency is the maximum number of databases fetched from Snowflake
	// simultaneously.  Defaults to min(16, runtime.NumCPU()*4).
	DBConcurrency int

	// FileConcurrency is the maximum number of files written to disk in parallel
	// for each database.  Defaults to runtime.NumCPU() * 4.
	FileConcurrency int
}

// applyDefaults fills in zero-value concurrency fields with CPU-proportional
// defaults so callers do not need to set them explicitly.
func (o *ExportOptions) applyDefaults() {
	if o.DBConcurrency <= 0 {
		// Each database fetch is now a single Snowflake round-trip (pure I/O),
		// so concurrency is not bounded by CPU. A generous default keeps
		// Snowflake busy across large account inventories.
		o.DBConcurrency = min(16, runtime.NumCPU()*4)
	}
	if o.FileConcurrency <= 0 {
		o.FileConcurrency = runtime.NumCPU() * 4
	}
}

// ExportResult reports the outcome of exporting one database.
type ExportResult struct {
	Database string   `json:"database"`
	Files    int      `json:"files"`
	Skipped  int      `json:"skipped"` // unparsable statements + existing files left untouched (SkipExisting)
	Errors   []string `json:"errors,omitempty"`
}

// ProgressFunc is called each time a database export finishes (from arbitrary
// goroutines; implementations must be goroutine-safe).
type ProgressFunc func(done, total int, result ExportResult)

// FetchDDL is the function signature for retrieving a database's raw DDL.
// It is called concurrently from multiple goroutines.
type FetchDDL func(ctx context.Context, database string) (string, error)

// ─── ExportDatabases ─────────────────────────────────────────────────────────

// ExportDatabases fetches and exports DDL for every database in the list.
//
// Parallelism is controlled by opts:
//   - Up to DBConcurrency databases are fetched from Snowflake concurrently.
//   - For each database, up to FileConcurrency goroutines write files in parallel.
//
// progress is called (goroutine-safely) after each database completes and may
// be nil.  The returned slice has the same length and order as databases.
func ExportDatabases(
	ctx context.Context,
	databases []string,
	fetch FetchDDL,
	opts ExportOptions,
	progress ProgressFunc,
) []ExportResult {
	opts.applyDefaults()

	total := len(databases)
	results := make([]ExportResult, total)

	// Channel-based semaphore: at most DBConcurrency goroutines run at once.
	sem := make(chan struct{}, opts.DBConcurrency)

	var wg sync.WaitGroup
	var done atomic.Int32

	for i, db := range databases {
		wg.Add(1)
		go func(idx int, dbName string) {
			defer wg.Done()

			// Wait for a semaphore slot, but bail out if the context is canceled.
			select {
			case sem <- struct{}{}: // acquire
				defer func() { <-sem }() // release
			case <-ctx.Done():
				return
			}

			res := exportOne(ctx, dbName, fetch, opts)
			results[idx] = res

			n := int(done.Add(1))
			if progress != nil {
				progress(n, total, res)
			}
		}(i, db)
	}

	wg.Wait()
	return results
}

// ─── per-database export ──────────────────────────────────────────────────────

// writeJob is a unit of work dispatched to the file-writer goroutine pool.
// absPath is the destination file and content is the SQL to write.
type writeJob struct {
	absPath string
	content []byte
}

// exportOne fetches, splits, and writes DDL for a single database.
func exportOne(ctx context.Context, database string, fetch FetchDDL, opts ExportOptions) ExportResult {
	res := ExportResult{Database: database}

	rawDDL, err := fetch(ctx, database)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("fetch DDL: %v", err))
		return res
	}

	// Parse all statements and resolve file-path collisions.
	// This is intentionally single-threaded: the collision resolver is stateful
	// and sequential resolution gives deterministic, reproducible output.
	stmts := sqltok.Split(rawDDL)
	tracker := newNameTracker()

	wantKind := make(map[Kind]bool, len(opts.ObjectTypes))
	for _, k := range opts.ObjectTypes {
		wantKind[k] = true
	}
	wantSchema := make(map[string]bool, len(opts.Schemas))
	for _, s := range opts.Schemas {
		wantSchema[strings.ToUpper(s)] = true
	}

	jobs := make([]writeJob, 0, len(stmts))
	for _, s := range stmts {
		obj := Parse(s)
		if obj.Kind == KindUnknown || obj.Name == "" {
			res.Skipped++
			continue
		}

		// User-selected filters. Database/schema anchors are always kept so
		// the exported tree stays loadable; everything else must match.
		if obj.Kind != KindDatabase && obj.Kind != KindSchema {
			if len(wantKind) > 0 && !wantKind[obj.Kind] {
				continue
			}
			if len(wantSchema) > 0 {
				schema := strings.ToUpper(obj.Schema)
				if !wantSchema[schema] && !wantSchema[strings.ToUpper(database)+"."+schema] {
					continue
				}
			}
		}

		// resolve() before the SkipExisting check so numbered-suffix
		// assignment stays deterministic regardless of what is on disk.
		rel := tracker.resolve(obj.FilePathFor(opts.PathTemplate, database))
		absPath := filepath.Join(opts.OutputDir, rel)

		if opts.SkipExisting {
			if _, statErr := os.Stat(absPath); statErr == nil {
				res.Skipped++
				continue
			}
		}

		jobs = append(jobs, writeJob{
			absPath: absPath,
			content: []byte(obj.SQL + ";\n"),
		})
	}

	if len(jobs) == 0 {
		return res
	}

	// Pre-create all unique output directories before dispatching parallel
	// writers.  This reduces MkdirAll calls from O(files) to O(unique dirs)
	// and removes the per-file MkdirAll from the parallel hot-path entirely.
	seenDirs := make(map[string]struct{}, len(jobs)/4)
	for _, j := range jobs {
		d := filepath.Dir(j.absPath)
		if _, ok := seenDirs[d]; !ok {
			seenDirs[d] = struct{}{}
			if err := os.MkdirAll(d, 0o755); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("mkdir %s: %v", d, err))
			}
		}
	}

	// Fan-out writes across FileConcurrency workers.
	nWorkers := min(opts.FileConcurrency, len(jobs))
	if nWorkers < 1 {
		nWorkers = 1
	}

	jobCh := make(chan writeJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var mu sync.Mutex
	var fileCount atomic.Int32
	var writeWg sync.WaitGroup

	for range nWorkers {
		writeWg.Add(1)
		go func() {
			defer writeWg.Done()
			for j := range jobCh {
				if ctx.Err() != nil {
					return
				}
				if err := atomicWrite(j.absPath, j.content); err != nil {
					mu.Lock()
					res.Errors = append(res.Errors, err.Error())
					mu.Unlock()
				} else {
					fileCount.Add(1)
				}
			}
		}()
	}

	writeWg.Wait()
	res.Files = int(fileCount.Load())

	// Remove the legacy "_root/" directory that was created by exports before
	// fully-qualified-name support was added (GET_DDL(..., true)).  Now that all
	// objects use three-part names the directory is never written to, but old
	// runs may have left files there that would confuse users.
	rootDir := filepath.Join(opts.OutputDir, sanitize(database), "_root")
	if _, statErr := os.Stat(rootDir); statErr == nil {
		_ = os.RemoveAll(rootDir)
	}

	return res
}

// ─── atomic file write ────────────────────────────────────────────────────────

// atomicWrite writes content to a temporary file in the same directory as
// path, then atomically renames it to path.  Rename is atomic on POSIX
// systems, so readers never see a partially-written file.
//
// The parent directory must already exist; call os.MkdirAll before the first
// write to any new directory (exportOne does this upfront for all paths).
func atomicWrite(path string, content []byte) error {
	// Write to a temp file in the same directory so os.Rename never crosses
	// a filesystem boundary.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write temp %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename → %s: %w", path, err)
	}

	return nil
}
