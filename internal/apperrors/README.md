# internal/apperrors

> Sentinel errors shared across all `internal/` packages.

## Responsibility

Provides a single, canonical location for application-level error values that
must cross package boundaries. Any package that needs to signal a
connection-required precondition can return or check `apperrors.ErrNotConnected`
without importing `internal/app` (which would create an import cycle).

## Key files

- `errors.go` — declares the exported sentinel error variables.
- `doc.go` — package documentation and `// thaw:domain: Core IPC & App Lifecycle`
  annotation used by the semantic-map generator.

## Key types & functions

| Symbol | Description |
|--------|-------------|
| `ErrNotConnected` | Returned by `*App` IPC methods when `a.client == nil`. The frontend surfaces this as a "not connected" user message. |

## Patterns & integration

- Every method in `internal/app/*.go` that requires a live Snowflake connection
  opens with the same guard:
  ```go
  if a.client == nil {
      return nil, apperrors.ErrNotConnected
  }
  ```
- Callers in `internal/app` import `thaw/internal/apperrors` directly; no other
  packages need to import this package because error propagation flows outward
  through `internal/app`.

## Gotchas

- The package is intentionally tiny — do not add non-sentinel (wrapping) errors
  here. Errors that carry dynamic context belong in their domain package.
