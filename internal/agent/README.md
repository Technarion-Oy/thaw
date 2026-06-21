# internal/agent

Builds SQL for Snowflake **AGENT** objects (Cortex AI agents).

An agent is a schema-level Cortex object that pairs an orchestration LLM with a
set of tools (Cortex Analyst, Cortex Search, custom SQL/procedures, …). Its
behaviour is described by a YAML/JSON **specification** (models, orchestration
budget, instructions, tools, tool_resources) supplied via `FROM SPECIFICATION
$$ … $$`, plus an optional `PROFILE` JSON object holding display metadata
(`display_name`, `avatar`, `color`).

## Types & functions

- `AgentConfig` — name, case flag, `OR REPLACE` / `IF NOT EXISTS`, `Comment`,
  `Profile` (JSON object string), `Specification` (YAML/JSON spec body).
- `BuildCreateAgentSql(db, schema, cfg)` — renders:

  ```sql
  CREATE [OR REPLACE] AGENT [IF NOT EXISTS] <fqn>
    [COMMENT = '…']
    [PROFILE = '<json>']
    FROM SPECIFICATION
    $$
    <spec>
    $$;
  ```

  The spec is emitted inside `$$ … $$` so multi-line YAML needs no escaping;
  blank required parts fall back to placeholders so the create-modal preview
  stays a completable template.

## ALTER / lifecycle

There is no `ALTER AGENT … RENAME`, `UNSET`, or `TAG`. Mutations go through the
free-form `App.AlterAgent(db, schema, name, clause)` in `internal/app/agent.go`:

- `SET COMMENT = '…'` / `SET PROFILE = '…'`
- `MODIFY LIVE VERSION SET SPECIFICATION = $$ … $$` — replaces the live spec
  wholesale (omitted fields are removed).

`SHOW AGENTS` omits the spec, so `DESCRIBE AGENT` (`App.DescribeAgent`) supplies
the `agent_spec` column read by the properties modal. `GET_DDL` supports agents
via the `CORTEX_AGENT` object type (handled in `internal/snowflake`).

See also: `internal/externalagent` (the version-based External Agent type).
