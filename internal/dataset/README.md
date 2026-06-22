# internal/dataset

> SQL builder for Snowflake DATASET objects.

## Responsibility

Builds the `CREATE DATASET` DDL from a structured config. A dataset is a
schema-level object from the Snowpark ML feature set: it holds versioned,
immutable snapshots of data used for ML training and evaluation. Most datasets
are produced through the Snowpark ML Python API; the SQL `CREATE DATASET`
statement built here creates an *empty* dataset — it only names the dataset and
carries the optional `OR REPLACE` / `IF NOT EXISTS` flags. There is no `COMMENT`
or other property on `CREATE`.

Data is loaded one version at a time. The entire `ALTER DATASET` surface is
version management, issued as free-form `ALTER DATASET <fqn> <clause>` statements
from `internal/app/dataset.go` (`App.AlterDataset`):

- **Add a version** — `ADD VERSION '<name>' FROM ( <query> ) [PARTITION BY <expr>]
  [COMMENT = '<text>'] [METADATA = '<json>']`.
- **Drop a version** — `DROP VERSION '<name>'`.

`ALTER DATASET` has no `RENAME`, `SET COMMENT`, or `SET TAG` clause.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `DatasetConfig`, `BuildCreateDatasetSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `DatasetConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists` |
| `BuildCreateDatasetSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] DATASET [IF NOT EXISTS] <fqn>;` |

## Patterns & integration

- A blank name emits the placeholder `dataset_name`, so the live SQL preview
  reads as a completable template while the user is still typing.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateDatasetSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterDataset` (in `internal/app/dataset.go`) runs the ADD/DROP
  VERSION clauses, and `App.ListDatasetVersions` runs `SHOW VERSIONS IN DATASET`
  for the properties panel's version listing.
- Discovery: `Client.ListExtendedObjects` runs `SHOW DATASETS IN SCHEMA` with the
  fixed kind `"DATASET"`. Datasets are not surfaced by `SHOW OBJECTS`, so — like
  models, masking policies, and alerts — no dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW DATASETS LIKE …` for the
  `DATASET` kind; the modal lazily lists versions and exposes ADD/DROP VERSION.

## Gotchas

- **`GET_DDL` is not supported** for datasets (the get_ddl object-type enumeration
  omits `DATASET`), so there is no DDL export / "View Definition" / comparison
  path and no `buildGetDDLQuery` mapping for this kind. `App.GetObjectDDL` rejects
  the `DATASET` kind up front, and the sidebar excludes datasets from the
  DDL-driven menu actions. The properties panel relies on `SHOW DATASETS` +
  `SHOW VERSIONS IN DATASET`.
- **`RENAME` is not supported** — `ALTER DATASET` has no `RENAME TO`, so datasets
  *are* added to the sidebar's Rename-exclusion.
- **Version names are string literals** — `ADD VERSION 'v1'` / `DROP VERSION 'v1'`
  take single-quoted literals (e.g. `'v1.0'`), unlike model version names which
  are identifiers. The `FROM` clause wraps a query in parentheses.
