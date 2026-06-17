# internal/externalfunction

Builds SQL for Snowflake **EXTERNAL FUNCTION** objects — user-defined functions
that call code executed *outside* Snowflake (AWS Lambda, Azure Functions, GCP
Cloud Functions) by proxying an HTTPS request through an **API integration**.
Unlike regular UDFs, external functions have no body: every call routes through
`API_INTEGRATION` and a remote URL, with optional transport options (headers,
batching, compression, request/response translators).

## What it does

`BuildCreateExternalFunctionSql(db, schema, cfg)` renders a
`CREATE EXTERNAL FUNCTION` statement from an `ExternalFunctionConfig`:

```
CREATE [OR REPLACE] [SECURE] EXTERNAL FUNCTION <fqn> ( [ <arg> <type> [, ...] ] )
  RETURNS <result_type> [NOT NULL]
  [ { CALLED ON NULL INPUT | RETURNS NULL ON NULL INPUT | STRICT } ]
  [ { VOLATILE | IMMUTABLE } ]
  [ COMMENT = '<string>' ]
  API_INTEGRATION = <integration>
  [ HEADERS = ( '<h>' = '<v>', ... ) ]
  [ CONTEXT_HEADERS = ( <context_fn>, ... ) ]
  [ MAX_BATCH_ROWS = <int> ]
  [ COMPRESSION = { NONE | AUTO | GZIP | DEFLATE } ]
  [ REQUEST_TRANSLATOR = <udf> ]
  [ RESPONSE_TRANSLATOR = <udf> ]
  AS '<url_of_proxy_and_resource>';
```

## Types & builders

- `ExternalFunctionConfig` — name + case sensitivity, `OrReplace`, `Secure`,
  `Args` (`[]ExternalFunctionArg`), `Returns`, `NotNull`, `NullHandling`,
  `Volatility`, `Comment`, `ApiIntegration`, `Headers` (`[]HeaderPair`),
  `ContextHeaders`, `MaxBatchRows`, `Compression`, `RequestTranslator`,
  `ResponseTranslator`, `Url`.
- `ExternalFunctionArg` — a single `{Name, Type}` parameter.
- `HeaderPair` — a single `{Name, Value}` HTTP header.
- `BuildCreateExternalFunctionSql` — the only exported builder.
- `BuilderOptions` + `GetBuilderOptions` (`options.go`) — the fixed choice lists
  the grammar accepts (`COMPRESSION`, null-handling and volatility modifiers, and
  the supported `CONTEXT_HEADERS` context functions). They live here, not
  hardcoded in the React modal, so the SQL grammar and its UI options stay
  defined in one place; the create modal fetches them via
  `App.GetExternalFunctionOptions`.

## Gotchas

- `API_INTEGRATION` and the `AS '<url>'` are **mandatory**. The builder fills in
  placeholders (`<api_integration>`, `<url_of_proxy_and_resource>`) for empty
  required fields so the live preview stays readable; the Create modal gates
  submission on the real values being present.
- External functions share the **regular `FUNCTION` management commands** — there
  is no `ALTER`/`DROP EXTERNAL FUNCTION`. Mutations (`SET`/`UNSET COMMENT`,
  `SET SECURE`, `SET API_INTEGRATION`, …) are issued as free-form
  `ALTER FUNCTION <fqn>(<args>) <clause>` via `App.AlterExternalFunction`; the
  Sidebar drops them with `DROP FUNCTION <fqn>(<args>)`. All of these need the
  argument **signature**, which the tree carries as `objArgs`.
- `SHOW EXTERNAL FUNCTIONS` omits the transport detail (API integration, URL,
  headers, translators, compression). `App.DescribeExternalFunction` runs
  `DESCRIBE FUNCTION <fqn>(<args>)` to supply it for the properties panel.
- External functions also surface in `SHOW FUNCTIONS` with
  `is_external_function = Y`; `internal/snowflake` (`showInSchema`) skips those on
  the generic `FUNCTION` path so they appear only under **External Functions**,
  with `dedupeExternalFunctions` as a belt-and-suspenders fallback.
- `GET_DDL` has **no `EXTERNAL_FUNCTION` object type** — external functions are
  retrieved via the `'FUNCTION'` type with the argument signature appended. The
  `"EXTERNAL FUNCTION"` → `FUNCTION` normalization lives in `internal/snowflake`
  (`buildGetDDLQuery`), not here.
