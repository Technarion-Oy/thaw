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

	// column-parsing patterns used by table migration strategies
	migrConstraintPrefixRE = regexp.MustCompile(
		`(?i)^\s*(CONSTRAINT|PRIMARY\s+KEY|UNIQUE|CHECK|FOREIGN\s+KEY|CLUSTER\s+BY)`)
	migrColNameRE = regexp.MustCompile(`(?i)^\s*"?([A-Za-z_][A-Za-z0-9_$]*)"?\s+\S`)
	migrColTypeRE = regexp.MustCompile(`(?i)^\w+(?:\s*\([^)]*\))?`)
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
func (a *App) ExecuteMigration(selected []MigrationObject, database string, maxPasses int, strategy TableMigrationStrategy) ([]MigrationExecEvent, error) {
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
				if _, err := a.client.Execute(ctx, fmt.Sprintf("USE DATABASE %s", migrQuote(mo.Database))); err != nil {
					// Non-fatal; proceed anyway
				}
			}

			execErr := a.executeMigrationObject(ctx, mo, strategy)

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

// ─── TableMigrationStrategy ───────────────────────────────────────────────────

// TableMigrationStrategy controls how TABLE objects that already exist in
// Snowflake are handled during migration. It has no effect on non-TABLE objects,
// which are always executed via their raw DDL.
type TableMigrationStrategy string

const (
	// StrategyInPlace applies ALTER TABLE ADD/DROP/ALTER COLUMN statements to
	// the existing table without touching unaffected rows.
	StrategyInPlace TableMigrationStrategy = "in_place"

	// StrategyBlueGreenSwap creates a temporary table with the new schema,
	// copies shared columns from the original, then atomically swaps the two
	// tables with ALTER TABLE … SWAP WITH. Columns absent from the new schema
	// are discarded; all other rows are preserved.
	StrategyBlueGreenSwap TableMigrationStrategy = "blue_green_swap"

	// StrategyViewAbstraction renames the original table to <name>_v1, creates
	// the new table with the updated schema, and creates a compatibility view
	// <name>_compat that exposes shared columns from the archived data.
	StrategyViewAbstraction TableMigrationStrategy = "view_abstraction"

	// StrategyDestructiveRebuild drops the existing table before recreating it.
	// All existing data is permanently lost.
	StrategyDestructiveRebuild TableMigrationStrategy = "destructive_rebuild"
)

// ─── column helpers ──────────────────────────────────────────────────────────

// migrColDef is a column name plus its normalised type expression.
type migrColDef struct {
	Name     string // upper-cased, unquoted
	TypeExpr string // base type only, e.g. "VARCHAR(255)" or "NUMBER(38,0)"
}

// migrTempCounter generates unique temp-table name suffixes within a session.
var migrTempCounter atomic.Int64

// migrTempName returns a temporary table name unlikely to collide with
// existing objects.
func migrTempName(base string) string {
	n := migrTempCounter.Add(1)
	if len(base) > 50 {
		base = base[:50]
	}
	return fmt.Sprintf("%s__thaw_tmp_%d", base, n)
}

