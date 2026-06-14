# components/tag

> Modals for creating and managing Snowflake TAG objects.

## Components

| File | Purpose |
|---|---|
| `CreateTagModal.tsx` | Create form with a live `CREATE TAG` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS, **Allowed values** (a free-form `Select mode="tags"` whitelist; empty ⇒ any string permitted), comment, and a collapsible **Propagation (tag lineage)** section — `PROPAGATE` mode + nested `ON_CONFLICT` (the `ALLOWED_VALUES_SEQUENCE` keyword or a fixed value). |
| `TagPropertiesModal.tsx` | `SHOW TAGS` metadata with an inline-editable **Comment**, an **Allowed values** editor (add / remove individual values, or clear all), and a **References** section that lists the objects and columns the tag is applied to. |

## Integration

- Both delegate to IPC: `BuildCreateTagSql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterTag` / `GetTagReferences` (properties + edits).
- `AlterTag(db, schema, name, clause)` runs free-form `ALTER TAG … <clause>` for
  `RENAME TO`, `SET`/`UNSET COMMENT`, and `ADD`/`DROP`/`UNSET ALLOWED_VALUES`.
- `GetTagReferences(db, schema, name)` queries
  `SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES` for the live applications of the tag;
  it is loaded on demand (the view is slow and requires governance privileges),
  and its tabular `QueryResult` is rendered in an Ant Design `Table`.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Tags** group (kind `"TAG"`). Tags are not queryable tables, so there is no
  **Select Top 1000 Rows**; `ALTER TAG` supports `RENAME TO`, so **Rename** is
  offered.
- The `TagConfig` form shape mirrors the Wails-generated `tag.TagConfig`; a plain
  object literal is cast `as any` only at the IPC boundary.
