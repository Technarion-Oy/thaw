# internal/service

> SQL builder for Snowflake SERVICE (Snowpark Container Services) objects.

## Responsibility

Builds the `CREATE SERVICE` DDL from a structured config. A service is a
long-running, containerized application that runs in a compute pool, defined by a
YAML specification (supplied inline or referenced from a stage file). Services
expose ingress endpoints, container logs, and a per-container status.

Snowflake requires the compute pool and the specification to come first, then the
remaining properties in documented order. Services have **no `OR REPLACE`** and
**cannot be renamed** (`ALTER SERVICE` has no `RENAME TO`). The mutable
properties — `MIN_INSTANCES`, `MAX_INSTANCES`, `AUTO_RESUME`, `QUERY_WAREHOUSE`,
`EXTERNAL_ACCESS_INTEGRATIONS`, `COMMENT` — plus the `SUSPEND`/`RESUME` lifecycle
are issued as free-form `ALTER SERVICE <fqn> <clause>` statements directly from
`internal/app/service.go` (`App.AlterService`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ServiceConfig`, `BuildCreateServiceSql`, spec-source constants |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ServiceConfig` | CREATE parameters: name, case sensitivity, `IfNotExists`, `ComputePool`, `SpecSource` (`inline`/`stage`), `Template` (toggles the `SPECIFICATION_TEMPLATE[_FILE]` variants), `SpecInline`/`SpecStage`/`SpecFile`, `TemplateVars` (`USING` bindings), `ExternalAccessIntegrations`, `AutoResume`, `MinInstances`, `MaxInstances`, `QueryWarehouse`, `Comment` |
| `BuildCreateServiceSql(db, schema, cfg)` | Emits `CREATE SERVICE [IF NOT EXISTS] <fqn> IN COMPUTE POOL … { FROM SPECIFICATION[_TEMPLATE] $$…$$ \| FROM @<stage> SPECIFICATION[_TEMPLATE]_FILE='…' } [USING (k => v, …)] [options];` |
| `TemplateVar` | A single `name => value` binding for the `USING` clause of a templated spec |
| `SpecSourceInline` / `SpecSourceStage` | `SpecSource` values selecting inline vs. staged specification |

## Patterns & integration

- A blank name emits the placeholder `service_name`, a blank compute pool emits
  `<compute_pool>`, and a blank spec emits a minimal YAML template — so the live
  SQL preview reads as a completable template while the user is still typing.
- Inline specs are wrapped in dollar-quoting (`$$ … $$`) so multi-line YAML needs
  no escaping; staged specs reference `FROM @<stage> SPECIFICATION_FILE = '…'`.
- When `Template` is set, the spec keyword becomes `SPECIFICATION_TEMPLATE` /
  `SPECIFICATION_TEMPLATE_FILE` and the `TemplateVars` are emitted as a trailing
  `USING ( key => value, … )` clause (only for templates). Values are rendered as
  SQL literals by `renderUsingValue`: integers/floats and the keywords
  `TRUE`/`FALSE`/`NULL` are emitted bare, everything else is single-quoted.
  Bindings with a blank key are skipped; if none remain, `USING` is omitted.
- `App.BuildCreateServiceSql` (in `internal/app/builders.go`) is the thin IPC
  delegator. `App.AlterService` (in `internal/app/service.go`) runs the
  lifecycle/edit clauses, and `App.GetServiceLogs`, `App.ListServiceEndpoints`,
  and `App.GetServiceContainers` back the properties panel's lazy sections.
- Discovery: `Client.ListExtendedObjects` runs `SHOW SERVICES IN SCHEMA` with the
  fixed kind `"SERVICE"`. Services are not surfaced by `SHOW OBJECTS`, so — like
  masking policies, network rules, image repositories, tags, and alerts — no
  dedupe pass is needed.
- Properties panel: `internal/objects` runs `SHOW SERVICES LIKE …` for the
  `SERVICE` kind and enriches it with `DESCRIBE SERVICE` to surface the `spec`.

## Gotchas

- **No `OR REPLACE`** — `CREATE SERVICE` does not support it (only `IF NOT
  EXISTS`); the builder never emits it. Redeploy code with `ALTER SERVICE … FROM
  SPECIFICATION`.
- **No `RENAME`** — `ALTER SERVICE` has no `RENAME TO` clause, so services are
  excluded from the sidebar's Rename action (like alerts, external tables, image
  repositories, and network rules).
- **`GET_DDL` is not supported** for services, so there is no DDL export / "View
  Definition" path and no `buildGetDDLQuery` mapping for this kind. The properties
  panel relies on `SHOW SERVICES` + `DESCRIBE SERVICE`.
- **`SUSPEND` deletes the containers** — suspending a service shuts down and
  removes its containers (state is reconstructed on `RESUME` from the spec).
