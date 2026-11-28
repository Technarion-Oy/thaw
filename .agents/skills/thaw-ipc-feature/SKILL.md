---
name: thaw-ipc-feature
description: >-
  Use this skill when adding or modifying a backend domain feature, Wails Go-to-frontend
  IPC method, native menu item, or feature flag (including IT admin policy locks) in Thaw.
---

# Thaw IPC Feature & Feature Flag Workflow

Follow this procedure when building or updating a feature that spans the Go backend (`internal/`) and React/TypeScript frontend (`frontend/src/`).

## 1. Locate the Domain & Read Context
1. Open `internal/architecture/semantic_map.go` and find the target domain in `GetCodebaseSemanticMap()`.
2. Restrict new and modified files to the directories owned by that domain.
3. Read the `README.md` of each `internal/<pkg>/` and `frontend/src/<dir>/` folder you will touch, plus `docs/concepts/patterns.md`.

## 2. Implement Domain Logic in `internal/<domain>/`
1. Put all SQL generation, `snowflake.QueryResult` parsing, validation, and orchestration in `internal/<domain>/` — never in `internal/app/` or React components.
2. Export pure `Build*Sql(...) (string, error)` builders and `Parse*(res *snowflake.QueryResult)` parsers for connectionless unit tests.
   - If a builder embeds an unquoted/pre-validated value, keep the builder **unexported**, validate in the exported `Get*` wrapper, and panic on invariant violation inside the builder (see `internal/queryhistory`).
3. Reuse `internal/snowflake/result.go` (`ColIdx`, `Cell*`, `PropertyPair`, `ResultToPairs`) and `internal/snowflake` identifier quoting (`"DB"."SCHEMA"."OBJ"`, escaping `"` as `""`).
4. Initialize empty returned slices with `make([]T, 0, n)` (never `nil`, which Wails marshals as JSON `null`) and format `time.Time` fields as RFC3339 `string`.
5. If adding a new package, add `// thaw:domain: <Domain Name>` in `doc.go`, create `internal/<domain>/README.md`, and run `go generate ./internal/architecture/`.
6. Write unit tests in `internal/<domain>/*_test.go`.

## 3. Add Thin-Delegator IPC Binding in `internal/app/`
1. Add the public method on `(a *App)` in the matching `internal/app/<domain>.go` file (not `app.go`):
   - Check connection (`if a.client == nil { return …, apperrors.ErrNotConnected }`) and any required feature flag (`a.GetFeatureFlags().<Flag>`).
   - Delegate in one call to `internal/<domain>` and return domain types.
2. Run `wails generate module` from the repo root to update `frontend/wailsjs/`. Never edit `frontend/wailsjs/` by hand.

## 4. Wire Native Menu Items (If Applicable)
1. In `internal/app/menu.go`, use `addConnectionItem(menu, label, accelerator, "menu:<event>", data...)` for items requiring a Snowflake connection, or `AddText` for offline-capable items.
2. When multiple items open the same dialog, emit one event with a differentiating `data` payload and narrow that payload defensively in `QueryPage.tsx`.
3. If the menu item is feature-flag gated, register it in the flag-gated slice recomputed by `setMenuFeatureFlags` (see `containerServices` in `menu.go`).

## 5. Wire Feature Flags (Only When Justified)
Basic functionality ships always-on (issue #567). Add a user-facing toggle only if it hides an admin surface, declutters a whole optional workflow, stops background Snowflake/AI queries, or is experimental/opt-in.

### Adding a flag
1. Add the `bool` field to `FeatureFlags` in `internal/config/config.go`, set its default in `DefaultFeatureFlags()`, bump `flagsVersion`, and add migration logic in `MigrateFlags()`.
2. If admin-lockable, add it to `internal/config/adminconfig.go` and its `_darwin.go` / `_windows.go` policy-key tables.
3. Run `wails generate module`.
4. Add `<FlagRow />` (with `locked={locked.<flag>}`) to `frontend/src/components/settings/FeatureFlagsModal.tsx` and list the field in `frontend/src/store/featureFlagsStore.ts`.
5. Gate every UI entry point (sidebar, toolbar, menu, modal) via `featureFlagsStore` and enforce on the Go IPC method when relevant.

### Removing a user toggle
- **Not admin-lockable**: delete the field from `FeatureFlags`/`DefaultFeatureFlags`/`MigrateFlags`, bump `flagsVersion`, remove `<FlagRow />` and store entry, un-gate consumers, and run `wails generate module`.
- **Keep admin lock**: leave `config.go` and `adminconfig.go` fields intact, remove only `<FlagRow />`, and add the field to `ForceAlwaysOnFlags()` in `internal/config/config.go`.

## 6. Build the Frontend UI
1. Import bindings from `../../../wailsjs/go/app/App` (or `wailsjs/go/sqleditor/Service`) and guard arrays with `?? []`.
2. Mount modals conditionally (`{open && <Modal open destroyOnClose … />}`) so global drag/resize resets on reopen.
3. Use `const { modal } = AntApp.useApp()` for confirmations — never static `Modal.confirm`.
4. If the dialog is reachable from multiple entry points, hold visibility in a small Zustand store and mount it once in `QueryPage.tsx`.
5. Add `// @thaw-domain: <Domain Name>` to new domain TS/TSX files and run `go generate ./internal/architecture/`.

## 7. Verify & Update Docs
1. Run:
   ```bash
   go test -race ./...
   cd frontend && npx tsc --noEmit && npm test
   ```
2. Update `README.md`, `FEATURES.md`, `CLAUDE.md`, `GEMINI.md`, affected `internal/<pkg>/README.md` and `frontend/src/<dir>/README.md`, and `docs/concepts/*.md` in the same PR.
