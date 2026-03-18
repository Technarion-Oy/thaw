// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/ddl"
)

// ─── event name constants ─────────────────────────────────────────────────────

const (
	migrationAnalyzeProgressEvent = "migration:analyze:progress"
	migrationExecProgressEvent    = "migration:exec:progress"
)

// ─── structs ──────────────────────────────────────────────────────────────────

// MigrationObject represents a single DDL object discovered in a local source file.
type MigrationObject struct {
	FilePath   string `json:"filePath"`
	Database   string `json:"database"`
	Schema     string `json:"schema"`
	ObjectKind string `json:"objectKind"`
	ObjectName string `json:"objectName"`
	ArgSig     string `json:"argSig"`    // non-empty for FUNCTION/PROCEDURE overloads
	DDL        string `json:"ddl"`       // full DDL text (no trailing semicolon)
	IsReplace  bool   `json:"isReplace"` // true when CREATE OR REPLACE
}

// MigrationDiffItem pairs a local object with its remote status.
type MigrationDiffItem struct {
	Object    MigrationObject `json:"object"`
	Status    string          `json:"status"`    // "new"|"changed"|"unchanged"|"removed"
	LocalDDL  string          `json:"localDDL"`
	RemoteDDL string          `json:"remoteDDL"`
}

// MigrationAnalyzeProgress is emitted during AnalyzeMigration.
type MigrationAnalyzeProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// MigrationExecEvent is emitted during ExecuteMigration for each object processed.
type MigrationExecEvent struct {
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Object string `json:"object"` // "DB.SCHEMA.KIND.NAME"
	Status string `json:"status"` // "running"|"success"|"failed"|"skipped"
	Error  string `json:"error"`
	Pass   int    `json:"pass"`
}

// ─── module-level compiled regexps ───────────────────────────────────────────

var (
	migrationBlockCommentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)
	migrationLineCommentRE  = regexp.MustCompile(`--[^\n]*`)
	migrationWhitespaceRE   = regexp.MustCompile(`\s+`)
	migrationUseDatabaseRE  = regexp.MustCompile(`(?i)^\s*USE\s+DATABASE\s+"?([^"\s;]+)"?`)
	migrationUseSchemaRE    = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+"?([^"\s;]+)"?`)
	migrationIsReplaceRE    = regexp.MustCompile(`(?i)^\s*CREATE\s+OR\s+REPLACE\b`)
)

// ─── ScanMigrationSource ─────────────────────────────────────────────────────

// ScanMigrationSource walks dir, parses every .sql file, and returns one
// MigrationObject per unique DDL CREATE statement found. It tracks USE DATABASE
// / USE SCHEMA context across files and within files to infer the target
// database and schema for objects that are not fully qualified.
func (a *App) ScanMigrationSource(dir string) ([]MigrationObject, error) {
	// keyed by db+\x00+schema+\x00+kind+\x00+name+\x00+argSig (last wins)
	seen := make(map[string]MigrationObject)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable directories
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".sql") {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // silently skip unreadable files
		}

		stmts := ddl.Split(string(raw))

		var ctxDB, ctxSch string

		for _, stmt := range stmts {
			// Track USE DATABASE context
			if m := migrationUseDatabaseRE.FindStringSubmatch(stmt); m != nil {
				ctxDB = strings.ToUpper(m[1])
				ctxSch = "" // switching DB resets schema context
				continue
			}
			// Track USE SCHEMA context
			if m := migrationUseSchemaRE.FindStringSubmatch(stmt); m != nil {
				ctxSch = strings.ToUpper(m[1])
				continue
			}

			obj := ddl.Parse(stmt)
			if obj.Kind == ddl.KindUnknown {
				continue
			}

			mo := MigrationObject{
				FilePath:   path,
				ObjectKind: string(obj.Kind),
				ObjectName: obj.Name,
				ArgSig:     obj.ArgSig,
				DDL:        obj.SQL,
				IsReplace:  migrationIsReplaceRE.MatchString(stmt),
			}

			// Resolve database/schema from object ident, falling back to USE context
			db := obj.Database
			if db == "" {
				db = ctxDB
			}
			schema := obj.Schema
			if schema == "" {
				schema = ctxSch
			}
			mo.Database = strings.ToUpper(db)
			mo.Schema = strings.ToUpper(schema)

			key := remoteKey(mo.Database, mo.Schema, mo.ObjectKind, mo.ObjectName, mo.ArgSig)
			seen[key] = mo
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]MigrationObject, 0, len(seen))
	for _, mo := range seen {
		result = append(result, mo)
	}
	return result, nil
}

