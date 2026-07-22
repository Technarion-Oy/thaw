# internal/users

> ALTER USER per-property SQL builder for the user administration UI.

## Responsibility

Builds and executes single-property `ALTER USER ... SET / UNSET` statements,
mirroring the `internal/warehouse` property-builder pattern: the frontend's
`UserPropertiesModal` edits one property at a time, and each save routes
through `BuildAlterUserPropertySQL` so all validation and quoting lives here,
never inline in a component.

## Files

| File | Purpose |
|---|---|
| `users.go` | `BuildAlterUserPropertySQL(name, property, value)` — validates and renders one property change; `AlterProperty` executes it via the Snowflake client. |
| `users_test.go` | Table test covering quoting, escaping, UNSET-on-empty, enum/integer validation, and the `DEFAULT_SECONDARY_ROLES` / `TYPE` / `PASSWORD` special forms. |

## Property semantics

- **Strings** (`loginName`, `displayName`, `firstName`, `middleName`, `lastName`, `email`, `comment`) — quoted string literals; empty value → `UNSET`.
- **Identifiers** — `defaultWarehouse` / `defaultRole` are double-quoted exactly (their values are canonical-case names from SHOW via the UI's Selects); `networkPolicy` is typed free-hand and rendered via `QuoteOrBare` so bare names fold. `defaultNamespace` (`DB` or `DB.SCHEMA`) splits quote-aware via `snowflake.SplitQualifiedName` (the shared `sqltok`-based qualified-name splitter, capped at 2 parts; `splitNamespace` here is a thin alias) — explicitly-quoted parts keep exact case, bare parts stay bare, and a quoted part containing a literal dot (`"MY.DB".PUB`) stays one part. Empty → `UNSET`.
- **Integers** (`daysToExpiry`, `minsToUnlock`, `minsToBypassMfa`) — validated non-negative; empty → `UNSET`.
- **Booleans** (`disabled`, `mustChangePassword`) — `TRUE`/`FALSE` only; no UNSET. MFA is deliberately not managed here (`DISABLE_MFA` is a legacy Duo-era property with contested support); use `minsToUnlock`/`minsToBypassMfa` or `ALTER USER … REMOVE MFA METHOD` via the SQL editor.
- **Enums** — `type` (`PERSON`/`SERVICE`/`LEGACY_SERVICE`, empty → `UNSET`); `defaultSecondaryRoles` (`ALL` → `('ALL')`, `NONE` → `()`, empty → `UNSET`).
- **`password`** — set-only, never trimmed (spaces are legal), empty is an error.
- **RSA public keys** (`rsaPublicKey` → `RSA_PUBLIC_KEY`, `rsaPublicKey2` → `RSA_PUBLIC_KEY_2`) — the two key-pair-auth slots. The value is the stripped base64 payload; all whitespace/newlines are stripped so a copy-pasted multi-line key works, then it must match a strict base64 charset (`^[A-Za-z0-9+/]+=*$`). Anything else is **rejected** — a full PEM (`-----BEGIN/-----END-----` lines) gets a dedicated message, other non-base64 input a generic one. Because the field is fed by a free-form paste UI, this charset gate (not an assumption that "base64 has no backslashes") is what makes `QuoteStringLit` safe: no value that passes can contain a `'` or `\` to break out of the single-quoted literal. Empty → `UNSET` (removes the key, locking out that private key). This is the single builder for key registration; `internal/keypair` only *generates* keys.

## Gotchas

- `ListUsers` and `GetUserDDL` still live on the Snowflake client
  (`internal/snowflake/client.go`), not here — this package only owns the
  ALTER property path.
- Enum/integer validation delegates to the shared
  `snowflake.ValidateEnumValue` / `snowflake.ValidateNonNegativeInt` helpers
  (also used by `internal/warehouse`).
- `MINS_TO_BYPASS_MFA` is rejected by Snowflake for users without MFA enrolled;
  the error surfaces to the UI rather than being pre-checked.
