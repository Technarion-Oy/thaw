# internal/model

> SQL builder for Snowflake MODEL (ML Model Registry) objects.

## Responsibility

Builds the `CREATE MODEL` DDL from a structured config. A model is a schema-level
object from the Snowpark ML Model Registry: it holds one or more versioned ML
artifacts and can be invoked as a function for inference. Models support
versioning — a set of versions, a default version used for direct method calls,
and optional per-version aliases.

Most models are registered through the Snowpark ML Python API. The SQL
`CREATE MODEL` statement built here covers the two SQL-creatable shapes:

- **Copy an existing model** — `FROM MODEL <src> [VERSION <v>]`.
- **Load from an internal stage** — `FROM @<stage>[/<path>]` (serialized model
  artifacts).

`CREATE MODEL` has no `COMMENT`/`TAG` clause; those (plus `DEFAULT_VERSION`,
per-version `ALIAS`, and `RENAME`) are applied afterwards via free-form
`ALTER MODEL <fqn> <clause>` statements from `internal/app/model.go`
(`App.AlterModel`), without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ModelConfig`, `BuildCreateModelSql`, `SourceModel`/`SourceStage` consts |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ModelConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `VersionName` (WITH VERSION), `SourceType`, `SourceModel`/`SourceVersion`, `StageLocation` |
| `BuildCreateModelSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] MODEL [IF NOT EXISTS] <fqn> [WITH VERSION <v>] FROM {MODEL <src> [VERSION <v>] \| @stage};` |

## Patterns & integration

- A blank name emits the placeholder `model_name`; a blank source model emits
  `source_model_name` and a blank stage path emits `@my_stage/model_path`, so the
  live SQL preview reads as a completable template while the user is still typing.
- `SourceType` selects the FROM variant (`SourceModel` = `"model"`,
  `SourceStage` = `"stage"`); any value other than `"stage"` is treated as a
  model copy.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateModelSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterModel` (in `internal/app/model.go`) runs the edit clauses,
  and `App.ListModelVersions` runs `SHOW VERSIONS IN MODEL` for the properties
  panel's version listing.
- Discovery: `Client.ListExtendedObjects` runs `SHOW MODELS IN SCHEMA` with the
  fixed kind `"MODEL"`. Models are not surfaced by `SHOW OBJECTS`, so — like
  masking policies, image repositories, tags, and alerts — no dedupe pass is
  needed.
- Properties panel: `internal/objects` runs `SHOW MODELS LIKE …` for the `MODEL`
  kind; the modal highlights `default_version_name` and lazily lists versions.

## Gotchas

- **`GET_DDL` is not supported** for models (the get_ddl object-type enumeration
  omits `MODEL`), so there is no DDL export / "View Definition" / comparison path
  and no `buildGetDDLQuery` mapping for this kind. `App.GetObjectDDL` rejects the
  `MODEL` kind up front, and the sidebar excludes models from the DDL-driven menu
  actions. The properties panel relies on `SHOW MODELS` + `SHOW VERSIONS IN
  MODEL`.
- **`RENAME` is supported** — `ALTER MODEL <fqn> RENAME TO <new>` works, so models
  are *not* added to the sidebar's Rename-exclusion.
- **Editable via ALTER** — `COMMENT`, `DEFAULT_VERSION`, per-version `ALIAS`, and
  tags are mutable through `App.AlterModel`; the model artifacts themselves are
  immutable (new versions are added via the Python API, not SQL).
