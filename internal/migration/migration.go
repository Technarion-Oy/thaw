// SPDX-License-Identifier: GPL-3.0-or-later

package migration

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"thaw/internal/ddl"
	"thaw/internal/snowflake"
	"thaw/internal/sqltok"
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

// ─── statement classification (token-based) ──────────────────────────────────
//
// Comment stripping, USE DATABASE/SCHEMA context tracking, and CREATE OR REPLACE
// detection are done over the sqltok token stream rather than regexes. This is
// more robust (nested block comments, "" / '' escapes, comment-like sequences
// inside string literals) and shares one tokenization per statement. Column
// parsing is likewise token-based — see parseLocalTableColumns / parseColumnSegment.

// useContext detects a "USE DATABASE <name>" or "USE SCHEMA <name>" statement,
// returning the keyword ("DATABASE"/"SCHEMA") and the upper-cased, unquoted
// target name. ok is false for any other statement.
func useContext(sig []sqltok.Token, stmt string) (kind, name string, ok bool) {
	if len(sig) < 3 || !strings.EqualFold(sig[0].Text(stmt), "USE") {
		return "", "", false
	}
	kw := strings.ToUpper(sig[1].Text(stmt))
	if (kw != "DATABASE" && kw != "SCHEMA") || !sig[2].Kind.IsIdentLike() {
		return "", "", false
	}
	return kw, identUpper(sig[2], stmt), true
}

// isCreateOrReplace reports whether the statement begins with CREATE OR REPLACE.
func isCreateOrReplace(sig []sqltok.Token, stmt string) bool {
	return len(sig) >= 3 &&
		strings.EqualFold(sig[0].Text(stmt), "CREATE") &&
		strings.EqualFold(sig[1].Text(stmt), "OR") &&
		strings.EqualFold(sig[2].Text(stmt), "REPLACE")
}

// identUpper returns the upper-cased text of an identifier token, stripping a
// surrounding pair of double quotes from a quoted identifier.
func identUpper(t sqltok.Token, src string) string {
	return strings.ToUpper(sqltok.StripQuotePair(t.Text(src)))
}

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

		stmts := sqltok.Split(string(raw))

		var ctxDB, ctxSch string

		for _, stmt := range stmts {
			sig := sqltok.SignificantTokens(stmt)

			// Track USE DATABASE / USE SCHEMA context.
			if kw, name, ok := useContext(sig, stmt); ok {
				if kw == "DATABASE" {
					ctxDB = name
					ctxSch = "" // switching DB resets schema context
				} else {
					ctxSch = name
				}
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
				IsReplace:  isCreateOrReplace(sig, stmt),
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
//
// Remote DDL is sourced exclusively from GET_DDL('database', X, true) rather
// than per-object GET_DDL calls.  The database-level dump is preferred because:
//   - It is the canonical form Snowflake produces for export/clone workflows.
//   - Per-object GET_DDL for streams wraps ON TABLE references as a single
//     double-quoted identifier ("DB.SCHEMA.TABLE") which is syntactically
//     incorrect and doesn't match the unqualified form in the database dump.
//   - One query per database is more efficient than N per-object queries.
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

			stmts := sqltok.Split(ddlText)

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
	// StripComments drops comments (handling nested block comments and leaving
	// string literals intact); Fields+Join collapses every whitespace run to a
	// single space and trims the ends.
	s := strings.Join(strings.Fields(sqltok.StripComments(sql)), " ")
	s = strings.ToUpper(s)
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
//
// It is token-based: the column-list parentheses, the comma split between
// columns, and each column's type-precision parentheses are all located on the
// sqltok token stream. Commas or parentheses appearing inside a string default
// or a comment (e.g. DEFAULT '(a,b)') therefore no longer corrupt the split, and
// a quoted column name containing spaces is captured whole.
func parseLocalTableColumns(ddlText string) []colDef {
	sig := migSigToks(sqltok.Tokenize(ddlText))

	// The column list is the first parenthesised group.
	open := -1
	for i := range sig {
		if sig[i].Kind == sqltok.LParen {
			open = i
			break
		}
	}
	if open < 0 {
		return nil
	}
	closeIdx := matchParen(sig, open)
	if closeIdx < 0 {
		return nil
	}

	var cols []colDef
	depth := 0
	segStart := open + 1
	addSegment := func(end int) {
		if c, ok := parseColumnSegment(sig[segStart:end], ddlText); ok {
			cols = append(cols, c)
		}
		segStart = end + 1
	}
	for i := open + 1; i < closeIdx; i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
		case sqltok.Comma:
			if depth == 0 {
				addSegment(i)
			}
		}
	}
	addSegment(closeIdx) // the final column, between the last comma and ")"
	return cols
}

// parseColumnSegment classifies one comma-separated entry of a CREATE TABLE
// column list. It returns ok=false for empty entries and for table-level
// constraint / clustering clauses (CONSTRAINT, PRIMARY KEY, UNIQUE, CHECK,
// FOREIGN KEY, CLUSTER BY). Otherwise it returns the column name (quotes
// stripped, upper-cased) and its type expression — the first type token plus any
// trailing "( … )" precision, e.g. NUMBER(38,0) — also upper-cased.
func parseColumnSegment(seg []sqltok.Token, sql string) (colDef, bool) {
	if len(seg) == 0 {
		return colDef{}, false
	}
	// Skip table-level constraint / clustering clauses.
	switch upperToken(seg[0], sql) {
	case "CONSTRAINT", "UNIQUE", "CHECK":
		return colDef{}, false
	case "PRIMARY", "FOREIGN":
		if len(seg) > 1 && upperToken(seg[1], sql) == "KEY" {
			return colDef{}, false
		}
	case "CLUSTER":
		if len(seg) > 1 && upperToken(seg[1], sql) == "BY" {
			return colDef{}, false
		}
	}
	// A column definition needs a name token and a type token.
	if len(seg) < 2 || !isWordToken(seg[0]) || !isWordToken(seg[1]) {
		return colDef{}, false
	}
	name := upperIdentText(seg[0], sql)
	// Type expression: the first type token plus an optional "( … )" precision.
	typeEnd := seg[1].End
	if len(seg) > 2 && seg[2].Kind == sqltok.LParen {
		if cl := matchParen(seg, 2); cl >= 0 {
			typeEnd = seg[cl].End
		}
	}
	typeExpr := strings.ToUpper(strings.TrimSpace(sql[seg[1].Start:typeEnd]))
	if typeExpr == "" {
		return colDef{}, false
	}
	return colDef{Name: name, TypeExpr: typeExpr}, true
}

