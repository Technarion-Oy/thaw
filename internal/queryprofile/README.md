# internal/queryprofile

> Snowflake query execution profile fetcher, EXPLAIN plan parser, and performance diagnostic analyser.

## Responsibility

Three related capabilities live here:

1. **Operator stats** — wraps `GET_QUERY_OPERATOR_STATS` and returns typed
   `OperatorStat` rows with JSON columns pre-parsed into Go values.
2. **Explain plan** — runs `EXPLAIN USING JSON`, parses the result into a typed
   `ExplainPlan` tree (`ExplainGlobalStats` + `[][]ExplainNode`).
3. **Diagnostics** — walks the plan tree and emits `ExplainMarker` annotations
   (full table scans, cartesian joins, row explosion) mapped to Monaco editor
   line/column positions for inline highlighting.

## Key files

| File | Purpose |
|------|---------|
| `queryprofile.go` | All types and all functions — the package has a single implementation file |

## Key types & functions

### Operator stats
| Symbol | Description |
|--------|-------------|
| `OperatorStat` | One row from `GET_QUERY_OPERATOR_STATS`; JSON columns stored as `any` so they serialise as objects over IPC |
| `GetOperatorStats(ctx, client, queryID)` | Validates query ID format, executes the table function, returns parsed rows |

### Explain plan
| Symbol | Description |
|--------|-------------|
| `ExplainPlan` | Top-level plan: `GlobalStats ExplainGlobalStats` + `Operations [][]ExplainNode` |
| `ExplainGlobalStats` | `PartitionsTotal`, `PartitionsScanned`, `BytesAssigned` |
| `ExplainNode` | Single plan node: `ID`, `Parent`, `Operation`, `Objects`, `PartitionsScanned`, `PartitionsTotal`, `JoinType`, `EstimatedRows` |
| `GetExplainPlan(ctx, client, sql)` | Runs `EXPLAIN USING JSON <sql>`, unmarshals the JSON string from row 0 |
| `GetExplainPlanOnConn(ctx, client, conn, sql)` | Same but on a pinned `*sql.Conn` |

### Diagnostics
| Symbol | Description |
|--------|-------------|
| `DiagCode` | Typed string constant (`CodeFullTableScan`, `CodeCartesianJoin`, `CodeRowExplosion`) |
| `ExplainMarker` | Monaco-compatible marker: line/column range, message, severity (2/4/8), optional `ExplainData` |
| `ExplainData` | Structured plan data for rich hover tooltips: operation, object name, byte/partition counts, join type, row estimate |
| `ExplainResult` | Combined `Plan *ExplainPlan` + `Diagnostics []ExplainMarker`; returned by `RunExplain` |
| `RunExplain(ctx, client, sql)` | EXPLAIN + diagnostics in one call (avoids calling EXPLAIN twice) |
| `RunExplainOnConn(ctx, client, conn, sql)` | Same on a pinned connection |
| `GetExplainDiagnostics(ctx, client, sql)` | Diagnostics-only variant (no plan tree returned) |
| `GetDiagMessage(code, args...)` | Formats a human-readable message from `diagMessageTemplates` |

### Diagnostic rules (in `analyzePlan`)
- **Full table scan** (`TABLESCAN`, `INMEMTABLESCAN`, or `*SCAN` suffix): fires when
  `PartitionsTotal >= 10` and at least 50% are scanned. Warning at 50%, Error at 90%.
- **Cartesian join**: fires on any node where `Operation` or `JoinType` contains
  `CARTESIAN` or `CROSS` (case-insensitive). Always severity Error.
- **Row explosion**: fires on non-Cartesian join nodes where `EstimatedRows > 10 000 000`.
  Warning; Error when `> 1 000 000 000`.

## Patterns & integration (thin-delegator)

`internal/app` (e.g. `internal/app/query.go`) calls `RunExplainOnConn` on the
pinned connection used for the active query, or calls `GetOperatorStats` with a
completed query ID. The results are passed directly to the frontend over IPC.

```go
// internal/app/query.go (illustrative)
func (a *App) RunExplain(sql string) (*queryprofile.ExplainResult, error) {
    if a.client == nil { return nil, apperrors.ErrNotConnected }
    return queryprofile.RunExplain(a.ctx, a.client, sql)
}
```

## Gotchas

- `GetOperatorStats` validates that the query ID contains only hex digits and
  hyphens before interpolating it into SQL, preventing injection. Do not bypass
  this validation.
- The three JSON object columns (`OperatorStatistics`, `ExecutionTimeBreakdown`,
  `OperatorAttributes`) are typed `any` so the Go JSON encoder emits them as JSON
  objects rather than escaped strings when serialised over the Wails IPC layer.
  Changing these to `string` would break the frontend rendering.
- `findTokenPos` scans the SQL text linearly for the first occurrence of a keyword.
  It returns the fallback range `(1, 1, 1, 9999)` when the token is not found,
  meaning the marker will span the first line. This is intentional for cases where
  the relevant keyword cannot be located precisely (e.g. implicit comma joins).
- `asInt64Slice` first tries `[]int64` JSON unmarshalling and falls back to
  `[]float64` because Snowflake's driver may return the parent-operator array as
  floats.
