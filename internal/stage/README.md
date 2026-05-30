# internal/stage

> SQL builders and live-client helpers for Snowflake STAGE object management and file operations.

## Responsibility

Covers the full lifecycle of a Snowflake stage object:

- **DDL builders**: `CREATE STAGE` and `ALTER STAGE` SQL generation from typed
  config structs, including external-stage parameters (URL, storage integration,
  directory tables, encryption, file format).
- **File listing**: `LIST @STAGE` execution and result parsing into `StageFile`.
- **File transfer**: `PUT` (upload) and `GET` (download) command builders and
  executors.
- **File removal**: `REMOVE @STAGE` command builder and executor.

## Key files

| File | Purpose |
|------|---------|
| `stage.go` | All types and all functions — single implementation file |

(`doc.go` is absent; the `thaw:domain` annotation is at the top of `stage.go`.)

## Key types & functions

### `StageConfig`
Configuration for `CREATE STAGE`. Covers both internal and external stages:

| Field group | Fields |
|-------------|--------|
| Identity | `Name`, `Database`, `Schema`, `CaseSensitive`, `OrReplace`, `IfNotExists`, `Type` (`INTERNAL`/`EXTERNAL`) |
| External params | `Url`, `StorageIntegration`, `UsePrivatelinkEndpoint` |
| Encryption | `EncryptionType`, `KmsKeyId` |
| Directory table | `DirectoryEnabled`, `DirectoryAutoRefresh`, `DirectoryRefreshOnCreate`, `DirectoryNotificationIntegration` |
| File format | `FileFormatName` (named), `FileFormat fileformat.FileFormatConfig` (inline) |
| Misc | `Comment`, `Tags` |

### `BuildCreateStageSql(cfg StageConfig) string`
Returns a `CREATE [OR REPLACE] STAGE [IF NOT EXISTS] ...;` statement.
- External-only clauses (`URL`, `STORAGE_INTEGRATION`) are emitted only when
  `cfg.Type == "EXTERNAL"`.
- Directory table options differ between internal (`REFRESH_ON_CREATE`) and
  external (`AUTO_REFRESH`, `NOTIFICATION_INTEGRATION`).
- Inline file format is delegated to `fileformat.BuildInlineFileFormat`.

### `AlterStageConfig` / `BuildAlterStageSql(cfg AlterStageConfig) string`
Supports four `Action` values: `RENAME`, `REFRESH`, `SET`, `UNSET`.

### `StageFile`
```go
type StageFile struct {
    Name         string `json:"name"`
    Size         int64  `json:"size"`
    MD5          string `json:"md5"`
    LastModified string `json:"lastModified"`
}
```

### Live-client functions

| Function | SQL issued | Notes |
|----------|-----------|-------|
| `ListStageFiles(ctx, client, stageName, pattern)` | `LIST @stage [PATTERN='...']` | Prepends `@` if absent; returns `[]StageFile` |
| `UploadFileToStage(ctx, client, localPath, stageName, parallel, autoCompress, sourceCompression, overwrite)` | `PUT 'file://...' @stage ...` | Internal stages only |
| `DownloadFileFromStage(ctx, client, stageName, localDirPath, parallel, pattern)` | `GET @stage 'file://...' ...` | Internal stages only |
| `RemoveStageFiles(ctx, client, stageName, pattern)` | `REMOVE @stage [PATTERN='...']` | Optional regex pattern |

## Patterns & integration (thin-delegator)

`internal/app/stage.go` holds the `*App` IPC methods that are thin delegators to
this package. DDL builders are called, the SQL is executed via `client.Execute` or
`client.ExecDDL`, and the result is returned. The builder functions themselves
require no live connection.

The sidebar tree expansion for stage nodes (`stagedir:` / `stagefile:` key
prefixes) uses the `ListStageEntries` IPC method, which delegates to
`ListStageFiles` here.

## Gotchas

- `BuildAlterStageSql` has a latent bug in the `SET` action: the `first` flag is
  set to `false` only for `Comment` and `Url` — `StorageIntegration` and
  `DirectoryEnabled` do not update `first`, so a comma will always be prepended
  even for the first clause if those fields appear without `Comment`/`Url`. This
  does not affect current UI flows that set one property at a time.
- `UploadFileToStage` and `DownloadFileFromStage` escape single-quotes in
  `localPath` with a backslash (`\'`) rather than doubling, which is the correct
  form for the `file://` URI inside a Snowflake `PUT`/`GET` command — but this
  differs from standard SQL string literal escaping used elsewhere.
- `ListStageFiles` returns a `nil` slice (not empty) when no files are found
  because it uses `append` without pre-allocating. Callers should treat `nil` and
  empty as equivalent.
- Credentials (AWS key IDs, secret keys) are intentionally omitted from
  `StageConfig` for security reasons; the comment in the struct notes this
  explicitly.
