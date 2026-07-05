# frontend/src/components/task

> Full task management UI: creation, graph visualization, properties editing, execution history, and dependency management for Snowflake Tasks.

## Responsibility

Provides all interactive surfaces for Snowflake Task objects: creating tasks with full
scheduling and dependency config, visualising task DAGs with live status polling, editing
existing task properties, browsing run history, copying/removing tasks, and managing
child task links. Includes reusable sub-components `ScheduleEditor` and `WhenConditionBuilder`.

## Files

| File | Purpose |
|------|---------|
| `CreateTaskModal.tsx` | `CREATE TASK` form with compute type (warehouse/serverless), schedule, overlap policy, timeout, retry, notification integrations, WHEN condition, finalize task, AFTER dependencies, and SQL body. SQL preview is built inline by `buildSql()` (frontend-side). |
| `TaskGraphModal.tsx` | Interactive ReactFlow + dagre DAG of task dependencies. Polls task statuses every 3s. Context-menu actions: Properties, Execute, Suspend/Resume, Drop, Copy, Add child, Remove links, Export DDL. Properties calls the `onViewProperties` prop, which `Sidebar.tsx` wires to open `TaskPropertiesModal` for the right-clicked node. Uses a 60s module-level DDL cache. |
| `TaskPropertiesModal.tsx` | Properties/settings panel for an existing task. Inline editing of schedule, WHEN condition, notification integrations. Embeds `TaskHistoryModal`, `WhenConditionBuilder`, `ScheduleEditor`. |
| `ScheduleEditor.tsx` | Reusable schedule editor sub-component. Three modes: none, interval (N minutes/hours/days with Snowflake constraints), cron (5-field expr + timezone). Parses and emits schedule strings. |
| `WhenConditionBuilder.tsx` | Visual WHEN condition builder. Supports stream-has-data checks, predecessor task return-value predicates, and free-text expressions. Joined with AND/OR. |
| `TaskHistoryModal.tsx` | Paginated run history table using `GetTaskHistory` (TASK_HISTORY table function). Shows status, query ID, start/end times, error messages. |
| `TaskStatusesModal.tsx` | Bulk status view for all tasks in a schema with live refresh. |
| `CopyTaskModal.tsx` | Copies an existing task to a new name/schema, optionally with predecessor links. Calls `CopyTask`. |
| `AddExistingChildModal.tsx` | Adds an existing task as a child (AFTER dependency) of a root task. Calls `AddTaskChild`. |
| `RemoveChildLinksModal.tsx` | Removes selected child task links from a root task. Calls `RemoveTaskChildren`. |
| `ExecuteTaskModal.tsx` | Confirmation dialog for `EXECUTE TASK` (triggers a manual run). |

## Patterns & integration

**IPC calls (selected):**
- `ExecDDL(sql)` — executes CREATE / ALTER TASK DDL on submit
- `ListWarehouses()` / `ListNotificationIntegrations()` / `ListObjects(db, schema, "TASK")` — populate selects in CreateTaskModal
- `GetTaskStatuses(db, schema)` — polled every 3s in TaskGraphModal for live node colours
- `GetTaskHistory(db, schema, name, limit)` — task run history
- `AlterTask(db, schema, name, prop, value)` — inline property edits in TaskPropertiesModal
- `SuspendTaskList(tasks)` / `ResumeTaskList(tasks)` — bulk suspend/resume
- `ExportTaskDDL(db, schema, name)` — DDL export from graph context menu
- `CopyTask` / `AddTaskChild` / `RemoveTaskChildren` — dependency management

**SQL building in `CreateTaskModal`:** Unlike the dbtproject/pipe modals, `CreateTaskModal` builds its DDL preview with a local `buildSql()` function (pure TypeScript, no IPC round-trip). This avoids latency for the frequently-changing form.

**Graph layout:** `TaskGraphModal` uses `dagre` for automatic left-to-right DAG layout. Node colours reflect live task status (running, suspended, failed, etc.). The DDL cache (`Map<key, {ddl, ts}>`) is module-level with a 60s TTL, matching the pattern in `SqlEditor.tsx` and `DependenciesModal.tsx`.

**Sub-component integration:** `ScheduleEditor` and `WhenConditionBuilder` are controlled components that emit schedule strings and WHEN condition strings respectively. They are embedded in both `CreateTaskModal` and `TaskPropertiesModal`.

**Stores used:** `queryStore` is not directly used; task execution opens DDL in the SQL editor via `ExecDDL` rather than the query tab system.

## Gotchas

- `TaskGraphModal` polls every `POLL_MS = 3_000` ms. The interval is cleared on unmount. Long-running graphs with many tasks can generate frequent `GetTaskStatuses` calls.
- `CreateTaskModal` builds SQL locally — the backend is not involved in SQL generation for task creation. This means the preview does not benefit from backend identifier quoting logic; `identToken` and `esc` helpers handle quoting inline.
- `ScheduleEditor` enforces Snowflake's interval constraints (minimum 1 minute, etc.) via client-side validation; the backend does not validate schedule strings before `ExecDDL`.