// migrQuote double-quote-escapes a Snowflake identifier.
func migrQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// parseLocalTableColumns extracts column definitions from a CREATE TABLE DDL
// statement. Constraint and clustering clauses are skipped.
func parseLocalTableColumns(ddl string) []migrColDef {
	// Locate the outermost parenthesised block.
	start := strings.IndexByte(ddl, '(')
	if start < 0 {
		return nil
	}
	depth, end := 0, -1
	for i := start; i < len(ddl); i++ {
		switch ddl[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}

	var cols []migrColDef
	for _, part := range splitTopLevel(ddl[start+1:end], ',') {
		part = strings.TrimSpace(part)
		if part == "" || migrConstraintPrefixRE.MatchString(part) {
			continue
		}
		locs := migrColNameRE.FindStringSubmatchIndex(part)
		if locs == nil {
			continue
		}
		name := strings.ToUpper(part[locs[2]:locs[3]]) // capture group [1]
		// The full match ends at locs[1]; the trailing \S in the pattern is the
		// first character of the type expression — start rest from there so the
		// closing " of a quoted identifier is not included in the type clause.
		rest := strings.TrimSpace(part[locs[1]-1:])
		// Extract only the base type (strip NOT NULL, DEFAULT, etc.)
		typeExpr := ""
		if tm := migrColTypeRE.FindString(rest); tm != "" {
			typeExpr = strings.ToUpper(strings.TrimSpace(tm))
		} else if len(strings.Fields(rest)) > 0 {
			typeExpr = strings.ToUpper(strings.Fields(rest)[0])
		}
		if typeExpr != "" {
			cols = append(cols, migrColDef{Name: name, TypeExpr: typeExpr})
		}
	}
	return cols
}

// splitTopLevel splits s on sep, respecting nested parentheses.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if s[i] == sep && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// commonColumnNames returns column names from a that also appear in b,
// preserving the order from a.
func commonColumnNames(a, b []migrColDef) []string {
	set := make(map[string]bool, len(b))
	for _, c := range b {
		set[c.Name] = true
	}
	var out []string
	for _, c := range a {
		if set[c.Name] {
			out = append(out, c.Name)
		}
	}
	return out
}

// replaceDDLTableName rewrites the table identifier in a CREATE TABLE DDL so
// it references db.schema.newName. Handles quoted/unquoted and qualified names
// as well as CREATE OR REPLACE [TRANSIENT] TABLE variants.
func replaceDDLTableName(ddl, db, schema, newName string) string {
	newFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(newName)

	// Split at the first '(' to isolate the header.
	parenIdx := strings.IndexByte(ddl, '(')
	if parenIdx < 0 {
		return ddl
	}
	header := ddl[:parenIdx]
	body := ddl[parenIdx:]

	// Find the last "TABLE" keyword in the header (handles TRANSIENT TABLE).
	upper := strings.ToUpper(header)
	tablePos := strings.LastIndex(upper, "TABLE")
	if tablePos < 0 {
		return ddl
	}

	// Skip whitespace after TABLE to find the start of the identifier.
	i := tablePos + 5
	for i < len(header) && (header[i] == ' ' || header[i] == '\t') {
		i++
	}
	identStart := i

	// Scan the identifier, respecting double-quoted segments.
	inQuote := false
	for i < len(header) {
		ch := header[i]
		if ch == '"' {
			inQuote = !inQuote
		} else if !inQuote && (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') {
			break
		}
		i++
	}
	identEnd := i

	return header[:identStart] + newFQN + header[identEnd:] + body
}

// ─── Snowflake introspection helpers ─────────────────────────────────────────

// tableExists reports whether a BASE TABLE with the given name exists.
// Uses INFORMATION_SCHEMA to avoid triggering gosnowflake driver error logs
// when the table is absent.
func (a *App) tableExists(ctx context.Context, db, schema, name string) (bool, error) {
	sql := fmt.Sprintf(
		"SELECT COUNT(*) FROM %s.INFORMATION_SCHEMA.TABLES"+
			" WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'"+
			" AND TABLE_TYPE = 'BASE TABLE'",
		migrQuote(db),
		strings.ReplaceAll(strings.ToUpper(schema), "'", "''"),
		strings.ReplaceAll(strings.ToUpper(name), "'", "''"),
	)
	res, err := a.client.Execute(ctx, sql)
	if err != nil {
		return false, err
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return false, nil
	}
	switch v := res.Rows[0][0].(type) {
	case int64:
		return v > 0, nil
	case float64:
		return v > 0, nil
	case string:
		return v != "0" && v != "", nil
	}
	return false, nil
}

// describeTableColumns runs DESCRIBE TABLE and returns the column definitions.
// Only rows with kind = "Column" are included.
func (a *App) describeTableColumns(ctx context.Context, db, schema, table string) ([]migrColDef, error) {
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(table)
	res, err := a.client.Execute(ctx, "DESCRIBE TABLE "+fqn)
	if err != nil {
		return nil, err
	}
	var cols []migrColDef
	for _, row := range res.Rows {
		if len(row) < 3 {
			continue
		}
		kind, _ := row[2].(string)
		if !strings.EqualFold(kind, "Column") {
			continue
		}
		colName, _ := row[0].(string)
		typeExpr, _ := row[1].(string)
		cols = append(cols, migrColDef{
			Name:     strings.ToUpper(colName),
			TypeExpr: strings.ToUpper(typeExpr),
		})
	}
	return cols, nil
}

// tableRowCount returns the row count for a table using SHOW TABLES.
// It returns 0 without error when the table is not found or the count cannot
// be determined, so callers should treat any error as "unknown" rather than
// "empty".
func (a *App) tableRowCount(ctx context.Context, db, schema, name string) (int64, error) {
	// LIKE matching is case-insensitive; escape LIKE metacharacters in the name.
	escaped := strings.ReplaceAll(name, "'", "''")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	sql := fmt.Sprintf(
		"SHOW TABLES LIKE '%s' IN SCHEMA %s.%s",
		escaped, migrQuote(db), migrQuote(schema),
	)
	res, err := a.client.Execute(ctx, sql)
	if err != nil {
		return 0, err
	}
	if len(res.Rows) == 0 {
		return 0, nil
	}

	// Locate "name" and "rows" column indices dynamically — SHOW output can vary.
	nameIdx, rowsIdx := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "rows":
			rowsIdx = i
		}
	}
	if rowsIdx < 0 {
		return 0, nil
	}

	// SHOW TABLES LIKE can match multiple tables (e.g. MY_TABLE and MY1TABLE);
	// scan all rows and match on the exact name.
	for _, row := range res.Rows {
		if nameIdx >= 0 && len(row) > nameIdx {
			rowName, _ := row[nameIdx].(string)
			if !strings.EqualFold(rowName, name) {
				continue
			}
		}
		if len(row) <= rowsIdx {
			continue
		}
		switch v := row[rowsIdx].(type) {
		case int64:
			return v, nil
		case float64:
			return int64(v), nil
		case string:
			var n int64
			_, _ = fmt.Sscanf(v, "%d", &n)
			return n, nil
		}
	}
	return 0, nil
}

