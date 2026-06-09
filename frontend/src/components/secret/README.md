# frontend/src/components/secret

> Modals for creating and modifying Snowflake Secret objects.

## Responsibility

Provides `CREATE SECRET` and `ALTER SECRET` forms. Both modals use the debounced
SQL preview pattern: form state changes call a backend builder, which produces DDL
displayed inline. On submit the DDL is executed via `ExecDDL`. Five secret types are
supported: OAUTH2, CLOUD_PROVIDER_TOKEN, PASSWORD, GENERIC_STRING, SYMMETRIC_KEY.

## Files

| File | Purpose |
|------|---------|
| `CreateSecretModal.tsx` | `CREATE SECRET` form with type selector, type-specific credential fields, and `BuildCreateSecretSql` live preview. Calls `onSuccess(qualifiedName)` after creation. |
| `ModifySecretModal.tsx` | `ALTER SECRET` form. Loads current properties via `GetObjectProperties` + `ListSecurityIntegrations` in parallel on mount. Uses `BuildModifySecretSql` (returns multi-statement DDL joined by `\n\n`). Executes statements sequentially. |

## Patterns & integration

**IPC calls:**
- `BuildCreateSecretSql(db, schema, cfg)` / `BuildModifySecretSql(db, schema, name, cfg)` — debounced SQL builders; result shown in read-only preview
- `ExecDDL(sql)` — executes the generated DDL on submit
- `GetObjectProperties(db, schema, "SECRET", name)` — populates Modify form on mount
- `ListSecurityIntegrations()` — populates the OAuth security integration select
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl` in Create

**`secret.SecretConfig` type** (from `wailsjs/go/models`): `secretType`, `oauthScopes`, `oauthRefreshToken`, `oauthTokenEndpoint`, `securityIntegration`, `username`, `password`, `secretString`, `token`.

**Shared components:** `ObjectNameCaseControl` (Create only), `SqlPreview`, `MultiInput` (for OAuth scope list).

**`onSuccess` callback (Create):** Receives the fully-qualified secret name, uppercased when `caseSensitive` is false, double-quoted when true. The caller uses this to update the sidebar tree.

## Gotchas

- Sensitive fields (`password`, `secretString`, `oauthRefreshToken`) are never returned in `GetObjectProperties` output — they arrive as empty strings. The Modify modal leaves them blank by default; users must re-enter values to change them.
- `BuildModifySecretSql` may return multiple DDL statements (e.g., separate `ALTER SECRET … SET` statements). The Modify modal splits on `\n\n` and executes each statement sequentially with `for...of` + `await ExecDDL(stmt)`.
