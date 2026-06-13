# components/alert

> Modals for creating and managing Snowflake ALERT objects.

## Components

| File | Purpose |
|---|---|
| `CreateAlertModal.tsx` | Create form with a live `CREATE ALERT` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS, **Schedule** (interval or `USING CRON …`), **Warehouse** (a `ListWarehouses` picker; empty ⇒ serverless), comment, an **Advanced** section for alert-level **Tags**, a Monaco **Condition** editor, and a Monaco **Action** editor (the `THEN` statement). The condition editor has two insert helpers sharing one database→schema selection: **Insert SELECT** (a table/view column picker) and **Insert CALL…** (a procedure picker that opens the shared `procedure/CallProcedureModal` in insert mode to drop a fully-built `CALL proc(args)` statement in — Snowflake alert conditions accept a SELECT, SHOW, or CALL). |
| `AlertPropertiesModal.tsx` | `SHOW ALERTS` metadata with a Started/Suspended state tag, **Suspend** / **Resume** / **Execute now** actions, an inline-editable **Comment**, and the rendered **Condition** and **Action**. |

## Integration

- Both delegate to IPC: `BuildCreateAlertSql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterAlert` (properties + lifecycle).
- `AlterAlert(db, schema, name, clause)` runs free-form `ALTER ALERT … <clause>`
  for `RESUME`, `SUSPEND`, `EXECUTE`, `SET`/`UNSET COMMENT`, etc.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Alerts** group (kind `"ALERT"`). Alerts are not queryable tables, so there is
  no **Select Top 1000 Rows**; `ALTER ALERT` has no `RENAME`, so **Rename** is
  not offered.
- The `AlertConfig` form shape mirrors the Wails-generated `alert.AlertConfig`;
  the nested `tags` array means a plain object literal is cast `as any` only at
  the IPC boundary.
