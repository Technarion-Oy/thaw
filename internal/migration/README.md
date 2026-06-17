# internal/migration

> Schema migration engine: scans local SQL files, diffs against a live Snowflake database, deploys changes with multi-pass retry, and generates human-readable migration scripts.

## Responsibility

- Scan a directory tree of `.sql` files, parse every `CREATE` statement, track `USE DATABASE` / `USE SCHEMA` context, and produce a flat list of `MigrationObject` records (`ScanSource`).
- Compare local objects against the live Snowflake database by fetching database-level DDL via `GET_DDL('database', X, true)` in parallel, then classifying each object as `new`, `changed`, `unchanged`, or `removed` (`Analyze`).
- Execute a selected subset of migration objects against Snowflake in dependency order, with up to N retry passes for objects that fail due to unresolved dependencies (`Execute`).
- Support four table migration strategies for existing tables: `in_place`, `blue_green_swap`, `view_abstraction`, `destructive_rebuild`.
- Generate a human-readable SQL script from a diff result without a live connection (`GenerateScript`).
- Optionally create a pre-deployment safety net (backup set and/or zero-copy clone of the target database) via `CreateSnapshot`.
- Emit real-time progress events (`migration:analyze:progress`, `migration:exec:progress`) so the frontend can render a progress indicator.
- Support cancellation of in-flight `Analyze` or `Execute` calls via `Cancel`.

## Key files

| File | Purpose |
|------|---------|
| `migration.go` | All domain logic: `Service`, `ScanSource`, `Analyze`, `Execute`, `GenerateScript`, `CreateSnapshot`, all strategy implementations, all helpers |
| `migration_test.go` | Unit tests for `ScanSource`, `normalizeDDL`, strategy helpers |
| `doc.go` | Package doc and `// thaw:domain: Schema Migration` annotation |

## Key types & functions

```go
// migration.go:39
type MigrationObject struct {
    FilePath, Database, Schema, ObjectKind, ObjectName, ArgSig, DDL string
    IsReplace bool
}

// migration.go:51
type MigrationDiffItem struct {
    Object MigrationObject
    Status    string // "new"|"changed"|"unchanged"|"removed"
    LocalDDL  string
    RemoteDDL string
}

// migration.go:80
type TableMigrationStrategy string
const (
    StrategyInPlace           TableMigrationStrategy = "in_place"
    StrategyBlueGreenSwap     TableMigrationStrategy = "blue_green_swap"
    StrategyViewAbstraction   TableMigrationStrategy = "view_abstraction"
    StrategyDestructiveRebuild TableMigrationStrategy = "destructive_rebuild"
)

// migration.go:133
type Service struct { ... }
func NewService(emit func(eventName string, data interface{})) *Service

func (s *Service) ScanSource(dir string) ([]MigrationObject, error)
func (s *Service) Analyze(client *snowflake.Client, objects []MigrationObject, database string) ([]MigrationDiffItem, error)
func (s *Service) Execute(client *snowflake.Client, selected []MigrationObject, database string, maxPasses int, strategy TableMigrationStrategy) ([]MigrationExecEvent, error)
func (s *Service) CreateSnapshot(client *snowflake.Client, ...) error
func (s *Service) Cancel() error
func GenerateScript(items []MigrationDiffItem, database string, strategy TableMigrationStrategy) string
```

## Patterns & integration

- `ScanSource`, `Analyze`, and `GenerateScript` are also exposed via MCP tools (`scan_migration_source`, `analyze_migration`, `generate_migration_script`) in `internal/mcp/migration_tools.go`, using a no-op emit callback since MCP does not stream progress events.
- `Service` is instantiated in `internal/app/run.go` and registered as a Wails bound service; its methods are accessible from `wailsjs/go/migration/Service`.
- The `emit` callback passed to `NewService` is `wailsruntime.EventsEmit`; the frontend listens for `migration:analyze:progress` and `migration:exec:progress`.
- `Analyze` uses `GET_DDL('database', X, true)` (database-level dump) instead of per-object calls because: (a) it is more efficient; (b) per-object `GET_DDL` for streams wraps `ON TABLE` references in a single double-quoted identifier that does not match the local DDL.
- DDL comparison is done after `normalizeDDL`: strips block/line comments, collapses whitespace, uppercases, and strips trailing semicolons to avoid false positives from cosmetic formatting differences. Comment stripping, `USE DATABASE`/`USE SCHEMA` context tracking, and `CREATE OR REPLACE` detection all run over the `internal/sqltok` token stream (not regexes), so nested block comments and comment-like sequences inside string literals are handled correctly.
- `Execute` sorts objects by `executionPriority` (DATABASE → SCHEMA → SEQUENCE → TABLE → … → TASK → PIPE) before deployment. Objects that fail with "does not exist" or "not authorized" are deferred to the next pass (up to `maxPasses`, default 5).
- The `in_place` strategy uses `DESCRIBE TABLE` for the remote column list and parses the local DDL with `parseLocalTableColumns`; it issues individual `ALTER TABLE ADD/DROP/ALTER COLUMN TYPE` statements.
- The `blue_green_swap` strategy rewrites the local DDL table name via `replaceDDLTableName`, creates the temp table, copies shared columns, swaps with `ALTER TABLE … SWAP WITH`, then drops the temp.
- Empty tables (row count 0) always use destructive rebuild regardless of the chosen strategy.
- `ScanSource` uses `internal/sqlutil.Split` and `internal/ddl.Parse` for statement splitting and object kind detection.

## Gotchas

- `Analyze` and `Execute` each hold a `context.CancelFunc` in `s.cancelFunc`. Only one operation may be in-flight at a time per `Service` instance — starting a second call overwrites the cancel func of any previous one.
- The `tempCounter` for blue-green temp names is an `atomic.Int64` at package level, so temp names are unique within a process but not across restarts. Table names are capped at 50 characters before the suffix to stay within Snowflake's 255-char limit.
- `tableExists` queries `INFORMATION_SCHEMA.TABLES` (not `SHOW TABLES`) to avoid triggering gosnowflake driver error logs when the table is absent.
- `RemoveDDLTableName` (`replaceDDLTableName`) only rewrites the identifier immediately after the last `TABLE` keyword in the DDL header; it does not handle DDL with `EXTERNAL TABLE`, `TRANSIENT TABLE`, etc. correctly if those keywords appear in comments before `CREATE TABLE`.
