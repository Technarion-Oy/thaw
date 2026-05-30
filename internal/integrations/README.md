# internal/integrations

> SQL DDL builders for Snowflake `CREATE INTEGRATION` statements (Storage, API, Catalog, External Access, Notification, Security).

## Responsibility

Generates correct, injection-safe `CREATE INTEGRATION` DDL for all six Snowflake
integration types from typed parameter structs. All string values are single-quoted with
embedded quotes doubled (`sq`); identifier values are double-quoted (`qident`); enum
fields are validated against an allowed-values list (`mustBeOneOf`) before being embedded
in SQL.

Note: this package (`internal/integrations`) is distinct from `internal/integration`
(live Snowflake test suite).

## Key files

| File | Purpose |
|---|---|
| `builder.go` | All parameter structs and `Build*SQL` functions; internal helpers `sq`, `qident`, `boolKw`, `identToken`, `splitValues`, `squotedTuple`, `identListFromString`, `validateIdentRef`, `mustBeOneOf`, `secretsTuple`. |
| `builder_test.go` | Unit tests for the builder functions. |

## Key types & functions

| Symbol | Description |
|---|---|
| `StorageIntegrationParams` | S3/S3GOV/GCS/AZURE storage integration parameters. |
| `ApiIntegrationParams` | AWS/Azure/Google/git_https_api API integration parameters (incl. GitHub App, OAuth2, PrivateLink modes). |
| `CatalogIntegrationParams` | GLUE/OBJECT_STORE/POLARIS/ICEBERG_REST/SAP_BDC catalog integration parameters. |
| `ExternalAccessIntegrationParams` | Network rules, auth integrations, auth secrets. |
| `NotificationIntegrationParams` | EMAIL, WEBHOOK, Azure/GCP/AWS queue integrations. |
| `SecurityIntegrationParams` | API_AUTHENTICATION, EXTERNAL_OAUTH, OAUTH_PARTNER, OAUTH_CUSTOM, SAML2, SCIM. |
| `BuildStorageIntegrationSQL(p)` | Returns `(string, error)`. |
| `BuildApiIntegrationSQL(p)` | Routes to `buildGitHttpsApiSQL` for `git_https_api` provider. |
| `BuildCatalogIntegrationSQL(p)` | Handles all five catalog sources including auth-type switching for ICEBERG_REST. |
| `BuildExternalAccessIntegrationSQL(p)` | Validates network rule and secret references as identifier refs. |
| `BuildNotificationIntegrationSQL(p)` | Handles all seven notification subtypes. |
| `BuildSecurityIntegrationSQL(p)` | Handles all six security integration types. |

## Patterns & integration

- Called from `internal/app/integrations.go` thin delegators (one `Build*SQL` call per IPC method).
- All enum parameters are validated case-insensitively and the canonical Snowflake casing is returned; invalid values produce descriptive errors that surface in the frontend.
- `identListFromString` and `validateIdentRef` use a regex (`validIdentRef`) that accepts both double-quoted and unquoted (dot-separated) Snowflake identifier references, preventing injection via list fields like `ALLOWED_NETWORK_RULES`.

## Gotchas

- `git_https_api` API integration is handled by a dedicated `buildGitHttpsApiSQL` function because its DDL structure (especially `API_USER_AUTHENTICATION` block) differs significantly from the other API providers.
- `secretsTuple` distinguishes the special keywords `ALL` and `NONE` from ordinary identifier references to avoid quoting them.
