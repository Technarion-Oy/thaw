# internal/mcpserver

Builds SQL for Snowflake **MCP SERVER** objects.

An MCP server (Model Context Protocol) is a schema-level object that exposes
Snowflake tools and resources — Cortex Search services, Cortex Analyst semantic
views, SQL execution, Cortex agents, and generic UDFs / stored procedures — to
MCP clients through a single YAML specification.

## Types & functions

- `MCPServerConfig` — name, case flag, `OR REPLACE` / `IF NOT EXISTS`, and the
  required `Specification` (the tools YAML).
- `BuildCreateMCPServerSql(db, schema, cfg)` — renders:

  ```sql
  CREATE [OR REPLACE] MCP SERVER [IF NOT EXISTS] <fqn>
    FROM SPECIFICATION
    $THAW$
    <spec>
    $THAW$;
  ```

  The specification is wrapped in a tagged `$THAW$ … $THAW$` dollar-quote so
  multi-line YAML needs no escaping. There is **no `COMMENT`** clause in CREATE
  MCP SERVER.

## ALTER / lifecycle

Snowflake has **no `ALTER MCP SERVER`** statement — a server is changed by
re-issuing `CREATE OR REPLACE MCP SERVER … FROM SPECIFICATION` with a new spec,
and renamed via `CREATE OR REPLACE` as well (there is no `RENAME`). The
properties panel is therefore read-only: it shows the `server_spec` from
`DESCRIBE MCP SERVER` (via `App.DescribeMCPServer` in
`internal/app/mcpserver.go`) plus the SHOW metadata.

`SHOW MCP SERVERS` reports only `created_on` / `name` / `database_name` /
`schema_name` / `owner` / `comment`, so the full specification is read from
`DESCRIBE MCP SERVER` (`server_spec` column). `GET_DDL` does not support MCP
servers (handled by an exclusion in `internal/snowflake`).

See also: `internal/agent` (the specification-based native Agent type) and
`internal/mcp` (Thaw's *own* MCP server, unrelated to this Snowflake object).
