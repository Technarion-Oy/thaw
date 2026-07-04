# frontend/src/components/account

> Account-level administration panels and modals: users, warehouses, integrations, query history, and key-pair authentication.

## Responsibility

Provides the UI for administering Snowflake account objects that are not schema-scoped. All panels are hosted inside `AccountPanel`, which renders a tree of roles and warehouses and acts as the entry point for every sub-panel and modal in this folder.

## Files

| File | Purpose |
|---|---|
| `AccountPanel.tsx` | Root panel: Ant Design `Tree` of roles and warehouses; hosts all context actions (DDL copy, properties, metering, query history, DDL export); conditionally renders `UserManagementPanel`, `IntegrationsPanel`, `BackupPoliciesPanel`. Tagged `@thaw-domain: Object Browser & Administration`. |
| `UserManagementPanel.tsx` | Table of Snowflake users with Create, Properties, Drop, and key-pair assignment actions; reads from `ListUsers` IPC and routes to `CreateUserModal`, `UserPropertiesModal`, `KeyPairAuthModal`. Gated behind `userRoleManagement` feature flag. Actions are always enabled — insufficient-privilege failures surface as an error message rather than being pre-disabled (privilege pre-checks were unreliable). |
| `CreateUserModal.tsx` | Form for `CREATE USER` with login name, display name, role, warehouse, default database/schema, and password; calls `CreateUser` IPC. |
| `UserPropertiesModal.tsx` | `Properties: USER` modal (the "Properties" context action). Same per-property inline-edit pattern as `WarehousePropertiesModal`: every settable `ALTER USER` property is a typed `EditRow` (text / number / select / boolean switch) grouped into Identity / Defaults / Security sections, each saving independently through the `AlterUserProperty` IPC (backed by `internal/users.BuildAlterUserPropertySQL`) and reloading the list. Dropdowns for `TYPE`, `DEFAULT_SECONDARY_ROLES`, default warehouse/role; password reset row (set-only); read-only Info section (owner, logins, MFA) with a Disable-MFA action when Duo is enrolled. Values load via `GetObjectProperties` (`SHOW USERS` merged with `DESCRIBE USER`, so DESCRIBE-only properties like `NETWORK_POLICY` / `MIDDLE_NAME` read correctly); clearing a value emits `UNSET`. Row components come from `../common/PropertyRows`. Session/object *parameters* are out of scope (set them via the SQL editor). |
| `KeyPairAuthModal.tsx` | Generates or imports RSA key pairs; calls `CheckAvailableKeyTools`, `GenerateKeyPair`, and `SetUserPublicKey` IPC methods from `internal/keypair`. Returns a `keypair.KeyPairResult`. |
| `IntegrationsPanel.tsx` | Ant Design `Tree` grouped by integration kind (Storage, API, Security, Catalog, etc.); supports Create, Modify, Drop, and Properties via `ListIntegrations`, `DropIntegration`, `GetIntegrationProperties`. Create is always enabled (no privilege pre-check); failures surface as an error message. Gated behind `integrationsManagement` feature flag. |
| `CreateIntegrationModal.tsx` | Wizard-style form for `CREATE INTEGRATION` covering storage, API, and security kinds; calls `CreateIntegration` IPC. |
| `IntegrationModifyModal.tsx` | Modal for `ALTER INTEGRATION` property changes; calls `AlterIntegration` IPC. |
| `QueryHistoryModal.tsx` | Searchable/paginated query history table; calls `GetQueryHistory` IPC (backed by `internal/queryhistory`). Opens on the **current user / today** by default (time range pre-filled and adjustable). Scope switches between session (free-text Session ID box), user, warehouse, or all. Each expanded row shows its **Session ID** with a **Filter by this session** drill-down action; the session scope's Run stays disabled until an id is entered, so it never silently queries the pooled metadata connection. Gated behind `queryActivityHistory` feature flag. |
| `WarehouseMeteringModal.tsx` | Credit-usage charts and table for a selected warehouse; calls `GetWarehouseMeteringHistory` IPC (backed by `internal/warehouse`). Gated behind `warehouseCreditUsage` feature flag. |
| `WarehousePropertiesModal.tsx` | Displays and allows editing of warehouse properties; calls `GetObjectProperties` and `AlterWarehouseProperty` IPC methods; uses `PropertiesModal` from `common/` for the property table. Gated behind `warehouseManagement` feature flag. |

## Patterns & integration

- **IPC**: All calls go through `wailsjs/go/app/App` to `internal/app` delegators, which forward to `internal/users`, `internal/warehouse`, `internal/queryhistory`, `internal/keypair`, and `internal/snowflake`.
- **Stores**: `useConnectionStore` (current role/warehouse context), `useFeatureFlagsStore` (gate checks), `useDiffStore`/`useGitStore` (DDL export paths in `AccountPanel`).
- **Feature flags**: `userRoleManagement`, `warehouseManagement`, `warehouseCreditUsage`, `queryActivityHistory`, `integrationsManagement`, `backupPoliciesAndSets` — all checked from `useFeatureFlagsStore`. When a flag is disabled, the corresponding tree node or button is hidden.
- **No privilege pre-checks**: admin actions (create/edit/drop users, create integrations, warehouse metering) are *not* gated by up-front Snowflake privilege probes — those checks were unreliable. Every action is offered; if the current role lacks the privilege, the IPC call fails and the error surfaces via `message.error` or inline under the edited field. The former `Can*` IPC methods and their client-side role-hierarchy walkers have been deleted.
- **Shared components**: `AccountPanel` renders `BackupPoliciesPanel` from `../backup/` and `PropertiesModal` from `../common/` directly.
- **Clipboard**: uses Wails `ClipboardSetText` (not `navigator.clipboard`, which is blocked in WKWebView).

## Gotchas

- `KeyPairAuthModal` depends on OS-available tools (`openssl` / `ssh-keygen`); `CheckAvailableKeyTools` must be called first to select the generation method — do not assume any tool is present.
- `WarehousePropertiesModal` and `UserPropertiesModal` delegate property edits to the backend builders (`AlterWarehouseProperty` / `AlterUserProperty`); do not add ALTER SQL inline in the components.
