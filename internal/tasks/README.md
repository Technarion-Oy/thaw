# internal/tasks

> Snowflake task graph operations: status queries, predecessor management, graph-wide suspend/resume/drop, topological ordering, run history, and DDL export.

## Responsibility

- Fetch current task states and last-run results for every task in a schema (`GetStatuses`).
- Fetch per-task or per-graph execution history from `INFORMATION_SCHEMA.TASK_HISTORY()` (`GetTaskRunHistory`).
- Modify the DAG: add or remove predecessor links (`AddParents`, `RemoveParents`), clone a child task with replacement predecessors (`CloneChildTask`).
- Suspend or resume entire graphs via BFS traversal (`SuspendGraph`, `EnableDependents`).
- Drop a task tree depth-first, suspending each node before dropping it (`DropTree`).
- Compute a dependency-safe (Kahn's algorithm) topological order for suspend/resume sequences and DDL export (`GetTopologicalOrder`).
- Export a complete graph as a single DDL script with optional surrounding `ALTER TASK … SUSPEND/RESUME` bookends (`ExportGraphDDL`, `BuildGraphDDL`).
- List tasks eligible to serve as finalizers (`ListFinalizableTasks`), and check whether a task has child dependents (`HasChildren`).

## Key files

| File | Purpose |
|------|---------|
| `tasks.go` | All domain logic; no sub-files |
| `tasks_test.go` | Unit tests for `GetTopologicalOrder`, `parsePredecessorRefs`, `BuildGraphDDL` |
| `doc.go` | Package doc and `// thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

```go
// tasks.go:15
type FinalizabilityRow struct { Name, DisabledReason string }

// tasks.go:23
type StatusRow struct {
    Name, TaskState, Predecessors, LastRunState, LastRunTime, ErrorMsg, Finalize string
}

// tasks.go:33
type StatusesResult struct { Rows []StatusRow; HistoryError string }

// tasks.go:39
type TaskHistoryRow struct { Name, State, ReturnValue, ScheduledTime, StartTime, EndTime, ErrorCode, ErrorMessage, RunID, RootTaskID string }

// tasks.go:55
func GetTaskRunHistory(ctx, client, database, schema, taskName string, isRoot bool, days int) ([]TaskHistoryRow, error)

// tasks.go:1008
func GetStatuses(ctx, client, database, schema string) (StatusesResult, error)

// tasks.go:669
type TopologicalOrder struct {
    TopoOrder, FinalizerNames, SuspendOrder, ResumeOrder []string
}

// tasks.go:731
func GetTopologicalOrder(rows []StatusRow, rootName string) TopologicalOrder

// tasks.go:877
func BuildGraphDDL(order TopologicalOrder, ddlByName map[string]string, database, schema string, includeSuspendResume bool) ExportGraphDDLResult

// tasks.go:955
func ExportGraphDDL(ctx, client, database, schema, rootName string, includeSuspendResume bool) (ExportGraphDDLResult, error)
```

## Patterns & integration

- All functions take `(ctx context.Context, client *snowflake.Client, ...)` and return domain types, consistent with the thin-delegator pattern. The `*App` receiver methods in `internal/app/tasks.go` are one-liners that call into this package.
- `GetStatuses` issues two queries: `SHOW TASKS IN SCHEMA` for current states, then `INFORMATION_SCHEMA.TASK_HISTORY()` for the last 7 days. The history query may fail independently (e.g. insufficient privilege); in that case `StatusesResult.HistoryError` is set and `Rows` is still returned.
- `GetTopologicalOrder` is a pure function (no Snowflake connection); it takes the `[]StatusRow` already fetched by `GetStatuses`. Finalizer tasks are excluded from BFS traversal and appended to suspend/resume orders last.
- Predecessor strings from `SHOW TASKS` arrive in two formats — a valid JSON array `["DB.SCH.T"]` or a Snowflake-quoted form `["DB"."SCH"."T"]`. `parsePredecessorRefs` tries JSON first, then falls back to string splitting.
- `ExportGraphDDL` fetches individual task DDL in parallel (semaphore: 8 concurrent requests) via `client.GetObjectDDL`. Failed fetches are logged and the task appears in `FailedTasks`.
- `GetTaskRunHistory` for root tasks uses `ROOT_TASK_ID` (not name) to retrieve all child executions in a single call; it resolves the task ID via `SHOW TASKS LIKE … IN SCHEMA` first.
- `SuspendGraph` suspends the root first (to stop the scheduler), then suspends each descendant via BFS over `SHOW TASKS`.
- `CloneChildTask` performs a three-step sequence: SHOW → CREATE TASK … CLONE → REMOVE AFTER → ADD AFTER; rolls back the clone on REMOVE/ADD failure.

## Gotchas

- Tasks must be suspended before their `AFTER` list can be modified. `RemoveParents` and `AddParents` call `suspendIfRunning` automatically; the caller is responsible for calling `EnableDependents` afterwards if the task should be resumed.
- `SHOW TASKS LIKE '<name>'` is case-insensitive and may return multiple rows when names share a prefix. All functions that use LIKE do an exact case-insensitive match on the `name` column before using the result.
- The Snowflake `finalize` column on older Snowflake versions may contain `"null"` (the string, not a SQL NULL). `GetStatuses` treats the string `"null"` the same as empty and falls back to the `task_relations` VARIANT column.
- `bareIdent` (`tasks.go:187`) strips one surrounding double-quote pair and unescapes `""` → `"`. It is used throughout predecessor parsing to extract the simple task name from fully-qualified dotted identifiers.
