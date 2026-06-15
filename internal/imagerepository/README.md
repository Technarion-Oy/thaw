# internal/imagerepository

> SQL builder for Snowflake IMAGE REPOSITORY objects.

## Responsibility

Builds the `CREATE IMAGE REPOSITORY` DDL from a structured config. An image
repository is an OCI-compliant registry that stores container images for
Snowpark Container Services (services and jobs). Each repository exposes a
`repository_url`, and images pushed to it are enumerated with `SHOW IMAGES IN
IMAGE REPOSITORY`.

Image repositories have a minimal grammar: beyond the name they accept only
`OR REPLACE` / `IF NOT EXISTS` and an optional `COMMENT` on creation. They
cannot be renamed, and `COMMENT` is the only mutable property, so the edit
clauses (`SET`/`UNSET COMMENT`) are issued as free-form `ALTER IMAGE REPOSITORY
<fqn> <clause>` statements directly from `internal/app/imagerepository.go`
(`App.AlterImageRepository`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ImageRepositoryConfig`, `BuildCreateImageRepositorySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ImageRepositoryConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Comment` |
| `BuildCreateImageRepositorySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] IMAGE REPOSITORY [IF NOT EXISTS] <fqn> [COMMENT='…'];` |

## Patterns & integration

- A blank name emits the placeholder `image_repository_name` so the live SQL
  preview reads as a completable template while the user is still typing.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateImageRepositorySql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterImageRepository` (in
  `internal/app/imagerepository.go`) runs the edit clauses, and
  `App.ListImagesInRepository` runs `SHOW IMAGES IN IMAGE REPOSITORY` for the
  properties panel's image listing.
- Discovery: `Client.ListExtendedObjects` runs `SHOW IMAGE REPOSITORIES IN
  SCHEMA` with the fixed kind `"IMAGE REPOSITORY"`. Image repositories are not
  surfaced by `SHOW OBJECTS`, so — like masking policies, network rules, tags,
  and alerts — no dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW IMAGE REPOSITORIES LIKE …` for
  the `IMAGE REPOSITORY` kind; the `repository_url` column is highlighted in the
  modal.

## Gotchas

- **No `RENAME`** — `ALTER IMAGE REPOSITORY` has no `RENAME TO` clause, so image
  repositories are excluded from the sidebar's Rename action (like alerts,
  external tables, and network rules). To rename, recreate the repository.
- **`GET_DDL` is not supported** for image repositories, so there is no DDL
  export / "View Definition" path and no `buildGetDDLQuery` mapping for this
  kind. The properties panel relies entirely on `SHOW IMAGE REPOSITORIES`.
- **`COMMENT` is the only editable property** — there is no `ALTER` for the
  repository URL (it is server-assigned) or for encryption after creation.
