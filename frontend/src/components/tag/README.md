# components/tag

> Modals for creating and managing Snowflake TAG objects.

## Components

| File | Purpose |
|---|---|
| `CreateTagModal.tsx` | Create form with a live `CREATE TAG` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS, **Allowed values** (a free-form `Select mode="tags"` whitelist; empty ⇒ any string permitted), comment, and a collapsible **Propagation (tag lineage)** section (`TagPropagationFields`). |
| `TagPropertiesModal.tsx` | `SHOW TAGS` metadata with an inline-editable **Comment**, an **Allowed values** editor (add / remove individual values, or clear all), a **Propagation** editor (`TagPropagationFields` + Apply — issues `SET PROPAGATE = … [ON_CONFLICT = …]`, `UNSET PROPAGATE`, `UNSET ON_CONFLICT`), and a **References** section that lists the objects and columns the tag is applied to. |
| `TagPropagationFields.tsx` | Shared, controlled `PROPAGATE` mode picker + nested `ON_CONFLICT` control (the `ALLOWED_VALUES_SEQUENCE` keyword or a fixed value). Used by both the create and properties modals; reports the resolved `{ propagate, onConflict }`. |
| `TagReferencesModal.tsx` | Per-object **Tag References** viewer opened from any database, schema, object, or column context menu in the object browser. Lists the tags applied to that object via the no-latency `INFORMATION_SCHEMA.TAG_REFERENCES` table function (with the inheritance `LEVEL`), and — for column-bearing kinds (table/view/materialized view/dynamic/external/iceberg/hybrid/event table) — the per-column tags via `TAG_REFERENCES_ALL_COLUMNS`. Procedures / functions pass their argument-type signature (`args`) so the overload resolves. A `column` prop switches it to single-column mode (opened from a **column** context menu), filtering `TAG_REFERENCES_ALL_COLUMNS` to that one column. Delegates to `GetObjectTagReferences` / `GetColumnTagReferences`. |
| `TagManagementModal.tsx` | Centralized, account-wide **Tag Management** view (issue #515). A **References** tab browses `TAG_REFERENCES` with tag / value / database / domain / free-text filters, inline tag-value editing, per-row remove, and an **Apply tag…** sub-form (pick a tag from the catalog + object domain, then cascading database → schema → object → column dropdowns populated from `ListDatabases` / `ListUserSchemas` (excludes `INFORMATION_SCHEMA`) / `ListObjects` / `GetTableColumns`, + value; account-level domains fall back to a typed name). For the `COLUMN` domain the parent picker is restricted to tables and views (the kinds Snowflake lets you tag columns on), and the chosen parent's kind is carried into `ObjectTagRef.parentKind` so a view column emits `ALTER VIEW … ALTER COLUMN`; for inline edits on the References tab — where the parent kind isn't known — the backend resolves it from `INFORMATION_SCHEMA`. When the chosen tag declares **allowed values** (parsed from the catalog's `allowed_values`), the value field — both in the Apply form and the inline reference-row editor — becomes a dropdown restricted to those values instead of a free-text input. A **Tag catalog** tab lists `SHOW TAGS IN ACCOUNT` with free-text search. |

## Integration

- Create / properties delegate to IPC: `BuildCreateTagSql` / `ExecDDL` (create)
  and `GetObjectProperties` / `AlterTag` / `GetTagReferences` (properties + edits).
- `AlterTag(db, schema, name, clause)` runs free-form `ALTER TAG … <clause>` for
  `RENAME TO`, `SET`/`UNSET COMMENT`, `ADD`/`DROP`/`UNSET ALLOWED_VALUES`, and the
  propagation clauses `SET PROPAGATE = …`, `UNSET PROPAGATE`, `UNSET ON_CONFLICT`.
- `GetTagReferences(db, schema, name)` queries
  `SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES` for the live applications of the tag;
  it is loaded on demand (the view is slow and requires governance privileges),
  and its tabular `QueryResult` is rendered in an Ant Design `Table`.
- `TagReferencesModal` is opened from the **Tag References…** item on every
  object's context menu in `components/layout/Sidebar.tsx` — including
  **database** and **schema** nodes (`kind` `DATABASE` / `SCHEMA`, fewer name
  parts) and **column** nodes (which pass the `column` prop for single-column
  mode); it delegates to
  `GetObjectTagReferences` (and `GetColumnTagReferences` for column-bearing
  kinds), which wrap the no-latency `INFORMATION_SCHEMA.TAG_REFERENCES` /
  `TAG_REFERENCES_ALL_COLUMNS` table functions. The object menu passes the
  node's `objArgs` so procedures / functions resolve their overload. This differs from
  `GetTagReferences` (account-wide, tag-centric, lagging `ACCOUNT_USAGE`): it is
  object-centric and immediate.
- `TagManagementModal` delegates to `ListAccountTags`, `GetAllTagReferences`,
  `SetObjectTag`, and `UnsetObjectTag`. Its visibility lives in
  `store/tagManagementStore.ts` (`open` / `openView` / `closeView`) so it can be
  opened from the **Tools → Tag Management…** menu (listened for in `QueryPage`,
  which renders the modal) and from the **Tags** group context menu in
  `components/layout/Sidebar.tsx` (**Manage Tags…**).
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Tags** group (kind `"TAG"`). Tags are not queryable tables, so there is no
  **Select Top 1000 Rows**; `ALTER TAG` supports `RENAME TO`, so **Rename** is
  offered.
- The `TagConfig` form shape mirrors the Wails-generated `tag.TagConfig`; a plain
  object literal is cast `as any` only at the IPC boundary. `ObjectTagRef` is
  built via the generated `tagModels.ObjectTagRef.createFrom(...)`.