// migSigToks returns the significant tokens (dropping whitespace, newlines,
// comments, and the EOF sentinel) so the column walk indexes structural tokens.
func migSigToks(toks []sqltok.Token) []sqltok.Token {
	out := make([]sqltok.Token, 0, len(toks)/2)
	for _, t := range toks {
		switch t.Kind {
		case sqltok.Whitespace, sqltok.Newline, sqltok.LineComment, sqltok.BlockComment, sqltok.EOF:
			// skip
		default:
			out = append(out, t)
		}
	}
	return out
}

// matchParen returns the index of the ")" that matches the "(" at sig[open],
// handling nesting, or -1 if there is none.
func matchParen(sig []sqltok.Token, open int) int {
	depth := 0
	for i := open; i < len(sig); i++ {
		switch sig[i].Kind {
		case sqltok.LParen:
			depth++
		case sqltok.RParen:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// isWordToken reports whether t can be a column name or type keyword.
func isWordToken(t sqltok.Token) bool {
	return t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier || t.Kind == sqltok.QuotedIdent
}

// upperToken returns the upper-cased text of a keyword/identifier token, or ""
// for any other kind (so quoted/punctuation tokens never match a keyword check).
func upperToken(t sqltok.Token, sql string) string {
	if t.Kind == sqltok.Keyword || t.Kind == sqltok.Identifier {
		return strings.ToUpper(t.Text(sql))
	}
	return ""
}

// upperIdentText returns a column name upper-cased, with the surrounding quotes
// stripped from a "quoted identifier".
func upperIdentText(t sqltok.Token, sql string) string {
	s := t.Text(sql)
	if t.Kind == sqltok.QuotedIdent && len(s) >= 2 {
		s = s[1 : len(s)-1]
	}
	return strings.ToUpper(s)
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
//
// The name is located over the token stream: the TABLE keyword token followed
// by the (optionally qualified) identifier path is spliced by byte offsets, so
// a quoted table name containing the word TABLE — or a comment mentioning it —
// cannot shift the replacement. The DDL is returned unchanged when no
// CREATE TABLE name can be found.
func replaceDDLTableName(ddlText, db, schema, newName string) string {
	newFQN := migrQuote(db) + "." + migrQuote(schema) + "." + migrQuote(newName)

	sig := sqltok.SignificantTokens(ddlText)
	for i := range sig {
		if !strings.EqualFold(sig[i].Text(ddlText), "TABLE") || sig[i].Kind != sqltok.Keyword {
			continue
		}
		start := skipIfNotExists(sig, ddlText, i+1)
		_, next := sqltok.ReadIdentParts(sig, ddlText, start, 3)
		if next == start {
			return ddlText // no identifier after TABLE
		}
		return ddlText[:sig[start].Start] + newFQN + ddlText[sig[next-1].End:]
	}
	return ddlText
}

// skipIfNotExists returns the index just past an IF NOT EXISTS clause starting
// at sig[i], or i when the clause is absent.
func skipIfNotExists(sig []sqltok.Token, src string, i int) int {
	if i+2 < len(sig) &&
		strings.EqualFold(sig[i].Text(src), "IF") &&
		strings.EqualFold(sig[i+1].Text(src), "NOT") &&
		strings.EqualFold(sig[i+2].Text(src), "EXISTS") {
		return i + 3
	}
	return i
}

// ─── Snowflake introspection helpers ─────────────────────────────────────────

// tableExists reports whether a BASE TABLE with the given name exists.
// Uses INFORMATION_SCHEMA to avoid triggering gosnowflake driver error logs
// when the table is absent.
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

// describeTableColumns runs DESCRIBE TABLE and returns the column definitions.
// Only rows with kind = "Column" are included.
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

// tableRowCount returns the row count for a table using SHOW TABLES.
// It returns 0 without error when the table is not found or the count cannot
// be determined, so callers should treat any error as "unknown" rather than
// "empty".
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

// executeMigrationObject runs the appropriate strategy for a single migration
// object. Non-TABLE objects are always executed via their raw DDL. TABLE objects
// use the chosen strategy only when the table already exists; brand-new tables
// are always created via their DDL directly.
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

// executeInPlace diffs local column definitions against the remote schema and
// applies ALTER TABLE ADD/DROP/ALTER COLUMN TYPE statements.
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

// executeBlueGreenSwap creates a temporary table with the new schema (by
// rewriting the table name in the local DDL), copies shared columns from the
// original, atomically swaps the two tables, then drops the temp.
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

// executeViewAbstraction renames the existing table to <name>_v1, creates the
// new table from local DDL, and creates a compatibility view <name>_compat that
// exposes shared columns from the archived data.
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

// executeDestructiveRebuild drops the existing table and recreates it from
// local DDL. All existing data is permanently lost.
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
