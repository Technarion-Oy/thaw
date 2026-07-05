# internal/fileformat

> SQL builders for Snowflake FILE FORMAT objects and data preview for both local files and staged files.

## Responsibility

Three concerns are kept together here because they share `FileFormatConfig` as the central data model:

1. **SQL builder** — generates `CREATE FILE FORMAT` DDL and inline `FILE_FORMAT => (...)` clauses.
2. **Stage file preview** — runs a `SELECT * FROM @stage/path (FILE_FORMAT => ...)` query against Snowflake and returns up to 50 rows.
3. **Local file preview** — parses the first 50 rows of a local CSV or JSON file client-side (no Snowflake connection), mimicking Snowflake's parsing rules.

## Key files

| File | Purpose |
|---|---|
| `fileformat.go` | `FileFormatConfig`, `PreviewResult`, `BuildCreateFileFormatSql`, `BuildCreateTemporaryFileFormatSql`, `BuildInlineFileFormat`, per-type parameter emitters |
| `stagepreview.go` | `PreviewStageFile(ctx, client, stagePath, cfg)` |
| `localpreview.go` | `PreviewLocalFile(path, cfg)` — CSV and JSON parsers |
| `fileformat_test.go` | Unit tests for the SQL builder |
| `localpreview_test.go` | Unit tests for the local CSV/JSON parsers |
| `doc.go` | Package doc + `thaw:domain: Schema Migration` annotation |

## Key types & functions

### Types

| Type | Purpose |
|---|---|
| `FileFormatConfig` | All file format parameters for CSV, JSON, AVRO, ORC, PARQUET, XML; zero-values match Snowflake defaults so the builder emits only non-default parameters |
| `PreviewResult` | `Columns []string`, `Rows []map[string]string`, `Error string` |

### SQL builders

| Function | Emits |
|---|---|
| `BuildCreateFileFormatSql(db, schema, cfg)` | `CREATE [OR REPLACE] FILE FORMAT [IF NOT EXISTS] <fqn> TYPE = <type> [...];` |
| `BuildCreateTemporaryFileFormatSql(name, cfg)` | `CREATE OR REPLACE TEMPORARY FILE_FORMAT <name> TYPE = <type> [...];` |
| `BuildInlineFileFormat(cfg)` | Inline `TYPE = CSV, FIELD_DELIMITER = ',', ...` string for use inside `FILE_FORMAT => (...)` |

### Preview functions

| Function | Description |
|---|---|
| `PreviewStageFile(ctx, client, stagePath, cfg)` | Guards `stagePath` via `snowflake.ValidateStageRef` (returns an error on an invalid/injection-shaped ref, since it's spliced unquoted into the query), then creates a temporary file format, runs `SELECT * FROM <stagePath> (FILE_FORMAT => '<name>') LIMIT 50`, drops the temp format, returns `PreviewResult` |
| `PreviewLocalFile(path, cfg)` | Reads up to 1 MB of a local file, parses CSV or JSON, returns `PreviewResult` without a Snowflake connection |

## Patterns & integration

`*App` delegators in `internal/app/builders.go` and `internal/app/stage.go`:
- `App.BuildCreateFileFormatSql(db, schema, cfg)` → `fileformat.BuildCreateFileFormatSql`
- `App.GetLocalFilePreview(path, cfg)` → `fileformat.PreviewLocalFile` (no nil-check needed — no connection required)
- `App.GetStageFilePreview(stagePath, cfg)` → `fileformat.PreviewStageFile`

The SQL builder emits only parameters that differ from Snowflake's defaults, keeping generated DDL concise. Internal helpers `boolParam`, `identParam`, `noneOrStrParam`, `dateTimeParam`, and `nullIfParam` handle the default-comparison and quoting logic per parameter class.

`identParam` applies a strict allowlist for unquoted keyword-style values (compression codecs, binary formats, encoding names) to prevent SQL injection via the config struct.

## Gotchas

`PreviewStageFile` uses a temporary FILE FORMAT object rather than an inline `FILE_FORMAT => (...)` clause because Snowflake rejects non-constant expressions in the inline form for some account configurations. A best-effort fallback to the inline form is attempted if creating the temp format fails (e.g. no active database set).

When `ParseHeader = true`, `PreviewStageFile` fetches one extra row (`LIMIT 51`) with `ParseHeader` disabled in the query, then uses the first returned row as column headers — Snowflake's `SELECT` ignores `PARSE_HEADER=TRUE` for column naming.

`PreviewLocalFile` reads at most 1 MB to prevent OOM on large files. Only CSV and JSON are supported locally; other format types return an error message in `PreviewResult.Error`.

The local CSV parser handles Snowflake's `FIELD_OPTIONALLY_ENCLOSED_BY`, `ESCAPE`, and `ESCAPE_UNENCLOSED_FIELD` rules manually, including doubled-quote escaping inside quoted fields.