// ─── strategy dispatch ────────────────────────────────────────────────────────

// executeMigrationObject runs the appropriate strategy for a single migration
// object. Non-TABLE objects are always executed via their raw DDL. TABLE objects
// use the chosen strategy only when the table already exists; brand-new tables
// are always created via their DDL directly.
func (a *App) executeMigrationObject(ctx context.Context, mo MigrationObject, strategy TableMigrationStrategy) error {
	if strings.ToUpper(mo.ObjectKind) != "TABLE" {
		_, err := a.client.Execute(ctx, mo.DDL)
		return err
	}

	exists, err := a.tableExists(ctx, mo.Database, mo.Schema, mo.ObjectName)
	if err != nil {
		// Cannot determine existence; fall back to direct execution.
		_, execErr := a.client.Execute(ctx, mo.DDL)
		return execErr
	}
	if !exists {
		_, err = a.client.Execute(ctx, mo.DDL)
		return err
	}

	// Empty table: data-preserving strategies add no value — use a fast
	// CREATE OR REPLACE path instead (DROP + CREATE so the DDL is applied as-is
	// even when the local file lacks OR REPLACE).
	rowCount, rcErr := a.tableRowCount(ctx, mo.Database, mo.Schema, mo.ObjectName)
	if rcErr == nil && rowCount == 0 {
		return a.executeDestructiveRebuild(ctx, mo)
	}

	// Existing non-empty table: apply chosen strategy.
	switch strategy {
	case StrategyBlueGreenSwap:
		return a.executeBlueGreenSwap(ctx, mo)
	case StrategyViewAbstraction:
		return a.executeViewAbstraction(ctx, mo)
	case StrategyDestructiveRebuild:
		return a.executeDestructiveRebuild(ctx, mo)
	default: // StrategyInPlace
		return a.executeInPlace(ctx, mo)
	}
}

// ─── strategy: in_place ───────────────────────────────────────────────────────

