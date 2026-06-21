# internal/externalagent

Builds SQL for Snowflake **EXTERNAL AGENT** objects.

An external agent registers a third-party / generative-AI application in
Snowflake (for use with AI Observability). Unlike a native `AGENT` it has **no
inline specification** — it is version-based, where each version represents a
different implementation (alternative retriever, prompt, LLM, or inference
configuration). External agents share their namespace with model objects.

## Types & functions

- `ExternalAgentConfig` — name, case flag, `OR REPLACE` / `IF NOT EXISTS`,
  `VersionName` (optional `WITH VERSION`), `Comment`.
- `BuildCreateExternalAgentSql(db, schema, cfg)` — renders:

  ```sql
  CREATE [OR REPLACE] EXTERNAL AGENT [IF NOT EXISTS] <fqn>
    [WITH VERSION <version_name>]
    [COMMENT = '…'];
  ```

  Version names are emitted unquoted so they fold to uppercase (V1, V2, …).

## ALTER / lifecycle

There is no `ALTER EXTERNAL AGENT … RENAME`, `UNSET`, or `TAG`. Mutations go
through the free-form `App.AlterExternalAgent(db, schema, name, clause)` in
`internal/app/externalagent.go`:

- `SET COMMENT = '…'`
- `ADD VERSION <version_name>` — adds a new implementation version.

`SHOW EXTERNAL AGENTS` already reports `versions` (a JSON array) and
`default_version_name`, so no DESCRIBE enrichment is needed. `GET_DDL` does not
support external agents (handled by an exclusion in `internal/snowflake`).

See also: `internal/agent` (the specification-based native Agent type).
