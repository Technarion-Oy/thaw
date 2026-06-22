# internal/gateway

> SQL builder for Snowflake GATEWAY objects.

## Responsibility

Builds the `CREATE GATEWAY` and `ALTER GATEWAY` DDL from a structured config. A
gateway is a schema-level Snowpark Container Services (SPCS) object that fronts
service endpoints: it splits ingress HTTP traffic across up to five service
endpoints according to a YAML specification (`type: traffic_split` /
`split_type: custom` / `targets` with weights summing to 100) and exposes an
external ingress URL.

Both `CREATE` and `ALTER` carry **only** the specification — there is no
`COMMENT`, `TAG`, or any other clause:

```
CREATE [OR REPLACE] GATEWAY [IF NOT EXISTS] <fqn> FROM SPECIFICATION <spec>
ALTER  GATEWAY [IF EXISTS] <fqn>               FROM SPECIFICATION <spec>
```

The specification is emitted inside a tagged `$THAW$ … $THAW$` dollar-quote so
the multi-line YAML needs no escaping. Updating the specification is the entire
`ALTER GATEWAY` surface (re-route traffic); there is no `RENAME`, `SET COMMENT`,
or `SET TAG`.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `GatewayConfig`, `BuildCreateGatewaySql`, `BuildAlterGatewaySpecSql`, `wrapSpec` |
| `sql_test.go` | Unit tests for the SQL builders |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `GatewayConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Specification` |
| `BuildCreateGatewaySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] GATEWAY [IF NOT EXISTS] <fqn> FROM SPECIFICATION $THAW$ … $THAW$;` |
| `BuildAlterGatewaySpecSql(db, schema, name, spec)` | Emits `ALTER GATEWAY <fqn> FROM SPECIFICATION $THAW$ … $THAW$;` |

## Patterns & integration

- A blank name emits the placeholder `gateway_name` and a blank spec emits a
  minimal `traffic_split` template, so the live SQL preview reads as a
  completable template while the user is still typing.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive in Snowflake; the
  builder drops `IF NOT EXISTS` when `OrReplace` is also set (and the create
  modal prevents selecting both).
- `App.BuildCreateGatewaySql` (in `internal/app/builders.go`) is the thin IPC
  delegator for the create preview; `App.AlterGateway` (in
  `internal/app/gateway.go`) runs the `FROM SPECIFICATION` update, and
  `App.DescribeGateway` runs `DESCRIBE GATEWAY` for the properties panel (the
  live spec + ingress URL).
- Discovery: `Client.ListExtendedObjects` runs `SHOW GATEWAYS IN SCHEMA` with the
  fixed kind `"GATEWAY"`. Gateways are not surfaced by `SHOW OBJECTS`, so — like
  services, models, and MCP servers — no dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW GATEWAYS LIKE …` for the
  `GATEWAY` kind; the modal shows the SHOW metadata, the `DESCRIBE GATEWAY`
  ingress URL, and an **editable** specification (the one ALTER path).

## Gotchas

- **`GET_DDL` is not supported** for gateways (the get_ddl object-type enumeration
  omits `GATEWAY`), so there is no DDL export / "View Definition" / comparison
  path and no `buildGetDDLQuery` mapping for this kind. `App.GetObjectDDL` rejects
  the `GATEWAY` kind up front, and the sidebar excludes gateways from the
  DDL-driven menu actions. The live spec is read with `DESCRIBE GATEWAY`.
- **`RENAME` is not supported** — `ALTER GATEWAY` has no `RENAME TO`, so gateways
  *are* added to the sidebar's Rename-exclusion.
- **`SHOW GATEWAYS` omits the spec and ingress URL** — those come from
  `DESCRIBE GATEWAY` (the `spec` column is only returned to roles with USAGE /
  MODIFY / OWNERSHIP on the gateway).
- **The spec is dollar-quoted with a tagged `$THAW$` block**, not bare `$$`, so a
  literal `$$` inside the YAML can't prematurely close the block.