// executeInPlace diffs local column definitions against the remote schema and
// applies ALTER TABLE ADD/DROP/ALTER COLUMN TYPE statements.
func (a *App) executeInPlace(ctx context.Context, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)

	remoteCols, err := a.describeTableColumns(ctx, db, schema, name)
	if err != nil {
		return fmt.Errorf("describe remote columns: %w", err)
	}
	localCols := parseLocalTableColumns(mo.DDL)
	if len(localCols) == 0 {
		// Cannot parse local DDL; fall back to direct execution.
		_, execErr := a.client.Execute(ctx, mo.DDL)
		return execErr
	}

	remoteByName := make(map[string]migrColDef, len(remoteCols))
	for _, c := range remoteCols {
		remoteByName[c.Name] = c
	}
	localByName := make(map[string]migrColDef, len(localCols))
	for _, c := range localCols {
		localByName[c.Name] = c
	}

	// ADD new columns.
	for _, c := range localCols {
		if _, exists := remoteByName[c.Name]; !exists {
			sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", fqn, migrQuote(c.Name), c.TypeExpr)
			if _, err = a.client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("add column %s: %w", c.Name, err)
			}
		}
	}

	// DROP removed columns.
	for _, c := range remoteCols {
		if _, exists := localByName[c.Name]; !exists {
			sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", fqn, migrQuote(c.Name))
			if _, err = a.client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("drop column %s: %w", c.Name, err)
			}
		}
	}

	// ALTER COLUMN TYPE for changed columns.
	for _, lc := range localCols {
		rc, exists := remoteByName[lc.Name]
		if !exists {
			continue // just added above
		}
		if !strings.EqualFold(lc.TypeExpr, rc.TypeExpr) {
			sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", fqn, migrQuote(lc.Name), lc.TypeExpr)
			if _, err = a.client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("alter column %s type: %w", lc.Name, err)
			}
		}
	}

	return nil
}

// ─── strategy: blue_green_swap ────────────────────────────────────────────────

// executeBlueGreenSwap creates a temporary table with the new schema (by
// rewriting the table name in the local DDL), copies shared columns from the
// original, atomically swaps the two tables, then drops the temp.
func (a *App) executeBlueGreenSwap(ctx context.Context, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	tmpName := migrTempName(name)
	origFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	tmpFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(tmpName)

	// 1. Create temporary table with new DDL (same schema, different name).
	tmpDDL := replaceDDLTableName(mo.DDL, db, schema, tmpName)
	if _, err := a.client.Execute(ctx, tmpDDL); err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	dropTmp := func() { _, _ = a.client.Execute(ctx, "DROP TABLE IF EXISTS "+tmpFQN) }

	// 2. Determine shared columns: remote (old) ∩ new.
	remoteCols, err := a.describeTableColumns(ctx, db, schema, name)
	if err != nil {
		dropTmp()
		return fmt.Errorf("describe remote columns: %w", err)
	}
	newCols, err := a.describeTableColumns(ctx, db, schema, tmpName)
	if err != nil {
		dropTmp()
		return fmt.Errorf("describe new columns: %w", err)
	}
	commonNames := commonColumnNames(remoteCols, newCols)

	// 3. Copy shared columns from original into temp.
	if len(commonNames) > 0 {
		var colList strings.Builder
		for i, n := range commonNames {
			if i > 0 {
				colList.WriteString(", ")
			}
			colList.WriteString(migrQuote(n))
		}
		cols := colList.String()
		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", tmpFQN, cols, cols, origFQN)
		if _, err = a.client.Execute(ctx, insertSQL); err != nil {
			dropTmp()
			return fmt.Errorf("copy data: %w", err)
		}
	}

	// 4. Atomically swap: original now has new schema + copied rows.
	if _, err = a.client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s SWAP WITH %s", origFQN, tmpFQN)); err != nil {
		dropTmp()
		return fmt.Errorf("swap tables: %w", err)
	}

	// 5. Drop temp (now holds the old schema/data).
	dropTmp()
	return nil
}

// ─── strategy: view_abstraction ───────────────────────────────────────────────

