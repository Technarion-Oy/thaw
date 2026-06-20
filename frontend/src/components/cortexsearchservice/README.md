# components/cortexsearchservice

> Modals for creating and managing Snowflake CORTEX SEARCH SERVICE objects.

## Components

| File | Purpose |
|---|---|
| `CreateCortexSearchServiceModal.tsx` | Create form with a live `CREATE CORTEX SEARCH SERVICE` SQL preview, covering **both** documented shapes via an **Index mode** toggle. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive via the shared `NameWithReplaceOptions`; IF NOT EXISTS is force-cleared in multi mode, which Snowflake disallows there), case control; **single mode** → **search column** (`ON`) + optional **embedding model** combobox; **multi mode** → **vector indexes** (`Select mode="tags"`, ≥1 required, each a vector column or `BODY (model='…')` managed-embedding spec) + optional **text indexes**; shared **primary key** and **attributes** tag editors, a **Target Lag** composer (number + seconds/minutes/hours/days), a **warehouse** picker (`ListWarehouses`), an **Advanced** section (refresh mode, initialize, full-index-rebuild days, auto-suspend seconds, request-logging switch), an optional comment, and the **base query** (`AS`) in a shared Monaco SQL field. |
| `CortexSearchServicePropertiesModal.tsx` | `SHOW CORTEX SEARCH SERVICES` + `DESCRIBE CORTEX SEARCH SERVICE` metadata, exposing **every** `ALTER` option: **Refresh** plus **Suspend**/**Resume** split-dropdowns (each scoping to all / `INDEXING` / `SERVING`) with `indexing`/`serving` state tags; an **Overview** (search column, embedding model — both read-only/immutable); inline-editable **Settings** (`SET TARGET_LAG`, `SET WAREHOUSE`, `SET`/`UNSET ATTRIBUTES`, `SET`/`UNSET PRIMARY KEY`, `SET AUTO_SUSPEND`/`= NULL`, `SET FULL_INDEX_BUILD_INTERVAL_DAYS`, `SET REQUEST_LOGGING`, `SET`/`UNSET COMMENT`); a **Tags** section (chips from `GetCortexSearchServiceTags`, add → `SET TAG`, Popconfirm remove → `UNSET TAG`); a **Scoring Profiles** section (`ADD`/`DROP SCORING PROFILE`); the generic property rows; and the **base query** (`definition`) shown read-only. |

## Integration

- Both delegate to IPC: `BuildCreateCortexSearchServiceSql` / `ExecDDL` (create)
  and `GetObjectProperties` / `AlterCortexSearchService` /
  `FormatCortexSearchAttributes` / `GetCortexSearchServiceTags` / `ListWarehouses`
  (properties + edits).
- `AlterCortexSearchService(db, schema, name, clause)` runs free-form
  `ALTER CORTEX SEARCH SERVICE … <clause>` and is the single entry point for every
  mutation and lifecycle action: `SUSPEND`/`RESUME` (`[INDEXING|SERVING]`),
  `REFRESH`, `SET`/`UNSET` of `TARGET_LAG`, `WAREHOUSE`, `ATTRIBUTES`,
  `PRIMARY KEY`, `AUTO_SUSPEND`, `FULL_INDEX_BUILD_INTERVAL_DAYS`,
  `REQUEST_LOGGING`, `COMMENT`, `SET`/`UNSET TAG`, and `ADD`/`DROP SCORING
  PROFILE`.
- `FormatCortexSearchAttributes(cols)` joins the column list for the
  `SET ATTRIBUTES ( … )` / `SET PRIMARY KEY ( … )` clauses (drops blanks),
  mirroring the Go builder.
- `GetCortexSearchServiceTags(db, schema, name)` reads currently applied tags via
  `INFORMATION_SCHEMA.TAG_REFERENCES`; the modal treats an error as "no tags" and
  still allows `SET`/`UNSET TAG`.
- The properties modal reads everything via `GetObjectProperties`, which runs
  `SHOW CORTEX SEARCH SERVICES LIKE …` and enriches it with `DESCRIBE CORTEX
  SEARCH SERVICE` (search column, attributes, embedding model, definition, target
  lag, warehouse, serving/indexing state, primary key, auto-suspend,
  request-logging, full-index-build-interval) in `internal/objects`.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Cortex Search Services** group (kind `"CORTEX SEARCH SERVICE"`), in the
  **Machine Learning** Create-Object submenu.
- The form shape mirrors the Wails-generated
  `cortexsearchservice.CortexSearchServiceConfig`; a plain object literal is cast
  `as any` only at the IPC boundary.

## Gotchas

- **No `GET_DDL`** — the get_ddl object-type enumeration omits cortex search
  services, so there's no DDL/View-Definition/comparison path; the properties
  panel relies on `SHOW` + `DESCRIBE`.
- **No `RENAME`** — `ALTER CORTEX SEARCH SERVICE` has no `RENAME TO`, so the kind
  is in the sidebar's Rename-exclusion and gets a dedicated **Properties…** item.
- **`EMBEDDING_MODEL` is immutable** after creation, so the properties modal shows
  it read-only.
- Both CREATE shapes are modelled — the single-index (`ON <column>`) form and the
  multi-index (`TEXT INDEXES` / `VECTOR INDEXES`) form — chosen via the Index mode
  toggle. The multi-index form drops `EMBEDDING_MODEL` and `IF NOT EXISTS` (not
  allowed by Snowflake there). Scoring profiles are not part of CREATE but **are**
  reachable post-create via the properties modal's `ADD`/`DROP SCORING PROFILE`
  section (the profile body is entered as raw SQL).