// ─── AnalyzeMigration ────────────────────────────────────────────────────────

// AnalyzeMigration compares the supplied local objects against what is currently
// in Snowflake and returns a diff for each object. It emits
// migration:analyze:progress events while working.
//
// Remote DDL is sourced exclusively from GET_DDL('database', X, true) rather
// than per-object GET_DDL calls.  The database-level dump is preferred because:
//   - It is the canonical form Snowflake produces for export/clone workflows.
//   - Per-object GET_DDL for streams wraps ON TABLE references as a single
//     double-quoted identifier ("DB.SCHEMA.TABLE") which is syntactically
//     incorrect and doesn't match the unqualified form in the database dump.
//   - One query per database is more efficient than N per-object queries.
func (a *App) AnalyzeMigration(objects []MigrationObject, database string) ([]MigrationDiffItem, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.migrationCancelFunc = cancel
	defer func() {
		cancel()
		a.migrationCancelFunc = nil
	}()

	// Fill blank databases from the parameter
	dbFallback := strings.ToUpper(database)
	for i := range objects {
		if objects[i].Database == "" {
			objects[i].Database = dbFallback
		}
	}

	// ── Collect unique databases to fetch ────────────────────────────────────

	checkDBs := make(map[string]bool)
	for _, mo := range objects {
		if strings.ToUpper(mo.ObjectKind) == "DATABASE" {
			if mo.ObjectName != "" {
				checkDBs[strings.ToUpper(mo.ObjectName)] = true
			}
		} else if mo.Database != "" {
			checkDBs[strings.ToUpper(mo.Database)] = true
		}
	}

	dbList := make([]string, 0, len(checkDBs))
	for db := range checkDBs {
		dbList = append(dbList, db)
	}

	// ── Fetch database-level DDL and parse into a remote object map ───────────
	//
	// remoteDDLMap: remoteKey → canonical DDL statement text
	// existingDBs:  set of database names that exist remotely
	// existingSchemas: set of "DB\x00SCHEMA" keys that exist remotely

	remoteDDLMap := make(map[string]string) // remoteKey → DDL text
	existingDBs := make(map[string]bool)
	existingSchemas := make(map[string]bool)
	var remoteMu sync.Mutex

	var fetchWg sync.WaitGroup
	for db := range checkDBs {
		fetchWg.Add(1)
		go func(db string) {
			defer fetchWg.Done()

			escapedDB := strings.ReplaceAll(db, "'", "''")
			result, err := a.client.Execute(ctx,
				fmt.Sprintf("SELECT GET_DDL('database', '%s', true)", escapedDB),
			)
			if err != nil {
				return
			}
			if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
				return
			}
			ddlText, ok := result.Rows[0][0].(string)
			if !ok || ddlText == "" {
				return
			}



			stmts := ddl.Split(ddlText)

			remoteMu.Lock()
			defer remoteMu.Unlock()

			existingDBs[db] = true

			for _, stmt := range stmts {
				obj := ddl.Parse(stmt)
				if obj.Kind == ddl.KindUnknown {
					continue
				}
				objDB := strings.ToUpper(obj.Database)
				if objDB == "" {
					objDB = db
				}
				objSchema := strings.ToUpper(obj.Schema)
				objName := strings.ToUpper(obj.Name)
				objKind := strings.ToUpper(string(obj.Kind))

				if obj.Kind == ddl.KindSchema {
					// For SCHEMA objects obj.Name is the schema name itself
					existingSchemas[objDB+"\x00"+objName] = true
				}

				k := remoteKey(objDB, objSchema, objKind, objName, strings.ToUpper(obj.ArgSig))
				remoteDDLMap[k] = stmt
			}

		}(db)
	}
	fetchWg.Wait()


	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Build local key set for "removed" detection
	localKeys := make(map[string]bool, len(objects))
	for _, mo := range objects {
		localKeys[remoteKey(mo.Database, mo.Schema, mo.ObjectKind, mo.ObjectName, mo.ArgSig)] = true
	}

	// ── Worker pool: classify each local object ───────────────────────────────
	//
	// remoteDDLMap is read-only from here on — no mutex needed in workers.

	total := len(objects)
	var doneCount atomic.Int32
	results := make([]MigrationDiffItem, len(objects))

	work := make(chan int, len(objects))
	for i := range objects {
		work <- i
	}
	close(work)

	const workers = 16
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				if ctx.Err() != nil {
					return
				}
				mo := objects[idx]
				upperKind := strings.ToUpper(mo.ObjectKind)

				item := MigrationDiffItem{
					Object:   mo,
					LocalDDL: mo.DDL,
				}

				switch upperKind {
				case "DATABASE":
					// The database-level DDL dump can't meaningfully diff a bare
					// CREATE DATABASE statement — just report existence.
					if existingDBs[strings.ToUpper(mo.ObjectName)] {
						item.Status = "unchanged"
					} else {
						item.Status = "new"
					}

				case "SCHEMA":
					// Similarly, the nested dump makes SCHEMA DDL incomparable.
					schemaKey := strings.ToUpper(mo.Database) + "\x00" + strings.ToUpper(mo.ObjectName)
					if existingSchemas[schemaKey] {
						item.Status = "unchanged"
					} else {
						item.Status = "new"
					}

				default:
					k := remoteKey(mo.Database, mo.Schema, mo.ObjectKind, mo.ObjectName, mo.ArgSig)
					remoteDDL, exists := remoteDDLMap[k]
					if !exists {
						item.Status = "new"
					} else {
						item.RemoteDDL = remoteDDL
						if normalizeDDL(mo.DDL) == normalizeDDL(remoteDDL) {
							item.Status = "unchanged"
						} else {
							item.Status = "changed"
						}
					}
				}

				results[idx] = item
				n := int(doneCount.Add(1))
				wailsruntime.EventsEmit(a.ctx, migrationAnalyzeProgressEvent, MigrationAnalyzeProgress{
					Done:  n,
					Total: total,
				})
			}
		}()
	}
	wg.Wait()

	// Count outcomes for the log summary
	var cntNew, cntChanged, cntUnchanged int
	for _, r := range results {
		switch r.Status {
		case "new":
			cntNew++
		case "changed":
			cntChanged++
		case "unchanged":
			cntUnchanged++
		}
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// ── "Removed" items: remote objects absent from the local set ─────────────
	//
	// DATABASE and SCHEMA are structural containers — we never suggest removing
	// them automatically.
	for k, stmt := range remoteDDLMap {
		if localKeys[k] {
			continue
		}
		parts := strings.SplitN(k, "\x00", 5)
		if len(parts) != 5 {
			continue
		}
		kind := parts[2]
		if kind == "DATABASE" || kind == "SCHEMA" {
			continue
		}
		results = append(results, MigrationDiffItem{
			Object: MigrationObject{
				Database:   parts[0],
				Schema:     parts[1],
				ObjectKind: kind,
				ObjectName: parts[3],
				ArgSig:     parts[4],
			},
			Status:    "removed",
			RemoteDDL: stmt,
		})
	}

	// Sort: new(0) < changed(1) < unchanged(2) < removed(3)
	statusOrder := map[string]int{"new": 0, "changed": 1, "unchanged": 2, "removed": 3}
	sort.SliceStable(results, func(i, j int) bool {
		return statusOrder[results[i].Status] < statusOrder[results[j].Status]
	})

	// Count removed items that were appended after the worker pass
	cntRemoved := 0
	for _, r := range results {
		if r.Status == "removed" {
			cntRemoved++
		}
	}

	return results, nil
}

