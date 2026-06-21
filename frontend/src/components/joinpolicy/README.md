# components/joinpolicy

> UI for creating and inspecting Snowflake JOIN POLICY objects.

## Components

| File | Purpose |
|---|---|
| `CreateJoinPolicyModal.tsx` | Create dialog: name + replace/if-not-exists options, case control, a fixed (disabled) signature, a "Join required" switch that rewrites the body, a Monaco body editor (`JOIN_CONSTRAINT(...)` expression), an optional comment, and a live SQL preview. The signature is fixed to `() RETURNS JOIN_CONSTRAINT`. |
| `JoinPolicyPropertiesModal.tsx` | Properties dialog: Definition (signature / returns), an editable Body (`SET BODY -> …`), an editable Comment, a Tags section (`SET`/`UNSET TAG` with removable chips), the generic SHOW properties, and an on-demand References table (tables/views the policy is applied to). |

## Integration

- Both modals are rendered from `components/layout/Sidebar.tsx`, opened via the
  schema "Create Object → Security & Governance" submenu / the `JOIN POLICY`
  type node and the object node's "Properties…" entry. `RENAME TO` is reached via
  the context-menu "Rename…" item (default rename path).
- IPC: `BuildCreateJoinPolicySql` + `ExecDDL` (create), `GetObjectProperties`
  (load), `AlterJoinPolicy` (edit body / comment / tags / rename),
  `GetJoinPolicyTags` (tag chips), and `GetJoinPolicyReferences` (references).
- `JoinPolicyConfig` has no nested arrays, but the plain form-state object is
  still cast to `any` at the IPC boundary for consistency with the other create
  modals.

## Notes

- A join policy has a fixed, argument-less signature returning `JOIN_CONSTRAINT`;
  the body is a `JOIN_CONSTRAINT(JOIN_REQUIRED => <boolean_expression>)`
  expression and cannot reference user-defined functions, tables, or views.
- After creation, attach the policy to a table or view with
  `ALTER TABLE … ADD JOIN POLICY … ON (col, …)`.
- The Tags chips read `INFORMATION_SCHEMA.TAG_REFERENCES` (immediate
  consistency); the listing is best-effort, so `SET`/`UNSET TAG` still work if the
  read fails.
- The References table reads `ACCOUNT_USAGE.POLICY_REFERENCES`, which needs
  governance privileges and has propagation latency, so it is loaded on demand.
- Join policies require Enterprise Edition (or higher).
