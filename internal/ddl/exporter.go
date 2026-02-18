package ddl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

// ─── public types ─────────────────────────────────────────────────────────────

// ExportOptions configures the parallel DDL export pipeline.
type ExportOptions struct {
	// OutputDir is the root directory under which per-database sub-trees are written.
	OutputDir string

	// DBConcurrency is the maximum number of databases fetched from Snowflake
	// simultaneously.  Defaults to min(8, runtime.NumCPU()).
	DBConcurrency int

	// FileConcurrency is the maximum number of files written to disk in parallel
	// for each database.  Defaults to runtime.NumCPU() * 4.
	FileConcurrency int
}

func (o *ExportOptions) applyDefaults() {
	if o.DBConcurrency <= 0 {
		o.DBConcurrency = min(8, runtime.NumCPU())
	}
	if o.FileConcurrency <= 0 {
		o.FileConcurrency = runtime.NumCPU() * 4
	}
}

// ExportResult reports the outcome of exporting one database.
type ExportResult struct {
	Database string   `json:"database"`
	Files    int      `json:"files"`
	Skipped  int      `json:"skipped"` // statements that could not be parsed
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

			sem <- struct{}{} // acquire
			defer func() { <-sem }() // release

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

	stmts := Split(rawDDL)

	// Parse all statements and resolve file-path collisions.
	// This is intentionally single-threaded: the collision resolver is stateful
	// and sequential resolution gives deterministic, reproducible output.
	tracker := newNameTracker()
	dbDir := filepath.Join(opts.OutputDir, sanitize(database))

	jobs := make([]writeJob, 0, len(stmts))
	for _, s := range stmts {
		obj := Parse(s)
		if obj.Kind == KindUnknown || obj.Name == "" {
			res.Skipped++
			continue
		}

		rel := tracker.resolve(obj.FilePath())
		jobs = append(jobs, writeJob{
			absPath: filepath.Join(dbDir, rel),
			content: []byte(obj.SQL + ";\n"),
		})
	}

	if len(jobs) == 0 {
		return res
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
	return res
}

// ─── atomic file write ────────────────────────────────────────────────────────

// atomicWrite ensures the parent directory exists, writes content to a
// temporary file in the same directory, then renames it to path.
// Rename is an atomic operation on POSIX systems, so readers never see a
// partially-written file.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

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
