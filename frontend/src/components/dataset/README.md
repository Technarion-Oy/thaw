# components/dataset

> Modals for creating and managing Snowflake DATASET (ML data snapshot) objects.

## Components

| File | Purpose |
|---|---|
| `CreateDatasetModal.tsx` | Create form with a live `CREATE DATASET` SQL preview. Fields: name and OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other). CREATE DATASET makes an *empty* dataset; an info banner points users at the Properties dialog (Add version…) or the Snowpark ML Python API to load data. |
| `DatasetPropertiesModal.tsx` | `SHOW DATASETS` metadata as generic property rows, plus a lazily-loaded **Versions** table (`SHOW VERSIONS IN DATASET`) with a per-row Actions menu (`DROP VERSION`) and an **Add version…** dialog (`ADD VERSION 'v1' FROM ( <query> ) [PARTITION BY …] [COMMENT …] [METADATA …]`). This surfaces the entire `ALTER DATASET` surface — version management is all there is. |

## Integration

- Both delegate to IPC: `BuildCreateDatasetSql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterDataset` / `ListDatasetVersions` (properties +
  version listing + edits).
- `AlterDataset(db, schema, name, clause)` runs free-form `ALTER DATASET … <clause>`
  and is the single entry point for both mutations: `ADD VERSION` and
  `DROP VERSION`. Dataset version names are single-quoted **string literals**
  (e.g. `'v1.0'`), unlike model version names which are identifiers.
- `ALTER DATASET` has no `RENAME`, `SET COMMENT`, or `SET TAG`, and `GET_DDL` does
  not support datasets, so there is no rename, comment-edit, tag, or DDL-export
  path for this type.
