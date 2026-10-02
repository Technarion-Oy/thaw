# components/service

> Modals for creating and managing Snowflake SERVICE (Snowpark Container Services) objects.

## Components

| File | Purpose |
|---|---|
| `CreateServiceModal.tsx` | Create form with a live `CREATE SERVICE` SQL preview. Fields: name, IF NOT EXISTS (no OR REPLACE — Snowflake doesn't support it), compute pool (picker from `ListComputePools`), specification (`ServiceSpecFields`), min/max instances, auto resume, query warehouse (picker), external access integrations, and a comment. The staged-spec picker passes `label="Browse internal stage — select the specification file"`. |
| `SpecEditor.tsx` | **Form \| YAML** specification editor shared by every SPCS spec surface (#958). The YAML text is the one value parents keep; Form mode edits the parsed document (`specDoc.ts`) and re-serialises, YAML mode is Monaco with model path `inmemory://spcs/<kind>.spcs` so the kind's JSON schema binds (see `editor/monacoSetup.ts`). Opens in Form when the text parses as a mapping, else YAML (templates always YAML, with a one-line reason). Renders the kind's cross-field problems below the editor. Children = the form. |
| `specDoc.ts` | Pure helpers: `parseSpec` / `dumpSpec` (the `yaml` package), `getIn` / `setIn` (immutable; an empty value deletes the key and prunes empty mappings, never array items), and the validators `validateServiceSpec(doc, job)`, `validateGatewaySpec`, `validateInferenceSpec`, plus `specProblems(kind, text)` that the modals use to gate submit. Tested in `specDoc.test.ts`. |
| `specFields.tsx` | Path-bound field primitives over `SpecCtx` (`Str`, `Num`, `Sel`, `Tags`, `Bool` with a documented default, `KV` key/value rows, `List` repeatable cards, `Group` native `<details>` sub-section, `Grid`). Forms are just these bound to spec paths, so keys a form doesn't render survive a round trip. |
| `ServiceSpecForm.tsx` | Service / job spec form: containers (image as database → schema → repository → image:tag dropdowns — repositories from `ListImageRepositories`, path segments from `repository_url`, images from `ListImagesInRepository` cached per repository; the image:tag box also offers the typed text as a "Use …" option, for images not pushed yet or unlistable without READ — composing `/db/schema/repo/image:tag`, secret picker from `ListSecretsInAccount`, env, resources, probes, volume mounts, session token), endpoints (port ⇄ TCP port range, protocol, public, CORS), volumes (source-dependent fields; block snapshot picker from `ListSnapshots`), logging & metric groups, autoscaling conditions, capabilities, service roles. `job` hides endpoints, service roles, readiness probe and autoscaling. Secret `objectReference` and `stageConfig.resources` are YAML-only. |
| `specSchemas/*.json` | Hand-written JSON Schemas (service/job, gateway, inference) from the Snowflake reference — Snowflake publishes none. Registered in monaco-yaml for the YAML mode. |
| `ServiceSpecFields.tsx` | The specification section shared by `CreateServiceModal` and the Container Services `RunJobModal` (`kind` `service` / `job`): inline `SpecEditor` + `ServiceSpecForm` **or** staged file via the shared `components/shared/StageFilePicker` (label "Browse internal stage — select the specification file"), and a **Template** toggle that switches to `SPECIFICATION_TEMPLATE[_FILE]` and reveals a **Template variables** (`USING`) key/value editor. Ticking Template forces YAML mode. Controlled (`value` / `onChange(patch)`); exports `EMPTY_SPEC` and `specReady(value, kind)` (which also requires no `specProblems` for a non-template inline spec). |
| `ServicePropertiesModal.tsx` | `SHOW SERVICES` + `DESCRIBE SERVICE` metadata: a **Status** tag, inline-editable **Settings** (comment, min/max instances, auto resume, query warehouse via `ALTER SERVICE SET/UNSET`), an editable **Specification** (`SpecEditor` + `ServiceSpecForm`, keyed on the live spec so it opens as the form when the spec parses; or a staged file via `shared/StageFilePicker`) with a **Redeploy** button (`RedeployService` → `ALTER SERVICE … FROM SPECIFICATION`, confirm first), lazily-loaded `shared/LazyResultTable` sections for **Endpoints** (`SHOW ENDPOINTS IN SERVICE`), **Instances** (`SHOW SERVICE INSTANCES IN SERVICE`), **Containers** (`SHOW SERVICE CONTAINERS IN SERVICE`) and **Volumes** (`SHOW SERVICE VOLUMES IN SERVICE`), **Service roles** (`SHOW ROLES IN SERVICE`; per role a `ServiceRoleGrants` chip editor modelled on `UserPropertiesModal`'s `PolicyManager` — grantee chips from `SHOW GRANTS OF SERVICE ROLE`, × revokes, add row picks Role / Database role / Application role), and **Logs** (the exported `ServiceLogs` component — `SYSTEM$GET_SERVICE_LOGS` with container/instance/lines inputs; also opened by the Container Services Jobs tab's **Logs…**), plus the generic property rows. |

## Integration

- Create delegates to `BuildCreateServiceSql` / `ExecDDL` and reads
  `ListComputePools` / `ListWarehouses` for the pickers.
- Properties delegates to `GetObjectProperties` (SHOW + DESCRIBE), `AlterService`
  (lifecycle/edit clauses), `RedeployService`, `ListServiceEndpoints`,
  `ListServiceInstances`, `GetServiceContainers`, `ListServiceVolumes`,
  `ListServiceRoles`, `ListServiceRoleGrants`, `GrantServiceRole` /
  `RevokeServiceRole`, `ListRoles` / `ListDatabaseRoles` (grantee pickers), and
  `GetServiceLogs`.
- `AlterService(db, schema, name, clause)` runs free-form `ALTER SERVICE …
  <clause>` for SUSPEND/RESUME (from the sidebar) and SET/UNSET of the mutable
  properties (from the modal).
- The lazy tables build an antd `Table` directly from the raw
  `snowflake.QueryResult` `columns`/`rows`, so they adapt to whatever columns the
  Snowflake edition reports. They load on demand (not on open) to avoid extra
  round-trips.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Services** group (kind `"SERVICE"`), with **Suspend** / **Resume** lifecycle
  actions. Services are not queryable tables, so there is no **Select Top 1000
  Rows**; `ALTER SERVICE` has no `RENAME TO`, so **Rename** is not offered.
- The form shape mirrors the Wails-generated `service.ServiceConfig`; a plain
  object literal is cast `as any` only at the IPC boundary.

## Gotchas

- **No `OR REPLACE`, no `RENAME`, no `GET_DDL`** — `CREATE SERVICE` has no OR
  REPLACE, services can't be renamed, and `GET_DDL` doesn't support the kind, so
  there's no DDL/View-Definition path; the properties panel relies on `SHOW
  SERVICES` + `DESCRIBE SERVICE`.
- **`SHOW SERVICES` omits the spec** — the YAML specification is fetched via
  `DESCRIBE SERVICE` (the `spec` column) and merged into the properties.
- **Grantees may be unlistable** — if `SHOW GRANTS OF SERVICE ROLE` fails, the
  role row shows the error instead of chips and keeps only the grant form.
  Grantees arrive already split into parent/grantee (Go-side
  `service.ParseServiceRoleGrants`); a database/application-role grant needs its
  parent filled in (the field resets per grantee kind).
- **Spec unreadable ≠ spec empty** — `GetObjectProperties` omits the `spec` row
  when `DESCRIBE SERVICE` fails; the modal then shows a warning above the (empty)
  editor, since redeploying from it would replace a spec the user can't see.
- **Form edits drop YAML comments** — Form mode re-serialises the parsed document; switching to YAML and back only reformats when a field is edited.
- **Template ≠ YAML** — a `{{ var }}` spec isn't valid YAML until rendered, so templates get the YAML editor with no schema (`template.spcs`) and skip the cross-field checks.
- **Suspend deletes containers** — suspending a service shuts down and removes its
  containers; resuming reconstructs them from the spec.
