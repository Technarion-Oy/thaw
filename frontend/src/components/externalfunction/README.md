# components/externalfunction

UI for Snowflake **EXTERNAL FUNCTION** objects in the object browser — UDFs that
call code executed outside Snowflake (AWS Lambda, Azure Functions, GCP Cloud
Functions) by proxying an HTTPS request through an **API integration**.

## Components

- **`CreateExternalFunctionModal`** — builds a `CREATE EXTERNAL FUNCTION`
  statement. Name + `OR REPLACE` / `SECURE` (external functions have no
  `IF NOT EXISTS`), an argument editor (name + type rows), `RETURNS` type +
  `NOT NULL`, the required **API integration** (an `AutoComplete` sourced from
  `ListApiIntegrations`, free-typing allowed) and the required remote **URL**
  (`AS '<url>'`). Under **Advanced options**: null handling, volatility, static
  `HEADERS`, `CONTEXT_HEADERS` (a multi-select of the supported Snowflake context
  functions — `CURRENT_TIMESTAMP`, `CURRENT_USER`, …), `MAX_BATCH_ROWS`,
  `COMPRESSION`, and the request/response translators (an `AutoComplete` of the
  database's scalar UDFs from `ListUserFunctions` / `SHOW USER FUNCTIONS` (table
  and external functions are filtered out), free-typing
  allowed). Live SQL preview; submission is gated on
  name + return type + API integration + URL all being present. SQL is built by
  `BuildCreateExternalFunctionSql` (`internal/externalfunction`).
- **`ExternalFunctionPropertiesModal`** — overview (owner, created-on, language)
  + an editable **Settings** section (inline-editable comment and a `SECURE`
  toggle, both via `AlterExternalFunction`) + an **External Function Detail**
  section backed by `DescribeExternalFunction` (`DESCRIBE FUNCTION`): API
  integration, URL (`body`), headers, context headers, max batch rows,
  compression, request/response translators, signature, returns, null handling,
  and volatility. Takes the argument signature (`args`) needed to resolve the
  overload for `DESCRIBE` / `ALTER FUNCTION`.

## Notes

- External functions are dropped with `DROP FUNCTION <fqn>(<args>)` and renamed /
  altered with `ALTER FUNCTION <fqn>(<args>) …` — all of which need the argument
  signature, carried through the sidebar tree as `objArgs`.
- `GET_DDL` (View Definition / Compare) works via the `'FUNCTION'` object type;
  the kind normalization lives in `internal/snowflake`.
