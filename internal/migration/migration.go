// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package migration

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

	"thaw/internal/ddl"
	"thaw/internal/snowflake"
)

// ─── event name constants ─────────────────────────────────────────────────────

const (
	AnalyzeProgressEvent = "migration:analyze:progress"
	ExecProgressEvent    = "migration:exec:progress"
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

// MigrationAnalyzeProgress is emitted during Analyze.
type MigrationAnalyzeProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// MigrationExecEvent is emitted during Execute for each object processed.
type MigrationExecEvent struct {
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Object string `json:"object"` // "DB.SCHEMA.KIND.NAME"
	Status string `json:"status"` // "running"|"success"|"failed"|"skipped"
	Error  string `json:"error"`
	Pass   int    `json:"pass"`
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

// ─── module-level compiled regexps ───────────────────────────────────────────

var (
	blockCommentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineCommentRE  = regexp.MustCompile(`--[^\n]*`)
	whitespaceRE   = regexp.MustCompile(`\s+`)
	useDatabaseRE  = regexp.MustCompile(`(?i)^\s*USE\s+DATABASE\s+"?([^"\s;]+)"?`)
	useSchemaRE    = regexp.MustCompile(`(?i)^\s*USE\s+SCHEMA\s+"?([^"\s;]+)"?`)
	isReplaceRE    = regexp.MustCompile(`(?i)^\s*CREATE\s+OR\s+REPLACE\b`)

	// column-parsing patterns used by table migration strategies
	constraintPrefixRE = regexp.MustCompile(
		`(?i)^\s*(CONSTRAINT|PRIMARY\s+KEY|UNIQUE|CHECK|FOREIGN\s+KEY|CLUSTER\s+BY)`)
	colNameRE = regexp.MustCompile(`(?i)^\s*"?([A-Za-z_][A-Za-z0-9_$]*)"?\s+\S`)
	colTypeRE = regexp.MustCompile(`(?i)^\w+(?:\s*\([^)]*\))?`)
)

// ─── column helpers ──────────────────────────────────────────────────────────

// colDef is a column name plus its normalised type expression.
type colDef struct {
	Name     string // upper-cased, unquoted
	TypeExpr string // base type only, e.g. "VARCHAR(255)" or "NUMBER(38,0)"
}

// tempCounter generates unique temp-table name suffixes within a session.
var tempCounter atomic.Int64

// ─── Service ─────────────────────────────────────────────────────────────────

// Service manages schema migration operations.
type Service struct {
	cancelFunc context.CancelFunc
	emit       func(eventName string, data interface{})
}

// NewService creates a Service. The emit callback is used to send progress
// events (e.g. via wailsruntime.EventsEmit).
func NewService(emit func(eventName string, data interface{})) *Service {
	return &Service{emit: emit}
}

// ─── ScanSource ──────────────────────────────────────────────────────────────

// ScanSource walks dir, parses every .sql file, and returns one
// MigrationObject per unique DDL CREATE statement found. It tracks USE DATABASE
// / USE SCHEMA context across files and within files to infer the target
// database and schema for objects that are not fully qualified.
func (s *Service) ScanSource(dir string) ([]MigrationObject, error) {
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
			if m := useDatabaseRE.FindStringSubmatch(stmt); m != nil {
				ctxDB = strings.ToUpper(m[1])
				ctxSch = "" // switching DB resets schema context
				continue
			}
			// Track USE SCHEMA context
			if m := useSchemaRE.FindStringSubmatch(stmt); m != nil {
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
				IsReplace:  isReplaceRE.MatchString(stmt),
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

// ─── Analyze ─────────────────────────────────────────────────────────────────

// Analyze compares the supplied local objects against what is currently
// in Snowflake and returns a diff for each object. It emits
// migration:analyze:progress events while working.
func (s *Service) Analyze(client *snowflake.Client, objects []MigrationObject, database string) ([]MigrationDiffItem, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	defer func() {
		cancel()
		s.cancelFunc = nil
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

	// ── Fetch database-level DDL and parse into a remote object map ───────────

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
			result, err := client.Execute(ctx,
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
					if existingDBs[strings.ToUpper(mo.ObjectName)] {
						item.Status = "unchanged"
					} else {
						item.Status = "new"
					}

				case "SCHEMA":
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
				s.emit(AnalyzeProgressEvent, MigrationAnalyzeProgress{
					Done:  n,
					Total: total,
				})
			}
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// ── "Removed" items: remote objects absent from the local set ─────────────
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

	return results, nil
}

// ─── CreateSnapshot ──────────────────────────────────────────────────────────

// CreateSnapshot optionally creates a backup set and/or a zero-copy
// clone of the target database as a safety net before deployment.
func (s *Service) CreateSnapshot(client *snowflake.Client, database, backupSetDB, backupSetSchema, backupSetName string, doBackup bool, cloneDB string, doClone bool) error {
	ctx := context.Background()

	if doBackup && backupSetName != "" {
		fqn := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(backupSetDB), snowflake.QuoteIdent(backupSetSchema), snowflake.QuoteIdent(backupSetName))
		sql := fmt.Sprintf("CREATE OR REPLACE BACKUP SET %s FOR DATABASE %s", fqn, snowflake.QuoteIdent(database))
		if _, err := client.Execute(ctx, sql); err != nil {
			return fmt.Errorf("create backup set: %w", err)
		}
	}

	if doClone && cloneDB != "" {
		sql := fmt.Sprintf("CREATE OR REPLACE DATABASE %s CLONE %s", snowflake.QuoteIdent(cloneDB), snowflake.QuoteIdent(database))
		if _, err := client.Execute(ctx, sql); err != nil {
			return fmt.Errorf("create clone: %w", err)
		}
	}

	return nil
}

// ─── Execute ─────────────────────────────────────────────────────────────────

// Execute deploys the selected objects to Snowflake in dependency order
// with up to maxPasses retry passes for objects that fail due to dependency
// errors. It emits migration:exec:progress events throughout.
func (s *Service) Execute(client *snowflake.Client, selected []MigrationObject, database string, maxPasses int, strategy TableMigrationStrategy) ([]MigrationExecEvent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	defer func() {
		cancel()
		s.cancelFunc = nil
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
			s.emit(ExecProgressEvent, runEvt)

			// Set database context if known; non-fatal if it fails.
			if mo.Database != "" {
				_, _ = client.Execute(ctx, fmt.Sprintf("USE DATABASE %s", migrQuote(mo.Database)))
			}

			execErr := s.executeMigrationObject(ctx, client, mo, strategy)

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
				s.emit(ExecProgressEvent, evt)
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
				s.emit(ExecProgressEvent, evt)
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
				s.emit(ExecProgressEvent, evt)
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
		s.emit(ExecProgressEvent, evt)
	}

	return allEvents, nil
}

// ─── Cancel ──────────────────────────────────────────────────────────────────

// Cancel cancels any in-flight Analyze or Execute operation.
func (s *Service) Cancel() error {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	return nil
}

// ─── GenerateScript ──────────────────────────────────────────────────────────

// GenerateScript builds a human-readable SQL script for the supplied
// diff items using the chosen table migration strategy. It does not require an
// active Snowflake connection because all column information is derived from the
// DDL text already present in the diff items.
func GenerateScript(items []MigrationDiffItem, database string, strategy TableMigrationStrategy) string {
	return buildMigrationScript(items, database, strategy)
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
// any trailing semicolon so that cosmetic formatting differences don't register
// as spurious changes.
func normalizeDDL(sql string) string {
	s := blockCommentRE.ReplaceAllString(sql, " ")
	s = lineCommentRE.ReplaceAllString(s, " ")
	s = whitespaceRE.ReplaceAllString(s, " ")
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
func isDependencyError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "not authorized")
}

// migrTempName returns a temporary table name unlikely to collide with
// existing objects.
func migrTempName(base string) string {
	n := tempCounter.Add(1)
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
func parseLocalTableColumns(ddlText string) []colDef {
	// Locate the outermost parenthesised block.
	start := strings.IndexByte(ddlText, '(')
	if start < 0 {
		return nil
	}
	depth, end := 0, -1
	for i := start; i < len(ddlText); i++ {
		switch ddlText[i] {
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

	var cols []colDef
	for _, part := range splitTopLevel(ddlText[start+1:end], ',') {
		part = strings.TrimSpace(part)
		if part == "" || constraintPrefixRE.MatchString(part) {
			continue
		}
		locs := colNameRE.FindStringSubmatchIndex(part)
		if locs == nil {
			continue
		}
		name := strings.ToUpper(part[locs[2]:locs[3]]) // capture group [1]
		rest := strings.TrimSpace(part[locs[1]-1:])
		typeExpr := ""
		if tm := colTypeRE.FindString(rest); tm != "" {
			typeExpr = strings.ToUpper(strings.TrimSpace(tm))
		} else if len(strings.Fields(rest)) > 0 {
			typeExpr = strings.ToUpper(strings.Fields(rest)[0])
		}
		if typeExpr != "" {
			cols = append(cols, colDef{Name: name, TypeExpr: typeExpr})
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
func commonColumnNames(a, b []colDef) []string {
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
// it references db.schema.newName.
func replaceDDLTableName(ddlText, db, schema, newName string) string {
	newFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(newName)

	parenIdx := strings.IndexByte(ddlText, '(')
	if parenIdx < 0 {
		return ddlText
	}
	header := ddlText[:parenIdx]
	body := ddlText[parenIdx:]

	upper := strings.ToUpper(header)
	tablePos := strings.LastIndex(upper, "TABLE")
	if tablePos < 0 {
		return ddlText
	}

	i := tablePos + 5
	for i < len(header) && (header[i] == ' ' || header[i] == '\t') {
		i++
	}
	identStart := i

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

func tableExists(ctx context.Context, client *snowflake.Client, db, schema, name string) (bool, error) {
	sql := fmt.Sprintf(
		"SELECT COUNT(*) FROM %s.INFORMATION_SCHEMA.TABLES"+
			" WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'"+
			" AND TABLE_TYPE = 'BASE TABLE'",
		migrQuote(db),
		strings.ReplaceAll(strings.ToUpper(schema), "'", "''"),
		strings.ReplaceAll(strings.ToUpper(name), "'", "''"),
	)
	res, err := client.Execute(ctx, sql)
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

func describeTableColumns(ctx context.Context, client *snowflake.Client, db, schema, table string) ([]colDef, error) {
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(table)
	res, err := client.Execute(ctx, "DESCRIBE TABLE "+fqn)
	if err != nil {
		return nil, err
	}
	var cols []colDef
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
		cols = append(cols, colDef{
			Name:     strings.ToUpper(colName),
			TypeExpr: strings.ToUpper(typeExpr),
		})
	}
	return cols, nil
}

func tableRowCount(ctx context.Context, client *snowflake.Client, db, schema, name string) (int64, error) {
	escaped := strings.ReplaceAll(name, "'", "''")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	sql := fmt.Sprintf(
		"SHOW TABLES LIKE '%s' IN SCHEMA %s.%s",
		escaped, migrQuote(db), migrQuote(schema),
	)
	res, err := client.Execute(ctx, sql)
	if err != nil {
		return 0, err
	}
	if len(res.Rows) == 0 {
		return 0, nil
	}

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

func (s *Service) executeMigrationObject(ctx context.Context, client *snowflake.Client, mo MigrationObject, strategy TableMigrationStrategy) error {
	if strings.ToUpper(mo.ObjectKind) != "TABLE" {
		_, err := client.Execute(ctx, mo.DDL)
		return err
	}

	exists, err := tableExists(ctx, client, mo.Database, mo.Schema, mo.ObjectName)
	if err != nil {
		_, execErr := client.Execute(ctx, mo.DDL)
		return execErr
	}
	if !exists {
		_, err = client.Execute(ctx, mo.DDL)
		return err
	}

	rowCount, rcErr := tableRowCount(ctx, client, mo.Database, mo.Schema, mo.ObjectName)
	if rcErr == nil && rowCount == 0 {
		return executeDestructiveRebuild(ctx, client, mo)
	}

	switch strategy {
	case StrategyBlueGreenSwap:
		return executeBlueGreenSwap(ctx, client, mo)
	case StrategyViewAbstraction:
		return executeViewAbstraction(ctx, client, mo)
	case StrategyDestructiveRebuild:
		return executeDestructiveRebuild(ctx, client, mo)
	default: // StrategyInPlace
		return executeInPlace(ctx, client, mo)
	}
}

// ─── strategy: in_place ───────────────────────────────────────────────────────

func executeInPlace(ctx context.Context, client *snowflake.Client, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)

	remoteCols, err := describeTableColumns(ctx, client, db, schema, name)
	if err != nil {
		return fmt.Errorf("describe remote columns: %w", err)
	}
	localCols := parseLocalTableColumns(mo.DDL)
	if len(localCols) == 0 {
		_, execErr := client.Execute(ctx, mo.DDL)
		return execErr
	}

	remoteByName := make(map[string]colDef, len(remoteCols))
	for _, c := range remoteCols {
		remoteByName[c.Name] = c
	}
	localByName := make(map[string]colDef, len(localCols))
	for _, c := range localCols {
		localByName[c.Name] = c
	}

	for _, c := range localCols {
		if _, exists := remoteByName[c.Name]; !exists {
			sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", fqn, migrQuote(c.Name), c.TypeExpr)
			if _, err = client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("add column %s: %w", c.Name, err)
			}
		}
	}

	for _, c := range remoteCols {
		if _, exists := localByName[c.Name]; !exists {
			sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", fqn, migrQuote(c.Name))
			if _, err = client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("drop column %s: %w", c.Name, err)
			}
		}
	}

	for _, lc := range localCols {
		rc, exists := remoteByName[lc.Name]
		if !exists {
			continue
		}
		if !strings.EqualFold(lc.TypeExpr, rc.TypeExpr) {
			sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", fqn, migrQuote(lc.Name), lc.TypeExpr)
			if _, err = client.Execute(ctx, sql); err != nil {
				return fmt.Errorf("alter column %s type: %w", lc.Name, err)
			}
		}
	}

	return nil
}

// ─── strategy: blue_green_swap ────────────────────────────────────────────────

func executeBlueGreenSwap(ctx context.Context, client *snowflake.Client, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	tmpName := migrTempName(name)
	origFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	tmpFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(tmpName)

	tmpDDL := replaceDDLTableName(mo.DDL, db, schema, tmpName)
	if _, err := client.Execute(ctx, tmpDDL); err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	dropTmp := func() { _, _ = client.Execute(ctx, "DROP TABLE IF EXISTS "+tmpFQN) }

	remoteCols, err := describeTableColumns(ctx, client, db, schema, name)
	if err != nil {
		dropTmp()
		return fmt.Errorf("describe remote columns: %w", err)
	}
	newCols, err := describeTableColumns(ctx, client, db, schema, tmpName)
	if err != nil {
		dropTmp()
		return fmt.Errorf("describe new columns: %w", err)
	}
	commonNames := commonColumnNames(remoteCols, newCols)

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
		if _, err = client.Execute(ctx, insertSQL); err != nil {
			dropTmp()
			return fmt.Errorf("copy data: %w", err)
		}
	}

	if _, err = client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s SWAP WITH %s", origFQN, tmpFQN)); err != nil {
		dropTmp()
		return fmt.Errorf("swap tables: %w", err)
	}

	dropTmp()
	return nil
}

// ─── strategy: view_abstraction ───────────────────────────────────────────────

func executeViewAbstraction(ctx context.Context, client *snowflake.Client, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	archiveName := name + "_v1"
	origFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	archiveFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(archiveName)
	compatFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name+"_compat")

	remoteCols, err := describeTableColumns(ctx, client, db, schema, name)
	if err != nil {
		return fmt.Errorf("describe remote columns: %w", err)
	}

	if _, err = client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", origFQN, archiveFQN)); err != nil {
		return fmt.Errorf("rename table: %w", err)
	}

	if _, err = client.Execute(ctx, mo.DDL); err != nil {
		_, _ = client.Execute(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", archiveFQN, origFQN))
		return fmt.Errorf("create new table: %w", err)
	}

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
		if _, err = client.Execute(ctx, viewSQL); err != nil {
			return fmt.Errorf("create compat view: %w", err)
		}
	}

	return nil
}

// ─── strategy: destructive_rebuild ────────────────────────────────────────────

func executeDestructiveRebuild(ctx context.Context, client *snowflake.Client, mo MigrationObject) error {
	db, schema, name := mo.Database, mo.Schema, mo.ObjectName
	fqn := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(name)
	if _, err := client.Execute(ctx, "DROP TABLE IF EXISTS "+fqn); err != nil {
		return fmt.Errorf("drop table: %w", err)
	}
	if _, err := client.Execute(ctx, mo.DDL); err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}

// ─── script generation ────────────────────────────────────────────────────────

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
		ddlText := strings.TrimRight(mo.DDL, ";\n ")

		if strings.ToUpper(mo.ObjectKind) != "TABLE" || item.Status == "new" {
			fmt.Fprintf(&sb, "%s;\n\n", ddlText)
			continue
		}

		switch strategy {
		case StrategyInPlace:
			scriptInPlace(&sb, fqn, mo.ObjectName, item.LocalDDL, item.RemoteDDL)
		case StrategyBlueGreenSwap:
			scriptBlueGreen(&sb, fqn, db, mo.Schema, mo.ObjectName, item.LocalDDL, item.RemoteDDL, ddlText)
		case StrategyViewAbstraction:
			scriptViewAbstraction(&sb, fqn, db, mo.Schema, mo.ObjectName, item.LocalDDL, item.RemoteDDL, ddlText)
		case StrategyDestructiveRebuild:
			fmt.Fprintf(&sb, "-- Destructive Rebuild: %s\n", mo.ObjectName)
			fmt.Fprintf(&sb, "DROP TABLE IF EXISTS %s;\n%s;\n\n", fqn, ddlText)
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

	remoteByName := make(map[string]colDef, len(remoteCols))
	for _, c := range remoteCols {
		remoteByName[c.Name] = c
	}
	localByName := make(map[string]colDef, len(localCols))
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

func scriptBlueGreen(sb *strings.Builder, origFQN, db, schema, objectName, localDDL, remoteDDL, ddlText string) {
	tmpName := objectName + "__migration_tmp"
	tmpFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(tmpName)
	tmpDDL := strings.TrimRight(replaceDDLTableName(ddlText, db, schema, tmpName), ";\n ")

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

func scriptViewAbstraction(sb *strings.Builder, origFQN, db, schema, objectName, localDDL, remoteDDL, ddlText string) {
	archiveFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(objectName+"_v1")
	compatFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(objectName+"_compat")

	remoteCols := parseLocalTableColumns(remoteDDL)
	localCols := parseLocalTableColumns(localDDL)
	commonNames := commonColumnNames(remoteCols, localCols)

	fmt.Fprintf(sb, "-- View Abstraction: %s\n", objectName)
	fmt.Fprintf(sb, "ALTER TABLE %s RENAME TO %s;\n%s;\n", origFQN, archiveFQN, ddlText)
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
