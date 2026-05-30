# internal/secret

> SQL builders for creating and modifying Snowflake SECRET objects.

## Responsibility

Constructs `CREATE SECRET` and `ALTER SECRET` SQL statements for all Snowflake
secret types (OAuth2, Cloud Provider Token, Password, Generic String, Symmetric
Key). Validates the secret type before generating SQL and handles type-specific
clauses (OAuth flow, scopes, refresh token expiry, API authentication, etc.).

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (Object Browser & Administration) |
| `sql.go` | `SecretType`, `SecretConfig`, `BuildCreateSecretSql`, `BuildModifySecretSql` |

## Key types & functions

### `SecretType`
```go
type SecretType string

const (
    SecretTypeOAuth2             SecretType = "OAUTH2"
    SecretTypeCloudProviderToken SecretType = "CLOUD_PROVIDER_TOKEN"
    SecretTypePassword           SecretType = "PASSWORD"
    SecretTypeGenericString      SecretType = "GENERIC_STRING"
    SecretTypeSymmetricKey       SecretType = "SYMMETRIC_KEY"
)
```
`isValid()` returns true only for these five constants, preventing unknown type
values from being interpolated into SQL.

### `SecretConfig`
Carries all parameters that can appear in a `CREATE SECRET` statement. Fields that
do not apply to the selected `Type` are ignored by the builders. Received from the
frontend as JSON.

| Field group | Fields |
|-------------|--------|
| Common | `Name`, `CaseSensitive`, `OrReplace`, `IfNotExists`, `Type`, `Comment` |
| OAuth2 | `OAuthFlow` (`CLIENT_CREDENTIALS` or `AUTHORIZATION_CODE`), `ApiAuthentication`, `OAuthScopes`, `OAuthRefreshToken`, `OAuthRefreshTokenExpiry` |
| Cloud Provider Token | `ApiAuthentication`, `Enabled` |
| Password | `Username`, `Password` |
| Generic String | `SecretString` |

### `BuildCreateSecretSql(db, schema string, cfg SecretConfig) (string, error)`
Returns a complete `CREATE [OR REPLACE] SECRET [IF NOT EXISTS] ...;` statement.
Returns an error when `cfg.Type` fails the `isValid()` check.
- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive (`IF NOT EXISTS` is
  suppressed when `OrReplace` is true, mirroring Snowflake's own DDL rules).
- All string literals are single-quote-escaped via the private `escLit` helper.
- Identifier quoting delegates to `snowflake.QuoteIdent` / `snowflake.QuoteOrBare`.

### `BuildModifySecretSql(db, schema, name string, cfg SecretConfig, originalComment string) ([]string, error)`
Returns a slice of `ALTER SECRET ... SET ...;` statements (one for the property
changes, optionally a second `UNSET COMMENT` when the comment was cleared).
Only fields with non-empty values are included in the `SET` clause; the caller is
responsible for determining which fields have actually changed.

## Patterns & integration (thin-delegator)

`internal/app` passes the `SecretConfig` struct received from the frontend
directly to these builders, executes the resulting SQL via `client.ExecDDL` (or
`client.Execute`), and returns any error to the frontend. No live connection is
required to test the builders.

```go
// internal/app/objects.go (illustrative)
func (a *App) CreateSecret(db, schema string, cfg secret.SecretConfig) error {
    if a.client == nil { return apperrors.ErrNotConnected }
    sql, err := secret.BuildCreateSecretSql(db, schema, cfg)
    if err != nil { return err }
    _, err = a.client.Execute(a.ctx, sql)
    return err
}
```

## Gotchas

- `SecretTypeGenericString` and `SecretTypeSymmetricKey` constants are annotated
  with `// #nosec G101` suppression comments because `gosec` incorrectly flags
  them as hardcoded credentials. They are Snowflake DDL keyword values, not actual
  secrets.
- `BuildModifySecretSql` can return an empty slice (no statements) if no fields
  are set in `cfg` and `originalComment` is also empty. Callers should handle the
  empty-slice case gracefully.
- For `SecretTypeSymmetricKey`, `CREATE SECRET` only requires `ALGORITHM = GENERIC`
  with no user-supplied secret material; the `Modify` builder emits no SET clauses
  for this type because there is nothing to change after creation.
