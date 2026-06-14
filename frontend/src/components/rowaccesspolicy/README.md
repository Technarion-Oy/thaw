# components/rowaccesspolicy

> UI for creating and inspecting Snowflake ROW ACCESS POLICY objects.

## Components

| File | Purpose |
|---|---|
| `CreateRowAccessPolicyModal.tsx` | Create dialog: name + replace/if-not-exists options, case control, a signature editor (column arguments), a Monaco body editor (boolean expression), an optional comment, and a live SQL preview. Returns are fixed to `BOOLEAN`. |
| `RowAccessPolicyPropertiesModal.tsx` | Properties dialog: Definition (signature / returns), an editable Body (`SET BODY -> …`), an editable Comment, the generic SHOW properties, and an on-demand References table (tables/views the policy is applied to). |

## Integration

- Both modals are rendered from `components/layout/Sidebar.tsx`, opened via the
  schema "Create Object → Security & Governance" submenu / the
  `ROW ACCESS POLICY` type node and the object node's "Properties…" entry.
- IPC: `BuildCreateRowAccessPolicySql` + `ExecDDL` (create),
  `GetObjectProperties` (load), `AlterRowAccessPolicy` (edit body / comment /
  rename), and `GetRowAccessPolicyReferences` (references).
- The Wails-generated `RowAccessPolicyConfig` class carries a `convertValues`
  method (it has a nested `args` array), so the plain form-state object is cast
  to `any` at the IPC boundary.

## Notes

- A row access policy always returns `BOOLEAN`; `TRUE` keeps a row visible. There
  is no return-type field and no `EXEMPT_OTHER_POLICIES` option (unlike masking
  policies).
- After creation, attach the policy to a table or view with
  `ALTER TABLE … ADD ROW ACCESS POLICY … ON (col, …)`.
- The References table reads `ACCOUNT_USAGE.POLICY_REFERENCES`, which needs
  governance privileges and has propagation latency, so it is loaded on demand.