// executeViewAbstraction renames the existing table to <name>_v1, creates the
// new table from local DDL, and creates a compatibility view <name>_compat that
// exposes shared columns from the archived data.
func (a *App) executeViewAbstraction(ctx context.Context, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	archiveName := name + "_v1"
	origFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	archiveFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(archiveName)
	compatFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name+"_compat")

	// 1. Capture remote columns before renaming.
	remoteCols, err := a.describeTableColumns(ctx, db, schema, name)
	if err != nil {
		return fmt.Errorf("describe remote columns: %w", err)
	}

	// 2. Rename original table to archive.
	if _, err = a.client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", origFQN, archiveFQN)); err != nil {
		return fmt.Errorf("rename table: %w", err)
	}

	// 3. Create new table from local DDL.
	if _, err = a.client.Execute(ctx, mo.DDL); err != nil {
		// Roll back the rename.
		_, _ = a.client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", archiveFQN, origFQN))
		return fmt.Errorf("create new table: %w", err)
	}

	// 4. Create a compatibility view over the shared columns.
	localCols := parseLocalTableColumns(mo.DDL)
	commonNames := commonColumnNames(remoteCols, localCols)
	if len(commonNames) > 0 {
		var colList strings.Builder
		for i, n := range commonNames {
			if i > 0 {
				colList.WriteString(", ")
			}
			colList.WriteString(migrQuote(n))
		}
		viewSQL := fmt.Sprintf(
			"CREATE OR REPLACE VIEW %s AS SELECT %s FROM %s",
			compatFQN, colList.String(), archiveFQN,
		)
		if _, err = a.client.Execute(ctx, viewSQL); err != nil {
			return fmt.Errorf("create compat view: %w", err)
		}
	}

	return nil
}

// ─── strategy: destructive_rebuild ────────────────────────────────────────────

// executeDestructiveRebuild drops the existing table and recreates it from
// local DDL. All existing data is permanently lost.
func (a *App) executeDestructiveRebuild(ctx context.Context, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	if _, err := a.client.Execute(ctx, "DROP TABLE IF EXISTS "+fqn); err != nil {
		return fmt.Errorf("drop table: %w", err)
	}
	if _, err := a.client.Execute(ctx, mo.DDL); err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}

// ─── GenerateMigrationScript ──────────────────────────────────────────────────

// GenerateMigrationScript builds a human-readable SQL script for the supplied
// diff items using the chosen table migration strategy. It does not require an
// active Snowflake connection because all column information is derived from the
// DDL text already present in the diff items.
func (a *App) GenerateMigrationScript(items []MigrationDiffItem, database string, strategy TableMigrationStrategy) (string, error) {
	return buildMigrationScript(items, database, strategy), nil
}

func buildMigrationScript(items []MigrationDiffItem, database string, strategy TableMigrationStrategy) string {
	dbFallback := strings.ToUpper(database)

	sorted := make([]MigrationDiffItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		return executionPriority(sorted[i].Object.ObjectKind) < executionPriority(sorted[j].Object.ObjectKind)
	})

	var sb strings.Builder
	sb.WriteString("-- Schema Migration Script\n-- Generated by Thaw\n\n")

	header := sb.Len()
	var currentDB string

	for _, item := range sorted {
		if item.Status == "unchanged" || item.Status == "removed" {
			continue
		}

		mo := item.Object
		db := mo.Database
		if db == "" {
			db = dbFallback
		}

		if db != "" && strings.ToUpper(db) != currentDB {
			if sb.Len() > header {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "USE DATABASE %s;\n\n", migrQuote(db))
			currentDB = strings.ToUpper(db)
		}

		fqn := migrQuote(db) + "." + migrQuote(mo.Schema) + "." + migrQuote(mo.ObjectName)
		ddl := strings.TrimRight(mo.DDL, ";\n ")

		if strings.ToUpper(mo.ObjectKind) != "TABLE" || item.Status == "new" {
			fmt.Fprintf(&sb, "%s;\n\n", ddl)
			continue
		}

		// Existing TABLE: emit strategy-specific SQL.
		switch strategy {
		case StrategyInPlace:
			scriptInPlace(&sb, fqn, mo.ObjectName, item.LocalDDL, item.RemoteDDL)
		case StrategyBlueGreenSwap:
			scriptBlueGreen(&sb, fqn, db, mo.Schema, mo.ObjectName, item.LocalDDL, item.RemoteDDL, ddl)
		case StrategyViewAbstraction:
			scriptViewAbstraction(&sb, fqn, db, mo.Schema, mo.ObjectName, item.LocalDDL, item.RemoteDDL, ddl)
		case StrategyDestructiveRebuild:
			fmt.Fprintf(&sb, "-- Destructive Rebuild: %s\n", mo.ObjectName)
			fmt.Fprintf(&sb, "DROP TABLE IF EXISTS %s;\n%s;\n\n", fqn, ddl)
		}
	}

	return sb.String()
}