// ─── CreateMigrationSnapshot ─────────────────────────────────────────────────

// CreateMigrationSnapshot optionally creates a backup set and/or a zero-copy
// clone of the target database as a safety net before deployment.
func (a *App) CreateMigrationSnapshot(database, backupSetDB, backupSetSchema, backupSetName string, doBackup bool, cloneDB string, doClone bool) error {
	if a.client == nil {
		return ErrNotConnected
	}

	ctx := context.Background()
	q := func(s string) string {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}

	if doBackup && backupSetName != "" {
		fqn := fmt.Sprintf("%s.%s.%s", q(backupSetDB), q(backupSetSchema), q(backupSetName))
		sql := fmt.Sprintf("CREATE OR REPLACE BACKUP SET %s FOR DATABASE %s", fqn, q(database))
		if _, err := a.client.Execute(ctx, sql); err != nil {
			return fmt.Errorf("create backup set: %w", err)
		}
	}

	if doClone && cloneDB != "" {
		sql := fmt.Sprintf("CREATE OR REPLACE DATABASE %s CLONE %s", q(cloneDB), q(database))
		if _, err := a.client.Execute(ctx, sql); err != nil {
			return fmt.Errorf("create clone: %w", err)
		}
	}

	return nil
}

// ─── ExecuteMigration ────────────────────────────────────────────────────────

