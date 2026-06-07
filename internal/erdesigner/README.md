# internal/erdesigner

> Join query builder for the ER diagram: BFS pathfinding and SQL generation.

## Responsibility

Provides the computational backend for the visual join builder in the ER diagram modal. All functions are pure computation (no database access required) and are exposed to the frontend via Wails IPC through `internal/app/erdesigner.go`.

## Files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain: ER Designer` annotation |
| `join_pathfinder.go` | Types (`TableRef`, `JoinPath`, `JoinQueryState`, etc.), BFS pathfinder (`FindJoinPaths`), and state builder (`BuildJoinState`) |
| `join_pathfinder_test.go` | Table-driven tests for pathfinder and state builder |
| `join_sql.go` | SQL generator (`BuildJoinSQL`) with proper identifier quoting via `snowflake.QuoteIdent` |
| `join_sql_test.go` | Table-driven tests for SQL generator |

## Key functions

- **`FindJoinPaths(selectedTables, fks)`** — BFS pathfinding on FK adjacency graph. For 2 tables: returns all shortest paths (up to 10) for disambiguation. For 3+ tables: Steiner tree approximation returning a single path.
- **`BuildJoinState(path, selectedTables, database)`** — Converts a `JoinPath` into a `JoinQueryState` with ON conditions, FK pairs, and intermediate table marking.
- **`BuildJoinSQL(state)`** — Generates formatted `SELECT ... JOIN ...` SQL with fully-qualified quoted identifiers (`"DB"."SCHEMA"."TABLE"`), table aliases (`t1`, `t2`, ...), and `LIMIT 1000`.

## Dependencies

- `internal/snowflake` — `TableKey()` for canonical table keys, `QuoteIdent()` for proper SQL identifier quoting.

## Patterns

- All types use JSON struct tags for Wails IPC serialization.
- Uses `snowflake.TableKey()` for all table key operations (canonical `SCHEMA.TABLE` uppercase format).
- Generated SQL uses `snowflake.QuoteIdent()` for all identifiers, ensuring reserved keywords and special characters are properly handled.
- IPC bindings in `internal/app/erdesigner.go` are thin delegators (no client needed).