func scriptInPlace(sb *strings.Builder, fqn, objectName, localDDL, remoteDDL string) {
	remoteCols := parseLocalTableColumns(remoteDDL)
	localCols := parseLocalTableColumns(localDDL)
	if len(localCols) == 0 {
		fmt.Fprintf(sb, "-- Could not parse local columns for %s; manual review required.\n%s;\n\n",
			objectName, strings.TrimRight(localDDL, ";\n "))
		return
	}

	remoteByName := make(map[string]migrColDef, len(remoteCols))
	for _, c := range remoteCols {
		remoteByName[c.Name] = c
	}
	localByName := make(map[string]migrColDef, len(localCols))
	for _, c := range localCols {
		localByName[c.Name] = c
	}

	fmt.Fprintf(sb, "-- Smart In-Place: %s\n", objectName)
	written := false
	for _, c := range localCols {
		if _, exists := remoteByName[c.Name]; !exists {
			fmt.Fprintf(sb, "ALTER TABLE %s ADD COLUMN %s %s;\n", fqn, migrQuote(c.Name), c.TypeExpr)
			written = true
		}
	}
	for _, c := range remoteCols {
		if _, exists := localByName[c.Name]; !exists {
			fmt.Fprintf(sb, "ALTER TABLE %s DROP COLUMN %s;\n", fqn, migrQuote(c.Name))
			written = true
		}
	}
	for _, lc := range localCols {
		rc, exists := remoteByName[lc.Name]
		if !exists {
			continue
		}
		if !strings.EqualFold(lc.TypeExpr, rc.TypeExpr) {
			fmt.Fprintf(sb, "ALTER TABLE %s ALTER COLUMN %s TYPE %s;\n", fqn, migrQuote(lc.Name), lc.TypeExpr)
			written = true
		}
	}
	if !written {
		fmt.Fprintf(sb, "-- (no column changes detected)\n")
	}
	sb.WriteString("\n")
}

func scriptBlueGreen(sb *strings.Builder, origFQN, db, schema, objectName, localDDL, remoteDDL, ddl string) {
	tmpName := objectName + "__migration_tmp"
	tmpFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(tmpName)
	tmpDDL := strings.TrimRight(replaceDDLTableName(ddl, db, schema, tmpName), ";\n ")

	remoteCols := parseLocalTableColumns(remoteDDL)
	localCols := parseLocalTableColumns(localDDL)
	commonNames := commonColumnNames(remoteCols, localCols)

	fmt.Fprintf(sb, "-- Blue/Green Swap: %s\n", objectName)
	fmt.Fprintf(sb, "%s;\n", tmpDDL)
	if len(commonNames) > 0 {
		var colList strings.Builder
		for i, n := range commonNames {
			if i > 0 {
				colList.WriteString(", ")
			}
			colList.WriteString(migrQuote(n))
		}
		cols := colList.String()
		fmt.Fprintf(sb, "INSERT INTO %s (%s)\n  SELECT %s FROM %s;\n", tmpFQN, cols, cols, origFQN)
	}
	fmt.Fprintf(sb, "ALTER TABLE %s SWAP WITH %s;\n", origFQN, tmpFQN)
	fmt.Fprintf(sb, "DROP TABLE IF EXISTS %s;\n\n", tmpFQN)
}

func scriptViewAbstraction(sb *strings.Builder, origFQN, db, schema, objectName, localDDL, remoteDDL, ddl string) {
	archiveFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(objectName+"_v1")
	compatFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(objectName+"_compat")

	remoteCols := parseLocalTableColumns(remoteDDL)
	localCols := parseLocalTableColumns(localDDL)
	commonNames := commonColumnNames(remoteCols, localCols)

	fmt.Fprintf(sb, "-- View Abstraction: %s\n", objectName)
	fmt.Fprintf(sb, "ALTER TABLE %s RENAME TO %s;\n%s;\n", origFQN, archiveFQN, ddl)
	if len(commonNames) > 0 {
		var colList strings.Builder
		for i, n := range commonNames {
			if i > 0 {
				colList.WriteString(", ")
			}
			colList.WriteString(migrQuote(n))
		}
		fmt.Fprintf(sb, "CREATE OR REPLACE VIEW %s AS SELECT %s FROM %s;\n", compatFQN, colList.String(), archiveFQN)
	}
	sb.WriteString("\n")
}