// ExecuteMigration deploys the selected objects to Snowflake in dependency order
// with up to maxPasses retry passes for objects that fail due to dependency
// errors. It emits migration:exec:progress events throughout.
func (a *App) ExecuteMigration(selected []MigrationObject, database string, maxPasses int) ([]MigrationExecEvent, error) {
	if a.client == nil {
		return nil, ErrNotConnected
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.migrationCancelFunc = cancel
	defer func() {
		cancel()
		a.migrationCancelFunc = nil
	}()

	if maxPasses <= 0 {
		maxPasses = 5
	}

	dbFallback := strings.ToUpper(database)
	for i := range selected {
		if selected[i].Database == "" {
			selected[i].Database = dbFallback
		}
	}

	// Sort by execution priority
	sorted := make([]MigrationObject, len(selected))
	copy(sorted, selected)
	sort.SliceStable(sorted, func(i, j int) bool {
		return executionPriority(sorted[i].ObjectKind) < executionPriority(sorted[j].ObjectKind)
	})

	total := len(sorted)
	var allEvents []MigrationExecEvent
	var doneCount int

	q := func(s string) string {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}

	work := sorted

	for pass := 1; pass <= maxPasses && len(work) > 0; pass++ {
		if ctx.Err() != nil {
			break
		}
		var nextPass []MigrationObject

		for _, mo := range work {
			if ctx.Err() != nil {
				break
			}

			label := fmt.Sprintf("%s.%s.%s.%s", mo.Database, mo.Schema, mo.ObjectKind, mo.ObjectName)

			// Emit running
			runEvt := MigrationExecEvent{
				Done:   doneCount,
				Total:  total,
				Object: label,
				Status: "running",
				Pass:   pass,
			}
			allEvents = append(allEvents, runEvt)
			wailsruntime.EventsEmit(a.ctx, migrationExecProgressEvent, runEvt)

			// Set database context if known
			if mo.Database != "" {
				if _, err := a.client.Execute(ctx, fmt.Sprintf("USE DATABASE %s", q(mo.Database))); err != nil {
					// Non-fatal; proceed anyway
				}
			}

			_, execErr := a.client.Execute(ctx, mo.DDL)

			if execErr == nil {
				doneCount++
				evt := MigrationExecEvent{
					Done:   doneCount,
					Total:  total,
					Object: label,
					Status: "success",
					Pass:   pass,
				}
				allEvents = append(allEvents, evt)
				wailsruntime.EventsEmit(a.ctx, migrationExecProgressEvent, evt)
			} else if isDependencyError(execErr.Error()) && pass < maxPasses {
				nextPass = append(nextPass, mo)
				evt := MigrationExecEvent{
					Done:   doneCount,
					Total:  total,
					Object: label,
					Status: "skipped",
					Error:  execErr.Error(),
					Pass:   pass,
				}
				allEvents = append(allEvents, evt)
				wailsruntime.EventsEmit(a.ctx, migrationExecProgressEvent, evt)
			} else {
				doneCount++
				evt := MigrationExecEvent{
					Done:   doneCount,
					Total:  total,
					Object: label,
					Status: "failed",
					Error:  execErr.Error(),
					Pass:   pass,
				}
				allEvents = append(allEvents, evt)
				wailsruntime.EventsEmit(a.ctx, migrationExecProgressEvent, evt)
			}
		}

		work = nextPass
	}

	// Any remaining after max passes → failed
	for _, mo := range work {
		label := fmt.Sprintf("%s.%s.%s.%s", mo.Database, mo.Schema, mo.ObjectKind, mo.ObjectName)
		doneCount++
		evt := MigrationExecEvent{
			Done:   doneCount,
			Total:  total,
			Object: label,
			Status: "failed",
			Error:  "max retry passes exhausted",
			Pass:   maxPasses,
		}
		allEvents = append(allEvents, evt)
		wailsruntime.EventsEmit(a.ctx, migrationExecProgressEvent, evt)
	}

	return allEvents, nil
}

// ─── CancelMigration ─────────────────────────────────────────────────────────

// CancelMigration cancels any in-flight AnalyzeMigration or ExecuteMigration.
func (a *App) CancelMigration() error {
	if a.migrationCancelFunc != nil {
		a.migrationCancelFunc()
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// remoteKey builds a null-byte-separated, all-uppercase lookup key.
func remoteKey(db, schema, kind, name, argSig string) string {
	return strings.ToUpper(db) + "\x00" +
		strings.ToUpper(schema) + "\x00" +
		strings.ToUpper(kind) + "\x00" +
		strings.ToUpper(name) + "\x00" +
		strings.ToUpper(argSig)
}

// normalizeDDL strips comments, collapses whitespace, uppercases, and strips
// any trailing semicolon so that cosmetic formatting differences — including
// the trailing ";" that Snowflake's GET_DDL appends but local files omit —
// don't register as spurious changes.
func normalizeDDL(sql string) string {
	s := migrationBlockCommentRE.ReplaceAllString(sql, " ")
	s = migrationLineCommentRE.ReplaceAllString(s, " ")
	s = migrationWhitespaceRE.ReplaceAllString(s, " ")
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimRight(s, ";")
	s = strings.TrimSpace(s)
	return s
}

// executionPriority returns the deployment order for a given object kind.
// Lower numbers are deployed first.
func executionPriority(kind string) int {
	switch strings.ToUpper(kind) {
	case "DATABASE":
		return 0
	case "SCHEMA":
		return 1
	case "SEQUENCE":
		return 2
	case "TABLE":
		return 3
	case "FILE FORMAT":
		return 4
	case "STAGE":
		return 5
	case "VIEW":
		return 6
	case "MATERIALIZED VIEW":
		return 7
	case "FUNCTION":
		return 8
	case "PROCEDURE":
		return 9
	case "STREAM":
		return 10
	case "TASK":
		return 11
	case "PIPE":
		return 12
	default:
		return 99
	}
}

// isDependencyError reports whether an error message indicates a dependency
// issue that may resolve on a subsequent pass.
//
// Snowflake emits several patterns depending on object type:
//   - "Object 'X' does not exist or not authorized."
//   - "Table 'X' does not exist or not authorized."
//   - "View 'X' does not exist or not authorized."
//   - "002003 (42S02): SQL compilation error: Object 'X' does not exist…"
//
// We match on the common suffix "does not exist" (covers all variants) and
// the separate "not authorized" phrase, which can appear without "does not
// exist" when privilege grants are missing.
func isDependencyError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "not authorized")
}
